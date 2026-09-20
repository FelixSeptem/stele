## MODIFIED Requirements

### Requirement: Bounded manual canonical memory update
The service MUST expose a bounded update surface for canonical memory corrections without allowing destructive in-place overwrite of governed history. Factual corrections MUST validate temporal identity and half-open interval rules.

#### Scenario: Operator corrects canonical content
- **WHEN** a privileged caller updates the content or validity of an eligible canonical memory
- **THEN** the service preserves the stable `memory_id`, appends a new memory version, updates the current canonical projection, and records a manual update provenance operation with the temporal correction disposition

#### Scenario: Operator attempts to mutate excluded fields
- **WHEN** a privileged caller attempts to change scope, lifecycle state, or class through the bounded update endpoint
- **THEN** the service rejects the request and requires the caller to use the dedicated governance surface for that concern

#### Scenario: Operator submits an invalid interval
- **WHEN** a privileged caller submits a validity interval whose bounds are malformed or conflict with the temporal identity policy
- **THEN** the service rejects the update without changing canonical or historical records
