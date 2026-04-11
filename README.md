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
vulpine-mark --cdp ws://localhost:9222 --output annotated.png --json elements.json
```

## Library usage

```go
import "github.com/PopcornDev1/vulpine-mark/pkg/vulpinemark"

mark, err := vulpinemark.New("ws://localhost:9222")
if err != nil { panic(err) }
defer mark.Close()

result, err := mark.Annotate(context.Background())
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
	"fmt"
	"os"

	"github.com/PopcornDev1/vulpine-mark/pkg/vulpinemark"
)

func main() {
	// Connect to a running Chrome / Camoufox with --remote-debugging-port=9222.
	mark, err := vulpinemark.New("http://localhost:9222")
	if err != nil {
		panic(err)
	}
	defer mark.Close()

	// Capture the viewport and label every interactive element.
	result, err := mark.Annotate()
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("annotated.png", result.Image, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("labeled %d elements\n", len(result.Labels))

	// Pick a label — an AI agent would read the PNG and reply with one.
	if _, ok := result.Elements["@1"]; ok {
		if err := mark.Click("@1"); err != nil {
			panic(err)
		}
	}

	// Type into a labeled input, then hover another element.
	_ = mark.Type("@3", "hello world")
	_ = mark.Hover("@7")
}
```

Use `mark.AnnotateFullPage()` (or the `--full-page` CLI flag) to capture
and label the entire scrollable page, including elements currently below
the fold.

## Status

Early MVP. Currently supports Chrome and Camoufox/foxbridge over CDP WebSocket. iOS Safari support is on the roadmap via [MobileBridge](https://github.com/PopcornDev1/mobilebridge).

## License

MIT
