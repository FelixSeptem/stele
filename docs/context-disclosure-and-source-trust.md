# 上下文披露与来源信任

**角色：** 安全参考。请从
[文档门户](README.md)开始了解架构与集成地图。

PC2 增加 `context-disclosure-v1` 与 `source-metadata-v1`，同时保留
`provider-v1` 信封。它会更改公共访问默认值。消费者在使用上下文之前必须发现
两个扩展版本以及所提供的 OpenAPI 摘要。

## 配置开通

处于活动状态的精确作用域授权需要显式的披露配置。现有
授权没有已配置的 profile，会被拒绝；仅具有读/写访问并不
授予披露。管理员先创建主体与作用域授权，
然后调用 `PUT /v1/admin/principals/{principal_id}/grants/{grant_id}/disclosure`，
并携带管理员认证与精确的 tenant/project/namespace 请求头：

```json
{
  "disclosure": {
    "allowed_profiles": ["memory-reference-v1"],
    "default_profile": "memory-reference-v1",
    "source_attestation": false
  },
  "actor": "operator",
  "reason": "Provision reference access for the runtime"
}
```

该更新与操作者/原因审计是原子的。授权读取会暴露已配置的
`disclosure` 对象；对象缺失表示未配置访问。配置
不能由公共主体更改。官方引导脚本接受
`-DisclosureProfile memory-reference-v1`（默认）或 `memory-content-v1`，并且
初始时始终禁用证明。绝不打印一次性凭据。

| Profile | 允许的模式 | 来源分类 |
| --- | --- | --- |
| `memory-reference-v1` | `reference_only` | low, medium |
| `memory-content-v1` | `reference_only`, `bounded_content` | low, medium |

每个受支持的请求都会读取当前精确作用域权限。降级与
撤销会应用于已签发的绑定。存储故障返回受限的 503；
未配置、已撤销或不可用的 profile 返回通用 403。能力
发现列出受支持的注册表条目，绝不列出单个权益。

## 端点强制矩阵

| 界面 | 公共凭据行为 | 受保护的查找 |
| --- | --- | --- |
| 健康/版本/OpenAPI/Provider 能力 | 固定发现元数据 | 无记忆查找 |
| Provider 运行时初始化/状态 | 需要当前精确作用域授权 | 仅绑定元数据 |
| `POST /v1/provider/context` | 已授权的模式特定投影 | 仅合格的活动低/中等级来源 |
| `POST /v1/provider/events`, `POST /v1/events` | 当前授权与写访问；证明需单独授权 | 安全的事件 ID/重放确认 |
| `POST /v1/provider/intents` | 当前授权与现有意图门控 | 意图 ID/状态确认 |
| `POST /v1/provider/retrieve` | 当前授权解析模式特定投影 | 仅活动同作用域低/中等级来源；不披露分数、诊断、隐藏候选或外部引用 |
| `POST /v1/provider/turns` | 当前授权创建服务端签发轮次 | 仅运行时绑定会话；存储与返回的证据使用协商的封闭允许列表 |
| Provider 同步/遗忘/turn-outcome | 403 | 在操作查找之前拒绝 |
| 原生 search/context/list/history/provenance/session/intent 读取与生命周期路由 | 403 | 在操作查找之前拒绝 |
| MCP 数据工具，包括读取、写入与生命周期操作 | 拒绝 | 在来源/绑定查找之前拒绝 |
| 管理原生/MCP 操作 | 现有管理员与精确作用域检查 | 操作员特权保持显式 |

检索与轮次请求默认使用授权的 profile 与 `reference_only`。
只有当前 `memory-content-v1` 授权加上显式 `bounded_content` 才能
返回检索内容或轮次摘要。公共调用方不能提供
作用域、会话、轮次 ID、角色、诊断标志或其他权限覆盖；
绑定提供作用域/会话，轮次收到服务端签发的 ID。被拒绝的
原生或 MCP 路由
不构成使用其他凭据、本地排序、第二套记忆存储或
传输回退的理由。未来的数据路由在实现
相同披露边界之前保持关闭。固定推理能力发现仍可使用。

## 来源标签与谱系

普通事件输入接受一个封闭的可选 `source_metadata` 对象：

```json
{"version":"source-metadata-v1","trust":"untrusted","classification":"low"}
```

省略会解析为 `untrusted/restricted`。未知字段、标签、版本、
重复键、null 与格式错误的值无效。自由格式的应用
元数据绝不授予信任。`attested_data` 需要已认证主体的
独立精确作用域 `source_attestation` 权益；它验证已授权的
来源归属，绝不验证真实性或指令权限。重试会重新检查当前
权限。在同一幂等键下更改生效标签属于冲突。

分类顺序为 `low < medium < high < restricted`。派生数据使用
最大的来源分类与候选敏感度下限。所有来源
必须具有完整谱系证明才能保留 `attested_data`。缺失或
外部来源贡献 `untrusted/restricted`。提取器、模型与手动
更新不能使历史标签更宽松。

