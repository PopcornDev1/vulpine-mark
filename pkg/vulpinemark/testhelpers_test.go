package vulpinemark

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// makeWhitePNG returns the PNG bytes of a solid-white RGBA image with
// the given dimensions. Used by drawing smoke tests.
func makeWhitePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode white png: %v", err)
	}
	return buf.Bytes()
}
