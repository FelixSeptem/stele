package memory

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type contextCalibrationFeedbackSourceStub struct{ items []UsefulnessFeedback }

func (s contextCalibrationFeedbackSourceStub) ListUsefulnessFeedback(_ context.Context, _ ListUsefulnessFeedbackInput) ([]UsefulnessFeedback, error) {
	return append([]UsefulnessFeedback(nil), s.items...), nil
}

type contextCalibrationTaskSourceStub struct{ items []TaskEvaluation }

func (s contextCalibrationTaskSourceStub) ListTaskEvaluations(_ context.Context, _ ListTaskEvaluationsInput) ([]TaskEvaluation, error) {
	return append([]TaskEvaluation(nil), s.items...), nil
}

type contextCalibrationSummaryWriterStub struct{ saved ContextCalibrationSummary }

func (s *contextCalibrationSummaryWriterStub) UpsertContextCalibrationSummary(_ context.Context, summary ContextCalibrationSummary) (ContextCalibrationSummary, error) {
	s.saved = summary
	return summary, nil
}

func TestContextCalibrationSummaryRebuildServiceUsesOnlyActiveExactScopeEvidence(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	writer := &contextCalibrationSummaryWriterStub{}
	service := ContextCalibrationSummaryRebuildService{
		Feedback: contextCalibrationFeedbackSourceStub{items: []UsefulnessFeedback{
			{Scope: scope, Type: UsefulnessFeedbackTypeUseful, Subjects: []UsefulnessFeedbackSubject{{Kind: UsefulnessFeedbackSubjectMemory, ID: "memory-a"}}, CreatedAt: now.Add(-time.Hour), Reason: "must not be persisted"},
			{Scope: scope, Type: UsefulnessFeedbackTypeStale, Subjects: []UsefulnessFeedbackSubject{{Kind: UsefulnessFeedbackSubjectMemory, ID: "memory-b"}}, CreatedAt: now.Add(-time.Hour), SupersededAt: now},
			{Scope: Scope{Tenant: "tenant-b", Project: "project-a", Namespace: "namespace-a"}, Type: UsefulnessFeedbackTypeUseful, Subjects: []UsefulnessFeedbackSubject{{Kind: UsefulnessFeedbackSubjectMemory, ID: "foreign"}}, CreatedAt: now},
		}},
		Tasks: contextCalibrationTaskSourceStub{items: []TaskEvaluation{
			{Scope: scope, Verdict: TaskEvaluationVerdictSucceeded, Evidence: []TaskEvidenceLink{{Kind: TaskEvidenceTargetMemory, ID: "memory-c"}}, CreatedAt: now.Add(-30 * time.Minute)},
			{Scope: scope, Verdict: TaskEvaluationVerdictFailed, Evidence: []TaskEvidenceLink{{Kind: TaskEvidenceTargetMemory, ID: "memory-d"}}, CreatedAt: now, CorrectionState: TaskEvaluationCorrectionStateSuperseded},
		}},
		Store: writer, Limit: 10,
	}
	summary, err := service.BuildContextCalibrationSummary(context.Background(), ContextCalibrationBuildInput{Scope: scope, PolicyVersion: ContextCalibrationPolicyVersionV1, SummaryVersion: ContextCalibrationSummaryVersionV1, Now: now, MinimumEvidence: 2, ConfidenceThreshold: .5, DecayWindow: 24 * time.Hour, ContributionCap: .25})
	if err != nil {
		t.Fatal(err)
	}
	if summary.EvidenceCount != 2 || summary.Freshness != ContextCalibrationSummaryFresh || writer.saved.ID != summary.ID {
		t.Fatalf("summary=%+v saved=%+v, want two exact-scope active signals", summary, writer.saved)
	}
	payload, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"memory-a", "memory-c", "must not be persisted"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("summary payload leaked source value %q: %s", forbidden, payload)
		}
	}
}
