package vulpinemark

import (
	"context"
	"fmt"
)

// AnnotateJSON enumerates visible interactive elements and returns a
// Result with Elements/Labels populated but no screenshot. Useful
// when consumers only need the structured element list (for example,
// feeding a text-only LLM) and don't want to pay the capture cost.
//
// Result.Image is nil. Clusters are not computed; use Annotate or
// AnnotateClustered if you need visual output or clustering.
func (m *Mark) AnnotateJSON(ctx context.Context) (*Result, error) {
	elements, err := m.c.enumerate(ctx, true)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	maxEls := m.maxElements
	useStable := m.stableLabels
	filter := m.filter
	m.mu.Unlock()

	if filter != nil {
		kept := elements[:0]
		for _, e := range elements {
			if filter(e) {
				kept = append(kept, e)
			}
		}
		elements = kept
	}

	elements = applyMaxElements(elements, 0, maxEls)
	if useStable {
		assignStableLabels(elements)
	} else {
		for i := range elements {
			elements[i].Label = labelFor(i)
		}
	}

	byLabel := make(map[string]Element, len(elements))
	labels := make([]string, 0, len(elements))
	for _, el := range elements {
		byLabel[el.Label] = el
		labels = append(labels, el.Label)
	}
	result := &Result{
		Image:    nil,
		Elements: byLabel,
		Labels:   labels,
	}
	m.mu.Lock()
	m.lastResult = result
	m.mu.Unlock()
	return result, nil
}

// ensureLabels is a small guard used in tests to detect regressions
// that leave labels empty after enumeration.
func ensureLabels(labels []string) error {
	for i, l := range labels {
		if l == "" {
			return fmt.Errorf("label at index %d is empty", i)
		}
	}
	return nil
}
