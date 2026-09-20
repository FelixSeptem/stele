## ADDED Requirements

### Requirement: OpenAPI publishes additive temporal selectors
The authoritative OpenAPI document SHALL describe additive `as_of` and
`valid_during` search selectors, their mutual validation rules, explicit temporal
plan requirement, selected-version metadata, and stable temporal error categories.

#### Scenario: Consumer discovers temporal search contract
- **WHEN** an integration reads the published OpenAPI document
- **THEN** it can determine how to request current or historical validity without
  relying on undocumented fields or changing the existing recorded-time filters

#### Scenario: Invalid temporal selector is documented
- **WHEN** a caller supplies malformed or conflicting temporal selectors
- **THEN** the API returns the documented bounded validation error without
  exposing hidden history
