# context-assembly Specification

## Purpose
Define the structured context assembly surface that turns governed retrieval results into agent-ready sections with citations and budget-aware packing.
## Requirements
### Requirement: Agent-ready context assembly endpoint
The service SHALL expose a context assembly capability that returns structured
sections rather than a flat result list. Sections MAY be populated from an
authorized, lifecycle-visible versioned context projection as well as live
retrieval, while preserving the existing response shape and citation contract.

#### Scenario: Context is assembled for an agent request
- **WHEN** a client requests assembled context for a scoped query or interaction
- **THEN** the service returns structured sections including `profile`,
  `recent_session`, `recent_episodes`, `relevant_summaries`,
  `related_entities`, and `citations`

#### Scenario: Projection is available for the request
- **WHEN** an exact-scope verified projection contains eligible always-visible
  or session items
- **THEN** the assembler can include those items with redacted source citations
  and bounded projection diagnostics without changing section names

### Requirement: Summary-preferred context packing
Context assembly MUST prefer summary memory when it can represent a relevant episodic cluster without losing required evidence traceability.

#### Scenario: Summary can replace dense episodic detail
- **WHEN** a relevant summary memory exists for a dense or stale episodic cluster
- **THEN** the service includes the summary before expanding the full underlying episodic set and preserves evidence citations

### Requirement: Budget-aware context shaping
The service MUST support bounded context packing so the assembled response stays
within a caller-provided or service-default budget. Projection-backed and live
retrieval items MUST use the same deterministic budget accounting and MUST fail
closed when an item cannot fit.

#### Scenario: Context budget is constrained
- **WHEN** a client requests context assembly with a limited budget
- **THEN** the service trims and prioritizes sections according to retrieval or
  projection policy ordering and summary preference instead of returning
  unbounded memory

#### Scenario: Projection item exceeds remaining budget
- **WHEN** a lifecycle-visible projection item cannot fit within the remaining
  character/token budget
- **THEN** the item is omitted with a bounded budget reason and the assembler
  does not increase the requested budget or fetch a broader scope

### Requirement: Context assembly can include governed experience insights
The service SHALL support optional context assembly sections for active, evidence-backed derived insights.

#### Scenario: Caller requests known failure context
- **WHEN** a scoped context assembly request asks to include experience insights
- **THEN** the response can include `known_failures` entries derived from active `failure_pattern` insights that match the request scope and budget

#### Scenario: Caller requests experience lessons
- **WHEN** a scoped context assembly request asks to include experience lessons
- **THEN** the response can include `experience_lessons` entries that are backed by active failure patterns and evidence citations

#### Scenario: Insight section is not requested
- **WHEN** a context assembly request does not request experience insight sections
- **THEN** the service preserves the existing context assembly shape and does not include derived insights by default

#### Scenario: Hidden insight exists
- **WHEN** a matching derived insight is suppressed, forgotten, deleted, or out of scope
- **THEN** context assembly excludes that insight from `known_failures` and `experience_lessons`

### Requirement: Insight context assembly respects quality feedback
The service SHALL account for effective quality feedback when assembling optional derived insight context sections.

#### Scenario: Useful insight competes under budget
- **WHEN** multiple active derived insights match an optional insight section and the context budget is constrained
- **THEN** context assembly can prioritize insights with active `useful` feedback when evidence relevance is otherwise comparable

#### Scenario: Noisy insight competes under budget
- **WHEN** a matching insight has active `noisy`, `incorrect`, `stale`, or `needs_review` feedback
- **THEN** context assembly deprioritizes or omits that insight according to policy and budget without exposing raw admin feedback by default

#### Scenario: Feedback summary is requested for diagnostics
- **WHEN** an authorized admin or debug assembly path requests quality diagnostics for insight sections
- **THEN** the response can include summarized quality state without exposing feedback history through ordinary public context assembly

### Requirement: Feedback does not bypass lifecycle visibility rules
The service MUST keep lifecycle state and scope isolation as the primary visibility controls for insight context sections.

#### Scenario: Useful feedback exists on hidden insight
- **WHEN** an insight is suppressed, forgotten, deleted, or out of scope but has active `useful` feedback
- **THEN** context assembly excludes that insight from ordinary `known_failures` and `experience_lessons` sections

#### Scenario: Negative feedback exists on active insight
- **WHEN** an active insight has negative feedback but has not been suppressed by governed policy
- **THEN** context assembly treats the feedback as a ranking or omission signal rather than rewriting the insight lifecycle inline

### Requirement: Context assembly can verify replayed insight visibility
The service SHALL allow the operator smoke loop and authorized diagnostics to verify whether replayed active insights participate in optional context assembly sections according to scope, lifecycle, quality, and budget rules.

#### Scenario: Replayed insight is active and requested
- **WHEN** replay produces or updates an active insight that matches a scoped context assembly request with insight sections enabled
- **THEN** context assembly can include that insight in `known_failures` or `experience_lessons` with citations when budget and quality policy allow it

#### Scenario: Replayed insight is hidden or out of scope
- **WHEN** replay preserves, suppresses, skips, or creates an insight outside the request scope or visible lifecycle states
- **THEN** ordinary context assembly excludes that insight even if the replay report references it

#### Scenario: Operator requests context diagnostics
- **WHEN** an authorized admin or debug path requests diagnostics for replayed insight context
- **THEN** the response can identify whether replay output was included, omitted by budget, omitted by quality policy, or hidden by lifecycle and scope rules

### Requirement: Context assembly supports proof and session diagnostics
The service SHALL allow proof and memory-session workflows to request context assembly with diagnostic attribution.

