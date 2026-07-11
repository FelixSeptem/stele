## ADDED Requirements

### Requirement: Context assembly exposes feedback-aware diagnostics
The service SHALL expose bounded usefulness feedback diagnostics when assembling context for an authorized scope.

#### Scenario: Context item has feedback history
- **WHEN** an assembled context item has useful, noisy, stale, irrelevant, unsafe, or needs-review feedback history
- **THEN** context diagnostics can include bounded feedback reason codes and aggregate summary fields for that item

#### Scenario: Feedback references hidden memory
- **WHEN** feedback history references suppressed, forgotten, expired, deleted, or out-of-scope memory
- **THEN** context assembly does not expose the hidden content and only surfaces lifecycle-safe aggregate diagnostics when authorized

### Requirement: Feedback-aware ranking is explicit
The service MUST NOT silently change default context ranking based on usefulness feedback.

#### Scenario: Default context assembly runs
- **WHEN** a caller assembles context without an explicit feedback-aware ranking request
- **THEN** usefulness feedback can appear in diagnostics but does not alter the default context ranking behavior

#### Scenario: Feedback-aware ranking is requested
- **WHEN** a caller explicitly enables feedback-aware ranking for one context request
- **THEN** the service may use bounded usefulness summaries as ranking hints while preserving lifecycle, scope, budget, and citation safety rules

#### Scenario: Scope-wide ranking policy is requested
- **WHEN** an operator attempts to enable feedback-aware context ranking as a default scope-wide policy in this change
- **THEN** the service rejects or ignores that setting because this proposal only supports explicit per-request ranking hints
