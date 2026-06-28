package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OpenAIProviderConfig struct {
	APIKey     string
	BaseURL    string
	Timeout    time.Duration
	HTTPClient *http.Client
}

type openAIProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

type openAIEmbeddingRequest struct {
	Model      string `json:"model"`
	Input      string `json:"input"`
	Dimensions int    `json:"dimensions,omitempty"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func NewOpenAIProvider(cfg OpenAIProviderConfig) (Provider, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("openai embedding api key is required")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("openai embedding base url is required")
	}
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("parse openai embedding base url: %w", err)
	}

	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}

	return &openAIProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  client,
	}, nil
}

func (p *openAIProvider) GenerateEmbedding(ctx context.Context, input ProviderRequest) (ProviderResult, error) {
	if err := input.Target.Validate(); err != nil {
		return ProviderResult{}, err
	}
	if strings.TrimSpace(input.Text) == "" {
		return ProviderResult{}, fmt.Errorf("embedding input text is required")
	}

	payload, err := json.Marshal(openAIEmbeddingRequest{
		Model:      input.Target.Model,
		Input:      input.Text,
		Dimensions: input.Target.Dimensions,
	})
	if err != nil {
		return ProviderResult{}, fmt.Errorf("marshal openai embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return ProviderResult{}, fmt.Errorf("build openai embedding request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return ProviderResult{}, fmt.Errorf("openai embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return ProviderResult{}, fmt.Errorf("openai embedding request returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded openAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ProviderResult{}, fmt.Errorf("decode openai embedding response: %w", err)
	}
	if len(decoded.Data) == 0 || len(decoded.Data[0].Embedding) == 0 {
		return ProviderResult{}, fmt.Errorf("openai embedding response is missing embedding data")
	}

	embedding := make([]float32, len(decoded.Data[0].Embedding))
	for i, value := range decoded.Data[0].Embedding {
		embedding[i] = float32(value)
	}

	return ProviderResult{
		Provider:   strings.TrimSpace(input.Target.Provider),
		Model:      strings.TrimSpace(input.Target.Model),
		Dimensions: input.Target.Dimensions,
		Embedding:  embedding,
	}, nil
}
