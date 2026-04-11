package vulpinemark

import (
	"testing"
)

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

// TestStableLabelProbeCoprime verifies that the linear-probe offset
// can visit every slot in the stable-label ring in the worst case.
// This guards against the earlier bug where the modulus (9999 =
// 3·3·11·101) combined with an offset divisible by 3 or 11 would
// visit only a fraction of the slots, causing label assignment to
// fail spuriously for large pages.
func TestStableLabelProbeCoprime(t *testing.T) {
	// stableLabelModulus must be prime; if so every offset in
	// [1, modulus-1] is automatically coprime with the modulus and
	// the probe visits every slot before wrapping.
	if !isPrime(stableLabelModulus) {
		t.Fatalf("stableLabelModulus = %d must be prime", stableLabelModulus)
	}

	// Pick the worst-case start (n=1) and the largest offset the
	// current implementation can produce (7), then walk every slot
	// and assert we land on each integer in [1, modulus] exactly once.
	const startN = 1
	const offset = 7
	hits := make(map[int]bool, stableLabelModulus)
	for i := 0; i < stableLabelModulus; i++ {
		cand := ((startN-1+i*offset)%stableLabelModulus + 1)
		if hits[cand] {
			t.Fatalf("probe revisited slot %d at step %d (modulus=%d, offset=%d)",
				cand, i, stableLabelModulus, offset)
		}
		hits[cand] = true
	}
	if len(hits) != stableLabelModulus {
		t.Fatalf("probe covered %d/%d slots", len(hits), stableLabelModulus)
	}

	// End-to-end: filling the used set with modulus-1 elements and
	// asking for one more must succeed — never fall through to the
	// sequential-fallback loop — because the probe visits every slot.
	used := make(map[int]struct{}, stableLabelModulus)
	for i := 1; i < stableLabelModulus; i++ {
		used[i] = struct{}{}
	}
	lbl := stableLabelFor(Element{Role: "button", Text: "only-free"}, used)
	if lbl == "" {
		t.Fatal("stableLabelFor returned empty")
	}
}

func isPrime(n int) bool {
	if n < 2 {
		return false
	}
	for i := 2; i*i <= n; i++ {
		if n%i == 0 {
			return false
		}
	}
	return true
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
