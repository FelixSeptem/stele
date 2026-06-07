# Proposal: Operations, Admin, And Self-Hosting Hardening

## Why

`Stele` 现在已经完成 foundation、governance pipeline，以及 retrieval and context assembly 的核心闭环，已经具备“写入 governed memory”和“对外提供 agent-ready retrieval”的基本产品能力。

但从 self-hosted memory service 的定位来看，当前运行面仍然明显偏弱：

- `worker` 目前更接近单次执行路径，而不是稳定后台执行面。
- `scheduler` 还没有形成可长期运行的维护任务调度能力。
- operator 仍然缺少 backlog、失败、延迟、历史状态的正式 inspection surface。
- self-hosting 所需的 deploy assets、bootstrap docs、smoke-check path 还不完整。

如果不补这一层，`Stele` 虽然“能工作”，但还不够“能稳定运行、能排障、能自部署”。这会直接削弱它对齐 Supabase 式服务定位的目标。

这一阶段直接对应 roadmap 中的 `Phase 5: Operations And Self-Hosting`。重点不是新增 memory 语义，而是把已有 runtime、worker、scheduler、admin、deploy surface 硬化为可操作、可排障、可自部署的服务形态。

## What Changes

本 proposal 聚焦 worker orchestration、scheduler maintenance、observability、admin inspection surface 与 self-hosting bootstrap，不扩展新的 public memory product APIs。

### 1. Worker orchestration hardening

建立稳定后台执行路径，至少包括：

- 持续 worker loop
- reservation / lease / retry model
- 幂等保护
- failure handling and recovery

### 2. Scheduler and maintenance jobs

建立周期维护执行面，至少包括：

- retention / expiry scheduling
- summary compaction scheduling
- cleanup scheduling
- maintenance cadence 与 request path 分离

### 3. Observability

建立最小可用的运维可观测性，至少包括：

- structured logs
- metrics hooks
- tracing hook points
- ingest / governance / retrieval / forgetting / backlog operational signals

### 4. Admin and inspection surfaces

建立 operator-only 的 inspection APIs，至少包括：

- job status and backlog inspection
- worker / scheduler health diagnostics
- memory history and lifecycle visibility inspection
- provenance-oriented diagnostics

### 5. Self-hosting bootstrap assets

建立最小自部署交付物，至少包括：

- a production-oriented `Dockerfile` for packaging the `stele` service runtime
- a `docker-compose.yml` for running `api`, `worker`, `scheduler`, and PostgreSQL together in self-hosted environments
- bootstrap documentation
- required PostgreSQL extension and config docs
- local or self-host smoke-check guidance

## Capabilities

本 change 拆成四个 capability，分别沉淀到独立 spec：

- `worker-orchestration-and-maintenance-jobs`
- `service-observability`
- `admin-inspection-surface`
- `self-hosting-bootstrap`

对应目录：

- `openspec/changes/operations-admin-and-self-hosting-hardening/specs/worker-orchestration-and-maintenance-jobs/spec.md`
- `openspec/changes/operations-admin-and-self-hosting-hardening/specs/service-observability/spec.md`
- `openspec/changes/operations-admin-and-self-hosting-hardening/specs/admin-inspection-surface/spec.md`
- `openspec/changes/operations-admin-and-self-hosting-hardening/specs/self-hosting-bootstrap/spec.md`

## Scope Boundary

本 proposal 明确不包含以下内容：

- SDK implementation
- hosted control plane or dashboard
- new end-user application flows
- public memory mutation and history APIs
- advanced autoscaling or multi-region HA
- external workflow engine migration

## Non-goals

本 proposal 不解决以下问题：

- 新的 memory class、memory state、governance semantics
- 新的 public retrieval semantics
- learned ranking or personalization
- embedding provider orchestration
- full RBAC system or organization management

## Success Criteria

该 proposal 完成后，应满足以下条件：

- worker 能以稳定 loop 持续执行治理任务，并对 retry / duplicate execution 保持可控。
- scheduler 能周期性触发 retention、expiry、compaction 与 cleanup 等 maintenance jobs。
- operator 能通过日志、指标和基础诊断信号观察 API、worker、scheduler 的运行状态。
- admin-only inspection surface 能查看 job status、backlog、memory history、lifecycle visibility 和 provenance diagnostics，而不需要直接连数据库。
- 新 operator 能根据 `Dockerfile`、`docker-compose.yml` 和 bootstrap docs 启动 `api`、`worker`、`scheduler` 与 PostgreSQL，并完成基本 smoke check。
- 这些运维能力不会改变 public retrieval surface 的默认 lifecycle visibility 语义。

## Impact

### Product impact

- `Stele` 从“功能可用”向“可运维的 self-hosted service”推进。
- 自部署和运维排障首次成为正式产品面的一部分。

### Engineering impact

- 强化 `internal/jobs`、`internal/app`、`internal/governance` 与 admin HTTP surface 的职责边界。
- 明确 worker / scheduler 的可靠执行语义与可观测性要求。
- 为后续 public memory management APIs 提供更稳的运行底座。

### Proposal sequencing impact

后续 proposal 应基于本 change 的运维面继续推进：

- public memory mutation and history APIs 可以建立在更稳定的 admin / runtime 基础之上。
- 更高级的 ranking diagnostics、embedding provider orchestration 或 scale-out 方案应建立在基础 observability 已存在的前提下。

## Roadmap Mapping

本 proposal 对应 roadmap 中的以下任务：

- Phase 5
  - Task 5.1 Worker orchestration
  - Task 5.2 Scheduler and maintenance jobs
  - Task 5.3 Observability
  - Task 5.4 Admin and inspection endpoints
  - Task 5.5 Deployment and bootstrap docs

## Follow-up Proposals

建议按以下顺序继续补 proposal：

1. `memory-management-and-history-apis`

## Artifact References

- Plan: `docs/plans/2026-05-28-stele-v1-memory-service.md`
- Roadmap: `docs/roadmaps/2026-05-28-stele-v1-roadmap.md`
- Prior archived change: `openspec/changes/archive/003-hybrid-retrieval-and-context-assembly`
- OpenSpec workflow: `openspec status --change "operations-admin-and-self-hosting-hardening"`
- Apply command: `/opsx:apply`
