## MODIFIED Requirements

### Requirement: Manual mutations preserve retrieval projection consistency
The service MUST keep retrieval projections consistent when manual mutation changes canonical content or class.

#### Scenario: Manual mutation materially changes canonical content
- **WHEN** a manual mutation changes the retrievable text or class of canonical memory
- **THEN** the service refreshes lexical or relation projections as needed, prevents stale semantic embeddings from continuing to participate in default retrieval, and marks the current canonical projection eligible for durable semantic rebuild

#### Scenario: Manual mutation preserves vector audit continuity
- **WHEN** a manual mutation invalidates the previously active semantic projection
- **THEN** the service keeps the prior vector revision auditable, records that the new canonical projection requires rebuild, and does not silently overwrite semantic lineage in place
