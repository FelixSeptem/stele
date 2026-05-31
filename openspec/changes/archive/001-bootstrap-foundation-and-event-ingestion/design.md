## Context

`Stele` 的第一份 change 目标不是做出完整 memory system，而是先建立一个可持续扩展的基础层。这个基础层必须同时满足几个约束：

- 代码库需要有稳定的服务入口和包边界，后续 governance、retrieval、operations proposal 不能反复推翻基础结构。
- PostgreSQL 必须从一开始就是唯一系统事实源，因为 v1 不支持多存储后端。
- API 必须先建立 project / tenant / namespace 三级隔离的最小闭环，否则后续 memory 写入和读取都缺少安全边界。
- 第一条纵切要足够小，但不能只是 demo。它必须打通 `POST /v1/events`、原始事件存储、基础 provenance 记录和迁移体系。

当前仓库几乎为空，因此这一阶段需要同时完成工程骨架、运行时、数据库和第一个 API 的落地。

## Goals / Non-Goals

**Goals:**

- 建立单一二进制、多运行模式的服务基础框架。
- 固化配置加载、PostgreSQL 初始化、迁移执行和健康检查机制。
- 定义 v1 memory 的基础领域模型和最小数据库 schema。
- 建立 API key 和 scope context 的基础隔离机制。
- 打通 `POST /v1/events` 到 PostgreSQL 的完整写入链路，并保留基础 provenance 信息。

**Non-Goals:**

- 不实现 candidate extraction、consolidation、summary 或 forgetting。
- 不实现 semantic / lexical retrieval。
- 不实现完整的 admin surface、metrics、tracing 或部署编排。
- 不实现完整的 project 管理、用户管理或组织 RBAC。

## Decisions

### Decision 1: Use one Go binary with mode-based startup

选择单一 Go 二进制，通过配置或命令参数切换 `api`、`worker`、`scheduler`。

Rationale:

- 与 roadmap 中的服务形态一致。
- 避免前期拆分多个 deployable，降低演进成本。
- 公共配置、日志、数据库和 domain 代码可以复用。

Alternatives considered:

- 多二进制拆分：更清晰，但当前阶段会过早放大构建和部署复杂度。
- 只做 API 模式：短期更快，但会迫使后续再重构 runtime model。

### Decision 2: Treat PostgreSQL as both source of truth and migration anchor

第一阶段即把 PostgreSQL 定为唯一系统事实源，并在基础 schema 中预留全文和向量扩展接入点。

Rationale:

- 与总体 plan 保持一致，避免双写或后迁移。
- 后续治理、检索、审计都依赖统一存储模型。
- 可以先不启用所有高级检索能力，但 schema 不需要大改。

Alternatives considered:

- 先用内存或文件存储做原型：会让后续 schema、事务和隔离语义全部重写。
- 把 vector store 抽到外部系统：超出 v1 基础阶段范围，增加运维面。

### Decision 3: Model raw events as append-only records and preserve provenance from day one

`POST /v1/events` 只负责写入原始事件与 provenance，不在本 proposal 中直接生成 governed memory。

Rationale:

- 把第一阶段的职责收窄到“可靠接收并存储事实”。
- 给后续 governance proposal 留出清晰输入边界。
- append-only raw event 能天然支撑审计、重放和重新治理。

Alternatives considered:

- 事件写入时同步生成 canonical memory：会把治理逻辑过早耦合进第一份 change。
- 只存 canonical memory 不存 raw event：会削弱审计与重放能力。

### Decision 4: Enforce scope through middleware-resolved request context

API key 验证和 `project` / `tenant` / `namespace` 解析通过 HTTP middleware 完成，handler 只消费解析后的 scope context。

Rationale:

- 统一所有 API 的隔离入口。
- 避免业务 handler 反复解析鉴权和作用域字段。
- 便于后续 repository 和 service 层复用 scope 语义。

Alternatives considered:

- 每个 handler 自行解析作用域：容易漂移，且难保证默认安全。
- 依赖外部网关透传完整身份：当前 proposal 仍需要服务内部最小边界约束。

### Decision 5: Define canonical memory schema now, even if this phase only writes raw events

虽然本阶段只实现 raw event ingestion，但 canonical memory、memory versions、provenance links 等基础 schema 也一起落地。

Rationale:

- 防止 governance proposal 反向推翻基础表结构。
- 允许后续 proposal 直接叠加候选记忆和 consolidation 能力。
- 让 versioning 和 provenance 语义在 v1 一开始就固定。

Alternatives considered:

- 只创建 raw events 表：短期简单，但后续 schema 迁移风险高。

## Risks / Trade-offs

- [基础 schema 过早冻结] → 通过只定义必要字段和扩展占位列，避免把治理和检索细节写死。
- [作用域模型过于简化] → 先固定 `project / tenant / namespace` 最小集合，把更复杂的 identity 系统延后。
- [API 先于完整 OpenAPI 沉淀] → 本阶段至少提供初始 contract skeleton，并要求后续变更同步更新。
- [worker 和 scheduler 模式当前无实质工作] → 接受空实现模式，换取 runtime model 早定型。

## Migration Plan

1. 初始化 Go module、runtime 入口和配置系统。
2. 接入 PostgreSQL bootstrap 与迁移 runner。
3. 落基础 schema：raw events、canonical memories、memory versions、provenance links。
4. 暴露 `health` / `ready` 和受保护的 API 路由骨架。
5. 接入 API key 与 scope middleware。
6. 实现 `POST /v1/events`，在单事务内写入 raw event 和基础 provenance 记录。
7. 补初始 OpenAPI 描述与验证测试。

Rollback strategy:

- schema 迁移保持前向兼容，第一阶段上线前允许整库重建。
- 如果 `POST /v1/events` 有问题，可以禁用该路由并保留基础 runtime 与 schema。

## Open Questions

- API key 的持久化表是否在本 proposal 中一并定义，还是先使用配置注入的静态 key 方案。
- OpenAPI 文档源是否采用手写 schema 文件还是代码生成优先。
- `event_id` 是否直接使用 UUIDv7，还是统一抽象成服务内部 ID 生成器。
