// Package vulpinemark annotates browser screenshots with numbered labels
// over interactive elements. AI agents can then act by label ("click @14")
// instead of guessing pixel coordinates.
package vulpinemark

// Mark is a connection to a CDP-speaking browser. Construct one with New
// and call Annotate to capture and label the active page.
type Mark struct {
	c          *cdpClient
	lastResult *Result
}

// Result is the output of a single Annotate call.
type Result struct {
	// Image is the annotated screenshot encoded as PNG.
	Image []byte
	// Elements maps "@N" labels to their element metadata.
	Elements map[string]Element
	// Labels lists all labels in document order.
	Labels []string
}

// New connects to the given CDP endpoint. Endpoint may be:
//   - a page-level WebSocket URL (ws://host:port/devtools/page/<id>)
//   - a browser-level HTTP URL (http://host:port) — the first page target
//     is auto-discovered via /json/list
func New(endpoint string) (*Mark, error) {
	c, err := dialCDP(endpoint)
	if err != nil {
		return nil, err
	}
	return &Mark{c: c}, nil
}

// Close releases the CDP connection.
func (m *Mark) Close() error {
	return m.c.close()
}

// Annotate captures the current viewport, enumerates visible interactive
// elements, and returns a labeled PNG plus element map.
func (m *Mark) Annotate() (*Result, error) {
	return m.annotate(true, false)
}

// AnnotateFullPage captures the entire scrollable page (not just the
// viewport) and labels every interactive element on it, including those
// currently scrolled off-screen. Uses Page.captureScreenshot with
// captureBeyondViewport=true.
func (m *Mark) AnnotateFullPage() (*Result, error) {
	return m.annotate(false, true)
}

func (m *Mark) annotate(viewportOnly, fullPage bool) (*Result, error) {
	elements, err := m.c.enumerate(viewportOnly)
	if err != nil {
		return nil, err
	}
	for i := range elements {
		elements[i].Label = labelFor(i)
	}

	var shot []byte
	if fullPage {
		shot, err = m.c.captureFullPageScreenshot()
	} else {
		shot, err = m.c.captureScreenshot()
	}
	if err != nil {
		return nil, err
	}

	_, _, scale, err := m.c.viewportSize()
	if err != nil {
		// Non-fatal: assume 1.0 if layout metrics unavailable.
		scale = 1.0
	}

	annotated, err := drawAnnotations(shot, elements, scale)
	if err != nil {
		return nil, err
	}

	byLabel := make(map[string]Element, len(elements))
	labels := make([]string, 0, len(elements))
	for _, el := range elements {
		byLabel[el.Label] = el
		labels = append(labels, el.Label)
	}

	result := &Result{
		Image:    annotated,
		Elements: byLabel,
		Labels:   labels,
	}
	m.lastResult = result
	return result, nil
}

// LastResult returns the Result from the most recent successful Annotate
// call, or nil if Annotate has not been called yet.
func (m *Mark) LastResult() *Result {
	return m.lastResult
}
