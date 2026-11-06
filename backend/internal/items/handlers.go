package items

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"favorite-things/backend/internal/auth"
	"favorite-things/backend/internal/httpx"
	"favorite-things/backend/internal/storage"
)

const (
	maxTitle      = 200
	maxDesc       = 10000
	maxCategory   = 100
	maxTags       = 20
	maxTagLength  = 40
	maxImages     = 20
	maxImageBytes = 10 << 20
)

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

type Input struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Visibility  string   `json:"visibility"`
	Tags        []string `json:"tags"`
}

func (in *Input) normalize() error {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.Category = strings.TrimSpace(in.Category)
	if in.Visibility == "" {
		in.Visibility = "private"
	}

	switch {
	case in.Title == "":
		return errors.New("title is required")
	case utf8.RuneCountInString(in.Title) > maxTitle:
		return fmt.Errorf("title must be at most %d characters", maxTitle)
	case utf8.RuneCountInString(in.Description) > maxDesc:
		return fmt.Errorf("description must be at most %d characters", maxDesc)
	case utf8.RuneCountInString(in.Category) > maxCategory:
		return fmt.Errorf("category must be at most %d characters", maxCategory)
	case in.Visibility != "public" && in.Visibility != "private":
		return errors.New("visibility must be public or private")
	}

	seen := make(map[string]bool, len(in.Tags))
	tags := make([]string, 0, len(in.Tags))
	for _, t := range in.Tags {
		t = normalizeTag(t)
		if t == "" || seen[t] {
			continue
		}
		if utf8.RuneCountInString(t) > maxTagLength {
			return fmt.Errorf("tag %q must be at most %d characters", t, maxTagLength)
		}
		seen[t] = true
		tags = append(tags, t)
	}
	if len(tags) > maxTags {
		return fmt.Errorf("at most %d tags are allowed", maxTags)
	}
	in.Tags = tags
	return nil
}

type Handler struct {
	store *Store
	files *storage.Store
}

func NewHandler(store *Store, files *storage.Store) *Handler {
	return &Handler{store: store, files: files}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/items", h.list)
	mux.HandleFunc("POST /api/items", auth.RequireUser(h.create))
	mux.HandleFunc("GET /api/items/{id}", h.get)
	mux.HandleFunc("PUT /api/items/{id}", auth.RequireUser(h.update))
	mux.HandleFunc("DELETE /api/items/{id}", auth.RequireUser(h.delete))
	mux.HandleFunc("POST /api/items/{id}/images", auth.RequireUser(h.uploadImage))
	mux.HandleFunc("DELETE /api/items/{id}/images/{imageId}", auth.RequireUser(h.deleteImage))
	mux.HandleFunc("PUT /api/items/{id}/rating", auth.RequireUser(h.setRating))
	mux.HandleFunc("DELETE /api/items/{id}/rating", auth.RequireUser(h.clearRating))
	mux.HandleFunc("GET /api/tags", auth.RequireUser(h.tags))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	viewer := viewerID(r)
	q := r.URL.Query()

	f := Filter{
		Query:      strings.TrimSpace(q.Get("q")),
		Tag:        q.Get("tag"),
		Category:   q.Get("category"),
		Visibility: q.Get("visibility"),
		Limit:      queryInt(q.Get("limit"), 24, 1, 100),
		Offset:     queryInt(q.Get("offset"), 0, 0, 1_000_000),
	}
	if q.Get("scope") == "mine" {
		if viewer == "" {
			httpx.Error(w, http.StatusUnauthorized, "sign in required")
			return
		}
		f.OwnerID = viewer
	} else {
		f.Visibility = ""
	}

	list, err := h.store.List(r.Context(), viewer, f)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if list == nil {
		list = []Item{}
	}
	for i := range list {
		if err := h.sign(r.Context(), list[i].Images); err != nil {
			httpx.Internal(w, err)
			return
		}
	}
	httpx.JSON(w, http.StatusOK, list)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	h.writeItem(w, r, id, http.StatusOK)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := in.normalize(); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := h.store.Create(r.Context(), u.ID, in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	h.writeItem(w, r, id, http.StatusCreated)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := in.normalize(); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.store.Update(r.Context(), u.ID, id, in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !updated {
		httpx.Error(w, http.StatusNotFound, "item not found")
		return
	}
	h.writeItem(w, r, id, http.StatusOK)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	keys, deleted, err := h.store.Delete(r.Context(), u.ID, id)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !deleted {
		httpx.Error(w, http.StatusNotFound, "item not found")
		return
	}
	h.removeObjects(r.Context(), keys)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	if !h.ownedBy(w, r, id, u.ID) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+(1<<20))
	if err := r.ParseMultipartForm(maxImageBytes); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid upload or file too large")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if len(data) > maxImageBytes {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "image must be 10 MB or smaller")
		return
	}
	contentType := http.DetectContentType(data)
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "unsupported image type; use JPEG, PNG, GIF or WebP")
		return
	}

	key := fmt.Sprintf("items/%s/%s%s", id, uuid.NewString(), ext)
	if err := h.files.Put(r.Context(), key, contentType, data); err != nil {
		httpx.Internal(w, err)
		return
	}

	img, added, err := h.store.AddImage(r.Context(), id, key, contentType, int64(len(data)), maxImages)
	if err != nil {
		h.removeObjects(r.Context(), []string{key})
		httpx.Internal(w, err)
		return
	}
	if !added {
		h.removeObjects(r.Context(), []string{key})
		httpx.Error(w, http.StatusBadRequest, fmt.Sprintf("an item can have at most %d images", maxImages))
		return
	}

	imgs := []Image{img}
	if err := h.sign(r.Context(), imgs); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, imgs[0])
}

