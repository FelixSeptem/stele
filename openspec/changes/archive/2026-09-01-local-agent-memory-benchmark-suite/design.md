## Context

Stele 的系统记录是 PostgreSQL + pgvector，普通记忆写入经过 event/candidate/active 等生命周期，检索同时支持 lexical、semantic 和 hybrid 路径。上一项 `retrieval-evaluation-baseline` 提供了仓库自有 fixture 的 replay、指标、策略比较、报告和安全门槛，但它不是公开语料，也没有处理数据集许可证、session 级来源、graded qrels 或离线模型前提。

本变更面向个人开发者在本地 PostgreSQL 18 + pgvector 环境运行公开 Agent Memory 数据集。网络可能受限，且外部数据的许可证通常不允许直接再分发，因此 fetch、normalize、run 必须分层，所有运行输入都要可校验、可复现、可离线读取。首个适配器是 LoCoMo；LongMemEval 和 profile/通用检索数据集以注册表和后续适配器方式演进。

## Goals / Non-Goals

**Goals:**

- 建立可注册、可版本化、可校验的 dataset manifest 和缓存布局。
- 把外部数据统一转换成 Stele 的 session/turn、raw event、query、evidence group、graded qrel 和 provenance 模型。
- 提供 smoke、local-full、reproducible-extended 三档 retrieval-only benchmark，运行默认离线且无远程 fallback。
- 复用 retrieval evaluation baseline 的 replay、metrics、comparison、report 和 release policy；让 chunk、lexical、semantic、hybrid-rank 策略可横向比较。
- 在导入和执行全过程保留 project/tenant/namespace/session 隔离，并过滤 suppressed/forgotten memory。
- 在缺数据、缺模型、checksum 不匹配或 schema 不兼容时给出稳定的 prerequisite 状态和可诊断错误。
- 在标记本 change 完成前，执行一次通过真实 PostgreSQL + pgvector retrieval 路径的完整本地 benchmark，并将最终 machine-readable report 作为可审计 artifact 留存。

**Non-Goals:**

- 不实现答案生成、LLM judge、在线 leaderboard 或公网服务集成。
- 不改变普通 memory search、context assembly、生命周期 API 或生产数据模型。
- 不把受限外部数据复制进仓库；仓库只保存 metadata、转换器、schema、文档和小型 smoke fixture。
- 不在本变更内完成 agent runtime provider SDK。

## Decisions

### 1. 分层数据集注册表

定义 `DatasetManifest`，至少包含 `name`、`version`、`license`、`upstreamURL`、`upstreamCommit`、`sha256`、`sourcePath`、`conversionVersion`、`splits`、`embeddingProfile` 和 `redistribution` 状态。注册表按层维护：Layer 0 内部 fixture，Layer 1 LoCoMo，Layer 2 LongMemEval，Layer 3 Multi-Session Chat/PersonaChat，Layer 4 HotpotQA/TimeQA/BEIR。第一阶段只把 LoCoMo 标为可运行，其他层可以是明确的 planned/metadata-only 状态。

选择 manifest 而不是把数据集信息散落在脚本参数中，是为了让报告、缓存和复现实验拥有单一身份。替代方案是直接读取 Hugging Face 数据集 API，但会引入隐式网络、版本漂移和许可证信息丢失。

### 2. Fetch 与 run 严格分离

`fetch` 命令负责联网（若用户显式允许）、下载原始归档、验证 SHA256、记录 upstream commit/tag，并生成本地 manifest lock。`run` 命令只接受已锁定的本地目录；`STELE_BENCHMARK_OFFLINE=true` 是默认值，任何缺失都返回 `prerequisite_missing` 或 `checksum_mismatch`，不得尝试远程下载。缓存布局固定为 `<data-dir>/<dataset>/<version>/{raw,normalized,embeddings,reports}`，smoke subset 与 full 数据分开。

替代方案是运行时自动下载，体验短但不可审计，也会在国内/离线环境下产生不稳定失败。

### 3. 规范化中间格式

适配器输出稳定的中间格式，而不是直接写 PostgreSQL：

- `ConversationRecord`：conversation/session id、按序 turns、时间戳、speaker 和 source offsets。
- `MemoryEventRecord`：event id、class、text、observed-at、scope、source turn、预期 lifecycle。
- `BenchmarkQuery`：query id、session/corpus scope、文本、query type、evidence groups、must-not-return ids。
- `QREL`：query id、evidence id、relevance grade、evidence role、lifecycle expectation、source provenance。

中间格式使用带版本的 JSONL/JSON schema，导入器可幂等重放；这使数据转换、PostgreSQL fixture 和离线报告可以独立测试。不得把自然语言答案当作唯一 ground truth，LoCoMo/LongMemEval 的 supporting evidence 优先映射为 qrels。

### 4. Corpus 构建与 Stele 隔离

