# OpenFGA ReBAC Demo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a two-binary Go demo (CLI walkthrough + HTTP API) that teaches OpenFGA ReBAC concepts using a Google Drive-style permission model.

**Architecture:** Docker Compose runs MariaDB + OpenFGA server + Playground UI. Two Go binaries share an `internal/fga` wrapper: `cmd/cli` walks through concepts chapter-by-chapter, `cmd/server` serves a REST API that authorizes every request via OpenFGA. In-memory application state; FGA is the authorization system.

**Tech Stack:** Go 1.22+, OpenFGA Go SDK (`github.com/openfga/go-sdk`), chi router (`github.com/go-chi/chi/v5`), stdlib `log/slog`, Docker Compose, MariaDB 11.x.

**Design spec:** `docs/superpowers/specs/2026-04-15-openfga-demo-design.md`

---

## File Structure

```
OpenFGADemo/
├── docker-compose.yml
├── Makefile
├── go.mod
├── model/
│   └── authorization-model-basic.fga
├── cmd/
│   ├── cli/main.go
│   └── server/main.go
├── internal/
│   ├── fga/
│   │   ├── client.go
│   │   ├── bootstrap.go
│   │   └── client_test.go
│   ├── store/
│   │   ├── store.go
│   │   └── store_test.go
│   ├── auth/
│   │   ├── middleware.go
│   │   └── authorize.go
│   └── httpapi/
│       ├── handlers.go
│       ├── errors.go
│       └── handlers_test.go
├── scripts/
│   └── demo.sh
├── docs/
│   ├── model-explained.md
│   └── architecture.md
└── README.md
```

---

### Task 1: Docker Compose + Go module + FGA model

**Files:**
- Create: `docker-compose.yml`
- Create: `Makefile`
- Create: `go.mod`
- Create: `models/basic/authorization-model-basic.fga`

- [ ] **Step 1: Create `docker-compose.yml`**

```yaml
services:
  mariadb:
    image: mariadb:11
    environment:
      MYSQL_ROOT_PASSWORD: openfga
      MYSQL_DATABASE: openfga
      MYSQL_USER: openfga
      MYSQL_PASSWORD: openfga
    ports:
      - "3306:3306"
    healthcheck:
      test: ["CMD", "healthcheck.sh", "--connect", "--innodb_initialized"]
      interval: 5s
      timeout: 5s
      retries: 10

  openfga-migrate:
    image: openfga/openfga:latest
    command: migrate
    environment:
      OPENFGA_DATASTORE_ENGINE: mysql
      OPENFGA_DATASTORE_URI: "openfga:openfga@tcp(mariadb:3306)/openfga?parseTime=true"
    depends_on:
      mariadb:
        condition: service_healthy

  openfga:
    image: openfga/openfga:latest
    command: run
    environment:
      OPENFGA_DATASTORE_ENGINE: mysql
      OPENFGA_DATASTORE_URI: "openfga:openfga@tcp(mariadb:3306)/openfga?parseTime=true"
      OPENFGA_PLAYGROUND_ENABLED: "true"
    ports:
      - "8080:8080"
      - "3000:3000"
    depends_on:
      openfga-migrate:
        condition: service_completed_successfully
    healthcheck:
      test: ["CMD", "/usr/local/bin/grpc_health_probe", "-addr=:8081"]
      interval: 5s
      timeout: 5s
      retries: 10
```

- [ ] **Step 2: Create `models/basic/authorization-model-basic.fga`**

```
model
  schema 1.1

type user

type organization
  relations
    define member: [user]

type folder
  relations
    define owner: [user]
    define parent: [folder]
    define editor: [user, organization#member] or owner or editor from parent
    define viewer: [user, user:*, organization#member] or editor or viewer from parent

type document
  relations
    define parent: [folder]
    define owner: [user]
    define editor: [user, organization#member] or owner or editor from parent
    define viewer: [user, user:*, organization#member] or editor or viewer from parent
```

- [ ] **Step 3: Initialize Go module**

Run:
```bash
go mod init github.com/terenzio/OpenFGADemo
```

- [ ] **Step 4: Create `Makefile`**

```makefile
.PHONY: up down cli serve seed demo test lint

up:
	docker compose up -d
	@echo "Waiting for OpenFGA healthcheck..."
	@until curl -sf http://localhost:8080/healthz > /dev/null 2>&1; do sleep 1; done
	@echo "OpenFGA is ready. Playground: http://localhost:3000"

down:
	docker compose down -v

cli:
	go run ./cmd/cli

serve:
	go run ./cmd/server

seed:
	go run ./cmd/server -seed -exit

demo: up
	@sleep 2
	./scripts/demo.sh

test:
	go test ./...

lint:
	go vet ./...
```

- [ ] **Step 5: Verify Docker Compose starts**

Run:
```bash
make up
```
Expected: MariaDB starts, OpenFGA migrates and starts, Playground accessible at `http://localhost:3000`.

Run:
```bash
make down
```

- [ ] **Step 6: Commit**

```bash
git add docker-compose.yml Makefile go.mod model/
git commit -m "feat: add docker compose, go module, and fga model"
```

---

### Task 2: OpenFGA client wrapper (`internal/fga`)

**Files:**
- Create: `internal/fga/client.go`
- Create: `internal/fga/bootstrap.go`
- Create: `internal/fga/client_test.go`
- Modify: `go.mod` (via `go mod tidy`)

- [ ] **Step 1: Write failing test for FGA client Check**

Create `internal/fga/client_test.go`:

```go
package fga_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/terenzio/OpenFGADemo/internal/fga"
)

func TestCheck_Allowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stores" && r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(map[string]string{"id": "store-1", "name": "test"})
			return
		}
		if r.Method == http.MethodPost && contains(r.URL.Path, "/check") {
			json.NewEncoder(w).Encode(map[string]bool{"allowed": true})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client, err := fga.NewClient(srv.URL, "store-1")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	allowed, err := client.Check(context.Background(), "user:alice", "viewer", "document:roadmap")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !allowed {
		t.Error("expected allowed=true")
	}
}

func TestCheck_Denied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stores" && r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(map[string]string{"id": "store-1", "name": "test"})
			return
		}
		if r.Method == http.MethodPost && contains(r.URL.Path, "/check") {
			json.NewEncoder(w).Encode(map[string]bool{"allowed": false})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client, err := fga.NewClient(srv.URL, "store-1")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	allowed, err := client.Check(context.Background(), "user:bob", "viewer", "document:roadmap")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if allowed {
		t.Error("expected allowed=false")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/fga/ -v
```
Expected: FAIL — package `fga` does not exist yet.

- [ ] **Step 3: Write `internal/fga/client.go`**

