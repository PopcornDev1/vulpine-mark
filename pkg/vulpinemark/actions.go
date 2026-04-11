package vulpinemark

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNoAnnotation is returned by action methods when Annotate has not
// yet been called on this Mark.
var ErrNoAnnotation = errors.New("vulpinemark: no cached annotation; call Annotate first")

// ErrLabelNotFound is returned when a label like "@7" does not exist
// in the most recent annotation result.
var ErrLabelNotFound = errors.New("vulpinemark: label not found in last annotation")

// lookupLabel returns the cached element for a label, or an error if
// Annotate has not yet been called or the label is unknown. Safe for
// concurrent use.
func (m *Mark) lookupLabel(label string) (Element, error) {
	m.mu.Lock()
	last := m.lastResult
	m.mu.Unlock()
	if last == nil {
		return Element{}, ErrNoAnnotation
	}
	el, ok := last.Elements[label]
	if !ok {
		return Element{}, fmt.Errorf("%w: %q", ErrLabelNotFound, label)
	}
	return el, nil
}

// center returns the CSS-pixel center coordinates of the element.
func (e Element) center() (float64, float64) {
	return e.X + e.Width/2, e.Y + e.Height/2
}

// scrollIntoView scrolls the page so the element's center lands at the
// current viewport center, then returns the updated (x, y) viewport
// coordinates to click at. If the element already sits entirely inside
// the viewport, no scroll is performed.
func (m *Mark) scrollIntoView(ctx context.Context, el Element) (float64, float64, error) {
	vw, vh, _, err := m.c.viewportSize(ctx)
	if err != nil {
		// If we can't read the viewport, fall back to the cached center.
		cx, cy := el.center()
		return cx, cy, nil
	}
	cx, cy := el.center()
	// If the full element box fits in the viewport, no scroll needed.
	if el.X >= 0 && el.Y >= 0 && el.X+el.Width <= vw && el.Y+el.Height <= vh {
		return cx, cy, nil
	}

	currentScrollX, err := m.c.scrollX(ctx)
	if err != nil {
		currentScrollX = 0
	}
	currentScrollY, err := m.c.scrollY(ctx)
	if err != nil {
		currentScrollY = 0
	}
	// Element center in page-absolute CSS pixels.
	pageCenterX := cx + currentScrollX
	pageCenterY := cy + currentScrollY

	targetScrollX := pageCenterX - vw/2
	if targetScrollX < 0 {
		targetScrollX = 0
	}
	targetScrollY := pageCenterY - vh/2
	if targetScrollY < 0 {
		targetScrollY = 0
	}

	if err := m.c.scrollTo(ctx, targetScrollX, targetScrollY); err != nil {
		return 0, 0, err
	}

	// After scrolling, the element center in viewport coords is
	// (page absolute center - new scroll offset).
	newCx := pageCenterX - targetScrollX
	newCy := pageCenterY - targetScrollY
	return newCx, newCy, nil
}

// Click looks up the element by label, scrolls it into view if needed,
// and dispatches a mousePressed -> mouseReleased pair at its center via
// CDP Input.dispatchMouseEvent.
func (m *Mark) Click(ctx context.Context, label string) error {
	el, err := m.lookupLabel(label)
	if err != nil {
		return err
	}
	cx, cy, err := m.scrollIntoView(ctx, el)
	if err != nil {
		return fmt.Errorf("scroll into view: %w", err)
	}

	press := map[string]interface{}{
		"type":       "mousePressed",
		"x":          cx,
		"y":          cy,
		"button":     "left",
		"buttons":    1,
		"clickCount": 1,
	}
	if err := m.c.callCtx(ctx, "Input.dispatchMouseEvent", press, nil); err != nil {
		return fmt.Errorf("Input.dispatchMouseEvent mousePressed: %w", err)
	}

	release := map[string]interface{}{
		"type":       "mouseReleased",
		"x":          cx,
		"y":          cy,
		"button":     "left",
		"buttons":    0,
		"clickCount": 1,
	}
	if err := m.c.callCtx(ctx, "Input.dispatchMouseEvent", release, nil); err != nil {
		return fmt.Errorf("Input.dispatchMouseEvent mouseReleased: %w", err)
	}
	return nil
}

// Type clicks the element by label and then dispatches Input.insertText to
// enter the provided text. This is the simplest cross-browser "type"
// primitive CDP offers and doesn't depend on per-key layout mapping.
func (m *Mark) Type(ctx context.Context, label string, text string) error {
	if err := m.Click(ctx, label); err != nil {
		return err
	}
	// Give the page a brief moment to register focus.
	select {
	case <-time.After(20 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := m.c.callCtx(ctx, "Input.insertText", map[string]interface{}{"text": text}, nil); err != nil {
		return fmt.Errorf("Input.insertText: %w", err)
	}
	return nil
}

// Hover scrolls the element into view if needed and dispatches a
// mouseMoved event at its center.
func (m *Mark) Hover(ctx context.Context, label string) error {
	el, err := m.lookupLabel(label)
	if err != nil {
		return err
	}
	cx, cy, err := m.scrollIntoView(ctx, el)
	if err != nil {
		return fmt.Errorf("scroll into view: %w", err)
	}
	params := map[string]interface{}{
		"type":    "mouseMoved",
		"x":       cx,
		"y":       cy,
		"button":  "none",
		"buttons": 0,
	}
	if err := m.c.callCtx(ctx, "Input.dispatchMouseEvent", params, nil); err != nil {
		return fmt.Errorf("Input.dispatchMouseEvent mouseMoved: %w", err)
	}
	return nil
}
