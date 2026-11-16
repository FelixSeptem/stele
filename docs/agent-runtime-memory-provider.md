# Agent 运行时记忆 Provider

**角色：** 公共 Agent 运行时集成的参考。请从[文档门户](README.md)开始了解集成概览。Provider 的拒绝不授权回退到 MCP、原生读取、备用凭据、本地记忆、独立排序或上下文重建。

Stele 为外部 Agent 运行时提供可选启用、由 OpenAPI 支撑的记忆 Provider 契约。Provider 是一个服务适配器：它不执行 Agent、不构建提示词、不调用模型，也不维护第二套记忆存储。

## 启用与发现

在迁移达到 `GET /version` 所报告的 schema 版本之后，在所有 API 副本中设置
`STELE_PROVIDER_ENABLED=true`。默认值为 `false`。使用
`STELE_PROVIDER_SCHEMA_VERSIONS=provider-v1` 配置受支持的契约版本；请求、结果、引用与
元数据上限使用 `.env.local.example` 中列出的 `STELE_PROVIDER_MAX_*` 变量。

客户端在初始化运行时之前会读取有界的能力文档。该文档公布受支持的操作名、精确作用域维度、版本、
限制以及 OpenAPI 摘要。它绝不公布凭据、DSN、主体记录、作用域取值、积压数据、Provider 负载或内部错误。

能力文档还会公布与传输无关的同步契约。首个启用的传输方式是 `openapi_pull`；WebSocket 与 SSE 保留给
未来的适配器，本版本不会启用。同步契约会公布事件种类、最大批次与快照大小、游标大小以及回放保留窗口。

## 运行时作用域握手

运行时初始化需要一个处于活动状态的持久主体，以及一项精确的
`tenant/project/namespace` 授权。请求将 `agent_id`、
`session_id` 与可选的 `conversation_id` 分开。Stele 返回一个不透明的运行时
绑定，以及一个由服务端生成的 Provider 实例标识。此后每个操作都携带该绑定与会话身份；服务在
访问仓库之前会重新校验绑定、有效期、主体与精确授权。
更改作用域请求头或身份会返回通用拒绝，且绝不披露是否存在外部记录。

在由 PostgreSQL 支撑的部署中，运行时初始化会以原子方式持久化绑定，并确保其会话记录存在于该精确作用域中。这
使得第一次 `POST /v1/provider/turns` 无需单独调用原生会话创建即可创建由服务端签发的轮次。仅当现有会话 ID 的租户、项目与命名空间均匹配时才会复用；
跨作用域冲突会拒绝引导并且不持久化绑定。

## 公共检索与轮次投影

`POST /v1/provider/retrieve` 与 `POST /v1/provider/turns` 需要与 Provider 上下文相同的
运行时绑定/会话请求头，以及当前的精确作用域披露授权。检索使用封闭的 `ProviderRetrieveInput`；路径与路径
前缀选择器会在绑定内部收窄候选，且不能扩大作用域。生效的 profile 默认为当前授权所配置的 profile，且
响应模式默认为 `reference_only`。

```json
{
  "metadata": {"request_id":"retrieve-1","operation_id":"retrieve-op-1","schema_version":"provider-v1"},
  "input": {"query":"deployment plan","path":"compilations/run-1","top_k":5}
}
```

仅引用结果包含处于活动状态、同一作用域的低/中等级引用、来源
元数据、版本，以及由返回匹配推导出的引用。它们省略
内容、分数、诊断、原始查询、隐藏候选以及任意
引用身份。返回内容既需要被允许的 `memory-content-v1`
profile，也需要显式 `response_mode=bounded_content`；仅引用授权
无论在 profile 还是模式未获授权时，都会收到相同的受限 `403 disclosure_denied`。不带 `source_metadata` 的事件摄取会被接受，但
默认为 `untrusted/restricted`；治理仍可能提升该事件，
但它不会出现在公共检索或上下文结果中。仅当应用的数据策略允许
相应披露时，才提供显式的 `low` 或 `medium` 分类。当前权限存储故障会在搜索之前返回可重试的
`503 disclosure_unavailable`。如果完整的序列化结果
超过配置的 `max_context_bytes` 响应上限，Stele 会返回
受限的 `413 provider_response_limit_exceeded`，且不返回部分结果。

