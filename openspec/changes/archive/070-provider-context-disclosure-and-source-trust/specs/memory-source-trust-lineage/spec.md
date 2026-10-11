## Purpose

Preserve explicit source trust and classification across durable memory versions and derived context so governance cannot transform untrusted data into privileged instructions or weaker disclosure labels.

## ADDED Requirements

### Requirement: Ingestion resolves typed conservative source metadata

Ordinary public event ingestion and Provider events SHALL accept a closed optional `source_metadata` object using `source-metadata-v1`, trust values `untrusted` or `attested_data`, and classification values `low`, `medium`, `high`, or `restricted`. Missing metadata MUST resolve to `untrusted/restricted`; malformed types, unknown values, and unsupported versions MUST be rejected. Server scope/source floors MUST apply. Application metadata, event type, actor strings, memory class, or confidence MUST NOT confer trust or disclosure authority. An `attested_data` write MUST require current explicit exact-scope source-attestation entitlement; this is separate from disclosure permission and MUST NOT imply correctness or instruction authority.

#### Scenario: Ordinary write omits source labels

- **WHEN** an authorized writer submits a valid event without typed source metadata
- **THEN** the persisted event has effective untrusted trust and restricted classification and its application metadata cannot override those defaults

#### Scenario: Writer attempts to self-attest

- **WHEN** a writer without explicit source-attestation entitlement submits `attested_data` or a forged attestation in application metadata
- **THEN** typed self-attestation is refused and application metadata grants no attestation authority

#### Scenario: Authorized source is attested

- **WHEN** a currently authorized source-attesting principal writes a valid event with typed attested-data metadata
- **THEN** the service persists the source label with authenticated attribution and audit lineage without granting instruction or execution permissions

### Requirement: Source metadata participates in durable write identity

Resolved typed source metadata SHALL participate in the normalized event idempotency identity. An equivalent replay MUST preserve the original event/metadata/lineage without duplicate writes; a changed trust or classification under the same key MUST be a conflict. Current write/attestation authorization MUST still apply to replay, and an acknowledgment MUST respect the caller's disclosure ceiling.

#### Scenario: Equivalent labeled write is retried after restart

- **WHEN** a writer retries the same normalized content/path/source metadata and idempotency key after restart
- **THEN** the service returns the original event identity and bounded replay status with unchanged durable labels

#### Scenario: Retry changes classification or trust

- **WHEN** the same principal/scope/key is reused with different resolved typed source metadata
- **THEN** the service returns a bounded conflict and does not relabel the original event or create a duplicate

### Requirement: Derivation preserves the most restrictive source labels

Extraction, promotion, supersession, summary creation/compaction, insights/reflection, mutations/intents/merge, chunks, and context projections MUST preserve typed source lineage. Classification MUST be the maximum of contributing source labels, candidate sensitivity, and server floors under `low < medium < high < restricted`. Derived trust MUST be attested-data only when every contributing source is independently attested and lineage is complete; otherwise it MUST be untrusted. Missing/unknown source metadata MUST contribute untrusted/restricted. Automated governance, summarization, success reports, or model output MUST NOT upgrade trust or reduce classification.

#### Scenario: Candidate is promoted without changing content

- **WHEN** governance promotes a labeled non-root-path event through a candidate into canonical memory
- **THEN** candidate, canonical version, and provenance preserve the path and effective source labels without upgrading trust

#### Scenario: Mixed-source summary is created

- **WHEN** an attested low-classification source and an untrusted medium-classification source contribute to a summary or insight
- **THEN** the derived artifact is untrusted and at least medium-classification, with versioned lineage to both sources

#### Scenario: A derivation has incomplete source metadata

- **WHEN** any contributing source lacks a valid metadata snapshot or complete lineage
- **THEN** derivation remains untrusted/restricted and cannot be disclosed through an ordinary low/medium profile

#### Scenario: Classification floor exceeds writer labels

- **WHEN** a source is labeled low but its candidate sensitivity or server floor is high
- **THEN** derived classification remains at least high and the low label cannot bypass disclosure eligibility

### Requirement: Source labels are append-only version snapshots

The service SHALL durably preserve source labels on raw events, candidates, canonical representations, individual canonical versions, and versioned derived artifacts. Material canonical label changes MUST create a new version with audit/provenance, never overwrite prior version snapshots. Source-label migration MUST conservatively handle legacy records without retroactive trust upgrades or classification-lowering assertions. Restart, lifecycle transitions, rebuild, and rollback MUST preserve labels and historical lineage.

#### Scenario: Canonical update creates a new version

- **WHEN** governed consolidation changes content or effective source labels
- **THEN** a new version retains the new snapshot while prior versions retain their original metadata and provenance

#### Scenario: Projection is rebuilt after source changes

- **WHEN** a canonical source version changes or becomes hidden
- **THEN** a new derived projection uses current eligible version metadata without rewriting canonical history or making prior hidden content visible

#### Scenario: Legacy schema is upgraded

- **WHEN** the service migrates historical records lacking source metadata
- **THEN** absent/unprovable source labels resolve conservatively to untrusted/restricted and no record is automatically attested or asserted to be low-classification

### Requirement: Cached and derived context cannot launder source restrictions

Context eligibility and disclosure MUST validate the selected version's source metadata and the current eligibility of contributing evidence. Cached summaries, chunks, insights, and projections MUST NOT disclose foreign, hidden, high/restricted, stale, or unverifiable sources indirectly. Unsupported optional derivation visibility MUST be omitted; unavailable authoritative metadata dependencies MUST fail explicitly rather than produce a successful empty or trusted result.

#### Scenario: Cached summary contains forgotten evidence

- **WHEN** a cached summary references a source that has been forgotten or suppressed since materialization
- **THEN** ordinary context excludes the unsafe derived text/reference and exposes no hidden source identity or count

#### Scenario: Source version does not match cached metadata

- **WHEN** a cached item cannot prove metadata for its selected source version
- **THEN** the service uses a safe authorized source path only if eligibility is established, otherwise omits the item without guessing labels
