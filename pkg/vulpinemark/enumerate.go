package vulpinemark

import (
	"context"
	"encoding/json"
	"fmt"
)

// Element describes a single interactive element on the page.
type Element struct {
	// Label is the assigned badge identifier, e.g. "@1".
	Label string `json:"label"`
	// Tag is the lowercase HTML tag name (a, button, input, select, textarea).
	Tag string `json:"tag"`
	// Role is the semantic role of the element (button, link, input, etc).
	Role string `json:"role"`
	// Text is the best-effort accessible name of the element.
	Text string `json:"text"`
	// X is the viewport-relative CSS-pixel left of the bounding rect.
	X float64 `json:"x"`
	// Y is the viewport-relative CSS-pixel top of the bounding rect.
	Y float64 `json:"y"`
	// Width is the element's CSS-pixel width. JSON tag stays "w" for wire
	// compatibility.
	Width float64 `json:"w"`
	// Height is the element's CSS-pixel height. JSON tag stays "h" for wire
	// compatibility.
	Height float64 `json:"h"`
}

// enumerateJSTemplate is rendered with %t for the viewport-filter toggle.
// When the flag is false, elements outside the current viewport are kept
// (full-page mode). In both modes occlusion is checked via elementFromPoint.
const enumerateJSTemplate = `
((viewportOnly) => {
  const SELECTOR = [
    'a[href]',
    'button',
    'input:not([type="hidden"])',
    'select',
    'textarea',
    '[role="button"]',
    '[role="link"]',
    '[role="checkbox"]',
    '[role="menuitem"]',
    '[role="tab"]',
    '[role="radio"]',
    '[role="switch"]',
    '[contenteditable="true"]',
    '[tabindex]:not([tabindex="-1"])',
    '[onclick]'
  ].join(',');

  const vw = window.innerWidth;
  const vh = window.innerHeight;

  function isVisible(el, rect, style) {
    if (rect.width < 4 || rect.height < 4) return false;
    if (style.display === 'none') return false;
    if (style.visibility === 'hidden' || style.visibility === 'collapse') return false;
    if (parseFloat(style.opacity || '1') < 0.05) return false;
    if (viewportOnly) {
      if (rect.bottom < 0 || rect.right < 0) return false;
      if (rect.left > vw || rect.top > vh) return false;
    }
    if (el.getAttribute && el.getAttribute('aria-hidden') === 'true') return false;
    return true;
  }

  // isOccluded returns true if elementFromPoint at the center of rect
  // returns a node that is neither the candidate nor one of its
  // descendants. Only runs for elements currently inside the viewport —
  // elementFromPoint is undefined for points outside the visible area.
  function isOccluded(el, rect) {
    const cx = rect.left + rect.width / 2;
    const cy = rect.top + rect.height / 2;
    if (cx < 0 || cy < 0 || cx > vw || cy > vh) return false;
    const hit = document.elementFromPoint(cx, cy);
    if (!hit) return false;
    if (hit === el) return false;
    if (el.contains && el.contains(hit)) return false;
    if (hit.contains && hit.contains(el)) return false;
    return true;
  }

  function classify(el) {
    const tag = el.tagName.toLowerCase();
    const role = (el.getAttribute && el.getAttribute('role')) || '';
    if (role) return { tag, role };
    if (tag === 'a') return { tag, role: 'link' };
    if (tag === 'button') return { tag, role: 'button' };
    if (tag === 'select') return { tag, role: 'select' };
    if (tag === 'textarea') return { tag, role: 'textarea' };
    if (tag === 'input') {
      const t = (el.type || 'text').toLowerCase();
      if (t === 'submit' || t === 'button' || t === 'reset') return { tag, role: 'button' };
      if (t === 'checkbox') return { tag, role: 'checkbox' };
      if (t === 'radio') return { tag, role: 'radio' };
      return { tag, role: 'input' };
    }
    return { tag, role: tag };
  }

  function nameOf(el) {
    const aria = el.getAttribute && el.getAttribute('aria-label');
    if (aria) return aria.trim();
    const labelledBy = el.getAttribute && el.getAttribute('aria-labelledby');
    if (labelledBy) {
      const ref = document.getElementById(labelledBy);
      if (ref && ref.textContent) return ref.textContent.trim();
    }
    if (el.placeholder) return el.placeholder.trim();
    if (el.value && (el.tagName === 'INPUT' || el.tagName === 'BUTTON')) return String(el.value).trim();
    if (el.alt) return el.alt.trim();
    if (el.title) return el.title.trim();
    const text = (el.innerText || el.textContent || '').trim().replace(/\s+/g, ' ');
    return text;
  }

  const seen = new Set();
  const out = [];
  const all = document.querySelectorAll(SELECTOR);
  for (const el of all) {
    if (seen.has(el)) continue;
    seen.add(el);
    const rect = el.getBoundingClientRect();
    const style = window.getComputedStyle(el);
    if (!isVisible(el, rect, style)) continue;
    if (isOccluded(el, rect)) continue;
    const { tag, role } = classify(el);
    let text = nameOf(el);
    if (text.length > 80) text = text.slice(0, 77) + '...';
    out.push({
      tag, role, text,
      x: rect.left, y: rect.top, w: rect.width, h: rect.height
    });
  }
  return JSON.stringify(out);
})(%t)
`

