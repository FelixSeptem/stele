## ADDED Requirements

### Requirement: Intent APIs are OpenAPI-first and adapter-neutral
The public API contract MUST define exact-scope intent submission, idempotency conflict behavior, status/history inspection, and bounded response categories before any MCP or other adapter exposes the operation.

#### Scenario: Client submits an intent through the public API
- **WHEN** a client sends a valid intent with an idempotency key
- **THEN** the API returns the stable intent identity, accepted/pending status, and bounded next-action metadata described by OpenAPI

#### Scenario: Adapter calls an unsupported mutation path
- **WHEN** an adapter attempts to bypass the public intent contract or invoke direct canonical mutation
- **THEN** the service rejects the request and leaves canonical state unchanged