```go
package fga

import (
	"context"
	"fmt"

	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/client"
)

type Client struct {
	sdk     *client.OpenFgaClient
	storeID string
}

func NewClient(apiURL, storeID string) (*Client, error) {
	cfg, err := client.NewConfiguration(client.ClientConfiguration{
		ApiUrl:  apiURL,
		StoreId: storeID,
	})
	if err != nil {
		return nil, fmt.Errorf("fga config: %w", err)
	}

	sdk := client.NewOpenFgaClient(cfg)
	return &Client{sdk: sdk, storeID: storeID}, nil
}

func (c *Client) Check(ctx context.Context, user, relation, object string) (bool, error) {
	body := client.ClientCheckRequest{
		User:     user,
		Relation: relation,
		Object:   object,
	}
	resp, err := c.sdk.Check(ctx).Body(body).Execute()
	if err != nil {
		return false, fmt.Errorf("fga check: %w", err)
	}
	return resp.GetAllowed(), nil
}

func (c *Client) WriteTuples(ctx context.Context, tuples []openfga.TupleKey) error {
	body := client.ClientWriteRequest{
		Writes: tupleKeysToKeys(tuples),
	}
	_, err := c.sdk.Write(ctx).Body(body).Execute()
	if err != nil {
		return fmt.Errorf("fga write: %w", err)
	}
	return nil
}

func (c *Client) DeleteTuples(ctx context.Context, tuples []openfga.TupleKey) error {
	body := client.ClientWriteRequest{
		Deletes: tupleKeysToKeys(tuples),
	}
	_, err := c.sdk.Write(ctx).Body(body).Execute()
	if err != nil {
		return fmt.Errorf("fga delete: %w", err)
	}
	return nil
}

func (c *Client) ListObjects(ctx context.Context, user, relation, objectType string) ([]string, error) {
	body := client.ClientListObjectsRequest{
		User:     user,
		Relation: relation,
		Type:     objectType,
	}
	resp, err := c.sdk.ListObjects(ctx).Body(body).Execute()
	if err != nil {
		return nil, fmt.Errorf("fga list objects: %w", err)
	}
	return resp.GetObjects(), nil
}

func (c *Client) Expand(ctx context.Context, relation, object string) (*openfga.UsersetTree, error) {
	body := client.ClientExpandRequest{
		Relation: relation,
		Object:   object,
	}
	resp, err := c.sdk.Expand(ctx).Body(body).Execute()
	if err != nil {
		return nil, fmt.Errorf("fga expand: %w", err)
	}
	return resp.Tree, nil
}

func (c *Client) SDK() *client.OpenFgaClient {
	return c.sdk
}

func tupleKeysToKeys(tuples []openfga.TupleKey) []client.ClientTupleKey {
	keys := make([]client.ClientTupleKey, len(tuples))
	for i, t := range tuples {
		keys[i] = client.ClientTupleKey{
			User:     t.User,
			Relation: t.Relation,
			Object:   t.Object,
		}
	}
	return keys
}

func Tuple(user, relation, object string) openfga.TupleKey {
	return openfga.TupleKey{
		User:     user,
		Relation: relation,
		Object:   object,
	}
}
```

- [ ] **Step 4: Write `internal/fga/bootstrap.go`**

```go
package fga

import (
	"context"
	"fmt"
	"os"

	"github.com/openfga/go-sdk/client"
)

func (c *Client) Bootstrap(ctx context.Context, modelPath string) (string, error) {
	modelDSL, err := os.ReadFile(modelPath)
	if err != nil {
		return "", fmt.Errorf("read model: %w", err)
	}

	dsl := string(modelDSL)
	body := client.ClientWriteAuthorizationModelRequest{
		SchemaVersion: "1.1",
	}
	_ = body // We'll use the raw DSL approach via the SDK's JSON transformation

	resp, err := c.sdk.WriteAuthorizationModel(ctx).Body(dslToJSON(dsl)).Execute()
	if err != nil {
		return "", fmt.Errorf("write model: %w", err)
	}

	modelID := resp.GetAuthorizationModelId()
	return modelID, nil
}

func CreateStore(ctx context.Context, apiURL, storeName string) (string, error) {
	cfg, err := client.NewConfiguration(client.ClientConfiguration{
		ApiUrl: apiURL,
	})
	if err != nil {
		return "", fmt.Errorf("fga config: %w", err)
	}

	sdk := client.NewOpenFgaClient(cfg)
	resp, err := sdk.CreateStore(ctx).Body(client.ClientCreateStoreRequest{
		Name: storeName,
	}).Execute()
	if err != nil {
		return "", fmt.Errorf("create store: %w", err)
	}

	return resp.GetId(), nil
}

func EnsureStoreAndModel(ctx context.Context, apiURL, storeName, modelPath string) (*Client, string, error) {
	storeID, err := CreateStore(ctx, apiURL, storeName)
	if err != nil {
		return nil, "", err
	}

	c, err := NewClient(apiURL, storeID)
	if err != nil {
		return nil, "", err
	}

	modelID, err := c.Bootstrap(ctx, modelPath)
	if err != nil {
		return nil, "", err
	}

	return c, modelID, nil
}
```

Note: The `dslToJSON` function converts DSL to the JSON format the SDK expects. The OpenFGA Go SDK provides a transformer for this. We need to use the `github.com/openfga/language/pkg/go/transformer` package. Update `bootstrap.go` to import and use it:

Replace the `dslToJSON(dsl)` placeholder with the actual transformer call. The implementation should use `transformer.TransformDSLToJSON(dsl)` from `github.com/openfga/language/pkg/go/transformer`. If that package is not available or has changed, use the OpenFGA CLI's approach: read the `.fga` file and use the SDK's `WriteAuthorizationModel` with the parsed JSON model. The exact import path may need adjustment based on the current SDK version — check `pkg.go.dev` for the correct path.

The updated `bootstrap.go`:

```go
package fga

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/client"
	language "github.com/openfga/language/pkg/go/transformer"
)

func (c *Client) Bootstrap(ctx context.Context, modelPath string) (string, error) {
	modelDSL, err := os.ReadFile(modelPath)
	if err != nil {
		return "", fmt.Errorf("read model: %w", err)
	}

	modelJSON, err := language.TransformDSLToJSON(string(modelDSL))
	if err != nil {
		return "", fmt.Errorf("transform model: %w", err)
	}

	var body openfga.WriteAuthorizationModelRequest
	if err := json.Unmarshal([]byte(modelJSON), &body); err != nil {
		return "", fmt.Errorf("parse model json: %w", err)
	}

	resp, err := c.sdk.WriteAuthorizationModel(ctx).Body(body).Execute()
	if err != nil {
		return "", fmt.Errorf("write model: %w", err)
	}

	return resp.GetAuthorizationModelId(), nil
}

func CreateStore(ctx context.Context, apiURL, storeName string) (string, error) {
	cfg, err := client.NewConfiguration(client.ClientConfiguration{
		ApiUrl: apiURL,
	})
	if err != nil {
		return "", fmt.Errorf("fga config: %w", err)
	}

	sdk := client.NewOpenFgaClient(cfg)
	resp, err := sdk.CreateStore(ctx).Body(client.ClientCreateStoreRequest{
		Name: storeName,
	}).Execute()
	if err != nil {
		return "", fmt.Errorf("create store: %w", err)
	}

	return resp.GetId(), nil
}

func EnsureStoreAndModel(ctx context.Context, apiURL, storeName, modelPath string) (*Client, string, error) {
	storeID, err := CreateStore(ctx, apiURL, storeName)
	if err != nil {
		return nil, "", err
	}

	c, err := NewClient(apiURL, storeID)
	if err != nil {
		return nil, "", err
	}

	modelID, err := c.Bootstrap(ctx, modelPath)
	if err != nil {
		return nil, "", err
	}

	return c, modelID, nil
}
```

- [ ] **Step 5: Run `go mod tidy`**

Run:
```bash
go mod tidy
```

- [ ] **Step 6: Run tests**

Run:
```bash
go test ./internal/fga/ -v
```
Expected: PASS (both `TestCheck_Allowed` and `TestCheck_Denied`).

If the OpenFGA SDK client constructor or Check method behaves differently than expected with the stub server (e.g., SDK validates response schema more strictly), adjust the stub handler to return well-formed OpenFGA JSON responses. For example, the check response should include:
```json
{"allowed": true, "resolution": ""}
```

- [ ] **Step 7: Commit**

```bash
git add internal/fga/ go.mod go.sum
git commit -m "feat: add OpenFGA client wrapper with Check, WriteTuples, ListObjects, Expand"
```

---

### Task 3: In-memory store (`internal/store`)

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/store_test.go`

- [ ] **Step 1: Write failing tests for the store**

Create `internal/store/store_test.go`:

```go
package store_test

import (
	"testing"

	"github.com/terenzio/OpenFGADemo/internal/store"
)

func TestCreateAndGetDocument(t *testing.T) {
	s := store.New()
	doc := store.Document{ID: "doc1", Title: "Roadmap", Content: "v1 plan", FolderID: "folder1", OwnerID: "alice"}
	s.CreateDocument(doc)

	got, ok := s.GetDocument("doc1")
	if !ok {
		t.Fatal("expected document to exist")
	}
	if got.Title != "Roadmap" {
		t.Errorf("title = %q, want %q", got.Title, "Roadmap")
	}
}

