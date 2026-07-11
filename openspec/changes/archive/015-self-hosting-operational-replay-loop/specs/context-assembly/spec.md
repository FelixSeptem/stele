## ADDED Requirements

### Requirement: Context assembly can verify replayed insight visibility
The service SHALL allow the operator smoke loop and authorized diagnostics to verify whether replayed active insights participate in optional context assembly sections according to scope, lifecycle, quality, and budget rules.

#### Scenario: Replayed insight is active and requested
- **WHEN** replay produces or updates an active insight that matches a scoped context assembly request with insight sections enabled
- **THEN** context assembly can include that insight in `known_failures` or `experience_lessons` with citations when budget and quality policy allow it

#### Scenario: Replayed insight is hidden or out of scope
- **WHEN** replay preserves, suppresses, skips, or creates an insight outside the request scope or visible lifecycle states
- **THEN** ordinary context assembly excludes that insight even if the replay report references it

#### Scenario: Operator requests context diagnostics
- **WHEN** an authorized admin or debug path requests diagnostics for replayed insight context
- **THEN** the response can identify whether replay output was included, omitted by budget, omitted by quality policy, or hidden by lifecycle and scope rules
