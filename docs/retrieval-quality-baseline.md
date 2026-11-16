# 检索质量基线

首次由仓库自有的检索回放使用 `retrieval-fixture-v1` 夹具在 PostgreSQL 18 与
pgvector 上执行。

| 字段 | 值 |
| --- | --- |
| 夹具版本 | `retrieval-fixture-v1` |
| 表示版本 | `canonical-v1` |
| 排名版本 | `baseline-v1` |
| 兼容的嵌入修订 | `deterministic-v1`（没有向量时语义通道不启用） |
| 策略版本 | `quality-policy-v1` |
| 回放用例 | 13 |
| 安全失败 | 0 |
| 数据库 | PostgreSQL 18 / pgvector |

该回放完成了全部用例，并验证了仅追加的原始事件、规范版本、来源、作用域与生命周期行为。初始夹具查询有意包含语义改写；因此当前仅词法的回放为该基线记录了一个零候选池。这是分块前、融合前实现的明确测量结果，也是后续查询理解、分块表示与混合排名变更的参照点。

使用以下命令运行相同的自有栈检查：

```powershell
$env:STELE_TEST_RETRIEVAL_EVALUATION_DSN = '<owned-pg18-test-dsn>'
$env:STELE_TEST_RETRIEVAL_EVALUATION_OWNED = 'true'
pwsh -File scripts/retrieval-evaluation.ps1
```

该命令要求一个明确自有的可丢弃数据库与所有权标记，并且绝不会回退到 `STELE_POSTGRES_DSN` 或其他环境中的运维数据库。

## 分块表示影子化

分层分块表示是构建在原始事件与 canonical-memory 版本之上的派生 PostgreSQL 投影。它由源类型/ID/版本、分块策略、渲染器以及确定性的空白 token 计数器版本标识。原始事件与规范记忆保持不可变；分块行可重建且仅追加。

分块物化与检索消费被独立控制：

- `default_off` 保持 canonical-memory 检索路径不变；
- `shadow` 在授权评估路径上评估分块候选，并且只输出有界诊断信息；
- `active` 允许分块派生的证据参与，同时保留规范父身份与引用。

每次分块读取或父级/相邻扩展都会重新校验确切的 `tenant/project/namespace` 作用域与活动生命周期状态。隐藏、过期或外部的源会失败即关闭。父级与相邻证据受数量以及字符/token 预算约束，并且绝不会扩大会话或用户边界。

分块集成检查请使用明确自有的 PostgreSQL + pgvector DSN：

```powershell
$env:STELE_TEST_POSTGRES_CHUNK_DSN = '<owned-test-dsn>'
go test ./internal/storage/postgres -run MemoryChunkPostgres -count=1
```

禁用分块物化或消费即可回滚；无需破坏性迁移或重写规范记忆。派生分块的保留与删除必须遵循源生命周期以及运维方的 PostgreSQL 备份/恢复策略。

## 稳定候选融合

稳定混合融合契约将词法、语义、关系与活动分块检索视为有界的已排名通道。默认策略标识为 `rrf:rrf-v1`。它使用显式的排名常数与通道权重；对于可见候选 `c`，契约为：

```text
fused(c) = sum(weight(channel) / (rank_constant + rank))
```

`rank` 是通道的从 1 开始的排名，来自某通道的每个有界且通过校验的贡献都会向该候选的规范记忆身份累加一次。默认参数是 `rrf:rrf-v1` 的刻意组成部分：

- 排名常数 Rank constant: `60`
- 词法通道权重 Lexical channel weight: `1`
- 语义通道权重 Semantic channel weight: `1`
- 关系通道权重 Relation channel weight: `0.8`
- 分块通道权重 Chunk channel weight: `0.8`
- 每通道候选上限 Per-channel candidate bound: `50`
- 候选总量上限 Total candidate bound: `200`

候选在融合之前会经过作用域、生命周期、类别与谱系校验，然后由一个规范记忆身份表示。最终输出顺序是确定性的：融合得分降序、记忆类别策略优先级、源时间戳降序，最后是规范记忆 ID 升序。该顺序在聚合之后应用，因此来自多条召回路径的重复父级不会产生两个公开命中。

词法检索仍是必需的规范路径。语义、关系与授权分块检索是可选的：空或不可用的可选通道不贡献任何候选，结果回退到其余通过校验的通道。无效的策略定义以及无效的作用域、生命周期、谱系或候选上限输入会失败即关闭；它们绝不会扩大候选池，也不会让隐藏证据影响排名。

