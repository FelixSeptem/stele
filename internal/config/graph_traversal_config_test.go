package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvDefaultsGraphTraversalHardCap(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://stele:stele@localhost:5432/stele?sslmode=disable")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.GraphTraversal.MaxHops != 3 || cfg.GraphTraversal.MaxElapsed != 250*time.Millisecond {
		t.Fatalf("graph traversal defaults = %+v", cfg.GraphTraversal)
	}
}

func TestLoadFromEnvParsesGraphTraversalBounds(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://stele:stele@localhost:5432/stele?sslmode=disable")
	t.Setenv("STELE_GRAPH_MAX_HOPS", "2")
	t.Setenv("STELE_GRAPH_MAX_SEEDS", "8")
	t.Setenv("STELE_GRAPH_MAX_ELAPSED", "125ms")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.GraphTraversal.MaxHops != 2 || cfg.GraphTraversal.MaxSeeds != 8 || cfg.GraphTraversal.MaxElapsed != 125*time.Millisecond {
		t.Fatalf("graph traversal = %+v", cfg.GraphTraversal)
	}
}

func TestLoadFromEnvRejectsGraphTraversalOverCap(t *testing.T) {
	t.Setenv("STELE_POSTGRES_DSN", "postgres://stele:stele@localhost:5432/stele?sslmode=disable")
	t.Setenv("STELE_GRAPH_MAX_HOPS", "4")
	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("LoadFromEnv() error = nil, want graph hop cap rejection")
	}
}
