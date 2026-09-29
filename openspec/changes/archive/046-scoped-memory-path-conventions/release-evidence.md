# Release evidence: scoped memory path conventions

## Migration and rollback

- Migration `0019_scoped_memory_paths` was applied successfully by the real
  PostgreSQL migration runner in the disposable `pgvector/pgvector:pg18`
  test container.
- The migration is restartable (`ADD COLUMN IF NOT EXISTS`, idempotent root
  backfill, and `CREATE INDEX IF NOT EXISTS`) and creates scope-leading B-tree
  indexes for raw events, candidates, canonical memories, and governed intents.
- The down migration intentionally retains the path columns and indexes. An
  application rollback disables path selectors while preserving migrated path
  values; dropping the columns would make the prior application unable to read
  rows written by this release.
- Rollout is migration-first: the new application requires migration `0019`
  before startup. This evidence does not claim that the new application can
  query an unmigrated schema.
- Concurrent migration application passed with two runners against the same
  disposable database. The populated prior-release upgrade rehearsal also
  passed, including preservation of principal/grant, idempotency, canonical
  history, provenance, and cross-scope isolation.

## Query plan

The isolated PostgreSQL database reported the following prefix plan with
`enable_seqscan=off`:

```text
Index Scan using canonical_memories_scope_path_updated_at_idx
  Index Cond: tenant/project/namespace exact scope
  Filter: exact path OR memory_path ~~ (prefix || '/%')
```

This confirms that path matching is evaluated under the exact scope-leading
index condition. The segment-boundary predicate excludes siblings such as
`agents/researcher`.

## Conformance

- Real MCP/PostgreSQL/pgvector conformance passed after seeding records through
  ingestion, candidate admission, canonical promotion, and lifecycle action.
- Exact `agents/research` search returned only the exact-path record.
- Prefix `agents/research` search returned the exact record and its descendant,
  excluded the `agents/researcher` sibling, and excluded the suppressed hidden
  descendant.
- The same suite verified durable forget preview/apply replay, exact scope
  isolation, hidden-memory exclusion, and MCP response redaction.
- Unit and contract coverage includes normalization/bounds, fingerprints,
  pagination selectors, temporal/lifecycle visibility, context budgets, API,
  and MCP propagation.

## Verification commands

- `go test ./... -count=1 -timeout 15m` — passed.
- `go test -race ./... -timeout 20m` — passed.
- `go vet ./...` — passed.
- `openspec validate --all --strict` — 69 passed, 0 failed.
- `pwsh -File scripts/check-self-hosting-smoke-docs.ps1` — passed.
- `git diff --check` — passed.

The historical `scripts/check-quality-gate.ps1` and
`scripts/check-docs-consistency.ps1` paths referenced by the handoff are not
present in this checkout, so those two checks could not be executed.
