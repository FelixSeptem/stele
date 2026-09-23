package retrieval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

const ContextEfficiencySchemaVersionV1 = "context-efficiency-v1"

type ContextEfficiencyMetrics struct {
	RelevantTokenRatio    float64 `json:"relevant_token_ratio"`
	EvidenceDensity       float64 `json:"evidence_density"`
	DuplicateTokenRate    float64 `json:"duplicate_token_rate"`
	StaleTokenRate        float64 `json:"stale_token_rate"`
	QualityPerBudget      float64 `json:"quality_per_budget"`
	CandidateCount        int     `json:"candidate_count"`
	SelectedContextTokens int     `json:"selected_context_tokens"`
	ContextBudgetTokens   int     `json:"context_budget_tokens"`
	ElapsedMS             float64 `json:"elapsed_ms"`
	LatencyBucket         string  `json:"latency_bucket,omitempty"`
}

type ContextEfficiencyIdentity struct {
	SchemaVersion            string `json:"schema_version"`
	FixtureVersion           string `json:"fixture_version"`
	RepresentationVersion    string `json:"representation_version"`
	RendererVersion          string `json:"renderer_version,omitempty"`
	RetrievalPlanVersion     string `json:"retrieval_plan_version,omitempty"`
	TemporalPolicyVersion    string `json:"temporal_policy_version,omitempty"`
	GraphPolicyVersion       string `json:"graph_policy_version,omitempty"`
	FeedbackSummaryVersion   string `json:"feedback_summary_version,omitempty"`
	CalibrationPolicyVersion string `json:"calibration_policy_version,omitempty"`
	ReleasePolicyVersion     string `json:"release_policy_version,omitempty"`
}

type ContextEfficiencyEvidence struct {
	Identity    ContextEfficiencyIdentity `json:"identity"`
	Metrics     ContextEfficiencyMetrics  `json:"metrics"`
	GeneratedAt time.Time                 `json:"generated_at"`
}

type ContextEfficiencyDiagnosticCategory string

const (
	ContextEfficiencyDiagnosticInsufficientEvidence ContextEfficiencyDiagnosticCategory = "insufficient_evidence"
	ContextEfficiencyDiagnosticSummaryStale         ContextEfficiencyDiagnosticCategory = "summary_stale"
	ContextEfficiencyDiagnosticBaselineFallback     ContextEfficiencyDiagnosticCategory = "baseline_fallback"
	ContextEfficiencyDiagnosticBudgetBounded        ContextEfficiencyDiagnosticCategory = "budget_bounded"
)

type ContextEfficiencyHardFailureCategory string

const (
	ContextEfficiencyFailureProtectedRecall  ContextEfficiencyHardFailureCategory = "protected_recall_loss"
	ContextEfficiencyFailureIsolation        ContextEfficiencyHardFailureCategory = "scope_or_lifecycle_failure"
	ContextEfficiencyFailureCitation         ContextEfficiencyHardFailureCategory = "citation_failure"
	ContextEfficiencyFailureBudget           ContextEfficiencyHardFailureCategory = "budget_overflow"
	ContextEfficiencyFailureNondeterministic ContextEfficiencyHardFailureCategory = "nondeterministic_replay"
	ContextEfficiencyFailureSummaryStale     ContextEfficiencyHardFailureCategory = "summary_stale"
	ContextEfficiencyFailureRollback         ContextEfficiencyHardFailureCategory = "rollback_failure"
	ContextEfficiencyFailureAPILLeak         ContextEfficiencyHardFailureCategory = "ordinary_api_leakage"
)

func (m ContextEfficiencyMetrics) Validate() error {
	for name, value := range map[string]float64{
		"relevant token ratio": m.RelevantTokenRatio,
		"evidence density":     m.EvidenceDensity,
		"duplicate token rate": m.DuplicateTokenRate,
		"stale token rate":     m.StaleTokenRate,
		"quality per budget":   m.QualityPerBudget,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			return fmt.Errorf("%s must be between 0 and 1", name)
		}
	}
	if m.CandidateCount < 0 || m.CandidateCount > 10000 || m.SelectedContextTokens < 0 || m.SelectedContextTokens > 1_000_000 || m.ContextBudgetTokens < 0 || m.ContextBudgetTokens > 1_000_000 || math.IsNaN(m.ElapsedMS) || math.IsInf(m.ElapsedMS, 0) || m.ElapsedMS < 0 || m.ElapsedMS > 300_000 {
		return fmt.Errorf("context efficiency counts or elapsed time are outside bounded limits")
	}
	if strings.TrimSpace(m.LatencyBucket) != "" && len(m.LatencyBucket) > 32 {
		return fmt.Errorf("latency bucket is too long")
	}
	return nil
}

func (i ContextEfficiencyIdentity) Validate() error {
	if i.SchemaVersion == "" {
		i.SchemaVersion = ContextEfficiencySchemaVersionV1
	}
	if i.SchemaVersion != ContextEfficiencySchemaVersionV1 {
		return fmt.Errorf("unsupported context efficiency schema version")
	}
	if strings.TrimSpace(i.FixtureVersion) == "" || strings.TrimSpace(i.RepresentationVersion) == "" {
		return fmt.Errorf("fixture and representation versions are required")
	}
	for name, value := range map[string]string{"fixture": i.FixtureVersion, "representation": i.RepresentationVersion, "renderer": i.RendererVersion, "retrieval plan": i.RetrievalPlanVersion, "temporal policy": i.TemporalPolicyVersion, "graph policy": i.GraphPolicyVersion, "feedback summary": i.FeedbackSummaryVersion, "calibration policy": i.CalibrationPolicyVersion, "release policy": i.ReleasePolicyVersion} {
		if len(value) > 128 {
			return fmt.Errorf("%s version is too long", name)
		}
	}
	return nil
}

func (e ContextEfficiencyEvidence) Validate() error {
	if err := e.Identity.Validate(); err != nil {
		return err
	}
	if err := e.Metrics.Validate(); err != nil {
		return err
	}
	if e.GeneratedAt.IsZero() {
		return fmt.Errorf("generated at is required")
	}
	return nil
}

func (e ContextEfficiencyEvidence) StableIdentity() (string, error) {
	if err := e.Validate(); err != nil {
		return "", err
	}
	b, err := json.Marshal(struct {
		Identity ContextEfficiencyIdentity `json:"identity"`
		Metrics  ContextEfficiencyMetrics  `json:"metrics"`
	}{e.Identity, e.Metrics})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(b)
	return "efficiency-" + hex.EncodeToString(hash[:]), nil
}
