package vulpinemark

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// This file is a non-gated, always-on integration-style test that
// exercises the full enumerate -> screenshot -> annotate -> click/type/
// hover flow against a fake CDP server simulating a synthetic page.
// It differs from integration_test.go (which is gated on `-tags
// integration`) in three ways:
//
//  1. Runs unconditionally so every `go test` run catches regressions
//     in the action dispatch path.
//  2. Records all dispatched CDP methods and params, so we can assert
//     that Click emits Input.dispatchMouseEvent pairs, Type emits an
//     Input.insertText, and Hover emits a mouseMoved at the element's
//     center.
//  3. Serves three interactive elements (a button, a link, and an
//     input) positioned at deterministic coordinates so assertions
//     can check x/y values directly.

type fakePageServer struct {
	pngBytes []byte

	mu       sync.Mutex
	requests []fakeRPC
}

type fakeRPC struct {
	Method string
	Params map[string]interface{}
}

func newFakePageServer(t *testing.T) *fakePageServer {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for y := 0; y < 600; y++ {
		for x := 0; x < 800; x++ {
			img.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return &fakePageServer{pngBytes: buf.Bytes()}
}

func (s *fakePageServer) record(method string, params map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, fakeRPC{Method: method, Params: params})
}

func (s *fakePageServer) dispatchedMatching(method string) []fakeRPC {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []fakeRPC
	for _, r := range s.requests {
		if r.Method == method {
			out = append(out, r)
		}
	}
	return out
}

func (s *fakePageServer) handler() http.HandlerFunc {
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
				ID     int64                  `json:"id"`
				Method string                 `json:"method"`
				Params map[string]interface{} `json:"params"`
			}
			if err := json.Unmarshal(data, &req); err != nil {
				continue
			}
			s.record(req.Method, req.Params)

			resp := map[string]interface{}{"id": req.ID}
			switch req.Method {
			case "Runtime.evaluate":
				expr, _ := req.Params["expression"].(string)
				switch {
				case strings.Contains(expr, "devicePixelRatio"):
					resp["result"] = map[string]interface{}{
						"result": map[string]interface{}{"type": "number", "value": 1.0},
					}
				case strings.Contains(expr, "window.scrollX"),
					strings.Contains(expr, "window.scrollY"):
					resp["result"] = map[string]interface{}{
						"result": map[string]interface{}{"type": "number", "value": 0.0},
					}
				case strings.HasPrefix(expr, "window.scrollTo"):
					resp["result"] = map[string]interface{}{
						"result": map[string]interface{}{"type": "undefined"},
					}
				default:
					// Assume enumerate IIFE.
					fixture := `[
						{"tag":"button","role":"button","text":"Submit","x":10,"y":20,"w":100,"h":40,"confidence":0.95},
						{"tag":"a","role":"link","text":"Home","x":200,"y":20,"w":60,"h":20,"confidence":0.85},
						{"tag":"input","role":"input","text":"Email","x":10,"y":80,"w":300,"h":28,"confidence":0.8}
					]`
					resp["result"] = map[string]interface{}{
						"result": map[string]interface{}{"type": "string", "value": fixture},
					}
				}
			case "Page.getLayoutMetrics":
				resp["result"] = map[string]interface{}{
					"visualViewport": map[string]interface{}{
						"clientWidth":  800.0,
						"clientHeight": 600.0,
						"scale":        1.0,
					},
				}
			case "Page.captureScreenshot":
				resp["result"] = map[string]interface{}{
					"data": base64.StdEncoding.EncodeToString(s.pngBytes),
				}
			case "Input.dispatchMouseEvent", "Input.insertText":
				resp["result"] = map[string]interface{}{}
			default:
				resp["error"] = map[string]interface{}{"code": -32601, "message": "unknown: " + req.Method}
			}
			out, _ := json.Marshal(resp)
			_ = conn.WriteMessage(websocket.TextMessage, out)
		}
	}
}

