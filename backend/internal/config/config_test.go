package config

import (
	"os"
	"testing"
)

func unsetenv(t *testing.T, key string) {
	t.Helper()
	old, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(key, old)
			return
		}
		_ = os.Unsetenv(key)
	})
}

func clearStorageEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"S3_ENDPOINT", "S3_PUBLIC_ENDPOINT", "S3_REGION", "S3_BUCKET",
		"S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_PATH_STYLE",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
	} {
		unsetenv(t, key)
	}
}

func TestStorageDefaultsToMinIO(t *testing.T) {
	clearStorageEnv(t)

	cfg := Load()
	if cfg.S3Endpoint != "http://localhost:9000" {
		t.Errorf("S3Endpoint = %q, want the local MinIO endpoint", cfg.S3Endpoint)
	}
	if cfg.S3Region != "us-east-1" {
		t.Errorf("S3Region = %q, want us-east-1", cfg.S3Region)
	}
	if cfg.S3Bucket != "favorite-things" {
		t.Errorf("S3Bucket = %q, want favorite-things", cfg.S3Bucket)
	}
	if cfg.S3AccessKey != "minioadmin" || cfg.S3SecretKey != "minioadmin" {
		t.Errorf("credentials = %q/%q, want the MinIO defaults", cfg.S3AccessKey, cfg.S3SecretKey)
	}
	if !cfg.S3PathStyle {
		t.Error("S3PathStyle = false, want true for MinIO")
	}
}

func TestStorageUsesAWSAccessKeyNames(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("S3_ENDPOINT", "https://s3.us-west-2.amazonaws.com")
	t.Setenv("S3_REGION", "us-west-2")
	t.Setenv("S3_BUCKET", "my-bucket")
	t.Setenv("S3_PATH_STYLE", "false")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")

	cfg := Load()
	if cfg.S3AccessKey != "AKIAEXAMPLE" || cfg.S3SecretKey != "secret" {
		t.Errorf("credentials = %q/%q, want the AWS access key pair", cfg.S3AccessKey, cfg.S3SecretKey)
	}
	if cfg.S3Endpoint != "https://s3.us-west-2.amazonaws.com" {
		t.Errorf("S3Endpoint = %q", cfg.S3Endpoint)
	}
	if cfg.S3PathStyle {
		t.Error("S3PathStyle = true, want false for AWS S3")
	}
}

func TestStorageLegacyCredentialNamesStillWork(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("S3_ACCESS_KEY", "legacy-key")
	t.Setenv("S3_SECRET_KEY", "legacy-secret")

	cfg := Load()
	if cfg.S3AccessKey != "legacy-key" || cfg.S3SecretKey != "legacy-secret" {
		t.Errorf("credentials = %q/%q, want the legacy S3_* values", cfg.S3AccessKey, cfg.S3SecretKey)
	}
}

func TestStorageEmptyEndpointIsHonored(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("S3_ENDPOINT", "")

	if cfg := Load(); cfg.S3Endpoint != "" {
		t.Errorf("S3Endpoint = %q, want an empty endpoint (AWS default resolver)", cfg.S3Endpoint)
	}
}
