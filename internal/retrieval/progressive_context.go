package retrieval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type ProgressiveContextLevelKind string

const (
	ProgressiveContextLevelShortRetrieval ProgressiveContextLevelKind = "short_retrieval"
	ProgressiveContextLevelMediumOverview ProgressiveContextLevelKind = "medium_overview"
	ProgressiveContextLevelCanonicalChunk ProgressiveContextLevelKind = "canonical_chunk"
)

type ProgressiveContextFreshness string

const (
	ProgressiveContextFreshnessFresh   ProgressiveContextFreshness = "fresh"
	ProgressiveContextFreshnessStale   ProgressiveContextFreshness = "stale"
	ProgressiveContextFreshnessUnknown ProgressiveContextFreshness = "unknown"
)

type ProgressiveContextFailureReason string

const (
	ProgressiveContextFailureStale     ProgressiveContextFailureReason = "stale"
	ProgressiveContextFailureWatermark ProgressiveContextFailureReason = "watermark_missing_or_mismatch"
	ProgressiveContextFailureLifecycle ProgressiveContextFailureReason = "lifecycle_hidden"
	ProgressiveContextFailureIsolation ProgressiveContextFailureReason = "foreign_scope"
	ProgressiveContextFailureBudget    ProgressiveContextFailureReason = "budget_exceeded"
	ProgressiveContextFailureCitation  ProgressiveContextFailureReason = "citation_coverage"
)

type ProgressiveContextEvidence struct {
	Alias     string
	Memory    memory.CanonicalMemory
	Citations []Citation
}
type ProgressiveContextLevelInput struct {
	Identity                                       string
	Kind                                           ProgressiveContextLevelKind
	Projection                                     *memory.ContextProjection
	Evidence                                       []ProgressiveContextEvidence
	ExpectedWatermarkHash                          string
	MaxAge                                         time.Duration
	CharacterBudget, TokenBudget, ExpectedEvidence int
}
type ProgressiveContextLevelReport struct {
	Identity                           string                      `json:"identity"`
	Kind                               ProgressiveContextLevelKind `json:"kind"`
	Eligible                           bool                        `json:"eligible"`
	Freshness                          ProgressiveContextFreshness `json:"freshness"`
	SourceWatermarkHash                string                      `json:"source_watermark_hash,omitempty"`
	CharacterCount, TokenCount         int
	CharacterBudget, TokenBudget       int
	CitationCoverage, EvidenceCoverage float64
	RebuildIdentity                    string                            `json:"rebuild_identity"`
	FailureReasons                     []ProgressiveContextFailureReason `json:"failure_reasons,omitempty"`
	Efficiency                         *ContextEfficiencyMetrics         `json:"efficiency,omitempty"`
}
type ProgressiveContextEvaluationInput struct {
	Scope            memory.Scope
	EvaluatedAt      time.Time
	BaselineIdentity string
	Levels           []ProgressiveContextLevelInput
}
type ProgressiveContextEvaluationReport struct {
	BaselineIdentity string                          `json:"baseline_identity"`
	Levels           []ProgressiveContextLevelReport `json:"levels"`
}

