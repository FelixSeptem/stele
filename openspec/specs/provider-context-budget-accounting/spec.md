# provider-context-budget-accounting Specification

## Purpose
Define a measurable allocation for final authorized Provider context output, with explicit units, bounded accounting, and selection reasons that preserve disclosure and lifecycle safety.

## Requirements

### Requirement: Caller allocation has explicit independent units

Provider context SHALL retain positive `budget` as a maximum returned item-placement count and accept an optional closed `budget_limits` object containing `max_content_bytes`, `max_response_bytes`, and `max_citations`. Limits MUST be integers no greater than 2,147,483,647; item and response limits MUST be positive, and content/citation limits MUST permit explicit zero. Omitted optional limits MUST resolve to published server ceilings, independently intersected with caller limits. Nulls, duplicate keys, case aliases, unknown/token fields, wrong types, negative values, overflow, and trailing JSON MUST produce bounded 400 `invalid_request` errors without assembly or successful output. Units MUST NOT be inferred or converted to model tokens.

#### Scenario: Caller omits optional limits

- **WHEN** a valid request supplies only a positive item budget and no limits object, or supplies an empty limits object
- **THEN** each optional effective limit equals its published server ceiling and requested accounting identifies those fields as omitted

#### Scenario: Requested limits exceed server policy

- **WHEN** valid caller limits exceed one or more server ceilings
- **THEN** the service reports the independently clamped effective values and uses them before bounded candidate arithmetic

#### Scenario: Zero allocation is explicit

- **WHEN** an authorized caller supplies zero content bytes or zero citations
- **THEN** zero remains the effective limit, content mode is not silently changed, and no item lacking space for its whole content and mandatory citation is returned

#### Scenario: Unsupported token allocation is submitted

- **WHEN** a request supplies `max_tokens`, `tokenizer`, a fractional limit, a null limit, a duplicate nested key, or an overflowing integer
- **THEN** it receives the documented bounded validation failure without a byte-to-token estimate or a successful fallback

### Requirement: Server ceilings separate items content bytes response bytes and citations

The service SHALL publish independent context item, content-byte, response-byte and citation ceilings. Defaults MUST be 100 items, content bytes inherited from the resolved response ceiling, the existing 1 MiB response ceiling, and 100 citations. Hard bounds MUST be 1,000 items, 4 MiB content/response bytes, and 1,000 citations. Configured item/response/citation ceilings MUST be positive, configured content bytes MUST permit zero and MUST NOT exceed the response ceiling, and invalid configuration MUST fail startup. Retrieval-result ceilings MUST retain their separate meaning. Discovery MUST advertise `context-budget-v1`, `context-packing-v1`, resolved limits and unsupported token-budget capability without principal-specific or privileged state.

#### Scenario: Operator overrides response bytes only

- **WHEN** the response ceiling is overridden and content configuration is omitted
- **THEN** the content ceiling inherits that resolved response value while item/citation defaults remain independent

#### Scenario: Configuration conflicts with bounds

- **WHEN** a configured item ceiling exceeds 1,000 or a content ceiling exceeds the configured response ceiling
- **THEN** startup fails with a bounded configuration error rather than silently advertising another limit

### Requirement: Usage measures the final disclosed representation

Every successful response SHALL report actual returned item placements, sum of UTF-8 bytes of disclosed content strings, unique canonical `(kind, id, version identity if present)` citation tuples, and exact uncompressed JSON body bytes. Reference-only content usage MUST be zero. Every repeated item placement and displayed content string MUST consume its corresponding budget; repeated serialized citation appearances MUST each consume response bytes while sharing a unique citation identity. The complete body measurement MUST include metadata, paths, source metadata, category/citation arrays, JSON escapes and the accounting block itself, excluding HTTP headers/framing/compression. The route MUST emit exactly the measured body with no added newline or formatting. Each usage dimension MUST not exceed its effective limit.

