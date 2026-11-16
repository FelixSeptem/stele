# 关键组件

**角色：** 维护者参考。请从[文档门户](README.md)开始了解系统概览与贡献路径。

本页把公开架构映射到仓库的实现区域。当本指南与实现不一致时，以被引用的 Go 包与契约为准。

## API 运行时

- `internal/app`：HTTP 装配、依赖组合、健康/就绪检查、管理员路由、Provider
  路由，以及运行时模式启动。
- `openapi`：OpenAPI 优先的 schema 与路由发布。
- `internal/auth`：API key/主体认证与角色校验。
- `internal/provider`：运行时绑定、Provider 能力、受治理适配器，以及同步游标契约。

API 在访问存储层之前解析身份与精确作用域。它不应为某个适配器再实现一套持久化或授权。
`GET /v1/admin/assurance/ingestion-freshness` 使用已认证的精确作用域上下文与不透明事件
fixture 引用；PostgreSQL 关联治理、规范记忆与成功的 Provider 读取观测。只保留脱敏报告，
fixture 身份由作用域内唯一键保护。

## 存储与记忆治理

- `internal/storage/postgres`：PostgreSQL 仓储、迁移、投影、仅追加历史、租约与作用域谓词。
- `internal/memory`：记忆类别、规范版本、意图、会话、生命周期输入、校验与服务级契约。
- `internal/jobs`：受治理记忆意图的 River worker 适配器；它只携带意图 ID，并把事务
  所有权保留在 PostgreSQL 仓储边界内。其他派生工作种类仍留在既有 worker 上。
- `internal/policy`：遗忘与保留策略决策。
- `internal/governance`：原始事件治理、声明、恢复与审计。

存储方法必须应用精确的 `tenant/project/namespace` 谓词，并返回安全的未找到或拒绝结果，
且不得确认外部记录是否存在。

## 检索与上下文

- `internal/retrieval`：词法、语义、混合、融合、重排、上下文装配、引用、时间约束与
  有界诊断。
- PostgreSQL 全文检索提供词法路径。
- 配置后，pgvector 提供语义检索。

检索输出是 agent 响应的证据，不是规范记忆，不得原样写回并当作新事实。

`internal/retrieval` 还负责仅用于评估的任务适配派生表示比较：精确摘录加检索、
主题投影加检索、层级/父级优先证据加检索。内部评估器与
`internal/storage/postgres` 表示报告仓储只接受脱敏的聚合、精确作用域记录。它们不注册
Provider、MCP、普通检索或 HTTP 路由；`flat-fusion-v1` 仍是普通检索选用的策略。
参见[检索表示实验](retrieval-progressive-context-experiments.md)。

对于公共 `context-budget-v1`，检索在既有上下文选择流程内负责可选的最终投影准入。
`internal/provider` 对已授权的模式相关条目 DTO 计价，解析独立的调用方/服务端
条目/内容/正文/引用下限，并在 `context-packing-v1` 下保留有界的计费预留空间。它最终
确定必需的封闭 requested/effective/usage/selection 计费结果，以及精确自计数的 JSON
大小。`internal/app` 写出这些最终字节且不追加换行；它不会再次运行任何打包器。
`internal/config` 解析条目默认 100、内容默认继承响应上限 1 MiB、引用默认 100 及其硬
边界；非法配置会导致启动失败。`openapi` 发布可移植整数、仅在响应中出现的省略 null、
固定身份与类型化的有界错误。

强制信封预检与最终拒绝返回 413，且不产生部分投影。当前精确作用域的披露/来源/版本
资格判断先于预算原因；被拒绝或隐藏的证据不贡献任何私有计数器。原生装配保留原有默认值，
公共可选段保持关闭。`internal/assurance` 独立于排序指标与已归档的 PC1/PC2 就绪证据，
校验实际序列化用量以及新的初始/重启证据。使用方必须发现 PC3 并迁移严格 schema；
回滚会先禁用依赖访问，再移除扩展。分配单位、零值/省略、错误与兼容性见
[Provider 契约](agent-runtime-memory-provider.md)。

## Worker 与 scheduler

- `internal/jobs`：worker 循环、scheduler 派发、租约、重试/退避、维护作业与持久化运行历史。
- `internal/workqueue`：持久化派生工作队列与检查点契约。

Worker 负责异步整合与恢复。Scheduler 决定何时派发有界工作；它不直接修改 API 授权状态。
新鲜度采集读取 worker 时间戳与生命周期状态；它不领取或重试治理工作。

