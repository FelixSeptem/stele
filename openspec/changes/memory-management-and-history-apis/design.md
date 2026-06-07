# Design: Memory Management And History APIs

## Overview

This change turns governed canonical memory into a first-class API resource instead of exposing memory only indirectly through retrieval results and admin diagnostics.

The design keeps a strict split between:

- public lifecycle-safe memory reads
- privileged lifecycle mutation and hidden-state inspection

It does **not** introduce free-form manual canonical memory authoring.

## Goals

- expose canonical memory as a stable SDK-facing resource
- expose append-only history and provenance lineage in a formal contract
- expose manual lifecycle actions as privileged, auditable API operations
- preserve existing retrieval and context assembly safety defaults

## Non-goals

- manual create or update of canonical memory payload
- merge or reclassify workflows
- governance semantic redesign
- dashboard or organization-level RBAC

## API Surface

### Public read APIs

- `GET /v1/memories`
- `GET /v1/memories/{memory_id}`
- `GET /v1/memories/{memory_id}/history`
- `GET /v1/memories/{memory_id}/provenance`

Public reads remain scope-bound and lifecycle-safe:

- hidden memory is not returned by default
- deleted payload is not leaked
- standard callers only see visible canonical memory

### Privileged APIs

- `POST /v1/admin/memories/{memory_id}:suppress`
- `POST /v1/admin/memories/{memory_id}:expire`
- `POST /v1/admin/memories/{memory_id}:delete`

Privileged inspection of hidden-state diagnostics continues to live on the admin surface and should align with the response models introduced by this proposal.

## Resource Model

Canonical memory should be represented as a stable resource view, not a retrieval hit:

- `id`
- `scope`
- `class`
- `state`
- `content`
- `created_at`
- `updated_at`

The list API should support:

- class filters
- time-window filters
- bounded pagination

The resource model should not expose ranking or retrieval-only score fields.

## History Model

History is append-only and derived from `memory_versions` plus lifecycle transitions.

The history contract should:

- return versions in a stable order
- preserve the distinction between current resource state and historical versions
- make deleted memory history inspectable without implying payload resurrection

## Provenance Model

Provenance should express lineage across:

- raw events
- candidate memories
- canonical promotion or summary creation
- lifecycle actions where applicable

The API should return stable references and timestamps rather than internal storage-specific details that are not meaningful to SDK consumers.

## Auth Boundary

Public memory reads use the standard scoped API surface.

Manual lifecycle actions use the admin boundary because they change memory visibility and retention semantics. This keeps read access and governance control separated even if both are later wrapped by the same SDK.

## Audit And Idempotency

Lifecycle actions must capture:

- action type
- actor
- reason
- applied timestamp

Repeated lifecycle requests must be safe to retry and should converge to one stable post-action outcome instead of creating contradictory durable mutations.

## Storage And Service Impact

Expected implementation touchpoints:

- `internal/app` for new handlers and auth wiring
- `internal/memory` for resource and history contract shaping
- `internal/storage/postgres` for list/detail/history/provenance and lifecycle action reads or writes
- `openapi` for memory resource, provenance, and lifecycle action schemas

Existing search and context assembly code should remain behaviorally compatible and continue to exclude hidden memory by default.
