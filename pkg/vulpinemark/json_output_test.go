package vulpinemark

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAnnotateJSON_NoImage(t *testing.T) {
	fixture := `[
		{"tag":"button","role":"button","text":"Go","x":0,"y":0,"w":80,"h":24,"confidence":0.9},
		{"tag":"a","role":"link","text":"Home","x":100,"y":0,"w":60,"h":20,"confidence":0.8}
	]`
	f := &fakeEnumerateServer{elementsJSON: fixture}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m, err := New(ctx, wsURL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	result, err := m.AnnotateJSON(ctx)
	if err != nil {
		t.Fatalf("AnnotateJSON: %v", err)
	}
	if result.Image != nil {
		t.Errorf("AnnotateJSON should return nil Image, got %d bytes", len(result.Image))
	}
	if len(result.Labels) != 2 {
		t.Errorf("got %d labels, want 2", len(result.Labels))
	}
	if err := ensureLabels(result.Labels); err != nil {
		t.Errorf("labels: %v", err)
	}
	if _, ok := result.Elements["@1"]; !ok {
		t.Error("missing @1")
	}
	if _, ok := result.Elements["@2"]; !ok {
		t.Error("missing @2")
	}
	// LastResult should reflect the JSON-only call.
	if m.LastResult() != result {
		t.Error("LastResult mismatch")
	}
}

func TestAnnotateJSON_WithFilter(t *testing.T) {
	fixture := `[
		{"tag":"button","role":"button","text":"Go","x":0,"y":0,"w":80,"h":24,"confidence":0.9},
		{"tag":"a","role":"link","text":"Home","x":100,"y":0,"w":60,"h":20,"confidence":0.8},
		{"tag":"input","role":"input","text":"Email","x":0,"y":50,"w":200,"h":28,"confidence":0.7}
	]`
	f := &fakeEnumerateServer{elementsJSON: fixture}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m, err := New(ctx, wsURL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	m.SetElementFilter(IncludeRoles("button"))
	result, err := m.AnnotateJSON(ctx)
	if err != nil {
		t.Fatalf("AnnotateJSON: %v", err)
	}
	if len(result.Labels) != 1 {
		t.Errorf("got %d labels, want 1 (filter to buttons)", len(result.Labels))
	}
	if el, ok := result.Elements["@1"]; !ok || el.Role != "button" {
		t.Errorf("@1 should be button, got %+v", el)
	}
}