`normalized_weighted:normalized-weighted-v1` 是一个显式的离线对比实验。它在应用声明的权重之前先对各通道的有界得分做归一化，并且绝不会被隐式选中。融合策略标识会写入评估元数据，因此 RRF 与实验结果保持可比，且回滚不需要数据迁移。

评估与回放诊断仅在授权的本地、CI 或管理路径上可用。它们会为可见的已评估结果保留有界的策略标识、通道/排名状态、可用性与最终处置类别。它们绝不包含原始候选池、原始提供方得分、隐藏标识符、作用域值、DSN、凭据或提供方错误文本。

Prometheus 计数器 `stele_retrieval_fusion_total` 仅输出低基数的 `strategy`、`version`、`channel`、`availability`、`candidate_count` 与 `outcome` 标签。候选数量分桶为 `0`、`1_10`、`11_50` 与 `51_plus`；标签绝不包含查询文本、记忆标识符、策略 ID 或作用域值。有界的 `stele_ranking_rollout_total` 计数器将成功的恢复记录为 `reason_code=rollback_restored`。

运行聚焦的契约覆盖测试：

```powershell
go test ./internal/retrieval ./internal/telemetry -count=1
go test ./internal/app -run RankingRollout -count=1
```

回放命令要求上文所述的、明确自有的可丢弃 `STELE_TEST_RETRIEVAL_EVALUATION_DSN`。它不得由 `STELE_POSTGRES_DSN` 提供或替代。

## 身份去重与多样性感知的上下文打包

检索流水线在作用域/生命周期校验与稳定融合之后、在任何多样性或预算选择之前执行身份与已校验谱系去重：

```text
scope + lifecycle validation
  -> bounded recall and stable fusion
  -> canonical/source-event/parent-lineage deduplication
  -> optional semantic clustering and diversity selection
  -> existing section, citation, and token/character budget packing
```

身份等价由规范记忆 ID、源事件 ID 或已校验的父记忆谱系确立。稳定融合顺序中的第一个候选是代表项；引用以确定性方式合并，并且在每个代表项上有界。无效、隐藏、过期、被抑制、被遗忘、已删除或外部的候选会在分组之前被丢弃，并且绝不会抑制可见候选。在没有获批多样性策略时，这种按身份去重后的顺序就是默认行为。

### 版本化的多样性策略生命周期

多样性策略按名称与版本进行版本化，并且只针对确切的 `tenant/project/namespace` 作用域解析。其有界参数包括语义阈值、MMR（或等价方法）的相关性/多样性权重、候选上限、成对比较预算、引用上限，以及记忆类别、源会话、实体与时间切片的覆盖权重。语义聚类仅由具有兼容活动嵌入修订的候选构成。如果嵌入、修订或可选计算不可用，选择器会报告有界的 `semantic_unavailable` 可用性类别，并回退到仅身份选择，而不会扩大召回或查询额外元数据。

发布（rollout）沿用现有的排名发布治理，并具有显式的 `diagnostics_only`/`shadow`、`active_for_scope`、`disabled` 与回滚状态。仅诊断与影子策略绝不会改变普通搜索或上下文输出。激活需要常规的预演、归因、证据阈值与无阻塞项门禁；缺失、格式错误、被禁用、隐藏或外部作用域的策略等同于没有策略。按节选择在资格判定与摘要偏好之后、现有预算之前运行，因此公开节名称、投影、引用与调用方预算保持不变。

授权的评估或管理诊断只能暴露策略标识、聚合处置（`selected`、`duplicate`、`omitted_by_diversity`、`omitted_by_budget`、`invalid` 或 `semantic_unavailable`）、有界计数与可用性类别。普通检索与上下文响应绝不暴露聚类成员关系、相似度值、候选池、隐藏内容、外部标识符、作用域值或提供方错误。

评估报告包含多样性策略名称/版本与聚合处置，以及重复率、受保护召回、证据覆盖、候选池规模与有界延迟。作用域或生命周期泄漏始终是硬失败；受保护召回、多跳覆盖、预算或延迟的回退会阻止激活。回滚会禁用活动选择并恢复先前获批的基线，而不重写规范记忆、原始事件、分块、来源或融合状态；遥测记录 `reason_code=rollback_restored`。

