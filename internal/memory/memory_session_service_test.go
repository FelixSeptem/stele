package memory

import (
	"context"
	"strings"
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
				"memory_ids":             []string{"mem_1"},
				"citations":              []string{"evt_1"},
				"quality_finding_ids":    []string{"finding_feedback_1"},
				"quality_finding_codes":  []string{string(QualityFindingFeedbackNoisyRepeated)},
				"hidden_memory_evidence": "mem_hidden",
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
	if report.Evidence.QualityFindingIDs[0] != "finding_feedback_1" || report.Evidence.QualityFindingCodes[0] != QualityFindingFeedbackNoisyRepeated {
		t.Fatalf("report evidence = %+v, want bounded quality finding links", report.Evidence)
	}
}

func TestMemorySessionReportIncludesAuthorizedFeedbackSummaries(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubMemorySessionStore{
		sessions: []MemorySessionRun{{
			ID:      "session_1",
			Scope:   scope,
			Status:  MemorySessionStatusCompleted,
			Verdict: ScopeProofVerdictPassed,
		}},
		turns: []MemorySessionTurn{{
			ID:                 "turn_1",
			SessionID:          "session_1",
			Scope:              scope,
			Status:             MemorySessionTurnStatusVerified,
			VerificationStatus: ScopeProofVerdictPassed,
			ExpectedRecall:     []string{"evt_1"},
		}},
		verifications: []MemorySessionVerification{{
			ID:        "verification_1",
			SessionID: "session_1",
			TurnID:    "turn_1",
			Scope:     scope,
			Status:    ScopeProofStepStatusCompleted,
			Verdict:   ScopeProofVerdictPassed,
		}},
	}
	summaries := &stubUsefulnessSummaryReader{
		summaries: map[string]UsefulnessFeedbackSummary{
			"session:session_1": {
				Subject:          UsefulnessFeedbackSubject{Kind: UsefulnessFeedbackSubjectSession, ID: "session_1"},
				TotalActive:      1,
				PositiveCount:    1,
				EffectiveQuality: UsefulnessQualityPositive,
			},
			"turn:turn_1": {
				Subject:          UsefulnessFeedbackSubject{Kind: UsefulnessFeedbackSubjectTurn, ID: "turn_1"},
				TotalActive:      1,
				NeedsReviewCount: 1,
				EffectiveQuality: UsefulnessQualityNeedsReview,
			},
			"verification:verification_1": {
				Subject:          UsefulnessFeedbackSubject{Kind: UsefulnessFeedbackSubjectVerification, ID: "verification_1"},
				TotalActive:      1,
				NegativeCount:    1,
				EffectiveQuality: UsefulnessQualityNegative,
			},
			"expected_recall:event:evt_1": {
				Subject: UsefulnessFeedbackSubject{Kind: UsefulnessFeedbackSubjectExpectedRecall, ExpectedRecallTarget: ExpectedRecallTarget{
					Kind: ExpectedRecallTargetEvent,
					ID:   "evt_1",
				}},
				TotalActive:      1,
				NegativeCount:    1,
				EffectiveQuality: UsefulnessQualityNegative,
			},
		},
	}
	service := NewMemorySessionService(MemorySessionServiceOptions{Store: store, UsefulnessSummarizer: summaries})

	report, err := service.ReadSessionReport(context.Background(), ReadMemorySessionRunInput{Scope: scope, SessionID: "session_1"})
	if err != nil {
		t.Fatalf("ReadSessionReport() error = %v", err)
	}
	if len(report.FeedbackSummaries) != 4 {
		t.Fatalf("feedback summaries = %+v, want session, turn, verification, expected recall", report.FeedbackSummaries)
	}
	if summaries.gotInputs[0].Scope != scope || summaries.gotInputs[0].Subject.Kind != UsefulnessFeedbackSubjectSession {
		t.Fatalf("first summary input = %+v, want scoped session subject", summaries.gotInputs[0])
	}
}

