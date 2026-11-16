# 本地 agent 记忆基准

Stele 的基准套件评估检索证据，而不是生成的答案。它被设计为在开发者机器上使用本地数据运行，并且不需要公共服务、远程嵌入 API 或 LLM 裁判。

## 数据集策略

仓库只包含一个合成的、仓库自有的 LoCoMo 形态冒烟夹具。其清单校验和基于规范的 LF 文本计算，因此 Git 检出的行尾不会改变冒烟验证结果。它不分发 LoCoMo、LongMemEval、Multi-Session Chat、PersonaChat、HotpotQA、TimeQA 或 BEIR 的完整语料。

## 基准扩展轨道

本地运行器将基准族分开维护。`memory` 覆盖 LoCoMo 与 LongMemEval 检索，`provider_contract` 回放 BFCL 形态的记忆操作，`specialized_retrieval` 覆盖 profile、时间与多跳夹具，`generic_retrieval` 保留用于 C-MTEB/MTEB/BEIR 策略对比，`stress` 报告受控的长上下文或多模态退化。族标签是每份清单/报告的一部分，来自不同族的结果不会被聚合为单一质量分。

默认保持离线。将受限数据集准备到本地缓存中，并附带校验和锁定的清单，然后使用 `benchmark fetch`、`benchmark normalize` 与 `benchmark run`。任何命令都不会静默下载模型或替换为其他数据集。`benchmark contract --fixture <path>`、`benchmark bfcl-v4 --source <path> --tenant <tenant> --project <project> --namespace <namespace>`、`benchmark specialized` 与 `benchmark stress` 为扩展轨道提供离线检查。使用 `benchmark clean --dataset <name> --version <version> --run-id <id>` 删除运行作用域产物，同时默认保留报告。

为可复现性，请将清单 SHA256、上游修订、转换版本、归一化校验和、qrels 校验和、嵌入与策略配置、PostgreSQL/pgvector 版本、运行作用域与报告路径一起保留。缺失或漂移的输入属于前置条件失败；运行器绝不会通过隐式下载来修复它们。受限源文件保留在 Git 之外的缓存中。

在获取完整外部数据集之前，请审阅其当前许可证与上游条款。创建清单，锁定数据集名称、版本、上游 URL、上游修订、SHA256、转换版本、划分与嵌入配置。将下载的文件保存在本地缓存下；除非明确允许再分发，否则不要将其加入 Git。

## 本地缓存

为本地完整运行设置以下变量：

```powershell
$env:STELE_BENCHMARK_DATA_DIR = "D:\stele-benchmark-data"
$env:STELE_BENCHMARK_DATASET = "locomo"
$env:STELE_BENCHMARK_DATA_VERSION = "<locked-version>"
$env:STELE_BENCHMARK_OFFLINE = "true"
```

确定性缓存布局为：

```text
<data-dir>/<dataset>/<version>/
  raw/
  normalized/
  embeddings/
  reports/
```

`fetch` 是唯一允许使用显式启用的网络连接的操作。它必须在替换原始数据之前校验清单 SHA256。`run` 默认离线，并且绝不会隐式下载数据、模型、向量或裁判。

## 冒烟验证

仓库自有的冒烟检查不需要 PostgreSQL、模型下载或网络请求：

```powershell
$env:STELE_BENCHMARK_DATA_DIR = "$PWD\.tmp\benchmark"
go run ./cmd/stele benchmark list
go run ./cmd/stele benchmark run-smoke
```

`run-smoke` 将 JSON 报告写入本地缓存。它记录数据集版本与校验和、模式、离线状态、查询指标、延迟与安全失败。夹具的候选生成器有意保持确定性；它验证适配器、缓存、qrels、报告与安全契约，而不是真实检索质量。

## 嵌入配置

语义与混合模式需要一个明确的本地模型或预缓存向量配置。在清单中锁定模型名称、修订、维度、归一化与向量源。维度或配置不匹配会以 `prerequisite_missing` 拒绝准入。