// enumerate returns the visible interactive elements on the active page,
// in document order. Labels are not yet assigned. When viewportOnly is
// true, elements outside the current viewport are filtered out.
func (c *cdpClient) enumerate(ctx context.Context, viewportOnly bool) ([]Element, error) {
	type evalResult struct {
		Result struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}

	var res evalResult
	expr := fmt.Sprintf(enumerateJSTemplate, viewportOnly)
	err := c.callCtx(ctx, "Runtime.evaluate", map[string]interface{}{
		"expression":    expr,
		"returnByValue": true,
		"awaitPromise":  false,
	}, &res)
	if err != nil {
		return nil, fmt.Errorf("Runtime.evaluate: %w", err)
	}
	if res.ExceptionDetails != nil {
		return nil, fmt.Errorf("page exception: %s", res.ExceptionDetails.Text)
	}
	var raw string
	if err := json.Unmarshal(res.Result.Value, &raw); err != nil {
		return nil, fmt.Errorf("decode value as string: %w", err)
	}
	var elements []Element
	if err := json.Unmarshal([]byte(raw), &elements); err != nil {
		return nil, fmt.Errorf("decode elements: %w", err)
	}
	return elements, nil
}

// scrollX reads window.scrollX via Runtime.evaluate.
func (c *cdpClient) scrollX(ctx context.Context) (float64, error) {
	return c.evalFloat(ctx, "window.scrollX")
}

// scrollY reads window.scrollY via Runtime.evaluate.
func (c *cdpClient) scrollY(ctx context.Context) (float64, error) {
	return c.evalFloat(ctx, "window.scrollY")
}

// scrollTo performs a synchronous window.scrollTo to the given page
// offsets (CSS pixels).
func (c *cdpClient) scrollTo(ctx context.Context, x, y float64) error {
	expr := fmt.Sprintf("window.scrollTo(%f, %f)", x, y)
	return c.callCtx(ctx, "Runtime.evaluate", map[string]interface{}{
		"expression":    expr,
		"returnByValue": true,
		"awaitPromise":  false,
	}, nil)
}

func (c *cdpClient) evalFloat(ctx context.Context, expr string) (float64, error) {
	type evalResult struct {
		Result struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	}
	var res evalResult
	err := c.callCtx(ctx, "Runtime.evaluate", map[string]interface{}{
		"expression":    expr,
		"returnByValue": true,
		"awaitPromise":  false,
	}, &res)
	if err != nil {
		return 0, err
	}
	if len(res.Result.Value) == 0 {
		return 0, nil
	}
	var f float64
	if err := json.Unmarshal(res.Result.Value, &f); err != nil {
		return 0, err
	}
	return f, nil
}
