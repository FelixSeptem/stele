## ADDED Requirements

### Requirement: Agent-ready context assembly endpoint
The service SHALL expose a context assembly capability that returns structured sections rather than a flat result list.

#### Scenario: Context is assembled for an agent request
- **WHEN** a client requests assembled context for a scoped query or interaction
- **THEN** the service returns structured sections including `profile`, `recent_session`, `recent_episodes`, `relevant_summaries`, `related_entities`, and `citations`

### Requirement: Summary-preferred context packing
Context assembly MUST prefer summary memory when it can represent a relevant episodic cluster without losing required evidence traceability.

#### Scenario: Summary can replace dense episodic detail
- **WHEN** a relevant summary memory exists for a dense or stale episodic cluster
- **THEN** the service includes the summary before expanding the full underlying episodic set and preserves evidence citations

### Requirement: Budget-aware context shaping
The service MUST support bounded context packing so the assembled response stays within a caller-provided or service-default budget.

#### Scenario: Context budget is constrained
- **WHEN** a client requests context assembly with a limited budget
- **THEN** the service trims and prioritizes sections according to retrieval ranking and summary preference instead of returning unbounded memory
