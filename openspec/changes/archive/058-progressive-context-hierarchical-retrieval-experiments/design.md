## Context

本变更建立在现有 PostgreSQL/pgvector 检索、稳定 hybrid fusion、层级 chunk、
versioned context projection、context assembly 和 retrieval release evidence
之上。proposal.md 说明了动机与范围；对应行为约束见本 change 下的 delta
spec。设计必须保持默认 flat-fusion 路径、canonical memory 生命周期和现有
tenant/project/namespace 隔离不变。

## Goals / Non-Goals

**Goals:**

- 将 L0/L1/L2 派生上下文和 parent-first expansion 统一为可回放、可比较、可
  审计的 shadow experiment。
- 让每个候选结果绑定 source watermark、scope/lifecycle/freshness 判定、预算、
  证据引用和确定性 identity。
- 复用现有 release gate、projection/chunk lineage、context budget 和 redacted
  observability，形成明确的 fallback、disablement 和 rollback 路径。
- 以 PostgreSQL 派生记录和现有离线 fixture 保存报告，支持 append-only 历史和
  有界清理。

**Non-Goals:**

- 不把实验策略接入默认在线 retrieval 或修改 public response shape。
- 不把 L0/L1/L2 summary 提升为 canonical memory，也不改变 lifecycle transition。
- 不新增 Redis、Kafka、文件系统 canonical store、SDK、UI 或 MCP surface。

## Decisions

### 1. 用显式 level/strategy identity 串起整条实验链

每次 replay 先解析一个不可变的 experiment policy，包含 level（L0/L1/L2）、
parent-first strategy、renderer、projection/chunk policy、baseline fusion、
fixture、evaluation clock 和 exact-scope identity。派生 projection/chunk、
candidate grouping、context pack 和最终 report 都引用这些逻辑 identity，并
绑定 source watermark 与 freshness window。

这样可以让报告在不泄漏内容的前提下判断兼容性和重放条件。只保存逻辑版本和
哈希/桶化结果，不保存 query、scope 原值、provider payload 或 raw score。

### 2. 先做 baseline replay，再做 shadow candidate replay

每个 fixture 在相同的 source snapshot、scope、预算和时间窗口下先运行稳定
flat-fusion baseline，然后运行 progressive level 与 parent-first candidate。
candidate 只产生比较结果和诊断，不写入默认排序或 public response。baseline
结果作为 fallback 和 rollback target 保存到同一份有界报告中。

相比直接替换排序，这种顺序能区分 fixture/数据变化与策略变化，也能在候选
超时或安全失败时给出确定的 baseline outcome。

### 3. 在 expansion 边界执行 fail-closed 验证

parent/child/adjacent expansion 只能使用已有 chunk lineage 和 source-version
snapshot。每一跳先验证 tenant、project、namespace、session、temporal validity
和 lifecycle，再扣减 candidate、token/character、depth 与 latency budget。
任一验证失败即记录 bounded disposition 并停止该分支；不会发起更宽的查询，也
不会用“相似度更高”覆盖 isolation/freshness 失败。

### 4. 报告与诊断沿用现有 redaction/retention 约束

实验报告只输出稳定 logical identities、aggregate metrics、reason buckets、
freshness、citation coverage、replay、fallback 和 rollback verdict。报告写入
现有 evidence/derived-artifact 路径，失败或超时的临时数据必须清理或标为
non-consumable；成功历史 append-only，并复用现有 retention cleanup。

### 5. 通过现有 release handoff 控制未来激活

实验完成并不等于激活。只有当 protected recall、integrity、isolation、duplicate、
latency、deterministic replay、freshness 与 rollback gates 全部通过，且 evidence
attestation 与 exact scope/strategy/baseline identity 匹配时，报告才可进入已有
release review。真正的 activation 仍由现有受治理的 rollout/disablement 流程完成。

### 6. 观测字段固定为低基数类别

新增 telemetry 使用固定的 level、strategy、mode、result、freshness、fallback、
budget、rollback 和 eligibility 类别或桶。任何 scope、query、identifier、DSN、
raw error 或 reason text 在 instrumentation 边界被拒绝、脱敏或桶化；普通 public
请求不会获得实验内部字段。

## Risks / Trade-offs

- [实验报告和 projection 版本增长] -> 复用 append-only derived history 与现有 retention
  cleanup，给每次运行和每种 artifact 设置数量、大小和时间上限。
- [层级 expansion 造成延迟或候选爆炸] -> 固定 depth/candidate/token/character/latency
  budgets，超过上限立即停止并回退 baseline。
- [projection watermark 与 canonical source 不一致] -> 读取前做 freshness/lifecycle
  校验，不满足时只记录 non-pass，不允许进入 candidate 或 release evidence。
- [策略指标改善但证据完整性下降] -> safety gates 优先于质量分数，任何 isolation、
  hidden、foreign、integrity 或 rollback 失败均覆盖 quality gain。
- [shadow diagnostics 泄漏内部信息] -> 复用现有授权诊断面和 redaction contract，
  仅返回固定 reason/count buckets，禁止 raw content、scope、ID 和 provider data。
- [确定性受时间或排序不稳定影响] -> replay 固定 evaluation clock，所有排序使用
  稳定 tie-breaker，并把 policy/renderer/source watermark 纳入 identity。

## Migration Plan

1. 增加实验 policy/report 与 level metadata 的派生结构和迁移，保持可回滚且不做
   destructive down migration。
2. 以离线 fixture 验证 L0/L1/L2、parent-first、scope/lifecycle/freshness、budget
   和 redaction 行为，再接入 owned PostgreSQL + pgvector evidence run。
3. 在 shadow-only 模式收集一段有界 freshness/latency/quality/rollback 证据，运行
   release gate 和 deterministic replay 校验；期间默认 retrieval 不变。
4. 若验证失败，删除或标记候选 artifacts non-consumable，禁用实验策略并继续使用
   baseline。只有后续独立 rollout 变更获得批准时才考虑激活。

## Open Questions

无。level vocabulary、shadow boundary、scope/freshness gates、budget limits 和
证据格式已在 delta specs 中确定，后续实现不需要改变本变更的行为契约。
