package vulpinemark

import (
	"fmt"
	"image/color"
	"strings"
)

// Palette maps element roles to badge colors. Use SetPalette on a Mark
// to switch between the built-in palettes (DefaultPalette,
// HighContrastPalette, MonochromePalette, ColorblindSafePalette) or
// supply your own.
type Palette struct {
	Button   color.RGBA
	Link     color.RGBA
	Input    color.RGBA
	Select   color.RGBA
	Checkbox color.RGBA
	Cluster  color.RGBA
	Other    color.RGBA
}

// DefaultPalette matches the original vulpine-mark color scheme.
var DefaultPalette = Palette{
	Button:   color.RGBA{R: 34, G: 197, B: 94, A: 255},   // green
	Link:     color.RGBA{R: 59, G: 130, B: 246, A: 255},  // blue
	Input:    color.RGBA{R: 168, G: 85, B: 247, A: 255},  // purple
	Select:   color.RGBA{R: 249, G: 115, B: 22, A: 255},  // orange
	Checkbox: color.RGBA{R: 236, G: 72, B: 153, A: 255},  // pink
	Cluster:  color.RGBA{R: 255, G: 200, B: 0, A: 255},   // amber
	Other:    color.RGBA{R: 100, G: 116, B: 139, A: 255}, // slate
}

// HighContrastPalette uses bold primary colors for maximum legibility
// on busy pages or for users with low vision.
var HighContrastPalette = Palette{
	Button:   color.RGBA{R: 0, G: 200, B: 0, A: 255},
	Link:     color.RGBA{R: 0, G: 80, B: 255, A: 255},
	Input:    color.RGBA{R: 180, G: 0, B: 220, A: 255},
	Select:   color.RGBA{R: 255, G: 120, B: 0, A: 255},
	Checkbox: color.RGBA{R: 220, G: 0, B: 120, A: 255},
	Cluster:  color.RGBA{R: 255, G: 215, B: 0, A: 255},
	Other:    color.RGBA{R: 0, G: 0, B: 0, A: 255},
}

// MonochromePalette is a grayscale palette, useful for print output or
// low-color displays.
var MonochromePalette = Palette{
	Button:   color.RGBA{R: 40, G: 40, B: 40, A: 255},
	Link:     color.RGBA{R: 80, G: 80, B: 80, A: 255},
	Input:    color.RGBA{R: 110, G: 110, B: 110, A: 255},
	Select:   color.RGBA{R: 140, G: 140, B: 140, A: 255},
	Checkbox: color.RGBA{R: 170, G: 170, B: 170, A: 255},
	Cluster:  color.RGBA{R: 20, G: 20, B: 20, A: 255},
	Other:    color.RGBA{R: 100, G: 100, B: 100, A: 255},
}

// ColorblindSafePalette uses the Wong colorblind-safe palette
// (Nature Methods 2011) so red-green and blue-yellow deficiencies
// still distinguish every role.
var ColorblindSafePalette = Palette{
	Button:   color.RGBA{R: 0, G: 158, B: 115, A: 255},   // bluish green
	Link:     color.RGBA{R: 0, G: 114, B: 178, A: 255},   // blue
	Input:    color.RGBA{R: 204, G: 121, B: 167, A: 255}, // reddish purple
	Select:   color.RGBA{R: 230, G: 159, B: 0, A: 255},   // orange
	Checkbox: color.RGBA{R: 213, G: 94, B: 0, A: 255},    // vermillion
	Cluster:  color.RGBA{R: 240, G: 228, B: 66, A: 255},  // yellow
	Other:    color.RGBA{R: 86, G: 180, B: 233, A: 255},  // sky blue
}

// ColorFor returns the palette color for the given element role.
func (p Palette) ColorFor(role string) color.RGBA {
	switch role {
	case "button":
		return p.Button
	case "link":
		return p.Link
	case "input", "textarea":
		return p.Input
	case "select":
		return p.Select
	case "checkbox", "radio", "switch":
		return p.Checkbox
	default:
		return p.Other
	}
}

// SetPalette replaces the palette used for subsequent annotate calls.
// Safe to call concurrently with in-flight Annotate calls.
func (m *Mark) SetPalette(p Palette) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.palette = p
	m.paletteSet = true
}

// currentPalette returns the configured palette, or DefaultPalette if
// none has been set.
func (m *Mark) currentPalette() Palette {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.paletteSet {
		return DefaultPalette
	}
	return m.palette
}

// PaletteByName returns a built-in palette by its CLI-facing name.
// Known names: "default", "high-contrast", "monochrome", "colorblind".
func PaletteByName(name string) (Palette, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "default":
		return DefaultPalette, nil
	case "high-contrast", "highcontrast", "hc":
		return HighContrastPalette, nil
	case "monochrome", "mono", "grayscale":
		return MonochromePalette, nil
	case "colorblind", "colorblind-safe", "wong":
		return ColorblindSafePalette, nil
	default:
		return Palette{}, fmt.Errorf("vulpinemark: unknown palette %q", name)
	}
}