func TestGetDocument_NotFound(t *testing.T) {
	s := store.New()
	_, ok := s.GetDocument("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestUpdateDocument(t *testing.T) {
	s := store.New()
	s.CreateDocument(store.Document{ID: "doc1", Title: "Old", Content: "old", FolderID: "f1", OwnerID: "alice"})

	ok := s.UpdateDocument("doc1", "New Title", "new content")
	if !ok {
		t.Fatal("expected update to succeed")
	}

	got, _ := s.GetDocument("doc1")
	if got.Title != "New Title" || got.Content != "new content" {
		t.Errorf("got %+v", got)
	}
}

func TestDeleteDocument(t *testing.T) {
	s := store.New()
	s.CreateDocument(store.Document{ID: "doc1", Title: "T", Content: "C", FolderID: "f1", OwnerID: "alice"})
	s.DeleteDocument("doc1")

	_, ok := s.GetDocument("doc1")
	if ok {
		t.Error("expected document to be deleted")
	}
}

func TestCreateAndGetFolder(t *testing.T) {
	s := store.New()
	s.CreateFolder(store.Folder{ID: "f1", Name: "Product", ParentID: "", OwnerID: "alice"})

	got, ok := s.GetFolder("f1")
	if !ok {
		t.Fatal("expected folder to exist")
	}
	if got.Name != "Product" {
		t.Errorf("name = %q, want %q", got.Name, "Product")
	}
}

func TestListDocuments(t *testing.T) {
	s := store.New()
	s.CreateDocument(store.Document{ID: "doc1", Title: "A", Content: "", FolderID: "f1", OwnerID: "alice"})
	s.CreateDocument(store.Document{ID: "doc2", Title: "B", Content: "", FolderID: "f1", OwnerID: "bob"})

	docs := s.ListDocumentsByIDs([]string{"doc1", "doc2"})
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestListFolders(t *testing.T) {
	s := store.New()
	s.CreateFolder(store.Folder{ID: "f1", Name: "A", OwnerID: "alice"})
	s.CreateFolder(store.Folder{ID: "f2", Name: "B", OwnerID: "bob"})

	folders := s.ListFoldersByIDs([]string{"f1"})
	if len(folders) != 1 {
		t.Errorf("expected 1 folder, got %d", len(folders))
	}
}

func TestCreateAndGetOrganization(t *testing.T) {
	s := store.New()
	s.CreateOrganization(store.Organization{ID: "acme", Name: "Acme Corp", OwnerID: "alice"})

	got, ok := s.GetOrganization("acme")
	if !ok {
		t.Fatal("expected org to exist")
	}
	if got.Name != "Acme Corp" {
		t.Errorf("name = %q, want %q", got.Name, "Acme Corp")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/store/ -v
```
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write `internal/store/store.go`**

```go
package store

import "sync"

type Document struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	FolderID string `json:"folder_id"`
	OwnerID  string `json:"owner_id"`
}

type Folder struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parent_id,omitempty"`
	OwnerID  string `json:"owner_id"`
}

type Organization struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OwnerID string `json:"owner_id"`
}

type Store struct {
	mu    sync.RWMutex
	docs  map[string]Document
	dirs  map[string]Folder
	orgs  map[string]Organization
}

func New() *Store {
	return &Store{
		docs: make(map[string]Document),
		dirs: make(map[string]Folder),
		orgs: make(map[string]Organization),
	}
}

func (s *Store) CreateDocument(d Document) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[d.ID] = d
}

func (s *Store) GetDocument(id string) (Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.docs[id]
	return d, ok
}

func (s *Store) UpdateDocument(id, title, content string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.docs[id]
	if !ok {
		return false
	}
	d.Title = title
	d.Content = content
	s.docs[id] = d
	return true
}

func (s *Store) DeleteDocument(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, id)
}

func (s *Store) ListDocumentsByIDs(ids []string) []Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Document
	for _, id := range ids {
		if d, ok := s.docs[id]; ok {
			result = append(result, d)
		}
	}
	return result
}

func (s *Store) CreateFolder(f Folder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirs[f.ID] = f
}

func (s *Store) GetFolder(id string) (Folder, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.dirs[id]
	return f, ok
}

func (s *Store) ListFoldersByIDs(ids []string) []Folder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Folder
	for _, id := range ids {
		if f, ok := s.dirs[id]; ok {
			result = append(result, f)
		}
	}
	return result
}

func (s *Store) CreateOrganization(o Organization) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orgs[o.ID] = o
}

