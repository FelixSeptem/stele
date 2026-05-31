## 1. Runtime And Configuration

- [x] 1.1 Initialize the Go module and top-level package layout for `cmd/stele`, `internal/app`, `internal/auth`, `internal/memory`, `internal/storage/postgres`, `internal/jobs`, and `openapi`
- [x] 1.2 Add a single service entrypoint that selects `api`, `worker`, or `scheduler` mode and verify each mode can boot through a no-op runner
- [x] 1.3 Implement environment-backed configuration loading and validation for runtime mode, HTTP listen address, PostgreSQL connection, and auth defaults

## 2. Database Bootstrap

- [x] 2.1 Add PostgreSQL bootstrap wiring and a migration runner that can execute the base schema against a clean database
- [x] 2.2 Create the initial migrations for raw events, canonical memories, memory versions, provenance links, and base indexes for scope, state, and time access
- [x] 2.3 Verify a clean database can be initialized and that startup fails clearly when the database is unreachable or misconfigured

## 3. HTTP Baseline And Scope Enforcement

- [x] 3.1 Add API mode HTTP server scaffolding with `health` and `ready` endpoints plus request ID, logging, and panic recovery middleware
- [x] 3.2 Implement API key authentication middleware for protected memory routes
- [x] 3.3 Implement request scope resolution for `project`, `tenant`, and `namespace`, and verify protected handlers receive normalized scope context

## 4. Memory Domain And Storage Contracts

- [x] 4.1 Define domain types for memory classes, memory states, scope hierarchy, raw events, canonical memories, memory versions, and provenance records
- [x] 4.2 Define repository interfaces for raw event writes, base memory reads, version access, and provenance persistence
- [x] 4.3 Implement PostgreSQL repositories for raw event creation and provenance persistence with independently verifiable persistence tests

## 5. Event Ingestion Vertical Slice

- [x] 5.1 Define the `POST /v1/events` request and response contract and reflect the endpoint in the initial OpenAPI document
- [x] 5.2 Implement validation for event type, content, timestamps, metadata shape, authentication, and resolved scope
- [x] 5.3 Implement the `POST /v1/events` handler so a successful request persists the raw event and provenance in one write flow and returns a stable `event_id`
- [x] 5.4 Verify the end-to-end ingestion path for success, invalid payloads, missing API key, and invalid scope failures