真实栈回放是有意选择加入的。它要求 `STELE_TEST_RETRIEVAL_EVALUATION_DSN` 中有一个明确自有的、可丢弃的 PostgreSQL + pgvector DSN。当该变量缺失时，评估器打印 `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` 并以退出码 `2` 结束；这是受控的非通过跳过，而不是成功的评估。评估器绝不会回退到 `STELE_POSTGRES_DSN` 或任何环境中的运维数据库。运行结束后移除夹具作用域，并确保报告不含凭据与原始候选内容。

### 有界的查询理解发布

查询分析将调用方的原始查询保留为首要的必需信号。归一化、别名、提示与分解都是确定性的，并受版本化的 `query-analysis-v1` 与 `query-analysis-limits-v1` 契约约束；派生信号复制确切的请求作用域、生命周期可见性、记忆类别与时间窗口约束。分析器错误、输出格式错误、超时或策略不符合条件都会回退到仅原始查询的检索。

查询分析发布策略使用现有的精确作用域排名治理。`diagnostics_only` 与 `dry_run`/shadow 可以收集有界的聚合证据，但不能改变普通排名；只有精确作用域的 `active_for_scope` 策略才允许派生信号进入公共融合流水线。被禁用、过期、外部、格式错误与已回滚的策略都解析为仅原始查询行为。

授权诊断仅暴露策略与限制版本、发布处置、归一化/时间状态、有界的提示/信号/子查询/候选计数、回退类别以及已耗预算。它们绝不包含查询文本、归一化文本、子查询、得分、计划、标识符或作用域值。普通搜索与上下文响应保持其现有模式，除非显式诊断路径请求，否则不包含这些字段。

在执行真实栈前置条件以及原始查询与已分析查询的对比并通过（使用明确自有的可丢弃 PostgreSQL + pgvector DSN）之前，主动分解仍不符合条件。没有该 DSN 时，稳定结果为 `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED`；这是非通过跳过，不得视为发布证据。

## 查询自适应的检索规划

查询自适应规划器是建立在现有词法、语义、关系与分块通道之上的、与提供方无关的确定性策略层。计划由 schema、planner 与策略版本标识，并记录兼容的分析、融合、排名与渲染器身份。支持的族为 `exact_lookup`、`semantic`、`temporal`、`entity_relation`、`multi_hop`、`procedural` 与 `general`；分类优先级与所有通道顺序都经过规范化，以便回放。

每个请求获得一个共享的硬性包络，覆盖总候选、每通道候选、已耗延迟、上下文条目、重排器余量与轮次。每通道份额可以在该包络内由有界的 `simple`、`moderate` 或 `complex` 查询复杂度类别重新分配，该类别仅由通过校验的分析计数推导。复杂度分配必须覆盖每个已声明通道，使每个通道保持在每通道硬限制之内，并保持声明的总候选包络，因此它绝不能放大工作量或放宽服务硬限制。类别配额与上下文优先级在作用域/生命周期校验之后应用。账本预留重排器余量，并重新分配未使用的通道容量，同时不允许任何通道或轮次超过聚合上限。规划器的重排器资格仅是一个提示：分别受治理的重排器策略与预留的账本余量都是必需的。

证据评估只使用有界的聚合计数与处置。如果首轮不足，符合条件的计划最多可以用原始查询、确切作用域、生命周期过滤器与剩余预算再运行一次后续轮次。失败、格式错误、不可用或超限的规划器输入会回退到获批基线；不存在递归检索循环，也没有规划器生成的最终答案。普通搜索与上下文响应形态保持不变。

规划器发布是精确作用域且依赖兼容的。`diagnostics_only` 与 `shadow` 在不改变结果的情况下收集授权、低基数的证据。只有未过期且依赖身份匹配的 `active_for_scope` 策略才能影响检索。被禁用、外部、格式错误、过期或已回滚的策略会失败即关闭至基线。没有明确自有的 PostgreSQL + pgvector 评估 DSN 时，评估器保持为稳定的 `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` 非通过状态，且不允许主动发布。

自有的规划器运行器不会把扩展可用性或词法代理当作 pgvector 证据。它会创建并激活夹具自有的 `planner-unit-x3-v1` 向量修订，要求生产语义查询返回带正得分的预期语义记忆，然后要求规划器回放候选保留语义通道贡献。向量播种、激活、直接语义检索或回放归因中的任何失败都会使本次运行失败。
