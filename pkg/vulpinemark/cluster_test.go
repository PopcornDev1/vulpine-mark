package vulpinemark

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestClusterElementsBasic(t *testing.T) {
	// 5 identically-sized product cards should cluster together; the
	// loose button stays ungrouped.
	els := []Element{
		{Role: "link", X: 0, Y: 0, Width: 100, Height: 100, Text: "card1"},
		{Role: "link", X: 110, Y: 0, Width: 100, Height: 100, Text: "card2"},
		{Role: "link", X: 220, Y: 0, Width: 100, Height: 100, Text: "card3"},
		{Role: "link", X: 0, Y: 110, Width: 100, Height: 100, Text: "card4"},
		{Role: "link", X: 110, Y: 110, Width: 100, Height: 100, Text: "card5"},
		{Role: "button", X: 0, Y: 300, Width: 80, Height: 30, Text: "Submit"},
	}
	clusters, ungrouped := clusterElements(els)
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if len(clusters[0].Members) != 5 {
		t.Errorf("cluster member count = %d, want 5", len(clusters[0].Members))
	}
	if clusters[0].Role != "link" {
		t.Errorf("cluster role = %q, want link", clusters[0].Role)
	}
	// Members must be in reading order (top-to-bottom, left-to-right).
	wantOrder := []string{"card1", "card2", "card3", "card4", "card5"}
	for i, m := range clusters[0].Members {
		if m.Text != wantOrder[i] {
			t.Errorf("member[%d].Text = %q, want %q", i, m.Text, wantOrder[i])
		}
	}
	if len(ungrouped) != 1 || ungrouped[0].Text != "Submit" {
		t.Errorf("ungrouped = %+v, want [Submit]", ungrouped)
	}
}

func TestClusterElementsMinSize(t *testing.T) {
	// Only 3 identically-sized items — must not cluster (min is 4).
	els := []Element{
		{Role: "link", X: 0, Y: 0, Width: 50, Height: 50},
		{Role: "link", X: 60, Y: 0, Width: 50, Height: 50},
		{Role: "link", X: 120, Y: 0, Width: 50, Height: 50},
	}
	clusters, ungrouped := clusterElements(els)
	if len(clusters) != 0 {
		t.Errorf("expected 0 clusters, got %d", len(clusters))
	}
	if len(ungrouped) != 3 {
		t.Errorf("ungrouped len = %d, want 3", len(ungrouped))
	}
}

func TestClusterElementsRoundingBucket(t *testing.T) {
	// Sizes within the 8px rounding bucket should cluster together.
	els := []Element{
		{Role: "button", X: 0, Y: 0, Width: 100, Height: 40},
		{Role: "button", X: 0, Y: 50, Width: 102, Height: 41},
		{Role: "button", X: 0, Y: 100, Width: 101, Height: 39},
		{Role: "button", X: 0, Y: 150, Width: 103, Height: 40},
	}
	clusters, _ := clusterElements(els)
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if len(clusters[0].Members) != 4 {
		t.Errorf("member count = %d, want 4", len(clusters[0].Members))
	}
}

func TestParseClusterRef(t *testing.T) {
	cases := []struct {
		in      string
		wantLbl string
		wantIdx int
		wantOK  bool
	}{
		{"@C5[3]", "@C5", 3, true},
		{"@C10[1]", "@C10", 1, true},
		{"@C5", "", 0, false},
		{"@C5[", "", 0, false},
		{"@C5[abc]", "", 0, false},
		{"@C5[0]", "", 0, false},
		{"@C5[-1]", "", 0, false},
		// Plain element labels must not be mistaken for cluster refs.
		{"@5[1]", "", 0, false},
		{"@5", "", 0, false},
	}
	for _, tc := range cases {
		lbl, idx, ok := parseClusterRef(tc.in)
		if lbl != tc.wantLbl || idx != tc.wantIdx || ok != tc.wantOK {
			t.Errorf("parseClusterRef(%q) = (%q,%d,%v), want (%q,%d,%v)",
				tc.in, lbl, idx, ok, tc.wantLbl, tc.wantIdx, tc.wantOK)
		}
	}
}

func TestLookupLabel_ClusterIndex(t *testing.T) {
	m := &Mark{
		lastResult: &Result{
			Elements: map[string]Element{},
			Clusters: []Cluster{{
				Label: "@C5",
				Role:  "link",
				Members: []Element{
					{Label: "@C5[1]", X: 0, Y: 0, Width: 10, Height: 10, Text: "a"},
					{Label: "@C5[2]", X: 0, Y: 20, Width: 10, Height: 10, Text: "b"},
					{Label: "@C5[3]", X: 0, Y: 40, Width: 10, Height: 10, Text: "c"},
				},
			}},
		},
	}

	el, err := m.lookupLabel("@C5[3]")
	if err != nil {
		t.Fatalf("lookupLabel @C5[3]: %v", err)
	}
	if el.Text != "c" {
		t.Errorf("wrong member: got %q, want c", el.Text)
	}

	_, err = m.lookupLabel("@C5[9]")
	if !errors.Is(err, ErrClusterIndexOutOfRange) {
		t.Errorf("out-of-range: got %v, want ErrClusterIndexOutOfRange", err)
	}

	_, err = m.lookupLabel("@C99[1]")
	if !errors.Is(err, ErrLabelNotFound) {
		t.Errorf("unknown cluster: got %v, want ErrLabelNotFound", err)
	}
}

