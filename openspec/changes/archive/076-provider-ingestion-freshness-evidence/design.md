## Context

The existing service has durable raw-event timestamps, governance completion
state, canonical lifecycle records, exact-scope retrieval and bounded assurance
reports. It also has aggregate backlog and maintenance freshness diagnostics.
The missing evidence is a single redacted, deterministic view that relates
those existing states for a bounded fixture without making public Provider
responses operationally verbose. See `proposal.md` for the motivation and
`specs/ingestion-freshness-evidence/spec.md` for the contract.

## Goals / Non-Goals

**Goals:**

- Reuse PostgreSQL as the sole source of truth and preserve exact scope and
  lifecycle gates at every stage.
- Make stage transitions and latency/SLO classification testable with pure
  deterministic logic and a fresh PostgreSQL assurance path.
- Attribute public retrievability to an actual authorized read observation.
- Keep reports and metrics safe for administrator inspection and CI evidence.

**Non-Goals:**

- No new queue, scheduler, public Provider context field, ranking strategy or
  canonical-memory mutation.
- No historical context snapshot, token estimate, Agent execution or task
  outcome judgment.

## Decisions

1. **Use a read-only stage evaluator over existing records.** Raw event creation
   and governance timestamps remain authoritative. Canonical and public-read
   stages are observed through existing lifecycle/disclosure-aware services or
   assurance fixtures. This avoids adding duplicated timestamps that could
   diverge from the source of truth.

2. **Represent stage outcomes as bounded categories.** Each stage has a closed
   disposition such as `complete`, `pending`, `retrying`, `exhausted`,
   `ineligible`, `hidden`, `unavailable` or `invalid`. Latencies use fixed
   buckets rather than arbitrary durations or identifiers in telemetry.

3. **Keep correlation opaque outside the evaluator.** The internal evaluator may
   use durable IDs after exact-scope authorization, but serialized evidence uses
   a report identity/hash and bounded counts. This supports reproducibility while
   avoiding existence leakage through admin or public surfaces.

4. **Add assurance before activation.** Unit tests validate ordering, buckets,
   replay and redaction. The live verifier creates fresh governed data, waits
   for durable completion, performs the authorized public read, and records only
   bounded initial/restart outcomes. No freshness result enables a ranking or
   disclosure policy.

5. **Reuse existing retention and telemetry.** Evidence follows the existing
   assurance/maintenance retention path and low-cardinality event conventions.
   No independent evidence database or cache is introduced.

Alternatives considered: storing per-stage timestamps on every event (rejected
because derived observations would become canonical schema state); exposing
raw IDs to operators (rejected because it creates an unnecessary disclosure
surface); and inferring public retrievability from SQL joins (rejected because
the contract requires an authorized public read observation).

## Risks / Trade-offs

- **[Risk]** A public read can race a worker or lifecycle transition. **Mitigation:**
  record observation time and disposition, recheck current authorization and
  lifecycle, and classify the result as unavailable rather than retrying without
  bounds.
- **[Risk]** Fixed buckets hide exact latency needed for debugging. **Mitigation:**
  keep exact timestamps only in privileged bounded internal evidence where the
  existing retention policy permits it; serialized reports and metrics remain
  bucketed/redacted.
- **[Risk]** Reusing current records cannot prove a stage that was never persisted.
  **Mitigation:** missing durable evidence is an explicit nonpassing category,
  never a synthesized success.

## Migration Plan

The change is additive and requires no destructive migration. Deploy the
evaluator and assurance contract with reporting disabled for ordinary Provider
traffic, run focused and fresh-stack evidence, then enable only the admin/
assurance inspection path. Rollback disables the new evidence path while
retaining existing event, governance and canonical records.

## Open Questions

None. The stage vocabulary, redaction boundary, replay identity and activation
boundary are fixed by the proposal and capability spec.