func TestMemorySessionReportIncludesTaskSummariesAndTaskNextActions(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubMemorySessionStore{
		sessions: []MemorySessionRun{{
			ID:      "session_1",
			Scope:   scope,
			Status:  MemorySessionStatusFailed,
			Verdict: ScopeProofVerdictFailed,
			Turns: []MemorySessionTurn{{
				ID:              "turn_1",
				SessionID:       "session_1",
				Scope:           scope,
				OutcomeEventIDs: []string{"evt_1"},
			}},
		}},
	}
	taskSummaries := &stubTaskSummaryReader{
		summaries: map[string]TaskEvaluationSummary{
			"session:session_1": {
				Scope:             scope,
				TotalEvaluations:  1,
				ActiveEvaluations: 1,
				VerdictCounts: map[TaskEvaluationVerdict]int{
					TaskEvaluationVerdictFailed: 1,
				},
				ContributionCounts: map[TaskContributionCategory]int{
					TaskContributionCategoryMemoryMissing: 1,
				},
				TaskEvaluationIDs:    []string{"task_eval_1"},
				LastTaskEvaluationID: "task_eval_1",
			},
		},
	}
	service := NewMemorySessionService(MemorySessionServiceOptions{Store: store, TaskSummarizer: taskSummaries})

	report, err := service.ReadSessionReport(context.Background(), ReadMemorySessionRunInput{Scope: scope, SessionID: "session_1"})
	if err != nil {
		t.Fatalf("ReadSessionReport() error = %v", err)
	}
	if len(report.TaskEvaluationSummaries) != 1 {
		t.Fatalf("task summaries = %+v, want session summary", report.TaskEvaluationSummaries)
	}
	if report.TaskEvaluationSummaries[0].LatestTaskEvaluationID != "task_eval_1" {
		t.Fatalf("task summary = %+v, want latest task id", report.TaskEvaluationSummaries[0])
	}
	if !containsString(report.NextActions, "review_task_failure") || !containsString(report.NextActions, "open_task_summary") {
		t.Fatalf("next actions = %+v, want task-derived actions", report.NextActions)
	}
}

func TestMemorySessionServicePropagatesTurnAndOutcomeIdempotencyKeys(t *testing.T) {
	now := time.Date(2026, 7, 11, 21, 0, 0, 0, time.UTC)
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubMemorySessionStore{
		sessions: []MemorySessionRun{{
			ID:      "session_1",
			Scope:   scope,
			Status:  MemorySessionStatusActive,
			Verdict: ScopeProofVerdictPending,
		}},
	}
	service := NewMemorySessionService(MemorySessionServiceOptions{
		Store: store,
		ContextAssembler: &stubMemorySessionContextAssembler{
			result: MemorySessionContextEvidence{Summary: "context"},
		},
		Now:   func() time.Time { return now },
		NewID: func(prefix string) string { return prefix + "_generated" },
	})

	first, err := service.CreateTurn(context.Background(), CreateMemorySessionTurnInput{
		Scope:          scope,
		SessionID:      "session_1",
		IdempotencyKey: "turn-key-1",
		Query:          "remember deployment preference",
	})
	if err != nil {
		t.Fatalf("CreateTurn() first error = %v", err)
	}
	second, err := service.CreateTurn(context.Background(), CreateMemorySessionTurnInput{
		Scope:          scope,
		SessionID:      "session_1",
		IdempotencyKey: "turn-key-1",
		Query:          "remember deployment preference",
	})
	if err != nil {
		t.Fatalf("CreateTurn() second error = %v", err)
	}
	if first.ID != second.ID || len(store.turns) != 1 {
		t.Fatalf("turns = %+v, first = %+v second = %+v, want duplicate-safe idempotent turn", store.turns, first, second)
	}
	if first.IdempotencyKey != "turn-key-1" {
		t.Fatalf("IdempotencyKey = %q, want propagated key", first.IdempotencyKey)
	}

	updated, err := service.RecordTurnOutcome(context.Background(), RecordMemorySessionTurnOutcomeInput{
		Scope:          scope,
		SessionID:      "session_1",
		TurnID:         first.ID,
		IdempotencyKey: "outcome-key-1",
		OutcomeEventIDs: []string{
			"evt_1",
		},
		ExpectedRecall: []string{"evt_1"},
	})
	if err != nil {
		t.Fatalf("RecordTurnOutcome() error = %v", err)
	}
	if updated.OutcomeIdempotencyKey != "outcome-key-1" {
		t.Fatalf("OutcomeIdempotencyKey = %q, want propagated key", updated.OutcomeIdempotencyKey)
	}
}

