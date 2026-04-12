package vulpinemark

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"sort"
)

// AnnotateHeatmap captures the current viewport, enumerates visible
// interactive elements, and returns a Result whose Image is a heatmap-
// overlaid screenshot. Each element gets a translucent fill with alpha
// proportional to its importance score instead of a numbered badge.
//
// Importance is defined as confidence * log(area+1) and then
// normalized to [0,1] across the returned element set, so the single
// most prominent element on the page always reaches the palette's
// brightest fill even when absolute confidences are low.
//
// Unlike Annotate, no labels are drawn — the goal is visual triage at
// a glance ("where should the agent look first?"). The Elements /
// Labels fields are still populated so programmatic consumers can
// inspect the ordering.
func (m *Mark) AnnotateHeatmap(ctx context.Context) (*Result, error) {
	elements, err := m.c.enumerate(ctx, true)
	if err != nil {
		return nil, err
	}

	shot, err := m.c.captureScreenshot(ctx)
	if err != nil {
		return nil, err
	}

	_, _, scale, err := m.c.viewportSize(ctx)
	if err != nil {
		scale = 1.0
	}

	m.mu.Lock()
	maxEls := m.maxElements
	useStable := m.stableLabels
	filter := m.filter
	m.mu.Unlock()

	if filter != nil {
		kept := elements[:0]
		for _, e := range elements {
			if filter(e) {
				kept = append(kept, e)
			}
		}
		elements = kept
	}

	elements = applyMaxElements(elements, 0, maxEls)
	if useStable {
		assignStableLabels(elements)
	} else {
		for i := range elements {
			elements[i].Label = labelFor(i)
		}
	}

	annotated, err := drawHeatmap(shot, elements, scale, m.currentPalette())
	if err != nil {
		return nil, err
	}

	byLabel := make(map[string]Element, len(elements))
	labels := make([]string, 0, len(elements))
	for _, el := range elements {
		byLabel[el.Label] = el
		labels = append(labels, el.Label)
	}
	result := &Result{
		Image:    annotated,
		Elements: byLabel,
		Labels:   labels,
	}
	m.mu.Lock()
	m.lastResult = result
	m.mu.Unlock()
	return result, nil
}

// heatmapImportance computes the raw importance score for an element:
// confidence * log(area + 1). Exposed for testing.
func heatmapImportance(el Element) float64 {
	area := el.Width * el.Height
	if area < 0 {
		area = 0
	}
	return el.Confidence * math.Log(area+1)
}

// rankByImportance returns the input elements sorted by descending
// heatmap importance. Stable, so ties preserve document order.
func rankByImportance(els []Element) []Element {
	out := make([]Element, len(els))
	copy(out, els)
	sort.SliceStable(out, func(i, j int) bool {
		return heatmapImportance(out[i]) > heatmapImportance(out[j])
	})
	return out
}

// drawHeatmap decodes pngBytes and paints translucent role-colored
// fills over each element, alpha proportional to normalized importance.
// Used by AnnotateHeatmap and exercised directly in tests.
func drawHeatmap(pngBytes []byte, elements []Element, scale float64, palette Palette) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, fmt.Errorf("decode screenshot png: %w", err)
	}
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)

	if len(elements) == 0 {
		var buf bytes.Buffer
		if err := png.Encode(&buf, dst); err != nil {
			return nil, fmt.Errorf("encode heatmap png: %w", err)
		}
		return buf.Bytes(), nil
	}

	// Normalize importance to [0,1] across the set. Use max so the
	// single most important element saturates the palette.
	var maxScore float64
	for _, el := range elements {
		s := heatmapImportance(el)
		if s > maxScore {
			maxScore = s
		}
	}
	if maxScore <= 0 {
		maxScore = 1
	}

	for _, el := range elements {
		bx := int(el.X * scale)
		by := int(el.Y * scale)
		bw := int(el.Width * scale)
		bh := int(el.Height * scale)
		if bw <= 0 || bh <= 0 {
			continue
		}
		score := heatmapImportance(el) / maxScore
		if score < 0 {
			score = 0
		}
		if score > 1 {
			score = 1
		}
		// Minimum alpha so low-importance elements remain visible,
		// maximum so the screenshot underneath still shows through.
		alpha := uint8(40 + score*(200-40))
		base := palette.ColorFor(el.Role)
		fill := color.RGBA{R: base.R, G: base.G, B: base.B, A: alpha}
		rect := image.Rect(bx, by, bx+bw, by+bh).Intersect(bounds)
		if rect.Empty() {
			continue
		}
		drawAlphaFill(dst, rect, fill)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, fmt.Errorf("encode heatmap png: %w", err)
	}
	return buf.Bytes(), nil
}

// drawAlphaFill blends a translucent RGBA over dst in rect, using
// standard "over" compositing (src alpha blended into existing pixels).
func drawAlphaFill(dst *image.RGBA, rect image.Rectangle, c color.RGBA) {
	a := float64(c.A) / 255.0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			off := dst.PixOffset(x, y)
			dr := float64(dst.Pix[off+0])
			dg := float64(dst.Pix[off+1])
			db := float64(dst.Pix[off+2])
			dst.Pix[off+0] = uint8(float64(c.R)*a + dr*(1-a))
			dst.Pix[off+1] = uint8(float64(c.G)*a + dg*(1-a))
			dst.Pix[off+2] = uint8(float64(c.B)*a + db*(1-a))
			dst.Pix[off+3] = 255
		}
	}
}
