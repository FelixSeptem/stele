# Proposal: Manual Memory Mutation And Reclassification

## Why

`Stele` 现在已经具备较完整的 governed memory 主线：

- raw event ingest
- candidate extraction and consolidation
- canonical memory versioning
- retrieval and context assembly
- public memory resource reads
- history / provenance 查询
- manual lifecycle actions

但对“memory 治理”来说，当前仍然缺一段关键闭环：

- operator 无法手工创建 canonical memory 来补充种子知识或运营事实。
- operator 无法修正错误 consolidation 结果，只能 suppress / expire / delete，不能直接改正内容。
- operator 无法合并重复 canonical memory，也无法把误分类 memory 调整到正确 class。
- 当前 history / provenance 虽然可查，但还没有与“人工纠偏”联动的正式 mutation contract。

如果没有这一层，`Stele` 仍然更像“可以读取和隐藏 memory 的服务”，而不是“可以持续治理 canonical memory 的服务”。

这一阶段的重点不是开放任意 public write path，而是在不削弱治理边界的前提下，增加显式、受控、可审计的人工纠偏能力。

## What Changes

本 proposal 聚焦 privileged manual mutation surface、merge / reclassify workflow，以及 mutation 之后的 history / provenance / retrieval consistency。

### 1. Privileged manual canonical memory mutation surface

建立受控的 canonical memory 人工写面，至少包括：

- `POST /v1/admin/memories`
- `PATCH /v1/admin/memories/{memory_id}`

这一层用于：

- 手工创建 canonical memory
- 修正 canonical content
- 保持 stable `memory_id`
- 以 append-only version 方式保存 material change

这一层明确不替代 raw event ingest，也不把 canonical memory 自由写入开放给普通 public caller。

### 2. Duplicate merge workflow

建立显式 merge surface，至少包括：

- `POST /v1/admin/memories/{memory_id}:merge`

merge 需要明确：

- target memory 保留 stable identifier
- source memory 必须在相同 `tenant` / `project` / `namespace` 内
- source memory 的历史和 provenance 不丢失
- source memory 在 merge 后不再出现在默认 public read / retrieval path

### 3. Manual reclassification workflow

建立显式 reclassify surface，至少包括：

- `POST /v1/admin/memories/{memory_id}:reclassify`

这一层用于修正误分类的 canonical memory，但应保持 guardrail：

- `summary` memory 仍然视为派生结果，不纳入人工 reclassify 范围
- reclassify 只允许有限、明确的 class transition
- class 变化仍然通过 append-only version 和 provenance 记录

### 4. Governance controls for manual mutation

建立人工治理写面的安全边界，至少包括：

- actor / reason / request attribution
- optimistic concurrency for conflicting operator writes
- append-only history preservation
- provenance operation taxonomy for manual create / update / merge / reclassify
- retrieval projection consistency

其中最关键的是：当 manual mutation 对 content 或 class 产生 material change 时，服务必须避免继续暴露旧的 semantic projection。

在当前系统还没有独立 embedding/reindex pipeline 的前提下，本 proposal 采用保守策略：

- lexical projection 与 relation projection 立即更新
- semantic embedding 失效时清空或标记为 stale，避免旧向量继续参与检索

## Capabilities

本 change 拆成三个 capability，分别沉淀到独立 spec：

- `manual-memory-mutation-surface`
- `memory-merge-and-reclassification`
- `manual-mutation-governance-controls`

对应目录：

- `openspec/changes/manual-memory-mutation-and-reclassification/specs/manual-memory-mutation-surface/spec.md`
- `openspec/changes/manual-memory-mutation-and-reclassification/specs/memory-merge-and-reclassification/spec.md`
- `openspec/changes/manual-memory-mutation-and-reclassification/specs/manual-mutation-governance-controls/spec.md`

## Scope Boundary

本 proposal 明确包含：

- privileged manual create of canonical memory
- privileged bounded update of canonical memory content
- duplicate canonical memory merge workflow
- bounded class reclassification workflow
- history / provenance / retrieval consistency rules for those mutations

本 proposal 明确不包含：

- raw event mutation APIs
- candidate memory mutation APIs
- generic JSON Patch or bulk mutation DSL
- approval queue or human review workflow
- unmerge or restore workflow
- embedding generation, backfill, or global reindex orchestration
- hosted control plane or dashboard UI

## Non-goals

本 proposal 不解决以下问题：

- 让普通 product caller 绕过 `POST /v1/events` 直接写 canonical memory
- 把 canonical memory 变成无约束的 CRUD 资源
- 修改已有 forgetting semantics
- 为 every mutation 自动生成新的 embedding
- 重写 retrieval ranking 或 context assembly contract
- 引入复杂 RBAC 体系

## Success Criteria

该 proposal 完成后，应满足以下条件：

- privileged caller 可以手工创建 canonical memory，而不依赖 synthetic raw event。
- privileged caller 可以以 append-only version 方式修正 canonical memory 内容。
- operator 可以把重复 memory merge 到一个 target memory，并保留 source history / provenance。
- operator 可以对允许的 memory class 执行 bounded reclassification。
- manual mutation 会记录 actor、reason、request_id、operation 和时间信息。
- 发生并发人工修改时，服务能通过 optimistic concurrency 阻止 silent overwrite。
- material mutation 不会继续暴露 stale semantic embedding 参与默认检索。
- public memory read、search 和 context assembly 的 lifecycle-safe 默认语义不会被削弱。

## Impact

### Product impact

- `Stele` 从“可以读和隐藏 memory”推进到“可以持续整理和纠偏 memory”。
- 运营或集成方首次具备正式的 canonical memory 治理写面。

### Engineering impact

- 扩展 `internal/app`、`internal/memory`、`internal/storage/postgres` 的 manual mutation contract。
- 迫使 canonical memory 的人工治理边界、version strategy、merge semantics 和 reclassification guardrail 形式化。
- 把 retrieval consistency 问题显式暴露出来，为后续 embedding pipeline proposal 提供清晰落点。

### Proposal sequencing impact

后续 proposal 应优先推进：

1. `embedding-and-reindex-pipeline`

原因很直接：本 proposal 会引入“mutation 后 vector 失效但暂不自动重建”的保守策略，下一阶段应补齐 embedding 生成、回填、重建和 provider decoupling。

## Roadmap Mapping

本 proposal 不对应原 roadmap 的单一 phase，而是把下列能力进一步产品化：

- Phase 3
  - canonical lifecycle and append-only versioning
- Phase 4
  - retrieval-safe projection consistency
- Phase 5
  - operator governance and admin surface

## Artifact References

- Plan: `docs/plans/2026-05-28-stele-v1-memory-service.md`
- Roadmap: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`
- Related archived change:
  - `openspec/changes/archive/005-memory-management-and-history-apis`
- Related specs:
  - `openspec/specs/canonical-memory-lifecycle/spec.md`
  - `openspec/specs/memory-management-surface/spec.md`
  - `openspec/specs/memory-history-and-provenance/spec.md`
  - `openspec/specs/manual-memory-lifecycle-actions/spec.md`
  - `openspec/specs/hybrid-memory-retrieval/spec.md`
