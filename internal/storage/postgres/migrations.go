package postgres

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func MigrationAssets() ([]string, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migration assets: %w", err)
	}

	assets := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		assets = append(assets, entry.Name())
	}
	sort.Strings(assets)
	return assets, nil
}

func BaseSchemaSQL() (string, error) {
	sql, err := migrationFS.ReadFile("migrations/0001_base_schema.up.sql")
	if err != nil {
		return "", fmt.Errorf("read base schema migration: %w", err)
	}

	return string(sql), nil
}