管理员可以通过
`GET /v1/admin/assurance/ingestion-freshness?fixture_reference=<event-uuid>` 检查脱敏的事件到读取新鲜度。
该端点仅从已认证的管理员作用域
请求头获取 tenant/project/namespace，并在该精确作用域内解析 UUID。它报告固定阶段
（`admitted`、`governance_completed`、`canonical_active`、`public_retrievable`）
以及受限的处置结果、延迟与 SLO 分桶。公共读取证据仅在成功且已披露的 Provider 检索通过响应
大小校验之后才会被记录。缺失、外部、隐藏或来源不合格的记录不会
暴露其存在性；普通 Provider 响应保持不变。该报告
仅属于运维证据，不代表任务完成或记忆写入回执。

轮次创建接受 `query`、可选的 `context_budget`（按条目放置数计，而非
token）、`include_relations`、`idempotency_key`，以及可选的披露 profile
与模式。作用域/会话由绑定持有；调用方不能提供会话 ID、
轮次 ID、角色或诊断覆盖。服务创建轮次 ID。其
响应省略原始查询，仅暴露轮次/会话 ID、状态、生效的
profile/模式，以及 `context_evidence.memory_ids`/`citations`。仅当显式请求且已授权 `bounded_content` 模式时才会包含摘要；内部
诊断绝不会包含在 Provider 轮次证据中。格式错误或未知
字段返回受限的 `400 invalid_request`，绑定/披露拒绝返回
通用 `403`，披露权限不可用返回可重试的 `503`。
无需任何 schema 或数据库迁移。

<a id="durable-reference-contract-pc5"></a>
## 持久引用契约（PC5）

当运行时需要持久的任务、运行、移交或编译引用时，
先发现 `durable_references` 能力，并在发送请求之前要求
`contract_version=durable-reference-v1`。该契约
是对 `provider-v1` 的增量扩展；严格客户端在能力
或版本缺失时会失败即关闭，绝不替换为排序检索。受支持的事件
类型为 `provider.reference.task`、`provider.reference.run`、
`provider.reference.handoff` 与 `provider.reference.compilation`。

使用 `POST /v1/provider/references`，并携带标准元数据信封以及
`reference_id`（UTF-8，最多 128 字节）、规范化的 `memory_path`（最多
512 字节）、一个受支持的 `event_type`，以及最多 4096 个 UTF-8 字节的值。
规范化操作元数据必须编码为最多 4096 个 JSON 字节。该
值还必须能装入完整的 16384 字节完成响应，包括 JSON
转义以及为最大合法作用域、元数据和谱系预留的空间。
因此，转义密集的值即使低于 UTF-8 上限，也可能在准入之前返回 HTTP 400；
纯 ASCII 值可以使用完整的 4096 字节上限。
响应为 HTTP 202，并报告 `accepted` 或 `pending`；事件 ID 与
`replayed` 标志用于标识幂等准入。接受并不等于治理
完成。使用事件 ID 轮询 `POST /v1/provider/references/status`，或
使用 `POST /v1/provider/references/lookup` 解析精确的 `reference_id` 与路径。
`completed` 响应包含事件、候选、记忆、版本与来源 UUID。`retry_after_ms`
上限为 30000；可重试失败只使用 `retryable` 或 `dependency`
类别。

`reference_only` 会省略值。`bounded_content` 需要当前的
内容披露授权。省略的来源元数据默认为
`untrusted/restricted`；`attested_data` 来源仅在有当前精确作用域证明授权
时才会被接受。状态完成要求原始事件处理、候选提升、
处于活动状态的当前规范版本以及提升来源。缺失、
外部、隐藏、被抑制、被遗忘或已删除的引用会返回
`unavailable`，且不泄露身份或值。精确查找会为所请求的 ID 与路径选择
最近的来源，然后检查其当前可见性
与生命周期；当该来源被隐藏时，它绝不会复活更早的版本。

该能力公布 `max_response_bytes=16384` 与 `poll_after_ms=1000`。
引用路由要求所有绑定作用域维度只能使用 ASCII 字母、
数字、`_`、`.`、`:` 或 `-`，且每个最多 128 字节；conversation 可以
为空。不兼容的绑定会在写入/读取分派之前收到 HTTP 400 `validation`，代码为
`invalid_reference_scope`。现有非引用
运行时身份的限制保持不变。
引用写入不拥有任务状态、调度、编译提交、
工件发布或 webhook 投递。PC5 复用现有的作用域/路径
索引，且无需迁移或新增依赖。混合版本滚动发布保持 PC1–PC4 路由
不变；回滚时可禁用该能力，同时保留已接受的事件、
来源与生命周期历史。

