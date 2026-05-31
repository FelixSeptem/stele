package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func OpenPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}

type bootstrapDB interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

func BootstrapDatabase(ctx context.Context, db bootstrapDB) error {
	sql, err := BaseSchemaSQL()
	if err != nil {
		return err
	}

	return applySQL(ctx, db, sql)
}

func applySQL(ctx context.Context, db bootstrapDB, sql string) error {
	if db == nil {
		return fmt.Errorf("bootstrap database is required")
	}

	if strings.TrimSpace(sql) == "" {
		return fmt.Errorf("bootstrap SQL is empty")
	}

	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin bootstrap transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, stmt := range splitSQLStatements(sql) {
		if strings.TrimSpace(stmt) == "" {
			continue
		}

		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("execute bootstrap statement: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit bootstrap transaction: %w", err)
	}

	return nil
}

func splitSQLStatements(sql string) []string {
	lines := strings.Split(sql, ";")
	statements := make([]string, 0, len(lines))
	for _, part := range lines {
		stmt := strings.TrimSpace(part)
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}

	return statements
}
