# Stele 自托管

**角色：** 运维参考。若需要简短的本地开发路径，请先阅读
[快速开始](getting-started.md)；生产配置、迁移、备份、恢复和灾难恢复请回到本文。

## 概述

PC5 引用支持复用现有的作用域/路径索引和能力发现；无需迁移。混合的 API/worker
版本会在持久引用版本上失败即关闭；回滚会禁用该能力，同时保留持久事件和来源。

PC5 的兼容性、回滚以及全新 PostgreSQL/重启验证见
[引用维护指南](provider-durable-references.md)。

PC6 在启用兼容副本之前需要增量迁移 0037。为让 scheduler 异步核对符合条件的投影反馈，
以及让 API 副本提供该能力，请设置 `STELE_PROVIDER_ENABLED=true`。回滚时保留仅追加的
投影和反馈历史；不支持破坏性的向下迁移。引导、协商、错误、待处理语义和公开重启证据请遵循
[投影维护指南](provider-evidence-projections.md)。

`Stele` 以三种进程模式运行在同一个 PostgreSQL 实例之上：

- `api`：公开摄取、记忆读取、检索、上下文装配，以及管理检查或生命周期路由
- `worker`：针对已认领原始事件的持续治理处理循环
- `scheduler`：定期维护调度，包括嵌入重建、派生洞察推导、摘要压缩、保留策略清扫和作业执行清理

仓库附带面向生产的 `Dockerfile` 以及本地 `docker-compose.yml`，运维人员无需手工重建运行时接线即可启动整套服务。

## 前置条件

- Docker 26+ 与 Docker Compose v2
- PostgreSQL 17+，并具备：
  - `pgcrypto`
  - 来自 `pgvector` 的 `vector`
- 一个可供三种 Stele 模式共同访问的 DSN

随附的 compose 文件使用 `pgvector/pgvector:pg17`，其中已包含所需的 `vector` 扩展。
PostgreSQL 数据卷挂载在 `/var/lib/postgresql`，因此同一份配置也支持 PostgreSQL 18
按主版本区分数据目录的布局。

## 运行时变量

所有模式必需：

- `STELE_MODE`：`api`、`worker`、`scheduler` 之一
- `STELE_POSTGRES_DSN`：PostgreSQL 连接字符串

常用可选变量：

- `STELE_HTTP_ADDR`：`api` 模式的监听地址，默认 `:8080`
- `STELE_AUTH_BOOTSTRAP_ADMIN_KEY`：一次性运维密钥，仅用于创建第一个持久管理员主体
- `STELE_AUTH_API_KEYS` 与 `STELE_AUTH_ADMIN_API_KEYS`：已废弃的无限制密钥列表；启动会拒绝它们。不要把这些名称复制到部署中；请使用下方受约束的引导流程。
- `STELE_DATABASE_MIGRATION_POLICY`：`auto`（默认）、`validate` 或 `off`，用于外部管理的前向迁移
- `STELE_HTTP_MAX_REQUEST_BODY_BYTES`、`STELE_HTTP_MAX_HEADER_BYTES`：有界的请求限制
- `STELE_HTTP_READ_HEADER_TIMEOUT`、`STELE_HTTP_READ_TIMEOUT`、`STELE_HTTP_WRITE_TIMEOUT`、`STELE_HTTP_IDLE_TIMEOUT`、`STELE_HTTP_SHUTDOWN_TIMEOUT`：有界的 HTTP 与排空超时
- `STELE_AUTH_DEFAULT_TENANT`：scheduler 默认租户作用域
- `STELE_AUTH_DEFAULT_PROJECT`：scheduler 默认项目作用域
- `STELE_AUTH_DEFAULT_NAMESPACE`：scheduler 默认命名空间作用域

迁移策略在 API 流量或后台作业认领之前进行评估。随附的单节点 Compose 配置使用
`auto`；若在启动所有模式之前单独运行迁移命令并单独管理迁移，则使用 `validate`。
`off` 不会绕过兼容性检查；它仅用于有明确文档记录的外部迁移工作流。

独立的迁移命令为：

```bash
STELE_POSTGRES_DSN='<operator-managed-dsn>' stele migrate status
STELE_POSTGRES_DSN='<operator-managed-dsn>' stele migrate up
STELE_MIGRATION_OUTPUT=json STELE_POSTGRES_DSN='<operator-managed-dsn>' stele migrate status
```

`migrate up` 只向前执行，并使用与运行时启动相同的 PostgreSQL 迁移账本和串行化机制。
它绝不会执行自动降级。迁移 0041 增加了精确作用域的公开读取观测和脱敏的摄取新鲜度证据。
向下迁移被有意设为不支持，因为这些记录属于审计历史；回滚应用程序二进制时应禁用检查并保留证据。

管理员可以使用普通 admin API key 和精确作用域头，通过
`GET /v1/admin/assurance/ingestion-freshness?fixture_reference=<event-uuid>`
检查事件夹具。响应区分已接纳、治理完成、规范态活跃和公开可检索阶段。默认窗口为从接纳入库到检查
15 分钟，以及治理/规范态或规范态/检索各转换 5 分钟。报告只包含不透明哈希和有界类别，不包含作用域值、事件或记忆 ID、路径、内容、查询文本、候选或排序数据。

`migrate status` 会报告驱动状态（`status`、`current_version`、`latest_version`、`dirty` 和 `pending`）以及
`integrity_status`/`integrity_rows`。`verified` 完整性状态表示每个已应用带编号迁移都在
`stele_schema_migration_ledger` 中具有预期的内嵌 SHA-256 记录。干净且较旧的受支持部署可能报告
`legacy`，直到下一次串行 `migrate up` 记录该不可变前缀；在状态同时为
`current` 和 `verified` 之前，不要在仅校验策略下接受运行时流量。

如果 `migrate status` 报告 `dirty`、`incompatible` 或 `divergent`，不要以 `auto` 重启应用模式，也不要手工编辑历史迁移资源或任一迁移账本。停止部署，保留 PostgreSQL 备份，检查有界诊断信息，然后执行文档记录的前向修复迁移，或将经过验证的备份恢复到显式目标。禁止自动向下迁移；只有在确认模式兼容性后才可使用较旧的镜像。迁移失败必须被修复或恢复，之后就绪状态才被视为有效。

## PostgreSQL 备份、恢复与验证

备份和恢复是由运维人员拥有的流程。请在运维主机上安装 PostgreSQL 客户端工具
（`pg_dump`、`pg_restore` 和 `psql`），提供显式的源/目标 DSN，并把生成的产物与清单保留在受保护目录中。脚本绝不会打印连接字符串或密码。

创建带校验和的自定义格式备份：

```powershell
pwsh -File scripts/stele-backup.ps1 `
  -SourceDsn $env:STELE_POSTGRES_DSN `
  -Destination .\backups\stele-$(Get-Date -Format yyyyMMdd-HHmmss).dump `
  -ServiceVersion $env:STELE_SERVICE_VERSION
```

只恢复到独立的可丢弃或替代目标。目标必须显式指定，且 `-ConfirmDestructive` 为必填；与源相同及模板数据库会被拒绝：

```powershell
pwsh -File scripts/stele-restore.ps1 `
  -Artifact .\backups\stele.dump `
  -Manifest .\backups\stele.dump.manifest.json `
  -SourceDsn $env:STELE_POSTGRES_DSN `
  -TargetDsn $env:STELE_VERIFY_POSTGRES_DSN `
  -ConfirmDestructive
```

让 Stele 连接已恢复的目标启动，然后执行有界迁移和作用域安全读取验证：

```powershell
pwsh -File scripts/stele-restore-verify.ps1 `
  -TargetDsn $env:STELE_VERIFY_POSTGRES_DSN `
  -Manifest .\backups\stele.dump.manifest.json `
  -BaseUrl http://localhost:8080 `
  -ApiKey $env:STELE_ADMIN_API_KEY `
  -Tenant $env:STELE_AUTH_DEFAULT_TENANT `
  -Project $env:STELE_AUTH_DEFAULT_PROJECT `
  -Namespace $env:STELE_AUTH_DEFAULT_NAMESPACE
```

只有成功的验证才应通过经过认证的保障恢复验证工作流记录为 `backup_restore_proof`。备份、
保留策略、调度、存储持久性以及恢复点/恢复时间目标仍由运维人员负责；Stele 不会调度或
上传备份。切勿隐式覆盖源数据库。

API 模式在 `GET /openapi.yaml` 发布运行中的契约，并在 `GET /version` 发布有界的
构建/模式兼容性元数据。这两个端点都是无需认证的发现接口，并且有意排除 DSN、凭据、
作用域值、迁移 SQL 和运维积压细节。

记忆路径是已经过认证的精确租户/项目/命名空间作用域内的可选元数据。在摄取或受治理的意图
写入时使用 `memory_path`；省略的值会以 `/` 存储和返回。搜索、上下文和记忆浏览接受精确的
`path` 选择器。后代需要显式的 `path_prefix` 选择器，它匹配完整的斜杠分隔段（因此
`agents/research` 不会匹配 `agents/researcher`）。`path` 与 `path_prefix` 互斥，并受
OpenAPI 模式中记录的共享路径语法约束。

### Agent 自我模型记忆约定

需要关于某个 agent 的持久、由调用方管理的信息的集成，可以选择在已经授权的精确
租户/项目/命名空间作用域内，使用 `agents/{agent-id}/self/{category}` 作为 `memory_path`。
初始类别为 `capabilities`、`preferences`、`limitations` 和 `lessons`。agent-id 仅是路径数据：
它不会选择作用域、标识已认证的主体或授予访问权限。共享的记忆路径校验和请求的
精确作用域头继续适用。

对描述性能力、偏好和限制使用现有的 `profile` 记忆类。对可复用的运维经验或流程使用
`procedural`。类别放置不会绕过受治理意图、接纳、版本历史、来源、遗忘或生命周期可见性。记住的
能力和限制可能已过时；服务器发布的能力、授权、配置和运行时强制限制仍为权威。此约定
不新增 API 或 MCP 工具，也不会自动把这些记忆注入到普通上下文中。

例如，使用已授权的精确作用域，通过现有的受治理事件流写入一条偏好：

```bash
curl -X POST http://localhost:8080/v1/events \
  -H 'X-API-Key: <authorized-api-key>' \
  -H 'X-Stele-Tenant: tenant-a' -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -H 'Idempotency-Key: <stable-request-key>' \
  -H 'Content-Type: application/json' \
  -d '{"event_type":"agent.preference","content":"Prefers concise answers","memory_path":"agents/agent-a/self/preferences"}'
```

仅在需要定向召回时请求该精确类别：

```bash
curl -X POST http://localhost:8080/v1/memories/search \
  -H 'X-API-Key: <authorized-api-key>' \
  -H 'X-Stele-Tenant: tenant-a' -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' -H 'Content-Type: application/json' \
  -d '{"query":"response style","top_k":10,"path":"agents/agent-a/self/preferences"}'
```

若有意请求有界的子树，请使用现有的段边界 `path_prefix` 选择器而不是 `path`；所有结果仍受
相同精确作用域、生命周期可见性和调用方提供的结果/上下文预算约束。例如，上下文装配可以显式请求该子树：

```bash
curl -X POST http://localhost:8080/v1/context/assemble \
  -H 'X-API-Key: <authorized-api-key>' \
  -H 'X-Stele-Tenant: tenant-a' -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' -H 'Content-Type: application/json' \
  -d '{"query":"agent operating preferences and lessons","budget":2048,"path_prefix":"agents/agent-a/self"}'
```

省略任一个选择器都不会把自我模型记忆变成常驻的上下文分区。

可选的 agent 运行时记忆 Provider 接口默认禁用。其作用域握手、有界元数据/引用契约、配置、一致性
和回滚流程记录在
[`docs/agent-runtime-memory-provider.md`](agent-runtime-memory-provider.md)。

<a id="provider-context-allocation-configuration"></a>
#### Provider 上下文分配配置

PC3 发布 `context_budget_version=context-budget-v1`、
`context_packing_policy_version=context-packing-v1` 和
`token_budget_supported=false`。在所有 API 副本上部署一致的设置，并在每次变更后
验证能力发现。这些独立限制管控最终授权的 Provider 投影；原生/admin/MCP 装配保留其
现有默认值，检索结果限制保留各自独立的含义。

| 环境变量 | 解析后的默认值 | 有效的服务器边界 / 单位 |
| --- | --- | --- |
| `STELE_PROVIDER_MAX_CONTEXT_ITEMS` | `100` | 返回条目放置数 `1..1000` |
| `STELE_PROVIDER_MAX_CONTEXT_CONTENT_BYTES` | 省略或为空时继承解析后的 `STELE_PROVIDER_MAX_CONTEXT_BYTES` | `0..4194304` 个 UTF-8 内容字节；不能超过响应上限 |
| `STELE_PROVIDER_MAX_CONTEXT_BYTES` | `1048576`（1 MiB） | `1..4194304` 个完整未压缩序列化 JSON 正文字节 |
| `STELE_PROVIDER_MAX_CITATIONS` | `100` | `1..1000` 个唯一的规范 kind/id/version 引用标识 |

未配置的内容值会同时继承响应覆盖值和默认值；显式的内容 `0` 是有效的。负数、溢出、非整数、条目/正文/引用为零、超过硬边界或内容大于响应的设置会让启动失败，而不是被静默修正。例如，将响应字节设为 `65536` 而内容未设置时，两个字节上限都会解析为 `65536`；将内容设为 `0` 会保留该零值且不改变披露模式。

调用方 `input.budget` 是一个正的可移植整数（最大 2147483647），可选封闭的
`budget_limits.max_content_bytes`、`max_response_bytes` 和 `max_citations`。省略或使用 `{}` 时采用已发现的上限。调用方内容/引用为零是有效的，正文字节至少为一，且有效限制是调用方与服务器最小值的独立取值。Null/重复/大小写/未知/令牌字段、小数、负数、溢出和尾部 JSON 会返回有界 400
`invalid_request`。不支持 token 预算和分词器。

每个成功的上下文正文都包含必填的封闭 `budget_accounting`。内容测量未转义的 UTF-8 字符串（引用模式为零）；正文字节包含外层信封、路径、源元数据、JSON 转义、重复引用和
计账本身，且不额外添加换行。整项接纳保留现有的资格/顺序/分区策略，保留强制引用并保守地预留有界
诊断空间，因此不必填满每个字节。隐藏或被拒绝的
证据不能贡献私有计数或存在性提示；可选的公开
洞察/目标/反馈/原始诊断路径保持关闭。

无法容纳调用方/服务器正文空间的空信封会在检索之前返回 413
`context_limit_exceeded`；最终的序列化/计账拒绝也不会产生部分结果。错误具有单独的服务器边界。披露使用当前精确作用域授权（403 `disclosure_denied`），而不可用的授权使用可重试的 503，而不是空结果或预算耗尽的成功。请为强制信封分配足够空间，而不要依赖更小的条目请求来修复它。

请升级严格消费者以处理必填计账和独立条目上限；
发现两个预算/打包标识并验证实际正文用量。在声称就绪之前，需运行全新的自有
PostgreSQL/pgvector 初始证据以及 API/worker 重启证据，并与归档的 PC1/PC2 证据分开。回滚要求先为需要 PC3 的消费者禁用
访问，再部署缺少该能力的二进制；保留 PC2
来源/授权强制和持久数据。PC3 不新增数据库迁移。缺失
能力即为不兼容，而不是允许剥离预算或改用其他
路由、本地打包或字节到 token 估算。

#### Provider 上下文连续性配置

PC4 新增迁移 `0036_provider_context_continuity`，它创建一个
仅元数据的 PostgreSQL 回执表及其查找/过期/容量索引。
在启用感知 PC4 的 Provider 消费者之前应用该迁移。该迁移
是增量式的：只有在较旧副本不广播 PC4 能力时，混合副本集群才是安全的。回滚期间，先禁用需要
摘要/连续性的消费者，然后部署较旧的二进制；不要向下迁移，
也不要删除回执、规范记忆、历史或授权。旧二进制只是
不提供较新的能力。

