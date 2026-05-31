# AGENTS.md

## Repo Scope

- Build `Stele` as a Go-based, self-hosted agent memory service.
- This repository owns the service only.
- Do not add SDK, UI, or end-user product logic unless explicitly requested.

## Architecture Defaults

- PostgreSQL is the only system of record.
- Use `pgvector` for semantic retrieval.
- Use PostgreSQL full-text search for lexical retrieval.
- Treat graph memory as an enhancement layer, not the primary persistence model.
- Prefer three runtime modes: `api`, `worker`, and `scheduler`.

## Memory Rules

- Keep memory classes explicit: `profile`, `episodic`, `procedural`, `summary`, `relation`.
- Keep lifecycle explicit: `event -> candidate -> active|suppressed|forgotten|deleted`.
- Do not overwrite canonical memory in place.
- Preserve versions, provenance, and audit history.
- Favor hot write plus async consolidation over heavy synchronous write paths.

## Isolation Rules

- Enforce `project`, `tenant`, and `namespace` boundaries in every API, query, and background job.
- Default retrieval must never return suppressed or forgotten memory unless an explicit admin or debug path requests it.

## Change Rules

- Keep public APIs OpenAPI-first and self-host friendly.
- Add tests for ingestion, consolidation, retrieval, forgetting, and isolation whenever behavior changes.
- Update `docs/` when governance, lifecycle, or public API contracts change.

## Library Guidance

- Before implementing core infrastructure, check `pkg.go.dev` for mature Go libraries that can be adopted instead of building from scratch.
- Pay special attention to existing libraries for web frameworks, routing, configuration, logging, PostgreSQL access, migrations, background jobs, validation, and OpenAPI tooling.
- Prefer well-maintained libraries with clear documentation, active usage, and stable APIs when they reduce boilerplate or operational risk.
