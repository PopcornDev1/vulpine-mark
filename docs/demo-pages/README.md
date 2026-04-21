# Vulpine Mark Demo Pages

Deterministic static pages for validating `vulpine-mark` output.

## Serve locally

```bash
python3 -m http.server 8123 --directory docs/demo-pages
```

Then point a CDP-capable browser at `http://127.0.0.1:8123/index.html` and run the CLI against the active tab.

## Suggested checks

### Label stability

```bash
vulpine-mark --cdp http://localhost:9222 --output stability.png --json stability.json
vulpine-mark --cdp http://localhost:9222 --output stability-stable.png --json stability-stable.json
```

Use `stability.html` and compare repeated captures with stable-label mode enabled in the library or any future CLI flag that exposes it.

### DPR handling

Use `dpr.html` on a retina display or with browser zoom changes. Verify badges stay anchored to the compact controls.

### Full-page capture

```bash
vulpine-mark --cdp http://localhost:9222 --full-page --output full-page.png --json full-page.json
```

Use `full-page.html` and confirm controls in sections 3-5 are labeled in the stitched output.

### Cluster mode

```bash
vulpine-mark --cdp http://localhost:9222 --clustered --output catalog.png --json catalog.json
```

Use `clustered-catalog.html` and verify repeated cards collapse into cluster labels instead of one badge per card.

### Diff mode

Take a baseline on `diff-modal.html`, click `Launch approval modal`, save the first result with `--save-result`, then run:

```bash
vulpine-mark --cdp http://localhost:9222 --diff baseline.json --output diff.png --json diff.json
```

Only the modal and moved summary controls should be highlighted.
