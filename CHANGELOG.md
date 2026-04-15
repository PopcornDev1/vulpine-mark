# Changelog

## v0.1.0

Initial public release of `vulpine-mark`.

Highlights:

- standalone Go package and CLI
- CDP-native interactive element enumeration
- annotated PNG output with `@N` labels
- structured JSON element maps
- click, type, and hover by label
- full-page capture support
- clustered labeling for repeated layouts
- diff mode for changed or moved elements
- stable semantic labels across re-renders
- SVG overlay and heatmap output modes
- role-based filter callbacks
- hermetic unit coverage plus integration coverage

Scope notes:

- browser-facing library only
- no hosted API, billing, or tenant logic in the package
- works with CDP-capable browsers such as Chrome and Camoufox/foxbridge