[引用维护指南](provider-durable-references.md) 记录了
复用基线、需求到测试矩阵、滚动发布与证据要求。

## 证据投影契约（PC6）

独立于 `provider-v1`、上下文与持久引用，在 `turn-outcome-feedback-projection-v1` 上协商
`projections`。专用的
`POST /v1/provider/projections`（202）与
`POST /v1/provider/projections/history`（200）使用封闭的元数据/输入信封、
完整的当前运行时绑定以及精确作用域披露。准入需要
读写权限；history 允许只读引用披露。请求
上限为 32768 个 JSON 字节，响应上限为 65536 字节，引用/来源
各最多八个，history 最多八条记录。

一个逻辑效果拥有仅追加的来源修订与稳定的幂等
投递。更大的修订会更正先前的证据；冲突的键复用与
陈旧修订会失败即关闭。记录标记为 `runtime_reported_evidence`，并
将准入时的引用快照与当前重新授权的归属分开。
只有已完成的合格 PC5 引用才能馈入异步的版本化
反馈映射器；写入返回 `none|pending`，绝不会激活排序/洞察。
PC4 回执验证需要相同的绑定与匹配的摘要。

[投影维护指南](provider-evidence-projections.md) 提供
完整的请求/响应结构、错误类别、更正/历史示例、
授权/生命周期行为、调度器启用、迁移 0037、回滚以及
全新栈证据要求。通用 turns/outcomes 仍受支持；
严格的 PC6 消费者绝不将它们用作回退。Danny 的任务、租约、提交、
发布与投递状态仍由运行时拥有。

## 操作元数据与引用

Provider 操作用有界的 `request_id`、`operation_id`、
`idempotency_key`、`event_seq` 与 `schema_version` 元数据。等价的
持久重试会返回原始结果。将同一键用于不同的
规范化负载会返回 `conflict`；重复或陈旧的序列值绝不会
重放副作用。摄取、意图、检索、上下文与生命周期请求
会委托给现有的受治理服务契约。

对于持久引用，`event_seq` 是回显的兼容性归属信息，而不是
排序或重放门控。持久幂等键在后续写入与
重启之后仍保留等价的重试身份；不同的键可以共享同一个
序列值。引用写入不会推进普通的摄取序列
跟踪器。精确查找按 `created_at` 降序，再按 `id` 降序选择最新的持久来源。

可见结果可以包含有界的引用，带有来源种类、稳定
引用、版本或投影水位，以及可用性。普通
Provider 响应省略原始查询、隐藏候选、分数、排序计划、
Provider 负载、凭据与外部标识。被抑制、被遗忘、
已删除、陈旧或超出作用域的证据默认被排除。

稳定的 Provider 错误类别为 `authentication`、`scope`、
`compatibility`、`validation`、`conflict`、`lifecycle`、`stale`、`dependency`、
`retryable` 与 `resync_required`。不支持的 schema 版本会在
操作分派之前失败。`resync_required` 结果是失败即关闭：运行时必须
启动新的快照，而不能假设被跳过的事件已应用。

<a id="provider-context-json-contract"></a>
## Provider 上下文 JSON 契约

`POST /v1/provider/context` 使用所提供的 `ProviderContextRequest` 与
`ProviderContextResponse` OpenAPI schema。发现 `context-disclosure-v1` 与
`source-metadata-v1`、`context_budget_version=context-budget-v1`、
`context_packing_policy_version=context-packing-v1` 以及
`token_budget_supported=false`；在使用之前将 `schema_digest` 与 `/openapi.yaml`
比对。仅凭基础 `provider-v1` 不能确立预算兼容性。
能力文档还会公布 `context_digest`
（`context-digest-v1`、SHA-256、`context-canonical-v1` 与
`context-selection-v1`）以及 `context_continuity`（`context-continuity-v1`）。依赖任一扩展的
客户端必须要求这些精确版本，而不能把能力缺失当作兼容的旧行为。
发送 `X-API-Key`、服务端签发的 `X-Stele-Runtime-Binding` 与
`X-Stele-Runtime-Session`，以及精确的 tenant/project/namespace 请求头。作用域与
会话由绑定持有。当前精确作用域授权必须由管理员显式
配置开通；现有授权处于未配置状态并会被拒绝。

最小请求 (ProviderContextRequest):