在 API 和 scheduler 副本上保持这些设置一致：

| 环境变量 | 默认值 | 有效边界 / 含义 |
| --- | ---: | --- |
| `STELE_PROVIDER_CONTEXT_CONTINUITY_TTL_SECONDS` | `86400` | `60..604800`；权威回执有效期（秒） |
| `STELE_PROVIDER_CONTEXT_CONTINUITY_MAX_RECEIPTS_PER_BINDING` | `128` | `1..4096`；每个完整运行时绑定保留的行数 |
| `STELE_PROVIDER_CONTEXT_CONTINUITY_MAX_RECEIPTS` | `100000` | `1..1000000`；整个服务保留的行数 |
| `STELE_PROVIDER_CONTEXT_CONTINUITY_CLEANUP_BATCH` | `500` | `1..5000`；单次清理运行处理的仅过期行数 |

启动会拒绝超出这些边界的值，也会拒绝超过全局容量的每绑定容量。PostgreSQL 提供签发和过期时间，使用
事务性的跨进程锁进行去重/容量控制，并可能复用未过期的兼容回执而不延长其过期时间。已保留的过期行
仍会计入容量，直到有界的 scheduler 清理将其移除；
活跃引用永远不会成为清理候选。省略连续性输入时不会发生
回执读写。不要在应用日志、指标、
故障证据或备份标注中放置不透明引用、投影摘要值、
查询、返回的记忆 ID、引用或正文。

scheduler 作业是 `provider_context_continuity_cleanup`。将其有界的
成功/失败遥测与普通 scheduler 健康监控结合，但不要从中推导
租户、绑定、路径、查询或来源信息。若需发布（rollout）证明，
请针对全新的 PostgreSQL/pgvector 运行自有的公开 Provider 校验器，
引导一个精确作用域凭据，签发并比较引用和内容
投影，重启 API/worker，然后测试来源生命周期/授权变更、
过期和不可行的分配。SQL 回执检查是补充性的；它
必须展示仅元数据持久化，且绝不能替代公开路由证据。

### 推理 Provider 边界

可选的模型辅助推理默认禁用，并且对于 API、worker、scheduler、检索或上下文装配启动从来不是
必需的。推理边界与 Provider 无关：已配置的 Provider 只接收
有界的、作用域内的证据，并返回候选或受治理的意图，
而不是直接变更规范记忆。被接受的候选在现有治理可以审查或接纳之前，会保留来源、
证据引用、策略/Provider 版本、不确定性和回放标识。

运维人员通过 `STELE_REASONING_ENABLED`、
`STELE_REASONING_MODE`（`offline`、`shadow` 或 `live`）以及有界的
`STELE_REASONING_MAX_*` 限制来配置该边界。在 Provider 注册之前，live 或 shadow
能力保持禁用；离线夹具回放不需要 Provider。
已授权的调用方可以在 `GET /v1/reasoning/capabilities` 检查脱敏的、作用域绑定的能力文档。API、worker 和 scheduler 从共享配置推导出
同一份文档。

可选的 OpenAI 兼容适配器使用以下额外设置：

- `STELE_REASONING_ENDPOINT`：精确的 chat-completions 兼容端点；
- `STELE_REASONING_MODEL`：配置的模型标识符；
- `STELE_REASONING_API_KEY`：运维凭据，绝不会在诊断中返回；
- `STELE_REASONING_TIMEOUT`：出站超时，默认 `30s`；
- `STELE_REASONING_MAX_REQUEST_BYTES` 与 `STELE_REASONING_MAX_RESPONSE_BYTES`：
  传输边界，默认 `128KiB` 与 `64KiB`。

远程 `shadow` 和 `live` 模式要求所有端点/模型/凭据设置完整，不完整时启动失败。
适配器发送一次有界的、服务器自有的结构化输出请求，并且不会执行自动重试。请先以 `shadow`
开始，以比较仅候选的诊断；通过设置
`STELE_REASONING_ENABLED=false` 或 `STELE_REASONING_MODE=offline` 回滚。离线回放绝不会联系已配置的端点。

离线回放使用校验和锁定的夹具和确定性的规范化
信封，不进行远程 Provider 调用。选择性启用的影子执行会将
Provider 结果与基线比较，但仍不具权威性；分歧、
陈旧、超时、取消、格式错误的输出以及 Provider 不可用
状态会保留基线并产生有界的诊断类别。重试
和回退行为是幂等的，不能扩展作用域、预算或
截止时间。一致性报告省略提示、思维链、凭据、
隐藏标识符和原始 Provider 载荷。

此边界不会自动激活 `hypothesis`、`goal`、`contradiction` 或
`causal_link` 洞察。具体的模型适配器和模型路由
策略仍属于单独的变更。若要回滚，请禁用推理模式；
离线/影子证据和候选记录保持可审计，而普通
记忆治理继续保持不变。

### 受治理的保留洞察激活

保留洞察激活是一个单独的、默认禁用的策略层。
初始策略只能接纳经过审查的 `hypothesis` 候选；`goal`、
`contradiction` 和 `causal_link` 在各自类型拥有独立的
证据和发布契约之前保持禁用。Provider 或回放运行只能返回
候选。精确作用域、证据子集、来源、置信度/不确定性、
Provider/模式版本、来源水位和幂等性检查会在
激活决策之前运行。

请先使用离线回放或影子。影子与回放报告有界的
`would_activate`、`quarantine`、`stale`、`incomplete` 或 `reject` 类别，
并且绝不会创建活跃洞察或改变默认的检索/上下文行为。
激活决策和证据链接是仅追加的，可在
已授权的 `GET /v1/admin/reasoning/activation-decisions` 路由上检查。响应
只包含作用域绑定的版本、指纹、处置类别、
原因码和有界时间戳；它不会暴露提示、
思维链、凭据、原始 Provider 载荷、隐藏标识符或
外部作用域值。

通过禁用策略来停止它。回滚时，持久化同一精确作用域
策略版本并设置 `rolled_back=true`；这会阻止新的接纳，同时
保留先前的洞察版本、证据和审计历史。现有
洞察必须使用正常的受治理抑制/过期转换，而不是
破坏性删除或规范记忆重写。

### 脱敏检索轨迹与记忆完整性评估

检索轨迹和记忆完整性证据是一个仅用于评估、
默认禁用的诊断层。它记录有界的通道可用性、
候选/扩展/预算/延迟分桶、新鲜度、回退和处置
类别。它还针对整合、合并、重分类、反思和上下文
投影工作，将动作成功与信息
完整性分开记录。

发布和评估报告将第二道门表示为
`information-integrity` 结果，独立于所请求的
组织动作是否完成。

该服务在此证据中绝不存储原始查询文本、租户/项目/命名空间值、
记忆或事件标识符、隐藏候选、原始分数、提示文本、
Provider 载荷、凭据或无界的执行计划。
作用域在管理员输出中以哈希表示；读取报告之前，授权仍会解析
确切的调用方作用域。已授权的运维人员可以使用普通
admin API key 和精确作用域头，在 `GET /v1/admin/retrieval/integrity-reports` 列出
有界报告。

缺失、被更改、意外重复、错位、外部作用域、
隐藏生命周期、陈旧水位、非确定性或回滚不完整的
证据属于硬性未通过结果。质量提升或成功的组织
动作不能覆盖它。该层不会激活渐进式上下文、
父级优先检索、重排、保留洞察、默认检索或
上下文装配。

报告是仅追加的派生 PostgreSQL 记录。收集仅针对
显式评估或影子工作运行；收集失败属于降级证据，
不会使普通搜索/上下文请求失败。保留策略清理只删除
过期的派生报告产物，并发出有界清理遥测；它
绝不会删除或重写规范源记忆。通过禁用收集或报告
比较来回滚。现有报告在过期之前保持可检查。

### 任务适配表示评估

表示评估是由维护者拥有的离线/影子诊断，而不是
部署开关。它仅针对四个声明的任务
族，比较校验和锁定的精确摘录、主题
投影和分层证据，并且始终与已授权的源搜索并行，处于相同的固定
候选、字符、token 和延迟限制之下。没有任何环境
变量、Provider 能力、MCP 工具、OpenAPI 路由或普通请求
参数可以启用候选。生产请求仍使用
`flat-fusion-v1`。

只能在具备 Docker 的检出目录中使用一次性的自有栈校验器：

```powershell
pwsh -NoProfile -File scripts/stele-representation-evaluation-verify.ps1
```

它会创建一个随机的自有 PostgreSQL + pgvector 容器，绝不采用
`STELE_POSTGRES_DSN`，最多输出有界的脱敏 JSON 报告，并且只删除
它自己的容器。缺少源搜索伴随项、作用域/生命周期或
新鲜度失败、受保护事实丢失、引用损坏、超出限制、
非确定性回放、持久化/重启失败或清理失败都算作
未通过，并保留基线。过期清理只移除派生的报告
行；它无法修改规范记忆。后续发布审查可以将
新的兼容证据作为一项前提条件，但激活需要
单独的授权变更，且不在本评估器范围内。

校验器会播种规范版本和来源，安装实际的 pgvector
修订，物化并读取现有投影，并执行两个
条件以及重复的候选和基线回滚查询。它会在真实 PostgreSQL 容器重启期间保留
四份聚合报告，并在作用域过期清理之前比较
它们的校验和与规范版本历史。
源漂移的故障注入会被回滚，不能使保留的
通过证据失效。随机发布的宿主端口会在重启后重新发现。
报告覆盖固定的合成语料；缺少外部任务判定和
维护观测仍为不确定。发布证据必须少于
24 小时，且不能超过其持久化的过期时间。

<a id="evidence-gated-resident-context-evaluation"></a>
### 证据门控的常驻上下文评估

这个仅限维护者的离线/影子诊断会将现有的 profile
置信度/大小放置策略与版本绑定的支持以及后续留出
效用进行比较。请在具备 Docker 的检出目录中运行它：

```powershell
pwsh -NoProfile -File scripts/stele-resident-evaluation-verify.ps1
```

校验器拥有一个一次性 PostgreSQL + pgvector 容器，并输出
有界的脱敏本地报告。它不会采用运维数据库。它会检查
实际持久化的规范版本、源元数据和投影引用、
变更源排除/降级、基线回滚以及真实数据库重启期间的投影/源
历史，然后移除它自己的容器。
缺少 Docker/数据库前置条件或持久化、重启或清理
检查失败都属于未通过。请保留无关容器和持久运维数据。

