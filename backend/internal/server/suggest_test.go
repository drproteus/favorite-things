package server_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"favorite-things/backend/internal/suggest"
)

// fakeModel stands in for Bedrock so the endpoint can be tested without AWS.
type fakeModel struct {
	reply string
	err   error
	calls int
}

func (m *fakeModel) Describe(_ context.Context, _ []byte, _ string) (string, error) {
	m.calls++
	return m.reply, m.err
}

// pngBytes is enough of a PNG for content sniffing to recognize the type.
var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)

func (f *fixture) suggestUpload(a *account, data []byte) (int, []byte) {
	f.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "photo.png")
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		f.t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		f.t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, f.url+"/api/items/suggest", &buf)
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if a != nil {
		req.Header.Set("Cookie", "fv_session="+a.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	return resp.StatusCode, out
}

func TestSuggestionIsOffByDefault(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	status, body := f.suggestUpload(alice, pngBytes)
	if status != http.StatusServiceUnavailable {
		t.Errorf("disabled suggestions: status %d, body %s", status, body)
	}
}

func TestSuggestionRequiresSignIn(t *testing.T) {
	f := newFixtureWith(t, suggest.NewHandler(true, &fakeModel{}, 10))
	if status, _ := f.suggestUpload(nil, pngBytes); status != http.StatusUnauthorized {
		t.Errorf("anonymous suggestion: status %d, want 401", status)
	}
}

func TestSuggestionReturnsCleanedFields(t *testing.T) {
	model := &fakeModel{reply: `{"title": "Game Boy Color", "description": "A blue handheld.", "tags": ["Retro", "#mint"]}`}
	f := newFixtureWith(t, suggest.NewHandler(true, model, 10))
	alice := f.newUser("Alice")

	status, body := f.suggestUpload(alice, pngBytes)
	if status != http.StatusOK {
		t.Fatalf("suggestion: %d %s", status, body)
	}
	got := decodeMap(t, body)
	if got["title"] != "Game Boy Color" {
		t.Errorf("title = %v", got["title"])
	}
	desc, _ := got["description"].(string)
	if !strings.Contains(desc, "#retro") || !strings.Contains(desc, "#mint") {
		t.Errorf("description %q should carry the tags as hashtags", desc)
	}
}

func TestSuggestionRejectsNonImages(t *testing.T) {
	model := &fakeModel{reply: `{"title": "x"}`}
	f := newFixtureWith(t, suggest.NewHandler(true, model, 10))
	alice := f.newUser("Alice")

	status, body := f.suggestUpload(alice, []byte("this is a text file"))
	if status != http.StatusBadRequest {
		t.Errorf("text upload: status %d, body %s", status, body)
	}
	if model.calls != 0 {
		t.Errorf("model was called for a non-image; that would spend money")
	}
}

func TestSuggestionDailyLimit(t *testing.T) {
	model := &fakeModel{reply: `{"title": "x", "description": "", "tags": []}`}
	f := newFixtureWith(t, suggest.NewHandler(true, model, 2))
	alice := f.newUser("Alice")

	for i := 0; i < 2; i++ {
		if status, body := f.suggestUpload(alice, pngBytes); status != http.StatusOK {
			t.Fatalf("call %d: %d %s", i+1, status, body)
		}
	}
	if status, _ := f.suggestUpload(alice, pngBytes); status != http.StatusTooManyRequests {
		t.Errorf("third call: status %d, want 429", status)
	}
	if model.calls != 2 {
		t.Errorf("model called %d times, want 2", model.calls)
	}
}

func TestSuggestionModelFailureHidesDetails(t *testing.T) {
	model := &fakeModel{err: errors.New("AccessDeniedException: secret account detail")}
	f := newFixtureWith(t, suggest.NewHandler(true, model, 10))
	alice := f.newUser("Alice")

	status, body := f.suggestUpload(alice, pngBytes)
	if status != http.StatusBadGateway {
		t.Errorf("model failure: status %d, want 502", status)
	}
	if strings.Contains(string(body), "AccessDenied") {
		t.Errorf("error details leaked to the browser: %s", body)
	}
}
