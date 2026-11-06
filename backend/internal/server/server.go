package server

import (
	"net/http"
	"path"
	"path/filepath"

	"favorite-things/backend/internal/auth"
	"favorite-things/backend/internal/collections"
	"favorite-things/backend/internal/config"
	"favorite-things/backend/internal/httpx"
	"favorite-things/backend/internal/items"
)

// New builds the HTTP handler: the JSON API under /api and, when WEB_DIR is set,
// the built React app with an index.html fallback for client-side routes.
func New(cfg config.Config, authSvc *auth.Service, authH *auth.Handler, itemsH *items.Handler, collectionsH *collections.Handler) http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	authH.Routes(api)
	itemsH.Routes(api)
	collectionsH.Routes(api)

	root := http.NewServeMux()
	root.Handle("/api/", authSvc.Middleware(api))
	if cfg.WebDir != "" {
		root.Handle("/", spaHandler(cfg.WebDir))
	}
	return root
}

func spaHandler(dir string) http.Handler {
	fsys := http.Dir(dir)
	files := http.FileServer(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := path.Clean("/" + r.URL.Path)
		if f, err := fsys.Open(name); err == nil {
			info, statErr := f.Stat()
			f.Close()
			if statErr == nil && !info.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}
