## Why

The documented public Provider routes for retrieval and server-issued turns are
currently rejected before their handlers run. Adding them to the route gate by
itself would expose the internal retrieval response and unprojected turn
evidence, so the public contract needs an equivalent disclosure boundary.

## What Changes

- Admit `POST /v1/provider/retrieve` and `POST /v1/provider/turns` for public
  runtime bindings only after current exact-scope disclosure authorization.
- Add closed retrieval request selection for disclosure profile and response
  mode, defaulting to reference-only output.
- Return typed, lifecycle-safe Provider retrieval projections without scores,
  diagnostics, hidden candidates, or content unless an authorized content
  profile explicitly requests bounded content.
- Persist and return turn context evidence through the negotiated disclosure
  mode so reference-only turns never retain text or diagnostics.
- Publish the routes' authorization, projection, failure, and compatibility
  behavior in OpenAPI and Provider documentation.

## Non-goals

- Add business-role authorization, token budgets, historical context replay,
  external-agent execution, or a client-side fallback route.
- Change native or administrative memory-session behavior.

## Capabilities

### New Capabilities

<!-- None. -->

### Modified Capabilities

- `agent-runtime-provider-adapter`: Public Provider retrieval and turn routes
  must apply current disclosure and return safe projections.
- `context-disclosure-policy`: The alternate-route boundary gains equivalent
  reference/content disclosure enforcement for retrieval and turns.

## Impact

- `internal/app` Provider route gate and HTTP handlers.
- `internal/provider` public projection DTOs and `internal/memory` turn
  evidence persistence.
- Provider OpenAPI and integration documentation.
- Related handler, disclosure-boundary, and memory-session tests.

Artifact references: `openspec instructions apply --change provider-disclosure-route-gate --json`.