默认评估器上限为每项条件独立 8 个条目、4096 个 UTF-8 内容字节、1024 个空白
token 和 16 个引用；Provider 限制和
客户端模型 token 计账保留其现有含义。不合格或
支持不足的条目保持按需。源和不可变版本
元数据必须同时授权披露；旧版受限元数据不能
被静默视为公开以完成实验。精确观测边界和合格类型见
[上下文投影](context-projections.md#evidence-gated-resident-context-evaluation)。

没有公开路由、请求/响应变更、新的授权角色、
运行时启用标志或迁移。停止评估后，普通
放置仍然生效；内部禁用会返回经测试的基线计划。
通过的合成一致性报告证明有界的服务行为，
而不是一般任务收益。缺少外部任务/成本观测仍为
不确定，观察到的差异属于观察性质，实时激活
需要单独的授权提案和发布证据。

### 锁定的纵向 v3 分阶段回放

维护者拥有的 `task-series-v3` 评估器在现有离线
纵向运行器上扩展出五个锁定条件、阶段可见的源窗口
和独立的成本/门禁观测。它不是部署开关：不新增公开
路由、请求/响应字段、Provider 能力、MCP 工具、新的授权
角色、运行时标志、数据库迁移或依赖。普通
检索保留 `flat-fusion-v1` 和现有的 v1/v2 束默认值。

请在具备 Docker 的检出目录中运行一次性自有校验器：

```powershell
pwsh -NoProfile -File scripts/stele-longitudinal-v3-verify.ps1 -Diagnostic
```

测试装置会创建一个随机的自有 PostgreSQL + pgvector 容器，在各自的精确作用域中播种每个
阶段，检查物理未来/撤回证据
排除、披露/生命周期/作用域和引用谱系，强制相等的
独立条目/字节/token/引用预算，测量服务延迟，并在所有候选条件之后
回放无记忆基线。它会重启
容器，重新发现新分配的临时宿主端口，重复
回放，比较脱敏输入标识，并且只移除它自己的容器
和卷。缺少判定、成本观测、门禁、重启回放或
清理检查都属于未通过；不会产生部分质量通过。

五条件锁定合成夹具是
`internal/benchmark/testdata/locked-longitudinal-task-series-v3.json`；在清单
或语料发生任何编辑后，两个校验和都会通过 `benchmark lock-task-series` 重新计算。外部模型/工具观测和任务判定仍
独立提供；缺失值保持不确定，观察到的
差异仍属观察性质。这仅仅是合成服务一致性：
它既不确立纵向任务收益，也不确立发布（rollout）权威。精确形状、校验、限制、默认值和兼容性说明见
[本地 agent 记忆基准](local-agent-memory-benchmark.md#locked-v3-staged-replay)。

## 主体引导与作用域访问

受保护请求会针对由 PostgreSQL 支撑的主体进行认证。每个请求必须包含 `X-API-Key`、
`X-Stele-Tenant`、`X-Stele-Project` 和
`X-Stele-Namespace`；精确作用域必须已授予该主体。公开
主体需要显式的精确作用域披露配置，并使用 Provider
上下文以及安全的事件/意图写入。原生数据读取、历史、生命周期和
MCP 工具会拒绝公开凭据。管理路由需要 `admin` 角色。参见
[披露/源管理与迁移](context-disclosure-and-source-trust.md)。

首次部署时，配置全部四个引导变量并启动 API：

```bash
export STELE_AUTH_BOOTSTRAP_ADMIN_KEY='<temporary-bootstrap-secret>'
export STELE_AUTH_DEFAULT_TENANT='tenant-a'
export STELE_AUTH_DEFAULT_PROJECT='project-a'
export STELE_AUTH_DEFAULT_NAMESPACE='namespace-a'
```

在该默认作用域内创建第一个持久管理员：

```bash
curl -X POST http://localhost:8080/v1/admin/principals \
  -H 'X-API-Key: <temporary-bootstrap-secret>' \
  -H 'X-Stele-Tenant: tenant-a' -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -H 'Content-Type: application/json' \
  -d '{"role":"admin","label":"operator"}'
```

响应中的 `credential_secret` 只返回一次。请把它存入密钥
管理器，从环境中移除引导密钥，并通过管理端点轮换或撤销
凭据。主体列表/读取、授权和审计
响应绝不会包含原始凭据或摘要。授权管理仅限
精确作用域；主体不能通过更改请求头来创建或请求不同的作用域。

事件客户端必须对每个 `POST /v1/events` 发送有界、稳定的
`Idempotency-Key`。精确重试会以
`replayed=true` 返回原始 `event_id`；用同一密钥提交另一个载荷会返回 `409`。已租约化的
进行中认领也会返回 `409` 并带 `Retry-After: 1`。接纳拒绝
返回 `422`，且不会创建已完成的幂等结果，因此客户端可以在
接纳条件解决后重试。

嵌入路由变量：

- `STELE_EMBEDDING_DEFAULT_PROVIDER`：记录在重建资格和向量修订上的默认 provider 名称
- `STELE_EMBEDDING_DEFAULT_MODEL`：记录在重建资格和向量修订上的默认模型名称
- `STELE_EMBEDDING_DEFAULT_DIMENSIONS`：用于漂移检测和重建资格的默认向量维度目标
- `STELE_EMBEDDING_CLASS_ROUTES`：可选的逗号分隔按类覆盖，格式为 `class=provider:model:dimensions`
- `STELE_EMBEDDING_OPENAI_API_KEY`：内置 OpenAI 兼容嵌入 provider 的 API key
- `STELE_EMBEDDING_OPENAI_BASE_URL`：OpenAI 兼容部署的可选基础 URL 覆盖，默认 `https://api.openai.com/v1`
- `STELE_EMBEDDING_OPENAI_TIMEOUT`：出站嵌入请求超时，默认 `30s`

作业调优变量：

- `STELE_JOBS_MAINTENANCE_INTERVAL`：scheduler 节奏，默认 `15m`
- `STELE_JOBS_SUMMARY_COMPACTION_INTERVAL`：摘要压缩节奏，默认 `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_RETENTION_INTERVAL`：保留策略与过期清扫节奏，默认 `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_CLEANUP_INTERVAL`：维护清理节奏，默认 `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_DERIVED_INSIGHT_DERIVATION_INTERVAL`：受治理经验洞察推导节奏，默认 `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_DERIVED_INSIGHT_BATCH_SIZE`：每个作用域和每次推导运行读取的最大失败证据记录数，默认 `100`
- `STELE_JOBS_DERIVED_INSIGHT_MINIMUM_EVIDENCE`：激活 `failure_pattern` 之前要求的最小重复证据数，默认 `2`
- `STELE_JOBS_JOB_EXECUTION_RETENTION`：已完成作业执行记录的保留窗口，默认 `168h`
- `STELE_JOBS_WORKER_POLL_INTERVAL`：worker 空闲轮询延迟，默认 `5s`
- `STELE_JOBS_WORKER_ERROR_BACKOFF`：worker 重试退避，默认 `15s`
- `STELE_JOBS_SCHEDULER_ERROR_BACKOFF`：scheduler 重试退避，默认 `30s`
- `STELE_JOBS_GOVERNANCE_MAX_ATTEMPTS`：已认领治理原始事件的自动重试预算，默认 `5`
- `STELE_JOBS_GOVERNANCE_RETRY_BACKOFF`：治理尝试失败后的重试等待，默认 `30s`
- `STELE_JOBS_GOVERNANCE_LEASE_RENEW_INTERVAL`：续订进行中治理认领租约的节奏，默认 `30s`
- `STELE_JOBS_MAINTENANCE_SCOPE_BATCH_LIMIT`：每个 scheduler 周期评估的最大已发现作用域数，默认 `100`

## 嵌入部署模式

仅词法部署：

- 保持 `STELE_EMBEDDING_DEFAULT_PROVIDER`、`STELE_EMBEDDING_DEFAULT_MODEL`、`STELE_EMBEDDING_DEFAULT_DIMENSIONS` 和 `STELE_EMBEDDING_CLASS_ROUTES` 未设置。
- `api`、`worker` 和 `scheduler` 仍会正常启动。
- 混合检索仍可通过词法和关系信号使用，而语义重建执行保持有意不活跃。
- 管理嵌入诊断会报告 `semantic_rebuild_enabled=false` 以及降级运行时原因。

Provider 支撑的部署：

- 通过 `STELE_EMBEDDING_DEFAULT_*` 或 `STELE_EMBEDDING_CLASS_ROUTES` 配置至少一个嵌入路由目标。
- 提供匹配的 provider 凭据和端点设置，例如为内置 OpenAI 兼容 provider 设置 `STELE_EMBEDDING_OPENAI_API_KEY`。
- 三种运行时模式使用相同的 provider 注册规则，因此 `api`、`worker` 和 `scheduler` 解析出相同的 provider 名称。
- 如果配置的路由指向未知 provider 或 provider 特定设置不完整，启动会快速失败。

Provider 支撑的环境示例：

```bash
export STELE_EMBEDDING_DEFAULT_PROVIDER=openai
export STELE_EMBEDDING_DEFAULT_MODEL=text-embedding-3-small
export STELE_EMBEDDING_DEFAULT_DIMENSIONS=1536
export STELE_EMBEDDING_CLASS_ROUTES='profile=openai:text-embedding-3-small:1536,episodic=openai:text-embedding-3-small:1536'
export STELE_EMBEDDING_OPENAI_API_KEY='<provider-key>'
```

## 使用 Compose 进行本地引导

启动整套服务（仓库提供 `.env.local.example`；先将其复制为
`.env.local` 并替换占位符）：

```bash
Copy-Item .env.local.example .env.local
docker compose --env-file .env.local up --build -d
```

观察服务启动：

```bash
docker compose ps
docker compose logs -f api worker scheduler
```

停止整套服务：

```bash
docker compose down
```

重置整套服务（包括 PostgreSQL 数据）：

```bash
docker compose down -v
```

## 手动构建镜像

直接构建服务镜像：

```bash
docker build -t stele:local .
```

使用相同的 bootstrap-admin 配置手动运行 API 模式：

```bash
docker run --rm -p 8080:8080 \
  -e STELE_MODE=api \
  -e STELE_HTTP_ADDR=:8080 \
  -e STELE_POSTGRES_DSN='<operator-managed-dsn>' \
  -e STELE_AUTH_BOOTSTRAP_ADMIN_KEY='<bootstrap-secret>' \
  -e STELE_AUTH_DEFAULT_TENANT=tenant-a \
  -e STELE_AUTH_DEFAULT_PROJECT=project-a \
  -e STELE_AUTH_DEFAULT_NAMESPACE=namespace-a \
  stele:local
```

## 冒烟检查

### 最初十分钟的自有冒烟路径

对于新的检出目录，使用自有快速路径来验证核心自托管
闭环，无需准备运行中的 API 或手工管理凭据：

```powershell
pwsh -File scripts/stele-first-ten-minutes.ps1
```

该命令拥有隔离的 Compose 项目、随机宿主端口、临时
凭据和清理。其默认截止时间为 **600 秒**（10 分钟），
`-TimeoutSeconds` 只能将其提高到 **900 秒**（15 分钟）。
稳定的阶段为 `preflight`、`start`、`discovery`、`readiness`、
`bootstrap`、`lifecycle`、`telemetry` 和 `cleanup`。成功运行以
`PASS` 结束；没有 Docker 或无法访问守护进程的本地机器返回
`SKIP` 且退出码为 `2`，这不算是通过。在 CI 中设置
`STELE_FIRST_TEN_MINUTES_CI=1`，以便相同的前置条件缺失会使
任务失败。仅在需要有界诊断时使用 `-KeepResources`；凭据仍会
被移除，输出只打印自有的 Compose 项目名称。

该路径检查 `/health`、`/readyz`、`/version`、`/openapi.yaml`、worker
处理、作用域搜索、上下文装配和 `/metrics`，同时复用
`scripts/stele-bootstrap-smoke.ps1` 进行引导和公开生命周期
断言。它会脱敏 DSN、API key、作用域值、记录标识符、
请求正文、provider 载荷和原始进程错误。它不新增 API 路由，
不改变 PostgreSQL，不运行备份/恢复或发布证据检查，也不
替代下面的完整产品验证路径。

### 最初十分钟的运维闭环

在 `docker compose up --build -d` 之后使用此闭环，以证明自托管的 `api`、`worker` 和 `scheduler` 模式已端到端接通。该闭环全程使用显式作用域头；不要省略它们，也不要复用其他命名空间的数据。

冒烟夹具作用域：

```text
tenant=tenant-a
project=project-a
namespace=namespace-a
bootstrap_admin_key=<bootstrap-secret>
runtime_api_key=<durable-runtime-credential-created-by-bootstrap>
actor=operator-a
```

对于可重复的 PowerShell 冒烟流程，请针对运行中的 API 运行仓库脚本。它会创建持久管理员、创建公开运行时主体、
将每个一次性凭据只写入显式提供的目录，并
检查持久管理员存在后引导凭据会被拒绝：

```powershell
pwsh -File scripts/stele-bootstrap-smoke.ps1 `
  -BootstrapKey $env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY `
  -Tenant $env:STELE_AUTH_DEFAULT_TENANT `
  -Project $env:STELE_AUTH_DEFAULT_PROJECT `
  -Namespace $env:STELE_AUTH_DEFAULT_NAMESPACE `
  -CredentialOutputDirectory .\.stele-smoke-credentials
```

该脚本默认配置 `-DisclosureProfile memory-reference-v1`，并禁用
来源证明。仅在获得批准的内容访问中使用 `memory-content-v1`。生命周期部分会测试受支持的公开写入和原生读取
拒绝，然后使用管理员凭据进行治理检查和
作用域内的摄取/治理/检索/上下文证明。
当单独的公开 Provider 夹具执行治理
和检索验证时，使用 `-SkipLifecycle`。运行 `scripts/stele-provider-context-verify.ps1` 以进行
全新的 pgvector 制备、带 profile 的读取、授权编辑、回放和重启检查。

对于隔离的真实栈产品门禁，请运行：

```powershell
pwsh -File scripts/stele-product-verify.ps1
```

这会创建一个唯一的 Compose 项目和生成的凭据，运行
引导/生命周期冒烟，并且只移除它自己的容器、网络和
卷。在缺少 Docker 或守护进程未运行的开发机器上，它会以
状态 `2` 退出并打印 `SKIP`；这明确不算通过。CI 会设置
`STELE_PRODUCT_VERIFY_CI=1`，此时缺少 Docker 前置条件会使任务失败。
仅在需要有界诊断时使用 `-KeepResources` 来保留唯一命名的验证资源。

对于受限网络，将 `STELE_PRODUCT_VERIFY_POSTGRES_IMAGE` 设置为从已配置的仓库镜像源
可用的镜像引用。Compose 还接受
`STELE_POSTGRES_IMAGE`、`STELE_POSTGRES_HOST_PORT` 和 `STELE_HTTP_HOST_PORT`
覆盖项；隔离校验器会自动选择随机宿主端口。

例如，当 `1ms.run` Docker 仓库代理可达时：

```powershell
$env:STELE_PRODUCT_VERIFY_POSTGRES_IMAGE = "docker.1ms.run/pgvector/pgvector:pg17"
$env:STELE_PRODUCT_VERIFY_GO_IMAGE = "docker.1ms.run/library/golang:1.25-bookworm"
$env:STELE_PRODUCT_VERIFY_RUNTIME_IMAGE = "docker.1ms.run/library/debian:bookworm-slim"
$env:STELE_PRODUCT_VERIFY_GOPROXY = "https://goproxy.cn,direct"
pwsh -File scripts/stele-product-verify.ps1
```

代理主机是运维人员的选择，除非显式设置此覆盖项，否则不会联系它。

相同的构建镜像覆盖项也可直接在 Compose 中使用，即
`STELE_GO_IMAGE`、`STELE_RUNTIME_IMAGE` 和 `STELE_GOPROXY`。默认值
仍为官方 Docker Hub 和 Go 模块代理引用。

请将 `.stele-smoke-credentials` 视为机密材料：把其中的值移入
运维管理的密钥存储，并在冒烟运行后移除该目录。

在摄取冒烟夹具之前，先用引导密钥创建第一个持久管理员，
然后创建精确作用域的公开/运行时主体。引导密钥仅对配置的默认作用域有效，并且在持久管理员存在后会被拒绝。每个返回的 `credential_secret` 只存储一次，并
为其配置用于事件写入和 Provider 上下文的披露配置文件。下面的
原生检查步骤需要持久管理员凭据。
公开凭据使用受支持的 Provider 上下文和事件/意图路径。

1. 确认进程存活和就绪：

```bash
curl http://localhost:8080/livez
curl http://localhost:8080/readyz
```

2. 摄取作用域内的冒烟夹具事件。重复的 `smoke.provider_failure` 事件会为派生洞察推导和回放创建合格证据；`smoke.operator_recovery` 为后续的检索和上下文装配提供一个运维人员可检索的恢复短语。

```bash
curl -X POST http://localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <durable-runtime-credential-created-by-bootstrap>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"event_type":"smoke.provider_failure","content":"smoke fixture: embedding provider unavailable during rebuild attempt","source_timestamp":"2026-07-11T00:00:00Z"}'

curl -X POST http://localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <durable-runtime-credential-created-by-bootstrap>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"event_type":"smoke.provider_failure","content":"smoke fixture: embedding provider unavailable during rebuild retry","source_timestamp":"2026-07-11T00:01:00Z"}'

curl -X POST http://localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <durable-runtime-credential-created-by-bootstrap>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"event_type":"smoke.operator_recovery","content":"smoke fixture: operator waits for provider recovery before retrying rebuilds","source_timestamp":"2026-07-11T00:02:00Z"}'
```

3. 检查 worker 和 scheduler 进度，无需直接查询 PostgreSQL：

```bash
curl http://localhost:8080/v1/admin/jobs/governance/status \
  -H 'X-API-Key: <admin-credential>'

curl 'http://localhost:8080/v1/admin/jobs/status?limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

如果治理仍处于待处理状态，请检查 worker 日志和 `STELE_POSTGRES_DSN`。如果缺少 scheduler 执行记录，请确认 `scheduler` 进程正在运行，且 `STELE_AUTH_DEFAULT_TENANT`、`STELE_AUTH_DEFAULT_PROJECT` 和 `STELE_AUTH_DEFAULT_NAMESPACE` 与夹具作用域匹配。

4. 验证检索和上下文装配。诊断标志对此已认证请求是安全的，并返回类别/计数解释，例如 `included`、`omitted_by_budget`、`omitted_by_quality` 或 `hidden_by_lifecycle_or_scope`。

```bash
curl -X POST http://localhost:8080/v1/memories/search \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"query":"provider recovery rebuild","top_k":5,"include_summaries":true,"include_relations":true}'

curl -X POST http://localhost:8080/v1/context/assemble \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"query":"avoid repeated provider rebuild failures","budget":6,"include_experience_insights":true,"include_diagnostics":true}'
```

5. 运行有界派生洞察回放预演。这会预览回放决策，而不会变更派生洞察或规范记忆。

```bash
curl -X POST http://localhost:8080/v1/admin/derived-insight-replays:dry-run \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"insight_types":["failure_pattern","lesson"],"evidence_window_start":"2026-07-11T00:00:00Z","evidence_window_end":"2026-07-11T00:10:00Z","evidence_limit":20,"actor":"operator-a","reason":"first-ten-minutes smoke replay dry-run"}'
```

预期的报告形态包括 `counters.evidence_evaluated`、诸如 `create` 或 `skip` 的决策类别，以及诸如 `repeated_evidence` 或 `insufficient_evidence` 的稳定原因码。

6. 排队有界回放应用并检查持久运行。Apply 会返回回放运行 id；广泛的变更稍后由 scheduler 拥有的后台工作执行。

```bash
curl -X POST http://localhost:8080/v1/admin/derived-insight-replays \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"insight_types":["failure_pattern","lesson"],"evidence_window_start":"2026-07-11T00:00:00Z","evidence_window_end":"2026-07-11T00:10:00Z","evidence_limit":20,"actor":"operator-a","reason":"first-ten-minutes smoke replay apply","idempotency_key":"smoke-replay-tenant-a-project-a-namespace-a"}'

curl 'http://localhost:8080/v1/admin/derived-insight-replays?mode=apply&limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl http://localhost:8080/v1/admin/derived-insight-replays/<replay-run-id>/report \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

如果运行保持 `pending`，请确认 scheduler 模式正在运行，并在 `/v1/admin/jobs/status` 中检查 `derived_insight_replay_execution`。如果运行为 `failed`，请先读取回放报告；它包含部分计数和失败摘要。如果运行为 `continuation_required`，请用更晚的有界窗口或调整后的 `evidence_limit` 重复回放。

7. 确认回放输出只能通过普通生命周期、质量、作用域和预算规则成为上下文可见：

```bash
curl -X POST http://localhost:8080/v1/context/assemble \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"query":"avoid repeated provider rebuild failures","budget":8,"include_experience_insights":true,"include_diagnostics":true}'
```

成功的回放可见性表现为带有引用的 `known_failures` 或 `experience_lessons` 条目。缺失并不自动意味着失败；请使用 `diagnostics` 区分预算压力、质量排序、生命周期隐藏或缺少合格证据。

8. 抓取低基数回放和冒烟信号指标：

```bash
curl http://localhost:8080/metrics | grep 'stele_derived_insight_replay_total'
curl http://localhost:8080/metrics | grep 'stele_operations_total'
```

回放指标使用诸如 `mode`、`result`、`insight_type`、`decision` 和 `reason` 的标签。它们有意排除租户、项目、命名空间、回放运行 id、洞察 id、actor、原因文本和原始错误消息。

### 持久作用域证明与记忆会话循环

当你需要对某个租户/项目/命名空间能否接受记忆、处理记忆、检索记忆、装配上下文、暴露质量发现、建议修复，以及在修复后重跑，得到可审计的答案时，请使用持久证明工作流。这将取代把不相关的冒烟端点响应当作主要事实来源的做法。

1. 创建一个作用域证明运行：

```bash
curl -X POST http://localhost:8080/v1/admin/scope-proofs \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"checks":["scope_resolution","ingestion","governance","retrieval","context","replay","quality","repair"],"fixture_mode":"smoke","actor":"operator-a","reason":"prove fresh scope memory loop"}'
```

2. 检查证明和报告。运行包含步骤状态；报告补充关联证据和稳定的后续动作。

```bash
curl http://localhost:8080/v1/admin/scope-proofs/<proof-run-id> \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl http://localhost:8080/v1/admin/scope-proofs/<proof-run-id>/report \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

3. 根据报告的 `next_actions` 诊断故障，而不是从原始 HTTP 响应猜测：

- `inspect_governance_status` 或 `inspect_worker_jobs`：读取 `/v1/admin/jobs/governance/status` 和 `/v1/admin/jobs/status`。
- `inspect_retrieval_quality`：创建或打开 `/v1/admin/memory-quality/evaluations`。
- `inspect_context_diagnostics`：使用 `"include_diagnostics":true` 重新运行 `/v1/context/assemble`。
- `open_quality_evaluation`：检查 `/v1/admin/memory-quality/evaluations/<evaluation-run-id>/findings`。
- `open_repair_plan`：仅在审查后检查或批准 `/v1/admin/memory-quality/repair-plans/<repair-plan-id>`。
- `inspect_derived_insight_replay`：读取 `/v1/admin/derived-insight-replays/<replay-run-id>/report`。

4. 修复后重跑。重跑会创建一个链接到上一次运行的新证明；它们不会覆盖原始证据。

```bash
curl -X POST http://localhost:8080/v1/admin/scope-proofs/<proof-run-id>:rerun \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-a","reason":"rerun after proof remediation"}'
```

5. 演练外部 agent 记忆会话契约。Stele 装配上下文并记录与记忆相关的结果，但外部 agent 仍负责模型调用、提示编排和最终答案。

```bash
curl -X POST http://localhost:8080/v1/memory-sessions \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"agent-a","reason":"serve user turn","metadata":{"integration":"self-hosting-smoke"}}'

curl -X POST http://localhost:8080/v1/memory-sessions/<session-id>/turns \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"query":"remember deployment preference","context_budget":1200,"include_relations":true,"include_experience_insights":true,"include_diagnostics":true}'

curl -X POST http://localhost:8080/v1/memory-sessions/<session-id>/turns/<turn-id>:outcome \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"outcome_event_ids":["<event-id>"],"expected_recall":["<event-id>"]}'

curl -X POST http://localhost:8080/v1/memory-sessions/<session-id>:verify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"turn_id":"<turn-id>","expected_recall":["<event-id>"]}'

curl http://localhost:8080/v1/memory-sessions/<session-id>/report \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

6. 抓取证明/会话指标。这些指标有意保持低基数，并排除租户、项目、命名空间、证明 id、会话 id、轮次 id、事件 id、记忆 id、actor 和原因文本。

```bash
curl http://localhost:8080/metrics | grep 'stele_scope_proof_steps_total'
curl http://localhost:8080/metrics | grep 'stele_memory_session_verifications_total'
```

这闭合了服务侧的闭环：`作用域证明 -> 会话上下文 -> 外部 agent 轮次 -> 轮次结果摄取 -> 治理 -> 检索/上下文验证 -> 质量/修复建议 -> 重跑证明/会话`。

### 生产就绪保障与一致性循环

在冒烟、作用域证明和记忆会话检查之后，当运维人员需要对某个租户/项目/命名空间是否已为生产就绪得到持久答案时，请使用此闭环。这些路由仅限管理员，并且必须全程使用相同的作用域头。

1. 运行健康评估。请求可以包含服务自有的容量/负载证明和运维人员提供的备份/恢复证明证据；Stele 只存储有界类别和计数器，而不存储原始部署机密或外部备份载荷。

```bash
curl -X POST http://localhost:8080/v1/admin/assurance/health-evaluations \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"capacity_load_proof":{"status":"healthy","severity":"info","reason":"capacity_within_thresholds","evidence":{"backlog_depth":0,"worker_latency_ms":120}},"backup_restore_proof":{"status":"healthy","severity":"info","reason":"backup_restore_fresh","evidence":{"marker":"restore-check-2026-07-13"}}}'
```

2. 定义外部 agent 集成证据契约。一致性配置文件声明外部 agent 应留下哪些 Stele 证据；Stele 不执行该 agent、不调用模型、不构建提示，也不生成最终答案。

```bash
curl -X POST http://localhost:8080/v1/admin/assurance/conformance-profiles \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"expected_evidence":[{"kind":"session","minimum_count":1,"freshness_window":"24h"},{"kind":"context","minimum_count":1,"freshness_window":"24h"},{"kind":"outcome","minimum_count":1,"freshness_window":"24h"},{"kind":"verification","minimum_count":1,"freshness_window":"24h"},{"kind":"usefulness_feedback","minimum_count":1,"freshness_window":"24h"},{"kind":"task_evaluation","minimum_count":1,"freshness_window":"24h"},{"kind":"proof","minimum_count":1,"freshness_window":"24h"}],"actor":"operator-a","reason":"production readiness evidence chain"}'
```

3. 运行一致性检查并检查缺失证据诊断。缺失证据使用稳定类别，例如 `session_without_outcome`、`turn_without_context`、`verification_missing`、`feedback_without_subject`、`task_evaluation_missing_evidence`、`repair_without_verification` 或 `rollout_without_dry_run`。

```bash
curl -X POST http://localhost:8080/v1/admin/assurance/conformance-runs \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"profile_id":"<conformance-profile-id>"}'

curl 'http://localhost:8080/v1/admin/assurance/conformance-runs?profile_id=<conformance-profile-id>' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

4. 生成并检查作用域就绪报告。就绪状态综合最新健康评估、一致性运行、容量/负载证明、备份/恢复证明、活跃故障计数器、告警候选计数器以及推荐的管理界面。

```bash
curl -X POST http://localhost:8080/v1/admin/assurance/readiness-reports \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{}'

curl 'http://localhost:8080/v1/admin/assurance/readiness-reports' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

5. 审阅故障和告警候选。故障保留仅追加的运维降级历史。告警投递适配器有意限定为 `disabled`、`stdout` 和通用 `webhook`；webhook 投递默认使用 HTTPS，拒绝不安全目标，限制超时和载荷大小，脱敏已配置的机密，并且不增加厂商专用的告警行为。

```bash
curl 'http://localhost:8080/v1/admin/assurance/incidents' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl 'http://localhost:8080/v1/admin/assurance/alert-candidates' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl 'http://localhost:8080/v1/admin/assurance/alert-candidates/<alert-candidate-id>/delivery-attempts' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

6. 通过推荐的运行手册提示进行修复，然后验证恢复。恢复验证会将修复证据链接到故障、告警候选、一致性运行、修复结果、排序回滚、证明运行、会话验证、容量/负载证明或备份/恢复证明，且不覆盖先前历史。

```bash
curl -X POST http://localhost:8080/v1/admin/assurance/recovery-verifications \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"target":"incident","target_id":"<incident-id>","status":"healthy","result_category":"recovered","checked_surfaces":["readiness","conformance","capacity_load"],"actor":"operator-a","reason":"verified after remediation"}'
```

7. 抓取保障指标和日志。指标使用诸如 `stele_assurance_health_evaluations_total`、`stele_assurance_incidents_total`、`stele_assurance_alert_candidates_total`、`stele_assurance_alert_delivery_total`、`stele_assurance_cleanup_total`、`stele_conformance_runs_total`、`stele_conformance_missing_evidence_total`、`stele_operational_proofs_total`、`stele_readiness_reports_total` 和 `stele_recovery_verifications_total` 的序列。结构化生命周期日志使用 `component=assurance event=lifecycle` 和 `component=conformance event=lifecycle`。

指标和生命周期日志有意排除租户、项目、命名空间、记录 id、actor、原因文本、查询文本、webhook URL 和收件人字段。作用域记录详情请使用上述管理检查路由。

运行时启动与排空遥测同样限定为 `mode`、`component`、
`operation` 和 `status`。它记录迁移校验、启动、信号、
就绪/排空、超时和清理结果，而不会输出 DSN、
凭据、作用域、主体或原始错误文本。产品验证只输出
阶段/结果类别；详细诊断保留在本地测试
输出或被显式保留的自有 Compose 资源中。

在此服务闭环之后，剩余的产品缺口是明确的：SDK/UI 上手、外部 agent 运行时集成、厂商专用的告警路由、托管故障管理以及自适应评分校准仍在本服务自有范围之外。

### 基线启动

1. 检查存活：

```bash
curl http://localhost:8080/health
```

预期：

```json
{"status":"ok"}
```

2. 检查就绪：

```bash
curl http://localhost:8080/ready
```

预期：

```json
{"status":"ready"}
```

`/ready` 只确认基线进程依赖（例如 PostgreSQL 连接性）。它不证明语义重建执行已接通。

3. 摄取一个事件：

```bash
curl -X POST http://localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"event_type":"conversation.message","content":"user prefers concise answers"}'
```

4. 检查治理积压：

```bash
curl http://localhost:8080/v1/admin/jobs/governance/status \
  -H 'X-API-Key: <admin-credential>'
```

5. 检查最近的 scheduler 和维护执行记录：

```bash
curl 'http://localhost:8080/v1/admin/jobs/status?limit=5' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

6. 在治理提升后检查某条记忆的历史：

```bash
curl http://localhost:8080/v1/admin/memories/<memory-id>/history \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

7. 检查一个作用域内的治理恢复候选：

```bash
curl 'http://localhost:8080/v1/admin/governance/raw-events?state=retry_wait&attempt_gte=1&limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

### 语义就绪

Provider 支撑的部署：

1. 通过积压检查确认嵌入运行时接线：

```bash
curl 'http://localhost:8080/v1/admin/embedding/rebuilds?limit=5' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

预期的运行时形态：

```json
{
  "runtime": {
    "configured": true,
    "semantic_rebuild_enabled": true,
    "registered_providers": ["openai"]
  },
  "items": []
}
```

2. 在治理提升后检查某条记忆，以验证重建状态、活跃向量谱系、漂移可见性和失败上下文：

```bash
curl http://localhost:8080/v1/admin/memories/<memory-id>/embedding \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

仅词法部署：

1. 运行相同的积压检查调用。
2. 预期基线启动保持健康，同时语义接线被报告为有意不活跃：

```json
{
  "runtime": {
    "configured": false,
    "semantic_rebuild_enabled": false,
    "reason": "semantic rebuild execution is inactive because no embedding routes are configured"
  },
  "items": []
}
```

3. 将 `semantic_rebuild_enabled=false` 加上降级原因视为仅词法部署的成功条件，而不是故障。

### 嵌入 Provider 切换发布（rollout）

当你需要将某个作用域迁移到新的嵌入 provider 或模型目标，而不重写现有向量谱系时，请使用切换计划。

1. 创建切换计划，包含显式不可变的目标快照和有界波次大小：

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"target":{"provider":"openai","model":"text-embedding-3-large","dimensions":3072},"classes":["profile","episodic"],"wave_size":25,"reason":"migrate semantic target"}'
```

2. 在激活前预检草稿计划。报告是即时生成且不持久化的，因此激活时仍会针对当前作用域和运行时状态重新运行接纳检查：

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:preflight \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

允许的报告具有 `decision:"allow"`。仅告警的报告仍允许激活；例如 `many_waves` 表示 scheduler 将需要多个有界的调度波次。被拒绝的报告具有 `decision:"deny"` 和稳定的阻塞码，例如 `target_unresolved`、`unsupported_class_route`、`scoped_plan_conflict` 或 `zero_eligible_memory`。

```json
{
  "component": "embedding_cutover",
  "decision": "deny",
  "blockers": [
    {
      "severity": "blocker",
      "code": "zero_eligible_memory"
    }
  ],
  "eligible_total": 0
}
```

3. 仅在预检允许且运行时检查确认目标 provider 当前已注册之后，才激活该计划：

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:activate \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"start bounded rollout"}'
```

4. 通过计划摘要和条目详情两方面监控发布（rollout）。`queued` 显示未调度的未来工作，`rebuilding` 显示当前已调度的波次，`failed` 突显需要运维人员关注的热点集合：

```bash
curl 'http://localhost:8080/v1/admin/embedding/cutovers?status=active&limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl http://localhost:8080/v1/admin/embedding/cutovers/<plan-id> \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

5. 仅当你希望停止未来波次时才暂停或取消。已在重建的工作保持正常的 worker 所有权。`pause` 保留未调度的工作以便后续恢复，而 `cancel` 让历史和已调度的工作保持可审计，但阻止剩余队列集合继续推进：

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:pause \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"hold next wave during incident review"}'

curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:cancel \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"abort rollout after provider regression"}'
```

6. 在发布（rollout）故障期间，以作用域和记忆两个粒度检查恢复审计。这些查询让你确认计划内发生了哪些重试或重新排队，以及是谁执行的：

```bash
curl 'http://localhost:8080/v1/admin/embedding/recovery-history?cutover_plan_id=<plan-id>&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl 'http://localhost:8080/v1/admin/memories/<memory-id>/embedding/recovery-history?cutover_plan_id=<plan-id>&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

7. 在发布（rollout）处于活跃状态时抓取运行时健康和指标：

```bash
curl http://localhost:8080/livez
curl http://localhost:8080/readyz
curl http://localhost:8080/metrics | grep 'stele_embedding_'
```

`livez` 只证明进程能够响应。`readyz` 在 `api` 模式检查 PostgreSQL；当语义重建或切换执行启用时，`worker` 和 `scheduler` 的就绪检查还包括嵌入 provider 可达性。Provider 网络故障不是切换激活的阻塞项，但会表现为就绪降级和 `stele_embedding_provider_probe_total` 指标。

指标接口包括接纳决策和发现码、活跃或暂停的切换计划计数、切换条目状态计数、重建积压仪表、provider 探测结果以及 scheduler 波次调度计数器。标签有意保持低基数，不包含记忆 id、原始事件 id、切换计划 id 或自由形式的错误消息。

8. 回滚被建模为朝先前 provider、模型和维度创建的新前向切换计划。不要就地变更 `vector_revisions`，也不要试图把旧修订重新激活作为临时的回滚捷径。

### 记忆质量与修复循环

当摄取仍然可用，但召回质量、语义投影、治理积压或修复可行性看起来降级时，请使用此闭环。该闭环保持在服务侧：它创建持久评估和修复记录，然后已批准的修复动作由 worker 执行拾取，而不是由管理请求内联执行。

1. 检查事件摄取返回的接纳元数据。正常响应包含 `event_id`；降级或排队响应还包含 `admission.decision` 和稳定的发现码，例如 `semantic_projection_degraded` 或 `governance_backlog_high`。

2. 创建作用域质量评估：

```bash
curl -X POST http://localhost:8080/v1/admin/memory-quality/evaluations \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"checks":["retrieval","context","admission_pressure","repair_pressure"],"actor":"operator-a","reason":"investigate degraded recall"}'
```

3. 检查该评估的发现：

