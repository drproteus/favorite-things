package server_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

// uploadBanner sends an image to a folder's banner endpoint. It needs MinIO running, as the other image tests do.
func (f *fixture) uploadBanner(a *account, folderID string, data []byte) (int, []byte) {
	f.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "banner.png")
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		f.t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		f.t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, f.url+"/api/collections/"+folderID+"/banner", &buf)
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

func (f *fixture) folderFromList(a *account, id string) map[string]any {
	f.t.Helper()
	return f.folderState(a, id)
}

func TestFolderDescriptionIsSavedAndLimited(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	folder := f.folder(alice, "Shelf", f.root(alice))

	status, body := f.do(alice, http.MethodPut, "/api/collections/"+folder, map[string]any{
		"name": "Shelf", "visibility": "private", "parentId": f.root(alice), "description": "  Retro **handhelds**  ",
	})
	if status != http.StatusOK {
		t.Fatalf("update description: %d %s", status, body)
	}
	if got := decodeMap(t, body)["description"]; got != "Retro **handhelds**" {
		t.Errorf("description = %q", got)
	}
	if got := f.folderFromList(alice, folder)["description"]; got != "Retro **handhelds**" {
		t.Errorf("description not kept in the folder list: %v", got)
	}

	status, body = f.do(alice, http.MethodPut, "/api/collections/"+folder, map[string]any{
		"name": "Shelf", "visibility": "private", "parentId": f.root(alice), "description": strings.Repeat("a", 2001),
	})
	expectRejected(t, "description too long", status, body)
}

func TestFolderBannerUploadAndRemove(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	folder := f.folder(alice, "Shelf", f.root(alice))

	if got := f.folderFromList(alice, folder)["bannerUrl"]; got != nil {
		t.Fatalf("new folder has a banner: %v", got)
	}

	status, body := f.uploadBanner(alice, folder, pngBytes)
	if status != http.StatusOK {
		t.Fatalf("upload banner: %d %s", status, body)
	}
	url, ok := decodeMap(t, body)["bannerUrl"].(string)
	if !ok || !strings.Contains(url, "collections/") {
		t.Fatalf("banner URL missing or unexpected: %v", decodeMap(t, body)["bannerUrl"])
	}
	if got := f.folderFromList(alice, folder)["bannerUrl"]; got == nil {
		t.Error("banner not shown in the folder list after upload")
	}

	if status, body := f.do(alice, http.MethodDelete, "/api/collections/"+folder+"/banner", nil); status != http.StatusNoContent {
		t.Fatalf("remove banner: %d %s", status, body)
	}
	if got := f.folderFromList(alice, folder)["bannerUrl"]; got != nil {
		t.Errorf("banner still present after removal: %v", got)
	}
}

func TestFolderBannerRules(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	bob := f.newUser("Bob")
	folder := f.folder(alice, "Shelf", f.root(alice))

	if status, body := f.uploadBanner(alice, f.root(alice), pngBytes); status != http.StatusBadRequest {
		t.Errorf("banner on the root folder: %d %s, want 400", status, body)
	}
	if status, body := f.uploadBanner(bob, folder, pngBytes); status != http.StatusNotFound {
		t.Errorf("another user's banner upload: %d %s, want 404", status, body)
	}
	if status, body := f.uploadBanner(alice, folder, []byte("plain text, not an image")); status != http.StatusBadRequest {
		t.Errorf("non-image banner: %d %s, want 400", status, body)
	}
	if status, _ := f.uploadBanner(nil, folder, pngBytes); status != http.StatusUnauthorized {
		t.Errorf("anonymous banner upload: %d, want 401", status)
	}
}
