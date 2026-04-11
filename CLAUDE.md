# Vulpine Mark — Visual Element Labeling

Standalone Go library + CLI that annotates browser screenshots with numbered badges (`@1`, `@2`...) over interactive elements via CDP. Public open-source repo, drives adoption of the wider VulpineOS ecosystem.

**Repo:** `PopcornDev1/vulpine-mark` (public, MIT)

## Layout

- `pkg/vulpinemark/` — library
  - `cdp.go` — minimal gorilla/websocket CDP client, `/json/list` page-target auto-discovery
  - `enumerate.go` — `Runtime.evaluate` JS that finds visible interactive elements + accessible names
  - `screenshot.go` — `Page.captureScreenshot` + `Page.getLayoutMetrics`
  - `annotate.go` — Go `image/draw` + `x/image/font/basicfont` for borders + numbered badges
  - `mark.go` — top-level `Mark.Annotate()` API
  - `actions.go` — click/type/hover helpers driven by labels
  - `cluster.go` — cluster mode: group repeated items under `@N[K]` labels
  - `diff.go` — diff mode: annotate only what changed between two snapshots
  - `palette.go` — palette packs (default / high-contrast / monochrome / colorblind)
  - `svg.go` — SVG overlay output (`AnnotateSVG`)
  - `stable_labels.go` — stable semantic-hash label IDs
- `cmd/vulpine-mark/` — CLI binary

## Build / test

```bash
go build ./...
go vet ./...
go test ./...
```

## Hard rules

- Push only to `PopcornDev1/vulpine-mark`. Never touch `CloverLabsAI` or any other org.
- One-line commit messages, no co-authors, push after every cohesive change.
- This repo is **public** — no proprietary VulpineOS internals here. Don't reference any private VulpineOS internals or private patches in code or docs.
- The native Juggler implementation lives in VulpineOS itself (private). This repo is the standalone CDP version only.
- Autonomous /loop mode: never ask permission, act and document in commits.

## Roadmap (MVP done; what's next)

- [ ] Real-page integration test (Chrome headless via testcontainers or similar)
- [ ] Unit tests for `enumerate` JS (snapshot test against fixture HTML)
- [x] Full-page mode (scroll + stitch screenshots, label off-viewport elements)
- [ ] DPR scaling fix for Retina screenshots (currently uses visualViewport.scale; verify on macOS)
- [x] Element visibility: occlusion check (elementFromPoint at center)
- [x] Click-by-label helper: `mark.Click(ctx, "@3")` dispatches mouse event at element center
- [x] Type-by-label helper: `mark.Type(ctx, "@5", "hello")`
- [x] Scroll-into-view before action (reuses viewport metrics)
- [x] Context-aware action helpers (all methods take `ctx context.Context`)
- [x] Cluster mode: group repeated items under `@N[K]` labels
- [x] Diff mode: annotate only what changed between two snapshots
- [x] Per-label confidence score + low-confidence fade
- [x] Output formats: SVG overlay (`AnnotateSVG`, `--svg`); JSON-only mode, base64 stdout still TODO
- [x] Palette packs: default / high-contrast / monochrome / colorblind (`SetPalette`, `--palette`)
- [x] Stable semantic-hash labels (`UseStableLabels`)
- [x] CLI: `--max-elements`, `--clustered`, `--diff`, `--save-result` (selectors still TODO)
- [ ] Doc: example annotated PNG in README
- [ ] GitHub Actions CI (build, vet, test on linux/macos)