func TestMemorySessionServiceIngestsOutcomeEventPayloadsThroughEventPath(t *testing.T) {
	now := time.Date(2026, 7, 11, 22, 0, 0, 0, time.UTC)
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	store := &stubMemorySessionStore{
		sessions: []MemorySessionRun{{
			ID:      "session_1",
			Scope:   scope,
			Status:  MemorySessionStatusActive,
			Verdict: ScopeProofVerdictPending,
		}},
		turns: []MemorySessionTurn{{
			ID:        "turn_1",
			SessionID: "session_1",
			Scope:     scope,
			Status:    MemorySessionTurnStatusContextAssembled,
		}},
	}
	ingestor := &stubMemorySessionOutcomeIngestor{eventID: "evt_ingested"}
	service := NewMemorySessionService(MemorySessionServiceOptions{
		Store:         store,
		EventIngestor: ingestor,
		Now:           func() time.Time { return now },
	})

	turn, err := service.RecordTurnOutcome(context.Background(), RecordMemorySessionTurnOutcomeInput{
		Scope:          scope,
		SessionID:      "session_1",
		TurnID:         "turn_1",
		IdempotencyKey: "outcome-key-1",
		OutcomeEventPayloads: []MemorySessionOutcomeEventPayload{{
			EventType: "agent_observation",
			Content:   "The user prefers staged rollouts.",
			Metadata:  map[string]any{"source": "test-agent"},
		}},
		ExpectedRecall: []string{"evt_ingested"},
	})
	if err != nil {
		t.Fatalf("RecordTurnOutcome() error = %v", err)
	}
	if len(turn.OutcomeEventIDs) != 1 || turn.OutcomeEventIDs[0] != "evt_ingested" {
		t.Fatalf("OutcomeEventIDs = %+v, want ingested event id", turn.OutcomeEventIDs)
	}
	if ingestor.gotInput.Scope != scope || ingestor.gotInput.EventType != "agent_observation" {
		t.Fatalf("ingest input = %+v, want scoped outcome event", ingestor.gotInput)
	}
	if ingestor.gotInput.Metadata["memory_session_id"] != "session_1" || ingestor.gotInput.Metadata["memory_session_turn_id"] != "turn_1" || ingestor.gotInput.Metadata["outcome_idempotency_key"] != "outcome-key-1" {
		t.Fatalf("ingest metadata = %+v, want session/turn outcome attribution", ingestor.gotInput.Metadata)
	}
}

