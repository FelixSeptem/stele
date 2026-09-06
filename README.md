# stele

Stele is a Go-based, self-hosted agent memory service. It exposes API, worker, and scheduler runtimes on top of PostgreSQL so an SDK or agent client can write governed memory, retrieve scoped context, and inspect operational state.

In addition to event ingest, search, and context assembly, the API now exposes direct canonical memory management routes for list, detail, history, provenance, and privileged lifecycle actions.

## Runtime Modes

- `api`: ingest, retrieval, context assembly, health/readiness, and admin inspection routes
- `worker`: continuous governance processing loop with lease-aware retries
- `scheduler`: periodic maintenance dispatch for embedding rebuilds, summary compaction, retention expiry, and cleanup

## Quick Start

Run the full self-hosted stack:

```bash
Copy-Item .env.example .env
# Replace the placeholder secrets in .env before sharing the environment.
docker compose up --build -d
```

Then verify:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

`/ready` confirms baseline dependency readiness only. On a fresh database, use
the bootstrap-admin key from `.env` to create a durable admin and a scoped
runtime principal before calling protected memory APIs. The complete
bootstrap-first smoke flow, provider-backed versus lexical-only embedding
configuration, migration policy, and semantic rebuild checks live in
[docs/self-hosting.md](docs/self-hosting.md).
