package vulpinemark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestCombineScale(t *testing.T) {
	cases := []struct {
		name string
		vv   float64
		dpr  float64
		want float64
	}{
		{"retina_normal_zoom", 1.0, 2.0, 2.0},
		{"retina_3x", 1.0, 3.0, 3.0},
		{"non_retina", 1.0, 1.0, 1.0},
		{"zoomed_in_retina", 1.5, 2.0, 3.0},
		{"zero_vv_defaults_to_1", 0, 2.0, 2.0},
		{"zero_dpr_defaults_to_1", 1.0, 0, 1.0},
		{"both_zero", 0, 0, 1.0},
		{"negative_falls_back", -1, -1, 1.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := combineScale(c.vv, c.dpr)
			if got != c.want {
				t.Errorf("combineScale(%v,%v) = %v, want %v", c.vv, c.dpr, got, c.want)
			}
		})
	}
}

// fakeCDPServer runs a minimal CDP-over-websocket server that responds
// to Page.getLayoutMetrics and Runtime.evaluate(window.devicePixelRatio).
// Other method calls yield an RPC error. Used only by tests.
type fakeCDPServer struct {
	dpr            float64
	visualViewport struct {
		ClientWidth, ClientHeight, Scale float64
	}
}

func (f *fakeCDPServer) handler() http.HandlerFunc {
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
				ID     int64           `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(data, &req); err != nil {
				continue
			}
			resp := map[string]interface{}{"id": req.ID}
			switch req.Method {
			case "Page.getLayoutMetrics":
				resp["result"] = map[string]interface{}{
					"visualViewport": map[string]interface{}{
						"clientWidth":  f.visualViewport.ClientWidth,
						"clientHeight": f.visualViewport.ClientHeight,
						"scale":        f.visualViewport.Scale,
					},
				}
			case "Runtime.evaluate":
				resp["result"] = map[string]interface{}{
					"result": map[string]interface{}{
						"type":  "number",
						"value": f.dpr,
					},
				}
			default:
				resp["error"] = map[string]interface{}{
					"code":    -32601,
					"message": "method not found: " + req.Method,
				}
			}
			out, _ := json.Marshal(resp)
			_ = conn.WriteMessage(websocket.TextMessage, out)
		}
	}
}

// newFakeMark spins up the fake CDP server and returns a Mark wired to
// it plus a teardown func. The Mark is not usable for real Annotate
// calls (no screenshot endpoint) but is fine for probing viewportSize
// and similar primitives.
func newFakeMark(t *testing.T, f *fakeCDPServer) (*Mark, func()) {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m, err := New(ctx, wsURL)
	if err != nil {
		srv.Close()
		t.Fatalf("New: %v", err)
	}
	return m, func() {
		_ = m.Close()
		srv.Close()
	}
}

func TestViewportSize_AccountsForDPR(t *testing.T) {
	f := &fakeCDPServer{dpr: 2.0}
	f.visualViewport.ClientWidth = 1280
	f.visualViewport.ClientHeight = 800
	f.visualViewport.Scale = 1.0

	m, teardown := newFakeMark(t, f)
	defer teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	w, h, scale, err := m.c.viewportSize(ctx)
	if err != nil {
		t.Fatalf("viewportSize: %v", err)
	}
	if w != 1280 || h != 800 {
		t.Errorf("viewportSize dims = %vx%v, want 1280x800", w, h)
	}
	if scale != 2.0 {
		t.Errorf("viewportSize scale = %v, want 2.0 (1.0 vv * 2.0 dpr)", scale)
	}
}

func TestViewportSize_VVZoomTimesDPR(t *testing.T) {
	f := &fakeCDPServer{dpr: 2.0}
	f.visualViewport.ClientWidth = 1024
	f.visualViewport.ClientHeight = 768
	f.visualViewport.Scale = 1.5

	m, teardown := newFakeMark(t, f)
	defer teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, scale, err := m.c.viewportSize(ctx)
	if err != nil {
		t.Fatalf("viewportSize: %v", err)
	}
	if scale != 3.0 {
		t.Errorf("viewportSize scale = %v, want 3.0 (1.5 vv * 2.0 dpr)", scale)
	}
}
