package collections

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"favorite-things/backend/internal/auth"
	"favorite-things/backend/internal/httpx"
	"favorite-things/backend/internal/media"
	"favorite-things/backend/internal/storage"
)

const (
	maxNameLength        = 100
	maxDescriptionLength = 2000
	maxBannerBytes       = 10 << 20
)

var allowedBannerTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

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
	mux.HandleFunc("GET /api/users/{id}/collections", h.publicList)
	mux.HandleFunc("POST /api/collections/{id}/banner", auth.RequireUser(h.uploadBanner))
	mux.HandleFunc("DELETE /api/collections/{id}/banner", auth.RequireUser(h.deleteBanner))
}

// publicList returns the folders of one user that anyone can see, for their profile page.
func (h *Handler) publicList(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	list, err := h.store.ListPublic(r.Context(), id.String())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if list == nil {
		list = []Collection{}
	}
	if err := h.signAll(r.Context(), list); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, list)
}

// sign adds short-lived URLs for the folder's banner and its generated sizes.
func (h *Handler) sign(ctx context.Context, c *Collection) error {
	if c.BannerKey == nil {
		return nil
	}
	url, err := h.files.URL(ctx, *c.BannerKey)
	if err != nil {
		return err
	}
	c.BannerURL = &url
	if len(c.bannerMeta) == 0 {
		return nil
	}
	views := make([]media.View, 0, len(c.bannerMeta))
	for _, variant := range c.bannerMeta {
		variantURL, err := h.files.URL(ctx, variant.Key)
		if err != nil {
			return err
		}
		views = append(views, media.View{URL: variantURL, Width: variant.Width, Height: variant.Height})
	}
	c.BannerVariants = views
	return nil
}

func (h *Handler) signAll(ctx context.Context, list []Collection) error {
	for i := range list {
		if err := h.sign(ctx, &list[i]); err != nil {
			return err
		}
	}
	return nil
}

// uploadBanner sets a folder's banner image. The previous image, if any, is deleted.
func (h *Handler) uploadBanner(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBannerBytes+(1<<20))
	if err := r.ParseMultipartForm(maxBannerBytes); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid upload or file too large")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBannerBytes+1))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if len(data) > maxBannerBytes {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "banner must be 10 MB or smaller")
		return
	}
	contentType := http.DetectContentType(data)
	ext, ok := allowedBannerTypes[contentType]
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "unsupported image type; use JPEG, PNG, GIF or WebP")
		return
	}

	key := fmt.Sprintf("collections/%s/%s%s", id, uuid.NewString(), ext)
	if err := h.files.Put(r.Context(), key, contentType, data); err != nil {
		httpx.Internal(w, err)
		return
	}

	result, err := media.Process(data, key, contentType)
	if err != nil {
		slog.Warn("generate banner variants", "key", key, "error", err)
	}
	variants := make([]media.Variant, 0, len(result.Variants))
	for _, v := range result.Variants {
		if err := h.files.Put(r.Context(), v.Key, v.ContentType, v.Data); err != nil {
			slog.Warn("store banner variant", "key", v.Key, "error", err)
			continue
		}
		variants = append(variants, v)
	}

	oldKeys, err := h.store.SetBanner(r.Context(), u.ID, id, &key, result.Width, result.Height, variants)
	if err != nil {
		h.removeObjects(r.Context(), media.Keys(key, variants))
		h.writeError(w, err)
		return
	}
	if len(oldKeys) > 0 {
		h.removeObjects(r.Context(), oldKeys)
	}
	h.writeFolder(w, r, u.ID, id, http.StatusOK)
}

// deleteBanner removes a folder's banner image.
func (h *Handler) deleteBanner(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	oldKeys, err := h.store.SetBanner(r.Context(), u.ID, id, nil, 0, 0, nil)
	if err != nil {
		h.writeError(w, err)
		return
	}
	if len(oldKeys) > 0 {
		h.removeObjects(r.Context(), oldKeys)
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeFolder loads one of the user's folders, signs its banner and writes it as JSON.
func (h *Handler) writeFolder(w http.ResponseWriter, r *http.Request, userID, id string, status int) {
	c, err := h.store.Get(r.Context(), userID, id)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if c == nil {
		httpx.Error(w, http.StatusNotFound, ErrNotFound.Error())
		return
	}
	if err := h.sign(r.Context(), c); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, status, c)
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
	Name        string `json:"name"`
	Visibility  string `json:"visibility"`
	ParentID    string `json:"parentId"`
	Description string `json:"description"`
	Recursive   bool   `json:"recursive"`
}

func (b *body) normalize() error {
	b.Name = strings.Join(strings.Fields(b.Name), " ")
	b.Description = strings.TrimSpace(b.Description)
	if b.Visibility == "" {
		b.Visibility = "private"
	}
	switch {
	case b.Name == "":
		return errors.New("name is required")
	case utf8.RuneCountInString(b.Name) > maxNameLength:
		return errors.New("name must be at most 100 characters")
	case utf8.RuneCountInString(b.Description) > maxDescriptionLength:
		return errors.New("description must be at most 2000 characters")
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
	if err := h.signAll(r.Context(), list); err != nil {
		httpx.Internal(w, err)
		return
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
	if in.Visibility == "" {
		in.Visibility = u.DefaultVisibility
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
	if err := h.sign(r.Context(), &c); err != nil {
		httpx.Internal(w, err)
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
	if in.Visibility == "" {
		in.Visibility = u.DefaultVisibility
	}
	if err := in.normalize(); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	parentID, ok := parseParent(w, in.ParentID)
	if !ok {
		return
	}
	c, err := h.store.Update(r.Context(), u.ID, id, in.Name, in.Visibility, parentID, in.Description, in.Recursive)
	if err != nil {
		h.writeError(w, err)
		return
	}
	if err := h.sign(r.Context(), &c); err != nil {
		httpx.Internal(w, err)
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
