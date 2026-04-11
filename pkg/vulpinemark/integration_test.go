//go:build integration

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
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// This file is only compiled when -tags integration is set. It runs a
// full Annotate flow against a fake CDP websocket server that fakes a
// tiny "page" with three interactive elements. We prefer this over
// pulling chromedp + a headless Chrome because the surface under test
// is our CDP wire handling, not Chrome itself — a fixture transport
// exercises every code path (enumerate, screenshot, layoutMetrics,
// DPR probe, draw) without the operational overhead of spinning up
// a browser container in CI.
//
// Run with:
//
//	go test -tags integration ./...

type integrationServer struct {
	pngBytes []byte
	vvW, vvH float64
	dpr      float64
}

func newIntegrationServer(t *testing.T) *integrationServer {
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
	return &integrationServer{
		pngBytes: buf.Bytes(),
		vvW:      800,
		vvH:      600,
		dpr:      1.0,
	}
}

func (s *integrationServer) handler() http.HandlerFunc {
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
				Params struct {
					Expression string `json:"expression"`
				} `json:"params"`
			}
			if err := json.Unmarshal(data, &req); err != nil {
				continue
			}
			resp := map[string]interface{}{"id": req.ID}
			switch req.Method {
			case "Runtime.evaluate":
				if strings.Contains(req.Params.Expression, "devicePixelRatio") {
					resp["result"] = map[string]interface{}{
						"result": map[string]interface{}{"type": "number", "value": s.dpr},
					}
					break
				}
				// Assume it's the enumerate IIFE. Return three elements.
				fixture := `[
					{"tag":"button","role":"button","text":"Go","x":20,"y":30,"w":80,"h":24,"confidence":0.9},
					{"tag":"a","role":"link","text":"About","x":120,"y":30,"w":60,"h":20,"confidence":0.8},
					{"tag":"input","role":"input","text":"Search","x":20,"y":80,"w":300,"h":28,"confidence":0.7}
				]`
				resp["result"] = map[string]interface{}{
					"result": map[string]interface{}{"type": "string", "value": fixture},
				}
			case "Page.getLayoutMetrics":
				resp["result"] = map[string]interface{}{
					"visualViewport": map[string]interface{}{
						"clientWidth":  s.vvW,
						"clientHeight": s.vvH,
						"scale":        1.0,
					},
				}
			case "Page.captureScreenshot":
				resp["result"] = map[string]interface{}{
					"data": base64.StdEncoding.EncodeToString(s.pngBytes),
				}
			default:
				resp["error"] = map[string]interface{}{"code": -32601, "message": "unknown: " + req.Method}
			}
			out, _ := json.Marshal(resp)
			_ = conn.WriteMessage(websocket.TextMessage, out)
		}
	}
}

func TestIntegration_AnnotateEndToEnd(t *testing.T) {
	s := newIntegrationServer(t)
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
	if n := len(result.Labels); n != 3 {
		t.Errorf("got %d labels, want 3", n)
	}
	for _, want := range []string{"@1", "@2", "@3"} {
		if _, ok := result.Elements[want]; !ok {
			t.Errorf("missing label %s", want)
		}
	}
	if len(result.Image) == 0 {
		t.Error("annotated image is empty")
	}
	// Decode the annotated PNG and assert it's still 800x600 (DPR 1.0).
	img, err := png.Decode(bytes.NewReader(result.Image))
	if err != nil {
		t.Fatalf("decode annotated: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 600 {
		t.Errorf("annotated image = %dx%d, want 800x600", b.Dx(), b.Dy())
	}
}

func TestIntegration_AnnotateRetina(t *testing.T) {
	s := newIntegrationServer(t)
	// Simulate a Retina capture: DPR 2.0 and a 1600x1200 screenshot.
	s.dpr = 2.0
	img := image.NewRGBA(image.Rect(0, 0, 1600, 1200))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	s.pngBytes = buf.Bytes()

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
	decoded, err := png.Decode(bytes.NewReader(result.Image))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b := decoded.Bounds(); b.Dx() != 1600 || b.Dy() != 1200 {
		t.Errorf("annotated retina image = %dx%d, want 1600x1200", b.Dx(), b.Dy())
	}
	// All three elements still labeled.
	if n := len(result.Labels); n != 3 {
		t.Errorf("got %d labels, want 3", n)
	}
}
