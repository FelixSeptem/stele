## 1. Provider Runtime Wiring

- [x] 1.1 Add a shared embedding provider registry builder that turns runtime config into a concrete `embedding.ProviderResolver` for `api`, `worker`, and `scheduler`
- [x] 1.2 Introduce startup validation for declared embedding routes so unknown or invalid configured providers fail honestly while lexical-only mode remains allowed
- [x] 1.3 Add provider runtime tests covering consistent mode wiring, degraded lexical-only startup, and invalid configured provider failure

## 2. Embedding Admin Inspection

- [x] 2.1 Add repository and service query contracts for scoped embedding backlog inspection, one-memory rebuild status, and vector revision lineage reads
- [x] 2.2 Implement admin HTTP routes and OpenAPI surface for embedding backlog inspection and memory-level embedding diagnostics under the existing admin auth boundary
- [x] 2.3 Add handler and service tests that verify scope isolation, drift and failure visibility, and degraded semantic runtime diagnostics

## 3. Operator Remediation Controls

- [x] 3.1 Add durable remediation operations that can retry or requeue eligible embedding rebuild records with actor and reason attribution
- [x] 3.2 Enforce lease-safety and state validation so remediation rejects actively rebuilding work and never mutates vector revision history directly
- [x] 3.3 Add repository, service, and HTTP tests for retry, requeue, invalid state transitions, and audit attribution

## 4. Bootstrap And Verification

- [x] 4.1 Update self-hosting docs and environment contract documentation for provider-backed versus lexical-only deployments
- [x] 4.2 Add smoke-check guidance and examples that distinguish baseline process readiness from semantic rebuild readiness
- [x] 4.3 Run targeted OpenAPI, config, runtime, and embedding rebuild test coverage; then update proposal-linked docs if route or operator behavior changed during implementation
