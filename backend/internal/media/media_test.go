package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestProcessGeneratesWidthsWithoutUpscaling(t *testing.T) {
	data := encodeJPEG(t, 1200, 800)

	result, err := Process(data, "items/1/photo.jpg", "image/jpeg")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if result.Width != 1200 || result.Height != 800 {
		t.Errorf("original size = %dx%d, want 1200x800", result.Width, result.Height)
	}

	wantWidths := []int{320, 640, 960, 1200}
	if len(result.Variants) != len(wantWidths) {
		t.Fatalf("got %d variants, want %d: %+v", len(result.Variants), len(wantWidths), result.Variants)
	}
	for i, want := range wantWidths {
		v := result.Variants[i]
		if v.Width != want {
			t.Errorf("variant %d width = %d, want %d", i, v.Width, want)
		}
		wantHeight := int(math.Round(float64(want) * 800 / 1200))
		if v.Height != wantHeight {
			t.Errorf("variant %d height = %d, want %d", i, v.Height, wantHeight)
		}
		if v.ContentType != "image/jpeg" {
			t.Errorf("variant %d type = %q, want image/jpeg", i, v.ContentType)
		}
		if !strings.HasSuffix(v.Key, "-"+strconv.Itoa(want)+"w.jpg") {
			t.Errorf("variant %d key = %q", i, v.Key)
		}
		if v.Data == nil {
			t.Errorf("variant %d has no data", i)
		}
		decoded, err := jpeg.Decode(bytes.NewReader(v.Data))
		if err != nil {
			t.Fatalf("decode variant %d: %v", i, err)
		}
		if got := decoded.Bounds().Dx(); got != want {
			t.Errorf("decoded variant %d width = %d, want %d", i, got, want)
		}
	}
}

func TestProcessKeepsTransparency(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 400, 200))
	for y := range 200 {
		for x := range 200 {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	var input bytes.Buffer
	if err := png.Encode(&input, img); err != nil {
		t.Fatal(err)
	}

	result, err := Process(input.Bytes(), "collections/1/banner.png", "image/png")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(result.Variants) != 2 {
		t.Fatalf("got %d variants, want 2", len(result.Variants))
	}
	// The right half is transparent, so variants must stay PNG.
	v := result.Variants[0]
	if v.ContentType != "image/png" || !strings.HasSuffix(v.Key, "-320w.png") {
		t.Fatalf("variant = %+v, want a 320px PNG", v)
	}
	decoded, err := png.Decode(bytes.NewReader(v.Data))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, a := decoded.At(300, 100).RGBA(); a != 0 {
		t.Errorf("transparent pixel has alpha %d, want 0", a)
	}
}

func TestProcessAppliesEXIFOrientation(t *testing.T) {
	data := withOrientation(encodeJPEG(t, 400, 200), 6)

	result, err := Process(data, "items/1/photo.jpg", "image/jpeg")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if result.Width != 200 || result.Height != 400 {
		t.Fatalf("oriented size = %dx%d, want 200x400", result.Width, result.Height)
	}
	if len(result.Variants) != 1 {
		t.Fatalf("got %d variants, want 1", len(result.Variants))
	}
	v := result.Variants[0]
	if v.Width != 200 || v.Height != 400 {
		t.Errorf("variant size = %dx%d, want 200x400", v.Width, v.Height)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(v.Data))
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded.Bounds(); got.Dx() != 200 || got.Dy() != 400 {
		t.Errorf("decoded variant bounds = %v, want 200x400", got)
	}
}

func TestProcessSkipsGIFVariants(t *testing.T) {
	img := image.NewPaletted(image.Rect(0, 0, 800, 600), color.Palette{color.Black, color.White})
	var input bytes.Buffer
	if err := gif.Encode(&input, img, nil); err != nil {
		t.Fatal(err)
	}

	result, err := Process(input.Bytes(), "items/1/photo.gif", "image/gif")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if result.Width != 800 || result.Height != 600 {
		t.Errorf("size = %dx%d, want 800x600", result.Width, result.Height)
	}
	if len(result.Variants) != 0 {
		t.Errorf("GIF got %d variants, want none to preserve animation", len(result.Variants))
	}
}

func TestProcessReturnsErrorForUndecodableData(t *testing.T) {
	_, err := Process([]byte("\x89PNG\r\n\x1a\nnot really a png"), "items/1/photo.png", "image/png")
	if err == nil {
		t.Fatal("Process accepted invalid image data")
	}
}

func TestTargetWidths(t *testing.T) {
	tests := []struct {
		width int
		want  []int
	}{
		{width: 100, want: []int{100}},
		{width: 320, want: []int{320}},
		{width: 1000, want: []int{320, 640, 960, 1000}},
		{width: 4000, want: []int{320, 640, 960, 1280, 1920}},
	}
	for _, tt := range tests {
		got := targetWidths(tt.width)
		if len(got) != len(tt.want) {
			t.Errorf("targetWidths(%d) = %v, want %v", tt.width, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("targetWidths(%d) = %v, want %v", tt.width, got, tt.want)
				break
			}
		}
	}
}

func TestMarshalVariantsIsAlwaysAnArray(t *testing.T) {
	data, err := MarshalVariants(nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "[]" {
		t.Errorf("MarshalVariants(nil) = %s, want []", data)
	}
}

// withOrientation inserts a minimal EXIF APP1 segment with the given orientation.
func withOrientation(data []byte, orientation uint16) []byte {
	tiff := make([]byte, 26)
	copy(tiff[:2], "II")
	binary.LittleEndian.PutUint16(tiff[2:4], 42)
	binary.LittleEndian.PutUint32(tiff[4:8], 8)
	binary.LittleEndian.PutUint16(tiff[8:10], 1) // one IFD entry
	binary.LittleEndian.PutUint16(tiff[10:12], 0x0112)
	binary.LittleEndian.PutUint16(tiff[12:14], 3) // SHORT
	binary.LittleEndian.PutUint32(tiff[14:18], 1)
	binary.LittleEndian.PutUint16(tiff[18:20], orientation)

	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:4], uint16(len(payload)+2))
	segment = append(segment, payload...)

	out := make([]byte, 0, len(data)+len(segment))
	out = append(out, data[:2]...)
	out = append(out, segment...)
	out = append(out, data[2:]...)
	return out
}

func encodeJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
