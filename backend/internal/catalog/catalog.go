// Package catalog manages the site-wide categories. Reads are public; changes require an admin.
// Items must reference one of these categories. Tags are not managed here: they are #hashtags
// written into item descriptions.
package catalog

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"favorite-things/backend/internal/auth"
	"favorite-things/backend/internal/httpx"
)

var (
	ErrDuplicate = errors.New("that name already exists")
	ErrInUse     = errors.New("this category is used by one or more items, so it can't be deleted")
)

const (
	categoryTable   = "site_categories"
	categoryMaxName = 50
)

type Term struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func normalizeName(raw string) (string, error) {
	name := strings.Join(strings.Fields(raw), " ")
	switch {
	case name == "":
		return "", errors.New("name is required")
	case utf8.RuneCountInString(name) > categoryMaxName:
		return "", fmt.Errorf("name must be at most %d characters", categoryMaxName)
	}
	return name, nil
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) list(ctx context.Context) ([]Term, error) {
	rows, err := s.pool.Query(ctx,
		fmt.Sprintf(`SELECT id, name FROM %s ORDER BY sort_order, lower(name)`, categoryTable))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Term, error) {
		var t Term
		err := r.Scan(&t.ID, &t.Name)
		return t, err
	})
}

func (s *Store) create(ctx context.Context, name string) (Term, error) {
	var t Term
	err := s.pool.QueryRow(ctx,
		fmt.Sprintf(`INSERT INTO %[1]s (name, sort_order)
			VALUES ($1, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM %[1]s))
			RETURNING id, name`, categoryTable), name).
		Scan(&t.ID, &t.Name)
	return t, mapErr(err)
}

func (s *Store) rename(ctx context.Context, id int64, name string) (Term, bool, error) {
	var t Term
	err := s.pool.QueryRow(ctx,
		fmt.Sprintf(`UPDATE %s SET name = $2 WHERE id = $1 RETURNING id, name`, categoryTable), id, name).
		Scan(&t.ID, &t.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return Term{}, false, nil
	}
	if err != nil {
		return Term{}, false, mapErr(err)
	}
	return t, true, nil
}

func (s *Store) remove(ctx context.Context, id int64) (bool, error) {
	tag, err := s.pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, categoryTable), id)
	if err != nil {
		return false, mapErr(err)
	}
	return tag.RowsAffected() > 0, nil
}

// reorder sets sort_order to each id's position in ids.
func (s *Store) reorder(ctx context.Context, ids []int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	stmt := fmt.Sprintf(`UPDATE %s SET sort_order = $2 WHERE id = $1`, categoryTable)
	for i, id := range ids {
		if _, err := tx.Exec(ctx, stmt, id, i+1); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func mapErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23505":
		return ErrDuplicate
	case "23503":
		return ErrInUse
	}
	return err
}

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/catalog", h.catalog)

	mux.HandleFunc("POST /api/admin/categories", auth.RequireAdmin(h.create))
	mux.HandleFunc("PUT /api/admin/categories/order", auth.RequireAdmin(h.reorder))
	mux.HandleFunc("PUT /api/admin/categories/{id}", auth.RequireAdmin(h.rename))
	mux.HandleFunc("DELETE /api/admin/categories/{id}", auth.RequireAdmin(h.remove))
}

func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	categories, err := h.store.list(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string][]Term{"categories": nonNil(categories)})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	name, ok := readName(w, r)
	if !ok {
		return
	}
	t, err := h.store.create(r.Context(), name)
	if errors.Is(err, ErrDuplicate) {
		httpx.Error(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, t)
}

func (h *Handler) rename(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	name, ok := readName(w, r)
	if !ok {
		return
	}
	t, found, err := h.store.rename(r.Context(), id, name)
	if errors.Is(err, ErrDuplicate) {
		httpx.Error(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Error(w, http.StatusNotFound, "not found")
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	deleted, err := h.store.remove(r.Context(), id)
	if errors.Is(err, ErrInUse) {
		httpx.Error(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !deleted {
		httpx.Error(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) reorder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := httpx.Decode(r, &body); err != nil || len(body.IDs) == 0 {
		httpx.Error(w, http.StatusBadRequest, "ids must be a non-empty list")
		return
	}
	seen := make(map[int64]bool, len(body.IDs))
	for _, id := range body.IDs {
		if seen[id] {
			httpx.Error(w, http.StatusBadRequest, "ids must not repeat")
			return
		}
		seen[id] = true
	}
	if err := h.store.reorder(r.Context(), body.IDs); err != nil {
		httpx.Internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func readName(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return "", false
	}
	name, err := normalizeName(body.Name)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return "", false
	}
	return name, true
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not found")
		return 0, false
	}
	return id, true
}

func nonNil(terms []Term) []Term {
	if terms == nil {
		return []Term{}
	}
	return terms
}
