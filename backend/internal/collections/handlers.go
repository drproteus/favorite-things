package collections

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"favorite-things/backend/internal/auth"
	"favorite-things/backend/internal/httpx"
	"favorite-things/backend/internal/storage"
)

const maxNameLength = 100

type Handler struct {
	store *Store
	files *storage.Store
}

func NewHandler(store *Store, files *storage.Store) *Handler {
	return &Handler{store: store, files: files}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/collections", auth.RequireUser(h.list))
	mux.HandleFunc("POST /api/collections", auth.RequireUser(h.create))
	mux.HandleFunc("PUT /api/collections/{id}", auth.RequireUser(h.update))
	mux.HandleFunc("DELETE /api/collections/{id}", auth.RequireUser(h.delete))
	mux.HandleFunc("POST /api/collections/bulk/move", auth.RequireUser(h.bulkMove))
	mux.HandleFunc("POST /api/collections/bulk/delete", auth.RequireUser(h.bulkDelete))
	mux.HandleFunc("POST /api/collections/bulk/visibility", auth.RequireUser(h.bulkVisibility))
}

func (h *Handler) bulkVisibility(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	var in struct {
		IDs        []string `json:"ids"`
		Visibility string   `json:"visibility"`
		Recursive  bool     `json:"recursive"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	ids, ok := httpx.UUIDList(w, in.IDs)
	if !ok {
		return
	}
	if in.Visibility != "public" && in.Visibility != "private" {
		httpx.Error(w, http.StatusBadRequest, "visibility must be public or private")
		return
	}
	if err := h.store.SetVisibilityMany(r.Context(), u.ID, ids, in.Visibility, in.Recursive); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) bulkMove(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	var in struct {
		IDs      []string `json:"ids"`
		ParentID string   `json:"parentId"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	ids, ok := httpx.UUIDList(w, in.IDs)
	if !ok {
		return
	}
	parentID, ok := parseParent(w, in.ParentID)
	if !ok {
		return
	}
	if err := h.store.MoveMany(r.Context(), u.ID, ids, parentID); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) bulkDelete(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	ids, ok := httpx.UUIDList(w, in.IDs)
	if !ok {
		return
	}
	mode, ok := deleteMode(w, r)
	if !ok {
		return
	}
	keys, err := h.store.DeleteMany(r.Context(), u.ID, ids, mode)
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.removeObjects(r.Context(), keys)
	w.WriteHeader(http.StatusNoContent)
}

func deleteMode(w http.ResponseWriter, r *http.Request) (DeleteMode, bool) {
	switch mode := DeleteMode(r.URL.Query().Get("contents")); mode {
	case "":
		return DeleteContents, true
	case DeleteContents, MoveContents:
		return mode, true
	default:
		httpx.Error(w, http.StatusBadRequest, "contents must be delete or move")
		return "", false
	}
}

func (h *Handler) removeObjects(ctx context.Context, keys []string) {
	for _, key := range keys {
		if err := h.files.Delete(ctx, key); err != nil {
			slog.Warn("delete object", "key", key, "error", err)
		}
	}
}

type body struct {
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
	ParentID   string `json:"parentId"`
	Recursive  bool   `json:"recursive"`
}

func (b *body) normalize() error {
	b.Name = strings.Join(strings.Fields(b.Name), " ")
	if b.Visibility == "" {
		b.Visibility = "private"
	}
	switch {
	case b.Name == "":
		return errors.New("name is required")
	case utf8.RuneCountInString(b.Name) > maxNameLength:
		return errors.New("name must be at most 100 characters")
	case b.Visibility != "public" && b.Visibility != "private":
		return errors.New("visibility must be public or private")
	}
	return nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	if _, err := h.store.RootID(r.Context(), u.ID); err != nil {
		httpx.Internal(w, err)
		return
	}
	list, err := h.store.List(r.Context(), u.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if list == nil {
		list = []Collection{}
	}
	httpx.JSON(w, http.StatusOK, list)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	var in body
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := in.normalize(); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	parentID, ok := h.parentOrRoot(w, r, u.ID, in.ParentID)
	if !ok {
		return
	}
	c, err := h.store.Create(r.Context(), u.ID, parentID, in.Name, in.Visibility)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in body
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := in.normalize(); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	parentID, ok := parseParent(w, in.ParentID)
	if !ok {
		return
	}
	c, err := h.store.Update(r.Context(), u.ID, id, in.Name, in.Visibility, parentID, in.Recursive)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	mode, ok := deleteMode(w, r)
	if !ok {
		return
	}
	keys, err := h.store.Delete(r.Context(), u.ID, id, mode)
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.removeObjects(r.Context(), keys)
	w.WriteHeader(http.StatusNoContent)
}

// parentOrRoot returns the requested parent folder, or the user's root folder when none is given.
func (h *Handler) parentOrRoot(w http.ResponseWriter, r *http.Request, userID, raw string) (string, bool) {
	if raw == "" {
		root, err := h.store.RootID(r.Context(), userID)
		if err != nil {
			httpx.Internal(w, err)
			return "", false
		}
		return root, true
	}
	return parseParent(w, raw)
}

func parseParent(w http.ResponseWriter, raw string) (string, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "parentId must be a folder ID")
		return "", false
	}
	return id.String(), true
}

func parseID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "folder not found")
		return "", false
	}
	return id.String(), true
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrRoot), errors.Is(err, ErrCycle):
		httpx.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNameTaken):
		httpx.Error(w, http.StatusConflict, err.Error())
	default:
		httpx.Internal(w, err)
	}
}
