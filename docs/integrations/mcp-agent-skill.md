# MCP 集成与 Agent Skill

**角色：** 面向管理员操作的 MCP 工作流的参考。在选择集成边界时，请从
[文档门户](../README.md)开始。
公共 Agent 运行时使用 [Provider 契约](../agent-runtime-memory-provider.md)，
而不是管理员 MCP 凭据或工具。

Stele 的 MCP 适配器是位于现有
OpenAPI 支撑服务之上的可选 Streamable HTTP 适配器。它默认禁用，且不会创建第二套
记忆系统。公共运行时使用 Provider 上下文契约，并带有显式配置开通的
精确作用域披露 profile。所有 MCP 数据工具（包括 `who_am_i`）在 PC2 下都会拒绝
公共凭据。MCP 可用于显式授权的
管理员工作流。请参阅[披露与来源信任](../context-disclosure-and-source-trust.md)。

MCP 不会从 PC5 获得任务或编译状态。公共运行时直接通过 Provider 契约协商
`durable-reference-v1`；管理员
MCP 工具仍受现有精确作用域授权约束。
引用写入最多使用 4096 个 UTF-8 值字节与 4096 个规范化元数据
JSON 字节。完整的完成响应必须在
为最大合法作用域、元数据与谱系预留空间之后装入 16384 个编码 JSON 字节；转义密集
的值即使低于 UTF-8 上限，也可能在准入之前以 HTTP 400 被拒绝。
关于状态、精确路径、披露、重放与滚动发布语义，请使用[引用维护指南](../provider-durable-references.md)。

PC6 运行时证据使用专用的 Provider 投影/历史操作，
而不是 MCP 管理员反馈工具。要求单独公布的
`turn-outcome-feedback-projection-v1` 能力、稳定的 effect/revision 键
以及当前完整绑定。待处理归属不能授权内容或
反馈影响；没有任何投影会完成 Danny 任务或激活洞察。
有关迁移
0037、请求/响应/错误形状与回滚，请参阅[投影维护](../provider-evidence-projections.md)。

## 启用适配器

在 API 运行时的 `.env.local` 中设置这些值：

```dotenv
STELE_MCP_ENABLED=true
STELE_MCP_PATH=/mcp
STELE_MCP_MAX_QUERY_BYTES=4096
STELE_MCP_MAX_PAYLOAD_BYTES=65536
STELE_MCP_MAX_RESULTS=50
STELE_MCP_MAX_IDS=100
```

完整的引导流程、主体授权与 MCP 一致性命令见
[自托管](../self-hosting.md)。使用与 OpenAPI 路由相同的 `X-API-Key` 主体
凭据。将凭据保存在本地环境或
Agent 配置中；绝不要把它们放入提示词、仓库文件或工具
参数示例。

## 管理员工具矩阵

以下工具需要持久管理员访问权限，以及已授权的精确
tenant/project/namespace 作用域。公共凭据不能使用它们。

| 工具 | 用途 | 只读 | 破坏性 | 幂等 |
| --- | --- | ---: | ---: | ---: |
| `who_am_i` | 解析主体、精确作用域、访问模式与绑定 | 是 | 否 | 是 |
| `memory_search` | 用有界查询与引用搜索受治理记忆 | 是 | 否 | 是 |
| `memory_context` | 从合格投影组装有界上下文 | 是 | 否 | 是 |
| `memory_browse` | 在单个作用域内浏览可见记忆 | 是 | 否 | 是 |
| `memory_remember` | 提交受治理的记忆意图 | 否 | 否 | 是 |
| `memory_forget` | 为一个已知记忆请求受治理操作 | 否 | 是 | 是 |
| `memory_forget_preview` | 预览有界的语义遗忘候选 | 是 | 否 | 是 |
| `memory_forget_apply` | 仅应用预览中已评审的 ID | 否 | 是 | 是 |

该矩阵与 `internal/mcp/schema.go` 描述符及
`internal/mcp/adapter.go` 注解保持一致。当
适配器被禁用、操作员缺少所需访问权限，或所需服务
未配置时，工具仍可能不可用。

## 首次调用与精确作用域

使用持久管理员 API 密钥连接到 `/mcp`，并调用 `who_am_i`。如果服务器返回
活动的运行时绑定，则后续调用使用该身份。不要在请求中添加不同的 tenant、project 或 namespace 来扩大
作用域。服务器在访问仓库之前会解析并重新检查绑定。

为 MCP 客户端配置已认证主体，以及（一旦由
运行时引导签发）服务器拥有的绑定请求头：

```text
X-API-Key: <placeholder-api-key>
X-Stele-Runtime-Binding: <server-issued-binding>
```

仅当客户端在工具参数中提供相同的绑定
ID 时，绑定请求头才是可选的。它绝不是凭空编造 tenant、project 或
namespace 取值的许可。

概念上的首次调用为：

