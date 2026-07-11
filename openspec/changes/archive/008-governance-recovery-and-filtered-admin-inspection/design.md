# Design: Governance Recovery And Filtered Admin Inspection

## Overview

这份 change 的目标不是创建新的后台执行系统，而是在现有 durable governance runtime 之上补一个可操作、可审计、可筛选的 operator recovery surface。

当前系统已经具备：

- durable raw event execution state
- retry wait / exhausted 终态
- renewable lease
- worker poll-driven execution
- admin inspection 的基础 job/backlog 查看能力

但还不具备：

- 面向 raw event 的精细 inspection
- exhausted item 的正式恢复路径
- recovery action 的受控审计

本设计遵循两个原则：

1. `raw_events` 继续作为治理执行真相源
2. recovery action 只修改 durable state，不开第二条命令执行链

## Goals

- 为 governance raw event 提供资源化的 admin inspection surface
- 支持按失败状态、尝试次数、时间窗口筛选 raw event
- 提供 `retry`、`reschedule`、`requeue` 三个单条恢复动作
- 为 recovery action 建立独立的 append-only recovery ledger
- 保持 worker poll 模型不变，由现有 worker 接手恢复后的 item

## Non-goals

- 批量恢复
- `ignore/drop` 永久处置动作
- leased item 的强制接管
- 立即同步执行 recovery target
- dashboard / UI
- 新队列或 command table

## Route Model

推荐 route family：

- `GET /v1/admin/governance/raw-events`
- `GET /v1/admin/governance/raw-events/{raw_event_id}`
- `GET /v1/admin/governance/raw-events/{raw_event_id}/recovery-history`
- `POST /v1/admin/governance/raw-events/{raw_event_id}:retry`
- `POST /v1/admin/governance/raw-events/{raw_event_id}:reschedule`
- `POST /v1/admin/governance/raw-events/{raw_event_id}:requeue`

这组路由保持 admin-only，与现有 `/v1/admin/jobs/...` 分离。  
原因是这里的关注对象已经不是“job execution summary”，而是“可恢复的治理 raw event 资源”。

## Inspection Model

### Derived raw event states

第一版不引入新的状态枚举列，而是基于现有 durable fields 推导 inspection state：

- `processed`
  - `governance_processed_at IS NOT NULL`
- `exhausted`
  - `governance_exhausted_at IS NOT NULL`
- `leased`
  - 未 processed、未 exhausted，且 `governance_lease_until > now`
- `retry_wait`
  - 未 processed、未 exhausted，且 `governance_next_attempt_at > now`
- `pending`
  - 未 processed、未 exhausted，且未被 lease 占用，且未进入 retry wait

### Filter set

第一版 inspection query 至少支持：

- `tenant`
- `project`
- `namespace`
- `state`
- `event_type`
- `attempt_gte`
- `attempt_lte`
- `failed_from`
- `failed_to`
- `next_attempt_from`
- `next_attempt_to`
- `limit`
- `cursor`

### Detail payload

单条 raw event 详情至少应包含：

- raw event 基础字段
- derived state
- current attempt
- worker ownership / lease window
- last failed at
- last error
- next attempt at
- exhausted at
- processed at

### Recovery history payload

recovery history 至少应包含：

- action
- actor
- reason
- occurred at
- before snapshot summary
- after snapshot summary

## Recovery Action Model

### Retry

用途：让 `retry_wait` item 尽快重新进入 worker claim path。

允许状态：

- `retry_wait`

行为：

- 将 `governance_next_attempt_at` 提前为 `now`
- 保留 attempt
- 保留 last failure 信息
- 写 recovery ledger

### Reschedule

用途：让 operator 人工调整下一次治理时间。

允许状态：

- `pending`
- `retry_wait`

行为：

- 将 `governance_next_attempt_at` 改为指定时间
- 不重置 attempt
- 不清除 failure history
- 写 recovery ledger

### Requeue

用途：把 `exhausted` item 恢复成重新可自动处理的状态。

允许状态：

- `exhausted`

行为：

- 清除 `governance_exhausted_at`
- 清除阻塞 claim 的恢复字段
- 重置自动恢复预算对应的 attempt state
- 使其重新按普通 worker poll path 进入 claim
- 写 recovery ledger

`requeue` 不做“立即执行”。  
恢复后的 event 仍由现有 `GovernanceWorker` 在下一个 poll 周期内正常接手。

## Recovery Ledger Model

建议新增表：`governance_recovery_ledger`

最小字段：

- `id`
- `raw_event_id`
- `tenant`
- `project`
- `namespace`
- `action`
- `actor`
- `reason`
- `before_snapshot`
- `after_snapshot`
- `created_at`

其中：

- `before_snapshot` / `after_snapshot` 可先用 JSONB，记录第一版所需的状态摘要
- 该表 append-only，不允许 update in place

这一层不复用 `job_executions`，因为 `job_executions` 记录的是后台 job 运行过程，不是 operator remediation intent。

## Concurrency And Safety

### State guard

所有 recovery action 都必须使用条件更新，不允许“查出来再盲写”。

统一 guard：

- `processed` item 拒绝 recovery
- `leased` item 拒绝 recovery
- 动作只允许命中自己定义的状态集合

### Transaction shape

每个 recovery action 必须在单事务内完成：

1. 读取目标 raw event 当前状态
2. 校验动作前置条件
3. 更新 `raw_events`
4. 插入 `governance_recovery_ledger`
5. commit

这样可以保证：

- 不会出现状态改了但 ledger 没写
- 不会出现 ledger 写了但状态没改

### Ownership safety

第一版不支持 operator 强制夺取 leased item。  
这条边界是刻意保留的，避免 admin action 破坏 `007` 建立起来的 worker lease safety contract。

## Error Semantics

推荐错误映射：

- `400 Bad Request`
  - 缺失字段、参数格式错误、时间值不合法
- `401/403`
  - admin auth 或 scope access 不合法
- `404 Not Found`
  - scope 内不存在该 raw event
- `409 Conflict`
  - 命中了 `leased`、`processed` 或不匹配的当前状态
- `422 Unprocessable Entity`
  - 动作语义本身不被接受，如 `reschedule` 目标时间非法
- `500 Internal Server Error`
  - repository / transaction / scan 异常

## Testing Strategy

至少要覆盖以下测试层：

### Repository tests

- inspection query 支持按 state / attempt / failed window / next attempt window 筛选
- `retry` 只允许 `retry_wait`
- `reschedule` 只允许 `pending` / `retry_wait`
- `requeue` 只允许 `exhausted`
- recovery action 与 ledger 写入同事务成功

### Service / contract tests

- derived state 计算正确
- leased / processed 的 conflict 行为稳定
- `requeue` 恢复后重新进入 worker 自动 claim path

### HTTP tests

- admin key required
- scoped headers required
- `X-Stele-Actor` required
- `reason` required
- action body validation
- response code mapping

### Regression tests

- recovery 不会直接旁路触发执行
- 现有 worker poll 流程能处理恢复后的 raw event

## Rollout Notes

第一版 rollout 应保持保守：

- 只开放单条动作
- 不开放 bulk endpoint
- 不提供 force takeover
- 不提供 ignore/drop

这能保证：

- 能尽快补齐 operator recovery gap
- 不会一次把治理控制面设计得过大

## Follow-up Dependency

这份 proposal 完成后，后续最自然的扩展是：

1. recovery metrics and observability
2. bulk remediation workflows
3. richer operator governance controls

但这些都不属于本 change。