#### Scenario: Multibyte content and escaping are returned

- **WHEN** selected content includes Chinese, emoji, quotes, backslashes or control characters
- **THEN** content usage equals its UTF-8 source-string bytes and response usage equals the escaped actual HTTP body bytes

#### Scenario: Reference-only mode has large provenance

- **WHEN** an authorized reference response includes long normalized paths, source metadata and repeated structured citations
- **THEN** item, unique citation and full body budgets constrain admission even though content usage remains zero

#### Scenario: Response size crosses a decimal digit boundary

- **WHEN** the exact response-byte value changes the serialized digit width of its own accounting field
- **THEN** the reported value equals the final emitted body length and the complete body remains within the effective ceiling

### Requirement: Safe accounting distinguishes selection outcomes

Every successful categorized result SHALL contain closed `budget_accounting` with contract and fixed public packing-policy versions, requested/effective limits, actual usage, and selection metadata. Requested optional limits MUST use null only to identify omission; effective/usage values MUST be nonnegative integers with positive effective item/response limits. Selection outcome MUST be `selected`, `empty`, or `budget_exhausted`. Omission reasons MUST be unique ordered enums `item_limit`, `content_bytes`, `citations`, `response_bytes`, derived only from already disclosable candidates in the bounded assembly stream. Unsupported sections MUST be unique ordered fixed public capability labels `experience_insights`, `goal_context`, `diagnostics`, `feedback_diagnostics` and MUST appear only when requested and unsupported. No hidden/source-rejection counts, record identifiers, query plans, raw scores, quality histories, free text or privileged policy identity may appear in this accounting. A fixed budget/packing version MUST NOT be presented as a projection digest or business authority.

#### Scenario: All eligible evidence is too large

- **WHEN** the bounded stream has disclosable candidates but none fits the allocation and its finalized empty accounting envelope fits
- **THEN** the service returns empty required arrays with `budget_exhausted`, actual usage and authorized budget reasons

#### Scenario: No disclosable evidence exists

- **WHEN** a valid authorized request has no disclosable candidates in its bounded assembly stream
- **THEN** it returns `empty` without hidden-evidence or budget-omission claims

#### Scenario: Some candidates fit

- **WHEN** at least one whole eligible item is admitted and other evaluated eligible items cannot fit
- **THEN** outcome is `selected` and the bounded ordered reason list describes only applicable allocation constraints

#### Scenario: Unsupported public sections are requested

- **WHEN** a public request enables insight, goal or raw diagnostic options currently closed by disclosure policy
- **THEN** accounting reports only the corresponding fixed unsupported capability labels, with no section content, record-existence information or private rejection count

### Requirement: Infeasible serialization and authoritative failures remain non-success

If the mandatory finalized empty response cannot fit the effective response allocation, or final serialization/accounting cannot prove all limits, the service MUST return bounded 413 `context_limit_exceeded` with no result, partial projection or streamed prefix. Error bodies MUST remain server-bounded and MUST NOT promise compliance with the caller's success-body allocation. Current disclosure denial MUST retain bounded 403 `disclosure_denied`; unavailable authoritative dependencies MUST produce a bounded retryable 503 dependency failure rather than `empty` or `budget_exhausted`. The final configured response ceiling MUST remain an independent defense after admission.

#### Scenario: Caller supplies a one-byte response limit

- **WHEN** a valid request's mandatory envelope exceeds its effective response limit
- **THEN** the service returns the documented 413 outcome without performing a broader search or emitting a context result

#### Scenario: Final cost and serialized body disagree

- **WHEN** assembly completes but the final body exceeds a limit or exact accounting cannot converge
- **THEN** the operation fails closed and does not drop a response suffix, remove citations or report a false successful empty projection

#### Scenario: Current authority cannot be loaded

- **WHEN** current disclosure grants or required source metadata are unavailable
- **THEN** the operation returns the bounded dependency failure without substituting a smaller successful projection