func (s *Store) GetOrganization(id string) (Organization, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orgs[id]
	return o, ok
}
```

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/store/ -v
```
Expected: PASS (all 8 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: add in-memory store for documents, folders, and organizations"
```

---

### Task 4: Auth middleware and authorize helper (`internal/auth`)

**Files:**
- Create: `internal/auth/middleware.go`
- Create: `internal/auth/authorize.go`

- [ ] **Step 1: Create `internal/auth/middleware.go`**

```go
package auth

import (
	"context"
	"net/http"
)

type contextKey string

const userIDKey contextKey = "user_id"

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-Id")
		if userID == "" {
			http.Error(w, `{"error":"missing X-User-Id header"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}
```

- [ ] **Step 2: Create `internal/auth/authorize.go`**

```go
package auth

import (
	"context"
	"fmt"
)

type Checker interface {
	Check(ctx context.Context, user, relation, object string) (bool, error)
}

func Authorize(ctx context.Context, checker Checker, relation, objectType, objectID string) error {
	userID := UserFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("no user in context")
	}

	allowed, err := checker.Check(ctx, fmt.Sprintf("user:%s", userID), relation, fmt.Sprintf("%s:%s", objectType, objectID))
	if err != nil {
		return fmt.Errorf("authorization check failed: %w", err)
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

var ErrForbidden = fmt.Errorf("forbidden")
```

- [ ] **Step 3: Run `go vet`**

Run:
```bash
go vet ./internal/auth/
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/auth/
git commit -m "feat: add auth middleware (X-User-Id) and FGA authorize helper"
```

---

### Task 5: HTTP API errors and handlers (`internal/httpapi`)

**Files:**
- Create: `internal/httpapi/errors.go`
- Create: `internal/httpapi/handlers.go`
- Create: `internal/httpapi/handlers_test.go`

- [ ] **Step 1: Create `internal/httpapi/errors.go`**

```go
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/terenzio/OpenFGADemo/internal/auth"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, err error) {
	if errors.Is(err, auth.ErrForbidden) {
		WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden"})
		return
	}
	WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
}
```

- [ ] **Step 2: Write failing tests for key handlers**

Create `internal/httpapi/handlers_test.go`:

```go
package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/terenzio/OpenFGADemo/internal/auth"
	"github.com/terenzio/OpenFGADemo/internal/httpapi"
	"github.com/terenzio/OpenFGADemo/internal/store"
)

type fakeFGA struct {
	allowed     bool
	listObjects []string
}

func (f *fakeFGA) Check(_ context.Context, user, relation, object string) (bool, error) {
	return f.allowed, nil
}

func (f *fakeFGA) ListObjects(_ context.Context, user, relation, objectType string) ([]string, error) {
	return f.listObjects, nil
}

func (f *fakeFGA) WriteTuples(_ context.Context, tuples []store.Tuple) error {
	return nil
}

func (f *fakeFGA) DeleteTuples(_ context.Context, tuples []store.Tuple) error {
	return nil
}

func TestGetDocument_Allowed(t *testing.T) {
	s := store.New()
	s.CreateDocument(store.Document{ID: "doc1", Title: "Roadmap", Content: "plan", FolderID: "f1", OwnerID: "alice"})

	fga := &fakeFGA{allowed: true}
	h := httpapi.NewHandler(s, fga)

	req := httptest.NewRequest(http.MethodGet, "/documents/doc1", nil)
	req = withUser(req, "alice")
	req = withChiParam(req, "id", "doc1")
	rec := httptest.NewRecorder()

	h.GetDocument(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var doc store.Document
	json.NewDecoder(rec.Body).Decode(&doc)
	if doc.Title != "Roadmap" {
		t.Errorf("title = %q, want Roadmap", doc.Title)
	}
}

func TestGetDocument_Forbidden(t *testing.T) {
	s := store.New()
	s.CreateDocument(store.Document{ID: "doc1", Title: "Roadmap", Content: "plan", FolderID: "f1", OwnerID: "alice"})

	fga := &fakeFGA{allowed: false}
	h := httpapi.NewHandler(s, fga)

	req := httptest.NewRequest(http.MethodGet, "/documents/doc1", nil)
	req = withUser(req, "bob")
	req = withChiParam(req, "id", "doc1")
	rec := httptest.NewRecorder()

	h.GetDocument(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestListDocuments(t *testing.T) {
	s := store.New()
	s.CreateDocument(store.Document{ID: "doc1", Title: "A", FolderID: "f1", OwnerID: "alice"})
	s.CreateDocument(store.Document{ID: "doc2", Title: "B", FolderID: "f1", OwnerID: "alice"})

	fga := &fakeFGA{listObjects: []string{"document:doc1", "document:doc2"}}
	h := httpapi.NewHandler(s, fga)

	req := httptest.NewRequest(http.MethodGet, "/documents", nil)
	req = withUser(req, "alice")
	rec := httptest.NewRecorder()

	h.ListDocuments(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var docs []store.Document
	json.NewDecoder(rec.Body).Decode(&docs)
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestCreateDocument_Forbidden(t *testing.T) {
	s := store.New()
	s.CreateFolder(store.Folder{ID: "f1", Name: "Product", OwnerID: "alice"})

	fga := &fakeFGA{allowed: false}
	h := httpapi.NewHandler(s, fga)

	body := `{"id":"doc1","title":"New","content":"stuff","folder_id":"f1"}`
	req := httptest.NewRequest(http.MethodPost, "/documents", strings.NewReader(body))
	req = withUser(req, "bob")
	rec := httptest.NewRecorder()

	h.CreateDocument(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func withUser(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), auth.UserIDKey, userID)
	return r.WithContext(ctx)
}

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
```

Note: The test uses `auth.UserIDKey` which means the `contextKey` type and `userIDKey` constant in `internal/auth/middleware.go` need to be exported. Update `internal/auth/middleware.go`:

Change:
```go
type contextKey string
const userIDKey contextKey = "user_id"
```
To:
```go
type ContextKey string
const UserIDKey ContextKey = "user_id"
```

And update `UserFromContext` to use `UserIDKey`.

- [ ] **Step 3: Run tests to verify they fail**

Run:
```bash
go test ./internal/httpapi/ -v
```
Expected: FAIL — `httpapi` package does not exist.

- [ ] **Step 4: Create `internal/httpapi/handlers.go`**

```go
package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/terenzio/OpenFGADemo/internal/auth"
	"github.com/terenzio/OpenFGADemo/internal/store"
)

type FGAClient interface {
	Check(ctx context.Context, user, relation, object string) (bool, error)
	ListObjects(ctx context.Context, user, relation, objectType string) ([]string, error)
	WriteTuples(ctx context.Context, tuples []store.Tuple) error
	DeleteTuples(ctx context.Context, tuples []store.Tuple) error
}
```

Wait — we shouldn't make `httpapi` depend on `store` for the `Tuple` type. Instead, define the tuple type in the `fga` package (it's already there as `openfga.TupleKey`). Let's use a simpler interface. Here's the corrected full `handlers.go`:

```go
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/terenzio/OpenFGADemo/internal/auth"
	"github.com/terenzio/OpenFGADemo/internal/store"
)

type FGAClient interface {
	Check(ctx context.Context, user, relation, object string) (bool, error)
	ListObjects(ctx context.Context, user, relation, objectType string) ([]string, error)
	WriteTuples(ctx context.Context, tuples []TupleWrite) error
	DeleteTuples(ctx context.Context, tuples []TupleWrite) error
}

type TupleWrite struct {
	User     string
	Relation string
	Object   string
}

type Handler struct {
	store *store.Store
	fga   FGAClient
}

func NewHandler(s *store.Store, fga FGAClient) *Handler {
	return &Handler{store: s, fga: fga}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.Middleware)

	r.Post("/organizations/{id}/members", h.AddOrgMember)
	r.Post("/folders", h.CreateFolder)
	r.Get("/folders", h.ListFolders)
	r.Post("/documents", h.CreateDocument)
	r.Get("/documents", h.ListDocuments)
	r.Get("/documents/{id}", h.GetDocument)
	r.Put("/documents/{id}", h.UpdateDocument)
	r.Post("/documents/{id}/share", h.ShareDocument)
	r.Delete("/documents/{id}/share", h.UnshareDocument)

	return r
}

func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFromContext(r.Context())
	docID := chi.URLParam(r, "id")

	allowed, err := h.fga.Check(r.Context(), fmt.Sprintf("user:%s", userID), "viewer", fmt.Sprintf("document:%s", docID))
	if err != nil {
		slog.Error("fga check failed", "error", err)
		WriteError(w, err)
		return
	}
	if !allowed {
		WriteError(w, auth.ErrForbidden)
		return
	}

	doc, ok := h.store.GetDocument(docID)
	if !ok {
		WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "not found"})
		return
	}
	WriteJSON(w, http.StatusOK, doc)
}

func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFromContext(r.Context())

	objectIDs, err := h.fga.ListObjects(r.Context(), fmt.Sprintf("user:%s", userID), "viewer", "document")
	if err != nil {
		WriteError(w, err)
		return
	}

	ids := make([]string, 0, len(objectIDs))
	for _, obj := range objectIDs {
		parts := strings.SplitN(obj, ":", 2)
		if len(parts) == 2 {
			ids = append(ids, parts[1])
		}
	}

	docs := h.store.ListDocumentsByIDs(ids)
	WriteJSON(w, http.StatusOK, docs)
}

func (h *Handler) CreateDocument(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFromContext(r.Context())

	var req struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		FolderID string `json:"folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid json"})
		return
	}

	if err := auth.Authorize(r.Context(), h.fga, "editor", "folder", req.FolderID); err != nil {
		WriteError(w, err)
		return
	}

	doc := store.Document{
		ID:       req.ID,
		Title:    req.Title,
		Content:  req.Content,
		FolderID: req.FolderID,
		OwnerID:  userID,
	}
	h.store.CreateDocument(doc)

	err := h.fga.WriteTuples(r.Context(), []TupleWrite{
		{User: fmt.Sprintf("user:%s", userID), Relation: "owner", Object: fmt.Sprintf("document:%s", req.ID)},
		{User: fmt.Sprintf("folder:%s", req.FolderID), Relation: "parent", Object: fmt.Sprintf("document:%s", req.ID)},
	})
	if err != nil {
		h.store.DeleteDocument(req.ID)
		slog.Error("fga write failed, rolled back doc", "error", err)
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, doc)
}

func (h *Handler) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")

	if err := auth.Authorize(r.Context(), h.fga, "editor", "document", docID); err != nil {
		WriteError(w, err)
		return
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid json"})
		return
	}

	if ok := h.store.UpdateDocument(docID, req.Title, req.Content); !ok {
		WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "not found"})
		return
	}

	doc, _ := h.store.GetDocument(docID)
	WriteJSON(w, http.StatusOK, doc)
}

func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFromContext(r.Context())

	var req struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		ParentID string `json:"parent_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid json"})
		return
	}

	folder := store.Folder{ID: req.ID, Name: req.Name, ParentID: req.ParentID, OwnerID: userID}
	h.store.CreateFolder(folder)

	tuples := []TupleWrite{
		{User: fmt.Sprintf("user:%s", userID), Relation: "owner", Object: fmt.Sprintf("folder:%s", req.ID)},
	}
	if req.ParentID != "" {
		tuples = append(tuples, TupleWrite{
			User: fmt.Sprintf("folder:%s", req.ParentID), Relation: "parent", Object: fmt.Sprintf("folder:%s", req.ID),
		})
	}

	if err := h.fga.WriteTuples(r.Context(), tuples); err != nil {
		slog.Error("fga write failed for folder", "error", err)
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, folder)
}

func (h *Handler) ListFolders(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFromContext(r.Context())

	objectIDs, err := h.fga.ListObjects(r.Context(), fmt.Sprintf("user:%s", userID), "viewer", "folder")
	if err != nil {
		WriteError(w, err)
		return
	}

	ids := make([]string, 0, len(objectIDs))
	for _, obj := range objectIDs {
		parts := strings.SplitN(obj, ":", 2)
		if len(parts) == 2 {
			ids = append(ids, parts[1])
		}
	}

	folders := h.store.ListFoldersByIDs(ids)
	WriteJSON(w, http.StatusOK, folders)
}

func (h *Handler) AddOrgMember(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")

	if err := auth.Authorize(r.Context(), h.fga, "owner", "organization", orgID); err != nil {
		WriteError(w, err)
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid json"})
		return
	}

	err := h.fga.WriteTuples(r.Context(), []TupleWrite{
		{User: fmt.Sprintf("user:%s", req.UserID), Relation: "member", Object: fmt.Sprintf("organization:%s", orgID)},
	})
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ShareDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")

	if err := auth.Authorize(r.Context(), h.fga, "owner", "document", docID); err != nil {
		WriteError(w, err)
		return
	}

	var req struct {
		User     string `json:"user"`
		Relation string `json:"relation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid json"})
		return
	}

	err := h.fga.WriteTuples(r.Context(), []TupleWrite{
		{User: req.User, Relation: req.Relation, Object: fmt.Sprintf("document:%s", docID)},
	})
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "shared"})
}

