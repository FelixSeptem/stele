# Proposal: Governance Recovery And Filtered Admin Inspection

## Why

`Stele` 在 `007-durable-worker-orchestration-and-scope-maintenance` 中已经补齐了 durable governance runtime 的核心语义：

- governance raw event failure 会持久化 retry context
- raw event 达到自动重试上限后会进入 `exhausted` 终态
- worker 支持 lease renewal
- scheduler 已经具备 scope-aware maintenance dispatch

这一步让后台治理链路从“能跑”提升到了“能稳定运行”。  
但它也留下了一个明确且现在已经无法继续回避的治理缺口：

- exhausted raw event 会被自动隔离，但没有正式的 operator recovery surface
- admin inspection 目前能看 backlog 和 job execution，但还不能按治理失败状态、尝试次数、时间窗口精确排查 raw event
- 运维人员无法通过 service 本身完成 `retry now`、`reschedule`、`clear exhausted / requeue` 这类恢复动作，只能依赖数据库直改

这意味着当前系统虽然具备 durable execution model，但还没有完整的“自动治理 + 人工兜底”闭环。  
对于以 memory 治理为重点的服务来说，这个缺口会直接影响：

- poison raw event 的恢复效率
- 生产运维对治理积压和失败项的可见性
- exhausted item 的处置规范性与审计能力
- self-hosting 场景下的操作可控性

因此，这一阶段应该在不引入新队列、不改变 worker 执行模型的前提下，把 governance raw event 的 inspection 和 recovery 面正式补齐。

## What Changes

本 proposal 聚焦两类能力：

1. governance raw event 的可筛选 admin inspection
2. governance raw event 的受控恢复与审计

### 1. Filtered governance raw event inspection

新增一组 admin-only inspection surface，用于按治理状态排查 raw event，而不是只查看总量或最近 job execution。

第一版至少支持：

- 列表查询 governance raw events
- 单条 raw event 详情查看
- recovery history 查看
- 按 `tenant/project/namespace`、`state`、`event_type`、`attempt range`、`failed_at window`、`next_attempt_at window` 筛选

### 2. Controlled remediation actions

新增一组 admin-only recovery actions，第一版只包含单条 raw event 的受控恢复，不做批量恢复。

动作集固定为：

- `retry`
- `reschedule`
- `requeue`

其中：

- `retry` 用于把 `retry_wait` item 提前到立即可被 worker claim
- `reschedule` 用于人工调整下一次可处理时间
- `requeue` 用于把 `exhausted` item 恢复成重新可自动进入 worker poll 的状态

这些动作都只修改 durable state，不直接触发旁路执行，仍由现有 worker 在下一次 poll 时接手。

### 3. Dedicated recovery audit ledger

新增独立的 `governance_recovery_ledger`，专门记录 operator recovery 动作。

每条记录至少包含：

- `raw_event_id`
- `scope`
- `action`
- `actor`
- `reason`
- `before_snapshot`
- `after_snapshot`
- `occurred_at`

该 ledger 是 append-only 审计面，不复用 `job_executions`。

### 4. Recovery-safe state guards

为了不破坏 `007` 建立好的 worker ownership 语义，本 proposal 会明确 recovery action 的 guard：

- `leased` item 不允许 recovery
- `processed` item 不允许 recovery
- 每个动作都只允许命中自己支持的状态集合
- raw event state update 和 recovery ledger write 必须在同一事务内提交

## Capability

本 change 更新两个 capability：

- `admin-inspection-surface`
- `worker-orchestration-and-maintenance-jobs`

对应目录：

- `openspec/changes/governance-recovery-and-filtered-admin-inspection/specs/admin-inspection-surface/spec.md`
- `openspec/changes/governance-recovery-and-filtered-admin-inspection/specs/worker-orchestration-and-maintenance-jobs/spec.md`

## Scope Boundary

本 proposal 明确包含：

- `/v1/admin/governance/raw-events/...` 资源化 admin route family
- governance raw event 列表、详情、recovery history inspection
- 按失败状态和时间窗口的筛选能力
- `retry`、`reschedule`、`requeue` 三个单条恢复动作
- 独立 recovery ledger
- recovery action 的事务性与状态 guard
- operator-facing 文档补充

本 proposal 明确不包含：

- 批量 recovery
- `ignore` / `drop` / terminal disposal action
- leased item 的强制接管
- 立即同步触发 worker 执行
- governance dashboard 或 web console
- 新的 queue / command bus / workflow engine

## Non-goals

本 proposal 不解决以下问题：

- 把 recovery action 扩展成批量运维平台
- 为 governance raw event 建立新的异步 command pipeline
- 修改现有 worker poll 模型
- 引入更复杂的 error classification 与按错误类型恢复策略
- 扩展 public memory API 或 retrieval contract
- 增加 observability backend 集成

## Success Criteria

该 proposal 完成后，应满足以下条件：

- operator 可以通过 admin API 按状态、尝试次数、失败时间和下一次重试时间筛选治理 raw event
- operator 可以查看单条 raw event 的当前治理状态和 recovery 历史
- `retry`、`reschedule`、`requeue` 动作可以在不直连数据库的前提下完成受控恢复
- recovery action 不会绕过现有 worker durable execution path
- recovery action 会被独立 ledger 审计，能够回答“谁在何时为何恢复了哪个 raw event”
- `leased` 和 `processed` item 不会被错误恢复，worker ownership 语义保持稳定

## Impact

### Product impact

- `Stele` 的治理链路从“自动隔离坏事件”提升为“自动隔离 + 人工恢复闭环”
- self-hosted operator 不再需要直接操作数据库来恢复 exhausted raw event

### Engineering impact

- 主要影响 `internal/app`、`internal/storage/postgres`、`internal/jobs`、`internal/governance`
- 需要新增 admin route、query contract、recovery transaction contract 与 recovery ledger persistence
- 需要为 inspection query 和 recovery action 设计更明确的状态派生语义

### Proposal sequencing impact

这个 change 完成后，后续更适合继续推进：

1. `service-observability`
2. `embedding-reindex-and-vector-governance`

原因是：

- 有了 recovery surface，observability 可以围绕真实的 operator workflow 定义治理指标
- 有了明确的 failed / exhausted / requeue 语义，后续 embedding 重建或其他后台队列也更容易采用同类治理方式

## Roadmap Mapping

本 proposal 对应 roadmap 中的以下任务收紧版：

- Phase 4
  - Task 4.2 Admin and inspection tooling
- Phase 5
  - Task 5.1 Worker orchestration 的 operator recovery follow-up

它不覆盖：

- Task 5.3 Observability
- 批量治理平台或 dashboard
- 更广义 policy center

## Artifact References

- Plan: `docs/plans/2026-05-28-stele-v1-memory-service.md`
- Roadmap: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`
- Related archived change:
  - `openspec/changes/archive/004-operations-admin-and-self-hosting-hardening`
  - `openspec/changes/archive/007-durable-worker-orchestration-and-scope-maintenance`
- Current implementation anchors:
  - `internal/app/app.go`
  - `internal/jobs/jobs.go`
  - `internal/storage/postgres/repository.go`
  - `internal/governance/contracts.go`
- Suggested OpenSpec workflow:
  - `openspec status --change governance-recovery-and-filtered-admin-inspection`
- Suggested verification command during implementation:
  - `go test ./internal/app ./internal/storage/postgres ./internal/jobs ./internal/governance -count=1`
