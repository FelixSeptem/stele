package memory

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTaskEvaluationServiceCreateTaskEvaluationNormalizesScopeAndAssignsTimestamps(t *testing.T) {
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	store := &stubTaskEvaluationStore{}
	service := NewTaskEvaluationService(TaskEvaluationServiceOptions{
		Store: store,
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_generated" },
	})

	created, err := service.CreateTaskEvaluation(context.Background(), validTaskEvaluation())
	if err != nil {
		t.Fatalf("CreateTaskEvaluation() error = %v", err)
	}

	if created.ID != "task_eval_1" {
		t.Fatalf("created.ID = %q, want task_eval_1", created.ID)
	}
	if store.created.ID != "task_eval_1" || store.created.Scope != (Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}) {
		t.Fatalf("store.created = %+v, want normalized scope", store.created)
	}
	if !store.created.CreatedAt.Equal(now) || !store.created.UpdatedAt.Equal(now) {
		t.Fatalf("timestamps = %+v, want now", store.created)
	}
}

func TestTaskEvaluationServiceSupersedeDelegatesToStore(t *testing.T) {
	store := &stubTaskEvaluationStore{}
	service := NewTaskEvaluationService(TaskEvaluationServiceOptions{Store: store})
	now := time.Date(2026, 7, 12, 10, 5, 0, 0, time.UTC)
	err := service.SupersedeTaskEvaluation(context.Background(), SupersedeTaskEvaluationInput{
		Scope:        Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		EvaluationID: "task_eval_1",
		Actor:        "operator-a",
		Reason:       "superseded by correction",
		SupersededAt: now,
	})
	if err != nil {
		t.Fatalf("SupersedeTaskEvaluation() error = %v", err)
	}
	if store.supersedeInput.EvaluationID != "task_eval_1" || store.supersedeInput.SupersededAt != now {
		t.Fatalf("supersedeInput = %+v, want delegated input", store.supersedeInput)
	}
}

func TestProceduralVerificationTaskEvidenceRequiresSameScopeVerification(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	verification := ProceduralMemoryVerification{ID: "procedural-verification-1", Scope: scope}
	store := &stubTaskEvaluationStore{}
	reader := &stubProceduralVerificationEvidenceReader{record: verification}
	service := NewTaskEvaluationService(TaskEvaluationServiceOptions{
		Store:                          store,
		ProceduralVerificationEvidence: reader,
	})

	evaluation := validTaskEvaluation()
	evaluation.Evidence = []TaskEvidenceLink{{Kind: TaskEvidenceTargetProceduralVerification, ID: verification.ID}}
	if _, err := service.CreateTaskEvaluation(context.Background(), evaluation); err != nil {
		t.Fatalf("CreateTaskEvaluation() error = %v", err)
	}
	if reader.input.Scope != scope || reader.input.VerificationID != verification.ID {
		t.Fatalf("verification read = %+v, want exact scope and id", reader.input)
	}
	createdCount := store.createCount

	foreign := validTaskEvaluation()
	foreign.Evidence = []TaskEvidenceLink{{Kind: TaskEvidenceTargetProceduralVerification, ID: "foreign-verification"}}
	reader.err = ErrNotFound
	if _, err := service.CreateTaskEvaluation(context.Background(), foreign); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign verification error = %v, want not found", err)
	}
	if store.createCount != createdCount {
		t.Fatal("foreign verification must not be persisted as task evidence")
	}
}

type stubTaskEvaluationStore struct {
	created        TaskEvaluation
	createCount    int
	supersedeInput SupersedeTaskEvaluationInput
}

type proceduralVerificationEvidenceReadInput struct {
	Scope          Scope
	VerificationID string
}

type stubProceduralVerificationEvidenceReader struct {
	record ProceduralMemoryVerification
	err    error
	input  proceduralVerificationEvidenceReadInput
}

func (s *stubProceduralVerificationEvidenceReader) ReadProceduralMemoryVerification(_ context.Context, scope Scope, verificationID string) (ProceduralMemoryVerification, error) {
	s.input = proceduralVerificationEvidenceReadInput{Scope: scope, VerificationID: verificationID}
	if s.err != nil {
		return ProceduralMemoryVerification{}, s.err
	}
	return s.record, nil
}

func (s *stubTaskEvaluationStore) CreateTaskEvaluation(ctx context.Context, evaluation TaskEvaluation) (TaskEvaluation, error) {
	s.created = evaluation
	s.createCount++
	return evaluation, nil
}

func (s *stubTaskEvaluationStore) ReadTaskEvaluation(ctx context.Context, input ReadTaskEvaluationInput) (TaskEvaluation, error) {
	return TaskEvaluation{}, nil
}

func (s *stubTaskEvaluationStore) ListTaskEvaluations(ctx context.Context, input ListTaskEvaluationsInput) ([]TaskEvaluation, error) {
	return nil, nil
}

func (s *stubTaskEvaluationStore) SupersedeTaskEvaluation(ctx context.Context, input SupersedeTaskEvaluationInput) error {
	s.supersedeInput = input
	return nil
}

func (s *stubTaskEvaluationStore) SummarizeTaskEvaluations(ctx context.Context, input SummarizeTaskEvaluationsInput) (TaskEvaluationSummary, error) {
	return TaskEvaluationSummary{}, nil
}