手动创建与更新操作会引入无归属的来源文本：其
新的规范快照会解析为 `untrusted/restricted`，即使先前
版本为低等级或已证明。先前版本保留其不可变标签。
合并与重新分类会保留贡献来源的限制性并集
并追加一个版本；它们不能重新标记旧历史。

迁移 0035 为原始事件、候选、规范记录、
版本、洞察/证据、分块与投影增加快照。规范变更会追加一个
版本；版本与原始/候选标签快照保持不可变。摘要与
洞察写入会连接持久的精确作用域来源。分块/投影条目会解析
来源身份与版本。旧快照保持不受信任/受限。
公共上下文会省略可选的洞察、目标、图、分块、缓存投影与
校准路径，因为其贡献证据无法被独立披露。
它不会报告它们的身份、文本或隐藏计数。

摘要仅当它具有同作用域的 `create_summary_memory`
来源、每个贡献原始事件都是低或中等级，并且每个原始事件
都有一个处于活动状态、同作用域、已提升且存储了版本的规范来源时才合格。缺失、
外部或隐藏的来源边会导致摘要在排序
与 `LIMIT` 之前被省略。授权依赖失败仍是失败。

## 消费上下文

默认模式为 `reference_only`。必需的分类数组在真正空结果时仍以
`[]` 存在。条目包含封闭的 `reference` 与有界
规范引用。引用携带种类、ID、版本、规范化路径、
记忆类别、可用性与来源元数据。它不携带内容、标题、
摘要、教训、任意来源元数据或诊断文本。

仅在具有 `memory-content-v1` 授权时显式请求 `bounded_content`。
内容条目包含 `memory`，其中有相同的类型化引用字段与内容。
高/受限、隐藏、外部或不可验证的来源不会披露身份或
路径。引用仅来自返回的条目；它们不授权另一次
读取。省略原始事件/来源 ID，因为它们需要独立的证据
授权。在使用之前验证按模式区分的 OpenAPI 形状。缺失
字段或格式错误的结果属于契约失败，绝不是空成功。

PC3 `context-budget-v1` 保留正整数条目放置 `budget`，并增加可选的
封闭 `budget_limits`，用于 UTF-8 内容字节、完整 JSON 响应体字节与
唯一规范引用。省略字段继承独立的服务端上限；
内容/引用为零有效且不会改变披露模式。完整且已授权的
条目保留其必需引用，确定性 `context-packing-v1`
准入会保守预留统计空间。必需的封闭
`budget_accounting` 报告请求中的省略 null、生效限制、精确
usage 与安全的有序原因。隐藏/被拒绝的证据不贡献计数器
或存在性提示；不受支持的公共小节保持关闭。

不可行的强制信封或最终统计/序列化拒绝
会返回受限的 413，且没有部分投影。当前权限仍在
打包之前检查：格式错误的分配使用 400，披露拒绝 403，不可用
权限可重试 503。token/分词器仍不受支持。显式发现预算
版本，更新严格消费者 schema 并验证实际响应体
usage。在回滚到较旧二进制之前禁用依赖消费者的访问；
保留 PC2 强制/数据。请参阅 [Provider 分配与迁移](agent-runtime-memory-provider.md)
以及[独立配置](self-hosting.md#provider-context-allocation-configuration)。
摘要/连续性仍属于 PC4，引用/结果工作流属于 PC5/PC6；归档的
PC2 证据不能确立 PC3 就绪。

Danny 拥有业务角色、工具/写入权限、编译器提交与任务
完成。Stele 拥有作用域选择与持久记忆证据。所有被召回的
文本都是消费者策略的数据；即使已证明的数据也不能授权
操作或覆盖指令。

## 升级与回滚

在滚动发布之前清点活动的公共授权与客户端。应用迁移 0035，
配置显式 profile，并部署匹配的 API/worker/调度器二进制。
在混合版本滚动发布期间保持公共凭据禁用：旧副本或
worker 不会强制执行该契约。在启用
流量之前验证迁移校验和、扩展
版本、摘要、治理完成与重启行为。新授权与旧数据不会获得隐式内容或证明。

回滚首先禁用 Provider，并在原生与 MCP 界面上吊销/禁用受影响的公共凭据。
然后在保留增量
列、授权、标签与仅追加历史的同时回退二进制。较旧的二进制绝不能服务
这些受限凭据。仅在强制执行
披露上限并通过其门控的版本上恢复访问。迁移 down 刻意
不可用，因为删除标签会抹除安全边界。

请参阅 [Provider 集成](agent-runtime-memory-provider.md)、
[Agent Skill 集成](integrations/mcp-agent-skill.md) 以及
[PC2 实现任务](../openspec/changes/archive/070-provider-context-disclosure-and-source-trust/tasks.md)。
