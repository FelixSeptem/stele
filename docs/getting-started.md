# Stele 入门指南

**角色：** 教程。本指南只启动本地开发栈；生产部署、迁移、备份、恢复与故障恢复
请使用[自托管](self-hosting.md)。

Stele 是面向 agent 运行时的自托管记忆服务。PostgreSQL 是唯一的记录系统；
检索索引与派生投影不能替代其版本化、受治理的记忆记录。

PostgreSQL 是唯一的记录系统。

## 开始之前

安装 Docker 26 或更高版本以及 Docker Compose v2。随附的技术栈使用
带 pgvector 的 PostgreSQL。支持的版本、部署配置与运维方负责的备份流程见
[自托管](self-hosting.md)。

## 启动本地技术栈

根据示例创建本地环境文件，然后把其中的开发占位符替换为适合本机的值：

```powershell
Copy-Item .env.local.example .env.local
docker compose --env-file .env.local up --build -d
```

请妥善保管 `.env.local`，不要提交到仓库。完整配置与 bootstrap 细节记录在
[自托管](self-hosting.md)中。

## 验证进程健康与就绪状态

API 暴露以下无需认证的进程检查：

- `GET /health` 和 `GET /livez` 报告服务/运行时进程存活。
- `GET /ready` 和 `GET /readyz` 报告基线依赖就绪（对 `/readyz` 而言还包括
  当前配置运行时模式的就绪状态）。

它们并不代表语义、发布或应用层就绪。请检查 `GET /openapi.yaml` 发布的契约，
并在依赖某个部署之前完成文档所述的 bootstrap 与冒烟流程。

## 理解第一个安全的工作流

受保护的记忆操作要求由持久化的管理员为主体授予权限，并为运行时主体授予精确的
tenant/project/namespace 作用域。bootstrap-admin 密钥用于创建第一个管理员，
它不是通用的运行时凭据。在为公共 Provider 请求上下文之前，先为目标作用域配置披露策略。

[Provider 参考](agent-runtime-memory-provider.md) 记录了公共运行时的能力发现、绑定、
上下文与写入操作。[MCP 集成指南](integrations/mcp-agent-skill.md) 记录了管理员运营
的工作流。不要在集成示例中使用直接数据库写入：写入必须遵循 Stele 的受治理 API 或
MCP 契约。

## 选择下一步集成路径

- 公共 agent 运行时，继续阅读 [Provider 参考](agent-runtime-memory-provider.md)。
- 管理员审阅记忆，使用 [MCP 集成指南](integrations/mcp-agent-skill.md)。
- 自定义端点级客户端，使用 [OpenAPI 契约](../openapi/spec.go)。
- 部署运维，继续阅读[自托管](self-hosting.md)。

## 清理并继续

完成后停止本地服务：

```powershell
docker compose --env-file .env.local down
```

这会保留具名 PostgreSQL 数据卷。删除持久化数据或执行任何破坏性清理之前，请先查阅
[自托管](self-hosting.md)。如需了解系统模型与包边界，继续阅读[架构](architecture.md)
与[关键组件](components.md)。
