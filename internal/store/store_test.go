package store_test

import (
	"testing"

	"github.com/terenzio/OpenFGADemo/internal/store"
)

func newTestStore() *store.Store {
	return store.New()
}

// TestCreateAndGetDocument verifies that a document can be stored and retrieved by ID.
func TestCreateAndGetDocument(t *testing.T) {
	s := newTestStore()

	doc := store.Document{
		ID:       "doc-1",
		Title:    "Hello World",
		Content:  "Some content",
		FolderID: "folder-1",
		OwnerID:  "user-1",
	}
	s.CreateDocument(doc)

	got, ok := s.GetDocument("doc-1")
	if !ok {
		t.Fatal("expected document to be found, got not found")
	}
	if got.ID != doc.ID {
		t.Errorf("ID: got %q, want %q", got.ID, doc.ID)
	}
	if got.Title != doc.Title {
		t.Errorf("Title: got %q, want %q", got.Title, doc.Title)
	}
	if got.Content != doc.Content {
		t.Errorf("Content: got %q, want %q", got.Content, doc.Content)
	}
	if got.FolderID != doc.FolderID {
		t.Errorf("FolderID: got %q, want %q", got.FolderID, doc.FolderID)
	}
	if got.OwnerID != doc.OwnerID {
		t.Errorf("OwnerID: got %q, want %q", got.OwnerID, doc.OwnerID)
	}
}

// TestGetDocument_NotFound verifies that looking up a missing document returns false.
func TestGetDocument_NotFound(t *testing.T) {
	s := newTestStore()

	_, ok := s.GetDocument("nonexistent")
	if ok {
		t.Fatal("expected document not to be found, but it was")
	}
}

// TestUpdateDocument verifies that title and content can be updated and are reflected on retrieval.
func TestUpdateDocument(t *testing.T) {
	s := newTestStore()

	doc := store.Document{
		ID:      "doc-2",
		Title:   "Original Title",
		Content: "Original content",
		OwnerID: "user-1",
	}
	s.CreateDocument(doc)

	updated := s.UpdateDocument("doc-2", "New Title", "New content")
	if !updated {
		t.Fatal("expected UpdateDocument to return true for existing document")
	}

	got, ok := s.GetDocument("doc-2")
	if !ok {
		t.Fatal("document not found after update")
	}
	if got.Title != "New Title" {
		t.Errorf("Title: got %q, want %q", got.Title, "New Title")
	}
	if got.Content != "New content" {
		t.Errorf("Content: got %q, want %q", got.Content, "New content")
	}

	// Other fields should remain unchanged.
	if got.OwnerID != "user-1" {
		t.Errorf("OwnerID should be unchanged: got %q", got.OwnerID)
	}
}

// TestUpdateDocument_NotFound verifies that updating a non-existent document returns false.
func TestUpdateDocument_NotFound(t *testing.T) {
	s := newTestStore()

	ok := s.UpdateDocument("ghost", "Title", "Content")
	if ok {
		t.Fatal("expected UpdateDocument to return false for missing document")
	}
}

// TestDeleteDocument verifies that a document is no longer retrievable after deletion.
func TestDeleteDocument(t *testing.T) {
	s := newTestStore()

	doc := store.Document{ID: "doc-3", Title: "To Delete", OwnerID: "user-1"}
	s.CreateDocument(doc)

	s.DeleteDocument("doc-3")

	_, ok := s.GetDocument("doc-3")
	if ok {
		t.Fatal("expected document to be deleted, but it was still found")
	}

	// Deleting a non-existent document should not panic.
	s.DeleteDocument("doc-3")
}

// TestCreateAndGetFolder verifies that a folder can be stored and retrieved by ID.
func TestCreateAndGetFolder(t *testing.T) {
	s := newTestStore()

	folder := store.Folder{
		ID:       "folder-1",
		Name:     "My Folder",
		ParentID: "",
		OwnerID:  "user-1",
	}
	s.CreateFolder(folder)

	got, ok := s.GetFolder("folder-1")
	if !ok {
		t.Fatal("expected folder to be found, got not found")
	}
	if got.ID != folder.ID {
		t.Errorf("ID: got %q, want %q", got.ID, folder.ID)
	}
	if got.Name != folder.Name {
		t.Errorf("Name: got %q, want %q", got.Name, folder.Name)
	}
	if got.OwnerID != folder.OwnerID {
		t.Errorf("OwnerID: got %q, want %q", got.OwnerID, folder.OwnerID)
	}
}

// TestListDocuments verifies that ListDocumentsByIDs returns exactly the requested documents.
func TestListDocuments(t *testing.T) {
	s := newTestStore()

	docs := []store.Document{
		{ID: "d1", Title: "Doc 1", OwnerID: "user-1"},
		{ID: "d2", Title: "Doc 2", OwnerID: "user-1"},
		{ID: "d3", Title: "Doc 3", OwnerID: "user-2"},
	}
	for _, d := range docs {
		s.CreateDocument(d)
	}

	result := s.ListDocumentsByIDs([]string{"d1", "d3"})
	if len(result) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(result))
	}

	ids := map[string]bool{}
	for _, d := range result {
		ids[d.ID] = true
	}
	if !ids["d1"] || !ids["d3"] {
		t.Errorf("expected d1 and d3 in results, got %v", ids)
	}

	// Requesting a non-existent ID should not cause an error; it is simply omitted.
	result2 := s.ListDocumentsByIDs([]string{"d1", "nonexistent"})
	if len(result2) != 1 {
		t.Fatalf("expected 1 document, got %d", len(result2))
	}
}

// TestListFolders verifies that ListFoldersByIDs returns exactly the requested folders.
func TestListFolders(t *testing.T) {
	s := newTestStore()

	folders := []store.Folder{
		{ID: "f1", Name: "Folder 1", OwnerID: "user-1"},
		{ID: "f2", Name: "Folder 2", OwnerID: "user-1"},
		{ID: "f3", Name: "Folder 3", OwnerID: "user-2"},
	}
	for _, f := range folders {
		s.CreateFolder(f)
	}

	result := s.ListFoldersByIDs([]string{"f2", "f3"})
	if len(result) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(result))
	}

	ids := map[string]bool{}
	for _, f := range result {
		ids[f.ID] = true
	}
	if !ids["f2"] || !ids["f3"] {
		t.Errorf("expected f2 and f3 in results, got %v", ids)
	}

	// Non-existent IDs should be silently skipped.
	result2 := s.ListFoldersByIDs([]string{"f1", "missing"})
	if len(result2) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(result2))
	}
}

// TestCreateAndGetOrganization verifies that an organization can be stored and retrieved.
func TestCreateAndGetOrganization(t *testing.T) {
	s := newTestStore()

	org := store.Organization{
		ID:      "org-1",
		Name:    "Acme Corp",
		OwnerID: "user-1",
	}
	s.CreateOrganization(org)

	got, ok := s.GetOrganization("org-1")
	if !ok {
		t.Fatal("expected organization to be found, got not found")
	}
	if got.ID != org.ID {
		t.Errorf("ID: got %q, want %q", got.ID, org.ID)
	}
	if got.Name != org.Name {
		t.Errorf("Name: got %q, want %q", got.Name, org.Name)
	}
	if got.OwnerID != org.OwnerID {
		t.Errorf("OwnerID: got %q, want %q", got.OwnerID, org.OwnerID)
	}

	_, ok = s.GetOrganization("nonexistent")
	if ok {
		t.Fatal("expected nonexistent org not to be found")
	}
}
