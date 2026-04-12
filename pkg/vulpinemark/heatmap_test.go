package vulpinemark

import (
	"bytes"
	"image/png"
	"testing"
)

func TestHeatmapImportanceOrdering(t *testing.T) {
	// Three elements: large high-confidence, medium high-confidence,
	// small high-confidence. Importance should strictly decrease.
	big := Element{Role: "button", X: 0, Y: 0, Width: 400, Height: 200, Confidence: 0.9}
	med := Element{Role: "link", X: 0, Y: 300, Width: 100, Height: 40, Confidence: 0.9}
	small := Element{Role: "input", X: 0, Y: 500, Width: 20, Height: 10, Confidence: 0.9}

	if !(heatmapImportance(big) > heatmapImportance(med)) {
		t.Errorf("big importance %.3f should exceed med %.3f",
			heatmapImportance(big), heatmapImportance(med))
	}
	if !(heatmapImportance(med) > heatmapImportance(small)) {
		t.Errorf("med importance %.3f should exceed small %.3f",
			heatmapImportance(med), heatmapImportance(small))
	}

	// rankByImportance should return elements in descending order.
	ranked := rankByImportance([]Element{small, big, med})
	if ranked[0].Width != big.Width {
		t.Errorf("ranked[0] should be big, got w=%.0f", ranked[0].Width)
	}
	if ranked[2].Width != small.Width {
		t.Errorf("ranked[2] should be small, got w=%.0f", ranked[2].Width)
	}

	// Confidence zero should nullify importance.
	invis := Element{Role: "button", X: 0, Y: 0, Width: 500, Height: 500, Confidence: 0}
	if heatmapImportance(invis) != 0 {
		t.Errorf("zero-confidence importance should be 0, got %.3f",
			heatmapImportance(invis))
	}

	// drawHeatmap should succeed and produce a valid PNG.
	shot := makeWhitePNG(t, 800, 600)
	out, err := drawHeatmap(shot, []Element{big, med, small}, 1.0, DefaultPalette)
	if err != nil {
		t.Fatalf("drawHeatmap: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("decode heatmap png: %v", err)
	}
	// Output should differ from the plain white input since we filled
	// translucent colors over it.
	if bytes.Equal(out, shot) {
		t.Errorf("heatmap output identical to input; expected overlay")
	}
}

func TestHeatmapEmptyElements(t *testing.T) {
	shot := makeWhitePNG(t, 100, 100)
	out, err := drawHeatmap(shot, nil, 1.0, DefaultPalette)
	if err != nil {
		t.Fatalf("drawHeatmap empty: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("decode empty heatmap png: %v", err)
	}
}
