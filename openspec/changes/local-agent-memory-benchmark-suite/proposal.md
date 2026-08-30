## Why

Stele 当前的 retrieval evaluation baseline 依赖仓库内的小型回归 fixture，能够守住生命周期、隔离和检索策略的基本契约，但还不能回答它在公开 Agent Memory 场景中的表现。缺少可版本化、可校验、可离线运行的数据集适配层，也使个人开发者难以在受限网络环境中复现实验、比较 chunk/hybrid-rank 策略并定位退化。

现在建立本地 benchmark suite，可以把公开数据集的获取、许可证和 provenance 管理，与运行阶段完全分离；先用 LoCoMo 建立长期记忆基线，再逐步覆盖 LongMemEval、profile/偏好和通用检索压力场景，为 Stele 作为 agent runtime memory provider 的 product-ready 质量门槛提供证据。

## What Changes

- 新增公开数据集 manifest 和本地缓存契约：记录名称、版本、许可证、上游 URL、commit/tag、SHA256、转换器版本、来源路径及 smoke/full split；默认不把受许可证限制的完整数据提交到 Git。
- 新增数据集适配流水线，将 LoCoMo 优先、LongMemEval 次之，以及 Multi-Session Chat/PersonaChat、HotpotQA/TimeQA/BEIR 等分层数据转换为 Stele 可消费的 conversation/session records、raw events、benchmark queries、evidence groups 和 graded qrels。
- 新增 retrieval-only benchmark 执行入口，复用既有 replay、metrics、comparison、report 和 release-policy 能力，至少输出 Recall@k、MRR、nDCG、multi-hop/evidence-group 命中、延迟和安全失败统计。
- 新增 smoke、local-full、reproducible-extended 三档运行配置；运行阶段默认离线，只读取本地缓存、固定 embedding 模型/ revision/维度/归一化方式和预缓存向量，不隐式联网下载。
- 新增 fetch 与 run 分离的 CLI/script 契约：fetch 可联网并校验 checksum、记录上游 provenance；run 在缺少数据或模型时稳定、可解释地 skip/fail，不使用远程 LLM、远程 embedding 或在线 judge 作为默认依赖。
- 新增数据集级隔离与生命周期映射，确保 project、tenant、namespace、session 边界在导入、构建 corpus、检索和报告中保持一致，并默认排除 suppressed/forgotten memory。
- 新增 benchmark 报告与 manifest/qrels 版本兼容性检查，支持同一 corpus 上多个 query、graded relevance、must-not-return 项和 evidence provenance，便于比较 chunk、lexical、semantic、hybrid-rank 策略。
- 本变更的完结门槛为：在本地 PostgreSQL + pgvector 上，使用已锁定 manifest 的 LoCoMo benchmark corpus 完整跑完至少一个非模拟 retrieval run，并保留可复现的 machine-readable report；仅完成单元测试、synthetic smoke 或报告 schema 不得视为完成。
- **不改变** 普通 `/v1/memories/search`、context assembly 或现有记忆生命周期 API；benchmark 作为独立的本地评测面提供。

## Non-goals

- 不在本变更中接入公网 benchmark 服务、在线 leaderboard 或临时免费公网依赖。
- 不把 answer generation、远程 LLM judge、在线 embedding API 设为第一阶段的成功条件；答案质量评测留待后续变更。
- 不重新设计 PostgreSQL/pgvector 存储模型，不以图数据库替代 PostgreSQL，也不把 benchmark 数据作为生产租户数据导入。
- 不承诺重新分发许可证不明确或禁止再分发的完整外部数据；仓库只保存元数据、转换代码、固定小型 smoke fixture 和获取说明。
- 不在本变更中实现新的 agent runtime provider SDK、UI 或终端用户产品逻辑。

## Capabilities

### New Capabilities

- `benchmark-dataset-manifest`: 定义公开数据集的版本、许可证、checksum、缓存布局、split 和 provenance manifest，以及 fetch/run 的离线边界。
- `agent-memory-benchmark-adapter`: 定义从 LoCoMo 等外部数据集到 Stele conversation、event、memory、query、evidence group 和 qrels 的规范化适配契约与分层数据集注册表。
- `offline-benchmark-execution`: 定义 smoke/local-full/reproducible-extended 三档本地 retrieval-only 执行、固定模型输入、无隐式联网和稳定 prerequisite 行为。
- `benchmark-qrels-and-reporting`: 定义 graded qrels、multi-hop/evidence 对齐、must-not-return、安全与延迟指标、报告格式及与现有 retrieval evaluation baseline 的复用关系。

### Modified Capabilities

- 无。现有 `hybrid-memory-retrieval`、`memory-search-contract` 和 `service-observability` 的对外要求不变；本变更只增加独立的 benchmark 适配和执行能力。

## Impact

- 新增 benchmark 数据模型、manifest/qrels 校验、数据集转换器、离线运行器、报告序列化和 CLI/script 入口，预计主要落在 `internal/retrieval`、`internal/storage/postgres`、`tool` 或 `cmd` 及 `scripts/`。
- 需要本地 PostgreSQL 18 + pgvector；embedding 可使用本地模型或预缓存向量，必须固定模型 revision、维度和归一化配置。
- 新增 `STELE_BENCHMARK_DATA_DIR`、`STELE_BENCHMARK_DATASET`、`STELE_BENCHMARK_DATA_VERSION`、`STELE_BENCHMARK_OFFLINE` 等配置，并记录数据集许可证和 checksum。
- 需要为导入、session/tenant/namespace 隔离、生命周期过滤、qrels 对齐、缺失 prerequisite、离线网络阻断和报告回归增加测试与文档。
- change completion 依赖一份真实运行报告，报告须含 dataset/version/checksum、qrels checksum、embedding/strategy profile、PostgreSQL + pgvector runtime identity、run scope、metrics、quality/safety outcome 和本地 artifact 路径。
- 相关工作流参考：`openspec-propose`、`openspec-apply`、`scripts/openspec-archive-seq.ps1`；实现应复用已完成的 retrieval evaluation baseline（`internal/retrieval/evaluation*.go`、`scripts/retrieval-evaluation.ps1`）。
