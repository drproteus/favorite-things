package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"

	"favorite-things/backend/internal/config"
	"favorite-things/backend/internal/httpx"
)

const stateCookie = "fv_oauth_state"

type profile struct {
	Provider       string
	ProviderUserID string
	Email          string
	EmailVerified  bool
	Name           string
	AvatarURL      string
}

type provider struct {
	oauth *oauth2.Config
	fetch func(ctx context.Context, client *http.Client) (profile, error)
}

type Handler struct {
	svc       *Service
	providers map[string]provider
	frontend  string
}

func NewHandler(svc *Service, cfg config.Config) *Handler {
	h := &Handler{
		svc:       svc,
		providers: map[string]provider{},
		frontend:  strings.TrimRight(cfg.FrontendURL, "/"),
	}
	if cfg.GoogleClientID != "" {
		h.providers["google"] = provider{
			oauth: &oauth2.Config{
				ClientID:     cfg.GoogleClientID,
				ClientSecret: cfg.GoogleClientSecret,
				RedirectURL:  cfg.PublicURL + "/api/auth/google/callback",
				Scopes:       []string{"openid", "email", "profile"},
				Endpoint:     google.Endpoint,
			},
			fetch: fetchGoogleProfile,
		}
	}
	if cfg.GitHubClientID != "" {
		h.providers["github"] = provider{
			oauth: &oauth2.Config{
				ClientID:     cfg.GitHubClientID,
				ClientSecret: cfg.GitHubClientSecret,
				RedirectURL:  cfg.PublicURL + "/api/auth/github/callback",
				Scopes:       []string{"read:user", "user:email"},
				Endpoint:     github.Endpoint,
			},
			fetch: fetchGitHubProfile,
		}
	}
	return h
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/providers", h.Providers)
	mux.HandleFunc("GET /api/auth/{provider}/login", h.Login)
	mux.HandleFunc("GET /api/auth/{provider}/callback", h.Callback)
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/me", h.Me)
	mux.HandleFunc("PUT /api/me", RequireUser(h.UpdateMe))
	mux.HandleFunc("GET /api/users/{id}", h.PublicUser)
}

func (h *Handler) Providers(w http.ResponseWriter, r *http.Request) {
	names := []string{}
	for _, n := range []string{"google", "github"} {
		if _, ok := h.providers[n]; ok {
			names = append(names, n)
		}
	}
	httpx.JSON(w, http.StatusOK, map[string][]string{"providers": names})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	p, ok := h.providers[r.PathValue("provider")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	state := randomToken()
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   h.svc.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.Redirect(w, r, p.oauth.AuthCodeURL(state), http.StatusFound)
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	p, ok := h.providers[r.PathValue("provider")]
	if !ok {
		http.NotFound(w, r)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   h.svc.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	q := r.URL.Query()
	c, err := r.Cookie(stateCookie)
	if err != nil || q.Get("state") == "" || c.Value != q.Get("state") {
		h.loginFailed(w, r, "invalid_state")
		return
	}
	if q.Get("error") != "" || q.Get("code") == "" {
		h.loginFailed(w, r, "denied")
		return
	}

	ctx := r.Context()
	token, err := p.oauth.Exchange(ctx, q.Get("code"))
	if err != nil {
		slog.Warn("oauth code exchange failed", "provider", r.PathValue("provider"), "error", err)
		h.loginFailed(w, r, "exchange_failed")
		return
	}

	prof, err := p.fetch(ctx, p.oauth.Client(ctx, token))
	if err != nil {
		slog.Warn("oauth profile fetch failed", "provider", r.PathValue("provider"), "error", err)
		h.loginFailed(w, r, "profile_failed")
		return
	}

	userID, err := h.svc.upsertUser(ctx, prof)
	if err != nil {
		slog.Error("upsert user", "error", err)
		h.loginFailed(w, r, "server_error")
		return
	}

	raw, err := h.svc.createSession(ctx, userID)
	if err != nil {
		slog.Error("create session", "error", err)
		h.loginFailed(w, r, "server_error")
		return
	}
	h.svc.setSessionCookie(w, raw)
	http.Redirect(w, r, h.frontend+"/", http.StatusFound)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := h.svc.revokeSession(r.Context(), c.Value); err != nil {
			slog.Error("revoke session", "error", err)
		}
	}
	h.svc.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r.Context())
	if u == nil {
		httpx.Error(w, http.StatusUnauthorized, "not signed in")
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) loginFailed(w http.ResponseWriter, r *http.Request, reason string) {
	http.Redirect(w, r, h.frontend+"/login?error="+reason, http.StatusFound)
}

// upsertUser finds the user linked to this provider account, or links/creates one.
// An existing user is matched by email only when the provider reports the email as verified.
func (s *Service) upsertUser(ctx context.Context, p profile) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx,
		`SELECT user_id::text FROM oauth_accounts WHERE provider = $1 AND provider_user_id = $2`,
		p.Provider, p.ProviderUserID).Scan(&userID)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		err = nil
		if p.Email != "" && p.EmailVerified {
			err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE lower(email) = lower($1)`, p.Email).Scan(&userID)
			if errors.Is(err, pgx.ErrNoRows) {
				err = nil
				userID = ""
			}
		}
		if err == nil && userID == "" {
			storedEmail := ""
			if p.EmailVerified {
				storedEmail = p.Email
			}
			err = tx.QueryRow(ctx,
				`INSERT INTO users (email, name, avatar_url) VALUES (NULLIF($1, ''), $2, $3) RETURNING id::text`,
				storedEmail, p.Name, p.AvatarURL).Scan(&userID)
		}
		if err == nil {
			_, err = tx.Exec(ctx,
				`INSERT INTO oauth_accounts (provider, provider_user_id, user_id, email) VALUES ($1, $2, $3::uuid, NULLIF($4, ''))`,
				p.Provider, p.ProviderUserID, userID, p.Email)
		}
	case err == nil:
		_, err = tx.Exec(ctx,
			`UPDATE users SET name = CASE WHEN $2 = '' THEN name ELSE $2 END, avatar_url = $3, updated_at = now() WHERE id = $1::uuid`,
			userID, p.Name, p.AvatarURL)
	}
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return userID, nil
}

func fetchGoogleProfile(ctx context.Context, client *http.Client) (profile, error) {
	var u struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := getJSON(ctx, client, "https://openidconnect.googleapis.com/v1/userinfo", &u); err != nil {
		return profile{}, err
	}
	return profile{
		Provider:       "google",
		ProviderUserID: u.Sub,
		Email:          u.Email,
		EmailVerified:  u.EmailVerified,
		Name:           u.Name,
		AvatarURL:      u.Picture,
	}, nil
}

func fetchGitHubProfile(ctx context.Context, client *http.Client) (profile, error) {
	var u struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := getJSON(ctx, client, "https://api.github.com/user", &u); err != nil {
		return profile{}, err
	}

	p := profile{
		Provider:       "github",
		ProviderUserID: strconv.FormatInt(u.ID, 10),
		Name:           u.Name,
		AvatarURL:      u.AvatarURL,
	}
	if p.Name == "" {
		p.Name = u.Login
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := getJSON(ctx, client, "https://api.github.com/user/emails", &emails); err != nil {
		slog.Warn("github emails unavailable", "error", err)
		return p, nil
	}
	for _, e := range emails {
		if !e.Verified {
			continue
		}
		if p.Email == "" || e.Primary {
			p.Email = e.Email
			p.EmailVerified = true
		}
	}
	return p, nil
}

func getJSON(ctx context.Context, client *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "favorite-things")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
