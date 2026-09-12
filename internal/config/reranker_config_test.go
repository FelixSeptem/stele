package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadFromEnvDefaultsRerankerDisabled(t *testing.T) {
	setRequiredConfigEnv(t)
	cfg, err := LoadFromEnv()
	if err != nil { t.Fatal(err) }
	if cfg.Reranker.Enabled || cfg.Reranker.Provider != "" { t.Fatalf("reranker default=%+v, want disabled", cfg.Reranker) }
}

func TestLoadFromEnvParsesRerankerSettings(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("STELE_RERANK_ENABLED", "true")
	t.Setenv("STELE_RERANK_PROVIDER", "openai-compatible")
	t.Setenv("STELE_RERANK_ENDPOINT", "http://localhost:9000/v1/rerank")
	t.Setenv("STELE_RERANK_MODEL", "bge-reranker-v2")
	t.Setenv("STELE_RERANK_API_KEY", "local-secret")
	t.Setenv("STELE_RERANK_TIMEOUT", "7s")
	t.Setenv("STELE_RERANK_MAX_CANDIDATES", "32")
	t.Setenv("STELE_RERANK_MAX_TEXT_BYTES", "4096")
	cfg, err := LoadFromEnv()
	if err != nil { t.Fatal(err) }
	if !cfg.Reranker.Enabled || cfg.Reranker.Provider != "openai-compatible" || cfg.Reranker.Endpoint == "" || cfg.Reranker.Model != "bge-reranker-v2" || cfg.Reranker.Timeout != 7*time.Second || cfg.Reranker.MaxCandidates != 32 || cfg.Reranker.MaxTextBytes != 4096 { t.Fatalf("reranker config=%+v", cfg.Reranker) }
}

func TestLoadFromEnvRejectsIncompleteEnabledReranker(t *testing.T) {
	setRequiredConfigEnv(t)
	t.Setenv("STELE_RERANK_ENABLED", "true")
	if _, err := LoadFromEnv(); err == nil { t.Fatal("expected enabled reranker validation error") }
}

func setRequiredConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("STELE_POSTGRES_DSN", "postgres://test")
	for _, key := range []string{"STELE_RERANK_ENABLED", "STELE_RERANK_PROVIDER", "STELE_RERANK_ENDPOINT", "STELE_RERANK_MODEL", "STELE_RERANK_API_KEY", "STELE_RERANK_TIMEOUT", "STELE_RERANK_MAX_CANDIDATES", "STELE_RERANK_MAX_TEXT_BYTES"} { _ = os.Unsetenv(key) }
}
