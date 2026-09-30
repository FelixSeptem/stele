## Why

Stele has governed derived insights and an agent-runtime memory-provider
contract, but it does not yet define how optional model-assisted reasoning may
enter the service. Without a provider-independent boundary, future hypothesis,
goal, contradiction, or causal-link work could couple model credentials and
vendor behavior to governance, bypass replay, or make unbounded external calls.
The boundary is needed now so later reasoning features can remain optional,
offline-testable, scope-safe, and reversible.

## What Changes

- Define a provider-independent reasoning capability and invocation contract for
  bounded, optional derivation requests.
- Define versioned capability/limit discovery, input and output envelopes,
  budgets, deadlines, cancellation, retryability, and stable error categories.
- Require reasoning outputs to be candidates or governed intents with scope,
  provenance, evidence, policy/version, and replay metadata; prohibit direct
  canonical-memory writes and direct lifecycle activation.
- Define offline deterministic replay and shadow execution semantics, including
  result equivalence, redaction, stale/degraded dependencies, fallback, and
  explicit disabled behavior.
- Define provider conformance evidence and operator diagnostics without exposing
  prompts, credentials, raw model payloads, hidden records, or internal chain of
  thought.
- Keep concrete LLM adapters, model selection, prompt templates, and automatic
  activation of reserved insight types outside this change.
- Update the v1 roadmap immediate-next-step bookkeeping from archived P8.2b to
  this P8.3 boundary proposal.

## Capabilities

### New Capabilities

- `reasoning-provider-boundary`: provider-independent contract for optional,
  bounded, replayable, and governance-safe reasoning derivation.

### Modified Capabilities

None. The new boundary composes the existing governed-insight and agent-runtime
provider contracts without changing their requirements.

## Impact

- New OpenSpec contract and design documentation for reasoning providers.
- Changes to governed derivation interfaces, provider registration/configuration,
  offline replay, shadow diagnostics, and conformance evidence during
  implementation.
- No database migration, new memory class, public end-user SDK, UI, or default
  model invocation in this proposal.
- No required external provider dependency; self-hosted deployments remain
  functional with reasoning disabled.

## Non-goals

- Implementing an OpenAI, Anthropic, local-model, or other vendor-specific
  reasoning adapter.
- Defining prompts, model routing, chain-of-thought capture, or final-answer
  generation.
- Automatically inferring or activating `hypothesis`, `goal`, `contradiction`,
  or `causal_link` insights.
- Allowing provider output to widen scope, grant access, mutate canonical memory
  directly, bypass governance, or override server capabilities and limits.
- Making reasoning a prerequisite for API, worker, scheduler, retrieval, or
  context assembly startup.

## References

- Roadmap: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md` (P8.3 and immediate
  next step).
- Existing insight governance: `openspec/specs/governed-experience-insights/spec.md`.
- Existing memory-provider boundary: `openspec/specs/agent-runtime-provider-adapter/spec.md`.
- Related provider runtime patterns:
  `openspec/specs/embedding-provider-runtime/spec.md` and
  `openspec/specs/reranker-provider-runtime/spec.md`.
- Workflow commands: `openspec status --change reasoning-provider-boundary`,
  `openspec validate --all --strict`, and
  `openspec instructions apply --change reasoning-provider-boundary --json`.
