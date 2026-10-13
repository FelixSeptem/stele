package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

// ContextSelectionManifest contains only stable nonsecret behavior identity.
// The HTTP boundary checks it both before and after assembly. Policy changes
// during that read fail closed; this does not claim a memory snapshot.
func (s *Service) ContextSelectionManifest(ctx context.Context, scope memory.Scope, session string) ([]byte, error) {
	type policyIdentity struct {
		Kind          string          `json:"kind"`
		Configuration json.RawMessage `json:"configuration"`
	}
	manifest := struct {
		Version       string                  `json:"manifest_version"`
		Eligibility   string                  `json:"eligibility_version"`
		Temporal      string                  `json:"temporal_version"`
		Selection     string                  `json:"selection_version"`
		Disclosure    string                  `json:"disclosure_version"`
		Source        string                  `json:"source_metadata_version"`
		Budget        string                  `json:"budget_version"`
		Packing       string                  `json:"packing_version"`
		Fusion        any                     `json:"fusion"`
		Quality       any                     `json:"quality"`
		Reranker      any                     `json:"reranker"`
		Graph         any                     `json:"graph"`
		Planner       any                     `json:"planner"`
		ChunkMode     memory.ChunkRolloutMode `json:"chunk_mode"`
		QueryAnalyzer bool                    `json:"query_analyzer"`
		QueryLimits   any                     `json:"query_limits"`
		Policies      []policyIdentity        `json:"policies"`
	}{"context-selection-manifest-v1", "active-current-disclosable-v1", "current-valid-source-v1", "context-selection-v1", "context-disclosure-v1", "source-metadata-v1", "context-budget-v1", "context-packing-v1", s.fusionStrategy, s.qualityBounds, struct {
		Mode              RerankerMode
		Provider, Version string
	}{s.rerankerMode, s.rerankerProvider, s.rerankerVersion}, s.graphTraversalLimits, s.retrievalPlanPolicy, s.chunkRollout, s.queryAnalyzer != nil, s.queryAnalysisLimits, []policyIdentity{}}
	add := func(kind string, p *memory.RankingRolloutPolicy) error {
		if p == nil {
			return nil
		}
		// Allowlist excludes actor/reason/history/attestation/private evidence.
		raw, err := json.Marshal(p)
		if err != nil {
			return err
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(raw, &fields); err != nil {
			return err
		}
		allowed := []string{"id", "status", "mode", "surfaces", "signal_sources", "threshold_status", "evidence_minimum", "fusion_strategy", "fusion_version", "fusion_rank_constant", "fusion_channel_weights", "fusion_per_channel_candidate", "fusion_total_candidates", "quality_feature_version", "reranker_provider", "reranker_version", "reranker_mode", "diversity_policy_name", "diversity_policy_version", "diversity_mmr_lambda", "diversity_semantic_threshold", "diversity_max_candidates", "diversity_max_pairwise_comparisons", "diversity_max_embedding_dimensions", "diversity_max_citations_per_candidate", "diversity_coverage_weights", "query_analysis_selector", "query_analysis", "retrieval_planner_selector", "retrieval_planner"}
		config := map[string]json.RawMessage{}
		for _, name := range allowed {
			if v, ok := fields[name]; ok {
				config[name] = v
			}
		}
		raw, err = json.Marshal(config)
		if err != nil {
			return err
		}
		manifest.Policies = append(manifest.Policies, policyIdentity{kind, raw})
		return nil
	}
	if s.rankingRolloutPolicyReader != nil {
		p, err := s.readActiveRankingPolicy(ctx, scope, memory.RankingRolloutSurfaceContext)
		if err != nil {
			return nil, err
		}
		if err = add("context", p); err != nil {
			return nil, err
		}
		if s.queryAnalyzer != nil {
			if reader, ok := s.rankingRolloutPolicyReader.(effectiveQueryAnalysisPolicyReader); ok {
				p, err := reader.ReadEffectiveQueryAnalysisRolloutPolicy(ctx, memory.ReadEffectiveQueryAnalysisRolloutPolicyInput{Scope: scope, Surface: memory.RankingRolloutSurfaceContext, SessionID: session})
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return nil, err
				}
				if err == nil {
					if p.Scope.Normalized() != scope.Normalized() {
						return nil, errors.New("policy scope mismatch")
					}
					if err = add("query_analysis", &p); err != nil {
						return nil, err
					}
				}
			}
		}
		if reader, ok := s.rankingRolloutPolicyReader.(effectiveRetrievalPlannerPolicyReader); ok {
			p, err := reader.ReadEffectiveRetrievalPlannerRolloutPolicy(ctx, memory.ReadEffectiveRetrievalPlannerRolloutPolicyInput{Scope: scope, Surface: memory.RankingRolloutSurfaceContext, SessionID: session})
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
			if err == nil {
				if p.Scope.Normalized() != scope.Normalized() {
					return nil, errors.New("policy scope mismatch")
				}
				if err = add("retrieval_planner", &p); err != nil {
					return nil, err
				}
			}
		}
	}
	// Encode configuration decimal numbers as precise strings. No rank outputs,
	// clocks, URLs, DSNs, credentials or candidate values enter the document.
	b, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	var object any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if err = decoder.Decode(&object); err != nil {
		return nil, err
	}
	return json.Marshal(decimalConfig(object))
}
func decimalConfig(v any) any {
	switch x := v.(type) {
	case json.Number:
		if _, err := strconv.ParseInt(string(x), 10, 64); err != nil {
			return string(x)
		}
		return x
	case map[string]any:
		for k, v := range x {
			x[k] = decimalConfig(v)
		}
	case []any:
		for i, v := range x {
			x[i] = decimalConfig(v)
		}
	}
	return v
}
