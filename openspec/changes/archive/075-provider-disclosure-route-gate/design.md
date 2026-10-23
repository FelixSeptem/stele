## Context

See `proposal.md` for the observed route-gate failure. The existing binding
middleware already validates principal, binding, session, scope, and expiry.
The current disclosure resolver is available in `internal/app`, and retrieval
can mark the search as disclosure-required. Context assembly has a distinct
budgeted projector; this change does not reuse raw `SearchResult` as a public
response.

## Goals / Non-Goals

**Goals:** expose the two documented routes while preserving current authority,
exact binding, lifecycle/source eligibility, bounded citations, and closed
public response shapes.

**Non-Goals:** public sync, native read routes, historical snapshot replay,
business roles, token allocation, or changes to administrative/native session
semantics.

## Decisions

- Resolve `currentDisclosure` in both handlers after runtime binding middleware
  and before protected operations. `DisclosureEntitlements.Resolve` remains the
  only profile/mode decision point.
- Keep Provider retrieve input closed and separate from the native search
  schema. Set the retrieval scope/session from the binding and enable
  `DisclosureRequired`; project only active same-scope low/medium versioned
  matches and create citations from those matches rather than trusting attached
  internal citation lists.
- Use a dedicated turn input DTO without caller-owned scope, session, turn ID,
  role, or diagnostic controls. Pass only the resolved disclosure profile/mode
  to the memory service. Project evidence before storage and again at the HTTP
  response boundary; this ensures both persistence and response honor the mode.
- Keep `reference_only` as the default even when the grant's default profile is
  content-capable. Explicit bounded content is the only path that returns
  retrieved content or turn summary.

Alternatives considered: adding only the two paths to the allowlist was rejected
because retrieve currently serializes scores/content/diagnostics and turns
persist evidence before HTTP response shaping. Reusing the native search DTO was
rejected because disclosure options would silently become accepted on unrelated
routes.

## Risks / Trade-offs

- Existing public consumers could depend on the raw retrieval shape. The route
  was publicly documented but blocked by the gate; the revised schema is
  explicit and consumers must follow OpenAPI rather than rely on internal fields.
- Turn evidence summaries are intentionally dropped in reference mode and
  diagnostics in both Provider modes. Native/admin calls without a resolved
  Provider response mode retain their existing behavior.
- The response projection omits ranking and hidden-candidate detail by design;
  debugging remains an authorized administrative responsibility.

## Migration Plan

No database migration is required. Deploy the handler/schema and documentation
together. Rollback removes the public route-gate admission while retaining
existing grants and stored turns; no disclosure grant change is needed.
