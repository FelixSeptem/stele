package memory

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestUsefulnessFeedbackValidateRequiresBoundedTaxonomyAndSubject(t *testing.T) {
	now := time.Date(2026, 7, 11, 9, 30, 0, 0, time.UTC)
	feedback := UsefulnessFeedback{
		ID:            "feedback_1",
		Scope:         Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Type:          UsefulnessFeedbackTypeNoisy,
		SourceSurface: UsefulnessFeedbackSourceSession,
		Subjects: []UsefulnessFeedbackSubject{{
			Kind: UsefulnessFeedbackSubjectMemory,
			ID:   "mem_1",
		}},
		Actor:          "agent-a",
		Reason:         "too broad",
		IdempotencyKey: "session_1:turn_1:mem_1:noisy",
		CreatedAt:      now,
	}

	if err := feedback.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	feedback.Type = UsefulnessFeedbackType("free_form")
	err := feedback.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid feedback type")
	}
	if !strings.Contains(err.Error(), "usefulness feedback type") {
		t.Fatalf("Validate() error = %v, want feedback type validation", err)
	}
}

func TestExpectedRecallTargetValidationDistinguishesKnownEvidenceFromOpaqueTokens(t *testing.T) {
	known := ExpectedRecallTarget{
		Kind:        ExpectedRecallTargetMemory,
		ID:          "mem_1",
		OpaqueToken: "not allowed with known id",
	}
	if err := known.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want known target to reject opaque token")
	}

	opaque := ExpectedRecallTarget{
		Kind:        ExpectedRecallTargetOpaque,
		OpaqueToken: "caller-expected-fact",
	}
	if err := opaque.Validate(); err != nil {
		t.Fatalf("Validate() opaque error = %v", err)
	}
}

func TestUsefulnessFeedbackValidateBoundsCallerProvidedFields(t *testing.T) {
	feedback := validUsefulnessFeedback()
	feedback.IdempotencyKey = strings.Repeat("k", 257)
	if err := feedback.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want idempotency key bound error")
	}

	feedback = validUsefulnessFeedback()
	feedback.Actor = strings.Repeat("a", 257)
	if err := feedback.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want actor bound error")
	}

	feedback = validUsefulnessFeedback()
	feedback.Reason = strings.Repeat("r", 2049)
	if err := feedback.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want reason bound error")
	}

	feedback = validUsefulnessFeedback()
	feedback.Metadata = make(map[string]any, 33)
	for i := 0; i < 33; i++ {
		feedback.Metadata[fmt.Sprintf("key_%d", i)] = "value"
	}
	if err := feedback.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want metadata count bound error")
	}

	feedback = validUsefulnessFeedback()
	feedback.Metadata = map[string]any{strings.Repeat("k", 129): "value"}
	if err := feedback.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want metadata key bound error")
	}

	feedback = validUsefulnessFeedback()
	feedback.Metadata = map[string]any{"key": strings.Repeat("v", 1025)}
	if err := feedback.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want metadata value bound error")
	}
}

func TestSummarizeUsefulnessFeedbackExcludesSupersededRecords(t *testing.T) {
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	subject := UsefulnessFeedbackSubject{Kind: UsefulnessFeedbackSubjectMemory, ID: "mem_1"}

	summary := SummarizeUsefulnessFeedback(subject, []UsefulnessFeedback{
		{
			ID:        "feedback_useful",
			Type:      UsefulnessFeedbackTypeUseful,
			Subjects:  []UsefulnessFeedbackSubject{subject},
			CreatedAt: now,
		},
		{
			ID:           "feedback_noisy",
			Type:         UsefulnessFeedbackTypeNoisy,
			Subjects:     []UsefulnessFeedbackSubject{subject},
			CreatedAt:    now.Add(time.Minute),
			SupersededAt: now.Add(2 * time.Minute),
		},
	})

	if summary.TotalActive != 1 || summary.Counts[UsefulnessFeedbackTypeUseful] != 1 || summary.Counts[UsefulnessFeedbackTypeNoisy] != 0 {
		t.Fatalf("summary = %+v, want only active useful feedback counted", summary)
	}
	if summary.EffectiveQuality != UsefulnessQualityPositive {
		t.Fatalf("EffectiveQuality = %q, want positive", summary.EffectiveQuality)
	}
}

func validUsefulnessFeedback() UsefulnessFeedback {
	return UsefulnessFeedback{
		ID:            "feedback_1",
		Scope:         Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Type:          UsefulnessFeedbackTypeUseful,
		SourceSurface: UsefulnessFeedbackSourceSession,
		Subjects: []UsefulnessFeedbackSubject{{
			Kind: UsefulnessFeedbackSubjectMemory,
			ID:   "mem_1",
		}},
		Actor:     "agent-a",
		Reason:    "helped answer",
		CreatedAt: time.Date(2026, 7, 11, 9, 30, 0, 0, time.UTC),
	}
}
