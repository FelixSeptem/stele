# Proposal: Bootstrap Foundation And Event Ingestion

## Why

`Stele` 的总体 roadmap 覆盖 foundation、memory model、governance、retrieval、operations 五个阶段。如果直接以一个大变更推进，范围会过宽，评审和实施都会失焦，也很难形成稳定的验收切面。

因此第一份 OpenSpec proposal 应该先收敛到一个最小但完整的产品纵切：

- 先把服务本身跑起来。
- 先把运行模式、配置、数据库和鉴权边界定住。
- 先打通第一个真实 API：`POST /v1/events`。
- 先把 raw event、canonical memory schema、versioning、provenance 这些后续能力依赖的底座定下来。

这个切分直接对应 `docs/roadmaps/2026-05-28-stele-v1-roadmap.md` 中的：

- `Phase 1: Foundation`
- `Phase 2: Memory Model And Event Ingestion`

这样做的原因很明确：

- governance、retrieval、context assembly 都依赖稳定的 ingest 和 canonical schema。
- 如果第一阶段不先锁定 scope model、storage schema、runtime mode，后续每个 proposal 都会重复改基础设施。
- `POST /v1/events` 是第一条最小可验证的产品路径，既能验证 API-first 模式，也能验证 self-hosted service 的整体结构是否成立。

## What Changes

本 proposal 只引入 v1 的基础服务能力和事件写入能力，不触达治理、检索和运维增强。

### 1. Service foundation

建立 Go 服务基础骨架，至少包括：

- 单一二进制入口，支持 `api`、`worker`、`scheduler` 三种运行模式。
- 配置加载与校验机制。
- PostgreSQL 连接初始化与迁移执行入口。
- 基础 HTTP 服务器与 `health` / `ready` 端点。
- 初始 OpenAPI 骨架。

### 2. Auth and scope isolation primitives

建立后续所有 memory API 的作用域边界，至少包括：

- `project`
- `tenant`
- `namespace`
- API key 鉴权入口
- 请求级 scope context 注入

这一阶段只要求把边界和校验机制定住，不要求完整的组织/用户权限系统。

### 3. Core memory domain model

定义供后续 proposal 复用的核心领域模型，至少包括：

- memory classes：`profile`、`episodic`、`procedural`、`summary`、`relation`
- memory states：`event`、`candidate`、`active`、`suppressed`、`forgotten`、`deleted`
- scope hierarchy：`tenant -> project -> namespace -> agent -> user -> session -> run`
- raw event aggregate
- canonical memory aggregate
- memory version
- provenance record

### 4. Initial PostgreSQL schema

为 first vertical slice 建立数据库基础结构，至少包括：

- raw events 表
- canonical memories 表
- memory versions 表
- provenance links 表
- policy / deletion marker 的基础占位结构
- scope、timestamp、state 相关索引
- `pgvector` 和全文检索字段的扩展预留

### 5. First public API: `POST /v1/events`

引入第一个对外可用的 memory API，用于写入原始事件，至少包括：

- 请求与响应模型
- 作用域校验
- event type / content / metadata / timestamp 校验
- raw event 持久化
- 稳定 `event_id` 返回
- 基础审计和 provenance 记录

## Capabilities

本 change 拆成四个 capability，分别沉淀到独立 spec：

- `service-runtime-foundation`
- `scoped-api-access`
- `memory-storage-foundation`
- `event-ingestion`

## Scope Boundary

本 proposal 明确不包含以下内容：

- candidate extraction
- scoring
- dedupe
- consolidation
- summary generation
- forgetting actions
- semantic retrieval
- lexical retrieval
- `POST /v1/memories/search`
- `POST /v1/context/assemble`
- relation-enhanced retrieval
- observability hardening beyond minimal service logging
- deploy assets beyond bare minimum local bootstrap requirements

这些内容将在后续 proposal 中继续拆分，建议按 roadmap 的后续顺序推进：

1. governance pipeline
2. retrieval and context assembly
3. operations and self-hosting hardening

## Non-goals

本 proposal 不解决以下问题：

- 候选记忆提炼与治理流水线
- 记忆合并、冲突处理、总结与遗忘
- 语义检索、关键词检索和上下文组装
- 图关系写入与关系增强检索
- 完整的组织、成员、角色权限体系
- 生产级可观测性和完备的 self-host 运维能力

## Success Criteria

该 proposal 完成后，应满足以下条件：

- 服务可以以 `api`、`worker`、`scheduler` 模式启动。
- 配置无效时能快速失败并给出明确错误。
- PostgreSQL 可以被初始化并执行基础迁移。
- 请求能经过 API key 与 scope 基础校验。
- `POST /v1/events` 可以成功接收并持久化 raw event。
- raw event 具有稳定标识，并保留基础 provenance 信息。
- canonical memory、version、provenance 的 schema 已可供后续 proposal 直接扩展。

## Impact

### Product impact

- 为 Stele 提供第一条真实可用的 API 路径。
- 为后续 memory governance 和 retrieval 提供稳定底座。

### Engineering impact

- 锁定代码库顶层结构和核心包边界。
- 锁定 PostgreSQL 作为唯一系统事实源。
- 锁定 v1 的 scope model 和 memory 基础模型。

### Proposal sequencing impact

后续 OpenSpec proposal 应基于这一基础继续增量推进，而不是重新设计：

- governance proposal 复用本 proposal 定义的 raw event 与 canonical memory schema。
- retrieval proposal 复用本 proposal 定义的 scope、auth、versioning 与 storage model。
- operations proposal 复用本 proposal 建立的 runtime mode 和 service bootstrap。

## Roadmap Mapping

本 proposal 对应 roadmap 中的以下任务：

- Phase 1
  - Task 1.1 Repository bootstrap
  - Task 1.2 Runtime mode entrypoint
  - Task 1.3 Configuration system
  - Task 1.4 PostgreSQL bootstrap and migrations
  - Task 1.5 Service baseline endpoints
  - Task 1.6 Auth and isolation primitives
- Phase 2
  - Task 2.1 Domain model for memory
  - Task 2.2 Schema design and migrations
  - Task 2.3 Repository interfaces and Postgres implementations
  - Task 2.4 `POST /v1/events` API
  - Task 2.5 Audit and provenance baseline

## Follow-up Proposals

建议按以下顺序继续补 proposal：

1. `governance-pipeline-and-memory-consolidation`
2. `hybrid-retrieval-and-context-assembly`
3. `operations-admin-and-self-hosting-hardening`

这样每份 proposal 都有清晰边界、稳定依赖和独立验收面。

## Artifact References

- Plan: `docs/plans/2026-05-28-stele-v1-memory-service.md`
- Roadmap: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`
- OpenSpec workflow: `openspec status --change "bootstrap-foundation-and-event-ingestion"`
- Apply command: `/opsx:apply`
