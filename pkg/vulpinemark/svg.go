package vulpinemark

import (
	"context"
	"fmt"
	"image/color"
	"strings"
)

// renderSVGOverlay returns an SVG document drawing borders and labels
// for the given elements and clusters. Coordinates are CSS pixels
// multiplied by scale so the overlay composites directly onto a
// screenshot of the same dimensions. The SVG root has width/height
// set to the full screenshot size; a consumer can layer it over the
// raster image (e.g. as an <img> plus <svg> stacked in CSS) and
// toggle its visibility.
func renderSVGOverlay(elements []Element, clusters []Cluster, scale float64, width, height int, palette Palette) string {
	var b strings.Builder
	fmt.Fprintf(&b,
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" data-vulpine-mark="1">`,
		width, height, width, height)
	b.WriteString(`<style>.vm-label{font-family:monospace;font-size:12px;font-weight:bold}</style>`)

	for _, cl := range clusters {
		if len(cl.Members) == 0 {
			continue
		}
		bbox := clusterBBox(cl.Members, scale)
		if bbox.Dx() <= 0 || bbox.Dy() <= 0 {
			continue
		}
		stroke := rgb(palette.Cluster)
		fmt.Fprintf(&b,
			`<rect x="%d" y="%d" width="%d" height="%d" fill="none" stroke="%s" stroke-width="3" stroke-dasharray="6,4"/>`,
			bbox.Min.X, bbox.Min.Y, bbox.Dx(), bbox.Dy(), stroke)

		first := cl.Members[0]
		bx := int(first.X * scale)
		by := int(first.Y * scale)
		label := fmt.Sprintf("%s[1..%d]", cl.Label, len(cl.Members))
		writeLabelBadge(&b, bx, by, label, stroke, "#000")
	}

	for _, el := range elements {
		bx := int(el.X * scale)
		by := int(el.Y * scale)
		bw := int(el.Width * scale)
		bh := int(el.Height * scale)
		if bw <= 0 || bh <= 0 {
			continue
		}
		clr := palette.ColorFor(el.Role)
		if el.Confidence > 0 && el.Confidence < 0.3 {
			clr = fadeToGray(clr)
		}
		stroke := rgb(clr)
		fmt.Fprintf(&b,
			`<rect x="%d" y="%d" width="%d" height="%d" fill="none" stroke="%s" stroke-width="2" opacity="0.8"/>`,
			bx, by, bw, bh, stroke)
		writeLabelBadge(&b, bx, by, el.Label, stroke, "#fff")
	}

	b.WriteString(`</svg>`)
	return b.String()
}

// writeLabelBadge appends a filled rect + text label above the element.
// Approximates the raster badge layout: 7px-per-char width, 16px tall,
// text baseline offset by 4px from the badge bottom.
func writeLabelBadge(b *strings.Builder, bx, by int, label, fill, textColor string) {
	const badgeH = 16
	badgeW := len(label)*7 + 8
	badgeX := bx
	badgeY := by - badgeH
	if badgeY < 0 {
		badgeY = by
	}
	fmt.Fprintf(b,
		`<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`,
		badgeX, badgeY, badgeW, badgeH, fill)
	fmt.Fprintf(b,
		`<text class="vm-label" x="%d" y="%d" fill="%s">%s</text>`,
		badgeX+4, badgeY+badgeH-4, textColor, escapeXML(label))
}

func rgb(c color.RGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// AnnotateSVG is like Annotate but also populates Result.SVG with a
// vector overlay that can be composited over the raster screenshot.
// The raster PNG (Result.Image) is still produced so callers that want
// both formats don't have to pay for a second CDP round-trip.
func (m *Mark) AnnotateSVG(ctx context.Context) (*Result, error) {
	result, err := m.Annotate(ctx)
	if err != nil {
		return nil, err
	}
	w, h, scale, err := m.c.viewportSize(ctx)
	if err != nil {
		scale = 1.0
	}
	// viewportSize returns CSS px; multiply by scale to get screenshot px.
	width := int(w * scale)
	height := int(h * scale)

	// Rebuild the element slice from the result. Iterate via
	// result.Labels (document order) rather than the Elements map so
	// SVG z-order is deterministic across runs — map iteration in Go
	// is intentionally randomized and would otherwise produce a
	// different byte stream every call.
	elements := make([]Element, 0, len(result.Elements))
	for _, label := range result.Labels {
		el, ok := result.Elements[label]
		if !ok {
			// Cluster labels live in result.Labels but not in the
			// per-element map; they are rendered from result.Clusters.
			continue
		}
		elements = append(elements, el)
	}
	result.SVG = renderSVGOverlay(elements, result.Clusters, scale, width, height, m.currentPalette())
	m.mu.Lock()
	m.lastResult = result
	m.mu.Unlock()
	return result, nil
}