## MCP 适配器

- `internal/mcp/schema.go`：工具名、请求结构、校验限制与工具描述符。
- `internal/mcp/adapter.go`：Streamable HTTP MCP 服务器、请求作用域解析、读取工具、
  受治理记忆写入与生命周期流程。
- `internal/mcp/contract_test.go`：协议级行为与脱敏覆盖。

当前工具记录在 [MCP 集成指南](integrations/mcp-agent-skill.md)中。MCP 仍是可选项且默认
禁用；OpenAPI 仍是主要服务边界。

## Provider 适配器与同步

增量式 PC5 持久引用能力在既有受治理记忆谱系之上暴露精确的
task/run/handoff/compilation 引用状态与查询。它与上下文独立协商，不拥有业务状态。

PC6 新增专用封闭投影/历史处理器与 DTO、事务性仅追加仓储、迁移 0037 以及作用域内的
scheduler 反馈作业。`internal/assurance` 独立校验所提供投影的 schema 与语义。准入、
当前 PC5 资格与后续反馈影响相互独立；[维护细节](provider-evidence-projections.md) 覆盖
它们的版本化、边界与恢复。

Provider 适配器组合既有的 event、intent、retrieval、context、lifecycle 与 status 服务。
更正更新增加一条可选加入、由服务端归属的路径，把更正后的投影转换为普通的受治理更新
意图；它们不会产生直接的规范写入。同步契约增加有界初始快照、有序回放、游标确认与显式的
`resync_required` 恢复。它不会创建客户端自有的记忆存储。

参见 [agent 运行时 Provider](agent-runtime-memory-provider.md) 以及
`openspec/specs/runtime-capability-and-event-sync-contract/spec.md` 中与传输无关的同步契约。

## 遥测与保障

- `internal/telemetry`：低基数指标与有界生命周期事件。
- `internal/assurance`：一致性 fixture、健康/就绪证据、运维证明与恢复验证。

遥测不得包含原始查询、负载、作用域、凭据、游标或无界标识符。保障证据与检索质量指标
相互独立。

## 上下文披露与来源信任

公共运行时凭据需要显式的精确作用域披露配置。Provider 上下文默认返回结构化引用；有界内容
需要 content profile 与显式模式。不受支持的 native/MCP 数据路由会在查询前拒绝公共凭据。
来源标签贯穿治理与历史持久保存；缺失谱系的数据视为不可信/受限。已证实的来源数据不授予
任何指令、工具、写入或业务状态权限。端点行为、管理、版本化以及升级/回滚见
[披露/来源指南](context-disclosure-and-source-trust.md)。

### Provider 上下文摘要与连续性

Provider HTTP 组件只在披露、选择与精确正文计费完成之后才最终确定 `context-digest-v1`。
其摘要规范化器是版本化且按域分隔的；独立的保障组件会重新计算摘要，而不复用检索、排序或
打包代码。迁移 `0036_provider_context_continuity` 增加的 PostgreSQL 回执组件只存储不透明
引用、完整绑定身份、规范化请求指纹、摘要标识与权威的签发/过期时间。scheduler 执行有界的
仅过期回执清理。它不会删除活跃回执，保留的过期行在清理前仍受容量约束。没有任何组件把上下文
响应当作比较的实现：比较会读取当前授权数据并返回完整的新正文。

## 程序性验证维护

`internal/memory` 拥有封闭的程序性验证处置、证据、归属与更正准入契约。它要求存在当前活跃的
procedural 目标以及精确作用域内合格的证据。其仅追加台账不是并行的记忆存储：
**PostgreSQL 仍是记录系统**，规范版本继续通过受治理意图流转。

`internal/storage/postgres` 持久化作用域内的验证行、持久化再验证运行，以及有界的
验证→意图→后继谱系。`internal/jobs` 派发并恢复 `procedural_revalidation` 工作，带有租约、
尝试次数与保留边界。该 worker 只在记录的来源已无法支撑该声明时，写入机械性的
`inconclusive` 或 `needs_revalidation` 结果；它永远不会提升某个流程，也不会写入更正。

`internal/app` 与 `openapi` 发布独立的、仅限管理员的 HTTP 界面。API、worker 与 scheduler
组合相同的策略、仓储、派发器、遥测、清理与脱敏检查依赖。Provider、普通检索与 MCP 公共
运行时组件不依赖此界面。