func (h *Handler) deleteImage(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	imageID, ok := parseID(w, r.PathValue("imageId"))
	if !ok {
		return
	}
	key, deleted, err := h.store.DeleteImage(r.Context(), u.ID, id, imageID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !deleted {
		httpx.Error(w, http.StatusNotFound, "image not found")
		return
	}
	h.removeObjects(r.Context(), []string{key})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setRating(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	var body struct {
		Rating int `json:"rating"`
	}
	if err := httpx.Decode(r, &body); err != nil || body.Rating < 1 || body.Rating > 5 {
		httpx.Error(w, http.StatusBadRequest, "rating must be an integer from 1 to 5")
		return
	}
	saved, err := h.store.SetRating(r.Context(), u.ID, id, body.Rating)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !saved {
		httpx.Error(w, http.StatusNotFound, "item not found")
		return
	}
	h.writeItem(w, r, id, http.StatusOK)
}

func (h *Handler) clearRating(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	if err := h.store.DeleteRating(r.Context(), u.ID, id); err != nil {
		httpx.Internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) tags(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	tags, err := h.store.ListTags(r.Context(), u.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if tags == nil {
		tags = []string{}
	}
	httpx.JSON(w, http.StatusOK, tags)
}

// writeItem loads an item visible to the viewer, signs its image URLs and writes it as JSON.
func (h *Handler) writeItem(w http.ResponseWriter, r *http.Request, id string, status int) {
	it, err := h.store.Get(r.Context(), viewerID(r), id)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if it == nil {
		httpx.Error(w, http.StatusNotFound, "item not found")
		return
	}
	if err := h.sign(r.Context(), it.Images); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, status, it)
}

// ownedBy writes an error response and returns false unless the item exists and belongs to userID.
func (h *Handler) ownedBy(w http.ResponseWriter, r *http.Request, id, userID string) bool {
	it, err := h.store.Get(r.Context(), userID, id)
	if err != nil {
		httpx.Internal(w, err)
		return false
	}
	if it == nil {
		httpx.Error(w, http.StatusNotFound, "item not found")
		return false
	}
	if it.OwnerID != userID {
		httpx.Error(w, http.StatusForbidden, "you do not own this item")
		return false
	}
	return true
}

func (h *Handler) sign(ctx context.Context, images []Image) error {
	for i := range images {
		url, err := h.files.URL(ctx, images[i].key)
		if err != nil {
			return err
		}
		images[i].URL = url
	}
	return nil
}

// removeObjects deletes stored files. Failures are logged; the database row is already gone.
func (h *Handler) removeObjects(ctx context.Context, keys []string) {
	for _, key := range keys {
		if err := h.files.Delete(ctx, key); err != nil {
			slog.Warn("delete object", "key", key, "error", err)
		}
	}
}

func viewerID(r *http.Request) string {
	if u := auth.UserFrom(r.Context()); u != nil {
		return u.ID
	}
	return ""
}

func parseID(w http.ResponseWriter, raw string) (string, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not found")
		return "", false
	}
	return id.String(), true
}

func queryInt(raw string, def, lo, hi int) int {
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return min(max(v, lo), hi)
}
