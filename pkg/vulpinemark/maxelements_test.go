package vulpinemark

import "testing"

func TestMaxElementsRespectsCap(t *testing.T) {
	elements := []Element{
		{Role: "button", Text: "high", Confidence: 0.9, X: 0, Y: 0, Width: 10, Height: 10},
		{Role: "button", Text: "low1", Confidence: 0.1, X: 20, Y: 0, Width: 10, Height: 10},
		{Role: "link", Text: "mid", Confidence: 0.5, X: 40, Y: 0, Width: 10, Height: 10},
		{Role: "link", Text: "low2", Confidence: 0.05, X: 60, Y: 0, Width: 10, Height: 10},
		{Role: "input", Text: "top", Confidence: 0.95, X: 80, Y: 0, Width: 10, Height: 10},
	}

	// Cap at 3, no clusters: should keep the three highest-confidence
	// entries (high=0.9, mid=0.5, top=0.95) in original document order.
	got := applyMaxElements(elements, 0, 3)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	texts := []string{got[0].Text, got[1].Text, got[2].Text}
	want := []string{"high", "mid", "top"}
	for i := range want {
		if texts[i] != want[i] {
			t.Errorf("index %d: got %q, want %q (all: %v)", i, texts[i], want[i], texts)
		}
	}

	// Cap at 5 with 2 clusters counted: budget = 3, same behavior.
	got2 := applyMaxElements(elements, 2, 5)
	if len(got2) != 3 {
		t.Errorf("with clusters=2, cap=5: len = %d, want 3", len(got2))
	}

	// Cap=0 disables: returns all.
	got3 := applyMaxElements(elements, 0, 0)
	if len(got3) != len(elements) {
		t.Errorf("cap=0: len = %d, want %d", len(got3), len(elements))
	}

	// Cap larger than input returns all.
	got4 := applyMaxElements(elements, 0, 100)
	if len(got4) != len(elements) {
		t.Errorf("cap=100: len = %d, want %d", len(got4), len(elements))
	}

	// Cluster count exceeds cap: budget clamps to 0, elements empty.
	got5 := applyMaxElements(elements, 10, 5)
	if len(got5) != 0 {
		t.Errorf("over-budget clusters: len = %d, want 0", len(got5))
	}
}

func TestSetMaxElementsGuardsNegative(t *testing.T) {
	m := &Mark{}
	m.SetMaxElements(-5)
	if m.maxElements != 0 {
		t.Errorf("negative value should clamp to 0, got %d", m.maxElements)
	}
	m.SetMaxElements(12)
	if m.maxElements != 12 {
		t.Errorf("positive value: got %d, want 12", m.maxElements)
	}
}
