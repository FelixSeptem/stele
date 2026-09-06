## ADDED Requirements

### Requirement: Upgrade documentation exposes migration integrity state
The self-hosting guide SHALL document migration status inspection before
deployment traffic is accepted, describe the integrity fields in human and JSON
output, and state the forward-remediation or verified-restore path for dirty,
divergent, and incompatible history.

#### Scenario: Operator upgrades a supported database
- **WHEN** an operator follows the documented upgrade procedure
- **THEN** the guide directs them to run `stele migrate status`, apply only
forward migrations under the documented policy, verify a current checksummed
state, and then start the runtime modes

#### Scenario: Operator sees migration integrity failure
- **WHEN** status reports dirty, divergent, or incompatible migration history
- **THEN** the guide prohibits editing historical migration files or automatic
down migration and directs the operator to preserve a backup and choose a
forward remediation or verified restore procedure