仅在显式选择时才允许仅词法冒烟。运行器不会静默地将语义或混合运行变成词法模式。

## 运行模式

- `smoke`：仓库自有的有界夹具；适用于 CI 与安装验证。
- `local-full`：完整的本地缓存数据集与向量；没有网络回退。
- `reproducible-extended`：完整数据加上锁定的模型/配置、策略/分块设置、qrels 与随机种子元数据。

## BFCL V4 Provider 契约一致性

`benchmark bfcl-v4` 针对 `memory_kv`、`memory_rec_sum` 或 `memory_vector` 之一，评估本地提供的、校验和锁定的 BFCL V4 记忆源信封。该信封携带经许可证审阅的上游源本地提取，以及其原始源摘要、上游修订、转换版本、前置条件/快照摘要，以及 Stele 可评估的归一化操作记录。请将原始上游源保留在 Git 之外。

该命令只校验 Provider 契约兼容性。它不会调用模型或 BFCL agent harness，不会调用远程工具，不会执行上游裁判，也不会产生官方 BFCL 分数。不受支持的操作与不完整的前置状态仍是显式的非通过覆盖结果；它们绝不会被静默映射到另一个操作。源提供的字段不能扩大声明的项目/租户/命名空间作用域或覆盖生命周期拒绝规则。确切作用域通过命令标志独立提供，并且每个源用例都必须与之完全匹配。

使用空的 `source_checksum` 准备信封，然后计算其规范锁定，并在运行一致性检查之前插入返回值：

```powershell
go run ./cmd/stele benchmark lock-bfcl-v4 `
  --source D:\stele-benchmark-data\bfcl-v4\memory-kv-envelope.json

go run ./cmd/stele benchmark bfcl-v4 `
  --source D:\stele-benchmark-data\bfcl-v4\memory-kv-envelope.json `
  --tenant benchmark `
  --project bfcl `
  --namespace local
```

本地信封形态有意保持狭窄：

```json
{
  "manifest": {
    "schema_version": "bfcl-v4-memory-source-v1",
    "dataset": "bfcl-v4-memory",
    "version": "v4.0",
    "upstream_revision": "<7-to-64-character-hexadecimal-revision>",
    "source_sha256": "<sha256-of-reviewed-source>",
    "source_checksum": "<output-of-lock-bfcl-v4>",
    "conversion_version": "bfcl-v4-provider-v1",
    "subset": "memory_kv"
  },
  "prerequisites": [{"id": "snapshot-1", "snapshot_sha256": "<sha256>"}],
  "cases": [{
    "id": "<local-case-id>",
    "operation": "get",
    "arguments": {},
    "expected": {},
    "scope": {"tenant": "benchmark", "project": "bfcl", "namespace": "local"},
    "prerequisite_id": "snapshot-1"
  }]
}
```

生成的报告保留族标识、源修订/摘要、前置条件摘要、聚合的受支持/不受支持/不完整覆盖、契约指标、状态与清理验证。它有意省略原始源记录、作用域、记忆标识符、隐藏候选与单个操作标识。在将该轨道用作外部输入证据之前，需要一次保留的、非合成的、经许可证审阅的本地源运行；仓库测试只验证适配器与安全契约。

## 实验性适配器与操作

当本地清单与原始/归一化缓存存在时，LongMemEval `s` 可运行。适配器接受清理后的 ModelScope haystack 格式，并保留问题日期、问题类型、答案会话 ID、弃答、更新/冲突状态与源会话来源。`m` 与 `oracle` 仍是显式选择的子集；省略 `--subset` 不会静默选择其一。

使用锁定本地缓存的示例：

```powershell
$env:STELE_POSTGRES_DSN = "postgres://stele:stele@127.0.0.1:55432/stele?sslmode=disable"
go run ./cmd/stele benchmark run `
  --data-dir D:\stele-benchmark-data `
  --dataset longmemeval `
  --version modelscope-cleaned-f180315e-s `
  --mode local-full `
  --strategy lexical `
  --subset s