func (h *Handler) UnshareDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")

	if err := auth.Authorize(r.Context(), h.fga, "owner", "document", docID); err != nil {
		WriteError(w, err)
		return
	}

	var req struct {
		User     string `json:"user"`
		Relation string `json:"relation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid json"})
		return
	}

	err := h.fga.DeleteTuples(r.Context(), []TupleWrite{
		{User: req.User, Relation: req.Relation, Object: fmt.Sprintf("document:%s", docID)},
	})
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "unshared"})
}
```

Update the test file's `fakeFGA` to match the `FGAClient` interface (using `TupleWrite` instead of `store.Tuple`):

```go
func (f *fakeFGA) WriteTuples(_ context.Context, tuples []httpapi.TupleWrite) error {
	return nil
}

func (f *fakeFGA) DeleteTuples(_ context.Context, tuples []httpapi.TupleWrite) error {
	return nil
}
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/httpapi/ -v
```
Expected: PASS (all 4 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/httpapi/ internal/auth/
git commit -m "feat: add HTTP handlers with FGA authorization checks"
```

---

### Task 6: HTTP server binary (`cmd/server`)

**Files:**
- Create: `cmd/server/main.go`

- [ ] **Step 1: Create `cmd/server/main.go`**

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	openfga "github.com/openfga/go-sdk"
	"github.com/terenzio/OpenFGADemo/internal/fga"
	"github.com/terenzio/OpenFGADemo/internal/httpapi"
	"github.com/terenzio/OpenFGADemo/internal/store"
)

func main() {
	seedFlag := flag.Bool("seed", false, "Seed the store with demo data")
	exitFlag := flag.Bool("exit", false, "Exit after seeding (use with -seed)")
	flag.Parse()

	apiURL := envOrDefault("FGA_API_URL", "http://localhost:8080")
	addr := envOrDefault("SERVER_ADDR", ":8000")
	modelPath := envOrDefault("FGA_MODEL_PATH", "models/basic/authorization-model-basic.fga")

	ctx := context.Background()

	slog.Info("bootstrapping OpenFGA store and model")
	client, modelID, err := fga.EnsureStoreAndModel(ctx, apiURL, "openfga-demo", modelPath)
	if err != nil {
		slog.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
	slog.Info("OpenFGA ready", "model_id", modelID)

	appStore := store.New()

	if *seedFlag {
		slog.Info("seeding demo data")
		if err := seedDemoData(ctx, client, appStore); err != nil {
			slog.Error("seed failed", "error", err)
			os.Exit(1)
		}
		slog.Info("seed complete")
		if *exitFlag {
			return
		}
	}

	adapter := &fgaAdapter{client: client}
	handler := httpapi.NewHandler(appStore, adapter)

	srv := &http.Server{
		Addr:    addr,
		Handler: handler.Routes(),
	}

	go func() {
		slog.Info("server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	slog.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutCtx)
}

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
	fgaTuples := make([]openfga.TupleKey, len(tuples))
	for i, t := range tuples {
		fgaTuples[i] = fga.Tuple(t.User, t.Relation, t.Object)
	}
	return a.client.WriteTuples(ctx, fgaTuples)
}

func (a *fgaAdapter) DeleteTuples(ctx context.Context, tuples []httpapi.TupleWrite) error {
	fgaTuples := make([]openfga.TupleKey, len(tuples))
	for i, t := range tuples {
		fgaTuples[i] = fga.Tuple(t.User, t.Relation, t.Object)
	}
	return a.client.DeleteTuples(ctx, fgaTuples)
}

func seedDemoData(ctx context.Context, client *fga.Client, appStore *store.Store) error {
	appStore.CreateOrganization(store.Organization{ID: "acme", Name: "Acme Corp", OwnerID: "alice"})
	appStore.CreateFolder(store.Folder{ID: "company", Name: "Company", OwnerID: "alice"})
	appStore.CreateFolder(store.Folder{ID: "product", Name: "Product", ParentID: "company", OwnerID: "alice"})
	appStore.CreateDocument(store.Document{ID: "roadmap", Title: "Product Roadmap", Content: "Q3 goals and milestones", FolderID: "product", OwnerID: "alice"})
	appStore.CreateDocument(store.Document{ID: "public-memo", Title: "Public Memo", Content: "Company-wide announcement", FolderID: "company", OwnerID: "alice"})

	tuples := []openfga.TupleKey{
		fga.Tuple("user:alice", "owner", "organization:acme"),
		fga.Tuple("user:eve", "member", "organization:acme"),
		fga.Tuple("user:frank", "member", "organization:acme"),

		fga.Tuple("user:alice", "owner", "folder:company"),
		fga.Tuple("user:alice", "owner", "folder:product"),
		fga.Tuple("folder:company", "parent", "folder:product"),

		fga.Tuple("user:alice", "owner", "document:roadmap"),
		fga.Tuple("folder:product", "parent", "document:roadmap"),
		fga.Tuple("user:charlie", "editor", "folder:product"),
		fga.Tuple("user:diana", "viewer", "folder:company"),

		fga.Tuple("user:alice", "owner", "document:public-memo"),
		fga.Tuple("folder:company", "parent", "document:public-memo"),
		fga.Tuple("user:*", "viewer", "document:public-memo"),

		fga.Tuple("organization:acme#member", "viewer", "folder:product"),
	}

	return client.WriteTuples(ctx, tuples)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 2: Run `go mod tidy`**

Run:
```bash
go mod tidy
```

- [ ] **Step 3: Verify build**

Run:
```bash
go build ./cmd/server
```
Expected: compiles without error.

- [ ] **Step 4: Commit**

```bash
git add cmd/server/ go.mod go.sum
git commit -m "feat: add HTTP server binary with seed command"
```

---

### Task 7: CLI teaching walkthrough (`cmd/cli`)

**Files:**
- Create: `cmd/cli/main.go`

- [ ] **Step 1: Create `cmd/cli/main.go`**

```go
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	openfga "github.com/openfga/go-sdk"
	"github.com/terenzio/OpenFGADemo/internal/fga"
)

var pause bool

func main() {
	flag.BoolVar(&pause, "pause", false, "Pause between chapters (press Enter to continue)")
	auto := flag.Bool("auto", true, "Run without pauses (default)")
	flag.Parse()
	if *auto && !isFlagSet("pause") {
		pause = false
	}

	apiURL := envOrDefault("FGA_API_URL", "http://localhost:8080")
	modelPath := envOrDefault("FGA_MODEL_PATH", "models/basic/authorization-model-basic.fga")

	ctx := context.Background()

	chapter1_Bootstrap(ctx, apiURL, modelPath)
}

func isFlagSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

var client *fga.Client

func chapter1_Bootstrap(ctx context.Context, apiURL, modelPath string) {
	printHeader("Chapter 1: Bootstrap")
	fmt.Println("Creating OpenFGA store and writing authorization model...")

	var modelID string
	var err error
	client, modelID, err = fga.EnsureStoreAndModel(ctx, apiURL, "openfga-demo-cli", modelPath)
	if err != nil {
		slog.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}

	fmt.Printf("  Store created successfully\n")
	fmt.Printf("  Model ID: %s\n", modelID)
	fmt.Println()
	fmt.Println("  Open the Playground at http://localhost:3000 to follow along!")
	waitForEnter()

	chapter2_DirectPermissions(ctx)
}

func chapter2_DirectPermissions(ctx context.Context) {
	printHeader("Chapter 2: Direct Permissions")
	fmt.Println("Concept: Users can be directly assigned relations on objects.")
	fmt.Println()

	writeTuples(ctx, "Setting up alice as owner of document:roadmap",
		fga.Tuple("user:alice", "owner", "document:roadmap"),
	)

	check(ctx, "Can alice view document:roadmap?", "user:alice", "viewer", "document:roadmap",
		"alice is owner → owner implies editor → editor implies viewer")

	check(ctx, "Can bob view document:roadmap?", "user:bob", "viewer", "document:roadmap",
		"bob has no relationship to this document")

	waitForEnter()
	chapter3_FolderInheritance(ctx)
}

func chapter3_FolderInheritance(ctx context.Context) {
	printHeader("Chapter 3: Folder Inheritance")
	fmt.Println("Concept: Documents inherit permissions from their parent folder.")
	fmt.Println("  Model rule: 'editor from parent' means if you're editor on the folder,")
	fmt.Println("  you're editor on any document inside it.")
	fmt.Println()

	writeTuples(ctx, "Placing document:roadmap under folder:product, making charlie editor of folder",
		fga.Tuple("folder:product", "parent", "document:roadmap"),
		fga.Tuple("user:charlie", "editor", "folder:product"),
	)

	check(ctx, "Can charlie edit document:roadmap?", "user:charlie", "editor", "document:roadmap",
		"charlie is editor on folder:product, document:roadmap's parent is folder:product, 'editor from parent' grants it")

	check(ctx, "Can charlie view document:roadmap?", "user:charlie", "viewer", "document:roadmap",
		"editor implies viewer")

	waitForEnter()
	chapter4_NestedFolders(ctx)
}

func chapter4_NestedFolders(ctx context.Context) {
	printHeader("Chapter 4: Nested Folders")
	fmt.Println("Concept: Permissions cascade through folder hierarchies.")
	fmt.Println("  folder:company → folder:product → document:roadmap")
	fmt.Println()

	writeTuples(ctx, "Creating folder hierarchy and granting diana viewer on company folder",
		fga.Tuple("user:alice", "owner", "folder:company"),
		fga.Tuple("folder:company", "parent", "folder:product"),
		fga.Tuple("user:diana", "viewer", "folder:company"),
	)

	check(ctx, "Can diana view folder:product?", "user:diana", "viewer", "folder:product",
		"diana is viewer on folder:company, folder:product's parent is folder:company, 'viewer from parent' cascades")

	check(ctx, "Can diana view document:roadmap?", "user:diana", "viewer", "document:roadmap",
		"two-hop inheritance: folder:company → folder:product → document:roadmap")

	check(ctx, "Can diana edit document:roadmap?", "user:diana", "editor", "document:roadmap",
		"diana only has viewer, not editor — viewer does NOT imply editor")

	waitForEnter()
	chapter5_Groups(ctx)
}

func chapter5_Groups(ctx context.Context) {
	printHeader("Chapter 5: Groups via Usersets")
	fmt.Println("Concept: Grant access to an entire group (organization members) at once.")
	fmt.Println("  Instead of 'user:eve', we write 'organization:acme#member'.")
	fmt.Println()

	writeTuples(ctx, "Creating org with members and granting org access to folder",
		fga.Tuple("user:eve", "member", "organization:acme"),
		fga.Tuple("user:frank", "member", "organization:acme"),
		fga.Tuple("organization:acme#member", "viewer", "folder:product"),
	)

	check(ctx, "Can eve view document:roadmap?", "user:eve", "viewer", "document:roadmap",
		"eve is member of organization:acme, acme#member has viewer on folder:product, document:roadmap is in folder:product")

	check(ctx, "Can frank view document:roadmap?", "user:frank", "viewer", "document:roadmap",
		"same path as eve — both are acme members")

	fmt.Println()
	fmt.Println("  Now adding grace to the organization (no new folder/document tuples needed):")
	writeTuples(ctx, "Adding grace as member of acme",
		fga.Tuple("user:grace", "member", "organization:acme"),
	)

	check(ctx, "Can grace view document:roadmap?", "user:grace", "viewer", "document:roadmap",
		"grace was added to acme — she automatically gets all of acme's permissions")

	waitForEnter()
	chapter6_PublicSharing(ctx)
}

func chapter6_PublicSharing(ctx context.Context) {
	printHeader("Chapter 6: Public Sharing (Wildcards)")
	fmt.Println("Concept: Grant access to everyone using the wildcard 'user:*'.")
	fmt.Println()

	writeTuples(ctx, "Creating a public document",
		fga.Tuple("user:alice", "owner", "document:public-memo"),
		fga.Tuple("user:*", "viewer", "document:public-memo"),
	)

	check(ctx, "Can a random user view document:public-memo?", "user:randomstranger", "viewer", "document:public-memo",
		"user:* grants viewer to ALL users — no specific tuple needed")

	check(ctx, "Can randomstranger edit document:public-memo?", "user:randomstranger", "editor", "document:public-memo",
		"wildcard only grants viewer, not editor — least privilege still applies")

	waitForEnter()
	chapter7_ReverseQueries(ctx)
}

func chapter7_ReverseQueries(ctx context.Context) {
	printHeader("Chapter 7: Reverse Queries (ListObjects)")
	fmt.Println("Concept: Instead of 'can user X access resource Y?', ask")
	fmt.Println("  'what resources can user X access?'")
	fmt.Println()

	listObjects(ctx, "What documents can diana view?", "user:diana", "viewer", "document")
	listObjects(ctx, "What folders can eve view?", "user:eve", "viewer", "folder")
	listObjects(ctx, "What documents can alice view?", "user:alice", "viewer", "document")

	waitForEnter()
	chapter8_Expand(ctx)
}

func chapter8_Expand(ctx context.Context) {
	printHeader("Chapter 8: Expand (Resolution Tree)")
	fmt.Println("Concept: See the full tree of WHY a permission is granted.")
	fmt.Println("  This is what OpenFGA resolves internally for every Check.")
	fmt.Println()

	fmt.Println("  Expanding: viewer on document:roadmap")
	tree, err := client.Expand(ctx, "viewer", "document:roadmap")
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Println("  Resolution tree:")
		printTree(tree, "    ")
	}

	fmt.Println()
	printHeader("Demo Complete!")
	fmt.Println("You've seen the core ReBAC concepts:")
	fmt.Println("  1. Direct permissions")
	fmt.Println("  2. Role implication (owner → editor → viewer)")
	fmt.Println("  3. Folder inheritance (permissions flow through parents)")
	fmt.Println("  4. Nested folder hierarchies")
	fmt.Println("  5. Group-based access via usersets")
	fmt.Println("  6. Public sharing via wildcards")
	fmt.Println("  7. Reverse queries with ListObjects")
	fmt.Println("  8. Resolution trees with Expand")
	fmt.Println()
	fmt.Println("Next: try the HTTP server!")
	fmt.Println("  make serve   # start the API")
	fmt.Println("  make seed    # populate demo data")
	fmt.Println("  curl -H 'X-User-Id: alice' http://localhost:8000/documents")
}