```json
{
  "name": "who_am_i",
  "arguments": {
    "runtime_binding_id": "<server-issued-binding>"
  }
}
```

如果调用中未提供绑定，适配器可以使用已认证 MCP 请求中的绑定请求头。
将无法解析或非活动的作用域视为
阻塞错误。

## 读取工作流

选择能回答当前问题的最小操作：

```text
who_am_i -> memory_context      # bounded working context
who_am_i -> memory_search       # focused query with citations
who_am_i -> memory_browse       # review or pagination
```

搜索参数示例：

```json
{
  "query": "deployment rollback decision",
  "path_prefix": "operations",
  "limit": 5,
  "runtime_binding_id": "<server-issued-binding>"
}
```

将 `query`、`limit`、`budget_bytes` 与路径选择器保持在公布的 MCP
限制内。将返回的内容视为有作用域的证据。即使 `attested_data` 也不授予
指令、工具、写入、编译器或任务完成权限。保留引用
并在对被召回文本采取行动之前应用消费者策略。

## 受治理的 remember 工作流

对于显式的持久记忆请求，使用 `memory_remember`。提供原因
与稳定的幂等键：

```json
{
  "content": "The deployment rollback runbook requires a schema check first.",
  "memory_path": "operations/deployments",
  "memory_class": "procedural",
  "reason": "Record an operator-approved runbook lesson",
  "idempotency_key": "<stable-request-key>",
  "runtime_binding_id": "<server-issued-binding>"
}
```

结果是一个受治理意图或已接受结果，而不是绕过
worker 整合或规范版本化的许可。只读主体会收到
受限的授权错误。

## 安全遗忘工作流

对于语义遗忘，始终先预览再应用：

```text
memory_forget_preview -> review returned IDs -> memory_forget_apply
```

预览示例：

```json
{
  "query": "obsolete deployment instructions",
  "path_prefix": "operations/deployments",
  "limit": 10,
  "runtime_binding_id": "<server-issued-binding>"
}
```

仅应用 Agent 或操作员已评审的 ID：

```json
{
  "preview_id": "<preview-id>",
  "memory_ids": ["<reviewed-memory-id>"],
  "action": "suppress",
  "reason": "Reviewed obsolete instructions",
  "idempotency_key": "<stable-request-key>",
  "runtime_binding_id": "<server-issued-binding>"
}
```

仅当已明确知道一个记忆 ID 时才使用 `memory_forget`。绝不要
把无约束的搜索变成破坏性批量操作。

## 错误与恢复

- `auth`：停止，并请求操作员审查所配置的访问权限；不要寻求更宽的凭据。
- `scope`：停止；操作员必须审查精确作用域与适用的绑定。
- `validation`：修正请求形状、路径、限制或必需的原因。
- `lifecycle`：使用允许的受治理生命周期路径或请求审查。
- `compatibility`：使用服务器公布的契约与工具版本。
- `dependency`：检查 MCP 启用状态与 API 服务配置。
- `retryable`：使用相同身份重试相同的幂等操作。

当 MCP 被禁用时，`/mcp` 返回未找到，而普通 OpenAPI 路由
仍可用。不要回退到直接 SQL 或无作用域的数据库客户端。

## 公共 Provider 工作流

公共运行时会发现能力，在
操作员配置开通之后初始化 `/v1/provider/runtime`，然后调用 `/v1/provider/context`。它也可以使用
`POST /v1/provider/retrieve` 获得封闭的检索投影，并使用
`POST /v1/provider/turns` 获得服务端签发的轮次。默认
`reference_only` 返回结构化引用。显式 `bounded_content` 需要
`memory-content-v1` 权限。验证模式特定响应与类型化
`source_metadata`；缺失字段属于契约失败，绝不是空成功。
由 PostgreSQL 支撑的初始化会以原子方式确保绑定的会话存在于
精确作用域中；第一次轮次无需单独的原生会话创建。
摄取时省略来源元数据意味着 `untrusted/restricted`；证明
需要独立的当前精确作用域权限。

