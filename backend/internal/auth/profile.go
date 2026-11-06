package auth

import (
	"errors"
	"net/http"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"favorite-things/backend/internal/httpx"
)

const maxProfileLength = 2000

// PublicUser is what anyone can see about a user.
type PublicUser struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	Profile   string `json:"profile"`
}

// UpdateMe saves the signed-in user's profile text and default visibility.
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r.Context())
	var in struct {
		Profile           string `json:"profile"`
		DefaultVisibility string `json:"defaultVisibility"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if utf8.RuneCountInString(in.Profile) > maxProfileLength {
		httpx.Error(w, http.StatusBadRequest, "profile must be at most 2000 characters")
		return
	}
	if in.DefaultVisibility != "public" && in.DefaultVisibility != "private" {
		httpx.Error(w, http.StatusBadRequest, "default visibility must be public or private")
		return
	}
	if _, err := h.svc.pool.Exec(r.Context(),
		`UPDATE users SET profile = $2, default_visibility = $3, updated_at = now() WHERE id = $1::uuid`,
		u.ID, in.Profile, in.DefaultVisibility); err != nil {
		httpx.Internal(w, err)
		return
	}
	updated := *u
	updated.Profile = in.Profile
	updated.DefaultVisibility = in.DefaultVisibility
	httpx.JSON(w, http.StatusOK, updated)
}

// PublicUser returns the public part of a user's profile. Anyone can read it, signed in or not.
func (h *Handler) PublicUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	var p PublicUser
	err = h.svc.pool.QueryRow(r.Context(),
		`SELECT id::text, name, avatar_url, profile FROM users WHERE id = $1::uuid`, id).
		Scan(&p.ID, &p.Name, &p.AvatarURL, &p.Profile)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}
