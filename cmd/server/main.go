// Command server runs the OpenFGA demo HTTP server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/terenzio/OpenFGADemo/internal/fga"
	"github.com/terenzio/OpenFGADemo/internal/httpapi"
	"github.com/terenzio/OpenFGADemo/internal/store"
)

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// fgaAdapter bridges *fga.Client to the httpapi.FGAClient interface.
type fgaAdapter struct {
	client *fga.Client
}

func (a *fgaAdapter) Check(ctx context.Context, user, relation, object string) (bool, error) {
	return a.client.Check(ctx, user, relation, object)
}

func (a *fgaAdapter) ListObjects(ctx context.Context, user, relation, objectType string) ([]string, error) {
	return a.client.ListObjects(ctx, user, relation, objectType)
}

func (a *fgaAdapter) WriteTuples(ctx context.Context, tuples []httpapi.TupleWrite) error {
	fgaTuples := make([]fga.TupleKey, len(tuples))
	for i, t := range tuples {
		fgaTuples[i] = fga.Tuple(t.User, t.Relation, t.Object)
	}
	return a.client.WriteTuples(ctx, fgaTuples)
}

func (a *fgaAdapter) DeleteTuples(ctx context.Context, tuples []httpapi.TupleWrite) error {
	fgaTuples := make([]fga.TupleKey, len(tuples))
	for i, t := range tuples {
		fgaTuples[i] = fga.Tuple(t.User, t.Relation, t.Object)
	}
	return a.client.DeleteTuples(ctx, fgaTuples)
}

func main() {
	seedFlag := flag.Bool("seed", false, "seed demo data into store and FGA")
	exitFlag := flag.Bool("exit", false, "exit after seeding (use with -seed)")
	flag.Parse()

	apiURL := envOrDefault("FGA_API_URL", "http://localhost:8080")
	serverAddr := envOrDefault("SERVER_ADDR", ":8000")
	modelPath := envOrDefault("FGA_MODEL_PATH", "model_basic_demo/authorization-model.fga")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Bootstrap: create store + write authorization model.
	slog.Info("bootstrapping OpenFGA store and model", "api_url", apiURL, "model_path", modelPath)
	fgaClient, modelID, err := fga.EnsureStoreAndModel(ctx, apiURL, "openfga-demo", modelPath)
	if err != nil {
		slog.Error("failed to bootstrap OpenFGA", "error", err)
		os.Exit(1)
	}
	slog.Info("OpenFGA bootstrap complete", "model_id", modelID)

	adapter := &fgaAdapter{client: fgaClient}
	appStore := store.New()

	if *seedFlag {
		slog.Info("seeding demo data")
		if err := seedDemoData(ctx, appStore, adapter); err != nil {
			slog.Error("failed to seed demo data", "error", err)
			os.Exit(1)
		}
		slog.Info("demo data seeded successfully")
		if *exitFlag {
			return
		}
	}

	handler := httpapi.NewHandler(appStore, adapter)
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: handler.Routes(),
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("server listening", "addr", serverAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sig := <-sigCh
	slog.Info("received signal, shutting down", "signal", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

// seedDemoData populates the in-memory store and FGA with demo entities and
// relationships for the teaching walkthrough.
func seedDemoData(ctx context.Context, s *store.Store, fgaClient httpapi.FGAClient) error {
	// --- Organizations ---
	s.CreateOrganization(store.Organization{ID: "acme", Name: "Acme Corp", OwnerID: "alice"})

	// --- Folders ---
	s.CreateFolder(store.Folder{ID: "company", Name: "Company", OwnerID: "alice"})
	s.CreateFolder(store.Folder{ID: "product", Name: "Product", ParentID: "company", OwnerID: "alice"})

	// --- Documents ---
	s.CreateDocument(store.Document{
		ID: "roadmap", Title: "Product Roadmap", Content: "Secret roadmap content",
		FolderID: "product", OwnerID: "alice",
	})
	s.CreateDocument(store.Document{
		ID: "public-memo", Title: "Public Memo", Content: "This memo is visible to everyone",
		FolderID: "company", OwnerID: "alice",
	})

	// --- FGA tuples ---
	tuples := []httpapi.TupleWrite{
		// Organization membership. (The basic model's `organization` type
		// only defines `member`; org ownership lives in the in-memory store.)
		{User: "user:eve", Relation: "member", Object: "organization:acme"},
		{User: "user:frank", Relation: "member", Object: "organization:acme"},

		// Folder ownership.
		{User: "user:alice", Relation: "owner", Object: "folder:company"},
		{User: "user:alice", Relation: "owner", Object: "folder:product"},

		// Folder hierarchy: product is a child of company.
		{User: "folder:company", Relation: "parent", Object: "folder:product"},

		// Document ownership and parent folders.
		{User: "user:alice", Relation: "owner", Object: "document:roadmap"},
		{User: "folder:product", Relation: "parent", Object: "document:roadmap"},
		{User: "user:alice", Relation: "owner", Object: "document:public-memo"},
		{User: "folder:company", Relation: "parent", Object: "document:public-memo"},

		// Public memo visible to everyone.
		{User: "user:*", Relation: "viewer", Object: "document:public-memo"},

		// charlie is editor on folder:product.
		{User: "user:charlie", Relation: "editor", Object: "folder:product"},

		// diana is viewer on folder:company.
		{User: "user:diana", Relation: "viewer", Object: "folder:company"},

		// acme members are viewers on folder:product.
		{User: "organization:acme#member", Relation: "viewer", Object: "folder:product"},
	}

	if err := fgaClient.WriteTuples(ctx, tuples); err != nil {
		return fmt.Errorf("writing seed tuples: %w", err)
	}

	return nil
}
