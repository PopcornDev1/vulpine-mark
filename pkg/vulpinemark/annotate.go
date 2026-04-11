package vulpinemark

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// roleColor returns the badge color for a given element role.
func roleColor(role string) color.RGBA {
	switch role {
	case "button":
		return color.RGBA{R: 34, G: 197, B: 94, A: 255} // green
	case "link":
		return color.RGBA{R: 59, G: 130, B: 246, A: 255} // blue
	case "input", "textarea":
		return color.RGBA{R: 168, G: 85, B: 247, A: 255} // purple
	case "select":
		return color.RGBA{R: 249, G: 115, B: 22, A: 255} // orange
	case "checkbox", "radio", "switch":
		return color.RGBA{R: 236, G: 72, B: 153, A: 255} // pink
	default:
		return color.RGBA{R: 100, G: 116, B: 139, A: 255} // slate
	}
}

// drawAnnotations decodes pngBytes, paints labeled badges over each element,
// and re-encodes to PNG. element coords are CSS pixels and are scaled to
// screenshot pixels by scale.
func drawAnnotations(pngBytes []byte, elements []Element, scale float64) ([]byte, error) {
	return drawAnnotationsWithClusters(pngBytes, elements, nil, scale)
}

// clusterColor is the palette used for cluster outlines / badges. It is
// intentionally distinct from any roleColor so clustered groups stand
// out from individual elements.
var clusterColor = color.RGBA{R: 255, G: 200, B: 0, A: 255} // amber

// drawAnnotationsWithClusters decodes pngBytes, paints labeled badges
// over each ungrouped element and a single outline+badge per cluster,
// and re-encodes to PNG.
func drawAnnotationsWithClusters(pngBytes []byte, elements []Element, clusters []Cluster, scale float64) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, fmt.Errorf("decode screenshot png: %w", err)
	}
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)

	face := basicfont.Face7x13

	for _, el := range elements {
		low := el.Confidence > 0 && el.Confidence < 0.3
		drawElementBadge(dst, el, scale, face, low)
	}

	for _, cl := range clusters {
		drawClusterBadge(dst, cl, scale, face)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, fmt.Errorf("encode annotated png: %w", err)
	}
	return buf.Bytes(), nil
}

// drawElementBadge renders a single element's border + label. When
// lowConfidence is true the badge is muted toward gray to signal
// low-confidence grounding.
func drawElementBadge(dst *image.RGBA, el Element, scale float64, face font.Face, lowConfidence bool) {
	bx := int(el.X * scale)
	by := int(el.Y * scale)
	bw := int(el.Width * scale)
	bh := int(el.Height * scale)
	if bw <= 0 || bh <= 0 {
		return
	}

	clr := roleColor(el.Role)
	if lowConfidence {
		clr = fadeToGray(clr)
	}

	drawRect(dst, image.Rect(bx, by, bx+bw, by+bh), color.RGBA{R: clr.R, G: clr.G, B: clr.B, A: 200}, 2)

	label := el.Label
	textW := font.MeasureString(face, label).Round()
	badgeW := textW + 8
	badgeH := 16
	badgeX := bx
	badgeY := by - badgeH
	if badgeY < 0 {
		badgeY = by
	}

	drawFilledRect(dst, image.Rect(badgeX, badgeY, badgeX+badgeW, badgeY+badgeH), clr)

	drawer := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(badgeX + 4),
			Y: fixed.I(badgeY + badgeH - 4),
		},
	}
	drawer.DrawString(label)
}

// drawClusterBadge renders a cluster outline + single "@N [1..count]"
// badge at the top-left of the first member.
func drawClusterBadge(dst *image.RGBA, cl Cluster, scale float64, face font.Face) {
	if len(cl.Members) == 0 {
		return
	}
	bbox := clusterBBox(cl.Members, scale)
	if bbox.Dx() <= 0 || bbox.Dy() <= 0 {
		return
	}

	// Dashed-style cluster outline in amber.
	drawRect(dst, bbox, clusterColor, 3)

	// Single badge at the top-left of the first member (reading order).
	first := cl.Members[0]
	bx := int(first.X * scale)
	by := int(first.Y * scale)

	label := fmt.Sprintf("%s[1..%d]", cl.Label, len(cl.Members))
	textW := font.MeasureString(face, label).Round()
	badgeW := textW + 8
	badgeH := 16
	badgeX := bx
	badgeY := by - badgeH
	if badgeY < 0 {
		badgeY = by
	}

	drawFilledRect(dst, image.Rect(badgeX, badgeY, badgeX+badgeW, badgeY+badgeH), clusterColor)

	drawer := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(badgeX + 4),
			Y: fixed.I(badgeY + badgeH - 4),
		},
	}
	drawer.DrawString(label)
}

// fadeToGray blends the given color halfway toward a neutral slate gray,
// used for low-confidence labels.
func fadeToGray(c color.RGBA) color.RGBA {
	const gr, gg, gb = 148, 163, 184 // slate-400
	return color.RGBA{
		R: uint8((int(c.R) + gr) / 2),
		G: uint8((int(c.G) + gg) / 2),
		B: uint8((int(c.B) + gb) / 2),
		A: c.A,
	}
}

// drawRect paints a hollow rectangle of the given thickness.
func drawRect(dst *image.RGBA, r image.Rectangle, c color.Color, thickness int) {
	for t := 0; t < thickness; t++ {
		// top
		drawHLine(dst, r.Min.X, r.Max.X, r.Min.Y+t, c)
		// bottom
		drawHLine(dst, r.Min.X, r.Max.X, r.Max.Y-1-t, c)
		// left
		drawVLine(dst, r.Min.X+t, r.Min.Y, r.Max.Y, c)
		// right
		drawVLine(dst, r.Max.X-1-t, r.Min.Y, r.Max.Y, c)
	}
}

func drawFilledRect(dst *image.RGBA, r image.Rectangle, c color.Color) {
	draw.Draw(dst, r, &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func drawHLine(dst *image.RGBA, x0, x1, y int, c color.Color) {
	if y < dst.Rect.Min.Y || y >= dst.Rect.Max.Y {
		return
	}
	if x0 < dst.Rect.Min.X {
		x0 = dst.Rect.Min.X
	}
	if x1 > dst.Rect.Max.X {
		x1 = dst.Rect.Max.X
	}
	for x := x0; x < x1; x++ {
		dst.Set(x, y, c)
	}
}

func drawVLine(dst *image.RGBA, x, y0, y1 int, c color.Color) {
	if x < dst.Rect.Min.X || x >= dst.Rect.Max.X {
		return
	}
	if y0 < dst.Rect.Min.Y {
		y0 = dst.Rect.Min.Y
	}
	if y1 > dst.Rect.Max.Y {
		y1 = dst.Rect.Max.Y
	}
	for y := y0; y < y1; y++ {
		dst.Set(x, y, c)
	}
}

// labelFor returns "@N" for the given index.
func labelFor(i int) string {
	return "@" + strconv.Itoa(i+1)
}
