## MODIFIED Requirements

### Requirement: Budget-aware context shaping

The service MUST support bounded context packing so the assembled response stays
within a caller-provided or service-default budget. Projection-backed and live
retrieval items MUST use the same deterministic budget accounting and MUST fail
closed when an item cannot fit. After normal eligibility, summary preference,
identity/lineage deduplication, and applicable diversity policy, an explicitly
enabled feedback-calibration policy MAY adjust ordering only within the same
budget and section envelope. Calibration MUST NOT increase the budget, change
section names, broaden scope, omit required citations, or bypass lifecycle
visibility. Provider final-output allocations MUST use explicit item, UTF-8 content-byte, unique citation and serialized-response units under `context-budget-v1`, rather than interpreting byte ceilings as item or token limits.

#### Scenario: Context budget is constrained

- **WHEN** a client requests context with a limited budget and an active
  calibration policy is eligible for the exact scope
- **THEN** the assembler applies deterministic baseline packing plus bounded
  calibration ordering and returns only evidence that fits the original budget

#### Scenario: Projection item exceeds remaining budget

- **WHEN** a lifecycle-visible projection item cannot fit within the remaining
  declared output allocation
- **THEN** the item is omitted with a bounded budget reason and the assembler
  does not increase the requested budget or fetch a broader scope

#### Scenario: Feedback signal would exceed the envelope

- **WHEN** a calibration signal would require additional candidates, output allocation, or
  sections beyond the request envelope
- **THEN** the signal is ignored or the request falls back to baseline packing
  without changing visibility or budget

#### Scenario: Diversity selection sees incomplete coverage attributes

- **WHEN** a visible eligible candidate lacks a source session, entity, or time
  slice attribute used by the active diversity policy
- **THEN** the assembler uses a bounded unknown category without loading broader
  source data, changing lifecycle visibility, widening the resolved scope, or
  treating absent feedback as a positive calibration signal

## ADDED Requirements

### Requirement: Public allocation uses retrieval-owned whole-item admission

For Provider final-output allocations, context assembly SHALL apply one deterministic admission policy to the final authorized projection inside the existing selection pipeline. Scope, lifecycle, classification, current-version/source visibility, section authorization, ranking, summary preference, deduplication, quotas, and diversity MUST remain authoritative. Whole selected items and required evidence MUST fit item/content/citation/response limits together. Admission MUST preserve relative order and applicable protected-group priority among fit candidates; it MUST NOT truncate content, strip mandatory citations, replace selected versions, rerank in a second Provider/consumer pipeline, expand the query/path/scope or fetch additional candidates to fill unused space. A deterministic documented conservative reserve for bounded accounting is permitted; maximal packing is not guaranteed.

#### Scenario: High priority evidence fits the allocation

- **WHEN** an ample allocation can hold the existing eligible protected evidence and required citations
- **THEN** final output preserves its baseline ordering, protected recall and section priority

#### Scenario: An early candidate cannot fit

- **WHEN** an eligible candidate exceeds remaining content, citation or response space
- **THEN** it is omitted as a whole and admission continues only through the existing bounded ordered candidate stream without new retrieval or alternate scoring

#### Scenario: Summary and ordinary items compete

- **WHEN** eligible summaries, profiles, session evidence, episodes and relations compete under one public allocation
- **THEN** the existing summary/section policy remains authoritative and all admitted output shares one final allocation including required structured evidence

### Requirement: Allocation cannot reopen closed section paths or alter native defaults

Final-output admission SHALL apply to projection/chunk/optional-section output only within its existing authorized envelope and safe disclosure contract. PC2-closed public projection, insight, goal, feedback and raw-diagnostic paths MUST remain closed, and reasoning/shadow gates MUST remain unchanged. Native/admin assembly without the Provider final-allocation contract MUST preserve its existing defaults and diagnostic permissions. Public budget metadata MUST not expose privileged candidate/channel/latency planning budgets or imply those budgets are output allocations.

#### Scenario: Public caller enables an unsupported optional section

- **WHEN** an ordinary Provider caller requests optional content whose public disclosure contract remains unsupported
- **THEN** no output from that path is admitted and only fixed service-capability labels may be reported

#### Scenario: Authorized internal projection is over budget

- **WHEN** an independently authorized projection or chunk-evidence path uses final-output admission and cannot fit its whole evidence
- **THEN** the item is omitted under the same explicit allocation without widening scope or weakening section/source gates

#### Scenario: Native assembly has no Provider allocation

- **WHEN** a native/admin request uses its existing assembly input without the public final-allocation contract
- **THEN** Provider-specific item/content/body defaults do not change its existing request or output behavior
