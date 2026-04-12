package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/PopcornDev1/vulpine-mark/pkg/vulpinemark"
)

func TestSaveLoadResultRoundTrip(t *testing.T) {
	// Build a Result that covers every field the --diff reader needs:
	// Elements (label-keyed), Labels (ordered), and Clusters (with
	// Members so AnnotateDiff can re-derive bounding boxes).
	orig := &vulpinemark.Result{
		Elements: map[string]vulpinemark.Element{
			"@1": {Label: "@1", Tag: "button", Role: "button", Text: "Go", X: 10, Y: 20, Width: 80, Height: 24, Confidence: 0.9},
			"@2": {Label: "@2", Tag: "a", Role: "link", Text: "Home", X: 100, Y: 20, Width: 40, Height: 20, Confidence: 0.85},
		},
		Labels: []string{"@1", "@2", "@3"},
		Clusters: []vulpinemark.Cluster{
			{
				Label: "@3",
				Members: []vulpinemark.Element{
					{Label: "@3[1]", Tag: "div", Role: "button", Text: "Item 1", X: 200, Y: 100, Width: 60, Height: 60, Confidence: 0.7},
					{Label: "@3[2]", Tag: "div", Role: "button", Text: "Item 2", X: 270, Y: 100, Width: 60, Height: 60, Confidence: 0.7},
				},
			},
		},
	}

	sr := savedResult{
		Elements: orig.Elements,
		Labels:   orig.Labels,
		Clusters: orig.Clusters,
	}
	data, err := json.MarshalIndent(sr, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "result.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := loadSavedResult(path)
	if err != nil {
		t.Fatalf("loadSavedResult: %v", err)
	}
	if !reflect.DeepEqual(loaded.Elements, orig.Elements) {
		t.Errorf("Elements round-trip mismatch\n got: %+v\nwant: %+v", loaded.Elements, orig.Elements)
	}
	if !reflect.DeepEqual(loaded.Labels, orig.Labels) {
		t.Errorf("Labels round-trip mismatch\n got: %+v\nwant: %+v", loaded.Labels, orig.Labels)
	}
	if !reflect.DeepEqual(loaded.Clusters, orig.Clusters) {
		t.Errorf("Clusters round-trip mismatch\n got: %+v\nwant: %+v", loaded.Clusters, orig.Clusters)
	}
}

func TestLoadSavedResult_MissingFile(t *testing.T) {
	_, err := loadSavedResult(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestBuildFilter_IncludeExcludeCombination(t *testing.T) {
	// No flags -> nil filter.
	if f := buildFilter("", ""); f != nil {
		t.Error("empty flags should yield nil filter")
	}
	// Include only.
	f := buildFilter("button,link", "")
	if f == nil || !f(vulpinemark.Element{Role: "button"}) || f(vulpinemark.Element{Role: "input"}) {
		t.Error("include filter misbehaved")
	}
	// Exclude only.
	f = buildFilter("", "checkbox, radio ")
	if f == nil || !f(vulpinemark.Element{Role: "button"}) || f(vulpinemark.Element{Role: "checkbox"}) {
		t.Error("exclude filter misbehaved")
	}
	// Combined: include wins if set, exclude trims further.
	f = buildFilter("button,link", "link")
	if !f(vulpinemark.Element{Role: "button"}) {
		t.Error("button should survive combined filter")
	}
	if f(vulpinemark.Element{Role: "link"}) {
		t.Error("link should be excluded by combined filter")
	}
	if f(vulpinemark.Element{Role: "input"}) {
		t.Error("input should not be included by combined filter")
	}
}

func TestLoadSavedResult_BadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := loadSavedResult(path)
	if err == nil {
		t.Fatal("expected json error")
	}
}