```json
{
  "metadata": {"request_id":"context-request-1","operation_id":"context-operation-1","schema_version":"provider-v1"},
  "input": {"query":"current task","budget":4}
}
```

默认值为 `projection_intent=conversation`、授权所配置的
`disclosure_profile`，以及 `response_mode=reference_only`。引用 profile
仅允许引用；内容 profile 允许引用或显式
请求的有界内容。两者都只披露处于活动状态的低/中等级来源。

### 投影摘要与新鲜连续性

每个成功的上下文响应都有一个封闭的 `result.context_digest` 与
`result.continuity` 对象，即使投影为空也是如此。摘要是
对规范且已授权投影的域分隔 SHA-256：精确作用域、
模式、投影意图、披露策略、生效预算、活动的
选择策略指纹、小节顺序、返回的来源版本/路径/
元数据以及引用。它不是凭据、缓存校验器、快照、
水位、业务状态回执，也不是在正常 Provider 契约之外
解引用记忆的许可。

对于普通的全新读取，省略 `continuity`。这会返回
`{"read_semantics":"fresh","outcome":"not_requested"}`，且不执行任何
回执 I/O。若要保留一个短期的、仅含元数据的比较点，请请求
签发：

```json
{
  "metadata": {"request_id":"context-request-issue","operation_id":"context-operation-issue","schema_version":"provider-v1"},
  "input": {"query":"current task","budget":4,"continuity":{"mode":"issue"}}
}
```

响应仍然是一个完整的新鲜上下文体，并带有
`{"read_semantics":"fresh","outcome":"issued","reference":"ctxc_<32 lowercase hex>","expires_at":"<UTC RFC3339 seconds>"}`。
将该不透明引用排除在日志、提示词、诊断与源码管理之外；
它不是凭据，但它是与关联性敏感的运行时元数据。

要比较稍后的等价读取，请使用相同的运行时绑定、会话、
作用域、查询、选择器、响应模式、profile、意图、生效分配
以及相关上下文选项：

```json
{
  "metadata": {"request_id":"context-request-compare","operation_id":"context-operation-compare","schema_version":"provider-v1"},
  "input": {
    "query":"current task", "budget":4,
    "continuity":{"mode":"compare","reference":"ctxc_0123456789abcdef0123456789abcdef"}
  }
}
```

比较会返回完整的当前响应体，并带有 `read_semantics=fresh_compare`
与 `outcome=unchanged` 或 `changed`；它绝不会返回 304、历史响应体、
精确快照回放、水位，也不会对被更改的隐藏证据给出解释/计数。
外部、未知或已过期的引用会返回受限的 409
连续性拒绝；被更改的原始请求会返回 409
`continuity_request_changed`；不兼容的已存储契约会返回 409
`continuity_incompatible`。当前授权与披露检查发生在
回执查找之前。存储/容量依赖故障是受限且可重试的 503
`provider_unavailable`。客户端在这些失败中的任何一种情况下都不得消费
部分输出或本地重建的结果。

回执仅包含绑定身份、请求指纹、摘要身份、
签发/过期时间戳以及不透明引用。PostgreSQL 提供时间并
强制 TTL/容量；查询文本、上下文内容、被选中的 ID、引用与
序列化响应体绝不会持久化到回执中。权威的
规范字节夹具（空/非空引用与有界内容用例，
包括 Unicode 转义）位于
[`internal/provider/testdata/context-digest-v1`](../internal/provider/testdata/context-digest-v1/)。
独立消费者必须重新计算并验证它们；不得计算
第二套检索排序、资格、生命周期或打包策略。

完整请求 (ProviderContextRequest):

```json
{
  "metadata": {"request_id":"context-request-2","operation_id":"context-operation-2","schema_version":"provider-v1"},
  "input": {
    "query":"current task","budget":8,"path_prefix":"tasks/demo",
    "budget_limits":{"max_content_bytes":4096,"max_response_bytes":16384,"max_citations":8},
    "projection_intent":"task_context","disclosure_profile":"memory-content-v1",
    "response_mode":"bounded_content","include_relations":true
  }
}
```

