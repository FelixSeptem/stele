# context-disclosure-policy Specification

## Purpose
Enforce versioned memory-data disclosure constraints for authenticated runtimes while preserving external ownership of business roles, execution permissions, and prompt construction.

## Requirements

### Requirement: Context disclosure intersects current authenticated authority

The service SHALL resolve context disclosure by intersecting the authenticated principal's current exact-scope disclosure entitlements, validated runtime binding, server policy, requested profile/mode, selectors, lifecycle eligibility, and existing optional-section gates. Initial versioned profiles MUST be `memory-reference-v1` (reference-only) and `memory-content-v1` (reference-only or bounded content), both limited to `low` and `medium` classified evidence. Missing entitlements, unknown/unauthorized profiles, or unauthorized modes MUST fail closed without content, denied identifiers, or evidence-existence details. A caller role, projection intent, path, or cached binding MUST NOT widen authority.

#### Scenario: Caller selects a smaller projection

- **WHEN** a runtime entitled to `memory-content-v1` requests `reference_only`
- **THEN** the service returns only the authorized reference projection for that operation and does not infer a change to the credential's independent entitlements

#### Scenario: Reference-only credential requests content

- **WHEN** a principal entitled only to `memory-reference-v1` requests `bounded_content` or `memory-content-v1`
- **THEN** the service returns the same bounded 403 disclosure denial without returning or looking up protected result content

#### Scenario: Caller manipulates hints or selectors

- **WHEN** a runtime changes projection intent, path/prefix, session headers, or caller role to target evidence outside its authorized envelope
- **THEN** the request is rejected or evidence is omitted under the existing validation/isolation rules without widening disclosure

#### Scenario: Authority changes after binding creation

- **WHEN** an administrator narrows or revokes the disclosure grant after a runtime binding is issued
- **THEN** the next request uses the current entitlement and cannot reuse the binding to recover prior content access

### Requirement: Reference-only projection has a closed structured allowlist

In `reference_only` mode the service MUST return mode-specific typed items containing only authorized object kind/identity, known selected version identity, normalized memory path when applicable, fixed availability, typed source metadata, and bounded authorized evidence citations. It MUST retain required categorized arrays and citation arrays, including empty arrays. Every identifier and path MUST pass exact-scope, classification, lifecycle, and source-visibility checks. It MUST NOT return content, titles, summaries, derived lesson/guidance/avoid/prefer text, goal text, arbitrary metadata or URLs, raw payloads, source-context maps, free-text diagnostics, query echoes, or hidden-count information anywhere in the response. Optional insight references MUST retain their existing authorization gates; goal sections without a supported stable reference contract MUST be omitted.

#### Scenario: All text-bearing sections contain sensitive fixture text

- **WHEN** a reference-mode request selects eligible memory and insight objects whose content, title, summary, lesson, goal, and provenance metadata contain fixture markers
- **THEN** the serialized response contains only the closed reference schema and none of those text-bearing fields or values

#### Scenario: Evidence points into a hidden or foreign source

- **WHEN** a candidate citation or derived item references an ineligible, foreign-scope, high/restricted, hidden, or unverifiable contributing source
- **THEN** the service omits the unsafe citation and omits any derived item whose safe source eligibility cannot be established, without revealing the source identity or path

#### Scenario: Version identity is unavailable

- **WHEN** an otherwise authorized reference lacks a provable selected version identity
- **THEN** the response omits the optional version identity instead of inventing one or substituting a different historical head

#### Scenario: Genuine empty reference projection

- **WHEN** a valid authorized request has no disclosable selected evidence
- **THEN** it returns a typed reference-mode result with all required category and citation collections as empty arrays, distinct from denial or dependency failure

### Requirement: Content projection remains data with explicit source metadata

`bounded_content` MUST require separate profile authorization, reuse existing context eligibility/selection and governed section gates, and disclose typed effective source metadata for every text-bearing object. Unprovable optional-section lineage MUST cause omission. Neither source attestation, profile, projection intent, nor derived guidance SHALL grant external execution, tool, write, task-state, or compiler authority. The final disclosed projection MUST obey `context-budget-v1` item/content/citation/body allocations through deterministic whole-item admission owned by context assembly, without text truncation or removing required evidence. The complete serialized response, including accounting, MUST also satisfy the configured Provider response ceiling; an oversized or unprovable finalized response MUST fail with a bounded non-success outcome rather than partial output or an alternate local ranking/packing mechanism. Caller `budget` retains item-count semantics; safe accounting MUST not expose hidden-source identities/counts or privileged diagnostics.

