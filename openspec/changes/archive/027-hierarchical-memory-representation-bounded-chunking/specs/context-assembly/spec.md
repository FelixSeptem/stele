## ADDED Requirements

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
