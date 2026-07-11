## ADDED Requirements

### Requirement: Derived insight replay is controllable from the admin surface
The service SHALL expose admin-only operations for replay dry-run planning, replay apply enqueueing, replay run inspection, and replay report reads.

#### Scenario: Operator requests replay dry-run
- **WHEN** an authorized operator submits a bounded replay dry-run request
- **THEN** the admin surface returns a replay plan and does not schedule mutation work

#### Scenario: Operator requests replay apply
- **WHEN** an authorized operator submits a bounded replay apply request with actor and reason attribution
- **THEN** the admin surface creates or returns a durable replay run identity and exposes where to inspect its status

#### Scenario: Operator reads replay report
- **WHEN** an authorized operator requests a replay run or report within the authorized scope
- **THEN** the admin surface returns status, request bounds, counters, failures, skip reasons, actor attribution, and linked insight identifiers when permitted

### Requirement: Replay admin controls preserve lifecycle and scope safety
The service MUST reject replay admin requests that bypass scope isolation, lifecycle governance, or durable background ownership.

#### Scenario: Replay targets unauthorized scope
- **WHEN** a caller requests replay for a tenant, project, or namespace outside its admin authorization
- **THEN** the admin surface rejects the request without exposing evidence or insight counts from that scope

#### Scenario: Replay tries to mutate hidden content directly
- **WHEN** a replay request attempts to make suppressed, forgotten, deleted, or out-of-scope insight content visible without governed lifecycle evaluation
- **THEN** the admin surface rejects the request or records a skipped decision rather than bypassing lifecycle controls
