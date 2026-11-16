# 记忆仓库快照导入

**角色：** 可选的评审优先 Markdown 导入边界的集成参考。
请从[文档门户](../README.md)开始，在
此路径、Provider、MCP 与原生 OpenAPI 之间进行选择。

Stele 仅通过 HTTP 接受有界的 `memory-repo-v1` Markdown 快照。
它是一个单向、评审优先的适配器：PostgreSQL 仍是系统记录，
已接受的事实会进入现有的受治理 `remember` 意图处理，
而不是直接写入规范记忆。

## 边界与启用

准入默认禁用。设置 `STELE_MEMORY_REPOSITORY_IMPORT_ENABLED=true`
以允许注册、预览与提交。之后禁用会阻止新的
准入，但保留已接受的工作、审计历史与受治理的 worker 排空。

该服务绝不克隆或获取 Git 仓库、读取调用方文件系统
路径、获取 URL、解压归档、跟随快照外部的链接，
也不执行 Markdown、代码围栏、命令、URL 或工件。V1 不新增
MCP 工具。

每个请求都使用精确的 tenant/project/namespace 作用域与当前
披露授权。未授权的作用域会在
仓库、预览、重放、生成或来源证据可能被披露之前被拒绝。Markdown
元数据绝不授予信任、权限、作用域、生命周期或策略特权。

## 评审优先的 HTTP 协议

1. `POST /v1/memory-repositories` 仅注册不透明标签。
2. `POST /v1/memory-repositories/{repository_id}/previews` 提供完整的
   JSON 快照以及显式的 `(entry_key, memory_class, memory_path)` 映射。
3. 检查诊断、生效信任、计划摘要、警告摘要及其
   24 小时有效期。使用 `Idempotency-Key`、匹配的
   生成版本、计划摘要与警告摘要提交未更改的计划。
4. 轮询导入状态或读取清单。`queued`、`running` 与
   `completed` 描述的是分派，而不是规范激活。

状态返回有界的 `queued`、`submitted`、`duplicate`、`rejected` 或
`failed` 条目结果，且不含来源负载。最多 100 个条目的清单
仅提供当前符合生命周期与披露条件的导航；它绝不增加
常驻可见上下文、排序加权或隐藏目标计数。

## 格式、限制与生命周期

`MEMORY.md` 是必需的。V1 接受 UTF-8 NFC 斜杠相对 `.md` 文件以及一个
窄子集：顶层单行事实项目符号、有界的尾部元数据
与围栏外的独立根相对 wiki 链接。不受支持的散文、
嵌套或多行列表、格式错误的元数据与无法解析的链接会被
诊断，而不会被解释为指令。

上限为 128 个文件、每个文件 64 KiB、解码内容 1 MiB、请求体
2 MiB、1,000 个条目、每个条目 8 KiB、路径/修订 256 字节、16 个元数据
对，以及每个元数据值 2,048 字节。超限与无效路径会拒绝
整个请求，且不进行部分导入。

接受会保留不可变的计划/修订证据、生成版本、尝试次数与
去重身份。相同键的等价重放会返回原始
导入；更改后的请求会冲突。缺失文件绝不会更新、遗忘、删除
或复活规范记忆。迁移 `0038_memory_repository_imports` 是
增量式的，且刻意非破坏性；在回滚之前禁用准入，
并在已接受的工作排空之前保留 worker/schema 兼容性。