```bash
curl http://localhost:8080/v1/admin/memory-quality/evaluations/<evaluation-run-id>/findings \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

4. 从可操作的发现创建修复计划。当你只想检查映射的动作时，请先使用 `dry_run:true`。

```bash
curl -X POST http://localhost:8080/v1/admin/memory-quality/repair-plans \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"evaluation_run_id":"<evaluation-run-id>","actor":"operator-a","reason":"repair degraded projection and backlog","dry_run":true}'
```

5. 使用 `dry_run:false` 重新创建或检查该计划，然后在动作列表安全时批准它：

```bash
curl -X POST http://localhost:8080/v1/admin/memory-quality/repair-plans/<repair-plan-id>:approve \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-a","reason":"approve bounded repair actions"}'
```

6. 通过修复计划、作业状态和指标监控修复执行：

```bash
curl http://localhost:8080/v1/admin/memory-quality/repair-plans/<repair-plan-id> \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl http://localhost:8080/metrics | grep 'stele_quality_'
```

7. 在可执行动作完成后运行修复后验证：

```bash
curl -X POST http://localhost:8080/v1/admin/memory-quality/repair-plans/<repair-plan-id>:verify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"checks":["retrieval","admission_pressure","repair_pressure"],"actor":"operator-a","reason":"verify repair outcome"}'
```

修复计划绝不会就地重写规范记忆内容、来源或版本历史。不受支持的发现会变为 `manual_review` 动作，可执行动作被约束到现有的受治理路径，例如嵌入重试、治理重新排队和派生洞察回放。

### 外部 agent 记忆反馈循环

此原生会话/反馈工作流需要具有精确作用域的持久管理员主体。
公开运行时使用单独协商的
[PC6 投影操作](provider-evidence-projections.md)；原生会话
和反馈管理不会授予公开凭据更宽的访问权限。

当 Stele 与外部 agent 运行时集成，且你需要从召回记忆到质量修复闭合产品反馈循环时，请使用此闭环。Stele 仍仅是记忆服务：它不调用模型、不构建提示、不运行外部 agent，也不生成最终答案。

1. 在外部 agent 轮次之前创建作用域记忆会话：

```bash
curl -X POST http://localhost:8080/v1/memory-sessions \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"agent-a","reason":"serve external turn","metadata":{"integration":"external-agent"}}'
```

2. 创建一个幂等轮次以装配记忆上下文：

```bash
curl -X POST http://localhost:8080/v1/memory-sessions/<session-id>/turns \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"idempotency_key":"turn-1","query":"deployment preference","context_budget":1200,"include_relations":true,"include_experience_insights":true,"include_diagnostics":true,"include_feedback_diagnostics":true}'
```

3. 在外部 agent 完成其自身轮次后，通过现有的摄取路径记录结果引用或有界的结果载荷：

```bash
curl -X POST http://localhost:8080/v1/memory-sessions/<session-id>/turns/<turn-id>:outcome \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"idempotency_key":"outcome-1","outcome_event_ids":["evt_existing"],"event_payloads":[{"event_type":"agent_observation","content":"User prefers staged rollout","metadata":{"source":"external-agent"}}],"expected_recall":["evt_existing"]}'
```

4. 请求召回验证。多次尝试会作为会话验证历史保留：

```bash
curl -X POST http://localhost:8080/v1/memory-sessions/<session-id>:verify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"turn_id":"<turn-id>","expected_recall":["evt_existing"]}'
```

5. 为返回的记忆、引用、会话、轮次、验证或缺失的预期召回记录有用性反馈。已知的预期召回目标使用带类型的 `kind` 加 `id`；不透明的调用方期望使用 `kind:"opaque"` 和 `opaque_token`，并且不会被当作内部标识符。

```bash
curl -X POST http://localhost:8080/v1/usefulness-feedback \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"type":"missing_expected","source_surface":"verification","subjects":[{"kind":"expected_recall","expected_recall_target":{"kind":"memory","id":"mem_expected"}}],"actor":"agent-a","reason":"expected memory was absent","idempotency_key":"feedback-expected-1"}'

curl -X POST http://localhost:8080/v1/usefulness-feedback \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"type":"noisy","source_surface":"context","subjects":[{"kind":"memory","id":"mem_noisy"}],"actor":"agent-a","reason":"retrieved but not useful","idempotency_key":"feedback-noisy-1"}'
```

6. 通过管理路由检查活跃摘要和反馈历史。这些原生会话报告需要管理员访问权限；公开 Provider 消费者不能将它们用作替代的披露路径。

```bash
curl 'http://localhost:8080/v1/admin/usefulness-feedback?subject_kind=memory&subject_id=mem_noisy&include_superseded=true' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl 'http://localhost:8080/v1/admin/usefulness-feedback/summary?subject_kind=memory&subject_id=mem_noisy' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

7. 更正错误反馈而不删除审计历史：

```bash
curl -X POST http://localhost:8080/v1/admin/usefulness-feedback/<feedback-id>:supersede \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-a","reason":"feedback was attached to the wrong subject"}'
```

8. 将重复的活跃反馈转换为质量发现，然后创建需要批准的修复建议：

```bash
curl -X POST http://localhost:8080/v1/admin/memory-quality/evaluations \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"checks":["retrieval","context"],"actor":"operator-a","reason":"inspect active usefulness feedback"}'

curl -X POST http://localhost:8080/v1/admin/memory-quality/repair-plans \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"evaluation_run_id":"<evaluation-run-id>","actor":"operator-a","reason":"review feedback-derived findings","dry_run":true}'
```

由反馈派生的修复动作仍受管理员门控。任何公开反馈请求都不能批准计划、抑制记忆、重试嵌入、检查治理、回放洞察或内联执行修复。

9. 在运维人员修复后重跑验证：

```bash
curl -X POST http://localhost:8080/v1/admin/memory-quality/repair-plans/<repair-plan-id>:verify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"checks":["retrieval","context"],"actor":"operator-a","reason":"verify feedback remediation"}'

curl http://localhost:8080/v1/memory-sessions/<session-id>/report \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

会话报告暴露有界的反馈摘要、验证尝试、质量评估 id、质量发现 id 和代码、修复计划 id 以及后续动作。它不会通过公开报告字段暴露隐藏的记忆内容或作用域之外的证据。

### 外部 agent 任务成功循环

此原生任务评估工作流需要管理员访问权限。存储的任务
证据不会成为权威的运行时任务状态。

当外部 agent 集成需要记录任务级成功证据并检查有界报告时，请使用此闭环。Stele 存储调用方提供的判定和证据；它不运行任务，也不推断成功。

1. 在外部运行完成后记录任务评估：

```bash
curl -X POST http://localhost:8080/v1/task-evaluations \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"objective":"validate memory recall","success_criteria":["return only scoped memory"],"verdict":"partial","contribution_categories":["memory_missing","hidden_memory"],"evidence":[{"kind":"session","id":"session_1"},{"kind":"expected_recall","id":"mem_expected_1"},{"kind":"opaque","opaque_token":"caller-opaque-evidence"}],"actor":"agent-a","reason":"record external task outcome","idempotency_key":"task_eval_1"}'
```

2. 读取有界的任务报告，以检查关联证据和后续动作：

```bash
curl http://localhost:8080/v1/task-evaluations/<task-evaluation-id>/report \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

3. 通过管理员边界检查或更正任务评估：

```bash
curl 'http://localhost:8080/v1/admin/task-evaluations?verdict=partial&limit=25' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl http://localhost:8080/v1/admin/task-evaluations/<task-evaluation-id> \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl -X POST http://localhost:8080/v1/admin/task-evaluations/<task-evaluation-id>/supersede \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-a","reason":"corrected verdict"}'

curl 'http://localhost:8080/v1/admin/task-evaluations/summary?evidence_target_kind=session&evidence_target_id=session_1' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

任务报告只暴露有界的关联 ID、证据类别和后续动作。不透明的调用方证据令牌仅为审计目的存储，不会被当作 Stele 内部标识符。

### 反馈循环剩余的产品缺口

本仓库现已闭合服务侧的任务成功与受治理的排序发布（rollout）循环：外部任务评估可以输入作用域内摘要，排序策略可以被预演、激活、检查、禁用和回滚，默认搜索/上下文排序只能通过受治理的排序发布（rollout）策略改变。若干产品界面仍有意留在本提案之外：

- 供调用方挂接会话、反馈主体、任务证据和发布（rollout）检查而无需手工构造 JSON 的 SDK/UI 采集界面。
- 决定何时调用会话、结果、验证、任务评估、反馈和排序发布（rollout）路由的外部 agent 运行时集成。
- 通用自托管 webhook 适配器之外的托管故障管理和厂商专用告警集成。
- 高级评分校准，包括自适应权重、流量切分、长期记忆有用性评分以及超出此处所实现的保守有界信号的离线评估测试装置。
- 最终用户的提示编排和模型调用，它们仍留在 Stele 的服务边界之外。

### 保障功能的剩余产品缺口

保障与一致性循环是服务自有且对自托管友好的，但它有意止步于持久诊断、管理检查、通用告警投递和恢复验证。剩余的产品缺口在本仓库边界之外：

- 引导调用方完成会话、上下文、结果、验证、有用性反馈、任务评估、证明、修复和排序发布（rollout）证据采集的 SDK/UI 采集界面。
- 外部 agent 运行时实现，包括 agent 执行、工具编排以及决定何时写入每条证据记录。
- 面向 Slack、PagerDuty、邮件提供商、工单系统或托管故障管理产品的厂商专用告警集成。
- 托管故障管理工作流，例如升级策略、值班表、寻呼、确认归属和事后复盘工具。
- 自适应评分校准，包括在线学习、流量切分、长期记忆有用性评分和离线评估测试装置。
- 模型调用、提示编排和最终答案生成，它们仍留在 Stele 的服务边界之外。

## 记忆管理界面

`Stele` 现在暴露两个不同的记忆管理边界：

- 公开读取界面，由持久运行时主体凭据认证
- 特权生命周期界面，由持久管理员主体凭据认证

公开读取路由：

- `GET /v1/memories`
- `GET /v1/memories/{memory_id}`
- `GET /v1/memories/{memory_id}/history`
- `GET /v1/memories/{memory_id}/provenance`

特权生命周期路由：

- `POST /v1/admin/memories`
- `PATCH /v1/admin/memories/{memory_id}`
- `POST /v1/admin/memories/{memory_id}:merge`
- `POST /v1/admin/memories/{memory_id}:reclassify`
- `POST /v1/admin/memories/{memory_id}:suppress`
- `POST /v1/admin/memories/{memory_id}:expire`
- `POST /v1/admin/memories/{memory_id}:delete`

特权治理恢复路由：

- `GET /v1/admin/governance/raw-events`
- `GET /v1/admin/governance/raw-events/{raw_event_id}`
- `GET /v1/admin/governance/raw-events/{raw_event_id}/recovery-history`
- `POST /v1/admin/governance/raw-events/{raw_event_id}:retry`
- `POST /v1/admin/governance/raw-events/{raw_event_id}:reschedule`
- `POST /v1/admin/governance/raw-events/{raw_event_id}:requeue`

特权嵌入检查与恢复路由：

- `GET /v1/admin/embedding/rebuilds`
- `GET /v1/admin/embedding/cutovers`
- `POST /v1/admin/embedding/cutovers`
- `GET /v1/admin/embedding/cutovers/{cutover_plan_id}`
- `POST /v1/admin/embedding/cutovers/{cutover_plan_id}:activate`
- `POST /v1/admin/embedding/cutovers/{cutover_plan_id}:pause`
- `POST /v1/admin/embedding/cutovers/{cutover_plan_id}:cancel`
- `GET /v1/admin/embedding/recovery-history`
- `GET /v1/admin/memories/{memory_id}/embedding`
- `GET /v1/admin/memories/{memory_id}/embedding/recovery-history`
- `POST /v1/admin/embedding/rebuilds/{memory_id}:retry`
- `POST /v1/admin/embedding/rebuilds/{memory_id}:requeue`

特权派生洞察检查路由：

- `GET /v1/admin/derived-insights`
- `GET /v1/admin/derived-insights/{insight_id}`
- `GET /v1/admin/derived-insights/{insight_id}/feedback`
- `POST /v1/admin/derived-insights/{insight_id}/feedback`
- `POST /v1/admin/derived-insight-feedback/{feedback_id}:supersede`
- `POST /v1/admin/derived-insights/{insight_id}:suppress`
- `POST /v1/admin/derived-insight-replays:dry-run`
- `GET /v1/admin/derived-insight-replays`
- `POST /v1/admin/derived-insight-replays`
- `GET /v1/admin/derived-insight-replays/{replay_run_id}`
- `GET /v1/admin/derived-insight-replays/{replay_run_id}/report`

特权记忆质量与修复路由：

- `POST /v1/admin/memory-quality/evaluations`
- `GET /v1/admin/memory-quality/evaluations/{evaluation_run_id}`
- `GET /v1/admin/memory-quality/evaluations/{evaluation_run_id}/findings`
- `POST /v1/admin/memory-quality/repair-plans`
- `GET /v1/admin/memory-quality/repair-plans/{repair_plan_id}`
- `POST /v1/admin/memory-quality/repair-plans/{repair_plan_id}:approve`
- `POST /v1/admin/memory-quality/repair-plans/{repair_plan_id}:verify`

任务成功检查路由：

- `POST /v1/task-evaluations`
- `GET /v1/task-evaluations/{evaluation_id}/report`
- `GET /v1/admin/task-evaluations`
- `GET /v1/admin/task-evaluations/{evaluation_id}`
- `GET /v1/admin/task-evaluations/summary`
- `POST /v1/admin/task-evaluations/{evaluation_id}/supersede`

特权手动变更和生命周期动作要求：

- 管理员 API key
- 作用域头
- `X-Stele-Actor`
- 包含 `reason` 的 JSON 请求体

特权治理恢复动作使用相同的管理员边界，并额外要求：

- 每个恢复动作都要求 `X-Stele-Actor`
- 每个恢复动作都要求 JSON 请求体中的 `reason`
- 仅 `:reschedule` 需要 `scheduled_for`

特权嵌入恢复动作使用相同的管理员边界，并要求：

- 每个恢复动作都要求 `X-Stele-Actor`
- 每个恢复动作都要求 JSON 请求体中的 `reason`

特权派生洞察抑制使用相同的管理员边界，并要求：

- JSON 请求体中的 `actor`
- JSON 请求体中的 `reason`

特权派生洞察反馈使用相同的管理员边界，并要求：

- JSON 请求体中的 `actor`
- JSON 请求体中的 `reason`
- 有界的 `type` 取值：`useful`、`noisy`、`incorrect`、`stale`、`redundant` 或 `needs_review`

特权记忆质量与修复动作使用相同的管理员边界，并要求：

- 每个评估和修复计划都要求作用域头
- 评估、修复计划创建和批准要求 JSON 请求体中的 `actor`
- 修复计划创建和批准要求 JSON 请求体中的 `reason`
- 有界的评估检查项：`retrieval`、`context`、`admission_pressure` 或 `repair_pressure`
- 由发现生成的有界修复动作：`embedding_retry`、`governance_requeue`、`derived_insight_replay` 或 `manual_review`

治理恢复查询过滤器：

- `state`
- `event_type`
- `attempt_gte`
- `attempt_lte`
- `failed_from`
- `failed_to`
- `next_attempt_from`
- `next_attempt_to`
- `limit`
- `cursor`

手动变更说明：

- `POST /v1/admin/memories` 仅支持运维人员编写的主类；派生 `summary` 记忆被排除。
- `PATCH /v1/admin/memories/{memory_id}` 只更新规范内容。作用域、生命周期和类别保留在专用界面上。
- merge 和 reclassify 需要 `expected_version` 乐观并发控制。
- `reclassify` 仅针对 `profile`、`episodic` 或 `procedural`；本阶段 `relation` 和 `summary` 仍被排除。
- 实质性的手动变更会立即清除过期的语义参与，并记录持久重建资格，以便后续后台重新生成。

治理恢复说明：

- 管理治理检查返回派生的原始事件状态：`pending`、`retry_wait`、`leased`、`exhausted` 和 `processed`。
- `retry` 仅针对 `retry_wait` 条目，并将 `next_attempt_at` 提前到立即可被 worker 拾取。
- `reschedule` 仅针对 `pending` 或 `retry_wait` 条目，并将 `next_attempt_at` 移动到运维人员提供的未来时间戳。
- `requeue` 仅针对 `exhausted` 条目，清除 `governance_exhausted_at`，重置自动尝试计数器，并将事件返回到普通 worker 认领路径。
- `leased` 和 `processed` 条目会被拒绝，而不是被强制接管。
- 每个恢复动作都会写入仅追加的 `governance_recovery_ledger`，包含 actor、原因以及变更前后的状态快照。
- 恢复绝不会触发直接执行。现有治理 worker 会在后续轮询中通过正常的持久认领路径拾取该事件。
- 第一阶段边界保持狭窄：仅支持单项恢复，不支持批量修复、不允许接管租约状态，也不提供 `ignore` 或 `drop` 终态动作。

## 受治理的经验洞察

- 派生洞察是独立的受治理记录，而不是规范记忆。它们不会就地重写规范记忆行、记忆版本、向量修订或来源。
- 首个活跃洞察类型是 `failure_pattern`。scheduler 从重复的作用域证据推导它，例如失败的作业执行、失败的嵌入重建记录、原始事件治理错误、恢复历史以及与失败相关的规范记忆内容。
- `lesson` 洞察是来自现有 `failure_pattern` 的确定性投影。lesson 必须引用证据并引用其源失败模式。
- `hypothesis`、`goal`、`contradiction` 和 `causal_link` 在本阶段仅为保留词汇。Stele 不会自主推断或激活这些类型。
- 推导是异步的。摄取、手动变更、检索和上下文装配请求不会在前台运行失败模式推导。
- 默认上下文装配排除派生洞察。调用方必须在 `POST /v1/context/assemble` 上设置 `include_experience_insights=true`，才能请求 `known_failures` 和 `experience_lessons` 分区。
- 已认证的调用方可以在 `POST /v1/context/assemble` 上设置 `include_diagnostics=true`，以获得包含、因预算省略、因质量省略和因生命周期隐藏的经验洞察分区的类别/计数诊断。
- 被抑制、已遗忘、已删除和超出作用域的洞察会从上下文装配中排除。管理检查仍可使用 `include_hidden=true` 读取隐藏洞察状态、证据和生命周期历史。
- `POST /v1/admin/derived-insights/{insight_id}:suppress` 记录一次有审计的生命周期转换，并保留关联证据历史。
- 运维人员可以为派生洞察记录质量反馈，而无需重写洞察正文或删除证据。活跃反馈摘要包含在管理洞察详情响应中，并被 scheduler 推导和可选的上下文装配排序使用。
- 反馈驱动的策略可以通过有审计的生命周期转换抑制嘈杂或错误的洞察。单条反馈记录不是破坏性删除，可以被取代，同时仍保留在反馈历史中可见。
- 反馈指标以低基数 `stele_insight_feedback_total` 序列通过 `/metrics` 暴露。标签有意排除租户、项目、命名空间、洞察 id、actor 和原因文本。
- 回放指标以低基数 `stele_derived_insight_replay_total` 序列通过 `/metrics` 暴露。标签有意排除租户、项目、命名空间、回放运行 id、洞察 id、actor、原因文本和原始错误消息。
- 此功能不新增 MCP 工具、推理 Provider 抽象、SDK 行为、UI 行为、全局 agent 自身命名空间或自主因果/目标推断。

派生洞察检查示例：

```bash
curl 'http://localhost:8080/v1/admin/derived-insights?type=failure_pattern&state=active&min_evidence_count=2&limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