func TestMemorySessionServiceRejectsInvalidOutcomeEventPayload(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	service := NewMemorySessionService(MemorySessionServiceOptions{
		Store: &stubMemorySessionStore{
			sessions: []MemorySessionRun{{
				ID:      "session_1",
				Scope:   scope,
				Status:  MemorySessionStatusActive,
				Verdict: ScopeProofVerdictPending,
			}},
		},
		EventIngestor: &stubMemorySessionOutcomeIngestor{eventID: "evt_should_not_write"},
	})

	_, err := service.RecordTurnOutcome(context.Background(), RecordMemorySessionTurnOutcomeInput{
		Scope:     scope,
		SessionID: "session_1",
		TurnID:    "turn_1",
		OutcomeEventPayloads: []MemorySessionOutcomeEventPayload{{
			EventType: "agent_observation",
			Content:   "",
		}},
	})
	if err == nil {
		t.Fatal("RecordTurnOutcome() error = nil, want invalid payload rejection")
	}
	if !strings.Contains(err.Error(), "content is required") {
		t.Fatalf("RecordTurnOutcome() error = %v, want content validation", err)
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

type stubUsefulnessSummaryReader struct {
	gotInputs []SummarizeUsefulnessFeedbackInput
	summaries map[string]UsefulnessFeedbackSummary
}

type stubTaskSummaryReader struct {
	gotInputs []SummarizeTaskEvaluationsInput
	summaries map[string]TaskEvaluationSummary
}

type stubMemorySessionOutcomeIngestor struct {
	eventID  string
	gotInput IngestEventInput
}

func (s *stubMemorySessionOutcomeIngestor) Ingest(ctx context.Context, input IngestEventInput) (RawEvent, error) {
	s.gotInput = input
	return RawEvent{ID: s.eventID, Scope: input.Scope, EventType: input.EventType, Content: input.Content, Metadata: input.Metadata}, nil
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
			session.Verifications = append([]MemorySessionVerification(nil), s.verifications...)
			return session, nil
		}
	}
	return MemorySessionRun{}, nil
}

func (s *stubUsefulnessSummaryReader) SummarizeUsefulnessFeedback(ctx context.Context, input SummarizeUsefulnessFeedbackInput) (UsefulnessFeedbackSummary, error) {
	s.gotInputs = append(s.gotInputs, input)
	if summary, ok := s.summaries[usefulnessFeedbackSummaryTestKey(input.Subject)]; ok {
		return summary, nil
	}
	return UsefulnessFeedbackSummary{Subject: input.Subject, EffectiveQuality: UsefulnessQualityUnknown}, nil
}

func (s *stubTaskSummaryReader) SummarizeTaskEvaluations(ctx context.Context, input SummarizeTaskEvaluationsInput) (TaskEvaluationSummary, error) {
	s.gotInputs = append(s.gotInputs, input)
	key := string(input.EvidenceTargetKind) + ":" + input.EvidenceTargetID
	if summary, ok := s.summaries[key]; ok {
		return summary, nil
	}
	return TaskEvaluationSummary{Scope: input.Scope.Normalized(), VerdictCounts: map[TaskEvaluationVerdict]int{}, ContributionCounts: map[TaskContributionCategory]int{}}, nil
}

func usefulnessFeedbackSummaryTestKey(subject UsefulnessFeedbackSubject) string {
	if subject.Kind == UsefulnessFeedbackSubjectExpectedRecall {
		return string(subject.Kind) + ":" + string(subject.ExpectedRecallTarget.Kind) + ":" + subject.ExpectedRecallTarget.ID + subject.ExpectedRecallTarget.OpaqueToken
	}
	return string(subject.Kind) + ":" + subject.ID
}

func (s *stubMemorySessionStore) CreateMemorySessionTurn(ctx context.Context, turn MemorySessionTurn) (MemorySessionTurn, error) {
	if turn.IdempotencyKey != "" {
		for _, existing := range s.turns {
			if existing.Scope == turn.Scope && existing.SessionID == turn.SessionID && existing.IdempotencyKey == turn.IdempotencyKey {
				return existing, nil
			}
		}
	}
	s.turns = append(s.turns, turn)
	return turn, nil
}

func (s *stubMemorySessionStore) UpdateMemorySessionTurnOutcome(ctx context.Context, input UpdateMemorySessionTurnOutcomeInput) (MemorySessionTurn, error) {
	for index, turn := range s.turns {
		if turn.ID == input.TurnID && turn.SessionID == input.SessionID && turn.Scope == input.Scope {
			turn.Status = input.Status
			turn.OutcomeIdempotencyKey = input.OutcomeIdempotencyKey
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
