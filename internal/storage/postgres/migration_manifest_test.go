package postgres

import (
	"strings"
	"testing"
)

func TestMigrationManifestIsDeterministicAndChecksummed(t *testing.T) {
	first, err := MigrationManifest()
	if err != nil {
		t.Fatalf("MigrationManifest() first call error = %v", err)
	}
	second, err := MigrationManifest()
	if err != nil {
		t.Fatalf("MigrationManifest() second call error = %v", err)
	}
	if len(first) != 3 {
		t.Fatalf("manifest length = %d, want 3: %+v", len(first), first)
	}
	if first[0] != second[0] {
		t.Fatalf("manifest is not deterministic: first=%+v second=%+v", first[0], second[0])
	}
	if first[0].Version != 1 || first[0].Name != "0001_base_schema.up.sql" {
		t.Fatalf("manifest entry = %+v, want version 1 base schema", first[0])
	}
	if first[1].Version != 2 || first[1].Name != "0002_context_projections.up.sql" {
		t.Fatalf("manifest entry = %+v, want version 2 context projections", first[1])
	}
	if first[2].Version != 3 || first[2].Name != "0003_governed_memory_intents_reflection_compaction.up.sql" {
		t.Fatalf("manifest entry = %+v, want version 3 governed memory", first[2])
	}
	if len(first[0].ChecksumSHA256) != 64 {
		t.Fatalf("checksum length = %d, want 64", len(first[0].ChecksumSHA256))
	}
	if strings.Trim(first[0].ChecksumSHA256, "0123456789abcdef") != "" {
		t.Fatalf("checksum = %q, want lowercase hexadecimal", first[0].ChecksumSHA256)
	}
}

func TestParseMigrationAssetAcceptsOnlyOrderedUpSQL(t *testing.T) {
	asset, err := parseMigrationAsset("0001_base_schema.up.sql")
	if err != nil {
		t.Fatalf("parseMigrationAsset() error = %v", err)
	}
	if asset.Version != 1 || asset.Name != "0001_base_schema.up.sql" {
		t.Fatalf("asset = %+v", asset)
	}

	for _, name := range []string{
		"base_schema.up.sql",
		"0000_base_schema.up.sql",
		"0001_base_schema.down.sql",
		"0001_base_schema.sql",
		"0001_base_schema.up.txt",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseMigrationAsset(name); err == nil {
				t.Fatalf("parseMigrationAsset(%q) error = nil, want rejection", name)
			}
		})
	}
}
