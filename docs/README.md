# Stele 文档

这是供评估、运行、集成或扩展 Stele 的开发者使用的文档入口。[仓库 README](../README.md)
是简短的项目概览；本页把下一步工作映射到相应层级的细节。

## 从这里开始

1. 阅读[入门指南](getting-started.md)，运行本地开发栈。
2. 阅读[架构](architecture.md)，理解运行时与记忆流程。
3. 在[集成 Stele](#集成-stele) 中选择集成边界。

## 理解 Stele

- [架构](architecture.md) 说明核心流程与边界。
- [关键组件](components.md) 将 Go 包映射到各自的职责。
- [最佳实践](best-practices.md) 覆盖安全的应用程序集成方式。
- [披露与来源信任](context-disclosure-and-source-trust.md) 说明来源标签与披露策略
  如何约束返回的上下文。

## 运行 Stele

- [入门指南](getting-started.md) 是简短的本地首次运行教程。
- [自托管](self-hosting.md) 是配置、迁移、备份、恢复与故障恢复的运维参考。

## 集成 Stele

- 需要受治理上下文或记忆写入的公共 agent 运行时，使用
  [Provider 参考](agent-runtime-memory-provider.md)。
- 管理员运营的检查与审阅工作流，使用
  [MCP 管理员指南](integrations/mcp-agent-skill.md)。
- 构建针对具体端点的服务客户端时，使用 [OpenAPI 源](../openapi/spec.go)。
  必须保留认证与精确作用域规则。

## 扩展与维护 Stele

- 查阅 [Stele v1 路线图](roadmaps/2026-05-28-stele-v1-roadmap.md) 了解基线状态与
  后续优先级，以及[证据验证的记忆维护路线图](roadmaps/2026-11-08-evidence-verified-memory-maintenance-roadmap.md)
  了解受 Anthropic 启发的验证、更正与评估待办。
- 变更 Go 包的职责归属之前，先阅读[关键组件](components.md)。
- 部署与恢复操作使用[自托管](self-hosting.md)。
- 提出变更时，遵循仓库的[测试](../docs/documentation_test.go)与
  [OpenSpec 变更](../openspec/changes)。
- 精确行为以 [OpenAPI 契约](../openapi/spec.go)、版本化 Provider 契约和 OpenSpec
  要求为权威；概览性文字只解释意图，不覆盖这些契约。
- 阅读 [OpenSpec 配置](../openspec/config.yaml) 了解仓库变更流程，阅读
  [已归档变更](../openspec/changes/archive) 了解既往设计与实现记录。

## 权威参考

| 关注点 | 权威来源 |
| --- | --- |
| HTTP 请求/响应结构 | [OpenAPI 源](../openapi/spec.go) |
| 公共运行时集成 | [Provider 参考](agent-runtime-memory-provider.md) 及其版本化能力 |
| 管理员工具 | [MCP 集成指南](integrations/mcp-agent-skill.md) |
| 部署与运维 | [自托管参考](self-hosting.md) |
| 变更要求与设计理由 | [OpenSpec 配置](../openspec/config.yaml) 与[活跃变更](../openspec/changes) |
