package vulpinemark

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestRoleColor(t *testing.T) {
	cases := []struct {
		role string
		want color.RGBA
	}{
		{"button", color.RGBA{R: 34, G: 197, B: 94, A: 255}},
		{"link", color.RGBA{R: 59, G: 130, B: 246, A: 255}},
		{"input", color.RGBA{R: 168, G: 85, B: 247, A: 255}},
		{"textarea", color.RGBA{R: 168, G: 85, B: 247, A: 255}},
		{"select", color.RGBA{R: 249, G: 115, B: 22, A: 255}},
		{"checkbox", color.RGBA{R: 236, G: 72, B: 153, A: 255}},
		{"radio", color.RGBA{R: 236, G: 72, B: 153, A: 255}},
		{"switch", color.RGBA{R: 236, G: 72, B: 153, A: 255}},
		{"", color.RGBA{R: 100, G: 116, B: 139, A: 255}},
		{"unknown", color.RGBA{R: 100, G: 116, B: 139, A: 255}},
	}
	for _, tc := range cases {
		got := roleColor(tc.role)
		if got != tc.want {
			t.Errorf("roleColor(%q) = %+v, want %+v", tc.role, got, tc.want)
		}
	}
}

func TestLabelFor(t *testing.T) {
	cases := []struct {
		idx  int
		want string
	}{
		{0, "@1"},
		{1, "@2"},
		{9, "@10"},
		{99, "@100"},
	}
	for _, tc := range cases {
		if got := labelFor(tc.idx); got != tc.want {
			t.Errorf("labelFor(%d) = %q, want %q", tc.idx, got, tc.want)
		}
	}
}

func TestDrawAnnotations_smoke(t *testing.T) {
	// Build a tiny 100x80 white PNG in-memory.
	const w, h = 100, 80
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			src.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode source png: %v", err)
	}

	elements := []Element{
		{Label: "@1", Tag: "button", Role: "button", Text: "OK", X: 10, Y: 10, W: 40, H: 20},
	}
	out, err := drawAnnotations(buf.Bytes(), elements, 1.0)
	if err != nil {
		t.Fatalf("drawAnnotations: %v", err)
	}
	dec, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode annotated png: %v", err)
	}
	gotW := dec.Bounds().Dx()
	gotH := dec.Bounds().Dy()
	if gotW != w || gotH != h {
		t.Errorf("annotated dims = %dx%d, want %dx%d", gotW, gotH, w, h)
	}
}

func TestDrawAnnotations_emptyElements(t *testing.T) {
	// Ensure zero-size elements are skipped cleanly.
	src := image.NewRGBA(image.Rect(0, 0, 20, 20))
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode: %v", err)
	}
	elements := []Element{
		{Label: "@1", Role: "button", X: 0, Y: 0, W: 0, H: 0},
	}
	if _, err := drawAnnotations(buf.Bytes(), elements, 1.0); err != nil {
		t.Fatalf("drawAnnotations with zero-size element: %v", err)
	}
}

func TestStripDataURI(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain base64", "iVBORw0KGgo=", "iVBORw0KGgo="},
		{"data png", "data:image/png;base64,iVBORw0KGgo=", "iVBORw0KGgo="},
		{"data plain", "data:text/plain,hello", "hello"},
		{"empty", "", ""},
		{"no comma no prefix", "notdata", "notdata"},
		{"comma but no prefix", "foo,bar", "foo,bar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripDataURI(tc.in); got != tc.want {
				t.Errorf("stripDataURI(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestElementCenter(t *testing.T) {
	e := Element{X: 10, Y: 20, W: 40, H: 60}
	cx, cy := e.center()
	if cx != 30 || cy != 50 {
		t.Errorf("center() = (%v,%v), want (30,50)", cx, cy)
	}
}
