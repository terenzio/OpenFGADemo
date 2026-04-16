// Package store provides a thread-safe in-memory data store for documents,
// folders, and organizations used by the OpenFGA demo application.
package store

import "sync"

// Document represents a user-owned document that lives inside a folder.
type Document struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	FolderID string `json:"folder_id"`
	OwnerID  string `json:"owner_id"`
}

// Folder represents a container for documents.
type Folder struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parent_id,omitempty"`
	OwnerID  string `json:"owner_id"`
}

// Organization is the top-level tenant entity.
type Organization struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OwnerID string `json:"owner_id"`
}

// Store is a thread-safe in-memory store for application state.
type Store struct {
	mu   sync.RWMutex
	docs map[string]Document
	dirs map[string]Folder
	orgs map[string]Organization
}

// New returns an initialised, empty Store.
func New() *Store {
	return &Store{
		docs: make(map[string]Document),
		dirs: make(map[string]Folder),
		orgs: make(map[string]Organization),
	}
}

// --- Document methods ---

// CreateDocument adds or replaces a document in the store.
func (s *Store) CreateDocument(d Document) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[d.ID] = d
}

// GetDocument retrieves a document by ID. The second return value is false when
// the document does not exist.
func (s *Store) GetDocument(id string) (Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.docs[id]
	return d, ok
}

// UpdateDocument changes the title and content of an existing document.
// It returns true when the document was found and updated, false otherwise.
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

// DeleteDocument removes a document from the store. It is a no-op when the
// document does not exist.
func (s *Store) DeleteDocument(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, id)
}

// ListDocumentsByIDs returns the documents that match the given IDs. IDs that
// do not correspond to any stored document are silently skipped.
func (s *Store) ListDocumentsByIDs(ids []string) []Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Document, 0, len(ids))
	for _, id := range ids {
		if d, ok := s.docs[id]; ok {
			result = append(result, d)
		}
	}
	return result
}

// --- Folder methods ---

// CreateFolder adds or replaces a folder in the store.
func (s *Store) CreateFolder(f Folder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirs[f.ID] = f
}

// GetFolder retrieves a folder by ID.
func (s *Store) GetFolder(id string) (Folder, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.dirs[id]
	return f, ok
}

// ListFoldersByIDs returns the folders that match the given IDs. Missing IDs
// are silently skipped.
func (s *Store) ListFoldersByIDs(ids []string) []Folder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Folder, 0, len(ids))
	for _, id := range ids {
		if f, ok := s.dirs[id]; ok {
			result = append(result, f)
		}
	}
	return result
}

// --- Organization methods ---

// CreateOrganization adds or replaces an organization in the store.
func (s *Store) CreateOrganization(o Organization) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orgs[o.ID] = o
}

// GetOrganization retrieves an organization by ID.
func (s *Store) GetOrganization(id string) (Organization, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orgs[id]
	return o, ok
}