`query` 必须非空白，且 `budget` 为不超过
2,147,483,647 的正整数条目放置数。可选的封闭 `budget_limits` 仅包含
`max_content_bytes`、`max_response_bytes` 与 `max_citations`，每个都是不超过
2,147,483,647 的整数。响应字节至少需要一个；内容字节与
引用允许为零。省略的对象/字段与空对象使用已发布的
服务端上限；显式零在调用方/服务端独立钳制后仍然保留。
内容为零不会把内容模式切换为引用：只有内容确实为空的完整条目才能装入。引用为零不会返回任何记忆条目，因为
每个公共条目都需要规范引用。token 字段（`max_tokens`、
`tokenizer`）属于未知字段，会导致校验失败；字节不是 token 估算。
使用精确 `path` 或显式 `path_prefix`；冲突的选择器无效。需要规范拼写、封闭对象与类型化值：
别名、重复键、null、未知字段与尾随 JSON 都会以受限 400 失败。未知或未授权的
profile 以及已知但未授权的模式返回通用 403。缺失的权限
依赖返回 503。业务角色或投影意图不能授予访问权限。

引用条目包含 `reference` 与 `citations`；内容条目包含
`memory` 与 `citations`。两者都使用种类、ID、版本、路径、类别、可用性以及
生效来源元数据。仅内容模式包含内容。高/受限、
隐藏、外部与不可验证的来源不会披露身份或路径。可选的
洞察、目标、诊断与缓存派生小节保持省略，因为
其证据未获得本公共契约的独立授权。

真实的空响应 (ProviderContextResponse):

```json
{
  "metadata": {"request_id":"context-request-1","operation_id":"context-operation-1","schema_version":"provider-v1"},
  "result": {
    "context_digest": {
      "contract_version":"context-digest-v1","algorithm":"sha-256",
      "canonicalization_version":"context-canonical-v1","selection_policy_version":"context-selection-v1",
      "selection_policy_fingerprint":"55dcb1869c32b2b972fa0f628fa5a948457403cf9da803267a4297574c4dfac2",
      "value":"e150aeaab65c4c28303cd9b5f215770e3a374f8127fb1f11b5613aafe5e36a78"
    },
    "continuity": {"contract_version":"context-continuity-v1","read_semantics":"fresh","outcome":"not_requested"},
    "response_mode":"reference_only",
    "disclosure": {
      "profile":"memory-reference-v1","policy_version":"context-disclosure-v1",
      "projection_intent":"conversation","source_metadata_version":"source-metadata-v1"
    },
    "budget_accounting": {
      "contract_version":"context-budget-v1","packing_policy_version":"context-packing-v1",
      "requested":{"max_items":4,"max_content_bytes":null,"max_response_bytes":null,"max_citations":null},
      "effective":{"max_items":4,"max_content_bytes":1048576,"max_response_bytes":1048576,"max_citations":100},
      "usage":{"items":0,"content_bytes":0,"response_bytes":1387,"citations":0},
      "selection":{"outcome":"empty","omission_reasons":[],"unsupported_sections":[]}
    },
    "profile":[],"recent_session":[],"recent_episodes":[],
    "relevant_summaries":[],"related_entities":[],"citations":[]
  },
  "citations":[]
}
```

该示例为便于阅读而做了美化排版；其 usage 描述的是紧凑 JSON
形式，不含结尾换行。实际 usage 始终描述路由实际发送的精确响应体，
而不是重新格式化后的副本。

在使用之前校验按模式区分的响应。缺失 `result`、必需数组缺失或
为 null、条目模式混杂、未知字段或形状被替换都属于
`failed-contract`，绝不是空成功。条目与聚合引用只能指明
实际返回的规范 ID 与版本。原始事件/来源证据会被省略。
引用不授予后续读取的权限。不会回显查询、已存储的应用
元数据、隐藏计数、排序分数或任意策略归属。

`budget_accounting` 在两种模式下都是必需且封闭的，与
`include_diagnostics` 无关。其 requested 对象将 `budget` 重复为 `max_items`；
仅响应中的 null 标识被省略的可选限制，而显式零仍为
零。生效值是调用方与服务端上限的独立最小值。
Usage 统计跨类别返回条目的出现次数、每个内容字符串的未转义 UTF-8 字节、
唯一的规范 `(kind,id,version)` 引用身份，
以及精确未压缩的序列化 JSON 响应体。路径、来源元数据、JSON
转义、元数据、所有数组、重复的内层/外层引用以及统计
本身都消耗响应字节；请求头、压缩与分帧不消耗。引用
模式消耗零内容字节。每个完整条目都保留其必需引用。

