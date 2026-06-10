# Design: Durable Worker Orchestration And Scope Maintenance

## Overview

这份 change 不新增 product-facing API，而是把现有后台执行面补成更可靠的 governance runtime。

当前代码已经有：

- raw event claim with lease
- governance worker polling loop
- maintenance scheduler loop
- job execution idempotency ledger

但还缺少 durable orchestration 的关键闭环：

- raw event failure 只有 attempt 增长，没有正式失败记录与终态语义
- worker lease 无续租机制，长处理存在 double-claim 风险
- scheduler 基本围绕单一默认 scope 运行，难以覆盖真实多 scope memory service

本设计目标是最小化重构，优先在现有 PostgreSQL-native 语义上补齐 durability，而不是引入外部 queue。

## Goals

- 为 governance raw event 引入 retryable failure、retry budget、exhausted terminal state
- 让 worker 在长处理期间可以续租 lease
- 让 scheduler 对 scope-bound maintenance job 按 eligible scope 派发
- 保持 PostgreSQL 为唯一系统记录源
- 保持当前 runtime mode 结构：`api`、`worker`、`scheduler`

## Non-goals

- 引入新的 public API
- 引入 hosted control plane 或 dashboard
- 把 job orchestration 改造成外部 workflow engine
- 在本阶段实现 exhausted item 的人工 replay surface
- 扩展 observability 或 admin inspection surface

## Current Gap Summary

### Raw event execution durability

当前 raw event 只有这些执行信号：

- `governance_worker_id`
- `governance_claimed_at`
- `governance_lease_until`
- `governance_processed_at`
- `governance_attempt`

这足以表达 `pending / leased / processed` 的基础状态，但还不能表达：

- 最近为什么失败
- 下次何时允许重试
- 是否已经达到最大尝试次数
- 是否已经进入需要人工干预的终态

### Lease safety

当前 worker 在 claim 后不会续租。  
如果 extraction、consolidation、summary compaction 联动耗时超过 lease window，另一个 worker 可能在 lease 过期后重新 claim 同一 raw event。

### Scheduler coverage

当前 scheduler runtime 通过默认 scope 构造 maintenance jobs，这适合最小垂直切片，但不适合真正的 multi-scope memory backend。  
summary compaction 和 retention sweep 应该按 eligible scope 运行，而不是只依赖一个默认 scope。

## Execution Model

### Raw event execution lifecycle

raw event 继续保持 public 不可见的内部执行生命周期，但它的后台运行语义需要明确化：

- `pending`: 未处理、未被有效 lease 占用、且未进入 exhausted 终态
- `leased`: 当前有 worker 持有有效 lease
- `retry_wait`: 最近一次执行失败，等待 backoff 窗口结束后再被 claim
- `processed`: 已完成
- `exhausted`: 达到自动重试上限，停止自动 claim，保留审计信息

这里不要求一定暴露新的 public enum 字段；实现上可以继续通过现有列和少量新增列派生这些状态。

### Recommended storage extension

为了最小化迁移与查询复杂度，建议保留当前 `processed_at + lease_until + attempt` 体系，再补充：

- `governance_last_failed_at`
- `governance_last_error`
- `governance_next_attempt_at`
- `governance_exhausted_at`

这样有几个好处：

- 不必引入新的状态列来重写现有 claim 查询
- `pending / leased / processed` 仍然可从现有字段推导
- `retry_wait / exhausted` 通过新增字段显式化
- 查询层容易表达“可 claim = 未 processed、未 exhausted、lease 过期、且到达 next_attempt_at”

## Failure And Retry Model

### Retryable failure

当 worker 持有 lease 但处理失败时，服务应：

- 记录当前 attempt 对应的 failure time
- 持久化截断后的 error summary
- 计算 `next_attempt_at`
- 释放当前 lease，避免直到 lease timeout 才能进入 retry window

这比单纯等待 lease 自然过期更可控，因为 retry cadence 变成显式策略，而不是隐式依赖 lease duration。

### Exhausted terminal state

当 attempt 达到配置上限后，服务应：

- 设置 `governance_exhausted_at`
- 保留最后一次失败原因
- 不再让 automatic claim query 选中该 raw event

这一步很关键，因为 memory 治理系统里总会出现 poison input。  
没有 exhausted terminal state，坏数据会永久扰动 worker 吞吐和维护窗口。

### Retry policy shape

本阶段推荐使用简单、稳定、可解释的策略：

- fixed or capped linear/exponential backoff
- global max attempts
- exhausted 后停止自动恢复

不在这份 proposal 中引入复杂 error classification。  
所有失败先统一走 bounded retry，再由后续 inspection / replay proposal 提供人工恢复面。

## Lease Renewal Model

### Why renewal is needed

只要单次 raw event processing 可能超过初始 lease，系统就必须允许 active owner 续租，否则 duplicate processing 风险不可接受。

