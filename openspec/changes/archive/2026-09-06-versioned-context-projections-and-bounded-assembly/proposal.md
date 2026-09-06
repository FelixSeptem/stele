## Why

Stele can assemble a bounded context from live retrieval, but it does not yet
persist a versioned, rebuildable explanation of why context material is visible
for an agent session. P2 requires a durable hierarchy that can provide stable
always-visible and session context without allowing shortcuts around canonical
memory versioning, provenance, lifecycle, or scope boundaries.

## What Changes

- Add a PostgreSQL-backed, append-only `context_projection` model with the
  projection kinds `always_visible`, `session`, `retrieval`, and
  `archival_history`; every item records a canonical-memory version or raw-event
  evidence reference and its scope-safe derivation metadata.
- Define class-aware projection policy: eligible profile material can populate
  `always_visible`; summary material can populate bounded session context;
  episodic, procedural, and relation material remain retrieval/on-demand; raw
  history remains evidence rather than canonical context.
- Add deterministic projection materialization and rebuild behavior that is
  idempotent, retains append-only history, and never writes canonical memory.
- Extend context assembly to consume authorized visible projections before its
  existing live retrieval sections while enforcing deterministic ordering,
  character/token budgets, lifecycle filtering, and redacted citations.
- Publish bounded projection diagnostics that identify inclusion or omission
  category without exposing hidden content, foreign scope identifiers, raw
  event payloads, credentials, or database details.
- Add PostgreSQL integration, isolation, lifecycle, rebuildability, and budget
  tests plus self-hosting documentation for inspecting and rebuilding context
  projections.

## Non-goals

- Do not add public memory-intent writes (`remember`, `update`, `forget`,
  `contradiction`, or `feedback`); those are a later P2 change.
- Do not add reflection triggers, reflection runs, automated extraction, review
  workflows, compaction execution, model invocation, SDKs, UI, or an external
  agent runtime adapter.
- Do not overwrite canonical memory or alter its lifecycle/version contract.
- Do not change chunking, hybrid fusion, reranking, or default retrieval
  ranking; live retrieval remains the fallback for on-demand material.
- Do not introduce a system of record other than PostgreSQL.

## Capabilities

### New Capabilities

- `versioned-context-projections`: Durable, scoped, versioned, and rebuildable
  context projection records with canonical-memory/raw-event evidence links and
  class-aware policy.

### Modified Capabilities

- `context-assembly`: Assemble context from lifecycle-visible authorized
  projections under deterministic character/token budgets with redacted
  projection citations and diagnostics.
- `canonical-memory-lifecycle`: Ensure context projections observe lifecycle
  visibility and never mutate canonical memory in place.
- `memory-history-and-provenance`: Expose projection derivation as an auditable
  relationship to canonical version and raw-event evidence.
- `self-hosting-bootstrap`: Document operator inspection and rebuild of scoped
  context projections as part of the self-hosted context workflow.

## Impact

- Storage: a forward PostgreSQL migration and repository methods for projection
  headers, items, source references, policy/rebuild metadata, and scoped reads.
- Domain/runtime: new projection types and a materializer wired through existing
  canonical memory, lifecycle, provenance, retrieval, context assembly, API,
  worker, and scheduler boundaries as necessary.
- Public contract: context responses gain additive projection-backed section and
  citation/diagnostic metadata; existing request behavior remains compatible.
- Verification: focused Go and real PostgreSQL + pgvector tests prove no
  cross-scope or hidden-lifecycle disclosure, no canonical mutation, stable
  rebuilds, and bounded assembly.
- Artifact references: use `openspec instructions apply --change
  versioned-context-projections-and-bounded-assembly --json`, run `go test
  ./internal/memory ./internal/retrieval ./internal/storage/postgres
  ./internal/app -count=1`, and validate with `openspec validate
  versioned-context-projections-and-bounded-assembly --strict`.
