package vulpinemark

import (
	"context"
	"fmt"
)

// diffRoundPx is the rounding granularity applied to coordinates when
// computing element stability keys for diff mode. Small animation
// jitter within this bucket is ignored.
const diffRoundPx = 4

// elementKey returns a stable fingerprint for diffing. Coordinates are
// rounded so minor layout jitter does not produce false positives.
func elementKey(e Element) string {
	return fmt.Sprintf("%s|%s|%s|%d|%d|%d|%d",
		e.Tag, e.Role, e.Text,
		roundTo(e.X, diffRoundPx),
		roundTo(e.Y, diffRoundPx),
		roundTo(e.Width, diffRoundPx),
		roundTo(e.Height, diffRoundPx),
	)
}

// elementIdentity returns a coord-free key matching tag|role|text so
// an element that moved can be detected as "same logical element, new
// position".
func elementIdentity(e Element) string {
	return fmt.Sprintf("%s|%s|%s", e.Tag, e.Role, e.Text)
}

// diffElements returns the subset of `current` whose elements are new
// or moved relative to `prev`. An element is "new" if its identity
// (tag|role|text) is not present in prev, and "moved" if the identity
// exists but the full stability key differs.
func diffElements(prev, current []Element) []Element {
	prevKeys := make(map[string]struct{}, len(prev))
	prevIdentities := make(map[string]struct{}, len(prev))
	for _, e := range prev {
		prevKeys[elementKey(e)] = struct{}{}
		prevIdentities[elementIdentity(e)] = struct{}{}
	}

	changed := make([]Element, 0)
	for _, e := range current {
		if _, ok := prevKeys[elementKey(e)]; ok {
			continue
		}
		// Either brand new or moved — both count as changed.
		_ = prevIdentities // reserved for future move-vs-new distinction
		changed = append(changed, e)
	}
	return changed
}

// AnnotateDiff re-captures the page and draws labels only on elements
// that are new or moved relative to the supplied previous Result.
// Useful for modal detection, before/after action verification, and
// minimizing label-churn noise in agent prompts. If prev is nil the
// call degrades to a regular Annotate.
func (m *Mark) AnnotateDiff(ctx context.Context, prev *Result) (*Result, error) {
	if prev == nil {
		return m.Annotate(ctx)
	}

	elements, err := m.c.enumerate(ctx, true)
	if err != nil {
		return nil, err
	}

	// Reconstruct the previous element list from the cached map so we
	// can compute stability keys against it.
	prevElements := make([]Element, 0, len(prev.Elements))
	for _, el := range prev.Elements {
		prevElements = append(prevElements, el)
	}
	for _, cl := range prev.Clusters {
		prevElements = append(prevElements, cl.Members...)
	}

	changed := diffElements(prevElements, elements)
	for i := range changed {
		changed[i].Label = "*" + labelFor(i)
	}

	shot, err := m.c.captureScreenshot(ctx)
	if err != nil {
		return nil, err
	}
	_, _, scale, err := m.c.viewportSize(ctx)
	if err != nil {
		scale = 1.0
	}

	annotated, err := drawAnnotations(shot, changed, scale)
	if err != nil {
		return nil, err
	}

	byLabel := make(map[string]Element, len(changed))
	labels := make([]string, 0, len(changed))
	for _, el := range changed {
		byLabel[el.Label] = el
		labels = append(labels, el.Label)
	}

	result := &Result{
		Image:    annotated,
		Elements: byLabel,
		Labels:   labels,
	}
	m.mu.Lock()
	m.lastResult = result
	m.mu.Unlock()
	return result, nil
}
