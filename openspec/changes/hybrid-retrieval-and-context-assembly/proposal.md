# Proposal: Hybrid Retrieval And Context Assembly

## Why

`Stele` 现在已经完成 raw event ingest、canonical memory foundation，以及 governance pipeline 的 candidate 提炼、consolidation、summary、retention 与 forgetting 闭环。系统已经具备“把 memory 治理成什么”的内部语义，但还没有“如何安全、稳定、可组合地把 memory 提供给 agent”的公开读取能力。

如果没有 retrieval 和 context assembly：

- canonical memory 无法以公开 API 被 SDK 或 agent 使用。
- suppression / expiry / delete 的生命周期语义无法在对外读路径上得到真正验证。
- summary memory 无法承担压缩上下文和降低 token 开销的职责。
- 当前服务仍然更像“内部治理引擎”，而不是对齐 Supabase 式产品定位的 memory service。

这一阶段直接对应 roadmap 中的 `Phase 4: Retrieval And Context Assembly`。它的目标不是回到 raw event 做普通 RAG，而是基于 governed canonical memory、summary 和 relation projection，提供 scope-aware、policy-safe、agent-ready 的读取面。

## What Changes

本 proposal 聚焦 retrieval contract、hybrid recall、public search API、context assembly API，以及可选的 relation-enhanced retrieval，不扩展运维和部署增强。

### 1. Search contract and orchestration

建立 retrieval 的稳定查询和结果模型，至少包括：

- query text
- scope filters
- class filters
- time window
- top-k
- summary inclusion
- relation inclusion
- score breakdown
- citations

### 2. Hybrid lexical and semantic retrieval

建立混合召回基础能力，至少包括：

- PostgreSQL full-text lexical retrieval
- pgvector semantic retrieval
- canonical memory 与 summary memory 的统一候选集
- lexical + semantic merge and rerank

### 3. Scope-aware filtering and lifecycle-safe reads

建立默认安全读取路径，至少包括：

- `tenant`、`project`、`namespace` 约束
- lower-scope optional filters
- class and time filters
- 默认排除 suppressed / forgotten / expired / deleted
- relation expansion 也必须遵守同样的 scope 和 policy 约束

### 4. Public retrieval APIs

暴露面向 SDK / agent 的读取 API，至少包括：

- `POST /v1/memories/search`
- `POST /v1/context/assemble`

### 5. Relation-enhanced retrieval

建立 bounded graph enhancement 路径，至少包括：

- entity / relation projection reads
- entity-centric neighborhood expansion
- optional relation-assisted rerank signals

## Capabilities

本 change 拆成四个 capability，分别沉淀到独立 spec：

- `memory-search-contract`
- `hybrid-memory-retrieval`
- `context-assembly`
- `relation-enhanced-retrieval`

对应目录：

- `openspec/changes/hybrid-retrieval-and-context-assembly/specs/memory-search-contract/spec.md`
- `openspec/changes/hybrid-retrieval-and-context-assembly/specs/hybrid-memory-retrieval/spec.md`
- `openspec/changes/hybrid-retrieval-and-context-assembly/specs/context-assembly/spec.md`
- `openspec/changes/hybrid-retrieval-and-context-assembly/specs/relation-enhanced-retrieval/spec.md`

## Scope Boundary

本 proposal 明确不包含以下内容：

- worker / scheduler 运维硬化
- admin inspection endpoints
- deploy manifests 或 bootstrap docs 扩展
- hosted control plane 或 dashboard
- 复杂 embedding provider 编排
- 面向最终应用的 SDK 实现

## Non-goals

本 proposal 不解决以下问题：

- 生产级 observability 与 backlog inspection
- 完整 admin / operator surface
- 高级 query personalization 或 learned ranking
- 跨数据库检索支持
- 复杂 graph traversal engine 或 graph-first persistence

## Success Criteria

该 proposal 完成后，应满足以下条件：

- retrieval 能基于 canonical active memory 与 summary memory 返回混合排序结果。
- lexical 与 semantic recall 可以合并为单一 ranked output。
- 默认搜索和上下文组装不会泄露 suppressed、forgotten、expired 或 deleted memory。
- `POST /v1/memories/search` 能返回带 citation 和 score metadata 的结果。
- `POST /v1/context/assemble` 能返回面向 agent 的结构化上下文分区，而不是扁平 chunk 列表。
- relation projection 可以作为可选增强，提高 entity-centric query 的 recall。
- 后续 operations proposal 可以围绕 retrieval latency、ranking quality 和 context packing 继续增强，而不需要重写 API 语义。

## Impact

### Product impact

- `Stele` 首次具备可直接接入 SDK 的公开读取面。
- memory service 从“治理完成”走向“可消费”。

### Engineering impact

- 引入 `internal/retrieval` 的核心边界。
- 扩展 PostgreSQL schema 以承载 FTS、vector 和 relation-assisted retrieval 的读模型。
- 为 OpenAPI 增加 search 与 context assembly 的正式 contract。

### Proposal sequencing impact

后续 proposal 应基于本 change 的读取语义继续推进：

- operations proposal 关注 retrieval latency、ranking diagnostics、worker / scheduler inspection 和 self-hosting hardening。
- admin proposal 如需新增 debug read path，必须显式绕过默认 lifecycle visibility，并与 public retrieval surface 分离。

## Roadmap Mapping

本 proposal 对应 roadmap 中的以下任务：

- Phase 4
  - Task 4.1 Search query model
  - Task 4.2 Lexical and semantic retrieval base
  - Task 4.3 Scope-aware filtering and policy enforcement
  - Task 4.4 `POST /v1/memories/search`
  - Task 4.5 `POST /v1/context/assemble`
  - Task 4.6 Relation-enhanced retrieval

## Follow-up Proposals

建议按以下顺序继续补 proposal：

1. `operations-admin-and-self-hosting-hardening`

## Artifact References

- Plan: `docs/plans/2026-05-28-stele-v1-memory-service.md`
- Roadmap: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`
- Prior active change: `openspec/changes/governance-pipeline-and-memory-consolidation`
- OpenSpec workflow: `openspec status --change "hybrid-retrieval-and-context-assembly"`
- Apply command: `/opsx:apply`