上下文装配选择性启用示例：

```bash
curl -X POST http://localhost:8080/v1/context/assemble \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"query":"prepare the next embedding rebuild run","budget":6,"include_experience_insights":true,"include_diagnostics":true}'
```

派生洞察抑制示例：

```bash
curl -X POST http://localhost:8080/v1/admin/derived-insights/<insight-id>:suppress \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-a","reason":"noisy duplicate"}'
```

派生洞察反馈示例：

```bash
curl -X POST http://localhost:8080/v1/admin/derived-insights/<insight-id>/feedback \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"type":"noisy","actor":"operator-a","reason":"too broad for context assembly","quality_score":0.2}'
```

派生洞察反馈历史示例：

```bash
curl 'http://localhost:8080/v1/admin/derived-insights/<insight-id>/feedback?include_superseded=true&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

派生洞察反馈取代示例：

```bash
curl -X POST http://localhost:8080/v1/admin/derived-insight-feedback/<feedback-id>:supersede \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-b","reason":"replaced by more accurate review"}'
```

## 嵌入生命周期

- 语义检索只读取链接到当前规范投影的活跃向量修订。被取代、失败、过期或缺失的修订会从默认语义召回中排除。
- 当规范记忆没有活跃语义投影时，混合检索仍可通过词法和关系召回完成。请求不会仅因为语义投影不可用而失败。
- 手动创建、更新、合并和重分类路径会同步使过期的活跃语义参与失效，然后依赖 scheduler 驱动的重建执行来稍后恢复语义召回。
- scheduler 通过持久的 `embedding_rebuilds` 状态发现缺失的嵌入以及 provider 或模型漂移，然后在不阻塞写入路径的情况下认领重建工作。
- 每次生成尝试都会记录在仅追加的 `vector_revisions` 中，包括失败的尝试和被取代的谱系。激活使用比较并提升的守卫，因此过期内容永远不会成为活跃语义投影。
- 路由元数据来自 `STELE_EMBEDDING_DEFAULT_*` 和可选的 `STELE_EMBEDDING_CLASS_ROUTES` 覆盖。这些设置定义了用于资格、漂移检测和审计历史的目标。
- Provider 实现仍是内部运行时集成点。内置运行时当前根据环境配置接线一个 OpenAI 兼容适配器。
- 如果未配置任何嵌入路由，服务保持仅词法模式，并通过管理嵌入诊断暴露语义不活跃状态。
- 如果配置了嵌入路由但匹配的 provider 无法注册，启动会失败而不是静默降级。
- 管理嵌入检查暴露重建积压、单记忆向量谱系、provider 或模型漂移以及失败尝试上下文，无需直接访问数据库。
- Provider 切换作为运维人员拥有的计划保持持久，具有不可变目标快照、作用域内成员、有界波次大小和仅追加的审计上下文。
- scheduler 通过普通的 `embedding_rebuilds` 路径以有界波次推进活跃切换计划，然后再认领工作执行。
- 暂停切换会停止未来波次，同时让已调度或正在重建的波次保持普通 worker 所有权。
- 取消切换会阻止未调度的剩余条目继续推进，但不会清除已完成、正在重建或失败的发布（rollout）历史。
- `POST /v1/admin/embedding/rebuilds/{memory_id}:retry` 仅将失败的恢复工作恢复为 `pending`。
- `POST /v1/admin/embedding/rebuilds/{memory_id}:requeue` 将符合条件的当前或失败重建工作返回为 `pending`，以便普通后台重建作业可以再次拾取。
- 已处于 `rebuilding` 状态的重建记录会被拒绝，而不是被强制接管。
- 每个嵌入恢复动作都会写入仅追加的 `embedding_recovery_ledger`，包含 actor、原因以及重建前后快照。
- 恢复历史可以按 `cutover_plan_id`、actor、动作和时间窗口过滤，因此发布（rollout）故障无需直接访问数据库即可解释。
- 嵌入恢复绝不会直接变更 `vector_revisions`。后台执行仍拥有追加、比较和提升行为。
- 反向切换是唯一支持的回滚模型。请朝先前目标创建新计划，而不是就地重写向量历史。

## 示例流程

1. 列出可见的规范记忆：

```bash
curl 'http://localhost:8080/v1/memories?class=profile&limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

2. 检查某条记忆历史：

```bash
curl http://localhost:8080/v1/memories/<memory-id>/history \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

3. 检查来源谱系：

```bash
curl http://localhost:8080/v1/memories/<memory-id>/provenance \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

4. 通过管理员边界抑制一条记忆：

```bash
curl -X POST http://localhost:8080/v1/admin/memories/<memory-id>:suppress \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"manual override"}'
```

5. 通过管理员边界直接播种一条规范记忆：

```bash
curl -X POST http://localhost:8080/v1/admin/memories \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"class":"profile","content":"user prefers concise answers","reason":"seed curated memory"}'
```

6. 使用乐观并发控制更正记忆内容：

```bash
curl -X PATCH http://localhost:8080/v1/admin/memories/<memory-id> \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"content":"user strongly prefers concise answers","expected_version":1,"reason":"correct curated fact"}'
```

7. 将一条重复记忆合并到存续目标：

```bash
curl -X POST http://localhost:8080/v1/admin/memories/<target-memory-id>:merge \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"source_memory_id":"<source-memory-id>","content":"user strongly prefers concise answers","expected_version":2,"reason":"merge duplicate canonical facts"}'
```

8. 将记忆重分类到正确的受治理类别：

```bash
curl -X POST http://localhost:8080/v1/admin/memories/<memory-id>:reclassify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"target_class":"procedural","expected_version":3,"reason":"fix canonical class"}'
```

9. 列出等待重试的治理原始事件：

```bash
curl 'http://localhost:8080/v1/admin/governance/raw-events?state=retry_wait&attempt_gte=1&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

10. 在修复前读取一条原始事件详情：

```bash
curl http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id> \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

11. 立即重试一条等待中的原始事件：

```bash
curl -X POST http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id>:retry \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"retry now after transient dependency recovery"}'
```

12. 将一条原始事件重新调度到更晚的维护窗口：

```bash
curl -X POST http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id>:reschedule \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"delay until downstream system is stable","scheduled_for":"2026-06-13T03:00:00Z"}'
```

13. 将一条耗尽状态的原始事件重新排队回普通 worker 轮询：

```bash
curl -X POST http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id>:requeue \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"clear exhausted state after operator review"}'
```

14. 读取一条原始事件的恢复审计历史：

```bash
curl http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id>/recovery-history \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

15. 检查一个作用域的嵌入重建积压：

```bash
curl 'http://localhost:8080/v1/admin/embedding/rebuilds?status=failed&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

16. 检查某条记忆的嵌入谱系：

```bash
curl http://localhost:8080/v1/admin/memories/<memory-id>/embedding \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

17. 重试一次失败的嵌入重建：

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/rebuilds/<memory-id>:retry \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"retry after provider recovery"}'
```

18. 将当前或此前失败的嵌入重建重新排队回普通 scheduler 所有权：

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/rebuilds/<memory-id>:requeue \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"requeue after routing update"}'
```

19. 列出最近和活跃的嵌入切换计划：

```bash
curl 'http://localhost:8080/v1/admin/embedding/cutovers?limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

20. 详细读取一个嵌入切换计划：

```bash
curl http://localhost:8080/v1/admin/embedding/cutovers/<plan-id> \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

21. 创建一个嵌入切换计划：

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"target":{"provider":"openai","model":"text-embedding-3-large","dimensions":3072},"classes":["profile"],"wave_size":10,"reason":"test provider cutover"}'
```

22. 预检一个切换计划：

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:preflight \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

23. 激活或暂停一个切换计划：

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:activate \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"start rollout"}'

curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:pause \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"stop next wave"}'
```

24. 在发布（rollout）故障期间检查作用域级嵌入恢复历史：

```bash
curl 'http://localhost:8080/v1/admin/embedding/recovery-history?cutover_plan_id=<plan-id>&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

25. 在同一故障期间检查某条记忆的嵌入恢复历史：

```bash
curl 'http://localhost:8080/v1/admin/memories/<memory-id>/embedding/recovery-history?cutover_plan_id=<plan-id>&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

## 集成证据工作流黄金路径

工作流契约记录外部集成是否已采集到某个 agent 轮次、任务或作业所需的服务侧证据。它不执行外部 agent，也不变更关联的源证据。

1. 为该作用域创建一个管理员拥有的模板。模板步骤使用有界类型和证据类别：

```bash
curl -X POST http://localhost:8080/v1/admin/workflows/templates \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"integration_kind":"agent_turn","completion_policy":"strict","actor":"operator-a","reason":"golden integration path","steps":[{"kind":"session_started","requirement":"required","allowed_evidence":["session"],"minimum_count":1,"requires_internal":true,"freshness_window":"24h","completion_window":"1h","position":1},{"kind":"turn_outcome_recorded","requirement":"required","allowed_evidence":["outcome"],"minimum_count":1,"requires_internal":true,"freshness_window":"24h","completion_window":"1h","position":2}]}'
```

2. 外部集成启动其作用域工作流运行。重复相同的幂等键会恢复原始运行：

```bash
curl -X POST http://localhost:8080/v1/workflows/runs \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"template_id":"<workflow-template-id>","idempotency_key":"external-turn-001","actor":"agent-integration","reason":"capture evidence for one turn"}'
```

3. 在通过各自路由写入正常的记忆会话/上下文/结果/验证/反馈/任务证据之后，将相关的作用域记录挂接为工作流步骤：

```bash
curl -X POST http://localhost:8080/v1/workflows/runs/<workflow-run-id>/steps \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"kind":"session_started","actor":"agent-integration","reason":"session evidence recorded","evidence_links":[{"kind":"session","source":"public_api","target_id":"<memory-session-id>"}]}'
```

4. 读取针对下一个缺失步骤的有界公开指引。此响应有意排除工作流 ID、作用域 ID、隐藏证据、提示和模型输出：

```bash
curl http://localhost:8080/v1/workflows/runs/<workflow-run-id>/next-actions \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

5. 管理员检查诊断出的缺口，并可以取代错误的证据链接而不更改源记录：

```bash
curl http://localhost:8080/v1/admin/workflows/runs/<workflow-run-id>/diagnostics \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl -X POST http://localhost:8080/v1/admin/workflows/evidence-links/<evidence-link-id>/supersede \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-a","reason":"replace invalid evidence link"}'
```

6. 对相同作用域运行现有的一致性和就绪路由，然后在工作流相关故障已修复时创建恢复验证。工作流证据以有界类别参与保障；它绝不会自动解决故障。

仅当需要 scheduler 驱动的过期诊断和高容量工作流历史清理时，才设置 `STELE_WORKFLOW_MAINTENANCE_ENABLED=true`。它默认禁用。`STELE_WORKFLOW_DIAGNOSTIC_INTERVAL`、`STELE_WORKFLOW_STALE_RUN_WINDOW`、`STELE_WORKFLOW_DIAGNOSTIC_SCAN_LIMIT`、`STELE_WORKFLOW_NEXT_ACTION_REFRESH_LIMIT` 和 `STELE_WORKFLOW_HISTORY_RETENTION` 约束该维护路径。监控 `stele_workflow_lifecycle_total` 以获取模板/运行/步骤/证据/诊断/后续动作/清理类别。

Stele 不提供 SDK/UI 界面、外部 agent 执行、模型调用、提示编排、工具编排或最终答案生成。这些仍属于外部集成；Stele 仅拥有作用域内的记忆和证据记录。

## 运维说明

## 检索质量评估

Stele 在
`internal/retrieval/testdata/retrieval-evaluation-fixture-v1.json` 保留仓库自有的检索夹具。它覆盖
单事实、多跳、时间、记忆类别、矛盾、噪声、重复、
生命周期和作用域隔离用例。夹具源仅用于测试，绝不是
生产数据或生成的答案。

从仓库根目录运行确定性单元覆盖：

```powershell
go test ./internal/retrieval ./internal/telemetry -count=1
```

真实 PostgreSQL 回放有意采用选择性启用，并要求在 `STELE_TEST_RETRIEVAL_EVALUATION_DSN` 中提供一次性的、由测试装置拥有的 DSN：

```powershell
$env:STELE_TEST_RETRIEVAL_EVALUATION_DSN = '<owned-test-dsn>'
$env:STELE_TEST_RETRIEVAL_EVALUATION_OWNED = 'true'
pwsh -File scripts/retrieval-evaluation.ps1 -TimeoutSeconds 300
```

