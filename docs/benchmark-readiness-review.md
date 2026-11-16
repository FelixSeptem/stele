# 基准扩展就绪性审查

审查日期：2026-09-03

## 证据

- `go test ./... -count=1 -timeout 15m`：通过。
- 针对重复导入、跨运行隔离、生命周期排除与可审计运行时身份的 PostgreSQL 集成测试：在 PostgreSQL 18.6 与 pgvector 0.8.6 上通过。
- `openspec validate agent-memory-benchmark-expansion --strict`：通过。
- `openspec validate --specs --strict`：46 通过，0 失败。
- 真实 LongMemEval `s` 报告保留在 `.tmp/benchmark-expansion/longmemeval/modelscope-cleaned-f180315e-s/reports/longmemeval-s/retrieval.json`。

该 LongMemEval 报告经过校验和锁定、非合成，并记录了归一化语料校验和、qrels 校验和、运行时身份、策略、作用域、指标、安全结果与产物路径。其 PostgreSQL 基准作用域在回放后已清理。

## 安全与隔离审查

- PostgreSQL 仍是唯一记录系统；基准代码不改变生产记忆 API 或生命周期语义。
- 每次基准导入与查询都使用基准自有的租户/项目/命名空间作用域。通用策略对比会拒绝生产作用域以及不兼容的语料/qrels/嵌入/归一化标识。
- 检索默认排除被抑制、被遗忘与已删除的记忆。
- 批导入使用确定性 ID 与有界事务；清理按依赖顺序进行，并且只限于播种器自有的 ID。
- 最终数据库检查发现清理后基准运行作用域下没有记录；生产作用域未包含在清理谓词中。

## 许可证与运维审查

- 受限外部语料保留在 Git 之外的本地缓存中。
- 清单要求上游修订、源校验和、转换版本、划分标识、qrels 校验和、许可证与再分发状态。
- C-MTEB/MTEB/BEIR 的选择以元数据锁定，并具有显式的本地存储预算；用户必须在获取之前完成上游许可证审阅。
- Needle、MRCR、LongBench-v2 与 VTCBench 视觉产物仍为计划中或受前置条件门控，除非明确提供本地产物与能力。它们是非门控的压力轨道，不影响记忆产品门禁。
- 离线模式是默认；不引入隐式网络、模型、远程裁判或公开排行榜依赖。

## 残留前置条件

该扩展已可用于已实现轨道的本地产品使用。仍需一次后续变更来将任何外部仅元数据/计划中的数据集提升为可运行状态：获取经许可证批准的本地产物，锁定其校验和/qrels，添加归一化基准样本（goldens），并运行其族特定门禁。这是显式前置条件，而不是隐藏回退。
