package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvParsesEmbeddingDefaultsAndClassRoutes(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_EMBEDDING_DEFAULT_PROVIDER", "openai")
	t.Setenv("STELE_EMBEDDING_DEFAULT_MODEL", "text-embedding-3-small")
	t.Setenv("STELE_EMBEDDING_DEFAULT_DIMENSIONS", "1536")
	t.Setenv("STELE_EMBEDDING_CLASS_ROUTES", "summary=openai:text-embedding-3-large:3072,profile=openai:text-embedding-3-small:1536")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.Embedding.DefaultProvider != "openai" {
		t.Fatalf("DefaultProvider = %q, want openai", cfg.Embedding.DefaultProvider)
	}
	if cfg.Embedding.DefaultModel != "text-embedding-3-small" {
		t.Fatalf("DefaultModel = %q, want text-embedding-3-small", cfg.Embedding.DefaultModel)
	}
	if cfg.Embedding.DefaultDimensions != 1536 {
		t.Fatalf("DefaultDimensions = %d, want 1536", cfg.Embedding.DefaultDimensions)
	}
	if len(cfg.Embedding.ClassRoutes) != 2 {
		t.Fatalf("len(ClassRoutes) = %d, want 2", len(cfg.Embedding.ClassRoutes))
	}
	if cfg.Embedding.ClassRoutes["summary"].Model != "text-embedding-3-large" {
		t.Fatalf("summary route = %+v, want large model", cfg.Embedding.ClassRoutes["summary"])
	}
}

func TestLoadFromEnvRejectsInvalidEmbeddingRouteFormat(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_EMBEDDING_CLASS_ROUTES", "summary=bad-format")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want invalid embedding route format")
	}
}

func TestLoadFromEnvParsesOpenAIEmbeddingProviderSettings(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_EMBEDDING_OPENAI_API_KEY", "test-openai-key")
	t.Setenv("STELE_EMBEDDING_OPENAI_BASE_URL", "https://embeddings.example.com/v1")
	t.Setenv("STELE_EMBEDDING_OPENAI_TIMEOUT", "45s")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.Embedding.OpenAI.APIKey != "test-openai-key" {
		t.Fatalf("OpenAI.APIKey = %q, want test-openai-key", cfg.Embedding.OpenAI.APIKey)
	}
	if cfg.Embedding.OpenAI.BaseURL != "https://embeddings.example.com/v1" {
		t.Fatalf("OpenAI.BaseURL = %q, want configured base URL", cfg.Embedding.OpenAI.BaseURL)
	}
	if cfg.Embedding.OpenAI.Timeout != 45*time.Second {
		t.Fatalf("OpenAI.Timeout = %v, want 45s", cfg.Embedding.OpenAI.Timeout)
	}
}
