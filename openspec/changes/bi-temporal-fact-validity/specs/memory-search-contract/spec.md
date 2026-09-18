## MODIFIED Requirements

### Requirement: Public memory search contract
The service SHALL expose a stable memory search contract for governed retrieval over canonical memory and summary memory. The request SHALL preserve existing recorded-time `time_from`/`time_to` filters and may add explicit valid-time selectors such as `as_of` or `valid_during`.

#### Scenario: Search request declares retrieval scope and filters
- **WHEN** a client submits a memory search request
- **THEN** the request supports query text, scope filters, class filters, recorded-time window, optional valid-time selector, top-k, summary inclusion, and optional relation inclusion

#### Scenario: Search response returns ranked governed memory hits
- **WHEN** the service returns a successful search response
- **THEN** the response includes ranked hits with stable memory identifiers, citations, and score metadata

#### Scenario: Historical selector is explicit
- **WHEN** a client requests `as_of` or `valid_during`
- **THEN** the request is represented as a temporal constraint that requires an authorized temporal plan and cannot silently broaden ordinary current retrieval

### Requirement: Structured search result metadata
The service MUST return enough retrieval metadata to support downstream SDK use without exposing raw internal storage details. Temporal results MUST identify the selected canonical version and bounded validity metadata when the caller is authorized to receive it.

#### Scenario: Client inspects a ranked result
- **WHEN** a client receives a search result hit
- **THEN** the result includes memory class, lifecycle-safe state representation, recorded timestamps, and citation-ready provenance references

#### Scenario: Authorized client inspects temporal result
- **WHEN** an authorized temporal search returns a historical hit
- **THEN** the result includes the selected version identity and bounded validity disposition without exposing hidden versions or raw storage details
