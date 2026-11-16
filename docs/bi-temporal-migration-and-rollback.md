# 双时态事实有效性：迁移、恢复与回滚

本说明记录迁移 `0014_bi_temporal_fact_validity` 的归属、恢复与回滚流程。它的编写目标是让仅能访问作用域内 PostgreSQL DSN 的操作员即可执行：在此变更产生的任何面向操作员的证据中，都不会出现查询文本、记忆标识符、原始分数或 Provider 负载。

## 迁移更改的内容

`0014_bi_temporal_fact_validity.up.sql` 仅为增量式（additive）。它：

- 向 `canonical_memories` 和 `memory_versions` 添加 `temporal_fact_id`、`temporal_head_version`、`ingested_at`、`valid_from`、`valid_to` 和 `validity_source`；
- 以旧版当前兼容（legacy current-compatible）形态回填既有行（`temporal_fact_id = id::text`，`ingested_at = valid_from = created_at`，`validity_source = 'legacy_current_compatible'`，`valid_to` 留为 NULL 以保持该行为当前有效）；
- 创建 `temporal_corrections` 作为仅追加（append-only）的更正账本；
- 向 `relation_projections`、`memory_chunk_derivations` 和 `context_projection_items` 添加时间列；
- 在时间身份和有效边界上创建作用域安全的索引。

每个 `ALTER` 都使用 `ADD COLUMN IF NOT EXISTS`，每个 `CREATE` 都使用 `IF NOT EXISTS`，因此重新运行迁移是无操作。回填 `UPDATE` 语句由 `WHERE` 子句保护，会跳过已填充的行，因此它们也是幂等（idempotent）的。

## 归属

- 该迁移与其他所有版本化迁移由同一个有序运行器应用；没有单独的时间引导步骤。
- 回填在迁移事务内运行。它不是后台作业，也不存在可供推理的部分回填稳态。
- 更正历史由 `temporal_corrections` 拥有。行是仅追加（append-only）的；事实的当前头由 `canonical_memories.temporal_head_version` 跟踪，而不是通过派生 `MAX(successor_version)`，因为派生的最大值无法检测竞争的写入者。

## 恢复

如果迁移在事务中途失败，运行器会报告脏状态（dirty）版本。恢复步骤如下：

1. 检查运行器报告的脏状态（dirty）版本。不要手工编辑 `temporal_corrections` 或已回填的列。
2. 重新运行迁移。由于每条语句都由 `IF NOT EXISTS` 保护且回填是幂等（idempotent）的，重新运行会从停止处完成迁移。
3. 确认普通当前检索返回与迁移尝试之前相同的行。旧版当前兼容（legacy current-compatible）行保持当前有效，因此失败或重复的迁移不得使既有记忆消失。

没有破坏性修复步骤。如果确实需要手动修复，那将是一个新的前向迁移，绝不是就地重写。

## 回滚

有两条不同的回滚路径，它们不可互换。

### 运维回滚（operational rollback，受支持路径）

运维回滚会**禁用感知时间的策略解析**（disables temporal-aware policy resolution）并返回已批准的当前基线。它会：

- 停止应用有效时间选择器，并回退到基线当前检索路径；
- **从不删除时间历史**（never deletes temporal history）；
- **从不重写规范数据**（never rewrites canonical data）；
- 使每条更正、有效性区间和来源链接保持可读。

这是操作员在生产中使用的路径。禁用策略在构造上就是保留数据的：时间列和更正账本只是不再被查询。

### 模式回退（schema revert，`0014_bi_temporal_fact_validity.down.sql`）

`down` 资产用于本地开发和测试数据库，以便将 schema 恢复到 `0014` 之前的形态。它会删除时间列和 `temporal_corrections` 表，因此确实会丢弃（does discard）时间历史。

它**不是**生产回滚路径（not the production rollback path）。对必须保留时间历史的数据库运行它属于破坏性操作。请优先使用上面的运维回滚。

## 回滚演练

演练由 `internal/storage/postgres/temporal_rollback_rehearsal_test.go` 中的 `TestTemporalRollbackRehearsalLeavesHistoryIntactAndEmitsBoundedEvidence` 执行。它针对模拟数据库证明：

1. 禁用时间策略会返回基线当前选择——生成的 SQL 不携带任何有效时间谓词；
2. 禁用后时间历史仍可读——更正账本和版本历史仍返回其行；
3. 输出的审计证据是有界（bounded）的——它只包含低基数类别，不含查询文本、记忆标识符、原始分数和 DSN。

演练可重复且不更改数据，因此可以在上线前后运行而没有副作用。
