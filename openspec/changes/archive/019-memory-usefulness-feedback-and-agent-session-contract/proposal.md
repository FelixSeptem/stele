## Why

Stele can now prove that a scope and a service-side memory session are operationally usable, but it still cannot learn whether recalled memories, citations, and derived insights were useful to an external agent's actual turn. Without durable usefulness feedback and stronger session attribution, the product loop stops at technical recall verification instead of closing the quality loop from memory use back into diagnostics, repair recommendations, and later verification.

## What Changes

- Add scoped usefulness feedback records for memory hits, raw events, context citations, derived insights, sessions, session turns, verifications, and expected-recall misses.
- Represent expected-recall miss feedback with bounded typed targets, including known evidence targets and opaque caller-provided tokens.
- Strengthen memory session turns with idempotent turn/outcome contracts, bounded outcome event payload ingestion through the existing event path, verification history preservation, and feedback summaries.
- Add durable usefulness summaries that aggregate bounded feedback categories without rewriting canonical memory, insight evidence, or provenance in place.
- Add append-only feedback correction/supersession so bad or stale feedback can be retired without deleting audit history.
- Expose feedback-aware diagnostics for retrieval and context assembly, including noisy, stale, missing expected recall, and hidden-memory safety signals.
- Bridge repeated negative feedback and expected-recall misses into scoped quality findings and approval-gated repair recommendations.
- Add admin inspection, low-cardinality metrics, OpenAPI documentation, and self-hosting runbook updates for the feedback/session verification loop.
- Keep feedback-aware ranking conservative: first-class diagnostics and per-request opt-in ranking hints are allowed, but default retrieval behavior and scope-level policy must not change in this proposal.

## Non-goals

- Do not add SDKs, UI, chat interfaces, model invocation, prompt orchestration, answer generation, or end-user agent runtime logic.
- Do not let caller feedback directly rewrite canonical memory content, memory versions, vector revisions, derived insight evidence, or lifecycle state.
- Do not auto-suppress memory, auto-approve repair plans, or execute repair actions inline with public session or feedback requests.
- Do not aggregate usefulness feedback across unauthorized tenant, project, or namespace boundaries.
- Do not expose cross-subject usefulness summaries through public routes except within the caller's own authorized session report.
- Do not expose high-cardinality feedback, memory, event, session, turn, proof, actor, or reason identifiers as metric labels.
- Do not make feedback-aware reranking the default behavior or add a scope-wide ranking policy in this change; default behavior can only surface diagnostics unless explicitly requested per request.

## Capabilities

### New Capabilities

- `memory-usefulness-feedback`: Durable scoped feedback, feedback summaries, feedback attribution, and feedback-driven diagnostics for memory hits, raw events, context citations, derived insights, sessions, session turns, verifications, and expected-recall misses.

### Modified Capabilities

- `scope-proof-and-session-loop`: Strengthen memory session turn contracts with idempotency, bounded outcome payload ingestion, verification history, and feedback summaries.
- `context-assembly`: Surface feedback-aware context diagnostics and optional ranking hints without changing default context behavior.
- `memory-search-contract`: Allow retrieval diagnostics to include feedback-aware quality signals and optional feedback-aware ranking hints.
- `event-ingestion`: Allow memory-session outcome payloads to be ingested through the existing event contract with explicit session/turn attribution.
- `memory-quality-admission-repair`: Convert repeated negative feedback, missing expected recall, and hidden-memory safety signals into bounded quality findings and approval-gated repair recommendations.
- `admin-inspection-surface`: Add admin inspection for feedback records, usefulness summaries, feedback-linked session reports, and quality/repair links.
- `service-observability`: Add low-cardinality metrics and diagnostics for feedback ingestion, usefulness summaries, session feedback outcomes, and feedback-derived quality findings.

## Impact

- API: Adds scoped feedback endpoints and extends memory session turn, outcome, verification, and report shapes; extends admin inspection routes for feedback and usefulness summaries.
- Storage: Adds PostgreSQL tables or records for feedback events, feedback subject links, feedback supersession, expected-recall targets, feedback summaries, session verification history, idempotency keys, and feedback-linked evidence metadata.
- Workers/scheduler: Adds bounded aggregation or maintenance work for usefulness summaries and feedback-derived quality finding generation where synchronous computation would be too heavy.
- Retrieval/context: Adds diagnostics and optional feedback-aware ranking hints while keeping hidden-memory and scope-safety defaults intact.
- Quality/repair: Reuses existing quality evaluation and repair planning surfaces; feedback may recommend manual review, suppression review, embedding retry, governance inspection, replay, or verification, but approval remains admin-gated.
- Observability/docs: Updates metrics, structured logs where needed, OpenAPI, and self-hosting documentation for the external-agent memory feedback loop.

## Artifact References

- Proposal/apply workflow: `.codex/skills/openspec-propose/SKILL.md`, `.codex/skills/openspec-apply-change/SKILL.md`
- Archive workflow after implementation: `.codex/skills/openspec-archive-change/SKILL.md`, `scripts/openspec-archive-seq.ps1`
- Expected verification: `openspec validate memory-usefulness-feedback-and-agent-session-contract --strict`, `go test ./... -count=1`
