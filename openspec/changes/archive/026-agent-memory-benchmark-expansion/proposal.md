## Why

Stele 当前的公开数据评测基础以 LoCoMo 为主，已经能够验证长期对话记忆的基本检索路径，但还不能完整证明它作为 agent runtime memory provider 在知识更新、冲突处理、用户画像、记忆操作契约和检索策略回归上的产品可用性。EvalScope 公开目录提供了 LongMemEval、BFCL-v4 memory、VTCBench 等可复用资源；现在需要将这些资源以本地、离线、可审计且分层的方式引入，形成不会把 memory、通用 IR、长上下文和 agent 工具调用混为一谈的 benchmark expansion。

## What Changes

- 新增分层 benchmark registry 和 adapter 生命周期，覆盖 LongMemEval、BFCL-v4 memory、PersonaChat/Multi-Session Chat、TimeQA、HotpotQA、C-MTEB/MTEB、BEIR、Needle-in-a-Haystack、OpenAI MRCR、LongBench-v2 和 VTCBench，并为每个数据集标注 `runnable`、`metadata-only` 或 `planned` 状态。
- 将 LongMemEval 作为 LoCoMo 之后的第二个正式 memory benchmark，规范化 session、turn、时间、问题类型、answer session、abstention、更新/冲突事实和 evidence qrels；支持 retrieval-only 评测，不要求远程 LLM judge 才能运行。
- 新增 BFCL-v4 `memory_kv`、`memory_rec_sum`、`memory_vector` 的 provider contract 回放，用于验证 agent runtime 的 memory read/write/search/update/forget 调用、参数 schema 和 scope 传递；该轨道不与 retrieval ranking 分数合并。
- 新增 profile/preference、temporal/update 和 multi-hop/evidence 专项适配器或小型合规 fixture，分别覆盖 PersonaChat/Multi-Session Chat、TimeQA 和 HotpotQA 风格用例。
- 新增通用检索回归轨道，支持选定的 C-MTEB/MTEB、BEIR 数据集，比较 lexical、semantic、hybrid、chunk、hybrid-rank 和 reranker 策略，并把结果标记为 `generic_retrieval`，不冒充 agent memory 分数。
- 新增长上下文与多模态压力轨道，支持 Needle、MRCR、LongBench-v2 和 VTCBench 的受控子集；默认只做压力和退化分析，VTCBench 优先使用 text mode，视觉模式需要显式能力声明。
- 统一所有外部数据的 manifest、版本、上游 revision、许可证、SHA256、转换版本、split、qrels checksum、缓存路径和 redistribution 状态；完整受限数据只允许进入本地缓存，不提交到 Git。
- 扩展离线运行和报告契约，支持按 benchmark family 单独运行、设置本地模型/预缓存向量、固定随机种子和容量预算，并在报告中区分 memory retrieval、provider contract、profile/temporal/multi-hop、generic IR 和 stress 结果。
- 增加 prerequisite、license、checksum、scope isolation、lifecycle filtering、must-not-return、evidence coverage、延迟和清理能力的验证，确保 benchmark 数据不会进入生产租户空间。
- **不改变**普通 memory search、context assembly、记忆生命周期 API 或 PostgreSQL 作为系统记录的架构；新增能力只扩展独立的本地 benchmark surface。

## Non-goals

- 不把 EvalScope leaderboard、在线 benchmark 服务、远程 embedding、远程 LLM judge 或公网搜索设为默认依赖。
- 不在本变更中实现新的 agent runtime SDK；BFCL 只验证 provider contract 和调用形状，不替代 runtime 本身的实现。
- 不把 LongBench-v2、Needle、MRCR、VTCBench 或 OneMillion-Bench 的模型答案分数直接作为 memory provider 质量，也不把不同 family 的分数强行合成为单一总分。
- 不在仓库中再分发许可证不明确或禁止再分发的完整数据集，不覆盖用户已有缓存，不隐式联网下载。
- 不将通用 MTEB/BEIR 语料或 benchmark run 写入生产 project、tenant、namespace，亦不修改生产数据模型。
- 不要求第一阶段完成所有压力数据集或多模态图像处理；每个扩展层必须先通过本地、checksum-locked、可重复的最小回归，再提升为 full run。

## Capabilities

### New Capabilities

- `memory-benchmark-suite-expansion`: 定义 LongMemEval 及 profile/preference 数据集的 manifest、adapter、session/evidence/qrels 映射和本地运行边界。
- `runtime-memory-provider-contract`: 定义 BFCL-v4 memory 子集的离线调用回放、memory operation schema、scope 传递和 contract 指标。
- `specialized-memory-retrieval-evaluation`: 定义 PersonaChat/Multi-Session Chat、TimeQA、HotpotQA 风格专项回归，以及 profile、temporal、update/conflict、multi-hop evidence 指标。
- `generic-retrieval-strategy-regression`: 定义 C-MTEB/MTEB、BEIR 选定子集的 lexical/semantic/hybrid/chunk/hybrid-rank/reranker 比较和独立报告身份。
- `long-context-stress-evaluation`: 定义 Needle、MRCR、LongBench-v2 和 VTCBench 受控子集的压力运行、容量预算、text/multimodal 能力声明和退化报告。
- `benchmark-family-reporting-and-governance`: 定义跨数据集 family 的报告 schema、许可证与 provenance、checksum/qrels 兼容性、质量/安全门槛、artifact 保留和清理策略。

### Modified Capabilities

无。本变更新增独立 benchmark 能力，不改变现有 memory ingestion、retrieval、lifecycle 或 runtime API 的需求契约。

## Impact

- 主要影响 `internal/benchmark`、`internal/retrieval`、本地 PostgreSQL/pgvector replay、benchmark CLI 和 `docs/`；预计新增多个 adapter、registry、fixture、qrels、manifest 和 report 类型。
- 需要继续支持 PostgreSQL 18 + pgvector、本地 embedding 模型或预缓存向量，并为不同数据集记录固定维度、revision、归一化和 tokenizer 信息。
- 需要扩展当前 benchmark cache 布局和运行参数，保持 fetch/normalize/run/report 分离，并提供 family 级 `list`、`fetch`、`normalize`、`run`、`report` 入口。
- 需要新增离线集成测试，覆盖真实 PG + pgvector retrieval、重复导入、跨 run/tenant/namespace 泄漏、生命周期排除、qrels 对齐、must-not-return、缺失依赖和容量清理。
- product-ready 完成门槛为：LongMemEval 至少完成一次非模拟、checksum-locked、本地 PostgreSQL + pgvector retrieval run；BFCL memory 三个子集可离线回放；profile、temporal、multi-hop 至少各有一组稳定回归；报告保留完整 provenance 和质量/安全结果。
- 相关工作流：先使用 `/opsx:apply` 实施本变更，完成后使用仓库规定的 `scripts/openspec-archive-seq.ps1` 归档，不手动重命名 archive 目录。
