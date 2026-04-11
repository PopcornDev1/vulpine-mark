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

// DiffKind classifies a changed element as either newly appeared or
// moved from a prior location.
type DiffKind int

const (
	// DiffNew means the element's identity (tag|role|text) was not
	// present in the previous Result at all.
	DiffNew DiffKind = iota
	// DiffMoved means the identity was present before but its
	// coordinates (or size) have shifted beyond the diff rounding
	// bucket.
	DiffMoved
)

// DiffEntry pairs a changed element with its diff classification.
type DiffEntry struct {
	Element Element
	Kind    DiffKind
}

// diffElements returns the subset of `current` whose elements are new
// or moved relative to `prev`, tagged with their DiffKind. An element
// is DiffNew if its identity (tag|role|text) is not present in prev,
// and DiffMoved if the identity exists but the full stability key
// (which includes rounded coordinates) differs.
func diffElements(prev, current []Element) []DiffEntry {
	prevKeys := make(map[string]struct{}, len(prev))
	prevIdentities := make(map[string]struct{}, len(prev))
	for _, e := range prev {
		prevKeys[elementKey(e)] = struct{}{}
		prevIdentities[elementIdentity(e)] = struct{}{}
	}

	changed := make([]DiffEntry, 0)
	for _, e := range current {
		if _, ok := prevKeys[elementKey(e)]; ok {
			continue
		}
		kind := DiffNew
		if _, seen := prevIdentities[elementIdentity(e)]; seen {
			// Same tag|role|text as a prior element but different
			// bucketed coordinates → moved, not new.
			kind = DiffMoved
		}
		changed = append(changed, DiffEntry{Element: e, Kind: kind})
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

	entries := diffElements(prevElements, elements)
	// Labels: new elements are prefixed "*@N" (freshly-appeared),
	// moved elements are prefixed "~@N" (same identity, new position).
	// The prefix lets an agent spot which changes are novel vs. which
	// are just layout shifts.
	changed := make([]Element, len(entries))
	for i, entry := range entries {
		el := entry.Element
		prefix := "*"
		if entry.Kind == DiffMoved {
			prefix = "~"
		}
		el.Label = prefix + labelFor(i)
		changed[i] = el
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