选择会保留资格、当前来源/版本、排序、小节
优先级、摘要偏好、配额与去重。超大条目会被
整体省略；Stele 会继续遍历现有的有界候选流，
不进行截断、剥离引用、替换版本、扩大查找，也不进行
第二次消费者打包/排序。确定性准入在 `context-packing-v1` 下保守
预留有界统计空间；可能留下未使用空间，且不承诺
最大打包。最终 `response_bytes` 包含其自身数字宽度，并等于
实际发出的紧凑响应体长度。

当返回任何条目时（即使有省略），`selection.outcome` 为 `selected`；
仅当没有剩余可披露候选时为
`empty`；当存在合格候选但没有任何完整条目能放入时为 `budget_exhausted`。有序且唯一的
`omission_reasons` 为 `item_limit`、`content_bytes`、`citations`、`response_bytes`。
只有被评估的已授权候选才会产生贡献；隐藏、外部、因授权、
质量或生命周期被拒绝的证据不贡献原因、私有计数、
身份或文本。有序且唯一的 `unsupported_sections` 为
`experience_insights`、`goal_context`、`diagnostics`、`feedback_diagnostics`，且
仅在显式请求时出现。这些固定能力标签不会重新打开公共
洞察/目标/投影/反馈/原始诊断门控，也不会暴露已存储的证据。

在检索之前，强制性的空信封必须能装入生效的响应
上限。不可行的分配会返回 413 `context_limit_exceeded`，且没有结果
或部分流。最终的序列化器/统计/上限不一致会失败即关闭。
格式错误的限制使用 400 `invalid_request`；当前披露拒绝使用 403
`disclosure_denied`；不可用的权威依赖使用可重试的 503，
绝不是空成功或预算耗尽成功。错误响应体有独立的服务端边界，
无需装入调用方的成功分配。仅减少条目预算无法
修复本身就超过响应体分配的信封。

消费者在分配记忆空间之前会先预留其他提示词组成部分。
要求固定的预算/打包版本、正确的模式、所有类别、所有封闭
统计字段、有序枚举、实际的条目/内容/引用计数与精确
捕获的响应字节长度；确保每个 usage 维度都符合其生效
限制，且生效值与发现的上限及请求一致。将
报告中的 requested 限制与原始请求比对，保留省略/null
与显式零的区别，并要求 unsupported-section 标签与其小节
标志一致。针对不同分配的自洽响应属于契约失败。缺失
统计、溢出、不兼容版本、不可能的 usage 或超限响应体
都属于 `failed-contract`，绝不是空结果。源数据仍然不授予任何业务、
工具、写入或编译器权限。分词器/token 预算仍不受支持。
上下文摘要与连续性在上述版本化 PC4 契约
下可用。PC5 使用前面描述的独立持久引用路由；PC6
结果/反馈证据使用单独协商的投影契约。

### 兼容性与维护说明

PC3 要求更新严格消费者 schema，以支持强制统计与
显式的预算/打包发现。独立的服务端条目上限（默认
100）可能减少以前超大的条目请求；预算省略现在可能产生
有界选择，而 PC2 会拒绝大响应体。PC3 没有数据库
迁移，也没有新的持久化存储。PC1/PC2 归档证据不能证明 PC3
就绪；在滚动发布之前，验证全新的初始证据以及 API/worker 重启后的分配/隔离/
生命周期证据。在回滚到不支持 PC3 的服务之前，
为需要该扩展的消费者禁用访问，同时保留 PC2 授权、
来源强制与持久数据。消费者必须判定为兼容性失败，
而不是剥离限制、扩大披露、切换路由或在本地打包。

这会更改公共默认值，并需要迁移 0035 与 0036 以及显式的授权
配置开通。旧绑定会在每个受支持
请求上对照当前授权进行检查。公共 `POST /v1/provider/retrieve` 与 `POST /v1/provider/turns` 使用
相同的当前授权投影边界：检索默认返回封闭的记忆
引用与引用引用，而轮次会收到服务端签发的 ID 与
封闭的证据允许列表。检索内容或轮次摘要需要显式的 `memory-content-v1` 加 `bounded_content`。
这两个路由都不接受
调用方的作用域/会话/轮次/角色覆盖，不暴露排序分数、诊断、
隐藏候选或外部引用，也不允许选择器扩大访问。
Provider 同步与原生/MCP 数据路由仍会在
查找之前拒绝公共凭据。管理操作保留现有的精确作用域检查。

