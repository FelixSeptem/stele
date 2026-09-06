## ADDED Requirements

### Requirement: Self-hosting guidance covers context projection inspection
The self-hosting guide SHALL document how operators inspect the exact-scope
projection status and source watermark, trigger a bounded rebuild, interpret
policy/lifecycle/budget omission diagnostics, and disable projection reads for
rollback without deleting canonical memory or provenance.

#### Scenario: Operator verifies a context projection
- **WHEN** an operator follows the documented projection workflow for an owned
  scope
- **THEN** the guide shows how to inspect projection kind/version/status,
  source watermark, bounded item counts, and lifecycle-safe citations

#### Scenario: Operator rolls back projection consumption
- **WHEN** a projection rollout causes stale or unsafe assembly behavior
- **THEN** the guide directs the operator to disable projection consumption and
  rebuild or remediate forward while retaining all durable source history
