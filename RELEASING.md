# Releasing vulpine-mark

This is the v0.1 release checklist for the public Set-of-Mark library.

## Pre-release checks

1. Confirm `main` is clean:

   ```bash
   git status --short --branch
   ```

2. Run the full verification set:

   ```bash
   go build ./...
   go vet ./...
   go test ./...
   go test ./... -race
   ```

3. Review these docs for drift:

   - [README.md](README.md)
   - [CHANGELOG.md](CHANGELOG.md)
   - [docs/visual-extraction-api-adapter.md](docs/visual-extraction-api-adapter.md)

4. Confirm no untracked workflow or local junk is staged:

   - `.github/`
   - screenshots or local scratch outputs
   - private notes

## Tagging

Create the release tag from `main`:

```bash
git tag v0.1.0
git push origin v0.1.0
```

For patch releases:

```bash
git tag v0.1.1
git push origin v0.1.1
```

## Release note checklist

Include:

- package + CLI positioning
- supported output modes
- supported browsers via CDP
- explicit note that hosted API behavior belongs in `vulpine-api`, not
  in this library

## Post-tag sanity check

Verify:

- the tag resolves to the expected GitHub commit
- `go install github.com/VulpineOS/vulpine-mark/cmd/vulpine-mark@vX.Y.Z` works
- README examples still match the current CLI flags and library surface