### Renewal contract

建议新增 `RenewClaimedRawEventLease` 这类 repository contract，要求：

- 只允许当前 `worker_id` 续租
- 只允许未 processed、未 exhausted 的 raw event 被续租
- 续租失败时返回 ownership mismatch 或 terminal state mismatch

### Worker behavior

`GovernanceWorker` 或其更外层 orchestration wrapper 应：

- 在开始处理后启动一个续租 ticker
- 在 `lease_renew_interval` 到期时刷新 lease
- 当处理结束或失败时停止 ticker
- 若续租失败，则中止后续 durable commit，避免在 lease ownership 已失效时继续提交副作用

这里的目标不是把 worker 变成复杂的 actor system，而是把“我还拥有这个 claim 吗”变成明确判断。

## Scope-Aware Maintenance Dispatch

### Distinguish scope-bound vs runtime-global jobs

本阶段需要把 maintenance job 分成两类：

- scope-bound
  - `summary_compaction`
  - `retention_sweep`
- runtime-global
  - `job_execution_cleanup`

scope-bound job 应按 eligible memory scope 运行。  
runtime-global job 继续单次运行即可，不应人为复制成每个 scope 各跑一遍。

### Scope discovery

推荐新增 repository method 来枚举 eligible scope，例如：

- 从 active canonical memories 中取 distinct scope
- 必要时结合 recent raw events 或 policy-bearing records

这里的核心原则是：

- scheduler 不再依赖单一默认 scope 才能工作
- scope discovery 本身不引入跨租户泄漏，因为它只在内部 runtime 使用

### Dispatch strategy

每个 scheduler tick：

1. 发现 eligible scope 集合
2. 为每个 scope 生成 scope-bound maintenance job execution
3. 保持 `job_name + scope + run_window` 级别幂等
4. 单独执行 runtime-global cleanup job

如果 scope 数量很大，本阶段只需要支持 batch limit 和后续 tick 继续推进，不要求一次吃完整个 universe。

## Job Execution Idempotency

当前 `job_executions` 已经具备不错的基础：

- `idempotency_key`
- `status`
- `attempt`
- `processed_count`
- `error_message`

本 change 继续沿用这个方向，但要明确：

- scope-bound maintenance 的幂等 key 必须显式包含 scope
- retry 只针对失败 execution record
- duplicate scheduler trigger 不应该产生重复 mutation

推荐继续使用：

- `job_name:tenant:project:namespace:run_window`

对于 runtime-global cleanup，可以继续借助 scheduler 默认 scope 记录 execution，直到未来有 service-global execution ledger proposal 为止。

## Runtime Configuration

建议新增或补强以下配置：

- `STELE_JOBS_GOVERNANCE_MAX_ATTEMPTS`
- `STELE_JOBS_GOVERNANCE_RETRY_BACKOFF`
- `STELE_JOBS_GOVERNANCE_LEASE_RENEW_INTERVAL`
- `STELE_JOBS_MAINTENANCE_SCOPE_BATCH_LIMIT`

保留现有：

- `STELE_JOBS_WORKER_POLL_INTERVAL`
- `STELE_JOBS_WORKER_ERROR_BACKOFF`
- `STELE_JOBS_SCHEDULER_ERROR_BACKOFF`
- `STELE_JOBS_SUMMARY_COMPACTION_INTERVAL`
- `STELE_JOBS_RETENTION_INTERVAL`
- `STELE_JOBS_CLEANUP_INTERVAL`

默认值应偏保守，优先稳定而不是追求高频调度。

## Failure Modes

### Poison raw event

同一个 raw event 多次失败后进入 exhausted。  
它应保留：

- attempt count
- last failure time
- last error summary

但不再自动进入 claim query。

### Lost lease during processing

如果续租失败或 worker 发现 ownership 已丢失，应停止后续 durable completion。  
比起冒险继续提交，宁可让后续 reclaim path 重新处理。

### Empty scope universe

如果当前没有 eligible scope，scheduler 不应报错退出。  
它应平稳等待下一轮 cadence。

### Discovery failure

如果 scope discovery 失败，scheduler 应将本轮视为失败并进入 backoff，而不是部分静默吞错。

## Testing Implications

实现时至少要覆盖：

- retryable raw event failure 记录与 next attempt 计算
- exhausted terminal state after max attempts
- long-running processing 的 lease renewal
- lease ownership 丢失后的安全停止
- 多 scope maintenance dispatch
- 相同 `job + scope + window` 的重复调度幂等

## Follow-up Dependency

这份 proposal 刻意不处理两个后续主题：

- exhausted raw event 的 admin replay / reset surface
- worker 和 scheduler 的 inspection / observability surface

完成这份 change 后，再推进：

1. `admin-inspection-surface`
2. `service-observability`

会更自然，也更容易定义出可靠的状态与指标语义。
