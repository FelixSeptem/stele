# Proposal: Durable Worker Orchestration And Scope Maintenance

## Why

`Stele` 当前已经具备 `worker` 和 `scheduler` 的基础运行骨架：

- `worker` mode 可以 claim raw event 并驱动 extraction + consolidation
- `scheduler` mode 可以周期触发 summary compaction、retention sweep、job execution cleanup
- PostgreSQL 中已经持久化了 raw event claim 字段和 `job_executions` 记录

但从“memory 治理闭环”和“可长期运行的 self-hosted service”来看，这一层还不够 durable，主要缺口集中在三件事：

- governance raw event 失败后只有 lease expiry + attempt count，没有正式的失败持久化、重试预算和终态隔离语义
- 当前 claim lease 是一次性窗口，长时间处理可能在 lease 过期后被其他 worker 重复 claim
- scheduler 仍然主要围绕单个默认 scope 运行，而不是对多 `tenant/project/namespace` 的 governed memory 做 scope-aware unattended maintenance

这意味着当前后台执行面更像“已经能跑起来”，还不是“可以稳定治理 memory 生命周期”的状态。  
对于你前面强调的 memory 治理与流转生命周期，这一步非常关键，因为：

- 没有 bounded retry 和 poison item 隔离，治理队列会被坏数据反复击穿
- 没有 lease renewal，长事务或慢处理可能导致重复治理副作用
- 没有 scope-aware maintenance dispatch，遗忘、压缩、整理仍然难以覆盖真实多租户 memory 组织形态

这一阶段不扩展新的 public API，也不引入 hosted control plane。重点是把现有后台执行语义补成 durable governance runtime。

## What Changes

本 proposal 聚焦 durable worker orchestration、failure accounting、lease recovery，以及 scope-aware scheduled maintenance dispatch。

### 1. Durable governance raw event execution state

为 governance raw event 处理建立正式的执行状态语义，至少包括：

- retryable failure 的持久化
- bounded retry budget
- poison / exhausted item 的自动隔离
- last failure time、error summary、next eligible retry time 等恢复信息

这一步的目标不是新增 public surface，而是让 raw event 从“被 claim 过”提升到“有明确后台执行生命周期”。

### 2. Renewable lease and crash-safe reclaim

在现有 claim 机制基础上补齐 lease durability，至少包括：

- worker 在长时间处理期间续租
- lease 失效后的安全 reclaim
- reclaim 只面向未完成、未终态、且到达重试时机的 item
- worker 在 lease ownership 丢失时避免继续提交副作用

### 3. Scope-aware scheduled maintenance dispatch

让 scheduler 从“固定默认 scope 上定时跑几个 job”演进到“按 eligible scope 进行维护派发”，至少包括：

- 发现需要 maintenance 的 scope
- 对 summary compaction、retention sweep 等 scope-bound job 按 scope 调度
- 每个 `job + scope + run window` 保持幂等
- runtime-global cleanup job 与 scope-bound maintenance job 区分处理

### 4. Runtime configuration for durable background execution

为后台 durable 执行补齐最小配置面，至少包括：

- raw event max retry attempts
- retry backoff
- lease renew interval
- maintenance scope batch limit
- scope discovery fallback rules

## Capability

本 change 只更新一个 capability：

- `worker-orchestration-and-maintenance-jobs`

对应目录：

- `openspec/changes/durable-worker-orchestration-and-scope-maintenance/specs/worker-orchestration-and-maintenance-jobs/spec.md`

## Scope Boundary

本 proposal 明确包含：

- governance raw event 的失败持久化、重试预算、终态隔离和 reclaim 语义
- renewable lease contract
- scope-aware scheduled maintenance dispatch
- 配置与文档层面的后台执行契约补充

本 proposal 明确不包含：

- admin inspection endpoints
- metrics / tracing / observability expansion
- public memory read or mutation API changes
- embedding regeneration pipeline
- distributed multi-node leader election
- external queue or workflow engine adoption

## Non-goals

本 proposal 不解决以下问题：

- 让 operator 直接通过 API replay exhausted raw events
- 为后台执行引入完整 dashboard 或 hosted control plane
- 把 PostgreSQL execution ledger 替换成 Kafka、Temporal、Asynq 或其他外部系统
- 修改 canonical memory 的业务生命周期定义
- 重写 retrieval 或 context assembly contract

## Success Criteria

该 proposal 完成后，应满足以下条件：

- governance raw event 在失败后会持久化 retry context，而不是只依赖 lease 过期重试
- raw event 在达到重试上限后会进入自动隔离终态，不再被无限重复 claim
- 长时间处理的 worker 可以续租 claim lease，避免并发重复处理同一 raw event
- scheduler 可以按多个 eligible scope 派发 summary compaction 与 retention maintenance
- 相同 `job + scope + cadence window` 的重复调度不会产生重复 durable mutation
- runtime-global cleanup job 仍然可安全执行，不与 scope-bound maintenance 语义混淆

## Impact

### Product impact

- `Stele` 的 memory 治理从“可调用能力”进一步变成“可持续运行系统”
- 忘记、整理、压缩等生命周期动作更接近真正多租户 memory backend 的运行形态

### Engineering impact

- 主要影响 `internal/jobs`、`internal/governance`、`internal/storage/postgres`、`internal/app`
- 需要把 raw event processing 从“claim then process”补齐为“claim -> renew/fail/complete/exhaust”
- 需要把 scheduler 从默认 scope wiring 提升为 scope discovery + per-scope dispatch

### Proposal sequencing impact

这个 change 完成后，后续更适合继续推进：

1. `admin-inspection-surface`
2. `service-observability`

原因很直接：后台 durable 语义明确后，再做 inspection 和 observability，指标、状态和诊断面才不会建立在不稳定执行模型上。

## Roadmap Mapping

本 proposal 对应 roadmap 中的以下任务收紧版：

- Phase 5
  - Task 5.1 Worker orchestration
  - Task 5.2 Scheduler and maintenance jobs

它不覆盖：

- Task 5.3 Observability
- Task 5.4 Admin and inspection endpoints
- Task 5.5 Deployment and bootstrap docs

## Artifact References

- Plan: `docs/plans/2026-05-28-stele-v1-memory-service.md`
- Roadmap: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`
- Related archived change:
  - `openspec/changes/archive/004-operations-admin-and-self-hosting-hardening`
- Current implementation anchors:
  - `internal/jobs/jobs.go`
  - `internal/governance/pipeline.go`
  - `internal/storage/postgres/repository.go`
  - `internal/app/app.go`
- OpenSpec workflow:
  - `openspec status --change durable-worker-orchestration-and-scope-maintenance`
- Suggested verification command during implementation:
  - `go test ./internal/jobs ./internal/governance ./internal/storage/postgres ./internal/app -count=1`
