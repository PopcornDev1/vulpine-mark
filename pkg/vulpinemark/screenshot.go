package vulpinemark

import (
	"context"
	"encoding/base64"
	"fmt"
)

// captureScreenshot returns a PNG of the current viewport.
func (c *cdpClient) captureScreenshot(ctx context.Context) ([]byte, error) {
	type result struct {
		Data string `json:"data"`
	}
	var res result
	err := c.callCtx(ctx, "Page.captureScreenshot", map[string]interface{}{
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
func (c *cdpClient) captureFullPageScreenshot(ctx context.Context) ([]byte, error) {
	type result struct {
		Data string `json:"data"`
	}
	var res result
	err := c.callCtx(ctx, "Page.captureScreenshot", map[string]interface{}{
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

// viewportSize fetches the current visual viewport in CSS pixels and
// the total screenshot-pixel scale factor, which is the product of the
// visual viewport zoom (visualViewport.scale) and the device pixel ratio
// (window.devicePixelRatio). Element bounding rects are reported in CSS
// pixels; multiply by the returned scale to map onto raster screenshot
// pixels. Prior versions returned only visualViewport.scale, which on
// Retina displays left badges rendered at half-size because the PNG is
// produced at devicePixelRatio (typically 2x) but visualViewport.scale
// is the zoom level (typically 1.0).
func (c *cdpClient) viewportSize(ctx context.Context) (width, height float64, scale float64, err error) {
	type layout struct {
		VisualViewport struct {
			ClientWidth  float64 `json:"clientWidth"`
			ClientHeight float64 `json:"clientHeight"`
			Scale        float64 `json:"scale"`
		} `json:"visualViewport"`
	}
	var res layout
	if err := c.callCtx(ctx, "Page.getLayoutMetrics", nil, &res); err != nil {
		return 0, 0, 0, fmt.Errorf("Page.getLayoutMetrics: %w", err)
	}
	dpr, derr := c.devicePixelRatio(ctx)
	if derr != nil {
		dpr = 1
	}
	return res.VisualViewport.ClientWidth, res.VisualViewport.ClientHeight,
		combineScale(res.VisualViewport.Scale, dpr), nil
}

// combineScale returns the total screenshot-pixel scale factor given
// the visual viewport zoom and the device pixel ratio, replacing
// zero or negative inputs with 1.0. Extracted for unit testing since
// the live viewportSize path requires a real CDP websocket.
func combineScale(visualViewportScale, devicePixelRatio float64) float64 {
	vv := visualViewportScale
	if vv <= 0 {
		vv = 1
	}
	dpr := devicePixelRatio
	if dpr <= 0 {
		dpr = 1
	}
	return vv * dpr
}

// devicePixelRatio queries the live window.devicePixelRatio via
// Runtime.evaluate. Returns 1.0 as a safe default if the evaluation
// fails or the page is cross-origin isolated in a way that rejects
// Runtime access.
func (c *cdpClient) devicePixelRatio(ctx context.Context) (float64, error) {
	type remoteObject struct {
		Value float64 `json:"value"`
	}
	type evalResult struct {
		Result remoteObject `json:"result"`
	}
	var res evalResult
	err := c.callCtx(ctx, "Runtime.evaluate", map[string]interface{}{
		"expression":    "window.devicePixelRatio",
		"returnByValue": true,
	}, &res)
	if err != nil {
		return 1, err
	}
	if res.Result.Value == 0 {
		return 1, nil
	}
	return res.Result.Value, nil
}
