# Visual Extraction API Adapter

This document defines how `vulpine-mark` powers the paid visual
extraction endpoint without moving commercial API behavior into the
public library.

## Scope boundary

`vulpine-mark` stays responsible for:

- connecting to a CDP endpoint
- enumerating visible interactive elements
- producing annotated images and structured element maps
- exposing labeling modes such as full-page, clustered, stable-label,
  heatmap, SVG, and JSON-only output

The paid API layer stays responsible for:

- tenant auth, billing, and quotas
- browser pool lifecycle
- navigation policy and retries
- request validation and endpoint contracts
- storage, async execution, and webhook delivery
- premium feature gating

Rule:

- `vulpine-mark` remains a browser-facing library
- `vulpine-api` remains the product-facing service

## Adapter shape

The commercial adapter should be a thin wrapper around a browser session
that has already been prepared by the host service.

Current shape in `vulpine-api`:

1. acquire a browser context from the service-owned pool
2. navigate to the target URL
3. wait for the page to settle per service policy
4. enable action-lock if available
5. resolve the page websocket URL
6. construct `vulpinemark.New(ctx, pageWSURL)`
7. apply library options:
   - `SetMaxElements`
   - `UseStableLabels`
8. call one of:
   - `Annotate`
   - `AnnotateFullPage`
   - `AnnotateClustered`
9. map the library result into the paid API response

That keeps all product logic outside the public library while preserving
one direct integration point: a live page websocket URL.

## Minimal public contract

The public library does not need API-specific request types. The stable
integration contract is:

```go
mark, err := vulpinemark.New(ctx, pageWSURL)
if err != nil {
	return err
}
defer mark.Close()

mark.SetMaxElements(maxElements)
mark.UseStableLabels(stableLabels)

result, err := mark.Annotate(ctx)
```

The host service may select a different annotate mode, but it should not
need any API-only hooks inside `vulpine-mark`.

## Response mapping

The adapter may expose a product-specific response shape, but it should
map directly from `vulpinemark.Result`:

- `Image` -> base64 image field or binary attachment
- `Labels` -> ordered label list
- `Elements` -> structured label map
- `Clusters` -> optional cluster data

Service-owned metadata such as:

- URL
- billing IDs
- task IDs
- tenant IDs
- audit timestamps

must stay outside the library result type.

## Recommended invariants

The commercial adapter should preserve these invariants:

1. no paid-only request or billing types inside `vulpine-mark`
2. no service-owned browser pool types inside `vulpine-mark`
3. no webhook, task, or tenant concepts inside `vulpine-mark`
4. no hidden navigation inside the library after construction
5. library output must remain reusable by non-Vulpine consumers

## Feature mapping

Current endpoint-safe mappings:

| API option | Library call / option |
| --- | --- |
| `full_page` | `AnnotateFullPage` |
| `clustered` | `AnnotateClustered` |
| `stable_labels` | `UseStableLabels(true)` |
| `max_elements` | `SetMaxElements(n)` |

Future premium behavior such as SVG overlay export, JSON-only output, or
heatmaps should be added by composing existing library calls rather than
adding API-specific branches to the library surface.

## Error handling split

Library errors:

- CDP connect failures
- page screenshot failures
- DOM enumeration failures
- invalid label/action usage

Service errors:

- quota exceeded
- auth failures
- browser-pool exhaustion
- timeout and retry policy
- endpoint validation
- persistence / async job state

The adapter should wrap library errors with service context, but not
flatten everything into library-specific types.

## Testing split

`vulpine-mark` tests should cover:

- DOM enumeration
- labeling correctness
- image / JSON / SVG output
- CDP interaction behavior

`vulpine-api` tests should cover:

- browser-pool integration
- action-lock behavior
- endpoint request/response contract
- quota and premium gating
- async and webhook paths

That keeps the public library independently testable and reusable.

## Non-goals

These do not belong in `vulpine-mark`:

- paid endpoint handlers
- tenant-aware request types
- hosted browser orchestration
- API key handling
- monitor or webhook workflows

## Compatibility note

This boundary allows the public library to remain MIT and generally
useful, while the paid API can evolve its own browser management and
commercial workflow logic without forcing product concerns into the
library package.
