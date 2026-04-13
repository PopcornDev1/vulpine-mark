# Vulpine Mark — Visual Element Labeling

Standalone Go library + CLI that annotates browser screenshots with numbered badges (`@1`, `@2`...) over interactive elements via CDP. Public open-source repo, drives adoption of the wider VulpineOS ecosystem.

**Repo:** `PopcornDev1/vulpine-mark` (public, MIT)

## Layout

- `pkg/vulpinemark/` — library
  - `cdp.go` — minimal gorilla/websocket CDP client, `/json/list` page-target auto-discovery
  - `enumerate.go` — `Runtime.evaluate` JS that finds visible interactive elements + accessible names
  - `screenshot.go` — `Page.captureScreenshot` + `Page.getLayoutMetrics` + DPR probe
  - `annotate.go` — Go `image/draw` + `x/image/font/basicfont` for borders + numbered badges
  - `mark.go` — top-level `Mark.Annotate()` API
  - `actions.go` — click/type/hover helpers driven by labels
  - `cluster.go` — cluster mode: group repeated items under `@N[K]` labels
  - `diff.go` — diff mode: annotate only what changed between two snapshots
  - `palette.go` — palette packs (default / high-contrast / monochrome / colorblind)
  - `svg.go` — SVG overlay output (`AnnotateSVG`)
  - `stable_labels.go` — stable semantic-hash label IDs
  - `heatmap.go` — heatmap mode (`AnnotateHeatmap`, translucent importance fills)
  - `json_output.go` — JSON-only mode (`AnnotateJSON`, no screenshot capture)
  - `filter.go` — custom `ElementFilter` callbacks + `IncludeRoles`/`ExcludeRoles`
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

## Coordination and local tooling

- Linear is the shared execution tracker for the VulpineOS ecosystem. Use the `VulpineOS` workspace, product/type/source labels, and link commits in issue comments when closing work.
- Codex has a persistent local Playwright MCP at `http://localhost:8931/mcp` for browser navigation, snapshots, console/network inspection, and screenshots. It writes artifacts to `~/.codex/mcp-output/playwright` and omits inline image payloads to reduce token usage.
- For visual/browser verification, prefer saved snapshots and screenshot filenames over pasting large page dumps or image data into chat.

## Roadmap (MVP done; what's next)

- [x] Real-page integration test (fake CDP transport, gated on `-tags integration`)
- [x] Unit tests for `enumerate` JS (selector snapshot + fixture-response decode)
- [x] Full-page mode (scroll + stitch screenshots, label off-viewport elements)
- [x] DPR scaling fix for Retina screenshots (`viewportSize` now returns `visualViewport.scale * devicePixelRatio`)
- [x] Element visibility: occlusion check (elementFromPoint at center)
- [x] Click-by-label helper: `mark.Click(ctx, "@3")` dispatches mouse event at element center
- [x] Type-by-label helper: `mark.Type(ctx, "@5", "hello")`
- [x] Scroll-into-view before action (reuses viewport metrics)
- [x] Context-aware action helpers (all methods take `ctx context.Context`)
- [x] Cluster mode: group repeated items under `@N[K]` labels
- [x] Diff mode: annotate only what changed between two snapshots
- [x] Per-label confidence score + low-confidence fade
- [x] Output formats: SVG overlay (`AnnotateSVG`, `--svg`) + JSON-only mode (`AnnotateJSON`, `--json-only`); base64 stdout still TODO
- [x] Palette packs: default / high-contrast / monochrome / colorblind (`SetPalette`, `--palette`)
- [x] Stable semantic-hash labels (`UseStableLabels`)
- [x] Heatmap mode: translucent importance-weighted fills (`AnnotateHeatmap`, `--heatmap`)
- [x] Custom element filter callbacks (`SetElementFilter`, `--include-role`, `--exclude-role`)
- [x] Real-page flow test (always-on, covers annotate + click + type + hover dispatch)
- [x] CLI: `--max-elements`, `--clustered`, `--diff`, `--save-result` (selectors still TODO)
- [ ] Doc: example annotated PNG in README
- [ ] GitHub Actions CI (build, vet, test on linux/macos)