事件摄取上的来源元数据默认为 `untrusted/restricted`。
`attested_data` 需要单独显式配置开通的权益，并且绝不
授权指令、工具、写入或任务完成。同一幂等键下的标签
变更会冲突。所有已存储版本都保留保守的谱系。
有关端点矩阵、配置开通、升级清单与凭据安全回滚，请参阅[披露/来源指南](context-disclosure-and-source-trust.md)。

## 快照与重连同步

下文的归档同步契约仅对显式
授权的管理员运行时可用。PC2 公共凭据会在
查找之前收到 403，包括内容 profile 凭据。公共上下文拒绝不
授权切换到该路由。在完成管理员绑定握手之后，
通过 Provider 边界请求初始有界快照：

```bash
curl -sS "$STELE_URL/v1/provider/sync" \
  -H "X-API-Key: $STELE_ADMIN_API_KEY" \
  -H "X-Stele-Runtime-Binding: $STELE_RUNTIME_BINDING" \
  -H "X-Stele-Runtime-Session: $STELE_RUNTIME_SESSION" \
  -H "Content-Type: application/json" \
  -d '{"schema_version":"sync-v1","max_events":100}'
```

响应包含快照水位、有界的 `next_cursor` 以及
`sync_complete` 标志。仅在运行时接受
完整响应之后才持久化该游标。要重连，请在下一个
请求中发送已确认的游标：

```json
{
  "schema_version": "sync-v1",
  "cursor": "<last-acknowledged-cursor>",
  "max_events": 100
}
```

事件在精确运行时作用域内有序，并带有序列、重放
身份、来源水位、schema 版本以及有界脱敏负载或
引用。重复游标是确定性的，且不会重放写入。
当游标已过期、格式错误、不兼容或超出保留
窗口时，服务会返回 HTTP 409，并带有 `category=resync_required`。丢弃
该游标并重复初始快照流程。同步仅用于读取/回放；
事件摄取、受治理意图以及管理员生命周期操作仍是
唯一的写入路径。

<a id="public-live-verification"></a>
## 一致性与回滚

Provider 一致性使用在单个显式拥有的精确
作用域中的确定性夹具。它检查能力兼容性、作用域拒绝、幂等重放、
生命周期过滤、引用完整性、依赖新鲜度以及
重启/回退行为。被跳过、不完整、陈旧或降级的运行不构成
Provider 就绪证据。一致性检查属于诊断，不会调用
外部 Agent，也不会变更规范记忆，除非通过普通的受治理
夹具摄取。

自托管产品验证器将该证据扩展到受治理的记忆
意图。它通过公共 OpenAPI 边界提交，确认持久
PostgreSQL `memory_intent` 工作引用，重放并冲突一个幂等
键，检查精确作用域检查与外部作用域拒绝，并验证 worker
与调度器重启恢复。它还会停止 worker，验证
`STELE_MEMORY_INTENT_POLICY_ENABLED=false` 会拒绝新的持久化，同时
排队记录仍可检查，然后重新启用相同的
`STELE_MEMORY_INTENT_POLICY_VERSION`，并确认只有精确作用域的待处理
工作恢复。生成的报告经过脱敏，仅使用有界的阶段、
恢复、回滚、清理、新鲜度与可消费性类别。缺失
回滚策略夹具或其他硬门控会被记录为降级
证据，并且绝不会启用 Provider 或更改默认检索。

要回滚 PC2，首先在
原生/MCP 界面上吊销或禁用受限的公共凭据，设置 `STELE_PROVIDER_ENABLED=false`，然后重启 API 副本。
较旧的二进制绝不能服务这些凭据。
现有公共 API、PostgreSQL 规范记录、仅追加历史以及
Provider 一致性证据保持完整。不要在 Provider 回滚过程中向下迁移或删除
规范记忆。

同步也可以独立禁用，而 Provider 操作
保持启用：公布 `synchronization.enabled=false`（不启用任何
传输）并且不注册同步来源。同步
请求随后会以有界 `provider_unavailable` 响应失败即关闭；
现有的摄取、意图、检索、上下文与生命周期路由继续
使用其正常的受治理契约。仅在公布契约、保留窗口与
来源投影均可用之后，才重新启用同步。

相关工作流命令为 `openspec validate --all`、
`go test ./internal/provider ./internal/app ./internal/assurance ./openapi` 以及
仓库质量门禁。

## 可选的轮次绑定上下文读取（STL-1）

