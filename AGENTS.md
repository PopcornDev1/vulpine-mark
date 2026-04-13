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

---

## For AI coding agents (Codex, Claude Code, etc.)

This section captures cross-session preferences. Treat them as binding unless the current session's user message explicitly overrides.

### User profile

- Senior C++ browser engine developer (Firefox internals, XPCOM, accessibility tree, DOM)
- Dev machine: MacBook M1 Pro with artifact builds enabled
- GitHub: `PopcornDev1`

### GitHub rules

- **Only push to repos on `PopcornDev1/`.** Never push to any organization. Specifically: never create, fork, or commit to `CloverLabsAI` — that is the user's employer and unauthorized changes cause real problems.
- Approved public repos you may interact with: `VulpineOS`, `vulpine-mark`, `foxbridge`, `vulpineos-docs`, `mobilebridge`.
- Before pushing, verify visibility: `gh repo view PopcornDev1/<name> --json visibility`.
- This repo is **public**. Do not reference proprietary sibling modules or private implementation details in any tracked file.

### Commit rules

- One-line commit messages. No multi-line descriptions. No `Co-Authored-By` trailers. No "Generated with Claude Code" footers.
- Commit and push after every cohesive change — frequent commits are a safety net.
- Use `git add <specific files>`; avoid `git add -A` so untracked junk doesn't leak into commits.
- **Never add `.github/workflows/*.yml` via commit** — the OAuth token in the user's keychain lacks `workflow` scope and pushes will be rejected. Leave workflow files untracked for the user to push manually.

### Workflow rules

- In normal interactive mode, **never commit, push, or create PRs without explicit user approval.** Stage and show diffs, then wait.
- In autonomous `/loop` overnight mode, act without asking and document in commit messages.
- Keep this file and the README accurate as work progresses. Drift between docs and code is treated as a bug.

### Testing rules

- After every change: `go build ./...`, `go vet ./...`, `go test ./... -race`. Fix failures before moving on.
- Don't claim a feature is done without running the tests.
- Tests must be hermetic: no live browsers, no live network. Use fake CDP transports and fixture responses.

### Ecosystem context

VulpineOS is the umbrella project — a Camoufox (Firefox 146) fork that runs fleets of AI browser agents. This repo is one of several public components that compose together. Private sibling modules exist and MUST NOT be referenced from this repo in code, commits, docs, or commit messages.
