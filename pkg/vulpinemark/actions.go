package vulpinemark

import (
	"fmt"
	"time"
)

// lookupLabel returns the cached element for a label, or an error if
// Annotate has not yet been called or the label is unknown.
func (m *Mark) lookupLabel(label string) (Element, error) {
	if m.lastResult == nil {
		return Element{}, fmt.Errorf("no cached annotation; call Annotate first")
	}
	el, ok := m.lastResult.Elements[label]
	if !ok {
		return Element{}, fmt.Errorf("label %q not found in last annotation", label)
	}
	return el, nil
}

// center returns the CSS-pixel center coordinates of the element.
func (e Element) center() (float64, float64) {
	return e.X + e.W/2, e.Y + e.H/2
}

// Click looks up the element by label and dispatches a mousePressed →
// mouseReleased pair at its center via CDP Input.dispatchMouseEvent.
func (m *Mark) Click(label string) error {
	el, err := m.lookupLabel(label)
	if err != nil {
		return err
	}
	cx, cy := el.center()

	press := map[string]interface{}{
		"type":       "mousePressed",
		"x":          cx,
		"y":          cy,
		"button":     "left",
		"buttons":    1,
		"clickCount": 1,
	}
	if err := m.c.call("Input.dispatchMouseEvent", press, nil); err != nil {
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
	if err := m.c.call("Input.dispatchMouseEvent", release, nil); err != nil {
		return fmt.Errorf("Input.dispatchMouseEvent mouseReleased: %w", err)
	}
	return nil
}

// Type clicks the element by label and then dispatches Input.insertText to
// enter the provided text. This is the simplest cross-browser "type"
// primitive CDP offers and doesn't depend on per-key layout mapping.
func (m *Mark) Type(label string, text string) error {
	if err := m.Click(label); err != nil {
		return err
	}
	// Give the page a brief moment to register focus.
	time.Sleep(20 * time.Millisecond)
	if err := m.c.call("Input.insertText", map[string]interface{}{"text": text}, nil); err != nil {
		return fmt.Errorf("Input.insertText: %w", err)
	}
	return nil
}

// Hover dispatches a mouseMoved event at the element's center.
func (m *Mark) Hover(label string) error {
	el, err := m.lookupLabel(label)
	if err != nil {
		return err
	}
	cx, cy := el.center()
	params := map[string]interface{}{
		"type":    "mouseMoved",
		"x":       cx,
		"y":       cy,
		"button":  "none",
		"buttons": 0,
	}
	if err := m.c.call("Input.dispatchMouseEvent", params, nil); err != nil {
		return fmt.Errorf("Input.dispatchMouseEvent mouseMoved: %w", err)
	}
	return nil
}
