# 受治理的矛盾洞察

矛盾检测是一种离线与 shadow 能力，由现有推理候选信封支撑。它只在一个 `tenant/project/namespace` 作用域内对精确规范化键分组，比较半开有效性区间，并记录 `contradiction`、`temporal_coexistence` 或 `unresolved_temporal`。缺失或无效的时间证据永远不会成为活动洞察。

候选会保留两个源版本、证据引用与摘要、源水位、provider/schema/policy 版本、重叠边界、不确定性、评审状态以及确定性重放标识。PostgreSQL 仍是系统记录。候选持久化是仅追加的，并在作用域重放标识上幂等；源更正会将派生记录标记为过期，而不是重写任一规范事实。

保留的激活策略默认禁用。操作员必须创建一个独立版本化的精确作用域策略，满足证据、重叠、新鲜度和不确定性门禁，并确认候选评审，之后 apply 请求才能激活它。apply 请求使用已有的激活决策和生命周期账本。重复相同的重放标识会返回重复决策，而不会创建另一个活动版本。回滚会禁用新的准入，同时保留候选、评审和审计历史。

普通检索和上下文组装会排除矛盾候选、shadow 结果、未解决或共存的区间、过期记录以及需要评审的记录。经授权的矛盾上下文请求只能包含来自精确作用域和源水位的新鲜、活动、已确认的重叠洞察，并受显式条目预算约束。

遥测和日志仅暴露时间处置、评审、新鲜度、回退和激活的固定类别。源声明、内容、作用域、标识符、Provider 负载、提示词和原始错误不会被输出。

## 验证

运行测试时使用仓库缓存：

```powershell
$env:GOCACHE='D:\code\stele\.gocache-codex-contradiction'
go test ./...
go test -race ./...
go vet ./...
openspec validate --all --strict
git diff --check
```

PostgreSQL 集成测试通过 `STELE_TEST_POSTGRES_REASONING_DSN` 选择加入。它验证 pgvector 扩展和候选持久化。仅对明确拥有的可弃数据库运行该测试；处于较旧迁移版本的 schema 必须先升级，测试才能被视为证据。
