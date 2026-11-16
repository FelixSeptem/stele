# 分层记忆分块

Stele 的分层分块表示是一个 derived PostgreSQL projection：它构建在不可变 raw events 与 canonical-memory 版本之上。一个分块记录其确切的源类型/ID/版本、适用时的父记忆、源会话/用户、字符与 token 边界、策略/渲染器/计数器版本以及生命周期快照。它从不替代规范记忆，并且可以从持久化源记录重建。

## 策略与发布

确定性分块器在应用按记忆类别设定的硬上限之前，优先使用消息、段落、句子、列表与围栏代码边界。profile 与 relation 记录偏好原子事实；episodic 记录偏好事件单元；procedural 记录保留有界的步骤组；summary 使用更大的有界覆盖单元。

分块物化与消费是相互独立的控制项：

- `default_off`：规范检索保持不变；
- `shadow`：授权评估可以比较分块候选，但公开结果不变；
- `active`：分块证据可以影响排名，同时规范父身份与引用保留在响应中。

## 安全与重建

每个操作都要求确切的 `tenant/project/namespace` 作用域。父级与相邻扩展还会校验会话/用户断言、活动生命周期状态以及字符/token 预算。隐藏、过期、外部或不可验证的源会失败即关闭（fail closed）。对相同源/版本与策略标识的重复物化是幂等的（idempotent）；源或策略发生变化会创建新的派生历史条目。

回滚（rollback）路径是禁用物化或分块消费。不需要破坏性 down migration，也不需要重写规范记忆。

## 自有栈验证

使用一个由测试 harness 明确自有的可丢弃 PostgreSQL + pgvector 数据库：

```powershell
$env:STELE_TEST_POSTGRES_CHUNK_DSN = '<owned-test-dsn>'
go test ./internal/storage/postgres -run MemoryChunkPostgres -count=1
```

不要在此集成检查中使用 `STELE_POSTGRES_DSN` 或运维数据库。

安全行为：不可验证的源会失败即关闭（fail closed）。
