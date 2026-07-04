package memory

import (
	"strings"
	"testing"
	"time"
)

func TestDerivedInsightValidateAcceptsActiveFailurePatternWithRepeatedEvidence(t *testing.T) {
	insight := DerivedInsight{
		ID:      "insight_123",
		Scope:   Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Type:    DerivedInsightTypeFailurePattern,
		State:   DerivedInsightStateActive,
		Title:   "Embedding rebuild repeatedly fails",
		Summary: "The OpenAI embedding rebuild path failed twice with provider unavailable.",
		Confidence: DerivedInsightConfidence{
			Score: 0.75,
		},
		Derivation: DerivedInsightDerivation{
			Source:              "failure_pattern_evaluator",
			Fingerprint:         "failure_pattern:tenant-a:project-a:namespace-a:embedding_provider_unavailable",
			EvidenceWindowStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			EvidenceWindowEnd:   time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
			DerivedAt:           time.Date(2026, 7, 2, 1, 0, 0, 0, time.UTC),
		},
		Evidence: []DerivedInsightEvidenceRef{
			{Kind: DerivedInsightEvidenceKindJobExecution, ID: "job_1", Relation: DerivedInsightEvidenceRelationSupports, ObservedAt: time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)},
			{Kind: DerivedInsightEvidenceKindEmbeddingRebuild, ID: "mem_1", Relation: DerivedInsightEvidenceRelationSupports, ObservedAt: time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC)},
		},
	}

	if err := insight.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDerivedInsightValidateRejectsInvalidScopeAndConfidence(t *testing.T) {
	insight := validFailurePatternInsight()
	insight.Scope.Tenant = ""
	insight.Confidence.Score = 1.25

	err := insight.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "tenant is required") {
		t.Fatalf("Validate() error = %v, want tenant validation", err)
	}
}

func TestDerivedInsightValidateRejectsActiveFailurePatternWithoutRepeatedEvidence(t *testing.T) {
	insight := validFailurePatternInsight()
	insight.Evidence = insight.Evidence[:1]

	err := insight.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want repeated evidence validation")
	}
	if !strings.Contains(err.Error(), "active failure_pattern requires at least 2 evidence references") {
		t.Fatalf("Validate() error = %v, want repeated evidence validation", err)
	}
}

func TestDerivedInsightValidateRejectsUngroundedLesson(t *testing.T) {
	insight := DerivedInsight{
		ID:      "lesson_123",
		Scope:   Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Type:    DerivedInsightTypeLesson,
		State:   DerivedInsightStateActive,
		Title:   "Retry only after provider recovers",
		Summary: "Avoid repeated embedding rebuild attempts while provider health is failing.",
		Confidence: DerivedInsightConfidence{
			Score: 0.7,
		},
		Derivation: DerivedInsightDerivation{
			Source:      "lesson_projection",
			Fingerprint: "lesson:tenant-a:project-a:namespace-a:embedding_provider_unavailable",
			DerivedAt:   time.Date(2026, 7, 2, 1, 30, 0, 0, time.UTC),
		},
		Evidence: []DerivedInsightEvidenceRef{
			{Kind: DerivedInsightEvidenceKindJobExecution, ID: "job_1", Relation: DerivedInsightEvidenceRelationSupports, ObservedAt: time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)},
		},
		Lesson: &DerivedInsightLesson{
			Guidance: "Avoid repeated embedding rebuild attempts while provider health is failing.",
		},
	}

	err := insight.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want ungrounded lesson validation")
	}
	if !strings.Contains(err.Error(), "source failure pattern id is required") {
		t.Fatalf("Validate() error = %v, want source failure pattern validation", err)
	}
}

func TestDerivedInsightValidateRejectsReservedTypesAsActiveGuidance(t *testing.T) {
	for _, insightType := range []DerivedInsightType{
		DerivedInsightTypeHypothesis,
		DerivedInsightTypeGoal,
		DerivedInsightTypeContradiction,
		DerivedInsightTypeCausalLink,
	} {
		t.Run(string(insightType), func(t *testing.T) {
			insight := validFailurePatternInsight()
			insight.Type = insightType

			err := insight.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want reserved type validation")
			}
			if !strings.Contains(err.Error(), "is reserved and cannot be active in this change") {
				t.Fatalf("Validate() error = %v, want reserved type validation", err)
			}
		})
	}
}

func TestListDerivedInsightsInputValidateRejectsUnsupportedFilters(t *testing.T) {
	input := ListDerivedInsightsInput{
		Scope: Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Type:  "unknown",
		Limit: 50,
	}

	if err := input.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid type filter")
	}

	input.Type = DerivedInsightTypeFailurePattern
	input.State = "unknown"
	if err := input.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid state filter")
	}
}

func validFailurePatternInsight() DerivedInsight {
	return DerivedInsight{
		ID:      "insight_123",
		Scope:   Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Type:    DerivedInsightTypeFailurePattern,
		State:   DerivedInsightStateActive,
		Title:   "Embedding rebuild repeatedly fails",
		Summary: "The embedding rebuild path failed repeatedly with provider unavailable.",
		Confidence: DerivedInsightConfidence{
			Score: 0.75,
		},
		Derivation: DerivedInsightDerivation{
			Source:              "failure_pattern_evaluator",
			Fingerprint:         "failure_pattern:tenant-a:project-a:namespace-a:embedding_provider_unavailable",
			EvidenceWindowStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			EvidenceWindowEnd:   time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
			DerivedAt:           time.Date(2026, 7, 2, 1, 0, 0, 0, time.UTC),
		},
		Evidence: []DerivedInsightEvidenceRef{
			{Kind: DerivedInsightEvidenceKindJobExecution, ID: "job_1", Relation: DerivedInsightEvidenceRelationSupports, ObservedAt: time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)},
			{Kind: DerivedInsightEvidenceKindEmbeddingRebuild, ID: "mem_1", Relation: DerivedInsightEvidenceRelationSupports, ObservedAt: time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC)},
		},
	}
}
