## ADDED Requirements

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
