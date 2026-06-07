# Proposal: Memory Management And History APIs

## Why

`Stele` 现在已经具备 governed memory 的核心内部能力：

- raw event ingest
- candidate extraction and consolidation
- canonical memory versioning
- summary compaction
- lifecycle-safe retrieval
- admin-only inspection for memory history

但从“可被 SDK 直接消费的 memory service”定位来看，当前对外产品面仍然缺一层正式的 memory management contract：

- client 可以通过 search 和 context assembly 间接消费 memory，但不能稳定地枚举和读取 canonical memory 资源本身。
- operator 可以通过 admin inspection 查某条 memory 的历史，但缺少正式的 memory list / get / provenance API。
- service 已经支持 `suppress` / `expire` / `delete` 语义，但还没有显式的人工 lifecycle action surface。
- SDK 集成方仍然只能“检索结果”，而不能“管理 memory 资源”。

如果不补这一层，`Stele` 更像一个有治理与检索能力的 backend，而不是一个真正对齐 Supabase 式产品定位的 memory service。

这一阶段的重点不是扩展新的 memory semantics，而是把已有 canonical memory、history、provenance、forgetting semantics，整理成稳定、可授权、可审计的 API surface。

## What Changes

本 proposal 聚焦 canonical memory 的资源化读面、history / provenance 查询，以及显式 lifecycle actions，不包含自由写入 canonical memory 的复杂 mutation 能力。

### 1. Stable memory resource contract

建立 canonical memory 的稳定资源模型，至少包括：

- stable memory identifier
- scope and class metadata
- lifecycle-safe state representation
- content payload rules
- created / updated timestamps
- pagination and filter model for list reads

### 2. Public memory read APIs

建立 SDK 可直接消费的 memory read surface，至少包括：

- `GET /v1/memories`
- `GET /v1/memories/{memory_id}`

这些读路径默认遵守现有 retrieval safety 语义：

- 默认不暴露 suppressed / forgotten / expired / deleted memory
- 默认不泄露 hidden lifecycle payload
- 仍然受 `tenant` / `project` / `namespace` scope 约束

### 3. History and provenance APIs

建立 memory 的 material history 与 evidence lineage surface，至少包括：

- `GET /v1/memories/{memory_id}/history`
- `GET /v1/memories/{memory_id}/provenance`

这一层需要明确：

- append-only version history 如何返回
- provenance 如何稳定表达 raw event / candidate / canonical lineage
- public safe view 与 privileged hidden-state inspection 的边界

其中默认的 product read path 只服务 visible memory；hidden lifecycle state 的深入检查继续走 privileged admin boundary。

### 4. Manual lifecycle actions

建立显式的 memory lifecycle management surface，至少包括：

- `POST /v1/admin/memories/{memory_id}:suppress`
- `POST /v1/admin/memories/{memory_id}:expire`
- `POST /v1/admin/memories/{memory_id}:delete`

这些 action 不引入新的 lifecycle semantics，而是把现有 forgetting semantics 暴露成受控 API，并补齐：

- actor / reason audit attribution
- idempotent action behavior
- retrieval / context assembly visibility propagation

## Capabilities

本 change 拆成三个 capability，分别沉淀到独立 spec：

- `memory-management-surface`
- `memory-history-and-provenance`
- `manual-memory-lifecycle-actions`

对应目录：

- `openspec/changes/memory-management-and-history-apis/specs/memory-management-surface/spec.md`
- `openspec/changes/memory-management-and-history-apis/specs/memory-history-and-provenance/spec.md`
- `openspec/changes/memory-management-and-history-apis/specs/manual-memory-lifecycle-actions/spec.md`

## Scope Boundary

本 proposal 明确包含：

- canonical memory list and detail APIs
- memory history and provenance APIs
- privileged manual lifecycle action APIs
- auth boundary and audit requirements for those APIs

本 proposal 明确不包含：

- raw event mutation APIs
- manual `create` / `update` / `merge` / `reclassify` of canonical memory
- bulk import/export
- embedding pipeline redesign
- dashboard or hosted control plane

## Non-goals

本 proposal 不解决以下问题：

- 让外部调用方自由创建 canonical memory
- 重写 governance pipeline 的 consolidation semantics
- 引入新的 memory class 或 lifecycle state
- 提供复杂 query language 或 ad hoc filtering DSL
- 替代现有 search / context assembly contract

## Success Criteria

该 proposal 完成后，应满足以下条件：

- SDK 可以通过稳定 API 列出和读取 canonical memory，而不必依赖 search 结果反推资源。
- memory list / get 默认不泄露 hidden lifecycle states 或 deleted payload。
- client 可以查询 memory 的 version history 和 provenance lineage，并且 public safe view 与 privileged inspection boundary 明确。
- privileged caller 可以通过正式 API 执行 `suppress`、`expire`、`delete` lifecycle actions。
- lifecycle actions 具备 actor、reason、time 等审计信息，并对重复请求保持可控的 idempotent behavior。
- retrieval 和 context assembly 的默认 lifecycle-safe 语义不会因为这些新 API 被削弱。

## Impact

### Product impact

- `Stele` 从“有 search 的 governed memory service”推进到“有资源管理面的 governed memory service”。
- SDK 不再只能消费 ranked retrieval output，也可以显式管理 canonical memory 资源。

### Engineering impact

- 补齐 `internal/memory`、`internal/app`、`internal/storage/postgres` 上的 canonical memory read / lifecycle mutation contract。
- 迫使 public memory resource representation、history representation、provenance representation 收敛为正式模型。
- 为后续 manual mutation proposal 提供稳定起点，而不是让未来直接绕过 canonical lifecycle。

### Proposal sequencing impact

后续 proposal 应基于本 change 继续推进：

1. `manual-memory-mutation-and-reclassification`
2. `embedding-and-reindex-pipeline`

## Roadmap Mapping

本 proposal 不对应原 roadmap 的单个独立 phase，而是把下列已有能力产品化：

- Phase 3
  - canonical lifecycle
  - forgetting and retention primitives
- Phase 4
  - lifecycle-safe retrieval defaults
- Phase 5
  - admin inspection foundation

## Artifact References

- Change roadmap: `docs/roadmaps/2026-06-07-memory-management-and-history-apis-roadmap.md`
- Change implementation plan: `docs/plans/2026-06-07-memory-management-and-history-apis-implementation.md`
- Plan: `docs/plans/2026-05-28-stele-v1-memory-service.md`
- Roadmap: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`
- Related archived change: `openspec/changes/archive/004-operations-admin-and-self-hosting-hardening`
- Related specs:
  - `openspec/specs/canonical-memory-lifecycle/spec.md`
  - `openspec/specs/forgetting-and-retention/spec.md`
  - `openspec/specs/admin-inspection-surface/spec.md`
  - `openspec/specs/memory-search-contract/spec.md`
