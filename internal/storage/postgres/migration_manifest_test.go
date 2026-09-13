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
	if len(first) != 9 {
		t.Fatalf("manifest length = %d, want 9: %+v", len(first), first)
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
	if first[3].Version != 4 || first[3].Name != "0004_hierarchical_memory_chunks.up.sql" {
		t.Fatalf("manifest entry = %+v, want version 4 hierarchical chunks", first[3])
	}
	if first[4].Version != 5 || first[4].Name != "0005_stable_hybrid_candidate_fusion.up.sql" {
		t.Fatalf("manifest entry = %+v, want version 5 stable fusion", first[4])
	}
	if first[5].Version != 6 || first[5].Name != "0006_diversity_policy.up.sql" {
		t.Fatalf("manifest entry = %+v, want version 6 diversity policy", first[5])
	}
	if first[6].Version != 7 || first[6].Name != "0007_query_analysis_rollout.up.sql" {
		t.Fatalf("manifest entry = %+v, want version 7 query-analysis rollout", first[6])
	}
	if first[7].Version != 8 || first[7].Name != "0008_quality_aware_reranking.up.sql" {
		t.Fatalf("manifest entry = %+v, want version 8 quality-aware reranking", first[7])
	}
	if first[8].Version != 9 || first[8].Name != "0009_durable_maintenance.up.sql" {
		t.Fatalf("manifest entry = %+v, want version 9 durable maintenance", first[8])
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
