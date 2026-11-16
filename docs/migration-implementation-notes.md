# 迁移实现说明

本说明记录在引入迁移生产代码之前，`product-ready-self-hosting-foundation` 变更所要求的实现决策。

## 当前引导清单

- `internal/storage/postgres/migrations.go` 嵌入 `migrations/*.sql` 并暴露 `BaseSchemaSQL`，它目前读取聚合的 `0001_base_schema.up.sql` 文件。
- `internal/storage/postgres/bootstrap.go` 暴露 `BootstrapDatabase`；它开启一个事务，按分号拆分聚合 SQL，执行每条语句，并提交事务。
- `internal/app/app.go` 从 API、worker 和 scheduler 的 PostgreSQL 启动路径调用 `BootstrapDatabase`。
- `internal/benchmark/longmemeval_postgres.go` 还为其本地基准数据库调用引导路径。
- `internal/storage/postgres/migrations/0001_base_schema.up.sql` 是受支持的 schema 快照。它创建 `vector` 扩展以及当前的原始事件、规范记忆、来源、工作流、保障及相关表和索引。
- `internal/storage/postgres/bootstrap_test.go` 目前使用 pgxmock 验证聚合 SQL 执行，但不验证有序版本、待处理迁移、脏状态（dirty state）、分叉或并发启动。

## 库评估

### 选定库：`golang-migrate/migrate/v4` `v4.19.1`

该模块已在 `v4.19.1` 携带 `github.com/golang-migrate/migrate/v4`。当迁移运行器实现后，它将从间接依赖提升为直接依赖。

选择理由：

- 成熟的 PostgreSQL 支持与广泛的生产采用；
- 显式的版本化迁移模型，具有有序的 `up`/`down` 资产；
- PostgreSQL 源驱动，具备数据库驻留的迁移元数据和脏状态（dirty）版本报告；
- 针对并发迁移者的 PostgreSQL 咨询锁串行化；
- `io/fs` 源支持，使不可变迁移可以继续嵌入服务二进制，而无需外部迁移文件依赖；
- 清晰的前向迁移、强制/恢复和无变更语义，可映射到 Stele 的有界状态类别；
- 与本仓库兼容的宽松开源许可。

### 考虑的替代方案

- `pressly/goose`：成熟且支持嵌入式迁移，但需要新的迁移抽象，且不会复用模块中已有的依赖。
- `go-jet/jet`/`tern`：有用的 PostgreSQL 工具，但与现有 Go 迁移依赖不太一致，且会引入不同的运维模型。
- 基于 `CREATE TABLE IF NOT EXISTS` 的仓库本地运行器：被拒绝，因为它无法表示有序升级、脏状态、分叉或安全的并发启动。

## 实现约束

运行器必须使用不可变的编号 SQL 资产、数据库驻留账本、PostgreSQL 拥有的串行化以及仅前向执行。运行时模式和独立 CLI 必须共享同一个运行器、锁和状态读取器。脏状态、分叉、不兼容或部分应用的状态必须失败即关闭；不允许自动 down 迁移。当前聚合 schema 是不可变初始迁移的来源，且不得继续作为可变启动重放机制。
