# Proposal: Governance Pipeline And Memory Consolidation

## Why

第一份 change 已经完成 foundation、scope isolation、base schema 和 `POST /v1/events` 的事件接入闭环，但当前系统仍然只是一个“可审计的 raw event store”，还不是一个真正可用的 memory service。

要让 `Stele` 对齐计划中的“治理优先”定位，下一步必须把 raw events 转换为 governed memory，并明确 memory 的组织方式和生命周期流转：

- 新建：从 raw event 提炼 candidate memory。
- 更新：把 candidate 合并为 canonical memory 的新版本。
- 检索前过滤：让 suppressed / forgotten / expired memory 在默认读路径上不可见。
- 遗忘：支持 suppression、expiry、delete 三类治理动作。
- 整理：支持 summary / compaction，把稠密 episodic event 压缩为更稳定的记忆形态。

这一阶段直接对应 roadmap 中的 `Phase 3: Governance Pipeline`。如果跳过这一步直接做检索，会导致 retrieval 面向未治理的原始事件工作，缺少冲突处理、版本演进和遗忘语义，最终只能得到“RAG store”，而不是 agent memory service。

## What Changes

本 proposal 聚焦 memory governance 的最小可用闭环，不引入公开检索 API，也不处理部署和运维增强。

### 1. Candidate extraction pipeline

建立 worker 驱动的治理输入输出边界，至少包括：

- raw event 的待处理状态与提取游标
- 从 raw event 到 candidate memory 的提取 contract
- 可插拔的 extractor interface
- 初始 deterministic extractor 路径

### 2. Candidate governance metadata

为 candidate memory 增加治理必需字段，至少包括：

- confidence
- importance
- freshness
- sensitivity
- mutability
- retention class
- source raw event linkage

### 3. Consolidation and canonical promotion

建立 candidate 到 canonical memory 的合并和晋升路径，至少包括：

- profile memory 的 supersession 规则
- episodic memory 的 coexistence 规则
- candidate dedupe / suppression 规则
- canonical memory version 追加写入
- provenance 扩展到 candidate 与 canonical mutation

### 4. Summary and compaction path

建立记忆整理路径，至少包括：

- session 或 topic cluster summary 记录
- stale episodic cluster compaction
- summary 到原始 evidence 的 provenance links

### 5. Forgetting and lifecycle controls

补齐遗忘和生命周期动作，至少包括：

- suppress
- expire
- delete
- retention policy evaluation
- 默认读取排除 suppressed / forgotten / expired

## Capabilities

本 change 拆成四个 capability，分别沉淀到独立 spec：

- `memory-governance-pipeline`
- `canonical-memory-lifecycle`
- `summary-compaction`
- `forgetting-and-retention`

对应目录：

- `openspec/changes/governance-pipeline-and-memory-consolidation/specs/memory-governance-pipeline/spec.md`
- `openspec/changes/governance-pipeline-and-memory-consolidation/specs/canonical-memory-lifecycle/spec.md`
- `openspec/changes/governance-pipeline-and-memory-consolidation/specs/summary-compaction/spec.md`
- `openspec/changes/governance-pipeline-and-memory-consolidation/specs/forgetting-and-retention/spec.md`

## Scope Boundary

本 proposal 明确不包含以下内容：

- 公共搜索 API：`POST /v1/memories/search`
- 公共上下文组装 API：`POST /v1/context/assemble`
- semantic retrieval / lexical retrieval / reranking
- relation-enhanced retrieval
- 完整 admin inspection surface
- 完整 observability hardening
- LLM provider 接入的复杂策略编排

本 proposal 的重点是 memory 被“治理成什么”，而不是“如何对外检索出来”。

## Non-goals

本 proposal 不解决以下问题：

- 混合检索和上下文组装
- graph 关系检索增强
- 多租户成员管理和 RBAC
- 生产级运维面板或部署编排
- 高级模型路由和 prompt orchestration

## Success Criteria

该 proposal 完成后，应满足以下条件：

- worker 能从 raw events 生成 candidate memories。
- candidate memory 带有最小治理元数据并能持久化。
- consolidation 能把可接受 candidate 晋升为 canonical active memory 或 suppress 为不可见状态。
- canonical memory 的 material change 以 append-only version 形式保存。
- summary memory 可从 episodic cluster 生成并保留 provenance。
- forgetting / expiry / delete 动作能改变默认读取可见性。
- 后续 retrieval proposal 可以直接建立在 governed canonical memory 之上，而不是回到 raw event。

## Impact

### Product impact

- `Stele` 从“事件接收服务”演进为“可治理的 memory service”。
- memory 生命周期首次具备可解释的流转语义。

### Engineering impact

- 引入 `internal/governance` 与 `internal/policy` 的核心边界。
- 明确 worker 对 canonical memory 和 provenance 的写入职责。
- 为后续 retrieval 规定默认可见性与 lifecycle 过滤语义。

### Proposal sequencing impact

后续 proposal 应基于本 change 的 governed memory 语义继续推进：

- retrieval proposal 读取 canonical active / summary memory，并默认排除 hidden lifecycle state。
- operations proposal 观察 worker backlog、consolidation lag、forgetting actions 和 retention outcomes。

## Roadmap Mapping

本 proposal 对应 roadmap 中的以下任务：

- Phase 3
  - Task 3.1 Candidate extraction pipeline contract
  - Task 3.2 Candidate persistence and scoring
  - Task 3.3 Consolidation rules
  - Task 3.4 Summary generation and compaction
  - Task 3.5 Forgetting and retention actions

## Follow-up Proposals

建议按以下顺序继续补 proposal：

1. `hybrid-retrieval-and-context-assembly`
2. `operations-admin-and-self-hosting-hardening`

## Artifact References

- Plan: `docs/plans/2026-05-28-stele-v1-memory-service.md`
- Roadmap: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`
- Prior archived change: `openspec/changes/archive/001-bootstrap-foundation-and-event-ingestion`
- OpenSpec workflow: `openspec status --change "governance-pipeline-and-memory-consolidation"`
- Apply command: `/opsx:apply`
