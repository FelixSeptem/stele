# Design: Manual Memory Mutation And Reclassification

## Overview

This change adds a privileged operator-facing mutation surface for governed canonical memory.

The design goal is to let operators correct, merge, and reclassify canonical memory without weakening the existing governance model:

- standard product callers still write raw events
- canonical memory remains append-only at the version level
- lifecycle-safe public reads stay unchanged
- manual corrections remain explicit, auditable, and bounded

## Goals

- add a privileged API for manual canonical memory creation
- add a bounded manual update flow for canonical memory content
- add explicit merge and reclassification workflows
- preserve stable `memory_id`, version history, and provenance lineage
- keep retrieval projections consistent after manual mutations

## Non-goals

- public free-form canonical memory CRUD
- raw event or candidate mutation
- approval queues or review workflows
- unmerge or restore semantics
- automatic embedding regeneration

## API Surface

### Privileged mutation APIs

- `POST /v1/admin/memories`
- `PATCH /v1/admin/memories/{memory_id}`
- `POST /v1/admin/memories/{memory_id}:merge`
- `POST /v1/admin/memories/{memory_id}:reclassify`

These routes live on the admin surface because they are governance overrides, not ordinary product writes.

### Public APIs remain unchanged

The existing public memory read, history, provenance, search, and context assembly APIs remain the standard SDK-facing consumption surface.

Ordinary clients should still use `POST /v1/events` for the normal write path.

## Mutation Model

### Manual create

Manual create writes canonical memory directly without synthesizing a raw event or candidate record.

The mutation should:

- create a new stable `memory_id`
- write the current canonical projection in `active` state
- create version `1`
- write a provenance record such as `manual_create_memory`

`summary` memory remains excluded from manual create because it is a derived compaction artifact rather than an operator-authored primary record.

### Manual update

Manual update is a bounded correction path, not generic patch machinery.

The mutable payload should be limited to canonical content. The following remain outside this endpoint:

- scope changes
- lifecycle state changes
- class changes

Those concerns already belong to separate surfaces:

- lifecycle changes use the existing lifecycle action APIs
- class changes use the dedicated reclassify API

Every material update must:

- keep the same `memory_id`
- append a new memory version
- update the current canonical projection atomically
- write a provenance record such as `manual_update_memory`

## Merge Model

Merge resolves duplicate canonical memories onto one surviving target identity.

The merge contract should require:

- target memory identified by route parameter
- source memory identified in the request body
- same `tenant`, `project`, and `namespace`
- same memory class
- both memories in a merge-eligible state

The merge transaction should:

- append a new version on the target memory using the merged content supplied by the operator
- preserve the target `memory_id`
- suppress the source memory so it disappears from default public reads and retrieval
- record provenance on both source and target sides for later audit

This design deliberately avoids introducing a new lifecycle state such as `merged`.

`suppressed` is sufficient for v1 because the source should remain auditable but hidden from normal recall.

## Reclassification Model

Reclassification corrects canonical class assignment when governance promoted a memory into the wrong bucket.

The initial transition set should stay intentionally narrow:

- allowed targets: `profile`, `episodic`, `procedural`
- excluded: `summary`
- excluded for now: `relation`

`summary` is excluded because it is derived and should continue to come from the compaction path.

`relation` is excluded from reclassification in this phase because it carries projection-specific parsing behavior that is better handled through manual create or update plus a later projection-focused proposal if needed.

Reclassification must:

- keep the same `memory_id`
- append a new version
- update the current canonical class atomically
- record a provenance operation such as `manual_reclassify_memory`

## Concurrency And Governance Controls

Manual mutations should not allow silent overwrite between operators.

For update, merge, and reclassification, the request contract should include an optimistic concurrency guard based on the caller's expected current version.

If the current stored version does not match the expected version, the service should reject the mutation with a conflict outcome instead of guessing operator intent.

Every manual mutation must record:

- actor
- reason
- request id
- operation
- applied timestamp

## Retrieval Projection Consistency

Manual mutation can invalidate downstream retrieval projections even if the canonical row update itself succeeds.

The consistency rules should be:

- update lexical search projection immediately
- update relation projection immediately when the memory class already relies on it
- clear or invalidate semantic embedding when content or class changes materially

The embedding rule is intentionally conservative.

Without a dedicated embedding and reindex pipeline, keeping the old vector would make semantic retrieval silently wrong.

It is safer for v1 manual mutation to fall back to lexical and relation recall than to continue serving stale vector matches.

## History And Provenance Impact

The existing history and provenance APIs should surface manual mutation operations naturally.

This proposal does not add separate history endpoints. Instead, it requires that:

- new manual versions appear in version history
- merge and reclassification operations appear in provenance lineage
- suppressed merge sources remain inspectable through privileged history paths

## Failure Modes

Expected conflict or validation failures include:

- expected version mismatch
- merge across different scopes
- merge across different classes
- reclassify into an excluded class
- mutation against a deleted memory

These should fail explicitly rather than being coerced into partial success.

## Follow-up Dependency

This design intentionally leaves vector regeneration to the next proposal.

The immediate follow-up should be an embedding and reindex pipeline that can:

- generate embeddings on demand
- backfill missing vectors
- rebuild vectors after manual mutation
- support provider rotation without rewriting memory history semantics