```

该命令在归一化之后离线，并写入一个运行作用域的 `reports/<run-id>/retrieval.json` 产物。成功的真实运行会记录 `synthetic_fixture: false`、PostgreSQL/pgvector 标识、归一化与 qrels 校验和、作用域、安全结果与产物路径。基准自有的数据库作用域在回放后清理；保留下来的报告与清单文件默认不被删除。

在导入大型本地语料之前，先运行容量预检并选择显式的事件批大小。拒绝会以 `capacity_refused` 与有界诊断报告；不会尝试任何 PostgreSQL 导入。运行作用域的归一化与嵌入产物可以用 `CleanupBenchmarkRun` 删除，同时保留报告产物以供审计。

CI 冒烟工作流只使用已签入的夹具。它不会下载外部语料、模型、向量或裁判，也不需要 PostgreSQL。

<a id="longitudinal-task-series-evaluation"></a>
## 纵向任务序列评估

纵向评估器在相同的有序任务阶段上比较至少两个显式声明的记忆条件。每个阶段提供一个外部判定的 `succeeded`、`partial`、`failed` 或 `inconclusive` 判定；Stele 绝不推断结果。`inconclusive` 被排除在效用分母之外，并使该序列不完整。效用使用锁定的指标映射 `succeeded=1`、`partial=0.5`、`failed=0`。过期/噪声伤害只统计检索实际返回的带注释证据别名。来源覆盖只统计具有已校验引用、确切作用域与可见生命周期的已返回别名。作用域/生命周期失败会独立地使安全门禁失败，即使任务效用更高。条件差异是观察性的，不是因果主张。

准备一个包含 `manifest` 与 `corpora` 字段的本地 JSON 源包。清单包含 `schema_version`、`metric_version`、确切 `scope`、一个 `baseline: true` 条件加上对比条件、有序的 `stages`、每个阶段的 `query_id`、确切 `scope`、外部 `verdict`，以及带可选 `stale` / `noisy` 标签的 `evidence` 别名。每个条件的归一化语料必须恰好包含这些查询；声明的证据别名必须是由 qrels 关联到该查询的活动、同作用域事件。`synthetic_fixture` 标识源是否为合成。请将任务内容与源语料保存在本地；保留的聚合报告排除任务/查询文本、作用域、别名、标识符与候选细节。

对于 `synthetic_fixture: false`，源包还需要一个可运行的 `dataset_manifest`，包含上游 URL 与修订、许可证、原始源 SHA-256、转换版本、划分标识与可运行支持声明。运行器只在聚合报告中记录其摘要/名称/版本。没有该来源锁定的非合成声明会被拒绝。

首先计算确定性的源与 qrel 锁定：

```powershell
go run ./cmd/stele benchmark lock-task-series --source D:\stele-benchmark-data\task-series\bundle.json
```

将返回的 `qrels_checksum` 与 `source_checksum` 插入源包的清单，然后针对隔离的 PostgreSQL + pgvector 数据库运行：

```powershell
$env:STELE_POSTGRES_DSN = "postgres://stele:stele@127.0.0.1:55432/stele?sslmode=disable"
go run ./cmd/stele benchmark run-task-series `
  --source D:\stele-benchmark-data\task-series\bundle.json `
  --report D:\stele-benchmark-data\task-series\reports\longitudinal.json `
  --tenant benchmark `
  --project task-series `
  --namespace local
```

该命令没有网络回退，要求确切作用域的源包与 PostgreSQL DSN，通过常规夹具播种器与检索回放路径在每个基准自有作用域中分别运行每个条件，并写入权限为 0600 的聚合报告。`synthetic_fixture: true`、不完整结果、校验和不匹配或任何安全失败都无法满足纵向完成证据。非合成完成报告还必须包含 `cleanup_verified: true`；清理失败即为运行失败，任何保留的报告都不能用作验收证据。不改变任何公开 API 或生产检索策略。