#### Scenario: Session turn assembles context
- **WHEN** a memory session starts a turn with a scoped query
- **THEN** context assembly returns agent-ready sections and records the memory ids, citations, and diagnostics used as session turn evidence

#### Scenario: Proof verifies expected context recall
- **WHEN** a proof run checks whether fixture memory appears in context
- **THEN** context assembly can report whether expected evidence was included, omitted by budget, omitted by quality, hidden by lifecycle, or unavailable

#### Scenario: Ordinary context request is unchanged
- **WHEN** a caller uses context assembly without proof or session attribution
- **THEN** the service preserves the existing context assembly behavior and response shape

### Requirement: Context assembly exposes feedback-aware diagnostics
The service SHALL expose bounded usefulness feedback diagnostics when assembling context for an authorized scope.

#### Scenario: Context item has feedback history
- **WHEN** an assembled context item has useful, noisy, stale, irrelevant, unsafe, or needs-review feedback history
- **THEN** context diagnostics can include bounded feedback reason codes and aggregate summary fields for that item

#### Scenario: Feedback references hidden memory
- **WHEN** feedback history references suppressed, forgotten, expired, deleted, or out-of-scope memory
- **THEN** context assembly does not expose the hidden content and only surfaces lifecycle-safe aggregate diagnostics when authorized

### Requirement: Feedback-aware ranking is explicit
The service MUST NOT silently change default context ranking based on usefulness feedback or task-success evidence unless feedback-aware ranking is explicitly requested for one context request or an authorized scoped rollout policy is active.

#### Scenario: Default context assembly runs without active policy
- **WHEN** a caller assembles context without an explicit feedback-aware ranking request and no active matching rollout policy exists
- **THEN** usefulness feedback and task-success signals can appear in diagnostics but do not alter the default context ranking behavior

#### Scenario: Feedback-aware ranking is requested
- **WHEN** a caller explicitly enables feedback-aware ranking for one context request
- **THEN** the service may use bounded usefulness summaries as ranking hints while preserving lifecycle, scope, budget, and citation safety rules

#### Scenario: Governed scope policy is active
- **WHEN** an authorized ranking rollout policy is active for the context surface and resolved scope
- **THEN** context assembly may use active usefulness feedback, task-success summaries, verification outcomes, and quality signals as bounded ranking hints while preserving lifecycle, budget, section, and citation safety rules

#### Scenario: Scope-wide ranking policy is requested without governance
- **WHEN** an operator attempts to enable feedback-aware context ranking as a default scope-wide setting outside the ranking rollout governance contract
- **THEN** the service rejects or ignores that setting and preserves baseline context ranking

#### Scenario: Ranking policy references hidden evidence
- **WHEN** active ranking signals reference suppressed, forgotten, expired, deleted, or out-of-scope memory
- **THEN** context assembly excludes hidden memory from ordinary sections and exposes only lifecycle-safe aggregate diagnostics where authorized

### Requirement: Context assembly exposes ranking rollout diagnostics
The service SHALL expose bounded diagnostics explaining how an active or dry-run ranking rollout policy affected context candidate priority and section packing.

#### Scenario: Context runs with rollout diagnostics
- **WHEN** an authorized request asks for ranking rollout diagnostics or dry-run comparison
- **THEN** the response can include baseline priority, adjusted priority, included or omitted status, bounded reason codes, signal categories, and evidence counts for lifecycle-visible context items

#### Scenario: Ranking changes budget outcome
- **WHEN** rollout ranking changes whether a lifecycle-visible item is included under the requested context budget
- **THEN** diagnostics can report the budget-safe inclusion or omission reason without exposing hidden memory content

### Requirement: Context assembly reports projection-safe diagnostics
The service SHALL expose bounded diagnostics for projection inclusion, staleness,
policy omission, lifecycle exclusion, scope mismatch, and budget omission
without exposing hidden content, raw event payloads, or foreign identifiers.

#### Scenario: Projection item is omitted safely
- **WHEN** a projection item is omitted because of lifecycle, policy, scope, or
  budget validation
- **THEN** diagnostics include only a stable reason category and bounded counts
  for the affected section

### Requirement: Context assembly packs chunk evidence within existing budgets
The service SHALL allow a chunk-derived retrieval hit to contribute bounded source,
parent, or adjacent evidence to existing context sections only when the evidence is
authorized, lifecycle-visible, exactly scoped, and fits the existing context budget.
The service MUST preserve existing section names and citation safety rules.

#### Scenario: Chunk evidence fits remaining context budget
- **WHEN** a visible chunk-derived result and its validated parent or adjacent
  evidence fit the remaining context budget
- **THEN** context assembly can include the bounded evidence with source citations
  in the applicable existing section

#### Scenario: Parent expansion exceeds budget or fails validation
- **WHEN** parent or adjacent evidence exceeds the remaining budget or cannot prove
  exact-scope lifecycle visibility
- **THEN** context assembly omits that evidence with a bounded authorized diagnostic
  and does not enlarge the budget or broaden the scope

### Requirement: Chunk context diagnostics are lifecycle-safe
The service SHALL expose chunk-related inclusion, source-validation, parent-context,
and budget-omission diagnostics only through authorized diagnostic paths. These
diagnostics MUST use bounded reason categories and MUST NOT disclose hidden source
content, foreign identifiers, or raw event payloads.

#### Scenario: Hidden chunk source is evaluated for context
- **WHEN** a chunk's source is suppressed, forgotten, expired, deleted, or out of
  scope during context assembly
- **THEN** the chunk is excluded and any authorized diagnostic reports only a stable
  aggregate lifecycle or scope reason

