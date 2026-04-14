# Vulpine Mark

Visual element labeling for AI browser agents.

Vulpine Mark annotates browser screenshots with numbered badges over interactive elements (`@1`, `@2`, `@3`...) and returns a structured element map alongside the image. AI agents can then say *"click @14"* instead of guessing pixel coordinates — boosting click accuracy by 20-30% on visual grounding benchmarks.

## Why

Agents that drive browsers from screenshots alone are bad at clicking. Microsoft's Set-of-Mark (SoM) paper showed that overlaying numbered labels on interactive elements closes most of the gap with element-handle-based agents. The catch: existing implementations need 11GB GPUs (OmniParser), are tied to research benchmarks (WebArena), or are proprietary (Anthropic Computer Use).

Vulpine Mark is the first **standalone**, **GPU-free**, **browser-agnostic** SoM tool. It reads element positions directly from the live DOM via CDP and paints labels in pure Go — no ML models, no GPU, no JS injection beyond a single `Runtime.evaluate` call.

## Features

- **Zero ML overhead** — reads bounding rects from the DOM, draws PNG labels in Go
- **CDP-native** — works with any browser exposing CDP (Chrome, Edge, Camoufox via foxbridge)
- **Color-coded by role** — green buttons, blue links, purple inputs, orange selects
- **Viewport-aware** — only labels elements actually visible to the agent
- **JSON element map** — structured `{label, tag, role, text, x, y, w, h}` for every badge
- **CLI + Go library** — drop into pipelines or import as a package

## Install

```bash
go install github.com/PopcornDev1/vulpine-mark/cmd/vulpine-mark@latest
```

## CLI usage

```bash
# Connect to a running browser, annotate the active page
vulpine-mark --cdp http://localhost:9222 --output annotated.png --json elements.json
```

## Library usage

```go
import (
    "context"
    "github.com/PopcornDev1/vulpine-mark/pkg/vulpinemark"
)

ctx := context.Background()
mark, err := vulpinemark.New(ctx, "http://localhost:9222")
if err != nil { panic(err) }
defer mark.Close()

result, err := mark.Annotate(ctx)
// result.Image     - annotated PNG bytes
// result.Elements  - map[string]Element keyed by "@N"
// result.Labels    - ordered []string
```

## Example

End-to-end: annotate the active page, persist the labeled PNG, then drive
a click by label.

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/PopcornDev1/vulpine-mark/pkg/vulpinemark"
)