<a id="locked-v3-staged-replay"></a>
## 锁定的 v3 分阶段回放

`task-series-v3` / `task-series-metrics-v3` 是纵向评估器的可选择加入、维护者自有的离线扩展。它声明恰好五个有序条件（`no_memory`、`source_records`、`organized`、`verified_corrected`、`resident`），阶段与留出标志相同，外加 SHA-256 的模型/运行时间/判据标识、1-32 个不同的非负种子，以及相等的独立条目/UTF-8 字节/空白 token/引用预算（范围 1-64、1-65536、1-16384、1-64，以及 1-60000 ms 的延迟上限）。这些是本地实验锁定，不是证明或验收阈值；外部判据必须在数据收集之前定义任务与成本验收。

每个条件最多声明 64 个源窗口，绑定参考、不可变的正面源版本、SHA-256 字面内容、可披露元数据、首个可用阶段与可选的首个撤回阶段。运行器只将阶段可见的事件、qrels 与查询注释物理播种到全新的自有 PostgreSQL 作用域中；对话会被拒绝。披露、生命周期、确切作用域、引用谱系与全部四个预算都会针对返回的行进行检查。后续的基线回放会对比脱敏的证据标识。强制使用规范 JSON 键；重复键、别名、未来/已撤回证据、缺失标识、受限元数据或 v1/v2 源包上的 v3 字段会以有界的 `invalid_manifest` / `checksum_mismatch` 错误失败即关闭。无记忆条件没有源、qrels 或预期证据。每个条件最多允许 64 个阶段与 64 个源，因此最多运行 320 次阶段搜索加 64 次基线回放。

锁定的合成夹具是 `internal/benchmark/testdata/locked-longitudinal-task-series-v3.json`。每当清单或语料发生变化时，用 `benchmark lock-task-series` 重新计算两个锁定；v1/v2 夹具保留其现有校验和与默认值，只有已记录的 v2 分母更正会改变其报告：重复错误与维护覆盖现在除以未被取代的阶段（包括 inconclusive 效用阶段），因此覆盖不能超过 1。请重新生成旧的 v2 报告产物，而不是将其与旧分母比较。V3 仅由 schema/指标标识选择；不涉及迁移、数据库 schema 变更、公开路由、授权角色或生产检索策略。回滚就是停止离线运行或改用 v1/v2 源包。

V3 报告将模型 token 与工具调用观察同效用、重复错误和维护分开，具有独立的已观察计数、总量、可空平均值与覆盖。实测服务延迟、作用域/生命周期/披露资格、未来证据排除与经过测试的基线回放都是独立门禁。缺失的观察或门禁是不确定的，绝不是零观察或通过；效用提升不能覆盖失败的门禁。序列化的 v3 报告只暴露实验/输入/观察摘要、门禁计数与有界聚合。报告绝不携带 ID、查询/内容文本、执行者、作用域或原始判据。在有 Docker 的检出中运行可丢弃的自有校验器：

```powershell
pwsh -NoProfile -File scripts/stele-longitudinal-v3-verify.ps1 -Diagnostic
```

它会创建一个随机自有的 PostgreSQL + pgvector 容器，回放全部五个条件与基线，重启容器（重新绑定其临时宿主端口），重复回放，验证确切的自有行/容器清理，并输出有界的合成一致性报告。它绝不采用运维 DSN。合成输入保持标注状态，外部结果保持不确定：这既不表明普遍任务收益，也不表明因果改进，并且不授权任何激活。

## 常驻上下文对比观察

内部常驻评估器通过将现有 profile 放置策略与一个证据过滤候选进行比较，来补充任务序列回放。其支持/效用观察绑定确切的源版本与内容哈希；后续条件观察必须绑定实际的源/输出摘要，并发生在选择以及所有支持/效用阶段之后。选择效用不能作为对比结果复用。合格的支持仍是外部观察，而不是服务发现的真相或程序性验证权威。

