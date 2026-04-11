package vulpinemark

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Approach for this file: rather than spin up a JS engine (goja +
// JSDOM-equivalent) to run enumerateJSTemplate against a fixture HTML
// page, we verify the enumerator contract from two angles:
//
//  1. Static inspection of enumerateJSTemplate: the SELECTOR list
//     contains every interactive tag/role we promise to label, and the
//     template is a valid %t format string producing a self-invoking
//     function. This catches accidental deletions during refactors.
//
//  2. Fixture CDP transport: a fake websocket CDP server replays a
//     canned Runtime.evaluate response (elements JSON) and we assert
//     cdpClient.enumerate round-trips it into a []Element matching the
//     snapshot. This exercises the Go decode path end-to-end without
//     a real browser.
//
// A real-browser integration test lives in integration_test.go behind
// a build tag.

func TestEnumerateJSTemplate_IncludesAllSelectors(t *testing.T) {
	wanted := []string{
		"a[href]",
		"button",
		"input:not([type=\"hidden\"])",
		"select",
		"textarea",
		"[role=\"button\"]",
		"[role=\"link\"]",
		"[role=\"checkbox\"]",
		"[role=\"menuitem\"]",
		"[role=\"tab\"]",
		"[role=\"radio\"]",
		"[role=\"switch\"]",
		"[contenteditable=\"true\"]",
		"[tabindex]:not([tabindex=\"-1\"])",
		"[onclick]",
	}
	for _, sel := range wanted {
		if !strings.Contains(enumerateJSTemplate, sel) {
			t.Errorf("enumerateJSTemplate missing selector %q", sel)
		}
	}
}

func TestEnumerateJSTemplate_HasFormatVerb(t *testing.T) {
	// The template is rendered with %t to toggle viewport-only filter.
	if !strings.Contains(enumerateJSTemplate, "%t") {
		t.Fatalf("enumerateJSTemplate missing %%t verb for viewportOnly toggle")
	}
	// And exposes isVisible, isOccluded, classify, nameOf, confidenceFor.
	for _, fn := range []string{"isVisible", "isOccluded", "classify", "nameOf", "confidenceFor"} {
		if !strings.Contains(enumerateJSTemplate, "function "+fn) {
			t.Errorf("enumerateJSTemplate missing function %s", fn)
		}
	}
}

func TestEnumerateJSTemplate_SelfInvoking(t *testing.T) {
	// Sanity-check the closing IIFE pattern so we know the rendered
	// expression is actually executable when Runtime.evaluate runs it.
	re := regexp.MustCompile(`\(viewportOnly\)\s*=>\s*\{[\s\S]*\}\)\(%t\)`)
	if !re.MatchString(enumerateJSTemplate) {
		t.Fatal("enumerateJSTemplate does not match self-invoking arrow pattern")
	}
}

func TestComputeConfidence_Table(t *testing.T) {
	cases := []struct {
		name string
		s    confidenceSignals
		want float64
	}{
		{"everything", confidenceSignals{HasName: true, HasAriaAttr: true, Area: 500, Occluded: false, Clipped: false}, 1.0},
		{"nothing", confidenceSignals{}, 0.2 + 0.1}, // not occluded + not clipped defaults
		{"named_only", confidenceSignals{HasName: true, Area: 0}, 0.3 + 0.2 + 0.1},
		{"occluded", confidenceSignals{HasName: true, HasAriaAttr: true, Area: 500, Occluded: true}, 0.3 + 0.2 + 0.2 + 0.1},
		{"clipped", confidenceSignals{HasName: true, HasAriaAttr: true, Area: 500, Clipped: true}, 0.3 + 0.2 + 0.2 + 0.2},
		{"tiny", confidenceSignals{HasName: true, Area: 50}, 0.3 + 0.2 + 0.1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := computeConfidence(c.s)
			if math.Abs(got-c.want) > 1e-9 {
				t.Errorf("computeConfidence(%+v) = %v, want %v", c.s, got, c.want)
			}
		})
	}
}

// fakeEnumerateServer is a CDP websocket stub that replays a fixed
// Runtime.evaluate JSON payload for enumerate() tests.
type fakeEnumerateServer struct {
	elementsJSON string
}

func (f *fakeEnumerateServer) handler() http.HandlerFunc {
	up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req struct {
				ID     int64  `json:"id"`
				Method string `json:"method"`
			}
			if err := json.Unmarshal(data, &req); err != nil {
				continue
			}
			resp := map[string]interface{}{"id": req.ID}
			switch req.Method {
			case "Runtime.evaluate":
				resp["result"] = map[string]interface{}{
					"result": map[string]interface{}{
						"type":  "string",
						"value": f.elementsJSON,
					},
				}
			default:
				resp["error"] = map[string]interface{}{"code": -32601, "message": "nope"}
			}
			out, _ := json.Marshal(resp)
			_ = conn.WriteMessage(websocket.TextMessage, out)
		}
	}
}

// TestEnumerate_DecodesFixtureResponse acts as the "snapshot" unit
// test: given a canned JSON payload that matches what enumerate.js
// would emit against the fixture HTML in testdata/fixture.html (a
// form with a button, a link, and an input), cdpClient.enumerate
// produces the expected []Element.
func TestEnumerate_DecodesFixtureResponse(t *testing.T) {
	fixture := `[
		{"tag":"button","role":"button","text":"Submit","x":10,"y":20,"w":100,"h":32,"confidence":1.0},
		{"tag":"a","role":"link","text":"Home","x":150,"y":20,"w":60,"h":24,"confidence":0.9},
		{"tag":"input","role":"input","text":"Email","x":10,"y":60,"w":200,"h":28,"confidence":0.8}
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

	els, err := m.c.enumerate(ctx, true)
	if err != nil {
		t.Fatalf("enumerate: %v", err)
	}
	if len(els) != 3 {
		t.Fatalf("got %d elements, want 3", len(els))
	}

	want := []Element{
		{Tag: "button", Role: "button", Text: "Submit", X: 10, Y: 20, Width: 100, Height: 32, Confidence: 1.0},
		{Tag: "a", Role: "link", Text: "Home", X: 150, Y: 20, Width: 60, Height: 24, Confidence: 0.9},
		{Tag: "input", Role: "input", Text: "Email", X: 10, Y: 60, Width: 200, Height: 28, Confidence: 0.8},
	}
	for i, w := range want {
		if els[i] != w {
			t.Errorf("element[%d] = %+v, want %+v", i, els[i], w)
		}
	}
}

func TestEnumerate_EmptyFixture(t *testing.T) {
	f := &fakeEnumerateServer{elementsJSON: `[]`}
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

	els, err := m.c.enumerate(ctx, true)
	if err != nil {
		t.Fatalf("enumerate: %v", err)
	}
	if len(els) != 0 {
		t.Errorf("want no elements, got %d", len(els))
	}
}
