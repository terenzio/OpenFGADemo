package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/terenzio/OpenFGADemo/internal/auth"
	"github.com/terenzio/OpenFGADemo/internal/store"
)

// fakeFGA implements FGAClient for testing.
type fakeFGA struct {
	allowed     bool
	listObjects []string
	writtenTuples []TupleWrite
	deletedTuples []TupleWrite
}

func (f *fakeFGA) Check(_ context.Context, _, _, _ string) (bool, error) {
	return f.allowed, nil
}

func (f *fakeFGA) ListObjects(_ context.Context, _, _, _ string) ([]string, error) {
	return f.listObjects, nil
}

func (f *fakeFGA) WriteTuples(_ context.Context, tuples []TupleWrite) error {
	f.writtenTuples = append(f.writtenTuples, tuples...)
	return nil
}

func (f *fakeFGA) DeleteTuples(_ context.Context, tuples []TupleWrite) error {
	f.deletedTuples = append(f.deletedTuples, tuples...)
	return nil
}

// withUser adds a user ID to the request context.
func withUser(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), auth.UserIDKey, userID)
	return r.WithContext(ctx)
}

// withChiParam adds a chi URL parameter to the request context.
func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

func TestGetDocument_Allowed(t *testing.T) {
	s := store.New()
	s.CreateDocument(store.Document{
		ID:       "doc1",
		Title:    "Test Doc",
		Content:  "Hello",
		FolderID: "folder1",
		OwnerID:  "alice",
	})

	fga := &fakeFGA{allowed: true}
	h := NewHandler(s, fga)

	r := httptest.NewRequest(http.MethodGet, "/documents/doc1", nil)
	r = withUser(r, "alice")
	r = withChiParam(r, "id", "doc1")

	w := httptest.NewRecorder()
	h.GetDocument(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var doc store.Document
	if err := json.NewDecoder(w.Body).Decode(&doc); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if doc.ID != "doc1" {
		t.Errorf("expected doc ID doc1, got %s", doc.ID)
	}
	if doc.Title != "Test Doc" {
		t.Errorf("expected title 'Test Doc', got %s", doc.Title)
	}
}

func TestGetDocument_Forbidden(t *testing.T) {
	s := store.New()
	s.CreateDocument(store.Document{
		ID:       "doc1",
		Title:    "Test Doc",
		Content:  "Hello",
		FolderID: "folder1",
		OwnerID:  "alice",
	})

	fga := &fakeFGA{allowed: false}
	h := NewHandler(s, fga)

	r := httptest.NewRequest(http.MethodGet, "/documents/doc1", nil)
	r = withUser(r, "bob")
	r = withChiParam(r, "id", "doc1")

	w := httptest.NewRecorder()
	h.GetDocument(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error != "forbidden" {
		t.Errorf("expected error 'forbidden', got %s", resp.Error)
	}
}

func TestListDocuments(t *testing.T) {
	s := store.New()
	s.CreateDocument(store.Document{ID: "doc1", Title: "Doc 1", FolderID: "f1", OwnerID: "alice"})
	s.CreateDocument(store.Document{ID: "doc2", Title: "Doc 2", FolderID: "f1", OwnerID: "alice"})

	fga := &fakeFGA{listObjects: []string{"document:doc1", "document:doc2"}}
	h := NewHandler(s, fga)

	r := httptest.NewRequest(http.MethodGet, "/documents", nil)
	r = withUser(r, "alice")

	w := httptest.NewRecorder()
	h.ListDocuments(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var docs []store.Document
	if err := json.NewDecoder(w.Body).Decode(&docs); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}
}

func TestCreateDocument_Forbidden(t *testing.T) {
	s := store.New()
	s.CreateFolder(store.Folder{ID: "folder1", Name: "Test Folder", OwnerID: "alice"})

	fga := &fakeFGA{allowed: false}
	h := NewHandler(s, fga)

	body := `{"id":"doc1","title":"New Doc","content":"body","folder_id":"folder1"}`
	r := httptest.NewRequest(http.MethodPost, "/documents", bytes.NewBufferString(body))
	r = withUser(r, "bob")

	w := httptest.NewRecorder()
	h.CreateDocument(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
