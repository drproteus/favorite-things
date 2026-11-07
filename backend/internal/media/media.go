// Package media turns an uploaded image into a set of smaller, web-friendly variants.
// The original is kept for compatibility and for formats that can't safely be resized,
// such as animated GIFs.
package media

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"path"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// TargetWidths are the widths generated for every upload. Images are never upscaled;
// a final variant at the image's own width is added when it is narrower than the
// largest target so every image has an efficient version to serve.
var TargetWidths = []int{320, 640, 960, 1280, 1920}

const (
	// MaxPixels keeps a decompression bomb from allocating an unbounded amount of
	// memory. Larger uploads are stored as-is without variants.
	MaxPixels   = 50_000_000
	jpegQuality = 82
)

// Variant is one generated size. Key and ContentType are stored with the image so
// the API can presign exactly the objects that exist. Data is only used while uploading.
type Variant struct {
	Key         string `json:"key"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	ContentType string `json:"contentType"`
	Data        []byte `json:"-"`
}

// Result describes an uploaded image and the variants generated for it.
type Result struct {
	Width    int
	Height   int
	Variants []Variant
}

// View is the client-facing description of a variant.
type View struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Keys returns the original key followed by the keys of every variant, for cleanup.
func Keys(key string, variants []Variant) []string {
	keys := make([]string, 0, len(variants)+1)
	keys = append(keys, key)
	for _, v := range variants {
		keys = append(keys, v.Key)
	}
	return keys
}

// MarshalVariants encodes variants for the database. It always produces a JSON
// array, never null, so SQL can safely expand the column.
func MarshalVariants(variants []Variant) ([]byte, error) {
	if variants == nil {
		variants = []Variant{}
	}
	return json.Marshal(variants)
}

// Process decodes data and generates resized versions of baseKey. JPEG and WebP
// uploads become JPEG variants; images with transparency become PNG variants.
// GIFs keep their original bytes because resizing would drop animation frames.
func Process(data []byte, baseKey, contentType string) (Result, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Result{}, fmt.Errorf("decode image config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return Result{}, errors.New("image has no dimensions")
	}

	// Phone photos often carry an EXIF orientation that browsers apply but Go's
	// decoders do not. Report rotated dimensions and bake the rotation into the
	// variants, which have no EXIF data of their own.
	orientation := 1
	if contentType == "image/jpeg" {
		orientation = jpegOrientation(data)
	}
	result := Result{Width: cfg.Width, Height: cfg.Height}
	if orientation >= 5 && orientation <= 8 {
		result.Width, result.Height = result.Height, result.Width
	}
	if contentType == "image/gif" || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return result, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Result{}, fmt.Errorf("decode image: %w", err)
	}
	img = applyOrientation(img, orientation)

	transparent := !isOpaque(img)
	for _, width := range targetWidths(result.Width) {
		height := int(math.Round(float64(result.Height) * float64(width) / float64(result.Width)))
		if height < 1 {
			height = 1
		}

		resized := image.NewNRGBA(image.Rect(0, 0, width, height))
		draw.CatmullRom.Scale(resized, resized.Bounds(), img, img.Bounds(), draw.Src, nil)

		var buf bytes.Buffer
		var ext, variantType string
		if transparent {
			ext, variantType = ".png", "image/png"
			if err := png.Encode(&buf, resized); err != nil {
				return Result{}, fmt.Errorf("encode %dpx png: %w", width, err)
			}
		} else {
			ext, variantType = ".jpg", "image/jpeg"
			if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: jpegQuality}); err != nil {
				return Result{}, fmt.Errorf("encode %dpx jpeg: %w", width, err)
			}
		}

		result.Variants = append(result.Variants, Variant{
			Key:         variantKey(baseKey, width, ext),
			Width:       width,
			Height:      height,
			ContentType: variantType,
			Data:        buf.Bytes(),
		})
	}
	return result, nil
}

// targetWidths lists the sizes to generate for an image. Images narrower than the
// smallest target get one variant at their own width.
func targetWidths(width int) []int {
	var widths []int
	for _, target := range TargetWidths {
		if target <= width {
			widths = append(widths, target)
		}
	}
	full := min(width, TargetWidths[len(TargetWidths)-1])
	if len(widths) == 0 || widths[len(widths)-1] < full {
		widths = append(widths, full)
	}
	return widths
}

// variantKey adds the width to the original key: photos/<id>/<uuid>-640w.jpg.
func variantKey(baseKey string, width int, ext string) string {
	stem := strings.TrimSuffix(baseKey, path.Ext(baseKey))
	return fmt.Sprintf("%s-%dw%s", stem, width, ext)
}

// jpegOrientation reads the EXIF orientation tag (1-8) from a JPEG, returning 1
// when it is missing or unreadable.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			i++
			continue
		}
		for i < len(data) && data[i] == 0xFF {
			i++
		}
		if i >= len(data) {
			break
		}
		marker := data[i]
		i++
		// Start of scan or end of image: no more metadata.
		if marker == 0xDA || marker == 0xD9 {
			break
		}
		// Standalone markers have no length field.
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			continue
		}
		if i+2 > len(data) {
			break
		}
		size := int(binary.BigEndian.Uint16(data[i : i+2]))
		if size < 2 || i+size > len(data) {
			break
		}
		if marker == 0xE1 && size > 8 && string(data[i+2:i+8]) == "Exif\x00\x00" {
			return exifOrientation(data[i+8 : i+size])
		}
		i += size
	}
	return 1
}

func exifOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return 1
	}
	offset := int(order.Uint32(tiff[4:8]))
	if offset < 8 || offset+2 > len(tiff) {
		return 1
	}
	entries := int(order.Uint16(tiff[offset : offset+2]))
	offset += 2
	for i := 0; i < entries; i++ {
		entry := offset + i*12
		if entry+12 > len(tiff) {
			return 1
		}
		if order.Uint16(tiff[entry:entry+2]) != 0x0112 {
			continue
		}
		if order.Uint16(tiff[entry+2:entry+4]) != 3 || order.Uint32(tiff[entry+4:entry+8]) < 1 {
			return 1
		}
		value := int(order.Uint16(tiff[entry+8 : entry+10]))
		if value >= 1 && value <= 8 {
			return value
		}
		return 1
	}
	return 1
}

// applyOrientation bakes an EXIF orientation into the pixels.
func applyOrientation(img image.Image, orientation int) image.Image {
	if orientation < 2 || orientation > 8 {
		return img
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	dstWidth, dstHeight := width, height
	if orientation >= 5 {
		dstWidth, dstHeight = height, width
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dstWidth, dstHeight))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var dx, dy int
			switch orientation {
			case 2: // mirror horizontally
				dx, dy = width-1-x, y
			case 3: // rotate 180°
				dx, dy = width-1-x, height-1-y
			case 4: // mirror vertically
				dx, dy = x, height-1-y
			case 5: // transpose
				dx, dy = y, x
			case 6: // rotate 90° clockwise
				dx, dy = height-1-y, x
			case 7: // transverse
				dx, dy = height-1-y, width-1-x
			case 8: // rotate 270° clockwise
				dx, dy = y, width-1-x
			}
			dst.Set(dx, dy, img.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return dst
}

// isOpaque reports whether every pixel is fully opaque. Most standard image types
// expose Opaque directly; the fallback handles custom decoders.
func isOpaque(img image.Image) bool {
	if opaque, ok := img.(interface{ Opaque() bool }); ok {
		return opaque.Opaque()
	}
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return false
			}
		}
	}
	return true
}
