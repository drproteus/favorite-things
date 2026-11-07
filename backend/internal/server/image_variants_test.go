package server_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

func jpegBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (f *fixture) uploadItemImage(a *account, itemID string, data []byte) (int, []byte) {
	f.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "photo.jpg")
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		f.t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		f.t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, f.url+"/api/items/"+itemID+"/images", &buf)
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

func TestItemImageUploadCreatesVariantsAndDeletesThem(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	item := f.item(alice, "Camera", f.root(alice), "private")

	status, body := f.uploadItemImage(alice, item, jpegBytes(t, 1200, 800))
	if status != http.StatusCreated {
		t.Fatalf("upload image: %d %s", status, body)
	}
	image := decodeMap(t, body)
	if got := image["width"]; got != float64(1200) {
		t.Errorf("image width = %v, want 1200", got)
	}
	variants, ok := image["variants"].([]any)
	if !ok || len(variants) != 4 {
		t.Fatalf("image variants = %v, want 4 sizes", image["variants"])
	}
	first := variants[0].(map[string]any)
	last := variants[len(variants)-1].(map[string]any)
	if first["width"] != float64(320) || last["width"] != float64(1200) {
		t.Errorf("variant widths = %v..%v, want 320..1200", first["width"], last["width"])
	}

	// The signed URL for a generated size must serve the stored object.
	variantURL := first["url"].(string)
	resp, err := http.Get(variantURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET variant = %d, want 200", resp.StatusCode)
	}

	// Variants are returned whenever the item is loaded, not only from the upload response.
	stored := f.itemState(alice, item)["images"].([]any)[0].(map[string]any)
	if got := len(stored["variants"].([]any)); got != 4 {
		t.Errorf("stored image has %d variants, want 4", got)
	}

	status, body = f.do(alice, http.MethodDelete, "/api/items/"+item+"/images/"+image["id"].(string), nil)
	if status != http.StatusNoContent {
		t.Fatalf("delete image: %d %s", status, body)
	}
	if got := f.itemState(alice, item)["images"].([]any); len(got) != 0 {
		t.Errorf("images after delete = %v, want none", got)
	}

	// Deleting the image must also remove its variants from storage.
	resp, err = http.Get(variantURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET deleted variant = %d, want 404", resp.StatusCode)
	}
}

func TestFolderBannerUploadCreatesVariants(t *testing.T) {
	f := newFixture(t)
	alice := f.newUser("Alice")
	folder := f.folder(alice, "Shelf", f.root(alice))

	status, body := f.uploadBanner(alice, folder, jpegBytes(t, 1200, 800))
	if status != http.StatusOK {
		t.Fatalf("upload banner: %d %s", status, body)
	}
	state := f.folderFromList(alice, folder)
	if got := state["bannerWidth"]; got != float64(1200) {
		t.Errorf("banner width = %v, want 1200", got)
	}
	variants, ok := state["bannerVariants"].([]any)
	if !ok || len(variants) != 4 {
		t.Fatalf("banner variants = %v, want 4 sizes", state["bannerVariants"])
	}
	if got := variants[0].(map[string]any)["width"]; got != float64(320) {
		t.Errorf("first banner variant width = %v, want 320", got)
	}
}
