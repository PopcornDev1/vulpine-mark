package vulpinemark

import (
	"context"
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

// defaultCallTimeout is the per-call timeout used by call() when the
// caller-provided context has no deadline of its own.
const defaultCallTimeout = 30 * time.Second

type cdpClient struct {
	conn   *websocket.Conn
	nextID int64

	// writeMu serializes WriteMessage calls. gorilla/websocket
	// explicitly forbids concurrent writers — without this, two
	// concurrent call() invocations would corrupt frames or panic.
	writeMu sync.Mutex

	mu      sync.Mutex
	pending map[int64]chan rpcResponse

	closeOnce sync.Once
	closed    chan struct{}

	// httpClient is used for /json/list discovery. May be nil, in which
	// case a default client is used.
	httpClient *http.Client
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

func dialCDP(ctx context.Context, endpoint string, httpClient *http.Client) (*cdpClient, error) {
	wsURL, err := resolveWSURL(ctx, endpoint, httpClient)
	if err != nil {
		return nil, err
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   1 << 20,
		WriteBufferSize:  1 << 20,
	}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", wsURL, err)
	}
	conn.SetReadLimit(64 << 20)

	c := &cdpClient{
		conn:       conn,
		pending:    make(map[int64]chan rpcResponse),
		closed:     make(chan struct{}),
		httpClient: httpClient,
	}
	go c.readLoop()
	return c, nil
}

func resolveWSURL(ctx context.Context, endpoint string, httpClient *http.Client) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	switch u.Scheme {
	case "ws", "wss":
		return endpoint, nil
	case "http", "https":
		return discoverPageWS(ctx, u, httpClient)
	default:
		return "", fmt.Errorf("unsupported scheme %q (use http://, https://, ws://, or wss://)", u.Scheme)
	}
}

// discoverListLimit caps the /json/list response size. Real Chrome
// payloads are a few KB; anything beyond this is hostile or broken.
const discoverListLimit = 1 << 20 // 1 MiB

func discoverPageWS(ctx context.Context, base *url.URL, httpClient *http.Client) (string, error) {
	listURL := *base
	listURL.Path = "/json/list"

	client := httpClient
	if client == nil {
		client = &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 1 {
					return errors.New("/json/list: redirects not allowed")
				}
				return nil
			},
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", listURL.String(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET %s: status %d", listURL.String(), resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, discoverListLimit+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > discoverListLimit {
		return "", fmt.Errorf("GET %s: body exceeds %d bytes", listURL.String(), discoverListLimit)
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

// callCtx dispatches a CDP method and waits for the response, honoring
// the provided context for cancellation. If ctx has no deadline, a
// defaultCallTimeout is applied.
func (c *cdpClient) callCtx(ctx context.Context, method string, params interface{}, out interface{}) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultCallTimeout)
		defer cancel()
	}

	id := atomic.AddInt64(&c.nextID, 1)
	ch := make(chan rpcResponse, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	req := rpcRequest{ID: id, Method: method, Params: params}
	data, err := json.Marshal(req)
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return err
	}
	c.writeMu.Lock()
	werr := c.conn.WriteMessage(websocket.TextMessage, data)
	c.writeMu.Unlock()
	if werr != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return werr
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
	case <-c.closed:
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return fmt.Errorf("cdp call %s: connection closed", method)
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return fmt.Errorf("cdp call %s: %w", method, ctx.Err())
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