func main() {
	ctx := context.Background()

	// Connect to a running Chrome / Camoufox with --remote-debugging-port=9222.
	mark, err := vulpinemark.New(ctx, "http://localhost:9222")
	if err != nil {
		panic(err)
	}
	defer mark.Close()

	// Capture the viewport and label every interactive element.
	result, err := mark.Annotate(ctx)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("annotated.png", result.Image, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("labeled %d elements\n", len(result.Labels))

	// Pick a label — an AI agent would read the PNG and reply with one.
	if _, ok := result.Elements["@1"]; ok {
		if err := mark.Click(ctx, "@1"); err != nil {
			panic(err)
		}
	}

	// Type into a labeled input, then hover another element.
	_ = mark.Type(ctx, "@3", "hello world")
	_ = mark.Hover(ctx, "@7")
}
```

Use `mark.AnnotateFullPage()` (or the `--full-page` CLI flag) to capture
and label the entire scrollable page, including elements currently below
the fold.

### Cluster mode

A typical product grid or list of search results has dozens of visually
identical items. Labeling each one individually drowns the agent in
`@1`, `@2`, `@3`... `@47` label soup. **Cluster mode** detects
repeated shapes and groups them under a single label — members are
accessed with bracket syntax.

```go
result, err := mark.AnnotateClustered(ctx)
// result.Clusters[0].Label == "@C1"
// result.Clusters[0].Members has N Elements

// Click the 3rd item in cluster @C1.
_ = mark.Click(ctx, "@C1[3]")
```

The annotated image draws a single amber outline around the union of
all cluster members plus one `@C<N>[1..count]` badge at the top-left,
instead of one badge per member. Cluster labels use the `@C<N>`
namespace so they never collide with ungrouped element labels
(`@1`, `@2`, ...). Elements that don't fit any cluster are labeled
individually as before.

### Diff mode

Given a previous `Result` from an earlier annotate, `AnnotateDiff`
re-captures the page and labels **only** elements that are new or have
moved. Useful for modal detection, before/after action verification,
and keeping agent prompts focused on what actually changed.

```go
before, _ := mark.Annotate(ctx)
_ = mark.Click(ctx, "@3")
after, _ := mark.AnnotateDiff(ctx, before)
// after.Image highlights only new or moved elements.
// Newly-appeared  elements are labeled "*@1", "*@2", ...
// Moved elements are labeled "~@1", "~@2", ... so the agent
// can tell fresh UI (e.g. a modal) from mere layout shifts.
```

### Palette packs

Four built-in color palettes ship with the library: `DefaultPalette`,
`HighContrastPalette`, `MonochromePalette`, and `ColorblindSafePalette`
(Wong, Nature Methods 2011). Swap palettes at runtime:

```go
mark.SetPalette(vulpinemark.ColorblindSafePalette)
```

From the CLI:

```bash
vulpine-mark --palette colorblind --output annotated.png
# Also: default, high-contrast, monochrome
```

The palette controls element badges and the cluster outline color, so
downstream color-based filters (e.g. "find all buttons") stay consistent
no matter which scheme you pick.

### SVG overlay output

`AnnotateSVG` returns the same `Result` populated with a vector
overlay in `Result.SVG`. The overlay is sized to match the raster
screenshot so frontends can layer it with CSS and toggle labels on
and off:

```go
result, _ := mark.AnnotateSVG(ctx)
os.WriteFile("overlay.svg", []byte(result.SVG), 0o644)
```

Or via CLI:

```bash
vulpine-mark --svg overlay.svg --output screenshot.png
```

### Stable labels across re-renders

By default, labels are assigned by enumeration order, so `@5` may
refer to a different element after a re-annotate if elements shift.
Opt into semantic-hash labels:

```go
mark.UseStableLabels(true)
```

Labels are then derived from `(role, accessible-name, rounded-x,
rounded-y)`, so the same logical element keeps the same label across
re-renders as long as its role, name, and rough position are
unchanged. Useful for long-running agent loops that re-annotate
between every action.

### Max elements cap

On dense pages, cap the number of labeled elements so the agent
prompt stays readable. Clusters count as one each toward the cap;
the lowest-confidence elements are dropped first.

```go
mark.SetMaxElements(20)
// or --max-elements 20
```

### Heatmap mode

Instead of discrete numbered badges, `AnnotateHeatmap` renders a
translucent overlay whose alpha is proportional to each element's
importance (`confidence * log(area + 1)`), normalized across the
returned set so the single most prominent element always saturates the
palette. Useful for visual triage at a glance — "where should the
agent look first?"

```go
result, _ := mark.AnnotateHeatmap(ctx)
os.WriteFile("heatmap.png", result.Image, 0o644)
```

Or from the CLI:

```bash
vulpine-mark --heatmap --output heatmap.png
```

The heatmap honors the currently-configured palette and any
`SetElementFilter` callback, so role-restricted heatmaps work out of
the box:

```go
mark.SetElementFilter(vulpinemark.IncludeRoles("button", "link"))
result, _ := mark.AnnotateHeatmap(ctx)
```

### JSON-only mode

When you only need the structured element list (e.g. feeding a
text-only LLM or building a selector index), skip screenshot capture
entirely:

```go
result, _ := mark.AnnotateJSON(ctx)
// result.Image == nil
// result.Elements, result.Labels populated as usual
```

From the CLI, `--json-only` writes the element map to `--json` (or to
stdout if no JSON path is given) and never touches `--output`:

```bash
vulpine-mark --json-only > elements.json
```

### Element filter callbacks

For fine-grained control over which elements get labeled, pass a
custom `ElementFilter`. It runs after enumeration and visibility /
occlusion checks.

```go
mark.SetElementFilter(func(el vulpinemark.Element) bool {
    return el.Role == "button" && el.Confidence > 0.5
})
```

Two helpers ship for the common cases:

```go
mark.SetElementFilter(vulpinemark.IncludeRoles("button", "link"))
mark.SetElementFilter(vulpinemark.ExcludeRoles("checkbox", "radio"))
```

From the CLI:

```bash
vulpine-mark --include-role button,link --exclude-role checkbox
```

### Confidence scores

Every `Element` returned by the library carries a `Confidence` field
in `[0, 1]` computed from an accessible-name presence, `aria-label`,
area, occlusion, and clipped-overflow status. Labels with confidence
below `0.3` are rendered in a muted gray so the agent knows to be
cautious about acting on them.

## Status

Early MVP. Currently supports Chrome and Camoufox/foxbridge over CDP WebSocket. iOS Safari support is on the roadmap via [MobileBridge](https://github.com/VulpineOS/mobilebridge).

## License

MIT