每次 benchmark run 使用显式的 benchmark project/tenant/namespace 前缀和 run id。导入流程调用现有 raw event ingestion/consolidation 入口，或者为 replay 提供等价的受控 fixture loader；不直接绕过生命周期写 canonical memory。查询只能访问当前 run 的 scope，默认过滤 suppressed、forgotten、deleted 记录。session source id 与原始 turn offset 建立双向映射，报告可回溯证据。

选择复用现有 ingestion/retrieval 路径，是为了测量真实 provider 行为；直接构造向量表虽然快，但会漏掉 chunk、lifecycle 和隔离回归。

### 5. Retrieval-only 评测和统一报告

运行器从 manifest 读取 query/qrels，调用既有 evaluation replay 接口，按策略和 top-k 输出 Recall@k、MRR、nDCG、evidence-group/multi-hop 命中、must-not-return 违规、p50/p95 latency 和安全失败数。报告必须携带 dataset/version、manifest checksum、qrels version、embedding profile、Stele revision、run mode 和 scope。答案生成评测可以在以后消费同一 normalized corpus，但不阻塞本变更。

选择 graded qrels 和 evidence groups 同时保留 binary recall，是为了兼容传统 IR 指标并表达多跳记忆的部分相关性。只比较最终答案会掩盖检索错误，且需要额外模型。

### 6. 三档运行档位和模型可复现性

- `smoke`：仓库内小 fixture 或 LoCoMo 固定子集，秒级/分钟级，适合 CI 和安装验证。
- `local-full`：用户本地已缓存的完整数据和 embedding，默认不联网。
- `reproducible-extended`：显式锁定数据、模型 revision、embedding dimensions、归一化、chunk/rank 配置和随机种子，生成可分享的 run manifest。

Embedding profile 是 manifest 的一部分；支持本地模型和预缓存向量两种 provider，但禁止无版本的“默认模型”。没有 embedding 时可运行 lexical-only smoke（并明确标注 profile），不可静默降级为远程服务。

### 7. CLI、配置和错误契约

提供稳定的 `benchmark fetch|normalize|run|report|list` 子命令（具体挂载到现有 `tool`/`cmd` 结构），配置通过显式参数和 `STELE_BENCHMARK_DATA_DIR`、`STELE_BENCHMARK_DATASET`、`STELE_BENCHMARK_DATA_VERSION`、`STELE_BENCHMARK_OFFLINE` 注入。机器可读输出包含 `status`、`run_id`、`prerequisites`、`metrics`、`artifacts` 和 `errors`；退出码区分 success、quality_gate_failed、prerequisite_missing、invalid_manifest 和 internal_error。

## Risks / Trade-offs

- [外部许可证或 URL 变化] → manifest 强制记录许可证/上游版本/checksum；fetch 失败时不覆盖已有缓存，并在文档中要求用户自行确认再分发权。
- [数据转换丢失 supporting evidence 或时间语义] → 保留原始 offsets、转换器版本和 source provenance；为每个适配器提供 golden normalization fixture。
- [本地 embedding 模型不可得或维度不匹配] → 运行前校验 embedding profile；支持预缓存向量和稳定 prerequisite skip，禁止隐式远程 fallback。
- [公开数据集规模导致 PostgreSQL/磁盘压力] → smoke/full 分层、批量导入、可清理 run namespace 和运行前容量检查；不把 benchmark 数据混入生产租户。
- [benchmark 结果与真实 agent runtime 行为偏离] → 通过真实 ingestion、consolidation 和 retrieval pipeline 构建 corpus；报告标注 run mode 和已跳过的阶段。
- [指标被 qrels 噪声误导] → 同时输出 binary/graded/multi-hop 指标和 query coverage，报告列出 unmapped evidence 与 must-not-return 违规。

## Migration Plan

1. 先新增 manifest/schema、注册表、规范化和 smoke fixture，不改变现有 API。
2. 实现 LoCoMo fetch/normalize/run，并接入已有 evaluation baseline；本地 PostgreSQL 验证后再开放 local-full。
3. 增加 LongMemEval 和 profile/通用检索适配器，逐个通过许可证、checksum、隔离和质量门槛。
4. 文档化本地模型、缓存准备、离线运行和清理流程；CI 只运行不含外部数据的 smoke。
5. 执行锁定 manifest 的完整本地 benchmark，确认 report 记录真实 PostgreSQL + pgvector runtime、输入 checksums、scope、策略、metrics 和 quality/safety gate；没有此 artifact 不得完成或归档 change。
6. 回滚时删除新 benchmark 命令和缓存/run namespace 即可；不需要生产 schema migration，保留已生成报告供审计。

## Open Questions

- LoCoMo 和 LongMemEval 的具体上游版本、可接受许可证清单及默认 smoke 子集需要在实现开始前锁定。
- 是否把 normalized schema 放在 `internal/benchmark` 还是 `tool/benchmark`，取决于现有 CLI 组织；公共契约应保持与具体命令路径无关。
- 首个本地 embedding profile 选择哪一个可在个人机器上稳定运行的模型，需结合仓库已有 embedding provider 和 pgvector 维度约束确认。
