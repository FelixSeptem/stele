# 持久化 scheduler 运行历史

Stele 在 PostgreSQL 中存储 scheduler 运行摘要和仅追加（append-only）的尝试详情。稳定的运行键派生自作业类别、规范化的精确租户/项目/命名空间作用域以及节奏窗口。重复的 scheduler 滴答会记录重复处置，并且不执行维护主体。

## 检查运行

使用带精确作用域的管理面：

```text
GET /v1/admin/jobs/run-history?limit=20&state=completed&cursor=<opaque>
X-API-Key: <admin-key>
X-Stele-Tenant: <tenant>
X-Stele-Project: <project>
X-Stele-Namespace: <namespace>
```

支持的过滤器为 `job_class`、`state`、`recovery`、`observed_from` 和 `observed_to`。响应按观察时间和运行键排序，并返回不透明延续游标。仓库在解析运行是否存在之前应用精确作用域；未授权作用域不会透露计数或记录。

使用以下方式读取单个运行及其有界尝试历史：

```text
GET /v1/admin/jobs/run-history/<run-key>
```

原始错误、凭据、负载、源内容和无关作用域数据绝不会包含在响应或 scheduler 遥测中。

## 恢复与取消

操作员可以通过受治理的管理操作取消待处理、重试中或失败的运行：

```text
POST /v1/admin/jobs/run-history/<run-key>?action=cancel
```

存在活动租约时取消会失败。恢复使用相同操作并指定 `action=recover`；它会记录先前的终态历史，并将运行返回到普通 scheduler 认领路径。它从不抢占活动租约，也不内联执行维护主体。

重试耗尽即为终态，不会自动重新入队。恢复必须是显式的，并作为恢复转换保持可见。

## 保留策略与新鲜度

高流量尝试详情会在配置的详情窗口之后被清理。终态摘要、尝试计数、检查点/源水位标识、终态处置以及必需的审计转换会保留。清理是幂等（idempotent）的，并输出有界的保留策略遥测。过期或分叉的水位证据不符合就绪或一致性声明的条件。

scheduler 生命周期指标是 `stele_scheduler_runs_total`。其标签是操作、结果、状态、租约、重试、恢复、记录、新鲜度、SLO 和时长分桶的固定类别；作用域值和标识符会被拒绝或规范化为 `unknown`。
