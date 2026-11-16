# Provider 证据投影维护

PC6 增加独立协商的 `turn-outcome-feedback-projection-v1`
能力。PostgreSQL 仍是唯一的系统记录。这些操作
记录运行时报告的证据；Stele 不完成 Danny 任务、不释放
租约、不提交世代、不发布工件，也不确认投递。

## 准入与检查

要求 `projections.enabled=true`、精确的契约版本以及 `GET /v1/provider/capabilities` 中受支持的操作。
基础 `provider-v1`、PC4
上下文回执与 PC5 持久引用都不意味着 PC6 可用。
严格消费者在该能力缺失时会失败即关闭，并验证所提供的
OpenAPI；通用 turn/outcome 路由为兼容性而保留，且
绝不是投影准入的替代品。

两个操作都需要 API 密钥、精确的 tenant/project/namespace 请求头、
`X-Stele-Runtime-Binding` 与 `X-Stele-Runtime-Session`。归属派生自
当前完整绑定，包括 principal、agent、conversation 与 Provider
实例。使用与 PC5 相同的 ASCII 作用域限制（每个维度 128 字节）。
准入需要当前读写授权，且其默认披露 profile
允许仅引用响应；`memory-reference-v1` 与
`memory-content-v1` 都支持该模式。history 允许只读授权，且
具有相同的披露约束。缺失/外部绑定与
作用域变更会在仓库分派之前失败。

`POST /v1/provider/projections` 返回 HTTP 202。其封闭 JSON 信封为：

```json
{
  "metadata": {
    "request_id": "report-1",
    "operation_id": "report-1",
    "schema_version": "provider-v1",
    "idempotency_key": "effect-42-revision-1"
  },
  "input": {
    "contract_version": "turn-outcome-feedback-projection-v1",
    "kind": "outcome",
    "effect_id": "effect-42",
    "source_revision": 1,
    "occurred_at": "2026-10-16T00:00:00Z",
    "reported_outcome": "succeeded"
  }
}
```

种类为 `turn`、`outcome` 与 `feedback`；报告的结果为 `succeeded`、
`failed`、`cancelled` 与 `unknown`。请求可以包含现有的
`session_run_id` 与 `turn_id`、`context_digest` 与 `context_reference`、
`references`（包含 PC5 `event_id` 选择器）、`provenance` 令牌、
`feedback`。请阅读权威 OpenAPI 以了解封闭枚举。提供的
轮次 ID 需要其会话运行；它们必须与绑定的会话以及
现有的精确作用域轮次匹配。报告的结果提供有界完成
上下文，绝不是业务状态权限。任意 task/lease/commit/
publication/delivery 字段、调用方作用域与授权覆盖都会被拒绝。

请求允许 32768 个编码 JSON 字节，规范元数据 4096 字节，
响应 65536 字节，八个唯一引用与八个唯一来源
令牌。effect/provenance 令牌使用 ASCII 字母、数字、`_`、`.`、`:` 与
`-`，最多 128 字节。来源修订为 1–2147483647 的整数；
发生时间为 UTC RFC3339 秒。反馈接受 `useful`、
`irrelevant` 或 `needs_review`，并需要必填的有界 `reason_code`（64
字节），且至少需要一个引用。空、null、重复、未知、
别名或格式错误的字段会失败即关闭；不会静默截断。
省略未使用的可选字符串，而不是提供空字符串。元数据
必须已经是规范的：周边令牌空白与需要
规范化的路径会被 PC6 拒绝。旧 Provider 元数据行为保留。
零时间戳 `0001-01-01T00:00:00Z` 会被拒绝。编码元数据与
Unicode 路径字节限制由服务与严格消费者强制，
此外还有 OpenAPI schema 的字段约束。

响应包含回显的元数据、契约版本、精确作用域以及一个
`record`：逻辑/投影 UUID、来源修订、可选的 `supersedes_id`、
`accepted|corrected` 处置、`evidence_kind=runtime_reported_evidence`、
引用快照、当前引用状态、上下文摘要归属以及
`feedback_state=none|pending`。不返回原始结果负载、记忆内容、反馈
文本或业务状态回执。

## 回放、更正与当前可见性

一个逻辑效果由完整绑定、种类与效果 ID 共同拥有。复用
幂等键并给出等价的规范化输入会返回原始
投影身份与处置，即使之后发生了更正或重启。
传输元数据不是语义输入；引用/来源顺序会被
规范化。同一键下的不同输入会冲突。更大的来源修订
会追加一个链接到前一个头的更正；相同修订会冲突，
延迟的低修订属于陈旧。重放较旧的已接受投递绝不会
覆盖当前头或创建重复反馈。

