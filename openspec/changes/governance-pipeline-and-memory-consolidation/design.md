## Context

`Stele` 已经具备 raw event ingest、scope isolation 和 canonical memory 基础 schema，但 memory 治理主线仍然缺失。当前系统没有定义：

- raw event 何时被提炼成 candidate memory
- candidate memory 如何被评分和筛选
- canonical memory 何时被新版本 supersede
- conflicting episodic evidence 如何共存
- suppressed / forgotten memory 如何从默认读取中消失
- summary memory 如何承担“整理”和“压缩”职责

因此第二份 change 的核心不是增加更多 API，而是明确 memory 在服务内部的治理和流转生命周期。

## Goals / Non-Goals

**Goals:**

- 建立 raw event -> candidate -> active|suppressed|forgotten|deleted 的治理主线。
- 引入 worker 驱动的 extraction、consolidation、summary、forgetting 行为边界。
- 固化 canonical memory 的 append-only version 更新语义。
- 让默认读取语义从现在开始以 lifecycle state 为准。

**Non-Goals:**

- 不对外暴露 search 和 context assembly API。
- 不在本阶段做复杂模型编排或多 provider 抽象。
- 不在本阶段交付完整运维与 admin surface。

## Decisions

### Decision 1: Governance remains asynchronous and worker-driven

raw event 接入继续保持热写入，治理动作全部通过 worker 异步执行。

Rationale:

- 避免把 extraction 和 consolidation 代价推到同步写路径。
- 保留原始事件作为重放输入。
- 更符合 memory service 的治理导向，而不是 request-path 导向。

### Decision 2: Candidate memory is a first-class persisted state

candidate 不是临时内存对象，而是持久化状态。

Rationale:

- 需要保留治理判定依据和 audit trail。
- consolidation、suppression、summary 和 forgetting 都依赖 candidate 历史。
- 后续可以在不重写 raw event 的前提下重新治理 candidate。

### Decision 3: Profile and episodic memory use different consolidation rules

- `profile` / `procedural` 更偏 mutable canonical fact，采用 supersession / merge。
- `episodic` 更偏 time-bound evidence，允许冲突共存，只在可见性和摘要层整理。

Rationale:

- 这是 agent memory 与普通 chunk store 的核心差异。
- 如果把所有 memory 都按覆盖式更新处理，会破坏事件证据和时间语义。

### Decision 4: Summary is a governed memory class, not an ad hoc cache

summary 以 canonical memory 的一类存在，并带 provenance。

Rationale:

- summary 需要被引用、过滤、版本化和遗忘。
- retrieval 和 context assembly 未来会优先利用 summary，而不是把它当一次性缓存。

### Decision 5: Forgetting is modeled as visibility control first, destruction second

- `suppress` 和 `expire` 主要改变默认读取可见性。
- `delete` 才进入 payload 清除语义。

Rationale:

- 大多数治理动作需要保留审计能力。
- 真正物理删除应是较少发生、需要明确动机的动作。

## Risks / Trade-offs

- [治理规则过早复杂化] → v1 先优先 deterministic 规则和有限状态机，不上复杂模型推理。
- [candidate 持久化增加存储成本] → 接受成本，换取治理可审计和可重放。
- [summary 和 canonical memory 交叉语义不清] → 用独立 memory class 和 provenance 约束区分角色。
- [遗忘过早影响 retrieval 设计] → 正是需要提前定义默认可见性，否则 retrieval proposal 会建立在错误前提上。

## Lifecycle Model

### Stage 1: Raw event admission

- raw event append-only 存储
- 记录 scope、timestamps、metadata、provenance
- 标记进入待治理队列

### Stage 2: Candidate extraction

- worker 拉取未处理 raw event
- extractor 生成 one or more candidate memories
- candidate 记录 class、content、governance metadata、source event linkage

### Stage 3: Consolidation

- candidate 按 scope + class + dedupe key 查找 canonical context
- 决定 promote / suppress / coexist / supersede
- 对 promote/supersede 写新 canonical version
- 对 suppress 保留 audit 但默认不可见

### Stage 4: Summary and compaction

- 对 stale 或 dense episodic cluster 生成 summary memory
- summary 保留 evidence provenance
- 原 episodic material 可根据 policy 继续 active、suppress 或 expire

### Stage 5: Forgetting

- 根据 retention class / operator action / policy 执行 suppress、expire、delete
- 默认 read path 仅返回 active 与允许的 summary

## Open Questions

- v1 是否需要在 candidate record 上保存 explainability 字段，用于记录 suppression / promotion 理由。
- delete 动作在 v1 是否只删除 payload，还是连索引与 relation projection 一并清除。
- summary 生成先做 deterministic compaction，还是预留可插拔 LLM summarizer interface。
