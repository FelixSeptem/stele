## MODIFIED Requirements

### Requirement: Usage measures the final disclosed representation

Every successful response SHALL report actual returned item placements, sum of UTF-8 bytes of disclosed content strings, unique canonical `(kind, id, version identity if present)` citation tuples, and exact uncompressed JSON body bytes. Reference-only content usage MUST be zero. Every repeated item placement and displayed content string MUST consume its corresponding budget; repeated serialized citation appearances MUST each consume response bytes while sharing a unique citation identity. The complete body measurement MUST include metadata, paths, source metadata, category/citation arrays, JSON escapes the accounting block itself, and all required context digest/continuity fields, excluding HTTP headers/framing/compression. The route MUST emit exactly the measured body with no added newline or formatting. Each usage dimension MUST not exceed its effective limit.

#### Scenario: Multibyte content and escaping are returned

- **WHEN** selected content includes Chinese, emoji, quotes, backslashes or control characters
- **THEN** content usage equals its UTF-8 source-string bytes and response usage equals the escaped actual HTTP body bytes

#### Scenario: Reference-only mode has large provenance

- **WHEN** an authorized reference response includes long normalized paths, source metadata and repeated structured citations
- **THEN** item, unique citation and full body budgets constrain admission even though content usage remains zero

#### Scenario: Response size crosses a decimal digit boundary

- **WHEN** the exact response-byte value changes the serialized digit width of its own accounting field
- **THEN** the reported value equals the final emitted body length and the complete body remains within the effective ceiling

#### Scenario: Digest and continuity envelopes consume the allocation

- **WHEN** a fresh or compare context result includes required digest/continuity metadata, reference and expiry where applicable
- **THEN** preflight and admission reserve their complete serialized cost, final usage includes their exact bytes, and an infeasible mandatory empty envelope fails without persisting a new receipt
