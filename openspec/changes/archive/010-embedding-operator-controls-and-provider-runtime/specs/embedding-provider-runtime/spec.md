## ADDED Requirements

### Requirement: Runtime registers configured embedding providers consistently across modes
The service SHALL build embedding provider registration from runtime configuration and make the resulting provider resolver available consistently to `api`, `worker`, and `scheduler` modes.

#### Scenario: Scheduler boots with configured provider support
- **WHEN** a deployment configures an embedding route whose provider implementation is available in the runtime
- **THEN** the scheduler starts with a provider resolver that can execute rebuild jobs for that configured target

#### Scenario: Worker and API share the same provider registration view
- **WHEN** multiple runtime modes start from the same effective embedding configuration
- **THEN** each mode resolves the same provider names and route targets instead of drifting through mode-specific registration logic

### Requirement: Invalid configured provider wiring fails honestly
The service MUST reject startup configurations that declare an embedding route or default provider target that cannot be resolved to a concrete provider implementation.

#### Scenario: Configured provider name is unknown
- **WHEN** runtime configuration names a provider that has no registered implementation
- **THEN** service startup fails with an explicit configuration error instead of starting with a silently broken semantic rebuild path

#### Scenario: Configured provider cannot satisfy target construction
- **WHEN** provider-specific configuration is incomplete or invalid for a declared route target
- **THEN** the runtime rejects that startup configuration before background rebuild execution begins

### Requirement: Degraded lexical-only runtime remains diagnosable
The service MUST allow startup without a configured embedding provider only when no embedding route requires one, and MUST surface that degraded state through operator-facing diagnostics.

#### Scenario: Deployment chooses lexical-only operation
- **WHEN** no default embedding target and no class-level embedding route are configured
- **THEN** the service may start without a provider resolver while clearly reporting that semantic rebuild execution is inactive

#### Scenario: Operator inspects degraded semantic runtime
- **WHEN** the runtime is operating without configured semantic providers
- **THEN** admin diagnostics and bootstrap guidance identify semantic rebuild execution as unavailable rather than presenting healthy rebuild throughput
