package config

import "testing"

func TestLoadFromEnvDefaultsMCPDisabledWithBoundedLimits(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.MCP.Enabled {
		t.Fatal("MCP enabled by default, want disabled")
	}
	if cfg.MCP.Path != "/mcp" {
		t.Fatalf("MCP path = %q, want /mcp", cfg.MCP.Path)
	}
	if cfg.MCP.MaxQueryBytes <= 0 || cfg.MCP.MaxPayloadBytes <= 0 || cfg.MCP.MaxResults <= 0 || cfg.MCP.MaxIDs <= 0 {
		t.Fatalf("MCP limits = %+v, want positive bounded defaults", cfg.MCP)
	}
}

func TestLoadFromEnvParsesMCPConfigurationOnlyForAPI(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_MCP_ENABLED", "true")
	t.Setenv("STELE_MCP_PATH", "/v1/mcp")
	t.Setenv("STELE_MCP_MAX_QUERY_BYTES", "2048")
	t.Setenv("STELE_MCP_MAX_PAYLOAD_BYTES", "8192")
	t.Setenv("STELE_MCP_MAX_RESULTS", "25")
	t.Setenv("STELE_MCP_MAX_IDS", "10")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if !cfg.MCP.Enabled || cfg.MCP.Path != "/v1/mcp" || cfg.MCP.MaxQueryBytes != 2048 || cfg.MCP.MaxPayloadBytes != 8192 || cfg.MCP.MaxResults != 25 || cfg.MCP.MaxIDs != 10 {
		t.Fatalf("MCP config = %+v, want parsed values", cfg.MCP)
	}

	t.Setenv("STELE_MODE", "worker")
	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("LoadFromEnv() error = nil for MCP enabled outside API mode")
	}
}

func TestLoadFromEnvRejectsUnsafeMCPConfiguration(t *testing.T) {
	for _, setting := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "enabled without api", key: "STELE_MCP_ENABLED", value: "true"},
		{name: "empty path", key: "STELE_MCP_PATH", value: ""},
		{name: "zero query", key: "STELE_MCP_MAX_QUERY_BYTES", value: "0"},
		{name: "oversized payload", key: "STELE_MCP_MAX_PAYLOAD_BYTES", value: "1048577"},
		{name: "zero results", key: "STELE_MCP_MAX_RESULTS", value: "0"},
		{name: "oversized ids", key: "STELE_MCP_MAX_IDS", value: "1001"},
	} {
		t.Run(setting.name, func(t *testing.T) {
			t.Setenv("STELE_MODE", "api")
			t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
			t.Setenv(setting.key, setting.value)
			if setting.name == "enabled without api" {
				t.Setenv("STELE_MODE", "worker")
			}
			if _, err := LoadFromEnv(); err == nil {
				t.Fatalf("LoadFromEnv() error = nil for %s", setting.name)
			}
		})
	}
}
