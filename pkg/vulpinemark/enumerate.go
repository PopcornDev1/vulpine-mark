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
	// Confidence is a heuristic score in [0,1] indicating how reliably
	// an AI agent can target this element. Low-confidence elements are
	// faded to gray in the annotated image so agents know to be
	// cautious. Computed by enumerate.js from: presence of an
	// accessible name, explicit aria-label, area, occlusion status,
	// and whether the element lives in a clipped overflow.
	Confidence float64 `json:"confidence"`
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

  // isInClippedOverflow walks up the ancestors looking for a block
  // with overflow:hidden/clip/auto/scroll whose clip rect doesn't
  // contain the element's center. Best-effort; returns false on
  // ambiguity.
  function isInClippedOverflow(el, rect) {
    let p = el.parentElement;
    while (p) {
      const ps = window.getComputedStyle(p);
      const ov = ps.overflow + ps.overflowX + ps.overflowY;
      if (/hidden|clip|scroll|auto/.test(ov)) {
        const pr = p.getBoundingClientRect();
        const cx = rect.left + rect.width / 2;
        const cy = rect.top + rect.height / 2;
        if (cx < pr.left || cx > pr.right || cy < pr.top || cy > pr.bottom) {
          return true;
        }
      }
      p = p.parentElement;
    }
    return false;
  }

  function confidenceFor(el, rect, hasName, occluded, clipped) {
    let s = 0;
    if (hasName) s += 0.3;
    if (el.getAttribute && el.getAttribute('aria-label')) s += 0.2;
    if (rect.width * rect.height > 100) s += 0.2;
    if (!occluded) s += 0.2;
    if (!clipped) s += 0.1;
    if (s > 1) s = 1;
    if (s < 0) s = 0;
    return s;
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
    const occluded = isOccluded(el, rect);
    if (occluded) continue;
    const { tag, role } = classify(el);
    let text = nameOf(el);
    if (text.length > 80) text = text.slice(0, 77) + '...';
    const clipped = isInClippedOverflow(el, rect);
    const conf = confidenceFor(el, rect, text.length > 0, occluded, clipped);
    out.push({
      tag, role, text,
      x: rect.left, y: rect.top, w: rect.width, h: rect.height,
      confidence: conf
    });
  }
  return JSON.stringify(out);
})(%t)
`

// confidenceSignals is a Go mirror of the JavaScript confidence
// scoring logic for unit-test purposes. Production code uses the JS
// implementation inside enumerateJSTemplate — keep the two in sync.
type confidenceSignals struct {
	HasName     bool
	HasAriaAttr bool
	Area        float64
	Occluded    bool
	Clipped     bool
}

// computeConfidence returns a score in [0,1] from the given signals.
// Weights mirror confidenceFor() in enumerateJSTemplate:
//
//	+0.3 has accessible name
//	+0.2 has aria-label
//	+0.2 area > 100 px
//	+0.2 not occluded
//	+0.1 not clipped
func computeConfidence(s confidenceSignals) float64 {
	score := 0.0
	if s.HasName {
		score += 0.3
	}
	if s.HasAriaAttr {
		score += 0.2
	}
	if s.Area > 100 {
		score += 0.2
	}
	if !s.Occluded {
		score += 0.2
	}
	if !s.Clipped {
		score += 0.1
	}
	if score > 1 {
		score = 1
	}
	if score < 0 {
		score = 0
	}
	return score
}

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
