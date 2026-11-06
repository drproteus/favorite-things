// Package catalog manages the site-wide vocabulary of categories and tags that
// users pick from. Reads are public; changes require an admin.
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

var ErrDuplicate = errors.New("that name already exists")

// kind describes one managed table. Table names are internal constants, never user input.
type kind struct {
	table  string
	maxLen int
	lower  bool
}

var (
	categoryKind = kind{table: "site_categories", maxLen: 50}
	tagKind      = kind{table: "site_tags", maxLen: 40, lower: true}
)

type Term struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (k kind) normalize(raw string) (string, error) {
	name := strings.Join(strings.Fields(raw), " ")
	if k.lower {
		name = strings.ToLower(name)
	}
	switch {
	case name == "":
		return "", errors.New("name is required")
	case utf8.RuneCountInString(name) > k.maxLen:
		return "", fmt.Errorf("name must be at most %d characters", k.maxLen)
	}
	return name, nil
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) list(ctx context.Context, k kind) ([]Term, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`SELECT id, name FROM %s ORDER BY lower(name)`, k.table))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Term, error) {
		var t Term
		err := r.Scan(&t.ID, &t.Name)
		return t, err
	})
}

func (s *Store) create(ctx context.Context, k kind, name string) (Term, error) {
	var t Term
	err := s.pool.QueryRow(ctx,
		fmt.Sprintf(`INSERT INTO %s (name) VALUES ($1) RETURNING id, name`, k.table), name).
		Scan(&t.ID, &t.Name)
	return t, mapErr(err)
}

func (s *Store) rename(ctx context.Context, k kind, id int64, name string) (Term, bool, error) {
	var t Term
	err := s.pool.QueryRow(ctx,
		fmt.Sprintf(`UPDATE %s SET name = $2 WHERE id = $1 RETURNING id, name`, k.table), id, name).
		Scan(&t.ID, &t.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return Term{}, false, nil
	}
	if err != nil {
		return Term{}, false, mapErr(err)
	}
	return t, true, nil
}

func (s *Store) remove(ctx context.Context, k kind, id int64) (bool, error) {
	tag, err := s.pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, k.table), id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func mapErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicate
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

	mux.HandleFunc("POST /api/admin/categories", auth.RequireAdmin(h.create(categoryKind)))
	mux.HandleFunc("PUT /api/admin/categories/{id}", auth.RequireAdmin(h.rename(categoryKind)))
	mux.HandleFunc("DELETE /api/admin/categories/{id}", auth.RequireAdmin(h.remove(categoryKind)))

	mux.HandleFunc("POST /api/admin/tags", auth.RequireAdmin(h.create(tagKind)))
	mux.HandleFunc("PUT /api/admin/tags/{id}", auth.RequireAdmin(h.rename(tagKind)))
	mux.HandleFunc("DELETE /api/admin/tags/{id}", auth.RequireAdmin(h.remove(tagKind)))
}

func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	categories, err := h.store.list(r.Context(), categoryKind)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	tags, err := h.store.list(r.Context(), tagKind)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string][]Term{"categories": nonNil(categories), "tags": nonNil(tags)})
}

func (h *Handler) create(k kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, ok := readName(w, r, k)
		if !ok {
			return
		}
		t, err := h.store.create(r.Context(), k, name)
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
}

func (h *Handler) rename(k kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		name, ok := readName(w, r, k)
		if !ok {
			return
		}
		t, found, err := h.store.rename(r.Context(), k, id, name)
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
}

func (h *Handler) remove(k kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseID(w, r)
		if !ok {
			return
		}
		deleted, err := h.store.remove(r.Context(), k, id)
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
}

func readName(w http.ResponseWriter, r *http.Request, k kind) (string, bool) {
	var body struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return "", false
	}
	name, err := k.normalize(body.Name)
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
