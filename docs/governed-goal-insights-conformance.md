# 受治理的目标洞察一致性运行

此运行检查 `goal` 洞察候选的有界 PostgreSQL 与 pgvector 路径。它是一次离线/shadow 且仅评审的检查。它不声明就绪状态、不启用目标激活，也不改变普通检索可见性。

## 操作员运行

提供一个带有 `vector` 扩展的可弃 PostgreSQL 实例和一个显式 DSN。当数据库为该运行隔离时，本仓库使用的现有 pgvector 镜像即可满足需要。例如，使用本地映射：

```powershell
$ownedGoalDsn = Read-Host 'Enter the disposable PostgreSQL + pgvector DSN'
pwsh -File scripts/stele-goal-conformance.ps1 -TestDSN $ownedGoalDsn
```

DSN 仅由测试进程使用。不要把凭据或 DSN 放入已提交的报告、issue、日志或文档工件中。

如果 `STELE_TEST_POSTGRES_GOAL_DSN` 不存在，脚本会以稳定的跳过码 `2` 退出，并声明未做出任何就绪声明。前置条件失败或缺少 pgvector 扩展属于非通过结果。

## 证据类别

一致性矩阵验证：

- 目标元数据 JSONB 往返保留 `proposed` 和 `review_required`；
- 重放重试幂等，并保留一行候选；
- 精确的租户/项目/命名空间作用域用于候选读取和派生读取；
- 默认派生洞察列表会省略目标，而显式实验性检查可以请求它们；
- shadow/仅评审处理不会更改规范记忆；
- 在运行继续之前，PostgreSQL `vector` 扩展已存在。

测试在其输出中仅保留有界状态和处置类别。它不会打印目标文本、提示词、证据负载、标识符、外部作用域值、凭据或原始数据库错误。

## 清理与失败解读

测试使用唯一租户，并在完成时删除其候选、派生洞察和证据行。请对可弃数据库运行该测试，以便被中断的进程可以通过删除该唯一一致性租户来清理。

- `PASS` 表示矩阵在提供的隔离数据库上完成。
- `SKIP` 表示未提供前置 DSN；这并不代表就绪。
- 非零失败表示仅评审安全性、作用域、持久化或 pgvector 前置条件未通过，必须在任何策略变更之前进行调查。
