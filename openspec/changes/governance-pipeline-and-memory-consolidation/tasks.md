## 1. Governance Runtime Foundation

- [x] 1.1 Add `internal/governance` and `internal/policy` package boundaries plus worker-facing service contracts for governance jobs
- [x] 1.2 Define job reservation and raw event processing contracts so the worker can claim ungoverned events independently of the HTTP ingest path

## 2. Candidate Memory Layer

- [x] 2.1 Extend the PostgreSQL schema for candidate memory records, governance metadata fields, and raw-event-to-candidate linkage
- [x] 2.2 Define candidate domain types and repository interfaces for creation, lookup, status transition, and provenance recording
- [x] 2.3 Implement PostgreSQL repositories and tests for candidate persistence and candidate status mutation

## 3. Extraction Pipeline

- [x] 3.1 Define extractor interfaces and deterministic extraction outputs from raw event to candidate memory
- [x] 3.2 Implement the first worker extraction flow that loads pending raw events and persists candidate memories with governance metadata
- [x] 3.3 Verify repeated worker execution remains idempotent for already-claimed or already-processed raw events

## 4. Consolidation And Versioning

- [x] 4.1 Define consolidation decision rules for profile supersession, episodic coexistence, and candidate suppression
- [x] 4.2 Implement canonical promotion and append-only memory version writes with provenance links for each mutation
- [x] 4.3 Verify consolidation outcomes for promote, supersede, coexist, and suppress paths with repository-backed tests

## 5. Summary And Compaction

- [x] 5.1 Define summary memory records, cluster selection rules, and provenance linkage requirements
- [x] 5.2 Implement the first compaction flow that creates summary memories from episodic clusters without destroying underlying evidence

## 6. Forgetting And Visibility

- [x] 6.1 Define suppress, expire, and delete action models plus retention evaluation rules
- [x] 6.2 Implement default lifecycle visibility filters so suppressed, forgotten, and expired records are excluded from non-admin reads
- [x] 6.3 Verify forgetting and retention actions change retrieval visibility without losing required audit history
