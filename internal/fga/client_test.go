package fga_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/terenzio/OpenFGADemo/internal/fga"
)

// validStoreID is a valid ULID for use in tests.
const validStoreID = "01HXYZ0000000000000000000G"

// stubServer creates an httptest server that handles OpenFGA API calls.
func stubServer(t *testing.T, checkAllowed bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasSuffix(r.URL.Path, "/check"):
			resp := map[string]interface{}{"allowed": checkAllowed}
			json.NewEncoder(w).Encode(resp)

		case strings.HasSuffix(r.URL.Path, "/write"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{})

		case strings.HasSuffix(r.URL.Path, "/list-objects"):
			resp := map[string]interface{}{
				"objects": []string{"document:doc1", "document:doc2"},
			}
			json.NewEncoder(w).Encode(resp)

		case strings.HasSuffix(r.URL.Path, "/expand"):
			resp := map[string]interface{}{
				"tree": map[string]interface{}{
					"root": map[string]interface{}{
						"name": "document:doc1#viewer",
						"leaf": map[string]interface{}{
							"users": map[string]interface{}{
								"users": []string{"user:anne"},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found: " + r.URL.Path})
		}
	}))
}

func TestNewClient(t *testing.T) {
	srv := stubServer(t, true)
	defer srv.Close()

	c, err := fga.NewClient(srv.URL, validStoreID)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if c == nil {
		t.Fatal("NewClient() returned nil")
	}
	if c.SDK() == nil {
		t.Fatal("SDK() returned nil")
	}
}

func TestCheck_Allowed(t *testing.T) {
	srv := stubServer(t, true)
	defer srv.Close()

	c, err := fga.NewClient(srv.URL, validStoreID)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	allowed, err := c.Check(context.Background(), "user:anne", "viewer", "document:doc1")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !allowed {
		t.Error("Check() = false, want true")
	}
}

func TestCheck_Denied(t *testing.T) {
	srv := stubServer(t, false)
	defer srv.Close()

	c, err := fga.NewClient(srv.URL, validStoreID)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	allowed, err := c.Check(context.Background(), "user:anne", "viewer", "document:doc1")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if allowed {
		t.Error("Check() = true, want false")
	}
}

func TestWriteTuples(t *testing.T) {
	srv := stubServer(t, true)
	defer srv.Close()

	c, err := fga.NewClient(srv.URL, validStoreID)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tuples := []fga.TupleKey{
		fga.Tuple("user:anne", "viewer", "document:doc1"),
	}
	if err := c.WriteTuples(context.Background(), tuples); err != nil {
		t.Fatalf("WriteTuples() error = %v", err)
	}
}

func TestDeleteTuples(t *testing.T) {
	srv := stubServer(t, true)
	defer srv.Close()

	c, err := fga.NewClient(srv.URL, validStoreID)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tuples := []fga.TupleKey{
		fga.Tuple("user:anne", "viewer", "document:doc1"),
	}
	if err := c.DeleteTuples(context.Background(), tuples); err != nil {
		t.Fatalf("DeleteTuples() error = %v", err)
	}
}

func TestListObjects(t *testing.T) {
	srv := stubServer(t, true)
	defer srv.Close()

	c, err := fga.NewClient(srv.URL, validStoreID)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	objects, err := c.ListObjects(context.Background(), "user:anne", "viewer", "document")
	if err != nil {
		t.Fatalf("ListObjects() error = %v", err)
	}
	if len(objects) != 2 {
		t.Fatalf("ListObjects() returned %d objects, want 2", len(objects))
	}
	if objects[0] != "document:doc1" {
		t.Errorf("ListObjects()[0] = %q, want %q", objects[0], "document:doc1")
	}
}

func TestExpand(t *testing.T) {
	srv := stubServer(t, true)
	defer srv.Close()

	c, err := fga.NewClient(srv.URL, validStoreID)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tree, err := c.Expand(context.Background(), "viewer", "document:doc1")
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if tree == nil {
		t.Fatal("Expand() returned nil tree")
	}
}

func TestTuple(t *testing.T) {
	tk := fga.Tuple("user:anne", "viewer", "document:doc1")
	if tk.User != "user:anne" {
		t.Errorf("Tuple().User = %q, want %q", tk.User, "user:anne")
	}
	if tk.Relation != "viewer" {
		t.Errorf("Tuple().Relation = %q, want %q", tk.Relation, "viewer")
	}
	if tk.Object != "document:doc1" {
		t.Errorf("Tuple().Object = %q, want %q", tk.Object, "document:doc1")
	}
}
