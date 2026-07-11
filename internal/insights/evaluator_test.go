package insights

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestFailurePatternEvaluatorActivatesRepeatedEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	observedAt := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)
	evaluator := FailurePatternEvaluator{
		MinimumEvidence: 2,
		Window:          24 * time.Hour,
		Now:             func() time.Time { return observedAt.Add(time.Hour) },
	}

	insights, err := evaluator.Evaluate(scope, []FailureEvidence{
		{
			Kind:       memory.DerivedInsightEvidenceKindJobExecution,
			ID:         "job_1",
			FailureKey: "Embedding provider unavailable",
			Message:    "provider unavailable",
			ObservedAt: observedAt,
		},
		{
			Kind:       memory.DerivedInsightEvidenceKindJobExecution,
			ID:         "job_2",
			FailureKey: "embedding_provider_unavailable",
			Message:    "provider unavailable",
			ObservedAt: observedAt.Add(time.Minute),
		},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if len(insights) != 1 {
		t.Fatalf("len(insights) = %d, want 1", len(insights))
	}
	got := insights[0]
	if got.Type != memory.DerivedInsightTypeFailurePattern || got.State != memory.DerivedInsightStateActive {
		t.Fatalf("insight type/state = %s/%s, want active failure_pattern", got.Type, got.State)
	}
	if got.Confidence.Score <= 0 {
		t.Fatalf("confidence = %v, want positive", got.Confidence.Score)
	}
	if len(got.Evidence) != 2 {
		t.Fatalf("len(evidence) = %d, want 2", len(got.Evidence))
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("derived insight Validate() error = %v", err)
	}
}

func TestFailurePatternEvaluatorRejectsIsolatedEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	evaluator := FailurePatternEvaluator{MinimumEvidence: 2}

	insights, err := evaluator.Evaluate(scope, []FailureEvidence{
		{
			Kind:       memory.DerivedInsightEvidenceKindEmbeddingRebuild,
			ID:         "mem_1",
			FailureKey: "provider unavailable",
			ObservedAt: time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if len(insights) != 0 {
		t.Fatalf("len(insights) = %d, want 0", len(insights))
	}
}

func TestFailurePatternEvaluatorIsIdempotentForSameEvidenceWindow(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	evaluator := FailurePatternEvaluator{MinimumEvidence: 2, Window: 24 * time.Hour}
	evidence := []FailureEvidence{
		{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)},
		{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 4, 11, 0, 0, 0, time.UTC)},
	}

	first, err := evaluator.Evaluate(scope, evidence)
	if err != nil {
		t.Fatalf("Evaluate(first) error = %v", err)
	}
	second, err := evaluator.Evaluate(scope, append([]FailureEvidence(nil), evidence...))
	if err != nil {
		t.Fatalf("Evaluate(second) error = %v", err)
	}

	if first[0].ID != second[0].ID || first[0].Derivation.Fingerprint != second[0].Derivation.Fingerprint {
		t.Fatalf("second derivation produced different identity: first=%s/%s second=%s/%s", first[0].ID, first[0].Derivation.Fingerprint, second[0].ID, second[0].Derivation.Fingerprint)
	}
}

func TestFailurePatternEvaluatorUpdatesEvidenceAndConfidence(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	evaluator := FailurePatternEvaluator{MinimumEvidence: 2, Window: 24 * time.Hour}
	base := []FailureEvidence{
		{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)},
		{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", FailureKey: "provider unavailable", ObservedAt: time.Date(2026, 7, 4, 11, 0, 0, 0, time.UTC)},
	}

	first, err := evaluator.Evaluate(scope, base)
	if err != nil {
		t.Fatalf("Evaluate(base) error = %v", err)
	}
	updated, err := evaluator.Evaluate(scope, append(base, FailureEvidence{
		Kind:       memory.DerivedInsightEvidenceKindJobExecution,
		ID:         "job_3",
		FailureKey: "provider unavailable",
		ObservedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
	}))
	if err != nil {
		t.Fatalf("Evaluate(updated) error = %v", err)
	}

	if updated[0].ID != first[0].ID {
		t.Fatalf("updated ID = %q, want same id %q", updated[0].ID, first[0].ID)
	}
	if len(updated[0].Evidence) != 3 {
		t.Fatalf("updated evidence count = %d, want 3", len(updated[0].Evidence))
	}
	if updated[0].Confidence.Score <= first[0].Confidence.Score {
		t.Fatalf("updated confidence = %v, want greater than %v", updated[0].Confidence.Score, first[0].Confidence.Score)
	}
}

func TestProjectLessonRequiresFailurePatternSource(t *testing.T) {
	pattern := memory.DerivedInsight{
		ID:      "insight_123",
		Scope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Type:    memory.DerivedInsightTypeFailurePattern,
		State:   memory.DerivedInsightStateActive,
		Title:   "Provider unavailable repeatedly",
		Summary: "Provider unavailable caused repeated failures.",
		Confidence: memory.DerivedInsightConfidence{
			Score: 0.75,
		},
		Derivation: memory.DerivedInsightDerivation{
			Source:      "failure_pattern_evaluator",
			Fingerprint: "failure_pattern:fingerprint",
			DerivedAt:   time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		},
		Evidence: []memory.DerivedInsightEvidenceRef{
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", Relation: memory.DerivedInsightEvidenceRelationSupports},
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", Relation: memory.DerivedInsightEvidenceRelationSupports},
		},
	}

	lesson, err := ProjectLesson(pattern)
	if err != nil {
		t.Fatalf("ProjectLesson() error = %v", err)
	}
	if lesson.Type != memory.DerivedInsightTypeLesson {
		t.Fatalf("lesson.Type = %s, want lesson", lesson.Type)
	}
	if lesson.Lesson == nil || lesson.Lesson.SourceFailurePatternID != pattern.ID {
		t.Fatalf("lesson source = %+v, want source pattern %q", lesson.Lesson, pattern.ID)
	}
	if len(lesson.Evidence) != len(pattern.Evidence) {
		t.Fatalf("lesson evidence count = %d, want %d", len(lesson.Evidence), len(pattern.Evidence))
	}
	if err := lesson.Validate(); err != nil {
		t.Fatalf("lesson Validate() error = %v", err)
	}

	pattern.ID = ""
	if _, err := ProjectLesson(pattern); err == nil {
		t.Fatal("ProjectLesson() error = nil, want source validation")
	}
}

func TestApplyFeedbackPolicySuppressesStrongNegativeInsight(t *testing.T) {
	pattern := activeFailurePatternForTest()
	summary := memory.DerivedInsightFeedbackSummary{
		InsightID:     pattern.ID,
		Counts:        map[memory.InsightFeedbackType]int{memory.InsightFeedbackTypeNoisy: 2},
		TotalActive:   2,
		NegativeCount: 2,
	}

	got, decision := ApplyFeedbackPolicy(pattern, summary)
	if got.State != memory.DerivedInsightStateSuppressed {
		t.Fatalf("state = %s, want suppressed", got.State)
	}
	if decision != FeedbackPolicyDecisionSuppress {
		t.Fatalf("decision = %s, want suppress", decision)
	}
	if got.Derivation.Metadata["feedback_policy_decision"] != string(FeedbackPolicyDecisionSuppress) {
		t.Fatalf("metadata = %+v, want feedback policy decision", got.Derivation.Metadata)
	}
}

func TestApplyFeedbackPolicyMarksNeedsReviewAsCandidate(t *testing.T) {
	pattern := activeFailurePatternForTest()
	summary := memory.DerivedInsightFeedbackSummary{
		InsightID:   pattern.ID,
		Counts:      map[memory.InsightFeedbackType]int{memory.InsightFeedbackTypeNeedsReview: 1},
		TotalActive: 1,
		NeedsReview: true,
	}

	got, decision := ApplyFeedbackPolicy(pattern, summary)
	if got.State != memory.DerivedInsightStateCandidate {
		t.Fatalf("state = %s, want candidate", got.State)
	}
	if decision != FeedbackPolicyDecisionNeedsReview {
		t.Fatalf("decision = %s, want needs_review", decision)
	}
}

func TestApplyFeedbackPolicyPreservesUsefulInsight(t *testing.T) {
	pattern := activeFailurePatternForTest()
	summary := memory.DerivedInsightFeedbackSummary{
		InsightID:     pattern.ID,
		Counts:        map[memory.InsightFeedbackType]int{memory.InsightFeedbackTypeUseful: 1},
		TotalActive:   1,
		PositiveCount: 1,
	}

	got, decision := ApplyFeedbackPolicy(pattern, summary)
	if got.State != memory.DerivedInsightStateActive {
		t.Fatalf("state = %s, want active", got.State)
	}
	if decision != FeedbackPolicyDecisionPreserve {
		t.Fatalf("decision = %s, want preserve", decision)
	}
}

func TestFailurePatternFingerprintIncludesScopeKindKeyAndWindow(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	windowStart := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	windowEnd := windowStart.Add(24 * time.Hour)

	first := FailurePatternFingerprint(scope, memory.DerivedInsightEvidenceKindJobExecution, "provider unavailable", windowStart, windowEnd)
	second := FailurePatternFingerprint(scope, memory.DerivedInsightEvidenceKindEmbeddingRebuild, "provider unavailable", windowStart, windowEnd)
	third := FailurePatternFingerprint(memory.Scope{Tenant: "tenant-b", Project: "project-a", Namespace: "namespace-a"}, memory.DerivedInsightEvidenceKindJobExecution, "provider unavailable", windowStart, windowEnd)

	if first == second {
		t.Fatal("fingerprint did not include evidence kind")
	}
	if first == third {
		t.Fatal("fingerprint did not include scope")
	}
	if first != FailurePatternFingerprint(scope, memory.DerivedInsightEvidenceKindJobExecution, "provider_unavailable", windowStart, windowEnd) {
		t.Fatal("fingerprint did not normalize equivalent failure keys")
	}
}

func activeFailurePatternForTest() memory.DerivedInsight {
	return memory.DerivedInsight{
		ID:      "insight_123",
		Scope:   memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Type:    memory.DerivedInsightTypeFailurePattern,
		State:   memory.DerivedInsightStateActive,
		Title:   "Provider unavailable repeatedly",
		Summary: "Provider unavailable caused repeated failures.",
		Confidence: memory.DerivedInsightConfidence{
			Score: 0.75,
		},
		Derivation: memory.DerivedInsightDerivation{
			Source:      "failure_pattern_evaluator",
			Fingerprint: "failure_pattern:fingerprint",
			DerivedAt:   time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
			Metadata:    map[string]any{},
		},
		Evidence: []memory.DerivedInsightEvidenceRef{
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_1", Relation: memory.DerivedInsightEvidenceRelationSupports},
			{Kind: memory.DerivedInsightEvidenceKindJobExecution, ID: "job_2", Relation: memory.DerivedInsightEvidenceRelationSupports},
		},
	}
}
