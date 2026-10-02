# Governed Autonomous Reasoning Insights

Stele treats provider-backed reasoning as a derived, non-authoritative
candidate flow. A provider may propose `hypothesis`, `goal`, `contradiction`,
or `causal_link`, but it cannot activate an insight or mutate canonical memory.

## Execution Modes

- `offline` validates a normalized candidate envelope and produces a stable
  replay identity without invoking a remote provider.
- `shadow` may invoke a bounded provider and record a `would_activate` result,
  but it does not change active derived insights, retrieval, or context output.
- Activation remains a separate exact-scope policy decision through the reserved
  insight activation service.

## Candidate Contract

Every candidate carries exact scope proof, active-only lifecycle visibility,
reference-only redaction policy, source watermark, evidence digest, provider and
schema versions, policy version, uncertainty, and replay identity. Evidence is
validated as an authorized subset before a candidate can be handed to policy
admission. Missing, stale, hidden, foreign, malformed, or over-budget inputs
fail closed and are represented by bounded dispositions.

The candidate envelope is persisted in PostgreSQL for audit and replay. The
record is append-only and contains no prompt, chain-of-thought, raw provider
payload, credential, or default context content.

## Verification

Focused tests cover evidence digest ordering, exact scope, unsafe mutation
rejection, deterministic replay, shadow non-authority, activation handoff,
candidate persistence, migration manifest continuity, and low-cardinality
reasoning telemetry. Any policy that is not separately enabled remains
shadow-only.
