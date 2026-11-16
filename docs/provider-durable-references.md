# 持久 Provider 引用维护

`durable-reference-v1` 能力为 Stele 现有的从事件到规范流程增加受治理的任务、运行、移交与
编译证据。Stele
不会成为任务调度器、业务状态所有者、编译权威、
工件发布者或 webhook 发送者。PC5 不意味着 PC6 支持。

## 复用与依赖基线

实现复用当前的 Provider 绑定、操作元数据、
事件幂等、摄取、候选治理、规范版本、
来源、披露授权与生命周期服务。PostgreSQL 仍是
唯一的系统记录。精确引用查找属于仓库操作，
绝不回退到排序检索。库基线是现有的
`pgx/v5`、`google/uuid` 与 `kin-openapi`；无需新增依赖、队列、SDK、UI 或
业务状态存储。成熟库评审应使用其
维护中的包文档：[pgx](https://pkg.go.dev/github.com/jackc/pgx/v5)、
[UUID](https://pkg.go.dev/github.com/google/uuid) 与
[OpenAPI](https://pkg.go.dev/github.com/getkin/kin-openapi/openapi3)。

| 需求 | 实现边界 | 验证 |
| --- | --- | --- |
| 显式能力/版本 | Provider 能力发现与封闭 OpenAPI DTO | 严格消费者拒绝能力缺失、版本错误、别名、未知字段、null |
| 受治理写入与重放 | 公共引用写入委托给普通摄取 | 序列化 HTTP 夹具；相同重放保留身份，变更后复用会冲突 |
| 完成与谱系 | 持久原始事件、候选、当前规范版本与提升来源 | 仓库测试加上全新公共验证器的有界 PostgreSQL 连接 |
| 精确 ID/路径查找 | 作用域引用仓库，与排序分离 | 精确非根路径、后代未命中、被选版本与 UUID 断言 |
| 当前授权 | 现有精确作用域授权与披露策略 | 外部作用域、授权变更、仅引用省略与有界内容检查 |
| 生命周期排除 | 当前规范生命周期与最新引用来源 | 抑制、遗忘、删除与不可用读取，且不会复活旧版本 |
| 重启持久性 | API/worker 重启后相同的已接受事件与规范谱系 | 初始/重启 PC5 身份哈希绑定到镜像、来源与 OpenAPI 摘要 |

## 公共维护契约

每个请求都使用标准的 `metadata` 加 `input` 信封、当前的
运行时绑定/会话请求头，以及精确的 tenant/project/namespace 请求头。
在任何引用写入或读取之前，所有绑定作用域维度（tenant、project、
namespace、agent、session、可选的 conversation 与 Provider 实例）必须
只包含 ASCII 字母、数字、`_`、`.`、`:` 或 `-`，且每个维度最多 128 字节。
必需维度必须非空。超过这些
引用专用限制的绑定会返回 HTTP 400，类别为 `validation`，代码为
`invalid_reference_scope`，且 `retryable=false`，发生在摄取或查找之前。
现有非引用运行时操作的更宽限制保持不变；
启用该能力时，请使用具有兼容身份的绑定。
仅使用规范的 snake_case。写入输入携带 `contract_version`、
`reference_id`、`memory_path`、`event_type`、`value` 以及可选的
`source_metadata`。受支持的事件类型为 `provider.reference.task`、
`provider.reference.run`、`provider.reference.handoff` 与
`provider.reference.compilation`。值使用 UTF-8 字节限制（4096），ID 使用
128 字节，路径使用 512 字节，响应使用 16384 字节。空、null、
未知、格式错误、不受支持或超大的值会校验失败，而
不会被重写或静默截断。

规范化操作元数据必须编码为最多 4096 个 JSON 字节。值
还必须能装入完整的编码完成响应，且在
为最大合法作用域、元数据与谱系预留空间之后仍不超过 16384 字节。JSON
转义计入该上限：转义密集的值即使 UTF-8 大小低于 4096 字节，也可能在准入之前以 HTTP
400 失败。纯
ASCII 值可以使用全部 4096 字节。该准入检查保留了在
响应分配范围内于治理与重启之后返回已接受值的能力。

写入返回 HTTP 202、一个 UUID 事件 ID 以及一个重放指示。使用该事件 ID 轮询状态，
或通过两个专用的 POST
读取路由查找精确 ID/路径。`accepted` 表示持久准入；`pending` 表示治理
尚未完成；`failed` 表示管道失败；`completed` 要求已提交
的提升谱系；`unavailable` 对缺失、外部或
当前隐藏的证据不可区分。完成包含事件/候选/记忆/版本/
来源 UUID 以及规范版本号。`retry_after_ms` 上限
为 30000，发现建议为 1000 ms。错误类别仍限于
`retryable` 或 `dependency`；HTTP 校验、作用域、冲突与授权
错误遵循 Provider 错误契约，不包含内部 SQL 或原始错误。

使用相同作用域幂等键的等价写入会返回原始
事件身份，包括在后续写入与重启之后；语义输入
或归属发生变化则会冲突。`metadata.event_seq` 会被校验并作为
兼容性归属回显。它不会为引用写入排序，也不会拒绝
其他方面有效的持久重放，且不同的幂等键可以共享
序列值。最新来源查找按 `created_at`
再按 `id` 降序排列持久记录。引用写入不会在普通 Provider 事件摄取跟踪器中
预留序列值。

每次读取都会重新授权当前绑定与授权。默认模式为
`reference_only`，它会省略 `value`；`bounded_content` 需要当前内容
披露权限。省略的引用写入标签默认为
`untrusted/restricted`；仅在有适当理由时使用显式的低/中等级标签。
`attested_data` 需要当前来源证明权限，包括在
重放期间。标签不授予指令或业务权限。隐藏或已移除的
最新来源不能在同一选择器下复活更旧的记录。

## 升级、兼容性与回滚

PC5 复用现有的作用域/路径索引，无需迁移。在启用严格消费者之前，验证全新安装与从先前 schema
升级，然后部署兼容的 API 与
worker 版本。保持 PC1–PC4 消费者不变；
基础 `provider-v1` 不能确立 PC5 支持。能力缺失或
不受支持的契约版本属于显式兼容性失败。混合
副本必须拒绝不受支持的请求，而不是降级为普通
事件或排序检索。

回滚会为消费者禁用 PC5，并随旧服务版本
移除能力可用性。保留已接受的事件、版本、来源元数据
与来源；不要删除持久历史或反转生命周期操作。
回滚之后，引用与内容披露仍依赖当前授权。

## 公共发布证据

在自有且全新的
PostgreSQL/pgvector 栈上运行 `scripts/stele-provider-context-verify.ps1`。它使用官方引导凭据，检查
所提供的 OpenAPI 与源 schema 及能力摘要是否一致，通过公共 HTTP 写入全部四种
受支持的事件类型，轮询治理，检查精确查找
与直接的持久谱系，然后重启 API/worker 并在生命周期
移除之前重复。`-ReferenceReportPath` 存储单独的有界 PC5 报告；各阶段
副本保留 `.initial.json` 与 `.restart.json`。重启报告必须验证
未变的谱系、被抑制/被遗忘/已删除的不可用结果，以及全新的
服务来源。它还会为相同的引用 ID/路径写入一个新的、独立受治理的来源，检查精确查找会选中它，隐藏该最新来源
而不回退到更旧来源，并确认更早的规范
版本与来源仍可供特权历史检查使用。
生命周期夹具使用现有的管理员操作 `:suppress`、
`:expire`（记录 `forgotten`）与 `:delete`，并带有必需的审计
操作者与原因，并直接在 PostgreSQL 中验证生成的状态。
每个 PC5 报告都会固定 Provider schema 与夹具版本、迁移版本
与清单摘要，并在声明就绪之前根据
源 SQL 资产验证每个已应用迁移的校验和。
报告仅包含类别、计数、布尔值与哈希；
绝不包含凭据、DSN、授权请求头、作用域取值、引用值
或原始错误响应体。

缺少 Docker、引导、PostgreSQL、pgvector 或重启证据仍是
未通过的验证器结果。仅靠聚焦单元测试与 HTTP 验收无法
确立全新栈就绪。在路线图与发布
决策中保留这一区别，直到实际证据通过。
