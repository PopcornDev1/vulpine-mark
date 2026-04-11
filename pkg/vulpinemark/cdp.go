package vulpinemark

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type cdpClient struct {
	conn   *websocket.Conn
	nextID int64

	mu      sync.Mutex
	pending map[int64]chan rpcResponse

	closeOnce sync.Once
	closed    chan struct{}
}

type rpcRequest struct {
	ID     int64       `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
	Method string          `json:"method,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("cdp error %d: %s", e.Code, e.Message)
}

func dialCDP(endpoint string) (*cdpClient, error) {
	wsURL, err := resolveWSURL(endpoint)
	if err != nil {
		return nil, err
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   1 << 20,
		WriteBufferSize:  1 << 20,
	}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", wsURL, err)
	}
	conn.SetReadLimit(64 << 20)

	c := &cdpClient{
		conn:    conn,
		pending: make(map[int64]chan rpcResponse),
		closed:  make(chan struct{}),
	}
	go c.readLoop()
	return c, nil
}

func resolveWSURL(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	switch u.Scheme {
	case "ws", "wss":
		return endpoint, nil
	case "http", "https":
		return discoverPageWS(u)
	default:
		return "", fmt.Errorf("unsupported scheme %q (use http://, https://, ws://, or wss://)", u.Scheme)
	}
}

func discoverPageWS(base *url.URL) (string, error) {
	listURL := *base
	listURL.Path = "/json/list"
	resp, err := http.Get(listURL.String())
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", listURL.String(), err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var targets []struct {
		Type                 string `json:"type"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &targets); err != nil {
		return "", fmt.Errorf("decode /json/list: %w", err)
	}
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			return t.WebSocketDebuggerURL, nil
		}
	}
	return "", errors.New("no page target found at " + listURL.String())
}

func (c *cdpClient) readLoop() {
	defer close(c.closed)
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.failPending(err)
			return
		}
		var resp rpcResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}
		if resp.ID == 0 {
			continue
		}
		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.mu.Unlock()
		if ok {
			ch <- resp
		}
	}
}

func (c *cdpClient) failPending(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, ch := range c.pending {
		ch <- rpcResponse{ID: id, Error: &rpcError{Message: err.Error()}}
		delete(c.pending, id)
	}
}

func (c *cdpClient) call(method string, params interface{}, out interface{}) error {
	id := atomic.AddInt64(&c.nextID, 1)
	ch := make(chan rpcResponse, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	req := rpcRequest{ID: id, Method: method, Params: params}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return resp.Error
		}
		if out != nil && len(resp.Result) > 0 {
			return json.Unmarshal(resp.Result, out)
		}
		return nil
	case <-time.After(30 * time.Second):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return fmt.Errorf("cdp call %s timed out", method)
	}
}

func (c *cdpClient) close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.conn.Close()
	})
	return err
}

// stripDataURI strips a base64 data URI prefix if present.
func stripDataURI(s string) string {
	if i := strings.Index(s, ","); i >= 0 && strings.HasPrefix(s, "data:") {
		return s[i+1:]
	}
	return s
}
