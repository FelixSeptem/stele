package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type OpenAIReranker struct {
	Endpoint      string
	APIKey        string
	Model         string
	Timeout       time.Duration
	MaxCandidates int
	MaxTextBytes  int
	Client        *http.Client
}
type openAIRerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}
type openAIRerankResponse struct {
	Results []struct {
		Index int     `json:"index"`
		Score float64 `json:"relevance_score"`
	} `json:"results"`
}

func (r OpenAIReranker) Rerank(ctx context.Context, req RerankRequest) ([]RerankScore, error) {
	if strings.TrimSpace(r.Endpoint) == "" || strings.TrimSpace(r.Model) == "" {
		return nil, fmt.Errorf("reranker endpoint and model are required")
	}
	if r.MaxCandidates <= 0 || len(req.Candidates) > r.MaxCandidates {
		return nil, fmt.Errorf("reranker candidate limit exceeded")
	}
	for _, c := range req.Candidates {
		if r.MaxTextBytes > 0 && len([]byte(c.Text)) > r.MaxTextBytes {
			return nil, fmt.Errorf("reranker text limit exceeded")
		}
	}
	payload, err := json.Marshal(openAIRerankRequest{Model: r.Model, Query: req.Query, Documents: func() []string {
		out := make([]string, len(req.Candidates))
		for i, c := range req.Candidates {
			out[i] = c.Text
		}
		return out
	}()})
	if err != nil {
		return nil, fmt.Errorf("reranker request encode failed")
	}
	client := r.Client
	if client == nil {
		client = &http.Client{}
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("reranker request failed")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if r.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.APIKey)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("reranker provider unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("reranker provider returned status %d", resp.StatusCode)
	}
	var decoded openAIRerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("reranker response malformed")
	}
	out := make([]RerankScore, 0, len(decoded.Results))
	for _, item := range decoded.Results {
		if item.Index < 0 || item.Index >= len(req.Candidates) {
			return nil, fmt.Errorf("reranker response index out of bounds")
		}
		out = append(out, RerankScore{ID: req.Candidates[item.Index].ID, Score: item.Score})
	}
	if err := ValidateRerankResponse(req.Candidates, out); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ID < out[j].ID
		}
		return out[i].Score > out[j].Score
	})
	return out, nil
}