任务成功、重复错误、过期证据伤害、维护成本、工具调用与模型 token 都由外部提供。缺失字段保持未观察；观察到的 false/零值会被保留。只有相互观察到的字段才会产生候选减基线的差值。没有两个任务判定时，状态保持 `inconclusive`；即使完整的配对也被标注为 `observational`，不含因果或激活主张。在 [self-hosting](self-hosting.md#evidence-gated-resident-context-evaluation) 中描述的自有校验器检查合成服务一致性与持久投影谱系。它不能替代非合成的纵向证据、受控重复试验、延迟门禁，或独立的隔离/生命周期/披露/回滚检查。

## 故障排查

| 状态 | 含义 | 操作 |
| --- | --- | --- |
| `prerequisite_missing` | 本地数据集、归一化划分、向量或模型配置缺失。 | 在本地准备该产物；不要期望自动下载。 |
| `checksum_mismatch` | 原始数据或归一化元数据与锁定不匹配。 | 只删除受影响的缓存版本，并从已验证源重新 fetch/normalize。 |
| `invalid_manifest` | 版本、许可证、来源、划分或嵌入配置不完整。 | 在运行前修复并重新锁定清单。 |
| 安全失败 | 返回了被禁止、跨作用域或非活动的记忆。 | 将该运行视为不可发布，并检查作用域/生命周期过滤器。 |
| 质量门禁失败 | 指标回退但没有安全违规。 | 在更改排名或分块策略之前，使用相同的语料、qrels 与嵌入配置比较报告。 |
| `capacity_refused` | 本地事件/查询/字节预算对所选语料过小。 | 增加显式本地预算或选择有界的划分/批次；不要通过导入生产作用域来绕过拒绝。 |
| task-series `checksum_mismatch` | 源包或其条件 qrels 在锁定之后发生变化。 | 重新审阅外部结果与归一化语料，然后重新计算两个锁定。 |
| task-series 不完整 | 某条件存在不确定或已被取代的阶段，且没有当前的外部判定覆盖。 | 为每个有序阶段提供兼容的外部结果；不要推断缺失的判定。 |

## PostgreSQL 与 pgvector

端到端完整基准使用 Stele 的 PostgreSQL + pgvector 检索路径，并且必须针对受支持的本地 PostgreSQL 版本运行。请将基准运行保持在隔离的基准项目/命名空间中。不要将公开基准语料数据导入生产租户或命名空间。批处理夹具播种器使用确定性 ID 与有界事务，因此中断的导入可以被清理并安全重跑。

在非合成的 LongMemEval 运行保留了一份机器可读报告（包含 PostgreSQL 与 pgvector 版本、源、归一化语料与 qrels 校验和、嵌入/策略配置、运行作用域、质量指标以及独立的安全门禁结果）之前，扩展变更不算完成。仓库自有的冒烟与合成 PostgreSQL 运行是有用的回归检查，但不满足该门禁。

## 可复现性检查清单

在共享或比较报告之前，请一起保留以下全部内容：

- [ ] PostgreSQL 主/次版本与 `pgvector` 扩展版本。
- [ ] 清单 JSON、上游修订、许可证决策、源 SHA256 与转换版本。
- [ ] 归一化划分校验和与 qrels 校验和/版本。
- [ ] 嵌入配置（或显式的 `lexical-only`）与策略/分块/排名配置。
- [ ] 数据集族、子集、运行 ID、基准租户/项目/命名空间与本地容量限制。
- [ ] 机器可读报告路径、状态、指标、错误、安全结果与清理结果。

对于全新的本地 PostgreSQL 18 容器，项目使用 pgvector 镜像 `docker.1ms.run/pgvector/pgvector:pg18` 并将其映射到未使用的宿主端口。在将报告视为可复现之前，运行 `go test ./...`、可选择加入的 PostgreSQL 基准测试以及 `openspec validate --specs --strict`。