Provider 事件与意图写入返回安全的 ID/状态，而不是已存储文本。
公共检索与轮次需要当前精确作用域披露授权与
运行时绑定；检索省略分数、诊断、隐藏候选与
外部引用，轮次返回投影证据。公共同步、原生数据
读取/生命周期与 MCP 工具都会被拒绝。
拒绝不允许传输回退、作用域变更、备用凭据、
本地排序或本地引用/事实存储。请查阅
[Provider JSON 契约](../agent-runtime-memory-provider.md#provider-context-json-contract)
与[升级/回滚指南](../context-disclosure-and-source-trust.md)。
MCP `budget_bytes` 仍是其管理员工具契约；Provider `input.budget`
统计返回的放置数，并使用 `context-budget-v1` 可选封闭
`budget_limits` 来表示 `max_content_bytes`、`max_response_bytes` 与 `max_citations`。
显式发现 `context_budget_version=context-budget-v1`、
`context_packing_policy_version=context-packing-v1` 与
`token_budget_supported=false`。省略字段继承独立的
服务端上限；内容/引用为零有效且不会改变模式。条目/响应体
必须为正，所有请求整数最多为 2147483647，且 token/null/
duplicate/case/unknown/fractional/overflow 输入返回受限 400。

公共 Provider 默认值为 100 个放置、100 个唯一引用、默认 1 MiB
完整 JSON 字节，内容继承自解析后的响应体上限。必需的
封闭 `budget_accounting` 统计返回条目的出现次数、未转义 UTF-8
内容（引用模式下为零）、唯一规范 kind/id/version 引用以及
实际完整响应体（包括路径、标签、转义、重复引用
与统计）。验证固定版本、省略 null、独立钳制
的生效限制与实际 usage。缺失/不正确的统计属于失败
契约。`selected`、`empty` 与 `budget_exhausted` 保持区分；安全有序
原因不披露隐藏候选计数。请求的不受支持公共洞察、
目标与诊断标签描述能力，并保持这些路径关闭。

Stele 拥有带必需引用的完整条目准入，以及一个确定性的
保守预留，可能留下未使用字节。消费者预留其他提示词
空间，不做本地排序/打包、内容截断或字节到 token
转换。不可行的强制信封/最终限制拒绝返回 413，且没有
部分输出；更小的条目限制无法修复信封空间。错误有
独立的服务端边界。当前披露拒绝为 403；不可用权限
是可重试的 503，而不是空成功。PC3 不会改变原生/管理/MCP
默认值。严格消费者需要 schema 迁移与显式协商；
在回滚之前禁用依赖访问，保留持久数据/PC2 强制，
并且不要把归档的 PC1/PC2 证据计为全新的 PC3 就绪。

Provider 上下文 PC4 还要求每个成功响应都带有必需的 `context_digest` 与
`continuity` 字段。发现 `context-digest-v1`、
`context-canonical-v1`、`context-selection-v1` 与
`context-continuity-v1`；捕获完整响应体并独立验证其
规范摘要。普通新鲜请求省略 continuity，仅在需要获取短期不透明、仅元数据的
比较引用时使用 `{"mode":"issue"}`，并使用相同的绑定/会话/等价分配
与 `{"mode":"compare","reference":"ctxc_<32 lowercase hex>"}`，以获得
标记为 `unchanged` 或 `changed` 的新鲜完整响应体。Provider 比较
不是 MCP 工具、HTTP 缓存校验器、响应体重放或水位。不要记录
引用/摘要，也不要实现本地记忆、排序或上下文打包
回退。409 连续性拒绝或可重试 503 意味着在
Provider 请求被修正或重试之前，没有上下文可以安全
消费。

## 参考模式

路线图记录了关于 Stash、Letta Code 与 Supermemory 的研究，作为 Agent 人体工学、记忆组织与 MCP 引导的设计
灵感：
[外部参考笔记](../roadmaps/2026-05-28-stele-v1-roadmap.md)。这些
笔记是模式，不是兼容性声明。Stele 仅采用
符合其 OpenAPI 优先、仅 PostgreSQL、精确作用域、版本化与受治理
生命周期契约的部分。这些项目的名称、端点、存储模型与产品
特性不是 Stele 的功能，除非本仓库的
公共契约记录了它们。

## 规范 Skill

可移植的 Agent 指令位于
[`skills/stele-memory/SKILL.md`](../../skills/stele-memory/SKILL.md)。将该
文件复制到 Agent 宿主的 skill 目录，或将其作为仓库上下文包含。
每当 MCP
工具或注解发生变化时，保持该 skill 与本指南与 `internal/mcp/schema.go` 一致。

## 仅 HTTP 的仓库快照

记忆仓库导入在 v1 中不新增 MCP 工具。启用后，它严格遵循
[仓库导入指南](memory-repository-import.md) 中描述的评审优先 HTTP 快照协议；
不要替换为 Git、
文件系统、URL、归档或本地记忆行为。

## 程序性验证维护

程序性验证维护是仅限管理员操作的 HTTP 工作流，
不是 MCP 工具。需要记录证据、检查脱敏后的
验证/重新验证状态或提交已被反驳的更正的操作员，应使用
记录的 `/v1/admin/procedural-memory-verifications` OpenAPI 操作，并携带
管理员凭据与精确作用域请求头。

该工作流不向公共 Provider 凭据开放，MCP 也不
提供回退路由。它保留仅追加的审计轨迹，并且只将
更正传递给现有的受治理更新路径；它绝不会让 MCP
调用方能够推断结果、直接创建规范版本或披露
隐藏证据。有关功能启用、边界、验证与回滚，请参阅[自托管](../self-hosting.md#procedural-verification-maintenance)。
