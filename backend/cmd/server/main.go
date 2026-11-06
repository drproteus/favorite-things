package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"favorite-things/backend/internal/auth"
	"favorite-things/backend/internal/collections"
	"favorite-things/backend/internal/config"
	"favorite-things/backend/internal/database"
	"favorite-things/backend/internal/items"
	"favorite-things/backend/internal/server"
	"favorite-things/backend/internal/storage"
)

func main() {
	// godotenv never overrides variables already set, so the first file found wins per key.
	for _, path := range []string{".env", "../.env"} {
		_ = godotenv.Load(path)
	}
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		return err
	}

	files := storage.New(cfg)
	if err := files.EnsureBucket(ctx); err != nil {
		return err
	}

	authSvc := auth.NewService(pool, cfg.SessionTTL, cfg.CookieSecure)
	handler := server.New(
		cfg,
		authSvc,
		auth.NewHandler(authSvc, cfg),
		items.NewHandler(items.NewStore(pool), files, collections.NewStore(pool)),
		collections.NewHandler(collections.NewStore(pool), files),
	)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("server listening", "addr", cfg.Addr, "public_url", cfg.PublicURL)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