`POST /v1/provider/projections/history` 接受相同的元数据信封加上
`input.contract_version`、`kind`、`effect_id` 与必填 `limit`（1–8）。它
返回最新优先的 `records` 与 `has_more`。这是一个有界检查
窗口：`has_more=true` 并不声称完整历史都在其中。这些
Provider 操作中没有携带内容的历史快照或更广泛的
管理绕过。

`references` 记录准入归属；`current_references` 在当前授权与生命周期下
重新检查 PC5。已接受/待处理/失败的记录
不能声称反馈资格。已完成记录携带规范 UUID
谱系与 `eligible=true`。缺失、外部、隐藏、被抑制、被遗忘
与已删除的来源共享 `unavailable`，且没有事件 ID 或谱系。读取
还会在当前引用不可用时脱敏以前可见的准入身份。裸上下文摘要为 `reported`；
只有属于完整绑定且摘要匹配的未过期 PC4
回执才是 `verified_receipt`。两种状态都不授予任务完成或内容访问。
当该回执是轮次绑定的（Provider
上下文读取中存在 `result.turn_id`）时，投影必须提供完全相同的 `turn_id`；省略或不同的值会变为
不可用，而不是已验证回执。
该检查仅增加关联。它不授予角色、不扩大检索，
也不暴露上下文快照、隐藏候选或排序细节。

## 异步反馈策略

在调度器以及 API 副本中启用 `STELE_PROVIDER_ENABLED=true`，
以运行有界的精确作用域 `provider_projection_feedback` 作业。它每次分派
在每个作用域中最多扫描 100 个逻辑效果，并记录策略
`projection-feedback-v1`。投影写入会将反馈留为 `pending`；该
字段描述准入，而不是处理或排序激活的承诺。
所有被引用的证据必须当前已完成且合格，作业才能
将最新修订映射到现有的有用性反馈日志中，并带有
不可变的投影/来源/版本链接。待处理、失败与不可用
证据会被保留，但不会产生活动的反馈影响。

作用域发现会读取投影账本，包括只有待处理
投影且没有规范记忆的作用域。它优先处理未检查的作用域，然后是最旧的
最后服务检查点，同时保留配置的作用域批次限制
与每个作用域 100 个效果的限额。持久化检查点在
仓库重启后保留轮换；一个作用域中未检查的积压不会垄断批次。

活动的反馈消费者会通过 `eligible_usefulness_feedback` 重新检查当前绑定/授权、来源元数据、
规范生命周期与精确谱系。
更正与撤销会在下一次作业扫描之前排除旧证据。
该作业会增加普通反馈取代审计记录；恢复可见性
不会重新激活已被取代的修订。稍后有效的来源修订可以
提供新证据。现有的摘要、排序策略与洞察激活
门控仍然必需；准入与映射不会激活它们。

## 错误与操作

| HTTP | 类别 / 代码 | 客户端操作 |
| --- | --- | --- |
| 400 | `validation` / `invalid_request`、`invalid_scope` 或 `invalid_projection_scope` | 修正封闭输入或不兼容的绑定 |
| 403 | `authentication` / `unauthorized`；`scope` / `scope_denied`、`runtime_binding_denied` 或 `disclosure_denied` | 解析当前权限；不要扩大作用域 |
| 409 | `compatibility` / `unsupported_projection_contract` 或 `unsupported_schema` | 协商受支持的版本；不做通用回退 |
| 409 | `conflict` / `projection_conflict` | 保留原始键；使用更大的修订与新键提交真正的更正 |
| 409 | `stale` / `projection_stale` | 检查当前历史；不要覆盖更新的证据 |
| 410 | `lifecycle` / `projection_unavailable` | 同等对待缺失/外部/过期证据 |
| 413 | `validation` / `projection_limit_exceeded` | 在准入之前缩减请求 |
| 503 | `dependency` / `projection_dependency_unavailable` | 使用相同键重试等价投递 |

两个 PC6 路由都会为主体、
作用域与运行时绑定拒绝以及操作错误返回 JSON `ProviderProjectionErrorResponse` 信封。披露或
授权依赖也可能返回 `disclosure_unavailable` 或
`authorization_unavailable`，类别为 `dependency`，HTTP 503。响应
绝不包含底层存储错误，也不区分缺失与外部
绑定。现有路由保留其既有的认证信封。
该信封与嵌套错误对象都是封闭的。所有四个错误字段
（`category`、`code`、`message`、`retryable`）都是必填；消息非空
且限制为 256 个 UTF-8 字节，编码后的信封限制为 4096
字节。严格消费者验证状态/类别/代码组合，并且
仅对依赖错误接受 `retryable=true`。

