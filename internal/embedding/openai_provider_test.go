package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAIProviderGenerateEmbeddingUsesConfiguredEndpointAndDimensions(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/embeddings" {
			t.Fatalf("path = %s, want /embeddings", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-openai-key" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("json.NewDecoder() error = %v", err)
		}

		if payload["model"] != "text-embedding-3-small" {
			t.Fatalf("model = %v, want text-embedding-3-small", payload["model"])
		}
		if payload["input"] != "hello semantic world" {
			t.Fatalf("input = %v, want hello semantic world", payload["input"])
		}
		if payload["dimensions"] != float64(1536) {
			t.Fatalf("dimensions = %v, want 1536", payload["dimensions"])
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"embedding": []float64{0.1, 0.2, 0.3}},
			},
		}); err != nil {
			t.Fatalf("json.NewEncoder() error = %v", err)
		}
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(OpenAIProviderConfig{
		APIKey:     "test-openai-key",
		BaseURL:    server.URL,
		Timeout:    5 * time.Second,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	result, err := provider.GenerateEmbedding(context.Background(), ProviderRequest{
		Text: "hello semantic world",
		Target: Target{
			Provider:   "openai",
			Model:      "text-embedding-3-small",
			Dimensions: 1536,
		},
	})
	if err != nil {
		t.Fatalf("GenerateEmbedding() error = %v", err)
	}

	if result.Provider != "openai" {
		t.Fatalf("Provider = %q, want openai", result.Provider)
	}
	if result.Model != "text-embedding-3-small" {
		t.Fatalf("Model = %q, want text-embedding-3-small", result.Model)
	}
	if result.Dimensions != 1536 {
		t.Fatalf("Dimensions = %d, want 1536", result.Dimensions)
	}
	if len(result.Embedding) != 3 {
		t.Fatalf("len(Embedding) = %d, want 3", len(result.Embedding))
	}
}
