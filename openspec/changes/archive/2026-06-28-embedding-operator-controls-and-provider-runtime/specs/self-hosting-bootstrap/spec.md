## ADDED Requirements

### Requirement: Bootstrap guidance covers semantic provider configuration
The service MUST document the configuration and startup expectations needed to run semantic rebuild execution with concrete embedding providers in self-hosted deployments.

#### Scenario: Operator prepares provider-backed deployment
- **WHEN** an operator reads the bootstrap documentation for a deployment that intends to use semantic rebuilds
- **THEN** the documentation specifies the required embedding route configuration, provider-specific settings, and failure modes for missing or invalid provider wiring

#### Scenario: Operator prepares lexical-only deployment
- **WHEN** an operator chooses to run without semantic providers
- **THEN** the documentation explains that lexical-plus-relation retrieval can still run while semantic rebuild execution remains intentionally inactive

### Requirement: Smoke checks distinguish semantic readiness from baseline startup
The service MUST provide a verification path that lets operators confirm whether embedding rebuild execution is truly wired, not merely whether the process is up.

#### Scenario: Provider-backed deployment verifies semantic readiness
- **WHEN** an operator runs the documented smoke check for a provider-backed deployment
- **THEN** the check confirms both baseline service readiness and the presence of actionable semantic rebuild wiring or embedding diagnostics

#### Scenario: Degraded deployment verifies expected semantic inactivity
- **WHEN** an operator runs the documented smoke check for a lexical-only deployment
- **THEN** the check can confirm that semantic rebuild execution is intentionally unavailable rather than silently misconfigured
