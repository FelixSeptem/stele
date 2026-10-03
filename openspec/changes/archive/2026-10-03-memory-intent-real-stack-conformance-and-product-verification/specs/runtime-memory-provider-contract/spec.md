## ADDED Requirements

### Requirement: Provider conformance covers governed memory intent operations
The provider conformance suite MUST include the supported intent operations and scoped status/history reads as contract cases, while preserving the existing event, retrieval, context, and lifecycle operation families.

#### Scenario: Provider replays a governed intent
- **WHEN** a conformance fixture submits a valid intent and repeats the same request after a runtime restart
- **THEN** the provider returns the original intent identity and bounded replay outcome without creating a duplicate request or transition

#### Scenario: Provider rejects an unsafe intent operation
- **WHEN** a fixture uses a foreign scope, conflicting idempotency payload, unsupported type, missing attribution, or hidden lifecycle target
- **THEN** the provider returns a bounded contract failure and does not expose foreign content or mutate canonical memory directly

### Requirement: Provider conformance proves intent processing dependencies
An intent conformance result MUST remain ineligible for readiness when the durable queue, worker recovery, migration compatibility, or required policy dependency is missing, stale, or degraded.

#### Scenario: Intent dependency is degraded
- **WHEN** the provider can accept an intent request but the durable worker or compatible policy cannot process it
- **THEN** the conformance report records a dependency or readiness failure and does not claim provider readiness

#### Scenario: Intent processing recovers after restart
- **WHEN** the provider conformance runner restarts the affected runtime after durable intent handoff
- **THEN** it observes recovery through the same intent identity and records the replay, lease, and lifecycle result separately from retrieval accuracy metrics
