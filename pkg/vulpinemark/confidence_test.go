package vulpinemark

import (
	"math"
	"testing"
)

func TestConfidenceScoreBounds(t *testing.T) {
	cases := []struct {
		name string
		in   confidenceSignals
		want float64
	}{
		{
			name: "zero signals",
			in:   confidenceSignals{Occluded: true, Clipped: true},
			want: 0.0,
		},
		{
			name: "all signals",
			in: confidenceSignals{
				HasName: true, HasAriaAttr: true, Area: 200,
				Occluded: false, Clipped: false,
			},
			want: 1.0,
		},
		{
			name: "name only, big area, visible, not clipped",
			in: confidenceSignals{
				HasName: true, Area: 500, Occluded: false, Clipped: false,
			},
			want: 0.8, // 0.3 + 0.2 + 0.2 + 0.1
		},
		{
			name: "occluded aria large",
			in: confidenceSignals{
				HasName: true, HasAriaAttr: true, Area: 5000,
				Occluded: true, Clipped: false,
			},
			want: 0.8, // 0.3+0.2+0.2+0.1 (no +0.2 for occluded)
		},
		{
			name: "tiny no name clipped",
			in: confidenceSignals{
				HasName: false, HasAriaAttr: false, Area: 50,
				Occluded: false, Clipped: true,
			},
			want: 0.2, // only +0.2 for not occluded
		},
		{
			name: "exactly area threshold (not greater)",
			in: confidenceSignals{
				Area: 100, Occluded: false, Clipped: false,
			},
			want: 0.3, // +0.2 not occluded, +0.1 not clipped (area NOT > 100)
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeConfidence(tc.in)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("computeConfidence(%+v) = %v, want %v", tc.in, got, tc.want)
			}
			if got < 0 || got > 1 {
				t.Errorf("score %v out of [0,1]", got)
			}
		})
	}
}

func TestConfidenceFadeToGrayDrawing(t *testing.T) {
	// Smoke test: a low-confidence element should still render without
	// error through the normal drawing pipeline.
	src := makeWhitePNG(t, 60, 60)
	elements := []Element{
		{Label: "@1", Role: "button", Text: "x", X: 10, Y: 10, Width: 30, Height: 20, Confidence: 0.1},
	}
	if _, err := drawAnnotations(src, elements, 1.0); err != nil {
		t.Fatalf("drawAnnotations low-confidence: %v", err)
	}
}
