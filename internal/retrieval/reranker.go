package retrieval

import (
	"context"
	"fmt"
	"math"
	"sort"
)

type RerankCandidate struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
type RerankRequest struct {
	Query      string
	Candidates []RerankCandidate
}
type RerankScore struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

type Reranker interface {
	Rerank(context.Context, RerankRequest) ([]RerankScore, error)
}

type RerankerMode string

const (
	RerankerModeDisabled    RerankerMode = "disabled"
	RerankerModeDiagnostics RerankerMode = "diagnostics_only"
	RerankerModeShadow      RerankerMode = "shadow"
	RerankerModeActive      RerankerMode = "active_for_scope"
)

func (m RerankerMode) Valid() bool {
	return m == RerankerModeDisabled || m == RerankerModeDiagnostics || m == RerankerModeShadow || m == RerankerModeActive
}

func ValidateRerankResponse(known []RerankCandidate, scores []RerankScore) error {
	allowed := make(map[string]struct{}, len(known))
	for _, candidate := range known {
		if candidate.ID == "" {
			return fmt.Errorf("rerank candidate id is required")
		}
		allowed[candidate.ID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(scores))
	for _, score := range scores {
		if _, ok := allowed[score.ID]; !ok {
			return fmt.Errorf("reranker returned unknown candidate %q", score.ID)
		}
		if _, ok := seen[score.ID]; ok {
			return fmt.Errorf("reranker returned duplicate candidate %q", score.ID)
		}
		if math.IsNaN(score.Score) || math.IsInf(score.Score, 0) {
			return fmt.Errorf("reranker returned invalid score")
		}
		seen[score.ID] = struct{}{}
	}
	return nil
}

type StaticReranker struct{ Scores map[string]float64 }

func (r StaticReranker) Rerank(_ context.Context, req RerankRequest) ([]RerankScore, error) {
	out := make([]RerankScore, 0, len(req.Candidates))
	for _, c := range req.Candidates {
		out = append(out, RerankScore{ID: c.ID, Score: r.Scores[c.ID]})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ID < out[j].ID
		}
		return out[i].Score > out[j].Score
	})
	return out, ValidateRerankResponse(req.Candidates, out)
}