在任何数据库连接或引导之前，包装脚本和直接运行 Go 测试装置都要求所有权标记。
包装脚本会校验显式 DSN，应用 30 秒到 15 分钟的有界超时，并创建隔离的
每次运行报告目录。测试装置还会拒绝 `STELE_POSTGRES_DSN` 的规范化等价形式，
每次运行使用唯一命名空间，并且只清理该次运行精确播种的 ID。

如果未设置 DSN 变量，命令会打印
`SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` 并以代码 `2` 退出；它绝不会回退到
`STELE_POSTGRES_DSN` 或其他环境数据库。播种器使用保留的 `eval` 租户和
`retrieval-baseline` 项目，通过正常的
原始事件/候选/规范态/来源/生命周期路径写入，并用夹具版本标记记录以标明所有权。请使用一次性数据库，并在回放后移除其
夹具作用域；切勿将此命令指向运维或生产数据库。

报告记录 `fixture_version`、`representation_version`、`ranking_version`、
兼容的嵌入修订、策略版本、证据组指标、候选
池大小和有界延迟。跨作用域或隐藏生命周期结果的
安全失败会覆盖所有质量分数。阈值修改需要新的策略版本；
基线和候选报告在比较前必须使用兼容的夹具和表示
版本。

已完成的拥有型运行还会保留 `release-evidence.json` 和
`release-evidence.txt`。它们只包含不透明的运行身份、
来源水位哈希、新鲜度/过期时间、确定性回放、回滚、
兼容性和聚合门禁字段。它们绝不会包含 DSN、原始作用域、
查询、源文本、记忆/事件标识符、原始分数、凭据或 provider
载荷。缺失 DSN 属于受控跳过，而过期、被拒绝或
不兼容的报告无法授权激活。

## 稳定融合发布（rollout）与评估器运行手册

在单个精确 `X-Stele-Tenant`、`X-Stele-Project` 和 `X-Stele-Namespace` 作用域下，
以已认证管理员身份创建并操作排序发布（rollout）。首先
通过 `POST /v1/admin/ranking-rollouts` 创建选择；某个
作用域的策略绝不会仅仅因为头值相似就适用于另一个作用域。

以 `status=diagnostics_only` 搭配
`mode=diagnostics_only`，或 `status=dry_run` 搭配 `mode=dry_run` 开始比较。通过自有检索评估
基础设施，将默认 `rrf:rrf-v1` 与显式实验
`normalized_weighted:normalized-weighted-v1` 进行比较。这些模式保留当前已批准的检索行为
并只产生有界的授权发布（rollout）/评估证据。

在现有发布（rollout）门禁记录成功的
预演、已记录的归属、`satisfied` 的证据阈值、无
阻塞项以及新鲜的匹配发布证据证明之前，不要激活任何选择。使用
`POST /v1/admin/ranking-rollouts/{policy_id}/dry-run` 运行受控
比较，并用
`GET /v1/admin/ranking-rollouts/{policy_id}/impact` 检查有界结果。只有同时具备
`status=active_for_scope` 和 `mode=active_for_scope` 的策略才会改变真实搜索或
上下文检索，而且仅对其精确策略作用域生效。在门禁满足后，
通过 `POST /v1/admin/ranking-rollouts/{policy_id}/activate` 并携带
脱敏的 `evidence` 对象激活它。

要停止发布（rollout），请使用
`POST /v1/admin/ranking-rollouts/{policy_id}/disable`。要恢复部署
基线，请使用 `POST /v1/admin/ranking-rollouts/{policy_id}/rollback`。回滚
会将后续检索返回到先前已批准的基线，而不修改规范
记忆、派生分块、原始事件或来源记录；有界的发布（rollout）
遥测会将恢复记录为 `rollback_restored`。

### 发布证据新鲜度核对

激活移交仅在其精确作用域证据与当前来源水位、策略版本、夹具和
表示身份、新鲜度截止时间、证明以及回滚
证明保持兼容时才有效。scheduler 和 worker 通过持久的
`release_evidence_reconciliation` 作业核对这些条件。过期或不匹配的移交会被标记为
不合格，当前激活必须失败即关闭；原始证据
和每个核对结论都保持仅追加且可检查。

管理员可以使用
`GET /v1/admin/release-evidence/reconciliation` 检查有界的当前状态，并使用
`POST /v1/admin/release-evidence/reconciliation` 携带
`actor` 和 `reason` 请求作用域绑定的维护运行。触发会返回持久运行身份，且
不会直接激活策略。需要新的兼容证据移交给
恢复资格。响应和指标只包含不透明身份、
有界原因类别、新鲜度分桶和转换时间戳；它们
绝不会暴露原始证据、查询、provider 载荷、凭据或外部
作用域值。

仅使用在 `STELE_TEST_RETRIEVAL_EVALUATION_DSN` 中显式提供的一次性自有 PostgreSQL
DSN 运行评估器。切勿使用环境中的
`STELE_POSTGRES_DSN` 进行回放。如果缺少评估器 DSN，
`pwsh -File scripts/retrieval-evaluation.ps1` 会打印
`SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` 并以代码 `2` 退出；这属于
受控跳过，而不是通过的评估。

### 身份去重与多样性发布（rollout）运维

搜索和上下文装配在选取证据之前始终校验精确 `tenant/project/namespace`
作用域和活跃生命周期状态。默认路径
随后按规范记忆、源事件或
经过验证的父记忆谱系对等价候选去重。保留稳定融合顺序中的首项，
并进行有界的确定性引用合并。无效、隐藏、被抑制、
被遗忘、已删除、已过期或外部候选会在分组前移除，
且不能抑制可见证据。

已批准的多样性策略是可选的第二阶段选择器。它按
策略名称/版本进行版本化，并且只适用于其精确作用域。参数
是有界的（语义阈值、MMR/等价 lambda、候选和成对
比较限制、引用限制以及类别/会话/实体/时间覆盖
权重）。分区多样性在资格和摘要偏好之后运行，
但在现有 token/字符预算打包器之前，保留公开
分区名称、响应形状、引用和调用方预算。

使用现有的排序发布（rollout）路由治理此生命周期：

1. 使用 `POST /v1/admin/ranking-rollouts` 创建策略，始终提供一个
   精确的租户、项目和命名空间。来自其他作用域的策略绝不会
   被考虑。
2. 从 `diagnostics_only`/`shadow` 或 `dry_run` 开始。这些状态保留
   当前的身份去重行为，并且只发出有界的授权
   评估证据。
3. 使用
   `POST /v1/admin/ranking-rollouts/{policy_id}/dry-run` 运行受控比较，然后检查
   `GET /v1/admin/ranking-rollouts/{policy_id}/impact`。激活需要
   成功的预演、归属、满足的证据阈值以及无
   阻塞项。只有 `status=active_for_scope` 加上
   `mode=active_for_scope` 才会改变该精确作用域的普通检索。
4. 使用 `POST /v1/admin/ranking-rollouts/{policy_id}/disable` 禁用，或使用
   `POST /v1/admin/ranking-rollouts/{policy_id}/rollback` 恢复
   先前已批准的基线。回滚是
   仅追加的治理：它不会重写规范记忆、原始事件、
   派生分块、来源或融合状态，并记录
   `reason_code=rollback_restored`。

如果没有策略适用、策略格式错误/隐藏/禁用，或语义
嵌入缺少兼容的活跃修订，选择会安全降级到
身份去重基线。授权的诊断类别是有界的
（`semantic_unavailable`、`duplicate`、`omitted_by_diversity`、
`omitted_by_budget`、`invalid` 或 `selected`）；普通客户端不会收到
策略内部信息、相似度值、簇成员关系、候选池、
隐藏内容、外部标识符、作用域值或 provider 错误。

评估报告保留策略名称/版本以及带
重复率、受保护召回、证据覆盖、候选池大小和有界
延迟的聚合处置。跨作用域或隐藏生命周期泄漏属于硬性失败；
受保护召回、多跳覆盖、预算和延迟回退会阻止
激活。真实栈证据必须使用
`STELE_TEST_RETRIEVAL_EVALUATION_DSN` 中显式拥有的一次性
PostgreSQL + pgvector DSN。如果缺失，
评估器会发出 `SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` 并以 `2` 退出——这是
受控未通过跳过，绝不是通过——并且绝不会回退到
`STELE_POSTGRES_DSN` 或其他环境数据库。

### RQ4 上下文效率校准运维

上下文效率校准默认禁用。仅配置有界的
`STELE_CONTEXT_CALIBRATION_*` 值：摘要年龄、最小证据、
置信度阈值、衰减窗口、贡献上限、候选/上下文限制
以及经过时间限制。启用部署配置不会激活
策略；精确作用域发布（rollout）仍必须提供兼容的新鲜摘要。

先使用 `diagnostics_only` 或 `dry_run`/影子。这些模式只计算
脱敏的聚合效率证据，并返回字节等价的已批准
基线。仅当摘要属于精确作用域、
非陈旧、未被取代且高于配置的证据阈值时，才允许活跃校准。它只能在资格、时间过滤、去重和
多样性之后调整排序。它不能增加候选、扩展图遍历、更改引用、
创建响应分区或超出上下文预算。

禁用或回滚排序发布（rollout），以便在
下一个匹配请求时恢复基线行为。派生摘要可重建，源反馈、
任务评估、规范记忆、来源和审计历史绝不会
被覆盖。受保护召回、隔离/生命周期/引用检查、陈旧或
重复溢出、预算溢出、非确定性回放、陈旧摘要
以及回滚失败都属于硬性发布失败，即使每预算质量
有所提升。普通 OpenAPI/MCP 响应不会暴露校准权重、
反馈历史、候选池、原始指标、ID、作用域或策略字段。

- `api` 以结构化键值风格记录请求完成和恐慌恢复。
- `GET /livez`、`GET /readyz` 和 `GET /metrics` 为自托管编排提供进程存活、模式感知就绪和 Prometheus 风格运行时指标。
- `worker` 记录轮询循环失败和成功的批量执行。
- `worker` 还以结构化键值风格记录作用域证明步骤转换和记忆会话验证转换。
- `scheduler` 记录维护作业执行结果和退避重试。
- worker 会以有界重试状态持久化可重试的治理失败，而不是仅依赖租约过期。
- 达到重试上限的原始事件会被标记为耗尽，并停止自动认领，直到显式管理恢复动作介入。
- scheduler 调度路径独立于公开请求流量。
- 嵌入重建 scheduler 通过共享观察钩子记录积压、provider 探测、切换波次调度、执行遥测和错误路径。
- 活跃嵌入切换在正常生命周期发现遍历之前由 scheduler 以有界波次推进，因此 provider 迁移复用与漂移校正相同的持久重建执行路径。
- 摘要压缩和保留策略清扫按符合条件已发现的作用域分派，配置的默认作用域仅在发现未返回任何作用域时作为回退。
- 作业执行清理保持运行时全局，并按节奏窗口运行一次，而不是按已发现作用域扇出。
- 持久维护执行通过 `STELE_JOBS_DURABLE_MAINTENANCE_ENABLED=false` 选择性启用；启用后，作用域调度使用稳定身份、租约续订/回收、有界重试和检查点安全完成。一致性门禁失败会保留先前的 scheduler 行为。
- 遥测钩子点为摄取、治理 worker 执行、检索、遗忘、治理积压检查、派生洞察反馈、嵌入接纳决策、切换状态快照、provider 就绪探测、scheduler 波次调度、嵌入重建积压及执行检查、作用域证明运行和步骤、记忆会话运行和轮次以及记忆会话验证结果接线。默认运行时为 API 指标端点安装 Prometheus 风格的进程内指标观察器。

### 持久维护与一致性冒烟

仅在 PostgreSQL 18 + pgvector 冒烟数据库可用之后，才在已忽略的
`.env.local` 中设置维护变量。在作用域一致性证据变绿之前，保持持久路径
禁用：

```text
STELE_JOBS_DURABLE_MAINTENANCE_ENABLED=false
STELE_JOBS_MAINTENANCE_LEASE_DURATION=1m
STELE_JOBS_MAINTENANCE_LEASE_RENEW_INTERVAL=30s
STELE_JOBS_MAINTENANCE_MAX_ATTEMPTS=5
STELE_JOBS_MAINTENANCE_RETRY_BACKOFF=30s
```

冒烟必须验证重复触发抑制、租约续订与陈旧回收、检查点恢复、
投影水位新鲜度、脱敏有界遥测，以及绝不会以规范记忆或故障审计历史为目标的保留策略。
要回滚，请将持久标志设为 `false`；
现有 scheduler 作业继续通过先前已批准的执行路径运行，且不会重写任何规范
记录。

### 可选重排器配置

API、worker 和 scheduler 共享一份可选的 OpenAI 兼容重排器
配置。它默认禁用；请通过进程环境、Docker/Kubernetes 机密或已忽略的
`.env.local` 文件注入端点、API key 和
模型。仅保留 `.env.example` 占位符，切勿提交
凭据、DSN、评估数据或 provider 载荷。

```text
STELE_RERANK_ENABLED=false
STELE_RERANK_MODE=shadow
STELE_RERANK_PROVIDER=
STELE_RERANK_ENDPOINT=
STELE_RERANK_MODEL=
STELE_RERANK_API_KEY=
STELE_RERANK_TIMEOUT=30s
STELE_RERANK_MAX_CANDIDATES=50
STELE_RERANK_MAX_TEXT_BYTES=8192
```

### 查询自适应规划器配置

除非管理员创建带有兼容的版本化规划器束的精确作用域排序
发布（rollout），否则规划器不会改变普通结果。在自有
PostgreSQL + pgvector 证据满足检索发布门禁之前，服务将规划器保持在
`diagnostics_only` 或 `shadow`。
规划器策略值与发布（rollout）记录一起持久化；它们不是
环境级的进程全局开关。因此，当这些值缺失或不兼容时，现有部署继续使用
基线。

规划器的硬性信封有意受策略约束：最多两个
遍历、有限的总和每通道候选预算、有限的延迟
截止时间、有限的上下文条目预算以及预留的重排器余量。唯一的
后续操作是一次由证据触发的遍历，使用原始查询以及
相同的精确作用域和生命周期可见性。禁用或回滚发布（rollout）以
立即返回基线，而不重写规范记忆或派生
状态。

使用已认证的管理排序发布（rollout）路由创建、预演、
激活、禁用或回滚规划器策略。普通搜索/上下文
响应不会暴露规划器身份、查询族、计划、预算或
后续细节。授权的诊断只包含有界的聚合
类别和计数。如果缺少 `STELE_TEST_RETRIEVAL_EVALUATION_DSN`，
`scripts/retrieval-evaluation.ps1` 会返回
`SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED`（退出码 `2`），激活保持
阻塞；它绝不会回退到 `STELE_POSTGRES_DSN`。

### 可选的有界图遍历配置

图遍历是派生的 PostgreSQL 关系投影增强，而不是
图数据库或第二个事实来源。除非管理员将兼容的、版本化的图策略附加到
精确作用域的活跃检索规划器发布（rollout），否则它不会改变普通结果。
部署环境定义绝对信封；作用域策略只能降低它。启用的
策略在未显式选择深度时运行一跳。部署
最大值默认为三跳，策略绝不能提高它。要在保留作用域计划的同时禁用
扩展，请设置 `hops_set=true` 和
`max_hops=0`；省略 `max_hops` 是有意不同的，并保持
安全的一跳默认值。因此，有效的显式深度为零到三。

```text
STELE_GRAPH_MAX_HOPS=3
STELE_GRAPH_MAX_SEEDS=32
STELE_GRAPH_MAX_EDGES_PER_HOP=64
STELE_GRAPH_MAX_PATHS_PER_SEED=32
STELE_GRAPH_MAX_PATHS_PER_REQUEST=256
STELE_GRAPH_MAX_CANDIDATES=100
STELE_GRAPH_MAX_ELAPSED=250ms
```

