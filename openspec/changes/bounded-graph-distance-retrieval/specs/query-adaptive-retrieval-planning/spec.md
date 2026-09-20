## MODIFIED Requirements

### Requirement: Retrieval plans are complete, bounded, and replayable
The service SHALL produce a complete retrieval plan containing the planner and
policy identities, query family, enabled existing recall channels, per-channel
and aggregate candidate budgets, fusion identity and parameters, memory-class
quotas, reranker eligibility, context-section priorities, pass limit, latency
budget, fallback identity, and—only for an enabled `entity_relation` or
`multi_hop` plan—the exact-scope graph-traversal policy identity, traversal
disposition, hop limit, and graph resource limits. Missing, unknown,
incompatible, negative, or over-limit parameters MUST be rejected before plan
execution. Graph parameters MUST be absent or disabled for every other query
family and MUST NOT exceed the deployment traversal hard envelope.

#### Scenario: Valid plan is constructed
- **WHEN** an approved planner policy resolves every required field within the
  service hard limits
- **THEN** the service accepts one immutable plan whose complete identity and
  bounded parameters can be replayed and compared

#### Scenario: Plan exceeds a service hard limit
- **WHEN** a policy requests a channel, candidate count, weight, quota, pass
  count, latency, context allowance, graph hop count, or graph resource limit
  beyond the service hard limit
- **THEN** the service rejects the plan and executes the approved baseline
  fallback rather than silently increasing the limit

#### Scenario: Graph traversal is proposed for an unsupported family
- **WHEN** a planner policy supplies an enabled graph-traversal disposition for
  exact lookup, semantic, temporal, procedural, or general fallback
- **THEN** the service rejects that disposition before repository access and
  executes the approved baseline fallback
