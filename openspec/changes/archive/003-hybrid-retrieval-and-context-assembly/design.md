## Context

`Stele` 已经把 raw event 治理成 canonical memory、summary 和 relation projection 的可审计状态，但外部消费者仍然缺少稳定的读路径。现阶段最关键的问题不是“再存更多数据”，而是“如何把治理后的 memory 按 scope、安全策略和 agent 消费模式正确组织出来”。

这一 change 需要回答：

- query contract 如何稳定下来，避免未来 SDK 持续重写。
- lexical、semantic、relation 三种信号如何协同，而不是彼此覆盖。
- summary memory 如何优先承担上下文压缩，而不是返回原始碎片。
- retrieval 和 context assembly 如何从第一天就继承 lifecycle visibility 语义。

## Goals / Non-Goals

**Goals:**

- 定义对外可用的 memory search 与 context assembly contract。
- 建立 PostgreSQL 上的 hybrid retrieval 读模型，兼顾 full-text 与 pgvector。
- 让 retrieval 默认遵守 scope isolation 和 forgetting semantics。
- 输出 agent-ready structured context，而不是无结构 hit list。

**Non-Goals:**

- 不在本阶段交付 admin diagnostics 或 ranking explain UI。
- 不做 learned reranker、多模型 embedding routing 或复杂 query planning。
- 不把 graph enhancement 升级为 primary persistence 或 traversal engine。

## Decisions

### Decision 1: Retrieval reads governed canonical memory, not raw events

public retrieval 只面向 canonical active memory、summary memory 和必要的 relation projection，不直接读取 raw events 作为常规召回源。

Rationale:

- raw event 缺少 consolidation、visibility 和 contradiction semantics。
- retrieval 回到 raw event 会绕开 governance 的价值。
- evidence 需要通过 citation / provenance 暴露，而不是让 raw events 直接变成 recall corpus。

### Decision 2: Hybrid retrieval is additive, not winner-takes-all

lexical recall、semantic recall 和 optional relation expansion 先各自产出候选，再统一 merge 和 rerank。

Rationale:

- lexical 对精确实体名、关键词和约束字段更敏感。
- semantic 对 paraphrase、抽象意图和 summary recall 更友好。
- relation expansion 适合 entity-centric query，但不应该主导所有查询。

### Decision 3: Lifecycle and scope filtering happen before candidate materialization

在 merge、rerank、context packing 之前，先执行 scope、class、time 和 lifecycle filter。

Rationale:

- 先过滤可以降低不必要的 ranking 干扰。
- hidden memory 不应该先进入候选再被事后丢弃。
- relation expansion 也必须建立在安全候选集之上。

### Decision 4: Context assembly is a first-class endpoint, not client-side glue

`POST /v1/context/assemble` 不是 `search` 的简单包装，而是带有 sectioning、budgeting 和 evidence preference 的独立服务能力。

Rationale:

- 多数 agent SDK 需要结构化 context，而不是自己拼装扁平结果。
- 服务侧更了解 memory class、summary 价值和 lifecycle semantics。
- 这样更符合 Supabase 式“service 暴露 API，SDK 直接接入”的产品姿态。

### Decision 5: Summary is preferred for packing, evidence remains attached

当 summary memory 可以表达同一主题或 session cluster 时，context assembly 优先放 summary，并保留必要 evidence citation，而不是把所有 episodic item 全量展开。

Rationale:

- 节约 token 预算。
- 保持 context 更稳定、可复用。
- 不丢失向下追溯证据的能力。

## Risks / Trade-offs

- [FTS + vector schema 扩展复杂度提升] → 接受复杂度，换取与 PostgreSQL-only 架构一致的 hybrid retrieval 基座。
- [relation enhancement 容易失控] → v1 只做 bounded local neighborhood expansion，不做任意图遍历。
- [context assembly 过度产品化] → 保持 section 固定且最小化，避免引入 prompt orchestration。
- [search / context API 语义重叠] → 明确 `search` 返回 ranked hits，`context/assemble` 返回 structured packing。

## Retrieval Model

### Stage 1: Query normalization

- 标准化 query text
- 解析 scope 和 optional lower-scope filters
- 解析 class filters、time window、top-k、summary / relation toggles

### Stage 2: Safe candidate source selection

- 仅选择 canonical active memory 和允许的 summary memory
- 默认排除 suppressed、forgotten、expired、deleted
- relation projection 只在明确开启时加入

### Stage 3: Parallel recall

- lexical recall：基于 PostgreSQL full-text search
- semantic recall：基于 pgvector similarity
- relation recall：基于 entity / relation projection 的局部扩展

### Stage 4: Merge and rerank

- 合并多路候选
- 去重到 memory identity 粒度
- 生成统一 score 与 component score breakdown

### Stage 5: Response shaping

- `search` 输出 ranked hits、citations、score metadata
- `context/assemble` 输出 `profile`、`recent_session`、`recent_episodes`、`relevant_summaries`、`related_entities`、`citations`

## Open Questions

- v1 是否需要在 search response 中暴露 lexical / semantic / relation 三路原始 component score。
- vector index 的 embedding 生成是否在 retrieval proposal 中先只定义 storage contract，而不要求 provider integration。
- context assembly 的 token budgeting 在 v1 是否采用近似字符预算，还是预留 tokenizer-aware interface。
