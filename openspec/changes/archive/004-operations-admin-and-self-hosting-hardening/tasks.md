## 1. Worker Reliability

- [x] 1.1 Replace one-shot worker execution with a stable worker loop, lease handling, and bounded retry semantics
- [x] 1.2 Add idempotency protections for repeated governance and maintenance job execution with repository-backed verification

## 2. Scheduler And Maintenance

- [x] 2.1 Define scheduler contracts and cadences for retention, expiry, compaction, and cleanup triggers
- [x] 2.2 Implement scheduler runtime dispatch that runs maintenance jobs independently of request traffic

## 3. Observability

- [x] 3.1 Add structured logs for `api`, `worker`, and `scheduler` modes with request or job correlation identifiers
- [x] 3.2 Add metrics and tracing hook points for ingest, governance, retrieval, forgetting, and backlog state
- [x] 3.3 Verify readiness, failure, and backlog signals expose actionable operator diagnostics

## 4. Admin Inspection Surface

- [x] 4.1 Define an admin-only route namespace and auth boundary separate from public API routes
- [x] 4.2 Add admin endpoints for job status, worker backlog, scheduler state, and maintenance health inspection
- [x] 4.3 Add admin inspection endpoints for memory history, lifecycle visibility, and provenance diagnostics

## 5. Self-Hosting Assets

- [x] 5.1 Add a production-oriented `Dockerfile` for packaging the `stele` runtime across `api`, `worker`, and `scheduler` modes
- [x] 5.2 Add a `docker-compose.yml` that wires `api`, `worker`, `scheduler`, and PostgreSQL for local or self-hosted startup
- [x] 5.3 Document bootstrap prerequisites, PostgreSQL extensions, config variables, runtime modes, container startup, and smoke-check steps
