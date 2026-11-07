package comments

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"favorite-things/backend/internal/auth"
	"favorite-things/backend/internal/httpx"
	"favorite-things/backend/internal/items"
)

const maxBody = 5000

const (
	VisibilitySignedIn = "signed_in"
	VisibilityPublic   = "public"
	VisibilityPrivate  = "private"
)

// allowedVisibilities is intentionally limited to signed-in for now. The database
// and API already carry a visibility value, so adding public or private comments is
// a matter of allowing those values here and exposing a control in the UI.
var allowedVisibilities = map[string]bool{
	VisibilitySignedIn: true,
}

type Handler struct {
	store *Store
	items *items.Store
}

func NewHandler(store *Store, itemsStore *items.Store) *Handler {
	return &Handler{store: store, items: itemsStore}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/items/{id}/comments", auth.RequireUser(h.list))
	mux.HandleFunc("POST /api/items/{id}/comments", auth.RequireUser(h.create))
	mux.HandleFunc("PUT /api/items/{id}/comments/{commentId}", auth.RequireUser(h.update))
}

type input struct {
	Body       string `json:"body"`
	Visibility string `json:"visibility"`
}

func (in *input) normalize(defaultVisibility string) error {
	in.Body = strings.TrimSpace(in.Body)
	switch {
	case in.Body == "":
		return errors.New("comment cannot be empty")
	case utf8.RuneCountInString(in.Body) > maxBody:
		return fmt.Errorf("comment must be at most %d characters", maxBody)
	}

	if in.Visibility == "" {
		in.Visibility = defaultVisibility
		return nil
	}
	if !allowedVisibilities[in.Visibility] {
		return errors.New("visibility must be signed_in for now")
	}
	return nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	itemID, ok := parseID(w, r.PathValue("id"), "item")
	if !ok {
		return
	}
	if _, ok := h.visibleItem(w, r, u.ID, itemID); !ok {
		return
	}

	list, err := h.store.List(r.Context(), itemID, u.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if list == nil {
		list = []Comment{}
	}
	httpx.JSON(w, http.StatusOK, list)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	itemID, ok := parseID(w, r.PathValue("id"), "item")
	if !ok {
		return
	}
	item, ok := h.visibleItem(w, r, u.ID, itemID)
	if !ok {
		return
	}
	// Comments start as owner-only. When other authors are allowed, this check
	// moves into a permission function without changing the API shape.
	if item.OwnerID != u.ID {
		httpx.Error(w, http.StatusForbidden, "only the item owner can comment for now")
		return
	}

	var in input
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := in.normalize(VisibilitySignedIn); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	comment, err := h.store.Create(r.Context(), itemID, u.ID, in.Body, in.Visibility)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, comment)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	itemID, ok := parseID(w, r.PathValue("id"), "item")
	if !ok {
		return
	}
	commentID, ok := parseID(w, r.PathValue("commentId"), "comment")
	if !ok {
		return
	}
	if _, ok := h.visibleItem(w, r, u.ID, itemID); !ok {
		return
	}

	var in input
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// An empty visibility means "keep the current one".
	if err := in.normalize(""); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	comment, updated, err := h.store.Update(r.Context(), itemID, commentID, u.ID, in.Body, in.Visibility)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !updated {
		httpx.Error(w, http.StatusNotFound, "comment not found")
		return
	}
	httpx.JSON(w, http.StatusOK, comment)
}

// visibleItem returns the item when the viewer may see it, writing 404 otherwise.
func (h *Handler) visibleItem(w http.ResponseWriter, r *http.Request, viewer, itemID string) (*items.Item, bool) {
	item, err := h.items.Get(r.Context(), viewer, itemID)
	if err != nil {
		httpx.Internal(w, err)
		return nil, false
	}
	if item == nil {
		httpx.Error(w, http.StatusNotFound, "item not found")
		return nil, false
	}
	return item, true
}

func parseID(w http.ResponseWriter, raw, kind string) (string, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, kind+" not found")
		return "", false
	}
	return id.String(), true
}