func TestRealPageFlow_AnnotateClickTypeHover(t *testing.T) {
	s := newFakePageServer(t)
	srv := httptest.NewServer(s.handler())
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m, err := New(ctx, wsURL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	result, err := m.Annotate(ctx)
	if err != nil {
		t.Fatalf("Annotate: %v", err)
	}
	if len(result.Labels) != 3 {
		t.Fatalf("got %d labels, want 3", len(result.Labels))
	}
	// Labels assigned in document order.
	if el := result.Elements["@1"]; el.Role != "button" || el.Text != "Submit" {
		t.Errorf("@1 = %+v, want button/Submit", el)
	}
	if el := result.Elements["@2"]; el.Role != "link" || el.Text != "Home" {
		t.Errorf("@2 = %+v, want link/Home", el)
	}
	if el := result.Elements["@3"]; el.Role != "input" {
		t.Errorf("@3 = %+v, want input", el)
	}

	// Click @1 — center of (10,20,100x40) is (60,40).
	if err := m.Click(ctx, "@1"); err != nil {
		t.Fatalf("Click: %v", err)
	}
	mouseEvents := s.dispatchedMatching("Input.dispatchMouseEvent")
	if len(mouseEvents) < 2 {
		t.Fatalf("Click should dispatch mousePressed+mouseReleased, got %d", len(mouseEvents))
	}
	last2 := mouseEvents[len(mouseEvents)-2:]
	if last2[0].Params["type"] != "mousePressed" {
		t.Errorf("first event type = %v, want mousePressed", last2[0].Params["type"])
	}
	if last2[1].Params["type"] != "mouseReleased" {
		t.Errorf("second event type = %v, want mouseReleased", last2[1].Params["type"])
	}
	for _, e := range last2 {
		if fx, _ := e.Params["x"].(float64); fx != 60 {
			t.Errorf("click x = %v, want 60", e.Params["x"])
		}
		if fy, _ := e.Params["y"].(float64); fy != 40 {
			t.Errorf("click y = %v, want 40", e.Params["y"])
		}
	}

	// Type @3 — dispatches a Click (another mousePressed+Released) then Input.insertText.
	if err := m.Type(ctx, "@3", "hello@example.com"); err != nil {
		t.Fatalf("Type: %v", err)
	}
	inserts := s.dispatchedMatching("Input.insertText")
	if len(inserts) != 1 {
		t.Fatalf("Type should dispatch one insertText, got %d", len(inserts))
	}
	if inserts[0].Params["text"] != "hello@example.com" {
		t.Errorf("insertText text = %v, want hello@example.com", inserts[0].Params["text"])
	}

	// Hover @2 — center of (200,20,60x20) is (230,30). mouseMoved.
	if err := m.Hover(ctx, "@2"); err != nil {
		t.Fatalf("Hover: %v", err)
	}
	mouseEvents = s.dispatchedMatching("Input.dispatchMouseEvent")
	lastHover := mouseEvents[len(mouseEvents)-1]
	if lastHover.Params["type"] != "mouseMoved" {
		t.Errorf("last event type = %v, want mouseMoved", lastHover.Params["type"])
	}
	if fx, _ := lastHover.Params["x"].(float64); fx != 230 {
		t.Errorf("hover x = %v, want 230", lastHover.Params["x"])
	}
	if fy, _ := lastHover.Params["y"].(float64); fy != 30 {
		t.Errorf("hover y = %v, want 30", lastHover.Params["y"])
	}

	// Verify the annotated PNG decodes and keeps the source dimensions.
	decoded, err := png.Decode(bytes.NewReader(result.Image))
	if err != nil {
		t.Fatalf("decode annotated: %v", err)
	}
	if b := decoded.Bounds(); b.Dx() != 800 || b.Dy() != 600 {
		t.Errorf("annotated image = %dx%d, want 800x600", b.Dx(), b.Dy())
	}
}
