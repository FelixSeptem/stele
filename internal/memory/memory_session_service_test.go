package memory

import (
	"context"
	"testing"
	"time"
)

func TestMemorySessionServiceRunsServiceSideSessionLoop(t *testing.T) {
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubMemorySessionStore{}
	contextAssembler := &stubMemorySessionContextAssembler{
		result: MemorySessionContextEvidence{
			Summary:     "profile and recent deployment preference",
			MemoryIDs:   []string{"mem_1"},
			Citations:   []string{"evt_1"},
			Diagnostics: []string{"included_profile"},
		},
	}
	nextID := 0
	service := NewMemorySessionService(MemorySessionServiceOptions{
		Store:            store,
		ContextAssembler: contextAssembler,
		Now:              func() time.Time { return now },
		NewID: func(prefix string) string {
			nextID++
			return prefix + "_1"
		},
	})

	session, err := service.CreateSession(context.Background(), CreateMemorySessionInput{
		Scope:    scope,
		Actor:    "agent-a",
		Reason:   "serve user turn",
		Metadata: map[string]any{"integration": "test-agent"},
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if session.ID != "session_1" || session.Status != MemorySessionStatusActive || session.Verdict != ScopeProofVerdictPending {
		t.Fatalf("session = %+v, want active pending session_1", session)
	}

	turn, err := service.CreateTurn(context.Background(), CreateMemorySessionTurnInput{
		Scope:                     scope,
		SessionID:                 session.ID,
		Query:                     "remember deployment preference",
		ContextBudget:             1200,
		IncludeRelations:          true,
		IncludeExperienceInsights: true,
	})
	if err != nil {
		t.Fatalf("CreateTurn() error = %v", err)
	}
	if turn.ID != "turn_1" || turn.Status != MemorySessionTurnStatusContextAssembled {
		t.Fatalf("turn = %+v, want assembled turn_1", turn)
	}
	if contextAssembler.gotRequest.Scope != scope || contextAssembler.gotRequest.Query != "remember deployment preference" {
		t.Fatalf("context request = %+v, want scoped query", contextAssembler.gotRequest)
	}
	if got := turn.ContextEvidence["summary"]; got != "profile and recent deployment preference" {
		t.Fatalf("context evidence summary = %v, want assembled summary", got)
	}

	updatedTurn, err := service.RecordTurnOutcome(context.Background(), RecordMemorySessionTurnOutcomeInput{
		Scope:           scope,
		SessionID:       session.ID,
		TurnID:          turn.ID,
		OutcomeEventIDs: []string{"evt_2"},
		ExpectedRecall:  []string{"evt_2"},
	})
	if err != nil {
		t.Fatalf("RecordTurnOutcome() error = %v", err)
	}
	if updatedTurn.Status != MemorySessionTurnStatusOutcomeRecorded || updatedTurn.OutcomeEventIDs[0] != "evt_2" {
		t.Fatalf("updatedTurn = %+v, want outcome recorded evt_2", updatedTurn)
	}

	verification, err := service.RequestVerification(context.Background(), RequestMemorySessionVerificationInput{
		Scope:          scope,
		SessionID:      session.ID,
		TurnID:         turn.ID,
		ExpectedRecall: []string{"evt_2"},
	})
	if err != nil {
		t.Fatalf("RequestVerification() error = %v", err)
	}
	if verification.ID != "verification_1" || verification.Status != ScopeProofStepStatusPending || verification.Verdict != ScopeProofVerdictPending {
		t.Fatalf("verification = %+v, want pending verification_1", verification)
	}

	report, err := service.ReadSessionReport(context.Background(), ReadMemorySessionRunInput{Scope: scope, SessionID: session.ID})
	if err != nil {
		t.Fatalf("ReadSessionReport() error = %v", err)
	}
	if report.Session.ID != session.ID || len(report.Session.Turns) != 1 || report.NextActions[0] == "" {
		t.Fatalf("report = %+v, want session turns and next actions", report)
	}
}

func TestMemorySessionReportExtractsFailureEvidenceAndNextActions(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubMemorySessionStore{
		sessions: []MemorySessionRun{{
			ID:              "session_1",
			Scope:           scope,
			Status:          MemorySessionStatusFailed,
			Verdict:         ScopeProofVerdictFailed,
			FailureCategory: ProofFailureCategoryContext,
		}},
		turns: []MemorySessionTurn{{
			ID:                 "turn_1",
			SessionID:          "session_1",
			Scope:              scope,
			Status:             MemorySessionTurnStatusFailed,
			VerificationStatus: ScopeProofVerdictFailed,
			FailureCategory:    ProofFailureCategoryRetrieval,
			ContextEvidence: map[string]any{
				"memory_ids": []string{"mem_1"},
				"citations":  []string{"evt_1"},
			},
			OutcomeEventIDs: []string{"evt_2"},
		}},
	}
	service := NewMemorySessionService(MemorySessionServiceOptions{Store: store})

	report, err := service.ReadSessionReport(context.Background(), ReadMemorySessionRunInput{Scope: scope, SessionID: "session_1"})
	if err != nil {
		t.Fatalf("ReadSessionReport() error = %v", err)
	}
	if report.Evidence.FailureCategories[0] != ProofFailureCategoryContext || report.Evidence.FailureCategories[1] != ProofFailureCategoryRetrieval {
		t.Fatalf("report evidence = %+v, want context and retrieval categories", report.Evidence)
	}
	if !containsString(report.NextActions, "inspect_context_diagnostics") || !containsString(report.NextActions, "inspect_retrieval_quality") || !containsString(report.NextActions, "open_quality_evaluation") {
		t.Fatalf("next actions = %+v, want context/retrieval quality diagnostics", report.NextActions)
	}
}

func TestMemorySessionServiceValidatesSessionLoopInputs(t *testing.T) {
	service := NewMemorySessionService(MemorySessionServiceOptions{Store: &stubMemorySessionStore{}})
	if _, err := service.CreateSession(context.Background(), CreateMemorySessionInput{}); err == nil {
		t.Fatal("CreateSession() error = nil, want scope validation error")
	}
	if _, err := service.CreateTurn(context.Background(), CreateMemorySessionTurnInput{}); err == nil {
		t.Fatal("CreateTurn() error = nil, want validation error")
	}
	if _, err := service.RecordTurnOutcome(context.Background(), RecordMemorySessionTurnOutcomeInput{}); err == nil {
		t.Fatal("RecordTurnOutcome() error = nil, want validation error")
	}
	if _, err := service.RequestVerification(context.Background(), RequestMemorySessionVerificationInput{}); err == nil {
		t.Fatal("RequestVerification() error = nil, want validation error")
	}
}

type stubMemorySessionContextAssembler struct {
	gotRequest MemorySessionContextRequest
	result     MemorySessionContextEvidence
	err        error
}

func (s *stubMemorySessionContextAssembler) AssembleSessionContext(ctx context.Context, request MemorySessionContextRequest) (MemorySessionContextEvidence, error) {
	s.gotRequest = request
	return s.result, s.err
}

type stubMemorySessionStore struct {
	sessions      []MemorySessionRun
	turns         []MemorySessionTurn
	verifications []MemorySessionVerification
}

func (s *stubMemorySessionStore) CreateMemorySessionRun(ctx context.Context, session MemorySessionRun) (MemorySessionRun, error) {
	s.sessions = append(s.sessions, session)
	return session, nil
}

func (s *stubMemorySessionStore) ListMemorySessionRuns(ctx context.Context, input ListMemorySessionRunsInput) ([]MemorySessionRun, error) {
	return append([]MemorySessionRun(nil), s.sessions...), nil
}

func (s *stubMemorySessionStore) ReadMemorySessionRun(ctx context.Context, input ReadMemorySessionRunInput) (MemorySessionRun, error) {
	for _, session := range s.sessions {
		if session.ID == input.SessionID && session.Scope == input.Scope {
			session.Turns = append([]MemorySessionTurn(nil), s.turns...)
			return session, nil
		}
	}
	return MemorySessionRun{}, nil
}

func (s *stubMemorySessionStore) CreateMemorySessionTurn(ctx context.Context, turn MemorySessionTurn) (MemorySessionTurn, error) {
	s.turns = append(s.turns, turn)
	return turn, nil
}

func (s *stubMemorySessionStore) UpdateMemorySessionTurnOutcome(ctx context.Context, input UpdateMemorySessionTurnOutcomeInput) (MemorySessionTurn, error) {
	for index, turn := range s.turns {
		if turn.ID == input.TurnID && turn.SessionID == input.SessionID && turn.Scope == input.Scope {
			turn.Status = input.Status
			turn.OutcomeEventIDs = append([]string(nil), input.OutcomeEventIDs...)
			turn.ExpectedRecall = append([]string(nil), input.ExpectedRecall...)
			turn.VerificationStatus = input.VerificationStatus
			turn.FailureCategory = input.FailureCategory
			turn.UpdatedAt = input.UpdatedAt
			s.turns[index] = turn
			return turn, nil
		}
	}
	return MemorySessionTurn{}, nil
}

func (s *stubMemorySessionStore) CreateMemorySessionVerification(ctx context.Context, verification MemorySessionVerification) (MemorySessionVerification, error) {
	s.verifications = append(s.verifications, verification)
	return verification, nil
}
