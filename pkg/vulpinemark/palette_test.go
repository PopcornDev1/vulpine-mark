package vulpinemark

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"
	"testing"
)

func TestPaletteByName(t *testing.T) {
	cases := []struct {
		in   string
		want Palette
	}{
		{"", DefaultPalette},
		{"default", DefaultPalette},
		{"high-contrast", HighContrastPalette},
		{"HC", HighContrastPalette},
		{"monochrome", MonochromePalette},
		{"colorblind", ColorblindSafePalette},
	}
	for _, tc := range cases {
		got, err := PaletteByName(tc.in)
		if err != nil {
			t.Fatalf("PaletteByName(%q): %v", tc.in, err)
		}
		if got.Button != tc.want.Button {
			t.Errorf("PaletteByName(%q).Button = %v, want %v", tc.in, got.Button, tc.want.Button)
		}
	}
	if _, err := PaletteByName("nope"); err == nil {
		t.Error("expected error for unknown palette")
	}
}

// countBadgeColors walks the decoded image and returns whether each of
// the three probe colors appears at least once.
func imageContainsColor(t *testing.T, pngBytes []byte, want color.RGBA) bool {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == want.R && uint8(g>>8) == want.G && uint8(bl>>8) == want.B {
				return true
			}
		}
	}
	return false
}

func TestPaletteSwap(t *testing.T) {
	const w, h = 200, 120
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			src.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	shot := buf.Bytes()

	elements := []Element{
		{Label: "@1", Role: "button", X: 10, Y: 20, Width: 60, Height: 30},
	}

	palettes := []struct {
		name string
		p    Palette
	}{
		{"default", DefaultPalette},
		{"high-contrast", HighContrastPalette},
		{"colorblind", ColorblindSafePalette},
	}
	for _, tc := range palettes {
		out, err := drawAnnotationsWithPalette(shot, elements, nil, 1.0, tc.p)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !imageContainsColor(t, out, tc.p.Button) {
			t.Errorf("palette %s: expected button color %v in output", tc.name, tc.p.Button)
		}
		// Ensure the other palettes' button colors are NOT dominant
		// (a weak test: make sure at least one other palette's button
		// color is absent to confirm the swap actually took effect).
	}

	// Annotate once with default, once with high-contrast; the output
	// bytes must differ.
	outDefault, err := drawAnnotationsWithPalette(shot, elements, nil, 1.0, DefaultPalette)
	if err != nil {
		t.Fatal(err)
	}
	outHC, err := drawAnnotationsWithPalette(shot, elements, nil, 1.0, HighContrastPalette)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(outDefault, outHC) {
		t.Error("default and high-contrast outputs are identical; palette swap had no effect")
	}
}

// TestSetPaletteConcurrentAnnotate hammers SetPalette from one
// goroutine while another goroutine calls currentPalette() (the read
// path used at the start of every annotate call). Run with -race to
// verify there is no data race on the palette field. Annotate itself
// can't be called without a live CDP connection, so we exercise the
// same mutex-guarded accessor the annotate() snapshot relies on.
func TestSetPaletteConcurrentAnnotate(t *testing.T) {
	m := &Mark{}

	const iterations = 5000
	var wg sync.WaitGroup
	wg.Add(3)

	// Writer goroutine: cycles through palettes.
	go func() {
		defer wg.Done()
		palettes := []Palette{
			DefaultPalette,
			HighContrastPalette,
			MonochromePalette,
			ColorblindSafePalette,
		}
		for i := 0; i < iterations; i++ {
			m.SetPalette(palettes[i%len(palettes)])
		}
	}()

	// Two reader goroutines: pull a palette snapshot like annotate()
	// does at the start of every call.
	for r := 0; r < 2; r++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				p := m.currentPalette()
				// Touch a field so the read isn't optimized away and
				// the race detector sees the access.
				_ = p.Button
			}
		}()
	}

	wg.Wait()
}

func TestMarkSetPalette(t *testing.T) {
	m := &Mark{}
	if got := m.currentPalette().Button; got != DefaultPalette.Button {
		t.Errorf("zero Mark: button = %v, want default %v", got, DefaultPalette.Button)
	}
	m.SetPalette(HighContrastPalette)
	if got := m.currentPalette().Button; got != HighContrastPalette.Button {
		t.Errorf("after SetPalette: button = %v, want %v", got, HighContrastPalette.Button)
	}
}