迁移 0037 是增量式的：不可变投影身份/修订与
反馈来源链接、可变调度检查点、作用域约束以及
当前资格视图。在启用兼容的 API/调度器副本之前，应用官方迁移并验证校验和
账本。不新增依赖、SDK、
UI、队列或第二套业务状态数据库。回滚会为消费者禁用 PC6
路由/能力，同时保留投影、反馈与审计
历史；down 迁移刻意拒绝破坏性移除。

全新栈就绪要求在自有
PostgreSQL + pgvector 栈上运行官方 Provider 验证器，针对所提供的
OpenAPI 发送序列化公共请求，检查精确作用域拒绝、回放/冲突/更正、PC4/PC5 归属、
生命周期排除以及 API/worker 重启。报告包含有界计数、
类别、布尔值与来源哈希，绝不包含凭据、DSN、作用域
取值、引用值或原始响应体。被跳过的测试与仅 HTTP 验收
无法确立 PC6 就绪。

针对从被检查源码构建的镜像运行官方验证器：

```powershell
pwsh -NoProfile -File scripts/stele-provider-context-verify.ps1 `
  -Image stele-pc6-verify:local `
  -ProjectionReportPath .tmp/pc6/projection.json
```

默认的自有模式会创建全新栈与官方凭据，停止
worker 与调度器以观察公共待处理归属，重启 API，
恢复普通消费者，并在 API/worker
重启后重复稳定检查。它只移除其自有栈、卷与临时凭据。
PC6 报告及其 `.pending.json`、`.initial.json` 与 `.restart.json`
快照与 PC4/PC5 报告分离。失败或不完整的阶段会写入
未通过报告；另一个契约的通过报告不能确立 PC6
就绪。`-UseExistingStack` 需要显式的自有栈凭据与
观察 DSN，并且其本身不能确立全新栈来源。

<a id="correction-to-version-handoff"></a>
## 更正到版本的移交

单独协商的 `corrections` 能力仅当
服务同时具有更正解析器与已启用的记忆意图策略时才可用。
能力文档仅当两者都就绪时才公布 `corrections` 操作令牌；
其嵌套的 `operation` 字段命名精确路由。严格
运行时验证整个文档，并在调用
`provider-correction-governed-version-v1`（位于
`POST /v1/provider/corrections`）之前要求已启用的契约、
操作令牌与版本；它不会复用通用意图证据作为
更正所有权的证明。封闭请求命名一个更正后的
`projection_id`、目标记忆 UUID 与 `expected_version`，以及有界内容
与原因。精确绑定、投影头所有权、当前 PC5 引用
资格与目标谱系都会在意图
幂等或队列准入之前由服务端解析。

公共确认仅暴露受治理的意图 UUID、状态以及一个
固定的已验证归属状态。它绝不返回来源负载、事件 ID、
候选 ID、排序细节或目标谱系。缺失、外部、首个、
隐藏与目标不匹配的来源会归并为 HTTP 410
`lifecycle/correction_unavailable`；陈旧的来源/目标为 HTTP 409
`stale/correction_stale`；键冲突为 HTTP 409
`conflict/correction_conflict`。授权或存储依赖会返回
可重试的 HTTP 503。兼容客户端会以相同键重放等价请求，
并且在拒绝时不会切换到通用意图、直接 SQL、备用作用域或
本地变更。

已接受的意图保持异步。worker 会重新验证来源与目标，
然后使用现有的候选/提升路径追加一个后继版本
与不可变来源。它不会覆盖先前版本或
完成外部任务。Riverqueue v0.44.1 仅用于记忆意图
执行：作业参数携带意图 UUID，准入在意图事务中插入 River 作业，
并且治理写入、意图转换与
`JobCompleteTx` 在一个事务中提交。失败的尝试会回滚全部
三者并保持可重试。其他派生工作类型继续使用
现有队列。专用 `stele_river` schema 按正常的
PostgreSQL 迁移策略迁移；validate/off 启动要求该 schema 处于
固定的 River 迁移版本。旧的可恢复记忆意图队列行
会以事务方式移交一次，而终态历史行不会被
重放。

迁移 `0042` 存储不可变更正归属，`0043` 记录
River 移交。回滚会禁用该能力与 worker 路由，但保留
迁移历史与已接受数据；不要删除 River schema 或
将已完成历史重新入队作为运维回滚捷径。完整就绪
仍需要针对更正提升、
取代、重启、重试与移交的自有全新栈证据；仅成功准入不是
发布门控。

运行 `pwsh -File scripts/stele-provider-correction-verify.ps1` 以执行
自有的 PostgreSQL 18 + pgvector 更正、River 重试/移交、投影
取代以及 API/worker 重启夹具。该脚本会启动并移除其
自有临时容器，并且只将有限的套件状态与镜像摘要
写入其报告；它不保留 DSN 或请求数据。
