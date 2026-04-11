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
		{"@5[3]", "@5", 3, true},
		{"@10[1]", "@10", 1, true},
		{"@5", "", 0, false},
		{"@5[", "", 0, false},
		{"@5[abc]", "", 0, false},
		{"@5[0]", "", 0, false},
		{"@5[-1]", "", 0, false},
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
				Label: "@5",
				Role:  "link",
				Members: []Element{
					{Label: "@5[1]", X: 0, Y: 0, Width: 10, Height: 10, Text: "a"},
					{Label: "@5[2]", X: 0, Y: 20, Width: 10, Height: 10, Text: "b"},
					{Label: "@5[3]", X: 0, Y: 40, Width: 10, Height: 10, Text: "c"},
				},
			}},
		},
	}

	el, err := m.lookupLabel("@5[3]")
	if err != nil {
		t.Fatalf("lookupLabel @5[3]: %v", err)
	}
	if el.Text != "c" {
		t.Errorf("wrong member: got %q, want c", el.Text)
	}

	_, err = m.lookupLabel("@5[9]")
	if !errors.Is(err, ErrClusterIndexOutOfRange) {
		t.Errorf("out-of-range: got %v, want ErrClusterIndexOutOfRange", err)
	}

	_, err = m.lookupLabel("@99[1]")
	if !errors.Is(err, ErrLabelNotFound) {
		t.Errorf("unknown cluster: got %v, want ErrLabelNotFound", err)
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
		{Label: "@1[1]", Role: "link", X: 10, Y: 10, Width: 40, Height: 40},
		{Label: "@1[2]", Role: "link", X: 60, Y: 10, Width: 40, Height: 40},
		{Label: "@1[3]", Role: "link", X: 110, Y: 10, Width: 40, Height: 40},
		{Label: "@1[4]", Role: "link", X: 10, Y: 60, Width: 40, Height: 40},
	}
	clusters := []Cluster{{Label: "@1", Role: "link", Members: members}}

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