#### Scenario: Content-capable runtime receives selected evidence

- **WHEN** an authorized runtime requests bounded content and existing selection gates pass
- **THEN** it receives the categorized content and authorized citations with conservative trust/classification labels, without an instruction-authority claim

#### Scenario: Optional text has unknown lineage

- **WHEN** a requested insight or goal text section lacks safely established source metadata or eligibility
- **THEN** the section is omitted rather than assigned a guessed low classification or attested trust

#### Scenario: Serialization exceeds configured ceiling

- **WHEN** the final authorized projection exceeds the configured Provider response limit
- **THEN** the operation returns a bounded non-success response and emits no partial projection

#### Scenario: A content item exceeds caller allocation

- **WHEN** an authorized whole item and its required evidence cannot fit the effective allocation
- **THEN** existing assembly omits that item and returns only fit authorized evidence with safe accounting, without truncating text or changing the requested mode

### Requirement: Alternate routes and acknowledgments cannot bypass disclosure

The service MUST enforce a constrained public principal's disclosure ceiling on every protected content or source-identity response, including Provider retrieve/status/sync, native memory/event/context/history/provenance/session/intent reads, MCP, and write/replay results. A route without equivalent classification, source, and mode enforcement MUST refuse the constrained principal before protected data lookup. Safe write acknowledgments MUST contain only authorized stable IDs and bounded fixed acceptance/replay metadata, never stored text or unrestricted metadata. Fixed content-free capabilities/runtime initialization remain usable under their existing authorization. Narrowing one content-capable request MUST NOT falsely imply that the credential is reference-only across all routes.

#### Scenario: Reference-only principal uses another content endpoint

- **WHEN** the principal calls retrieve, sync, native search/history, or an MCP content tool using the same credential and valid scope
- **THEN** the service either proves equivalent permitted reference disclosure or returns a bounded refusal without content, protected identifiers, or hidden counts

#### Scenario: Caller omits Provider runtime headers

- **WHEN** a constrained principal calls a native or MCP content route directly without a runtime binding
- **THEN** exact-scope principal disclosure enforcement still applies and prevents a bypass

#### Scenario: Write replay would echo stored event text

- **WHEN** a constrained principal ingests or replays its own authorized event
- **THEN** it receives only the safe acknowledgment and cannot obtain stored event text, payload metadata, or broader source access through the replay response

#### Scenario: Disclosure dependency is unavailable

- **WHEN** current grant or required metadata authority cannot be read reliably
- **THEN** the service returns a bounded dependency failure and does not switch to legacy content or report a successful empty projection

### Requirement: Context continuity cannot restore historical disclosure

Every context comparison SHALL recheck current grants, binding, exact scope, lifecycle, classification, source visibility and selected version eligibility through the ordinary authorized assembly boundary. Digest equality and possession of a continuity reference MUST NOT permit historical or locally cached body disclosure. Forgotten, suppressed, deleted, foreign or newly ineligible sources MUST never be resurrected by continuity; fresh results MUST omit such evidence without hidden identifiers/counts or free-text change explanations. Denial or unavailable authority MUST remain a non-success, never unchanged/empty success. Receipts MUST persist no content or historical evidence collection.

#### Scenario: Previously selected evidence is forgotten

- **WHEN** an otherwise valid comparison occurs after selected evidence is forgotten, suppressed or deleted
- **THEN** only the fresh eligible projection is returned, comparison reports changed when semantic identity changes and the former evidence is not disclosed

#### Scenario: Source classification becomes restrictive

- **WHEN** a previously selected source no longer satisfies the runtime's classification/source constraints
- **THEN** fresh comparison excludes the source without revealing its identity or the reason it was removed

#### Scenario: Authorization dependency fails during comparison

- **WHEN** current authority cannot be established despite a previously valid digest/reference
- **THEN** the request fails bounded dependency handling without using historical receipt metadata as authorization
