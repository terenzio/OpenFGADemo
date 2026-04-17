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

// FGAClient is the interface used by handlers to interact with OpenFGA.
type FGAClient interface {
	Check(ctx context.Context, user, relation, object string) (bool, error)
	ListObjects(ctx context.Context, user, relation, objectType string) ([]string, error)
	WriteTuples(ctx context.Context, tuples []TupleWrite) error
	DeleteTuples(ctx context.Context, tuples []TupleWrite) error
}

// TupleWrite represents a single FGA relationship tuple to write or delete.
type TupleWrite struct {
	User     string
	Relation string
	Object   string
}

// Handler holds the dependencies for all HTTP handlers.
type Handler struct {
	store *store.Store
	fga   FGAClient
}

// NewHandler creates a new Handler with the given store and FGA client.
func NewHandler(s *store.Store, fga FGAClient) *Handler {
	return &Handler{store: s, fga: fga}
}

// Routes returns a chi.Router with all application routes registered.
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

// AddOrgMember adds a member to an organization. Requires owner on org.
func (h *Handler) AddOrgMember(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")

	if err := auth.Authorize(r.Context(), h.fga, "owner", "organization", orgID); err != nil {
		WriteError(w, err)
		return
	}

	var body struct {
		User     string `json:"user"`
		Relation string `json:"relation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	err := h.fga.WriteTuples(r.Context(), []TupleWrite{
		{User: fmt.Sprintf("user:%s", body.User), Relation: body.Relation, Object: fmt.Sprintf("organization:%s", orgID)},
	})
	if err != nil {
		slog.Error("failed to write org member tuple", "error", err)
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// CreateFolder creates a new folder. The creator becomes the owner.
func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFromContext(r.Context())

	var body struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		ParentID string `json:"parent_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	folder := store.Folder{
		ID:       body.ID,
		Name:     body.Name,
		ParentID: body.ParentID,
		OwnerID:  userID,
	}
	h.store.CreateFolder(folder)

	tuples := []TupleWrite{
		{User: fmt.Sprintf("user:%s", userID), Relation: "owner", Object: fmt.Sprintf("folder:%s", body.ID)},
	}
	if body.ParentID != "" {
		tuples = append(tuples, TupleWrite{
			User: fmt.Sprintf("folder:%s", body.ParentID), Relation: "parent", Object: fmt.Sprintf("folder:%s", body.ID),
		})
	}

	if err := h.fga.WriteTuples(r.Context(), tuples); err != nil {
		slog.Error("failed to write folder tuples, rolling back", "error", err)
		h.store.DeleteFolder(body.ID) // rollback
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, folder)
}

// ListFolders lists folders the user can view via FGA.
func (h *Handler) ListFolders(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFromContext(r.Context())

	objects, err := h.fga.ListObjects(r.Context(), fmt.Sprintf("user:%s", userID), "viewer", "folder")
	if err != nil {
		slog.Error("failed to list folder objects", "error", err)
		WriteError(w, err)
		return
	}

	ids := stripTypePrefix(objects)
	folders := h.store.ListFoldersByIDs(ids)
	WriteJSON(w, http.StatusOK, folders)
}

// CreateDocument creates a new document. Requires editor on parent folder.
func (h *Handler) CreateDocument(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFromContext(r.Context())

	var body struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		FolderID string `json:"folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if err := auth.Authorize(r.Context(), h.fga, "editor", "folder", body.FolderID); err != nil {
		WriteError(w, err)
		return
	}

	doc := store.Document{
		ID:       body.ID,
		Title:    body.Title,
		Content:  body.Content,
		FolderID: body.FolderID,
		OwnerID:  userID,
	}
	h.store.CreateDocument(doc)

	tuples := []TupleWrite{
		{User: fmt.Sprintf("user:%s", userID), Relation: "owner", Object: fmt.Sprintf("document:%s", body.ID)},
		{User: fmt.Sprintf("folder:%s", body.FolderID), Relation: "parent", Object: fmt.Sprintf("document:%s", body.ID)},
	}

	if err := h.fga.WriteTuples(r.Context(), tuples); err != nil {
		slog.Error("failed to write document tuples, rolling back", "error", err)
		h.store.DeleteDocument(body.ID)
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, doc)
}

// ListDocuments lists documents the user can view via FGA.
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFromContext(r.Context())

	objects, err := h.fga.ListObjects(r.Context(), fmt.Sprintf("user:%s", userID), "viewer", "document")
	if err != nil {
		slog.Error("failed to list document objects", "error", err)
		WriteError(w, err)
		return
	}

	ids := stripTypePrefix(objects)
	docs := h.store.ListDocumentsByIDs(ids)
	WriteJSON(w, http.StatusOK, docs)
}

// GetDocument retrieves a single document. Uses an explicit inline FGA check
// (for teaching purposes, rather than using auth.Authorize).
func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserFromContext(r.Context())
	docID := chi.URLParam(r, "id")

	doc, ok := h.store.GetDocument(docID)
	if !ok {
		WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "document not found"})
		return
	}

	// Explicit inline FGA check (teaching form).
	allowed, err := h.fga.Check(r.Context(), fmt.Sprintf("user:%s", userID), "viewer", fmt.Sprintf("document:%s", docID))
	if err != nil {
		slog.Error("fga check failed", "error", err)
		WriteError(w, err)
		return
	}
	if !allowed {
		WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden"})
		return
	}

	WriteJSON(w, http.StatusOK, doc)
}

// UpdateDocument updates an existing document. Requires editor on doc.
func (h *Handler) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")

	if err := auth.Authorize(r.Context(), h.fga, "editor", "document", docID); err != nil {
		WriteError(w, err)
		return
	}

	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if ok := h.store.UpdateDocument(docID, body.Title, body.Content); !ok {
		WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "document not found"})
		return
	}

	doc, _ := h.store.GetDocument(docID)
	WriteJSON(w, http.StatusOK, doc)
}

// ShareDocument shares a document with another user. Requires owner on doc.
func (h *Handler) ShareDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")

	if err := auth.Authorize(r.Context(), h.fga, "owner", "document", docID); err != nil {
		WriteError(w, err)
		return
	}

	var body struct {
		User     string `json:"user"`
		Relation string `json:"relation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	err := h.fga.WriteTuples(r.Context(), []TupleWrite{
		{User: body.User, Relation: body.Relation, Object: fmt.Sprintf("document:%s", docID)},
	})
	if err != nil {
		slog.Error("failed to write share tuple", "error", err)
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// UnshareDocument removes a share from a document. Requires owner on doc.
func (h *Handler) UnshareDocument(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")

	if err := auth.Authorize(r.Context(), h.fga, "owner", "document", docID); err != nil {
		WriteError(w, err)
		return
	}

	var body struct {
		User     string `json:"user"`
		Relation string `json:"relation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	err := h.fga.DeleteTuples(r.Context(), []TupleWrite{
		{User: body.User, Relation: body.Relation, Object: fmt.Sprintf("document:%s", docID)},
	})
	if err != nil {
		slog.Error("failed to delete share tuple", "error", err)
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// stripTypePrefix removes the "type:" prefix from each object ID returned by
// ListObjects. For example, "document:abc" becomes "abc".
func stripTypePrefix(objects []string) []string {
	ids := make([]string, 0, len(objects))
	for _, obj := range objects {
		if idx := strings.Index(obj, ":"); idx != -1 {
			ids = append(ids, obj[idx+1:])
		} else {
			ids = append(ids, obj)
		}
	}
	return ids
}