每一跳都保持在调用方精确的租户/项目/命名空间、
授权、生命周期、有效时间和源版本约束之内。路径
仅是瞬态内部证明：普通搜索/上下文响应绝不会
包含路径、节点、边、标识符、跳数、距离、原始分数或
策略值。诊断和影子运行保留基线公开响应。
禁用或回滚精确作用域规划器策略以立即停止遍历；
不会重写任何规范记忆、投影、事实版本、来源或
审计历史。

对于拒绝可选 `dimensions` 请求字段的 OpenAI 兼容嵌入服务
（例如某些 SiliconFlow 部署），请设置
`STELE_EMBEDDING_OPENAI_SEND_DIMENSIONS=false`。Stele 仍会校验并
记录配置的目标维度；只会省略出站请求字段。

先使用仅诊断/影子模式，并针对一次性真实
PostgreSQL + pgvector 栈进行评估。活跃模式需要精确作用域发布（rollout），并
匹配逻辑 provider/版本/模式，以及现有的预演、证据、
归属和无阻塞门禁；不支持全局激活。禁用
或回滚会立即返回融合基线。Provider 错误、
超时、格式错误或超出作用域的结果以及边界违规会失败即关闭，
而不会使普通检索失败。管理/评估诊断和
`stele_retrieval_rerank_total` 保持低基数，并脱敏查询文本、
作用域值、记忆 ID、原始分数、端点 URL、API key、DSN 和原始
provider 载荷。

### 可选的 OpenAPI 支撑 MCP 适配器

MCP 接口默认禁用，且仅在 `api` 模式可用。
启用后，它在现有 OpenAPI 和
服务契约之上挂载 Streamable HTTP 端点；PostgreSQL 仍是唯一事实来源，适配器
不会创建第二个认证、配置文件或存储边界。

```text
STELE_MCP_ENABLED=false
STELE_MCP_PATH=/mcp
STELE_MCP_MAX_QUERY_BYTES=4096
STELE_MCP_MAX_PAYLOAD_BYTES=65536
STELE_MCP_MAX_RESULTS=50
STELE_MCP_MAX_IDS=100
```

MCP 数据工具需要持久管理员凭据；公开运行时
凭据会被拒绝，包括 `who_am_i`。公开 agent 使用
[集成指南](agent-runtime-memory-provider.md)中的 Provider 工作流，在被拒绝后
不能回退到 MCP 或原生读取。管理员工具可以在其有界参数中选择精确的
`tenant`/`project`/`namespace` 授权；
否则它必须提供服务器自有的运行时绑定头
`X-Stele-Runtime-Binding`。显式作用域始终优先，两条路径都不能
扩展主体的持久授权。`who_am_i` 只返回角色、访问
模式、是否为该请求验证了服务器自有的活跃作用域，以及
已验证精确作用域的不透明摘要；它不声称枚举所有
主体授权。凭据和原始作用域值绝不会返回。

首批工具被有意分开：`who_am_i`、`memory_search`、
`memory_context`、`memory_browse`、`memory_remember`、
`memory_forget`、`memory_forget_preview` 和 `memory_forget_apply`。
搜索/上下文委托给现有的时间、生命周期安全检索和
投影服务，并返回引用而不带原始排序诊断。
记住/更新请求是受治理的记忆意图。`memory_forget` 接受
一个显式记忆 ID，并要求现有的特权生命周期界面。
语义批量遗忘始终是预览 → 调用方审查固定 ID → 应用；
应用绝不会重新运行不受约束的语义查询。

### 受治理的记忆意图

对于公开凭据，受支持的写入是 `/v1/provider/intents`，只返回
有界的意图 ID/状态确认。原生意图读取和历史
需要管理员访问权限。来源证明和被接受的意图
都不会赋予指令、工具、编译或业务状态权威。

`POST /v1/memory-intents` 是 `remember`、
`update`、`forget`、`contradiction` 和 `feedback` 的 OpenAPI 优先写入边界。每个请求都绑定到
已认证的 `tenant/project/namespace` 作用域，并且必须包含 actor、
原因、请求身份和幂等键。`contradiction` 至少携带
两个作用域内证据引用；`feedback` 指定一个现有洞察目标。

提交会持久化不可变请求，并只将其稳定引用交给
其专用 PostgreSQL schema 中的 Riverqueue。它不会内联变更规范
记忆。意图接纳和入队是原子的；worker 治理
效果、转换历史和作业完成一起提交。其他
派生工作类型保留在其现有队列上。
等价重试返回原始请求身份；同一作用域幂等键下的不同规范化
载荷会被拒绝为冲突。

授权的运维人员可以读取 `GET /v1/admin/memory-intents/{intent_id}` 以及
`GET /v1/admin/memory-intents/{intent_id}/history` 上的有序脱敏转换账本。历史包含有界
状态类别和引用，绝不包含原始 provider 错误、凭据、
提示、声明或外部作用域数据。禁用策略会保留或拒绝
新工作，同时保留已提交请求及其审计历史；稍后的
兼容策略只能恢复符合条件的待处理工作。

自托管运行时使用一个由配置支撑的意图策略边界：
`STELE_MEMORY_INTENT_POLICY_ENABLED` 默认为 `true`，
`STELE_MEMORY_INTENT_POLICY_VERSION` 默认为 `v1`。更改任一值
都需要重启 API 和 worker 进程。禁用的 API 会拒绝新的
意图持久化；同样禁用策略的 worker 会让已排队的
工作保持可检查以便重试。使用相同的精确作用域和
兼容版本重新启用，以恢复 `accepted` 或 `pending` 工作。策略变更
不会删除意图行、转换、队列历史或规范版本。

Provider 更正更新通过
`provider-correction-governed-version-v1` 单独协商；在新 PostgreSQL + pgvector 的更正到后继、拒绝、回放、重启、
River 重试和旧版移交证据完成之前，请保持该能力禁用。PostgreSQL 迁移
`0043` 记录 River 作业关联，而 River 自己的迁移位于
`stele_river` schema 中。`auto` 应用两套迁移；`validate` 和 `off`
要求两个账本都与固定版本匹配。禁用该能力或回滚代码时，请保留 schema 和
已接受的意图历史。

SDK 契约测试无需外部 agent 即可运行。持久审查清单
测试和完整的真实栈一致性矩阵还只能
针对专用的、一次性的 PostgreSQL + pgvector 数据库运行；它们会应用
仓库迁移，并使用唯一夹具作用域，而不创建或
删除数据库。调用方拥有数据库的创建和移除，命令
绝不能指向运维、共享或生产数据库。

### 真实栈记忆意图产品验证

完整的一次性栈校验器还会通过 OpenAPI、PostgreSQL
以及 worker 和 scheduler 服务演练受治理的意图路径：

```powershell
pwsh -NoProfile -File scripts/stele-product-verify.ps1 `
  -ReportPath .tmp/memory-intent-conformance.json
```

该命令拥有唯一的 Compose 项目、生成的凭据和精确的
租户/项目/命名空间夹具作用域。它检查已接受的记住提交、
相同回放、冲突的幂等重用、外部作用域不披露、
作用域状态/历史、一个持久 `memory_intent` 队列引用，以及 worker
和 scheduler 重启恢复。它还会运行自有的持久工作租约、
检查点、重试和重试耗尽矩阵，并在矛盾/反馈审查夹具前后比较默认检索
和上下文哈希。
报告经过脱敏，只包含运行
和作用域哈希、有界阶段类别、兼容性标签、时间戳、
阶段/结果/类别计数的聚合摘要、恢复/回滚/清理
门禁观察以及可消费性裁决。它绝不会写入凭据、DSN、作用域、
意图/记忆/证据标识符、载荷或原始依赖错误。

设置 `STELE_PRODUCT_VERIFY_PROVIDER_CONTEXT=1` 可在校验器自有的栈上包含公开 Provider
上下文 schema、受治理摄取、精确路径/前缀、隔离、生命周期、
回放以及 API/worker 重启门禁。它会保留
单独的有界 `<ReportPath>.context.json`；缺失或失败的上下文证据
会使这个已启用的门禁失败。独立的全新栈命令和公开契约
迁移说明见 [Provider 指南](agent-runtime-memory-provider.md#public-live-verification)。

设置 `STELE_PRODUCT_VERIFY_MCP=1` 可包含可选 MCP 前置
检查，并在配置的端点启用时，运行聚焦一致性命令所使用的相同 PostgreSQL
MCP 矩阵。该矩阵覆盖
`memory_remember` 回放、只读变更拒绝、`memory_forget_preview`、
`memory_forget_apply`、应用回放、精确作用域隔离以及通过共享意图和生命周期服务的响应
脱敏。禁用的 MCP
适配器被记录为 `skip`，不会成为就绪声明。

缺少 Docker、PostgreSQL/pgvector、迁移兼容性或服务
就绪属于显式跳过或失败。如果回滚策略夹具或
另一项硬性门禁无法运行，报告会记录降级证据，不能
用作就绪或启用声明。清理限于生成的
Compose 项目和夹具资源；命令不得指向运维
或生产数据库。

聚焦矩阵演练实际的 Streamable HTTP 传输，并为
能力发现、身份、精确作用域隔离、
运行时绑定优先级、只读授权、检索/上下文/浏览、
时间与生命周期过滤、精确路径与路径前缀、记住
幂等性、受预览约束的遗忘、应用回放/冲突、响应
脱敏以及适配器禁用报告有界类别。它通过正常的摄取、
接纳、提升和生命周期边界播种，并且只移除其创建的
记录。当设置 `STELE_MCP_CONFORMANCE_EVIDENCE` 时，测试会写入
允许列表中的 JSON 产物，其中仅包含 schema 版本、类别名称、
状态和检查计数；DSN、凭据、作用域、查询、载荷和
夹具内容绝不会写入。

对于显式选择性启用运行，请使用以下包装脚本。未配置 DSN 时它返回跳过（退出码
2），配置的类别失败时返回失败（非零退出码）。
`-ApiBaseUrl` 是可选的；提供时，包装脚本会验证 `/health`
和 `/openapi.yaml` 仍然可用。它预期
`/mcp` 默认失败即关闭；对于已启用的适配器，请传入 `-ExpectEnabled`（以及配置时的 `-McpPath`）：

```powershell
$env:STELE_TEST_POSTGRES_MCP_DSN = '<operator-managed-dsn>'
pwsh -NoProfile -File scripts/stele-mcp-conformance.ps1 `
  -EvidencePath .tmp/mcp-conformance-evidence.json `
  -ApiBaseUrl http://localhost:8080
Remove-Item Env:STELE_TEST_POSTGRES_MCP_DSN
```

产品校验器为启用的路径拥有 PostgreSQL DSN，并在清理期间移除
临时 MCP 证据产物。失败的启用矩阵被
报告为有界一致性失败；它绝不会把产品报告
变成启用或就绪声明。

底层聚焦测试仍可直接使用：

```powershell
go test ./internal/storage/postgres -run '^TestMCPForgetLedgerPostgresPersistsReviewAndReplayAcrossRepositories$' -count=1
go test ./internal/storage/postgres -run '^TestMCPPostgresConformanceMatrix$' -count=1
```

现有账本测试继续证明跨仓库重建的预览持久性、
持久应用回放、冲突键拒绝以及
底层 PostgreSQL 记录。两个测试都会为其创建的每条记录执行定向清理，
但迁移会保留，且无法假设失败进程的清理；
请保持数据库一次性。一致性包装脚本
有意采用选择性启用，不会更改默认单元测试、产品验证或
CI 命令；发布作业可以在准备好隔离的
pgvector 数据库后调用它。

要禁用适配器，请设置 `STELE_MCP_ENABLED=false` 并重启 API 副本。
随后 `/mcp` 端点返回未找到，而普通 OpenAPI/provider
路由和规范 PostgreSQL 状态保持不变。在没有与 API 表面相同的网络控制时，不要暴露 MCP
监听器。

## 派生工作队列持久性

反思、压缩、投影重建、洞察维护和新鲜度工作
共享一个有界队列。生产默认值应使用 PostgreSQL 持久
模式；`memory_buffer` 是显式的低延迟模式，其中未刷新的派生
工作可能在溢出、刷新失败或进程重启时丢失。原始事件、
规范记忆版本、审查决策、检查点和已提交的
压缩证据绝不会经过这条有损路径缓冲。

队列模式在进程启动时选择。从内存模式切换到持久模式
需要先排空并记录丢失报告；从持久模式切换到
内存模式会让现有 PostgreSQL 工作保持可认领，且不会将其复制到
缓冲区。使用管理端点检查精确作用域状态：

```powershell
curl.exe -H "X-Admin-API-Key: $env:STELE_ADMIN_API_KEY" `
  -H "X-Stele-Tenant: tenant-a" -H "X-Stele-Project: project-a" `
  -H "X-Stele-Namespace: default" `
  http://localhost:8080/v1/admin/derived-work/status
curl.exe -H "X-Admin-API-Key: $env:STELE_ADMIN_API_KEY" `
  -H "X-Stele-Tenant: tenant-a" -H "X-Stele-Project: project-a" `
  -H "X-Stele-Namespace: default" `
  'http://localhost:8080/v1/admin/derived-work?limit=20'
```

状态响应只暴露有界的模式、深度、重试、终态、丢失
和新鲜度类别。详情页使用不透明游标和精确作用域
谓词；外部作用域既不会披露队列是否存在，也不会披露计数。重试
耗尽属于终态，直到授权恢复动作显式重新排队
该工作。

<a id="procedural-verification-maintenance"></a>
## 程序性验证维护

程序性验证维护默认禁用。它是仅限管理员、精确作用域的工作流，用于记录对
当前活跃程序性记忆版本的可归属检查、检查脱敏历史、排队
有界重新验证，并将被反驳的更正移交给现有受治理
更新管道。它不新增 Provider、普通检索或 MCP 公开
运行时访问，也绝不会直接更新规范记忆版本。

仅在每个 `api`、`worker` 和 `scheduler` 部署模式都验证了迁移和管理员授权
就绪之后才启用它：

| 变量 | 默认值 | 含义 |
| --- | --- | --- |
| `STELE_PROCEDURAL_VERIFICATION_ENABLED` | `false` | 为配置的作用域启用接纳、调度、特权检查和更正移交。 |
| `STELE_PROCEDURAL_VERIFICATION_POLICY_VERSION` | `procedural-verification-v1` | 与不可变断言一起记录的服务端选择的策略身份。 |
| `STELE_PROCEDURAL_VERIFICATION_BATCH_SIZE` | `10` | 单次轮询认领的最大有界重新验证运行数。 |
| `STELE_PROCEDURAL_VERIFICATION_MAX_ATTEMPTS` | `3` | 终态结算前的最大持久重新验证尝试数。 |
| `STELE_PROCEDURAL_VERIFICATION_LEASE_DURATION` | `1m` | 单个重新验证 worker 认领的租约时长。 |
| `STELE_PROCEDURAL_VERIFICATION_RETENTION` | `720h` | 清理使用的保留边界；它不会重写规范历史。 |

使用 `/v1/admin/procedural-memory-verifications` 及其文档化的 OpenAPI
动作，配合持久管理员密钥和所有精确作用域头。默认
回执是脱敏的：不要将原始证据、更正内容、worker
租约身份或作用域数据放入仪表板或指标标签。逐字稿/模型
断言属于来源，而不是独立佐证；任务结果是
观察性证据，不是变更权威。

在启用之前，请针对自有的 Docker PostgreSQL + pgvector 依赖运行
`pwsh -NoProfile -File scripts/stele-procedural-verification-verify.ps1`。
该脚本只发出有界的阶段/状态报告，并移除其随机命名的
容器。**回滚**：在所有模式中设置
`STELE_PROCEDURAL_VERIFICATION_ENABLED=false` 并排空当前
worker；保留不可变的验证、运行、意图、候选、后继
和规范历史。不要向下迁移、删除证据或重写记忆
版本作为回滚。
