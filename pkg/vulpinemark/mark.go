// Package vulpinemark annotates browser screenshots with numbered labels
// over interactive elements. AI agents can then act by label ("click @14")
// instead of guessing pixel coordinates.
package vulpinemark

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
)

// Mark is a connection to a CDP-speaking browser. Construct one with New
// and call Annotate to capture and label the active page. A Mark is safe
// to use from multiple goroutines: the cdp transport serializes writes
// and the cached lastResult is guarded by a mutex.
type Mark struct {
	c *cdpClient

	mu           sync.Mutex
	lastResult   *Result
	palette      Palette
	paletteSet   bool
	maxElements  int
	stableLabels bool
}

// Result is the output of a single Annotate call.
type Result struct {
	// Image is the annotated screenshot encoded as PNG.
	Image []byte
	// Elements maps "@N" labels to their element metadata. In clustered
	// results this contains only the ungrouped (non-cluster) elements.
	Elements map[string]Element
	// Labels lists all top-level labels in document order. For clustered
	// results, cluster labels (e.g. "@5") appear here alongside ordinary
	// element labels; individual cluster members are addressed with
	// "@5[1]", "@5[2]", ... via Click/Type/Hover.
	Labels []string
	// Clusters lists the grouped repeated-element clusters, in cluster-
	// number order. nil for non-clustered results.
	Clusters []Cluster
	// SVG, if populated, is a vector overlay (borders + labels) sized
	// to match Image's dimensions. Set by AnnotateSVG. Empty for
	// plain Annotate calls.
	SVG string
}

// New connects to the given CDP endpoint. Endpoint may be:
//   - a page-level WebSocket URL (ws://host:port/devtools/page/<id>)
//   - a browser-level HTTP URL (http://host:port) — the first page target
//     is auto-discovered via /json/list
//
// The provided context governs the dial (and /json/list discovery, if
// applicable) only. Subsequent calls take their own context.
func New(ctx context.Context, endpoint string) (*Mark, error) {
	c, err := dialCDP(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}
	return &Mark{c: c}, nil
}

// NewWithClient is like New but uses the supplied *http.Client for the
// /json/list discovery step. Useful for tests and for configuring
// custom transports, timeouts, or TLS settings.
func NewWithClient(ctx context.Context, endpoint string, client *http.Client) (*Mark, error) {
	c, err := dialCDP(ctx, endpoint, client)
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
func (m *Mark) Annotate(ctx context.Context) (*Result, error) {
	return m.annotate(ctx, true, false, false)
}

// AnnotateFullPage captures the entire scrollable page (not just the
// viewport) and labels every interactive element on it, including those
// currently scrolled off-screen. Uses Page.captureScreenshot with
// captureBeyondViewport=true.
func (m *Mark) AnnotateFullPage(ctx context.Context) (*Result, error) {
	return m.annotate(ctx, false, true, false)
}

// AnnotateClustered behaves like Annotate but groups visually similar
// repeated elements (e.g. a product grid or list of search results)
// under a single cluster label. Individual cluster members are
// addressed via "@N[K]" syntax with Click, Type, and Hover.
func (m *Mark) AnnotateClustered(ctx context.Context) (*Result, error) {
	return m.annotate(ctx, true, false, true)
}

func (m *Mark) annotate(ctx context.Context, viewportOnly, fullPage, clustered bool) (*Result, error) {
	elements, err := m.c.enumerate(ctx, viewportOnly)
	if err != nil {
		return nil, err
	}

	var shot []byte
	if fullPage {
		shot, err = m.c.captureFullPageScreenshot(ctx)
	} else {
		shot, err = m.c.captureScreenshot(ctx)
	}
	if err != nil {
		return nil, err
	}

	_, _, scale, err := m.c.viewportSize(ctx)
	if err != nil {
		// Non-fatal: assume 1.0 if layout metrics unavailable.
		scale = 1.0
	}

	m.mu.Lock()
	maxEls := m.maxElements
	useStable := m.stableLabels
	m.mu.Unlock()

	var clusters []Cluster
	if clustered {
		var ungrouped []Element
		clusters, ungrouped = clusterElements(elements)
		// Reassign clusters as the lowest labels, then give the
		// remaining ungrouped elements labels continuing after the
		// last cluster.
		for i := range clusters {
			clusters[i].Label = clusterLabelFor(i)
			clusters[i].BBox = clusterBBox(clusters[i].Members, scale)
			for j := range clusters[i].Members {
				clusters[i].Members[j].Label = fmt.Sprintf("%s[%d]", clusters[i].Label, j+1)
			}
		}
		ungrouped = applyMaxElements(ungrouped, len(clusters), maxEls)
		if useStable {
			assignStableLabels(ungrouped)
		} else {
			// Cluster labels live in a separate "@C<N>" namespace so
			// ungrouped elements can restart at @1 without colliding.
			for i := range ungrouped {
				ungrouped[i].Label = labelFor(i)
			}
		}
		elements = ungrouped
	} else {
		elements = applyMaxElements(elements, 0, maxEls)
		if useStable {
			assignStableLabels(elements)
		} else {
			for i := range elements {
				elements[i].Label = labelFor(i)
			}
		}
	}

	annotated, err := drawAnnotationsWithPalette(shot, elements, clusters, scale, m.currentPalette())
	if err != nil {
		return nil, err
	}

	byLabel := make(map[string]Element, len(elements))
	labels := make([]string, 0, len(elements)+len(clusters))
	for _, cl := range clusters {
		labels = append(labels, cl.Label)
	}
	for _, el := range elements {
		byLabel[el.Label] = el
		labels = append(labels, el.Label)
	}

	result := &Result{
		Image:    annotated,
		Elements: byLabel,
		Labels:   labels,
		Clusters: clusters,
	}
	m.mu.Lock()
	m.lastResult = result
	m.mu.Unlock()
	return result, nil
}

// SetMaxElements caps the number of elements that will be labeled in
// subsequent Annotate calls. When the enumerator returns more
// elements than the cap, the lowest-confidence ones are dropped
// until the cap is met. Clusters are counted as one each toward the
// cap so a dense grid does not exhaust the budget. A value of 0 (the
// default) disables the cap.
func (m *Mark) SetMaxElements(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n < 0 {
		n = 0
	}
	m.maxElements = n
}

// applyMaxElements drops the lowest-confidence entries from els until
// len(els)+clusterCount <= max. Returns the pruned slice. Order of
// surviving elements is preserved so label assignment stays in
// document order.
func applyMaxElements(els []Element, clusterCount, max int) []Element {
	if max <= 0 {
		return els
	}
	budget := max - clusterCount
	if budget < 0 {
		budget = 0
	}
	if len(els) <= budget {
		return els
	}
	type indexed struct {
		i int
		e Element
	}
	ix := make([]indexed, len(els))
	for i, e := range els {
		ix[i] = indexed{i: i, e: e}
	}
	sort.SliceStable(ix, func(a, b int) bool {
		return ix[a].e.Confidence > ix[b].e.Confidence
	})
	if budget > len(ix) {
		budget = len(ix)
	}
	kept := ix[:budget]
	sort.SliceStable(kept, func(a, b int) bool {
		return kept[a].i < kept[b].i
	})
	out := make([]Element, len(kept))
	for i, k := range kept {
		out[i] = k.e
	}
	return out
}

// LastResult returns the Result from the most recent successful Annotate
// call, or nil if Annotate has not been called yet.
func (m *Mark) LastResult() *Result {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastResult
}
