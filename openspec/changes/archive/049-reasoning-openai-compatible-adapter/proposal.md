## Why

The archived reasoning provider boundary defines safe envelopes, candidate-only
outputs, offline replay, and shadow semantics, but it deliberately has no
concrete provider. Self-hosted operators therefore cannot yet exercise the
boundary against an operator-controlled model endpoint. A narrowly scoped
OpenAI-compatible adapter is the next step because it proves the boundary can
make one bounded remote call without turning model credentials, prompt content,
or model availability into a core runtime dependency.

## What Changes

- Add an optional OpenAI-compatible reasoning adapter for a configured chat
  completion-style endpoint, model identity, bounded timeout, and credential.
- Define a strict, versioned structured-output exchange that maps only a
  normalized reasoning envelope to a candidate-only result; redact and discard
  provider request/response bodies after validation.
- Register the adapter consistently in `api`, `worker`, and `scheduler`, with
  disabled-by-default configuration and fail-closed validation for incomplete
  enabled configurations.
- Permit remote execution only in explicitly enabled `shadow` or `live` mode;
  offline replay stays remote-free, and no adapter request may change canonical
  memory, default retrieval, default context, lifecycle state, grants, or
  configuration.
- Add bounded health/capability diagnostics, categorized errors, deterministic
  local HTTP conformance fixtures, and self-hosting operator guidance.
- Update the v1 roadmap to mark P8.3 archived as change 048 and this P8.4
  adapter proposal as the immediate next step.

## Capabilities

### New Capabilities

- `reasoning-openai-compatible-adapter`: optional, secret-safe, bounded
  OpenAI-compatible HTTP execution for the existing reasoning provider boundary.

### Modified Capabilities

None. The adapter composes the existing `reasoning-provider-boundary` contract
without relaxing its scope, candidate, replay, or fallback requirements.

## Impact

- Affected code: `internal/reasoning`, `internal/config`, and the shared
  `internal/app` runtime registration path.
- Affected contracts: an operator-only reasoning capability/diagnostic surface
  may gain provider readiness details; any public contract remains OpenAPI-first
  and must not expose credentials, prompts, raw payloads, hidden IDs, or scope
  data beyond the caller's exact authorization.
- Affected documentation: self-hosting configuration and the v1 roadmap.
- No database migration, SDK, UI, new memory class, prompt authoring surface,
  model-routing policy engine, or automatic inference/activation of reserved
  insight types.

## Non-goals

- Supporting multiple vendor APIs, provider failover, provider-managed tools,
  streaming, final-answer generation, or arbitrary operator-supplied prompts.
- Persisting chain-of-thought, raw model/provider payloads, authorization
  headers, or credentials.
- Scheduling autonomous derivation or making a reasoning provider required for
  API, worker, scheduler, retrieval, or context startup.
- Activating `hypothesis`, `goal`, `contradiction`, or `causal_link` insights,
  or allowing any direct canonical-memory mutation.

## References

- Existing contract: `openspec/specs/reasoning-provider-boundary/spec.md`.
- Related HTTP patterns: `openspec/specs/embedding-provider-runtime/spec.md`
  and `openspec/specs/reranker-provider-runtime/spec.md`.
- Runtime implementation: `internal/embedding/openai_provider.go`.
- Workflow: `openspec status --change reasoning-openai-compatible-adapter`,
  `openspec instructions apply --change reasoning-openai-compatible-adapter
  --json`, and `openspec validate --all --strict`.
