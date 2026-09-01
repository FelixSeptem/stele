## Context

Stele 已有本地 benchmark 基础设施和 LoCoMo adapter，但公开数据支持仍集中在单一长期对话数据集。下一阶段需要在个人开发者可本地运行的约束下扩展到 LongMemEval、BFCL-v4 memory、profile/preference、temporal、multi-hop、通用 IR 和长上下文压力数据。数据规模、许可证、任务语义和评测方式差异很大，直接做一个统一 runner 会丢失 memory lifecycle、evidence provenance 和 provider contract 的边界。

设计约束包括：PostgreSQL 18 + pgvector 是 system of record；benchmark 默认离线；受限数据只进本地缓存；每次 run 必须有独立 scope；普通 memory API 不改变；报告必须能区分 retrieval、runtime contract、generic IR 和 stress 结果。

## Goals / Non-Goals

**Goals:**

- 建立按 benchmark family 分层的 registry、manifest、adapter 和 runner 契约。
- 首先完成 LongMemEval 的真实 retrieval-only adapter，并保留 oracle、update/conflict、preference、temporal、abstention 语义。
- 以 BFCL-v4 memory 子集验证 agent runtime 到 provider 的调用契约，而非将函数调用准确率混入 ranking 指标。
- 提供 profile/preference、temporal/update、multi-hop/evidence、generic IR 和长上下文压力的独立回归轨道。
- 保持 fetch/normalize/run/report 分离、可离线执行、可清理、可审计和可复现。

**Non-Goals:**

- 不实现新的 agent runtime SDK、在线 leaderboard、远程 judge 或公网搜索依赖。
- 不把所有数据集一次性标为 runnable；每个 adapter 必须通过许可证、checksum、隔离和质量门槛。
- 不修改生产 memory schema、普通 search API 或生命周期语义。
- 不把模型答案质量、通用 IR 或长上下文分数解释为 Stele memory provider 的单一分数。

## Decisions

### 1. 按 family 分层而非单一 adapter

registry 将数据集分为 `memory`、`provider_contract`、`specialized_retrieval`、`generic_retrieval` 和 `stress`。每个 family 有独立的 normalized schema、指标和报告 namespace。替代方案是统一映射到 EvalScope QA 输入，实施较快但会丢失 evidence groups、lifecycle 和 scope 约束，因此不采用。

### 2. LongMemEval 使用 evidence/session qrels

LongMemEval 的 `answer_session_ids`、问题类型、问题日期、abstention 和更新关系被转换为 session-aware qrels；答案字符串只作为可选 answer-evaluation 输入。retrieval-only run 不需要 LLM judge。`oracle` 用于上限对照，`retrieval_log` 只能标记为官方日志对照，不能冒充 Stele 实际检索。

### 3. BFCL memory 独立为 contract replay

`memory_kv`、`memory_rec_sum` 和 `memory_vector` 通过离线固定样本回放，验证 operation name、参数 schema、scope、空结果和拒绝行为。结果使用 operation accuracy、scope safety 和 malformed-call rate，不与 Recall@k/MRR 合并。

### 4. 外部数据统一 manifest 与缓存锁

manifest 必须记录 dataset/version/license/upstream revision/SHA256/conversion version/splits/qrels checksum/embedding profile/redistribution。缓存固定为 `<data-dir>/<dataset>/<version>/{raw,normalized,embeddings,reports}`。run 只读取已锁定缓存，缺失、漂移和 checksum 错误稳定失败。

### 5. 通用 IR 和压力数据只作为独立轨道

C-MTEB/MTEB、BEIR 用于 lexical/semantic/hybrid/chunk/hybrid-rank/reranker 回归；Needle、MRCR、LongBench-v2 和 VTCBench 用于长度、位置、多目标和 text/multimodal 压力。它们的报告必须带 family 身份，不能改变 memory product gate。

### 6. 渐进式 runnable 状态

第一阶段将 LongMemEval 标为 runnable；BFCL memory 和 profile/temporal/multi-hop 先以受控离线 fixture 或 adapter spike 验证；generic IR 和 stress 数据在容量、许可证和本地运行成本确认后逐个开放。任何数据集在没有 manifest、golden normalization、隔离测试和最小报告前保持 metadata-only。

## Risks / Trade-offs

- [LongMemEval 历史和本地 embedding 规模较大] → 默认提供 `s`/小 split、容量预检和预缓存向量，full/m 只在显式预算下运行。
- [答案 ground truth 与 evidence 不完全对齐] → 保留 answer session、turn provenance、graded qrels 和 unmapped evidence 计数，禁止静默丢弃。
- [受限许可证或 ModelScope 镜像漂移] → fetch 不覆盖旧缓存，强制 checksum/revision/license 审核并在报告中保留来源。
- [不同 family 指标不可比] → 报告按 family 分组，质量门槛使用各自指标和明确的非合并摘要。
- [BFCL 或 VTCBench 引入额外 Python/图像依赖] → 默认只支持离线最小子集，依赖缺失返回 prerequisite 状态，不影响核心 retrieval run。
- [benchmark 数据污染生产租户] → 使用 benchmark 专用 project/tenant/namespace 前缀，导入和查询都执行 scope predicates，并在清理后验证无残留。

## Migration Plan

1. 新增 family registry、manifest 字段、缓存布局和报告 schema，不改变现有 LoCoMo 行为。
2. 实现 LongMemEval normalize、session/evidence qrels、retrieval-only runner 和本地 PostgreSQL + pgvector smoke/full split。
3. 实现 BFCL memory contract replay 及 profile、temporal、multi-hop 的最小合规 fixture，再开放对应 adapter。
4. 以选定 C-MTEB/BEIR 数据集接入通用检索策略比较，加入容量和清理控制。
5. 增加 Needle/MRCR/LongBench-v2/VTCBench 的受控压力入口，默认不阻塞 memory gate。
6. 运行真实 checksum-locked LongMemEval，保留 machine-readable report；失败时只移除 benchmark cache/run namespace，不触碰生产数据。

## Open Questions

- LongMemEval 上游文件的最终 revision、许可证文本和默认本地 split 需在实现前锁定。
- BFCL-v4 Python 依赖是否作为可选工具链，还是转换为仓库拥有的 contract fixture，需结合个人开发者安装成本决定。
- 首批 C-MTEB/BEIR 子集以及 LongMemEval 的本地 embedding profile 需要基于磁盘、内存和中文覆盖率实测确认。
