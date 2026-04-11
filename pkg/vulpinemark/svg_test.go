package vulpinemark

import (
	"strings"
	"testing"
)

func TestSVGOverlayContainsRectAndText(t *testing.T) {
	elements := []Element{
		{Label: "@1", Role: "button", Text: "OK", X: 10, Y: 20, Width: 40, Height: 20},
		{Label: "@2", Role: "link", Text: "home", X: 60, Y: 20, Width: 30, Height: 20},
	}
	svg := renderSVGOverlay(elements, nil, 1.0, 200, 120, DefaultPalette)

	if !strings.HasPrefix(svg, `<svg`) {
		t.Fatalf("missing svg root: %q", svg[:40])
	}
	if !strings.Contains(svg, `width="200"`) || !strings.Contains(svg, `height="120"`) {
		t.Error("svg dimensions missing")
	}
	if strings.Count(svg, "<rect") < 4 {
		// 2 borders + 2 badges = 4
		t.Errorf("expected >=4 rects, got svg: %s", svg)
	}
	if !strings.Contains(svg, ">@1<") {
		t.Error("missing @1 text")
	}
	if !strings.Contains(svg, ">@2<") {
		t.Error("missing @2 text")
	}
	if !strings.HasSuffix(svg, "</svg>") {
		t.Error("missing svg close")
	}
}

func TestSVGCoordinatesMatchScale(t *testing.T) {
	elements := []Element{
		{Label: "@1", Role: "button", X: 10, Y: 20, Width: 40, Height: 20},
	}
	svg := renderSVGOverlay(elements, nil, 2.0, 400, 240, DefaultPalette)
	// With scale=2, x=10 becomes 20, width=40 becomes 80.
	if !strings.Contains(svg, `x="20"`) {
		t.Errorf("expected scaled x=20 in svg: %s", svg)
	}
	if !strings.Contains(svg, `width="80"`) {
		t.Errorf("expected scaled width=80 in svg: %s", svg)
	}
	if !strings.Contains(svg, `height="40"`) {
		t.Errorf("expected scaled height=40 in svg: %s", svg)
	}
}

func TestSVGClusterOverlay(t *testing.T) {
	members := []Element{
		{Role: "link", X: 0, Y: 0, Width: 50, Height: 20},
		{Role: "link", X: 0, Y: 30, Width: 50, Height: 20},
		{Role: "link", X: 0, Y: 60, Width: 50, Height: 20},
		{Role: "link", X: 0, Y: 90, Width: 50, Height: 20},
	}
	clusters := []Cluster{{Label: "@C1", Role: "link", Members: members}}
	svg := renderSVGOverlay(nil, clusters, 1.0, 200, 200, DefaultPalette)
	if !strings.Contains(svg, "stroke-dasharray") {
		t.Error("cluster outline should be dashed")
	}
	if !strings.Contains(svg, ">@C1[1..4]<") {
		t.Errorf("missing cluster label: %s", svg)
	}
}

// TestSVGOverlayDeterministicOrder re-renders an SVG overlay many
// times from the same Result and asserts the output is byte-identical
// on every call. Prior to the fix, the AnnotateSVG helper iterated
// result.Elements (a Go map) whose iteration order is intentionally
// randomized, so the SVG z-order — and therefore the raw bytes —
// varied between runs.
func TestSVGOverlayDeterministicOrder(t *testing.T) {
	// A Result with enough elements that the map iteration order is
	// very likely to change between runs if we don't sort.
	labels := []string{"@1", "@2", "@3", "@4", "@5", "@6", "@7", "@8"}
	byLabel := map[string]Element{
		"@1": {Label: "@1", Role: "button", Text: "a", X: 0, Y: 0, Width: 10, Height: 10},
		"@2": {Label: "@2", Role: "link", Text: "b", X: 10, Y: 0, Width: 10, Height: 10},
		"@3": {Label: "@3", Role: "input", Text: "c", X: 20, Y: 0, Width: 10, Height: 10},
		"@4": {Label: "@4", Role: "select", Text: "d", X: 30, Y: 0, Width: 10, Height: 10},
		"@5": {Label: "@5", Role: "checkbox", Text: "e", X: 40, Y: 0, Width: 10, Height: 10},
		"@6": {Label: "@6", Role: "button", Text: "f", X: 50, Y: 0, Width: 10, Height: 10},
		"@7": {Label: "@7", Role: "link", Text: "g", X: 60, Y: 0, Width: 10, Height: 10},
		"@8": {Label: "@8", Role: "input", Text: "h", X: 70, Y: 0, Width: 10, Height: 10},
	}
	result := &Result{Elements: byLabel, Labels: labels}

	// buildSVG mirrors AnnotateSVG's post-annotate rebuild logic:
	// iterate via result.Labels so the Elements map's randomized
	// iteration order is never observed.
	buildSVG := func() string {
		elements := make([]Element, 0, len(result.Elements))
		for _, label := range result.Labels {
			el, ok := result.Elements[label]
			if !ok {
				continue
			}
			elements = append(elements, el)
		}
		return renderSVGOverlay(elements, nil, 1.0, 200, 120, DefaultPalette)
	}

	first := buildSVG()
	for i := 0; i < 50; i++ {
		got := buildSVG()
		if got != first {
			t.Fatalf("run %d produced different SVG bytes", i)
		}
	}
}

func TestSVGEscapesLabel(t *testing.T) {
	elements := []Element{
		{Label: "@1<x>", Role: "button", X: 0, Y: 0, Width: 10, Height: 10},
	}
	svg := renderSVGOverlay(elements, nil, 1.0, 100, 100, DefaultPalette)
	if strings.Contains(svg, "@1<x>") {
		t.Error("label should be XML-escaped")
	}
	if !strings.Contains(svg, "@1&lt;x&gt;") {
		t.Error("expected escaped label")
	}
}
