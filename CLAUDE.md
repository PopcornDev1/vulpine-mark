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
- [ ] Full-page mode (scroll + stitch screenshots, label off-viewport elements)
- [ ] DPR scaling fix for Retina screenshots (currently uses visualViewport.scale; verify on macOS)
- [ ] Element visibility: occlusion check (elementFromPoint at center)
- [ ] Click-by-label helper: `mark.Click("@3", cdpClient)` dispatches mouse event at element center
- [ ] Type-by-label helper: `mark.Type("@5", "hello")`
- [ ] Output formats: SVG overlay, JSON-only mode, base64 stdout
- [ ] CLI: `--full-page`, `--include`/`--exclude` selectors, `--max-elements`
- [ ] Doc: example annotated PNG in README
- [ ] GitHub Actions CI (build, vet, test on linux/macos)
