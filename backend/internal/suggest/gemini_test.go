package suggest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiSendsImageAndKey(t *testing.T) {
	image := []byte("fake-jpeg-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/test-model:generateContent" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "secret-key" {
			t.Errorf("api key header = %q", r.Header.Get("x-goog-api-key"))
		}
		body, _ := io.ReadAll(r.Body)
		var req geminiRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatal(err)
		}
		parts := req.Contents[0].Parts
		if len(parts) != 2 || parts[0].InlineData == nil {
			t.Fatalf("expected image then text, got %+v", parts)
		}
		if parts[0].InlineData.MimeType != "image/jpeg" {
			t.Errorf("mime type = %s", parts[0].InlineData.MimeType)
		}
		if got, _ := base64.StdEncoding.DecodeString(parts[0].InlineData.Data); string(got) != string(image) {
			t.Errorf("image bytes not sent intact")
		}
		if !strings.Contains(parts[1].Text, "JSON") {
			t.Errorf("prompt text missing")
		}
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"title\":\"Game Boy\",\"description\":\"\",\"tags\":[]}"}]}}]}`))
	}))
	defer srv.Close()

	g := NewGemini("secret-key", "test-model")
	g.baseURL = srv.URL
	reply, err := g.Describe(context.Background(), image, "jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reply, "Game Boy") {
		t.Errorf("reply = %q", reply)
	}
}

func TestGeminiReportsErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"status":"INVALID_ARGUMENT","message":"API key not valid"}}`))
	}))
	defer srv.Close()

	g := NewGemini("bad-key", "test-model")
	g.baseURL = srv.URL
	_, err := g.Describe(context.Background(), []byte("x"), "png")
	if err == nil || !strings.Contains(err.Error(), "API key not valid") {
		t.Fatalf("error = %v, want the API's message", err)
	}
}
