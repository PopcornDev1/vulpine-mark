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
	src, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, fmt.Errorf("decode screenshot png: %w", err)
	}
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)

	face := basicfont.Face7x13

	for _, el := range elements {
		// Element box in screenshot pixels.
		bx := int(el.X * scale)
		by := int(el.Y * scale)
		bw := int(el.W * scale)
		bh := int(el.H * scale)
		if bw <= 0 || bh <= 0 {
			continue
		}

		clr := roleColor(el.Role)

		// Translucent border around the element.
		drawRect(dst, image.Rect(bx, by, bx+bw, by+bh), color.RGBA{R: clr.R, G: clr.G, B: clr.B, A: 200}, 2)

		// Badge sized to fit the label text.
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

		// White label text on the badge.
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

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, fmt.Errorf("encode annotated png: %w", err)
	}
	return buf.Bytes(), nil
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
