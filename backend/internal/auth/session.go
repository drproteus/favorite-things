package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"favorite-things/backend/internal/httpx"
)

const sessionCookie = "fv_session"

type contextKey struct{}

type User struct {
	ID                string `json:"id"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	AvatarURL         string `json:"avatarUrl"`
	IsAdmin           bool   `json:"isAdmin"`
	Profile           string `json:"profile"`
	DefaultVisibility string `json:"defaultVisibility"`
}

// Service manages server-side sessions. Only a SHA-256 hash of each session token is stored.
type Service struct {
	pool   *pgxpool.Pool
	ttl    time.Duration
	secure bool
}

func NewService(pool *pgxpool.Pool, ttl time.Duration, secure bool) *Service {
	return &Service{pool: pool, ttl: ttl, secure: secure}
}

func (s *Service) createSession(ctx context.Context, userID string) (string, error) {
	token := randomToken()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2::uuid, $3)`,
		hashToken(token), userID, time.Now().Add(s.ttl))
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) revokeSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
	return err
}

func (s *Service) userForToken(ctx context.Context, token string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text, COALESCE(u.email, ''), u.name, u.avatar_url, u.is_admin, u.profile, u.default_visibility
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`, hashToken(token)).
		Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.IsAdmin, &u.Profile, &u.DefaultVisibility)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Service) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.ttl.Seconds()),
	})
}

func (s *Service) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// Middleware attaches the signed-in user (if any) to the request context.
// It never rejects requests; use RequireUser on routes that need authentication.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil {
			if u, err := s.userForToken(r.Context(), c.Value); err == nil && u != nil {
				r = r.WithContext(context.WithValue(r.Context(), contextKey{}, u))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func UserFrom(ctx context.Context) *User {
	u, _ := ctx.Value(contextKey{}).(*User)
	return u
}

func RequireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if UserFrom(r.Context()) == nil {
			httpx.Error(w, http.StatusUnauthorized, "sign in required")
			return
		}
		next(w, r)
	}
}

func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := UserFrom(r.Context())
		if u == nil {
			httpx.Error(w, http.StatusUnauthorized, "sign in required")
			return
		}
		if !u.IsAdmin {
			httpx.Error(w, http.StatusForbidden, "admin access required")
			return
		}
		next(w, r)
	}
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