func printHeader(title string) {
	line := strings.Repeat("─", 60)
	fmt.Printf("\n── %s %s\n\n", title, line[:60-len(title)-4])
}

func writeTuples(ctx context.Context, description string, tuples ...openfga.TupleKey) {
	fmt.Printf("  %s\n", description)
	fmt.Println("  Writing tuples:")
	for _, t := range tuples {
		fmt.Printf("    + %s#%s@%s\n", t.Object, t.Relation, t.User)
	}

	if err := client.WriteTuples(ctx, tuples); err != nil {
		fmt.Printf("  ERROR writing tuples: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
}

func check(ctx context.Context, question, user, relation, object, because string) {
	fmt.Printf("  Q: %s\n", question)
	allowed, err := client.Check(ctx, user, relation, object)
	if err != nil {
		fmt.Printf("     ERROR: %v\n", err)
		return
	}

	symbol := "✗ DENIED"
	if allowed {
		symbol = "✓ ALLOWED"
	}
	fmt.Printf("     Check(%s, %s, %s) → %s\n", user, relation, object, symbol)
	fmt.Printf("     Because: %s\n\n", because)
}

func listObjects(ctx context.Context, question, user, relation, objectType string) {
	fmt.Printf("  Q: %s\n", question)
	objects, err := client.ListObjects(ctx, user, relation, objectType)
	if err != nil {
		fmt.Printf("     ERROR: %v\n", err)
		return
	}

	fmt.Printf("     ListObjects(%s, %s, %s) →\n", user, relation, objectType)
	if len(objects) == 0 {
		fmt.Println("     (none)")
	}
	for _, obj := range objects {
		fmt.Printf("       • %s\n", obj)
	}
	fmt.Println()
}

func printTree(tree *openfga.UsersetTree, indent string) {
	if tree == nil {
		fmt.Printf("%s(nil)\n", indent)
		return
	}
	root := tree.GetRoot()
	if leaf := root.GetLeaf(); leaf != nil {
		if users := leaf.GetUsers(); users != nil {
			for _, u := range users.GetUsers() {
				fmt.Printf("%s└─ user: %s\n", indent, u)
			}
		}
		if computed := leaf.GetComputed(); computed != nil {
			fmt.Printf("%s└─ computed: %s\n", indent, computed.GetUserset())
		}
		if tupleToUserset := leaf.GetTupleToUserset(); tupleToUserset != nil {
			fmt.Printf("%s└─ tupleToUserset: %s\n", indent, tupleToUserset.GetTupleset())
			for _, c := range tupleToUserset.GetComputed() {
				fmt.Printf("%s  └─ %s\n", indent, c.GetUserset())
			}
		}
	}
	if union := root.GetUnion(); union != nil {
		fmt.Printf("%s└─ union:\n", indent)
		for _, child := range union.GetNodes() {
			childTree := openfga.UsersetTree{Root: &child}
			printTree(&childTree, indent+"  ")
		}
	}
	if intersection := root.GetIntersection(); intersection != nil {
		fmt.Printf("%s└─ intersection:\n", indent)
		for _, child := range intersection.GetNodes() {
			childTree := openfga.UsersetTree{Root: &child}
			printTree(&childTree, indent+"  ")
		}
	}
	if difference := root.GetDifference(); difference != nil {
		fmt.Printf("%s└─ difference:\n", indent)
		if base := difference.GetBase(); base != nil {
			baseTree := openfga.UsersetTree{Root: base}
			printTree(&baseTree, indent+"  base: ")
		}
		if subtract := difference.GetSubtract(); subtract != nil {
			subTree := openfga.UsersetTree{Root: subtract}
			printTree(&subTree, indent+"  subtract: ")
		}
	}
}

func waitForEnter() {
	if !pause {
		return
	}
	fmt.Print("\n  Press Enter to continue...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 2: Verify build**

Run:
```bash
go build ./cmd/cli
```
Expected: compiles without error.

- [ ] **Step 3: Commit**

```bash
git add cmd/cli/
git commit -m "feat: add CLI teaching walkthrough with 8 chapters"
```

---

### Task 8: Demo script and documentation

**Files:**
- Create: `scripts/demo.sh`
- Create: `docs/model-explained.md`
- Create: `docs/architecture.md`
- Modify: `README.md`

- [ ] **Step 1: Create `scripts/demo.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${SERVER_ADDR:-http://localhost:8000}"

echo "=== OpenFGA Demo: HTTP API Walkthrough ==="
echo ""

echo "--- Seeding demo data ---"
go run ./cmd/server -seed -exit
echo ""

echo "--- Starting server in background ---"
go run ./cmd/server &
SERVER_PID=$!
sleep 2

cleanup() {
    echo ""
    echo "--- Stopping server ---"
    kill $SERVER_PID 2>/dev/null || true
}
trap cleanup EXIT

echo "--- As alice: list my documents ---"
curl -s -H "X-User-Id: alice" "$BASE_URL/documents" | jq .
echo ""

echo "--- As bob: try to view roadmap (should be 403) ---"
curl -s -w "\nHTTP %{http_code}\n" -H "X-User-Id: bob" "$BASE_URL/documents/roadmap"
echo ""

echo "--- As charlie: view roadmap (editor on folder:product) ---"
curl -s -H "X-User-Id: charlie" "$BASE_URL/documents/roadmap" | jq .
echo ""

echo "--- As diana: list documents (viewer on folder:company → sees everything) ---"
curl -s -H "X-User-Id: diana" "$BASE_URL/documents" | jq .
echo ""

echo "--- As eve: list documents (acme member → viewer on folder:product) ---"
curl -s -H "X-User-Id: eve" "$BASE_URL/documents" | jq .
echo ""

echo "--- As randomstranger: view public-memo (wildcard) ---"
curl -s -H "X-User-Id: randomstranger" "$BASE_URL/documents/public-memo" | jq .
echo ""

echo "--- As alice: create a new folder ---"
curl -s -X POST -H "X-User-Id: alice" -H "Content-Type: application/json" \
  -d '{"id":"engineering","name":"Engineering","parent_id":"company"}' \
  "$BASE_URL/folders" | jq .
echo ""

echo "--- As alice: create a new document ---"
curl -s -X POST -H "X-User-Id: alice" -H "Content-Type: application/json" \
  -d '{"id":"design-doc","title":"System Design","content":"Architecture overview","folder_id":"engineering"}' \
  "$BASE_URL/documents" | jq .
echo ""

echo "--- As alice: share design-doc with bob as editor ---"
curl -s -X POST -H "X-User-Id: alice" -H "Content-Type: application/json" \
  -d '{"user":"user:bob","relation":"editor"}' \
  "$BASE_URL/documents/design-doc/share" | jq .
echo ""

echo "--- As bob: edit the shared document ---"
curl -s -X PUT -H "X-User-Id: bob" -H "Content-Type: application/json" \
  -d '{"title":"System Design v2","content":"Updated architecture"}' \
  "$BASE_URL/documents/design-doc" | jq .
echo ""

echo "=== Demo complete! ==="
```

- [ ] **Step 2: Make demo script executable**

Run:
```bash
chmod +x scripts/demo.sh
```

- [ ] **Step 3: Create `docs/model-explained.md`**

```markdown
# Authorization Model — Explained

This document walks through the OpenFGA authorization model in `models/basic/authorization-model-basic.fga`.

## Schema

```
model
  schema 1.1
```

OpenFGA schema version 1.1 enables conditional relationships and type restrictions.

## Types

### `user`

The base type representing people. Has no relations of its own — users exist as subjects of other types' relations.

### `organization`

```
type organization
  relations
    define member: [user]
```

Organizations group users. The `member` relation accepts direct `user` assignments. This becomes powerful when used as a **userset**: `organization:acme#member` refers to "all members of acme."

### `folder`

```
type folder
  relations
    define owner: [user]
    define parent: [folder]
    define editor: [user, organization#member] or owner or editor from parent
    define viewer: [user, user:*, organization#member] or editor or viewer from parent
```

Key concepts:

- **`parent: [folder]`** — a folder can have a parent folder. This creates the hierarchy.
- **`editor: ... or owner`** — role implication. Owners are automatically editors.
- **`editor: ... or editor from parent`** — TupleToUserset. If you're an editor on the parent folder, you're an editor on this folder too. This is how permissions cascade.
- **`[user, organization#member]`** — type restrictions. Both direct users and organization member groups can be assigned.
- **`viewer: ... or editor`** — editors are automatically viewers. Combined with owner→editor, this gives us: owner → editor → viewer.
- **`user:*`** — wildcard. Allows granting viewer to all users at once (public sharing).

### `document`

```
type document
  relations
    define parent: [folder]
    define owner: [user]
    define editor: [user, organization#member] or owner or editor from parent
    define viewer: [user, user:*, organization#member] or editor or viewer from parent
```

Same pattern as folder. A document's `parent` is a folder, enabling permissions to cascade from folders to documents.

## Permission Resolution Example

Q: Can `user:eve` view `document:roadmap`?

Resolution path:
1. Is eve a direct viewer of document:roadmap? No.
2. Is eve an editor of document:roadmap? (editor implies viewer)
   - Is eve a direct editor? No.
   - Is eve an owner? No.
   - Is eve an editor via parent? document:roadmap's parent is folder:product.
     - Is eve an editor of folder:product? No direct editor, not owner, no parent editor.
3. Is eve a viewer via parent? document:roadmap's parent is folder:product.
   - Is eve a viewer of folder:product?
     - Is organization:acme#member a viewer of folder:product? **Yes!**
     - Is eve a member of organization:acme? **Yes!**
4. **Result: ALLOWED** — eve is a member of acme, acme#member is a viewer of folder:product, document:roadmap's parent is folder:product.
```

- [ ] **Step 4: Create `docs/architecture.md`**

```markdown
# Architecture

```
┌─────────────────────────────────────────────────┐
│  Docker Compose                                 │
│  ┌──────────┐   ┌─────────────┐   ┌──────────┐ │
│  │ MariaDB  │◄──│  OpenFGA    │──►│Playground│ │
│  │  :3306   │   │  :8080      │   │  :3000   │ │
│  └──────────┘   └─────────────┘   └──────────┘ │
└─────────────────────────────────────────────────┘
              ▲                  ▲
              │                  │
       ┌──────┴──────┐    ┌──────┴──────┐
       │  cmd/cli    │    │ cmd/server  │
       │ (teaching   │    │ (REST API   │
       │  walkthrough)│   │  using FGA) │
       └─────────────┘    └─────────────┘
              │                  │
              └── internal/fga ──┘
```

## Components

- **MariaDB** — Persistent datastore for OpenFGA. Stores authorization model, tuples, and change history.
- **OpenFGA Server** — The authorization engine. Exposes HTTP API on `:8080`. Handles Check, ListObjects, Expand, and Write operations.
- **OpenFGA Playground** — Web UI on `:3000`. Lets you visually explore the model, browse tuples, and run queries interactively.
- **cmd/cli** — Scripted teaching demo. Runs 8 chapters that progressively introduce ReBAC concepts.
- **cmd/server** — REST API that authorizes every request through OpenFGA. Shows how to integrate FGA into a real service.
- **internal/fga** — Shared OpenFGA client wrapper used by both binaries.
- **internal/store** — In-memory application data (documents, folders, organizations).
- **internal/auth** — Middleware (X-User-Id extraction) and authorization helper.
- **internal/httpapi** — HTTP handlers and error types.
```

- [ ] **Step 5: Update `README.md`**

```markdown
# OpenFGA ReBAC Demo

A teaching-oriented demo showing how to use [OpenFGA](https://openfga.dev) as a Relationship-Based Access Control (ReBAC) backend. Models a Google Drive-style permission system with folders, documents, organizations, and role inheritance.

## What You'll Learn

- **Direct permissions** — assigning users to resources
- **Role implication** — owner → editor → viewer
- **Folder inheritance** — permissions cascade from folders to documents
- **Nested hierarchies** — multi-level folder permission inheritance
- **Group access (usersets)** — grant permissions to all members of an organization
- **Public sharing (wildcards)** — `user:*` for open access
- **Reverse queries** — "what can this user access?" via ListObjects
- **Resolution trees** — visualize how permissions are computed via Expand

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- [curl](https://curl.se/) and [jq](https://jqlang.github.io/jq/) (for the demo script)

## Quick Start

### 1. Start the infrastructure

```bash
make up
```

This starts MariaDB, OpenFGA server, and the Playground UI.

### 2. Run the CLI walkthrough

```bash
make cli
```

Follow along in the [OpenFGA Playground](http://localhost:3000) to see tuples and test queries visually.

### 3. Try the HTTP API

```bash
make seed    # populate demo data
make serve   # start the API server on :8000
```

Then in another terminal:

```bash
# As alice (owner): list documents
curl -s -H "X-User-Id: alice" http://localhost:8000/documents | jq .

# As bob (no access): try to view a document
curl -s -H "X-User-Id: bob" http://localhost:8000/documents/roadmap

# As charlie (editor via folder): view a document
curl -s -H "X-User-Id: charlie" http://localhost:8000/documents/roadmap | jq .

# As randomstranger: view a public document
curl -s -H "X-User-Id: randomstranger" http://localhost:8000/documents/public-memo | jq .
```

Or run the full demo script:

```bash
make demo
```

### 4. Tear down

```bash
make down
```

## Project Structure

```
├── docker-compose.yml           # MariaDB + OpenFGA + Playground
├── models/basic/authorization-model-basic.fga # The authorization model (single source of truth)
├── cmd/cli/                     # CLI teaching walkthrough (8 chapters)
├── cmd/server/                  # REST API with FGA authorization
├── internal/fga/                # OpenFGA client wrapper
├── internal/store/              # In-memory document/folder/org storage
├── internal/auth/               # Auth middleware and helpers
├── internal/httpapi/            # HTTP handlers and error types
├── scripts/demo.sh              # Automated curl walkthrough
└── docs/                        # Model explanation and architecture
```

## Further Reading

- [docs/model-explained.md](docs/model-explained.md) — Annotated walkthrough of the authorization model
- [docs/architecture.md](docs/architecture.md) — System architecture diagram
- [OpenFGA Documentation](https://openfga.dev/docs)
- [OpenFGA Playground](https://play.fga.dev) — Online model playground
```

- [ ] **Step 6: Commit**

```bash
git add scripts/ docs/model-explained.md docs/architecture.md README.md
git commit -m "docs: add demo script, model explanation, architecture, and README"
```

---

### Task 9: Integration smoke test and final polish

**Files:**
- Modify: `Makefile` (already created in Task 1)
- Verify all builds and unit tests pass

- [ ] **Step 1: Run `go mod tidy`**

Run:
```bash
go mod tidy
```

- [ ] **Step 2: Run all unit tests**

Run:
```bash
go test ./... -v
```
Expected: all tests pass.

- [ ] **Step 3: Run `go vet`**

Run:
```bash
go vet ./...
```
Expected: no errors.

- [ ] **Step 4: Build both binaries**

Run:
```bash
go build ./cmd/cli && go build ./cmd/server
```
Expected: both compile without error.

- [ ] **Step 5: Integration smoke test (manual)**

Run:
```bash
make up
make cli
```
Expected: all 8 chapters run to completion, each Check prints ALLOWED or DENIED as documented.

Run:
```bash
make seed
make serve &
curl -s -H "X-User-Id: alice" http://localhost:8000/documents | jq .
curl -s -H "X-User-Id: bob" http://localhost:8000/documents/roadmap
kill %1
make down
```
Expected: alice gets documents, bob gets 403, server starts and stops cleanly.

- [ ] **Step 6: Commit any final fixes**

```bash
git add -A
git commit -m "chore: final integration polish and go mod tidy"
```

---

## Implementation Notes

**OpenFGA DSL transformer:** The `bootstrap.go` file uses `github.com/openfga/language/pkg/go/transformer` to convert the `.fga` DSL file to the JSON format required by `WriteAuthorizationModel`. If this package's import path has changed, check `pkg.go.dev` for the current path — it's part of the [openfga/language](https://github.com/openfga/language) repo.

**OpenFGA SDK version:** The code uses the `github.com/openfga/go-sdk` client SDK. The SDK's method signatures may vary between versions. If `client.ClientCheckRequest` or similar types have changed, consult the [SDK docs](https://pkg.go.dev/github.com/openfga/go-sdk). The patterns (Check, Write, ListObjects, Expand) remain the same.

**Expand tree printing:** The `printTree` function in the CLI is a best-effort recursive printer. The `UsersetTree` structure from the SDK can be deeply nested. If the output is garbled for complex resolutions, simplify by printing just the top-level nodes.

**chi URLParam naming:** The handlers use `chi.URLParam(r, "id")`. Make sure route definitions use `{id}` (chi v5 syntax), not `:id`.

**auth.Checker interface:** The `auth.Authorize` helper accepts a `Checker` interface with just `Check(ctx, user, relation, object) (bool, error)`. The `httpapi.FGAClient` interface extends this with `ListObjects`, `WriteTuples`, and `DeleteTuples`. The `fgaAdapter` in `cmd/server/main.go` bridges the concrete `fga.Client` to the `httpapi.FGAClient` interface.
