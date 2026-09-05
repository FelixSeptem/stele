package postgres

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// MigrationAsset is the immutable identity of one embedded forward migration.
// The checksum deliberately covers the exact embedded bytes, so changing a
// released asset is detectable without depending on database formatting.
type MigrationAsset struct {
	Version        int64
	Name           string
	ChecksumSHA256 string
}

var CurrentMigrationVersion = mustCurrentMigrationVersion()

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

// MigrationManifest returns the ordered immutable forward migration assets
// compiled into this binary. It returns a new slice on each call so callers
// cannot mutate shared process state.
func MigrationManifest() ([]MigrationAsset, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migration manifest: %w", err)
	}

	manifest := make([]MigrationAsset, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		asset, err := parseMigrationAsset(entry.Name())
		if err != nil {
			return nil, err
		}
		contents, err := migrationFS.ReadFile("migrations/" + asset.Name)
		if err != nil {
			return nil, fmt.Errorf("read migration asset %q: %w", asset.Name, err)
		}
		digest := sha256.Sum256(contents)
		asset.ChecksumSHA256 = hex.EncodeToString(digest[:])
		manifest = append(manifest, asset)
	}

	sort.Slice(manifest, func(i, j int) bool { return manifest[i].Version < manifest[j].Version })
	for i, asset := range manifest {
		if asset.Version != int64(i+1) {
			return nil, fmt.Errorf("migration manifest is not contiguous at version %d", asset.Version)
		}
	}
	if len(manifest) == 0 {
		return nil, fmt.Errorf("migration manifest has no forward assets")
	}
	return manifest, nil
}

func parseMigrationAsset(name string) (MigrationAsset, error) {
	if !strings.HasSuffix(name, ".up.sql") {
		return MigrationAsset{}, fmt.Errorf("migration asset %q is not a forward SQL migration", name)
	}
	prefix, rest, ok := strings.Cut(name, "_")
	if !ok || prefix == "" || rest == "" {
		return MigrationAsset{}, fmt.Errorf("migration asset %q has invalid name", name)
	}
	if len(prefix) < 4 || strings.Trim(prefix, "0123456789") != "" {
		return MigrationAsset{}, fmt.Errorf("migration asset %q has invalid version", name)
	}
	version, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil || version <= 0 {
		return MigrationAsset{}, fmt.Errorf("migration asset %q has invalid version", name)
	}
	if strings.TrimSuffix(rest, ".up.sql") == "" {
		return MigrationAsset{}, fmt.Errorf("migration asset %q has empty description", name)
	}
	return MigrationAsset{Version: version, Name: name}, nil
}

func mustCurrentMigrationVersion() int64 {
	manifest, err := MigrationManifest()
	if err != nil {
		panic(err)
	}
	return manifest[len(manifest)-1].Version
}

func BaseSchemaSQL() (string, error) {
	sql, err := migrationFS.ReadFile("migrations/0001_base_schema.up.sql")
	if err != nil {
		return "", fmt.Errorf("read base schema migration: %w", err)
	}

	return string(sql), nil
}
