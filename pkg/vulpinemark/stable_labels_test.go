package vulpinemark

import "testing"

func TestStableLabelsConsistency(t *testing.T) {
	// Same logical elements, second pass has a small pixel offset
	// within the stable-label bucket. Labels must be identical.
	first := []Element{
		{Role: "button", Text: "Sign in", X: 100, Y: 200, Width: 80, Height: 30},
		{Role: "link", Text: "About", X: 300, Y: 10, Width: 40, Height: 20},
		{Role: "input", Text: "email", X: 100, Y: 260, Width: 200, Height: 30},
	}
	second := make([]Element, len(first))
	copy(second, first)
	// Jitter: 2px shifts stay inside a 16px bucket.
	for i := range second {
		second[i].X += 2
		second[i].Y -= 1
	}

	assignStableLabels(first)
	assignStableLabels(second)

	byText1 := map[string]string{}
	for _, e := range first {
		byText1[e.Text] = e.Label
	}
	for _, e := range second {
		if byText1[e.Text] != e.Label {
			t.Errorf("element %q: first=%s second=%s (should match)",
				e.Text, byText1[e.Text], e.Label)
		}
	}
}

func TestStableLabelsUnique(t *testing.T) {
	// Two elements that hash to the same bucket must still receive
	// distinct labels.
	els := []Element{
		{Role: "button", Text: "OK", X: 0, Y: 0, Width: 10, Height: 10},
		{Role: "button", Text: "OK", X: 0, Y: 0, Width: 10, Height: 10},
		{Role: "button", Text: "OK", X: 0, Y: 0, Width: 10, Height: 10},
	}
	assignStableLabels(els)
	seen := map[string]bool{}
	for _, e := range els {
		if seen[e.Label] {
			t.Errorf("duplicate label %q", e.Label)
		}
		seen[e.Label] = true
	}
}

func TestStableLabelsDifferForDifferentElements(t *testing.T) {
	a := Element{Role: "button", Text: "Sign in", X: 0, Y: 0, Width: 10, Height: 10}
	b := Element{Role: "link", Text: "About", X: 0, Y: 0, Width: 10, Height: 10}
	used := map[int]struct{}{}
	la := stableLabelFor(a, used)
	lb := stableLabelFor(b, used)
	if la == lb {
		t.Errorf("different elements produced same label %q", la)
	}
}

func TestUseStableLabelsToggle(t *testing.T) {
	m := &Mark{}
	m.UseStableLabels(true)
	if !m.stableLabels {
		t.Error("UseStableLabels(true) did not set flag")
	}
	m.UseStableLabels(false)
	if m.stableLabels {
		t.Error("UseStableLabels(false) did not clear flag")
	}
}