// TestClusterLabelNamespaceDoesntCollideWithElements verifies that
// cluster labels ("@C1", "@C2", ...) and per-element labels ("@1",
// "@2", ...) live in disjoint namespaces so a Result that carries both
// can be looked up unambiguously. Prior to the "@C" prefix, three
// clusters + five elements would alias "@3" across the two kinds.
func TestClusterLabelNamespaceDoesntCollideWithElements(t *testing.T) {
	m := &Mark{
		lastResult: &Result{
			Elements: map[string]Element{
				"@1": {Label: "@1", Text: "el1", X: 0, Y: 0, Width: 10, Height: 10},
				"@2": {Label: "@2", Text: "el2", X: 0, Y: 20, Width: 10, Height: 10},
				"@3": {Label: "@3", Text: "el3", X: 0, Y: 40, Width: 10, Height: 10},
				"@4": {Label: "@4", Text: "el4", X: 0, Y: 60, Width: 10, Height: 10},
				"@5": {Label: "@5", Text: "el5", X: 0, Y: 80, Width: 10, Height: 10},
			},
			Clusters: []Cluster{
				{Label: "@C1", Role: "link", Members: []Element{
					{Label: "@C1[1]", Text: "c1m1", X: 0, Y: 0, Width: 10, Height: 10},
				}},
				{Label: "@C2", Role: "link", Members: []Element{
					{Label: "@C2[1]", Text: "c2m1", X: 0, Y: 0, Width: 10, Height: 10},
				}},
				{Label: "@C3", Role: "link", Members: []Element{
					{Label: "@C3[1]", Text: "c3m1", X: 0, Y: 0, Width: 10, Height: 10},
				}},
			},
		},
	}

	// Plain element "@3" must resolve to the element, NOT a cluster.
	el, err := m.lookupLabel("@3")
	if err != nil {
		t.Fatalf("lookup @3: %v", err)
	}
	if el.Text != "el3" {
		t.Errorf("@3 resolved to %q, want el3", el.Text)
	}

	// Cluster member "@C3[1]" must resolve to the 3rd cluster's first
	// member.
	cm, err := m.lookupLabel("@C3[1]")
	if err != nil {
		t.Fatalf("lookup @C3[1]: %v", err)
	}
	if cm.Text != "c3m1" {
		t.Errorf("@C3[1] resolved to %q, want c3m1", cm.Text)
	}

	// A bare "@C3" (no bracket) is not a valid clickable label since
	// clusters only expose members; lookup must NOT silently return an
	// element because of namespace leakage.
	if _, err := m.lookupLabel("@C3"); err == nil {
		t.Errorf("lookup @C3 should fail (not in Elements map)")
	}
}

// TestClusterElementsUsesClusterNamespace sanity-checks that
// clusterElements emits labels with the "@C" prefix.
func TestClusterElementsUsesClusterNamespace(t *testing.T) {
	els := []Element{
		{Role: "link", X: 0, Y: 0, Width: 100, Height: 100, Text: "a"},
		{Role: "link", X: 110, Y: 0, Width: 100, Height: 100, Text: "b"},
		{Role: "link", X: 220, Y: 0, Width: 100, Height: 100, Text: "c"},
		{Role: "link", X: 0, Y: 110, Width: 100, Height: 100, Text: "d"},
	}
	clusters, _ := clusterElements(els)
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters", len(clusters))
	}
	if clusters[0].Label != "@C1" {
		t.Errorf("cluster label = %q, want @C1", clusters[0].Label)
	}
}

func TestClusterDrawSmoke(t *testing.T) {
	// Tiny white source image.
	const w, h = 200, 200
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			src.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode src: %v", err)
	}

	members := []Element{
		{Label: "@C1[1]", Role: "link", X: 10, Y: 10, Width: 40, Height: 40},
		{Label: "@C1[2]", Role: "link", X: 60, Y: 10, Width: 40, Height: 40},
		{Label: "@C1[3]", Role: "link", X: 110, Y: 10, Width: 40, Height: 40},
		{Label: "@C1[4]", Role: "link", X: 10, Y: 60, Width: 40, Height: 40},
	}
	clusters := []Cluster{{Label: "@C1", Role: "link", Members: members}}

	out, err := drawAnnotationsWithClusters(buf.Bytes(), nil, clusters, 1.0)
	if err != nil {
		t.Fatalf("drawAnnotationsWithClusters: %v", err)
	}
	dec, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode out: %v", err)
	}
	if dec.Bounds().Dx() != w || dec.Bounds().Dy() != h {
		t.Errorf("dims = %dx%d, want %dx%d",
			dec.Bounds().Dx(), dec.Bounds().Dy(), w, h)
	}
}
