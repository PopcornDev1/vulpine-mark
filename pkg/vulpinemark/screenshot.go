package vulpinemark

import (
	"encoding/base64"
	"fmt"
)

// captureScreenshot returns a PNG of the current viewport.
func (c *cdpClient) captureScreenshot() ([]byte, error) {
	type result struct {
		Data string `json:"data"`
	}
	var res result
	err := c.call("Page.captureScreenshot", map[string]interface{}{
		"format":      "png",
		"fromSurface": true,
	}, &res)
	if err != nil {
		return nil, fmt.Errorf("Page.captureScreenshot: %w", err)
	}
	if res.Data == "" {
		return nil, fmt.Errorf("Page.captureScreenshot returned empty data")
	}
	png, err := base64.StdEncoding.DecodeString(stripDataURI(res.Data))
	if err != nil {
		return nil, fmt.Errorf("decode screenshot base64: %w", err)
	}
	return png, nil
}

// captureFullPageScreenshot returns a PNG of the entire scrollable page
// by using Page.captureScreenshot with captureBeyondViewport=true.
func (c *cdpClient) captureFullPageScreenshot() ([]byte, error) {
	type result struct {
		Data string `json:"data"`
	}
	var res result
	err := c.call("Page.captureScreenshot", map[string]interface{}{
		"format":                "png",
		"fromSurface":           true,
		"captureBeyondViewport": true,
	}, &res)
	if err != nil {
		return nil, fmt.Errorf("Page.captureScreenshot (full page): %w", err)
	}
	if res.Data == "" {
		return nil, fmt.Errorf("Page.captureScreenshot returned empty data")
	}
	png, err := base64.StdEncoding.DecodeString(stripDataURI(res.Data))
	if err != nil {
		return nil, fmt.Errorf("decode screenshot base64: %w", err)
	}
	return png, nil
}

// viewportSize fetches the current visual viewport in CSS pixels and the
// device pixel ratio so we can map element rects (CSS px) onto screenshot
// pixels.
func (c *cdpClient) viewportSize() (width, height float64, scale float64, err error) {
	type layout struct {
		VisualViewport struct {
			ClientWidth  float64 `json:"clientWidth"`
			ClientHeight float64 `json:"clientHeight"`
			Scale        float64 `json:"scale"`
		} `json:"visualViewport"`
	}
	var res layout
	if err := c.call("Page.getLayoutMetrics", nil, &res); err != nil {
		return 0, 0, 0, fmt.Errorf("Page.getLayoutMetrics: %w", err)
	}
	s := res.VisualViewport.Scale
	if s == 0 {
		s = 1
	}
	return res.VisualViewport.ClientWidth, res.VisualViewport.ClientHeight, s, nil
}