严格消费者可以从 `GET /v1/provider/capabilities` 要求 `context-turn-binding-v1` 能力，
创建 Provider 轮次，并在随后的上下文请求中发送其可选的
`turn_id`：

```json
{"metadata":{"request_id":"ctx-1","operation_id":"ctx-1","schema_version":"provider-v1"},"input":{"query":"current task","budget":4,"turn_id":"turn_abc"}}
```

该 ID 是匹配 `^[A-Za-z0-9_.:-]+$` 的非空 ASCII 令牌，最多 128
字节。Stele 在披露、连续性或检索之前，会针对完整的
服务端签发运行时绑定、会话以及精确 tenant/project/namespace
作用域验证不可变的轮次关联。缺失、外部、
未关联、会话错误与绑定错误的轮次共享同一种
不披露的拒绝。该字段仅用于归属：它不能扩大检索、选择
记忆路径、覆盖披露、代表业务角色、估算 token
或重放历史快照。

轮次绑定的响应使用 `context-turn-digest-v1` 与
`context-turn-canonical-v1`；已验证的 ID 会包含在其摘要与
请求指纹中，并在 `result.turn_id` 中回显。无轮次请求保留
v1 摘要字节与回执。跨轮次比较回执，或在
轮次绑定与无轮次请求之间比较，会返回
`continuity_request_changed`。仅当其 `turn_id` 匹配时，才会验证 PC6
回执归属。迁移
0040 增加不可变所有权与有界归属元数据；旧轮次
绝不会被静默认领，审计记录不包含查询、内容、
被选中的 ID、引用、隐藏候选或排序数据。

<a id="governed-correction-updates"></a>
## 受治理的更正更新

需要将更正后的外部结果转化为记忆
更新的严格消费者，必须从能力响应中单独要求 `corrections.enabled=true` 与
`contract_version=provider-correction-governed-version-v1`，
并在其 `operations` 列表中要求 `corrections` 操作令牌。更正能力的 `operation` 字段标识精确的
`POST /v1/provider/corrections` 路由。严格消费者可以使用
`provider.RequireCorrectionCapability` 来拒绝已禁用、格式错误或
不完整的公布。该专用路由不是
`/v1/provider/intents` 的替代品；它接受一个更正后的投影、
一个目标记忆 UUID 与期望版本、有界的替换内容以及一个
原因。请求大小限制为 32768 字节，内容限制为 16384 字节，原因
限制为 512 字节。请阅读所提供的 OpenAPI 以了解封闭信封与精确
元数据要求。

该请求使用当前运行时绑定/会话与精确作用域。Stele
从自身的 PostgreSQL 记录中解析投影头与合格的持久引用谱系，
并验证目标/版本仍匹配。调用方
不能提供或覆盖作用域、角色、授权、绑定、谱系或披露。
响应为 HTTP 202，仅包含受治理的 `intent_id`、`accepted|replayed`
状态与 `attribution_state=verified`；它并不证明
候选已被提升或任务已成功。等价重试使用相同的
幂等键；冲突的复用不会以更改后的内容重试。

外部、缺失、首个修订、隐藏或不匹配的来源都会返回相同的
HTTP 410 `correction_unavailable`；陈旧归属与幂等冲突
是受限的 HTTP 409 结果。可重试的依赖故障为 HTTP 503。
若能力缺失，严格客户端会失败即关闭，且不得回退到
通用意图证据、直接数据库访问、本地更正账本、
备用作用域或规范变更。更正与意图归属是
不可变的。由 River 支撑的 worker 会重新检查资格与目标版本，
异步创建受治理的后继版本，并在可重试失败时
一并回滚候选/版本、意图转换与 River 完成状态。
禁用该能力会停止新请求；回滚期间会保留迁移与已接受的审计
历史。

## 程序性验证边界

程序性验证仅限管理员对处于活动状态的
程序性记忆版本进行维护。它不新增 Provider 路由、能力、字段
或发现机制。Provider 响应绝不包含验证历史、
重新验证状态、更正后的验证负载，也不包含
可用于探测其存在性的路由。

已获授权的操作员可以使用单独记录的管理员 API 追加
可归属的验证证据，或将已被反驳的更正路由到受治理
意图。这不属于 Provider 更正，也不允许公共运行时
执行语义验证、绕过精确作用域、变更规范记忆或
声称任务成功。将任何 Provider 拒绝视为最终结果；不要切换到 MCP、
其他凭据或本地存储作为回退。
