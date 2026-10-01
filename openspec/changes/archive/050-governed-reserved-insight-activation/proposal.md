## Why

Stele now has a provider-independent reasoning boundary and an optional
OpenAI-compatible adapter, but `hypothesis`, `goal`, `contradiction`, and
`causal_link` remain permanently non-active vocabulary. That boundary is safe,
but it leaves no governed path for turning a validated, evidence-backed
candidate into a derived insight when an operator explicitly wants one. This
change adds that path without making reasoning mandatory, authoritative, or
able to mutate canonical memory directly.

## What Changes

- Add a versioned, scope-bound activation policy for reserved insight types,
  with disabled-by-default behavior and explicit owner, expiry, and rollback
  metadata.
- Add a candidate-to-insight review/admission flow that accepts only validated
  reasoning candidates or governed intents with exact scope, eligible evidence,
  provenance, confidence/uncertainty, and policy/provider versions.
- Keep activation append-only and lifecycle-governed: an accepted candidate
  creates a derived insight version and audit record; it never overwrites
  canonical memory or silently changes a prior insight in place.
- Define per-type eligibility and evidence rules. The first rollout is limited
  to reviewed `hypothesis` activation; `goal`, `contradiction`, and
  `causal_link` remain disabled unless their own policy and conformance rules
  are satisfied.
- Add deterministic offline replay and shadow evaluation for activation
  decisions, including stale-policy/source-watermark handling and baseline
  non-authoritative behavior.
- Add bounded diagnostics and operator inspection for policy decisions,
  candidate disposition, activation, suppression, expiry, and rollback without
  exposing prompts, chain-of-thought, credentials, raw provider payloads, or
  foreign identifiers.
- Add explicit stop/rollback controls and conformance evidence for exact-scope
  isolation, lifecycle safety, evidence completeness, idempotency, and
  provider-failure fallback.
- Update the v1 roadmap to replace the stale P8.4 immediate-next entry with
  this P8.5 proposal while preserving the separation between adapter execution
  and insight activation.

## Non-goals

- Making any reasoning provider required for `api`, `worker`, `scheduler`,
  retrieval, or context assembly startup.
- Allowing a provider, adapter, or model output to write canonical memory,
  grant access, widen scope, directly set lifecycle state, or bypass existing
  governance.
- Enabling all reserved insight types in one rollout. `goal`,
  `contradiction`, and `causal_link` remain disabled until separately
  evidenced by their type-specific policy.
- Persisting prompts, chain-of-thought, raw model responses, credentials, or
  hidden/cross-scope identifiers.
- Adding a graph database, second system of record, SDK, UI, or vendor-specific
  reasoning runtime.
- Changing default retrieval or context assembly to trust an activated insight
  without a separate, governed retrieval/context policy.

## Capabilities

### New Capabilities

- `governed-reserved-insight-activation`: versioned activation policy,
  candidate admission, per-type evidence rules, replay/shadow decisions,
  lifecycle/audit transitions, rollback, and bounded operator diagnostics for
  reserved reasoning insights.

### Modified Capabilities

- `reasoning-provider-boundary`: allow a validated candidate to enter a
  separately governed activation policy while retaining the prohibition on
  direct provider activation or canonical mutation.
- `governed-experience-insights`: replace the permanently non-active reserved
  vocabulary rule with policy-gated, evidence-backed activation semantics and
  preserve append-only lifecycle, replay, feedback, and audit requirements.
- `derived-insight-replay`: include activation-policy compatibility,
  candidate disposition, and stale-policy behavior in deterministic replay
  outcomes.

## Impact

- Affected areas include `internal/reasoning`, governed insight derivation and
  lifecycle handling, policy/config loading, admin inspection/diagnostics,
  replay and conformance tests, OpenAPI schemas where policy/diagnostic output
  is exposed, and the self-hosting/roadmap documentation.
- PostgreSQL remains the only system of record; any policy, candidate
  disposition, activation, rollback, and audit evidence must be rebuildable
  and scope-bound.
- No new external provider SDK or runtime dependency is required. The existing
  provider boundary and OpenAI-compatible adapter remain optional and
  failure-isolated.
- Related workflow references: `openspec status`, `openspec validate --all
  --strict`, and the existing reasoning conformance/replay commands should be
  used before implementation and archive.
