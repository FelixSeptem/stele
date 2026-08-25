## ADDED Requirements

### Requirement: Assurance and conformance runtime settings are validated
The service MUST validate runtime configuration for assurance cadence, conformance cadence, operational proof checks, retention windows, and alert delivery before serving API traffic or starting worker or scheduler execution.

#### Scenario: Assurance cadence uses default fallback
- **WHEN** assurance or conformance cadence settings are omitted
- **THEN** the service uses the existing maintenance interval as the default cadence without changing the supported `api`, `worker`, and `scheduler` runtime modes

#### Scenario: Retention settings are invalid
- **WHEN** assurance or conformance retention windows are negative, unparsable, or shorter than minimum safe bounds
- **THEN** startup fails with an actionable configuration error before cleanup jobs can run

#### Scenario: Operational proof settings are invalid
- **WHEN** capacity/load thresholds or backup/restore proof freshness windows are invalid, unbounded, or internally inconsistent
- **THEN** startup fails with an actionable configuration error rather than treating missing proof as healthy

#### Scenario: Webhook settings are unsafe
- **WHEN** alert delivery is configured for `webhook` with an unsupported scheme, unsafe local or metadata network target, missing explicit local override for insecure endpoints, rejected header, invalid timeout, or oversized payload limit
- **THEN** startup fails or the adapter remains disabled with an actionable configuration error before outbound delivery is attempted
