# 受治理的目标评审与实验性可见性一致性

此变更新增一个精确作用域、已脱敏的目标评审投影，以及一个可独立选择加入的 `goal_context` 部分。该部分不会出现在普通上下文请求中，仅在精确作用域存在当前已启用的可见性策略时由 PostgreSQL 读取器输出。Provider 输出和重放不会绕过该边界。

## 验证矩阵

- `go test ./internal/insights ./internal/retrieval ./internal/storage/postgres ./internal/app ./openapi -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- `openspec validate governed-goal-review-and-experimental-visibility --type change`
- `openspec validate --all --strict`

已脱敏的管理路由是 `GET /v1/admin/goals/review`。它为已认证的精确作用域返回有界评审和策略类别，且从不选取目标标题、摘要、提示词、Provider 负载或外部标识符。公开上下文请求接受 `include_goal_context`；默认值为 false，当该标志缺失或策略未通过时，普通部分保持不变。

## PostgreSQL / pgvector 运行

使用现有的可弃 pgvector 镜像，并在运行仓库一致性脚本前设置 `STELE_TEST_POSTGRES_GOAL_DSN`。该迁移是增量式（additive）的（`0032_goal_visibility`），可通过其配对的 down 迁移回滚。不要提交凭据或 DSN。
