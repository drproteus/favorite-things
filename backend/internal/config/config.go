package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr         string
	PublicURL    string
	FrontendURL  string
	WebDir       string
	DatabaseURL  string
	SessionTTL   time.Duration
	CookieSecure bool

	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string

	S3Endpoint       string
	S3PublicEndpoint string
	S3Region         string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	S3PathStyle      bool
}

func Load() Config {
	return Config{
		Addr:         env("ADDR", ":8080"),
		PublicURL:    env("PUBLIC_URL", "http://localhost:8080"),
		FrontendURL:  env("FRONTEND_URL", "http://localhost:5173"),
		WebDir:       env("WEB_DIR", ""),
		DatabaseURL:  env("DATABASE_URL", "postgres://favs:favs@localhost:5433/favorites?sslmode=disable"),
		SessionTTL:   envDuration("SESSION_TTL", 30*24*time.Hour),
		CookieSecure: envBool("COOKIE_SECURE", false),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),

		S3Endpoint:       env("S3_ENDPOINT", "http://localhost:9000"),
		S3PublicEndpoint: os.Getenv("S3_PUBLIC_ENDPOINT"),
		S3Region:         env("S3_REGION", "us-east-1"),
		S3Bucket:         env("S3_BUCKET", "favorite-things"),
		S3AccessKey:      env("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:      env("S3_SECRET_KEY", "minioadmin"),
		S3PathStyle:      envBool("S3_PATH_STYLE", true),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
