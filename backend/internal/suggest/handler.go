package suggest

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"favorite-things/backend/internal/auth"
	"favorite-things/backend/internal/httpx"
)

const (
	maxImageBytes = 10 << 20
	callTimeout   = 30 * time.Second
)

var imageFormats = map[string]string{
	"image/jpeg": "jpeg",
	"image/png":  "png",
	"image/gif":  "gif",
	"image/webp": "webp",
}

type Handler struct {
	enabled bool
	model   Model
	limit   *dailyLimit
}

// NewHandler returns a handler. When enabled is false, or model is nil, the endpoint answers 503.
func NewHandler(enabled bool, model Model, dailyLimit int) *Handler {
	return &Handler{enabled: enabled, model: model, limit: newDailyLimit(dailyLimit)}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/items/suggest", auth.RequireUser(h.suggest))
}

func (h *Handler) suggest(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.model == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "photo suggestions are turned off")
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
	format, ok := imageFormats[http.DetectContentType(data)]
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "unsupported image type; use JPEG, PNG, GIF or WebP")
		return
	}

	u := auth.UserFrom(r.Context())
	if !h.limit.allow(u.ID, time.Now()) {
		httpx.Error(w, http.StatusTooManyRequests, "daily photo suggestion limit reached; try again tomorrow")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), callTimeout)
	defer cancel()
	raw, err := h.model.Describe(ctx, data, format)
	if err != nil {
		slog.Error("photo suggestion failed", "error", err)
		httpx.Error(w, http.StatusBadGateway, "photo suggestion failed; fill in the item by hand")
		return
	}
	s, err := parseSuggestion(raw)
	if err != nil {
		slog.Error("photo suggestion unreadable", "error", err, "reply", raw)
		httpx.Error(w, http.StatusBadGateway, "photo suggestion failed; fill in the item by hand")
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

// dailyLimit caps how many suggestion calls each user can make per day. It is kept in memory,
// so it resets when the server restarts. That is enough to protect a proof-of-concept budget.
type dailyLimit struct {
	mu     sync.Mutex
	max    int
	counts map[string]dayCount
}

type dayCount struct {
	day string
	n   int
}

func newDailyLimit(max int) *dailyLimit {
	return &dailyLimit{max: max, counts: map[string]dayCount{}}
}

func (d *dailyLimit) allow(userID string, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	day := now.UTC().Format("2006-01-02")
	c := d.counts[userID]
	if c.day != day {
		c = dayCount{day: day}
	}
	if c.n >= d.max {
		return false
	}
	c.n++
	d.counts[userID] = c
	return true
}
