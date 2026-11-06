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

	// Photo suggestions use Amazon Bedrock. Credentials come from the standard AWS chain:
	// AWS_PROFILE, AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY, or an IAM role when deployed.
	SuggestEnabled    bool
	SuggestProvider   string // gemini or bedrock
	SuggestDailyLimit int
	GeminiAPIKey      string
	GeminiModel       string
	BedrockRegion     string
	BedrockModelID    string
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

		// Storage targets MinIO in local development and AWS S3 in production.
		// Credentials use the standard AWS names; when they are unset the MinIO
		// defaults apply. S3_ENDPOINT may be set to an empty value to let the AWS
		// SDK use its default regional endpoint.
		S3Endpoint:       envAllowEmpty("S3_ENDPOINT", "http://localhost:9000"),
		S3PublicEndpoint: os.Getenv("S3_PUBLIC_ENDPOINT"),
		S3Region:         env("S3_REGION", "us-east-1"),
		S3Bucket:         env("S3_BUCKET", "favorite-things"),
		S3AccessKey:      env("AWS_ACCESS_KEY_ID", env("S3_ACCESS_KEY", "minioadmin")),
		S3SecretKey:      env("AWS_SECRET_ACCESS_KEY", env("S3_SECRET_KEY", "minioadmin")),
		S3PathStyle:      envBool("S3_PATH_STYLE", true),

		SuggestEnabled:    envBool("SUGGEST_ENABLED", false),
		SuggestProvider:   env("SUGGEST_PROVIDER", "gemini"),
		SuggestDailyLimit: envInt("SUGGEST_DAILY_LIMIT", 50),
		GeminiAPIKey:      os.Getenv("GEMINI_API_KEY"),
		GeminiModel:       env("GEMINI_MODEL", "gemini-3.8-flash"),
		BedrockRegion:     env("BEDROCK_REGION", "us-east-1"),
		BedrockModelID:    env("BEDROCK_MODEL_ID", "amazon.nova-lite-v1:0"),
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// envAllowEmpty returns the value when the variable is set, even when its value
// is empty. It falls back only when the variable is absent entirely.
func envAllowEmpty(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
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

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
