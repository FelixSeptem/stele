package postgres

import (
	"embed"
	"fmt"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func BaseSchemaSQL() (string, error) {
	sql, err := migrationFS.ReadFile("migrations/0001_base_schema.up.sql")
	if err != nil {
		return "", fmt.Errorf("read base schema migration: %w", err)
	}

	return string(sql), nil
}