func EvaluateProgressiveContext(input ProgressiveContextEvaluationInput) (ProgressiveContextEvaluationReport, error) {
	if err := input.Scope.Validate(); err != nil {
		return ProgressiveContextEvaluationReport{}, err
	}
	if strings.TrimSpace(input.BaselineIdentity) == "" {
		return ProgressiveContextEvaluationReport{}, fmt.Errorf("baseline identity is required")
	}
	if input.EvaluatedAt.IsZero() {
		input.EvaluatedAt = time.Now().UTC()
	}
	report := ProgressiveContextEvaluationReport{BaselineIdentity: input.BaselineIdentity, Levels: make([]ProgressiveContextLevelReport, 0, len(input.Levels))}
	for _, level := range input.Levels {
		r := evaluateProgressiveLevel(input.Scope, input.EvaluatedAt, level)
		report.Levels = append(report.Levels, r)
	}
	return report, nil
}
func evaluateProgressiveLevel(scope memory.Scope, now time.Time, in ProgressiveContextLevelInput) ProgressiveContextLevelReport {
	r := ProgressiveContextLevelReport{Identity: in.Identity, Kind: in.Kind, CharacterBudget: in.CharacterBudget, TokenBudget: in.TokenBudget, Freshness: ProgressiveContextFreshnessUnknown, Eligible: true}
	if strings.TrimSpace(in.Identity) == "" {
		r.Eligible = false
	}
	if in.Projection != nil {
		p := in.Projection
		r.SourceWatermarkHash = p.SourceWatermarkHash()
		r.RebuildIdentity = progressiveProjectionIdentity(*p)
		if p.Status != memory.ContextProjectionStatusActive {
			addProgressiveFailure(&r, ProgressiveContextFailureLifecycle)
		}
		if p.SourceWatermarkHash() == "" || len(p.SourceWatermark.CanonicalVersionIDs)+len(p.SourceWatermark.RawEventIDs) == 0 || (in.ExpectedWatermarkHash != "" && in.ExpectedWatermarkHash != p.SourceWatermarkHash()) {
			addProgressiveFailure(&r, ProgressiveContextFailureWatermark)
		}
		if in.MaxAge > 0 && now.Sub(p.UpdatedAt) > in.MaxAge {
			r.Freshness = ProgressiveContextFreshnessStale
			addProgressiveFailure(&r, ProgressiveContextFailureStale)
		} else {
			r.Freshness = ProgressiveContextFreshnessFresh
		}
		for _, item := range p.Items {
			if item.Source.Scope.Normalized() != scope.Normalized() {
				addProgressiveFailure(&r, ProgressiveContextFailureIsolation)
				continue
			}
			if item.LifecycleState != memory.MemoryStateActive || item.Source.LifecycleState != "" && item.Source.LifecycleState != memory.MemoryStateActive {
				addProgressiveFailure(&r, ProgressiveContextFailureLifecycle)
				continue
			}
			r.CharacterCount += len([]rune(item.Text))
			r.TokenCount += len(strings.Fields(item.Text))
			if item.Citation.MemoryID != "" || item.Citation.RawEventID != "" {
				r.CitationCoverage++
			}
			r.EvidenceCoverage++
		}
	} else {
		r.Freshness = ProgressiveContextFreshnessFresh
		r.RebuildIdentity = progressiveEvidenceIdentity(in.Evidence)
		for _, e := range in.Evidence {
			if e.Memory.Scope.Normalized() != scope.Normalized() {
				addProgressiveFailure(&r, ProgressiveContextFailureIsolation)
				continue
			}
			if e.Memory.State != memory.MemoryStateActive {
				addProgressiveFailure(&r, ProgressiveContextFailureLifecycle)
				continue
			}
			r.CharacterCount += len([]rune(e.Memory.Content))
			r.TokenCount += len(strings.Fields(e.Memory.Content))
			if len(e.Citations) > 0 {
				r.CitationCoverage++
			}
			r.EvidenceCoverage++
		}
	}
	if r.EvidenceCoverage > 0 {
		r.CitationCoverage /= r.EvidenceCoverage
	}
	if in.ExpectedEvidence > 0 {
		r.EvidenceCoverage /= float64(in.ExpectedEvidence)
	}
	if (in.CharacterBudget > 0 && r.CharacterCount > in.CharacterBudget) || (in.TokenBudget > 0 && r.TokenCount > in.TokenBudget) {
		addProgressiveFailure(&r, ProgressiveContextFailureBudget)
	}
	if in.ExpectedEvidence > 0 && r.CitationCoverage < 1 {
		addProgressiveFailure(&r, ProgressiveContextFailureCitation)
	}
	selected := r.TokenCount
	budget := in.TokenBudget
	if budget <= 0 {
		budget = selected
	}
	relevant := r.EvidenceCoverage
	if in.ExpectedEvidence > 0 {
		relevant = minFloat(relevant, 1)
	}
	r.Efficiency = &ContextEfficiencyMetrics{RelevantTokenRatio: relevant, EvidenceDensity: relevant / float64(maxInt(selected, 1)), SelectedContextTokens: selected, ContextBudgetTokens: budget, DuplicateTokenRate: 0, StaleTokenRate: 0, QualityPerBudget: relevant / float64(maxInt(budget, 1)), CandidateCount: int(r.EvidenceCoverage), ElapsedMS: 0}
	return r
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}
func addProgressiveFailure(r *ProgressiveContextLevelReport, f ProgressiveContextFailureReason) {
	for _, x := range r.FailureReasons {
		if x == f {
			return
		}
	}
	r.FailureReasons = append(r.FailureReasons, f)
	r.Eligible = false
}
func progressiveProjectionIdentity(p memory.ContextProjection) string {
	b, _ := json.Marshal(struct {
		Schema, Policy, Renderer, Watermark string
		Items                               []memory.ContextProjectionItem
	}{p.SchemaVersion, p.PolicyVersion, p.RendererVersion, p.SourceWatermarkHash(), p.Items})
	h := sha256.Sum256(b)
	return "projection-" + hex.EncodeToString(h[:])
}
func progressiveEvidenceIdentity(e []ProgressiveContextEvidence) string {
	cp := append([]ProgressiveContextEvidence(nil), e...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Alias < cp[j].Alias })
	b, _ := json.Marshal(cp)
	h := sha256.Sum256(b)
	return "evidence-" + hex.EncodeToString(h[:])
}
func containsProgressiveFailure(g []ProgressiveContextFailureReason, w ProgressiveContextFailureReason) bool {
	for _, x := range g {
		if x == w {
			return true
		}
	}
	return false
}
