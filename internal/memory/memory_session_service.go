package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type MemorySessionStore interface {
	CreateMemorySessionRun(ctx context.Context, session MemorySessionRun) (MemorySessionRun, error)
	ListMemorySessionRuns(ctx context.Context, input ListMemorySessionRunsInput) ([]MemorySessionRun, error)
	ReadMemorySessionRun(ctx context.Context, input ReadMemorySessionRunInput) (MemorySessionRun, error)
	CreateMemorySessionTurn(ctx context.Context, turn MemorySessionTurn) (MemorySessionTurn, error)
	UpdateMemorySessionTurnOutcome(ctx context.Context, input UpdateMemorySessionTurnOutcomeInput) (MemorySessionTurn, error)
	CreateMemorySessionVerification(ctx context.Context, verification MemorySessionVerification) (MemorySessionVerification, error)
}

type MemorySessionContextRequest struct {
	Scope                     Scope
	SessionID                 string
	Query                     string
	Budget                    int
	IncludeRelations          bool
	IncludeExperienceInsights bool
	IncludeDiagnostics        bool
}

type MemorySessionContextEvidence struct {
	Summary     string   `json:"summary,omitempty"`
	MemoryIDs   []string `json:"memory_ids,omitempty"`
	Citations   []string `json:"citations,omitempty"`
	Diagnostics []string `json:"diagnostics,omitempty"`
}

func (e MemorySessionContextEvidence) Map() map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(e.Summary) != "" {
		out["summary"] = strings.TrimSpace(e.Summary)
	}
	if len(e.MemoryIDs) > 0 {
		out["memory_ids"] = append([]string(nil), e.MemoryIDs...)
	}
	if len(e.Citations) > 0 {
		out["citations"] = append([]string(nil), e.Citations...)
	}
	if len(e.Diagnostics) > 0 {
		out["diagnostics"] = append([]string(nil), e.Diagnostics...)
	}
	return out
}

type MemorySessionContextAssembler interface {
	AssembleSessionContext(ctx context.Context, request MemorySessionContextRequest) (MemorySessionContextEvidence, error)
}

type UsefulnessFeedbackSummarizer interface {
	SummarizeUsefulnessFeedback(ctx context.Context, input SummarizeUsefulnessFeedbackInput) (UsefulnessFeedbackSummary, error)
}

type MemorySessionServiceOptions struct {
	Store                MemorySessionStore
	ContextAssembler     MemorySessionContextAssembler
	EventIngestor        EventIngestor
	UsefulnessSummarizer UsefulnessFeedbackSummarizer
	Now                  func() time.Time
	NewID                func(prefix string) string
}

type MemorySessionService struct {
	store                MemorySessionStore
	contextAssembler     MemorySessionContextAssembler
	eventIngestor        EventIngestor
	usefulnessSummarizer UsefulnessFeedbackSummarizer
	now                  func() time.Time
	newID                func(prefix string) string
}

func NewMemorySessionService(options MemorySessionServiceOptions) *MemorySessionService {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	newID := options.NewID
	if newID == nil {
		newID = func(prefix string) string {
			return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), now().UnixNano())
		}
	}
	return &MemorySessionService{
		store:                options.Store,
		contextAssembler:     options.ContextAssembler,
		eventIngestor:        options.EventIngestor,
		usefulnessSummarizer: options.UsefulnessSummarizer,
		now:                  now,
		newID:                newID,
	}
}

func (s *MemorySessionService) CreateSession(ctx context.Context, input CreateMemorySessionInput) (MemorySessionRun, error) {
	if err := input.Validate(); err != nil {
		return MemorySessionRun{}, err
	}
	if s.store == nil {
		return MemorySessionRun{}, fmt.Errorf("memory session store is not configured")
	}
	now := s.now().UTC()
	return s.store.CreateMemorySessionRun(ctx, MemorySessionRun{
		ID:        s.newID("session"),
		Scope:     input.Scope.Normalized(),
		Status:    MemorySessionStatusActive,
		Verdict:   ScopeProofVerdictPending,
		Actor:     strings.TrimSpace(input.Actor),
		Reason:    strings.TrimSpace(input.Reason),
		Metadata:  normalizeMap(input.Metadata),
		CreatedAt: now,
		UpdatedAt: now,
		StartedAt: now,
	})
}

func (s *MemorySessionService) ListSessions(ctx context.Context, input ListMemorySessionRunsInput) ([]MemorySessionRun, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("memory session store is not configured")
	}
	return s.store.ListMemorySessionRuns(ctx, input)
}

func (s *MemorySessionService) ReadSession(ctx context.Context, input ReadMemorySessionRunInput) (MemorySessionRun, error) {
	if err := input.Validate(); err != nil {
		return MemorySessionRun{}, err
	}
	if s.store == nil {
		return MemorySessionRun{}, fmt.Errorf("memory session store is not configured")
	}
	return s.store.ReadMemorySessionRun(ctx, input)
}

func (s *MemorySessionService) CreateTurn(ctx context.Context, input CreateMemorySessionTurnInput) (MemorySessionTurn, error) {
	if err := input.Validate(); err != nil {
		return MemorySessionTurn{}, err
	}
	if s.store == nil {
		return MemorySessionTurn{}, fmt.Errorf("memory session store is not configured")
	}
	if s.contextAssembler == nil {
		return MemorySessionTurn{}, fmt.Errorf("memory session context assembler is not configured")
	}
	session, err := s.store.ReadMemorySessionRun(ctx, ReadMemorySessionRunInput{Scope: input.Scope, SessionID: input.SessionID})
	if err != nil {
		return MemorySessionTurn{}, err
	}
	budget := input.ContextBudget
	if budget == 0 {
		budget = 1200
	}
	evidence, err := s.contextAssembler.AssembleSessionContext(ctx, MemorySessionContextRequest{
		Scope:                     input.Scope.Normalized(),
		SessionID:                 session.ID,
		Query:                     strings.TrimSpace(input.Query),
		Budget:                    budget,
		IncludeRelations:          input.IncludeRelations,
		IncludeExperienceInsights: input.IncludeExperienceInsights,
		IncludeDiagnostics:        input.IncludeDiagnostics,
	})
	if err != nil {
		return MemorySessionTurn{}, err
	}
	turnID := strings.TrimSpace(input.TurnID)
	if turnID == "" {
		turnID = s.newID("turn")
	}
	now := s.now().UTC()
	return s.store.CreateMemorySessionTurn(ctx, MemorySessionTurn{
		ID:                 turnID,
		SessionID:          session.ID,
		Scope:              input.Scope.Normalized(),
		IdempotencyKey:     strings.TrimSpace(input.IdempotencyKey),
		Status:             MemorySessionTurnStatusContextAssembled,
		Query:              strings.TrimSpace(input.Query),
		ContextEvidence:    evidence.Map(),
		VerificationStatus: ScopeProofVerdictPending,
		CreatedAt:          now,
		UpdatedAt:          now,
	})
}

func (s *MemorySessionService) RecordTurnOutcome(ctx context.Context, input RecordMemorySessionTurnOutcomeInput) (MemorySessionTurn, error) {
	if err := input.Validate(); err != nil {
		return MemorySessionTurn{}, err
	}
	if s.store == nil {
		return MemorySessionTurn{}, fmt.Errorf("memory session store is not configured")
	}
	if _, err := s.store.ReadMemorySessionRun(ctx, ReadMemorySessionRunInput{Scope: input.Scope, SessionID: input.SessionID}); err != nil {
		return MemorySessionTurn{}, err
	}
	outcomeEventIDs := append([]string(nil), input.OutcomeEventIDs...)
	if len(input.OutcomeEventPayloads) > 0 {
		if s.eventIngestor == nil {
			return MemorySessionTurn{}, fmt.Errorf("memory session outcome ingestor is not configured")
		}
		for _, payload := range input.OutcomeEventPayloads {
			event, err := s.eventIngestor.Ingest(ctx, IngestEventInput{
				Scope:           input.Scope.Normalized(),
				EventType:       strings.TrimSpace(payload.EventType),
				Content:         strings.TrimSpace(payload.Content),
				Metadata:        memorySessionOutcomeMetadata(payload.Metadata, input),
				SourceTimestamp: payload.SourceTimestamp,
			})
			if err != nil {
				return MemorySessionTurn{}, err
			}
			outcomeEventIDs = append(outcomeEventIDs, event.ID)
		}
	}
	return s.store.UpdateMemorySessionTurnOutcome(ctx, UpdateMemorySessionTurnOutcomeInput{
		Scope:                 input.Scope.Normalized(),
		SessionID:             strings.TrimSpace(input.SessionID),
		TurnID:                strings.TrimSpace(input.TurnID),
		OutcomeIdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
		Status:                MemorySessionTurnStatusOutcomeRecorded,
		OutcomeEventIDs:       outcomeEventIDs,
		ExpectedRecall:        append([]string(nil), input.ExpectedRecall...),
		VerificationStatus:    ScopeProofVerdictPending,
		UpdatedAt:             s.now().UTC(),
	})
}

func memorySessionOutcomeMetadata(metadata map[string]any, input RecordMemorySessionTurnOutcomeInput) map[string]any {
	out := normalizeMap(metadata)
	out["memory_session_id"] = strings.TrimSpace(input.SessionID)
	out["memory_session_turn_id"] = strings.TrimSpace(input.TurnID)
	out["memory_session_source"] = "outcome"
	if key := strings.TrimSpace(input.IdempotencyKey); key != "" {
		out["outcome_idempotency_key"] = key
	}
	return out
}

func (s *MemorySessionService) RequestVerification(ctx context.Context, input RequestMemorySessionVerificationInput) (MemorySessionVerification, error) {
	if err := input.Validate(); err != nil {
		return MemorySessionVerification{}, err
	}
	if s.store == nil {
		return MemorySessionVerification{}, fmt.Errorf("memory session store is not configured")
	}
	if _, err := s.store.ReadMemorySessionRun(ctx, ReadMemorySessionRunInput{Scope: input.Scope, SessionID: input.SessionID}); err != nil {
		return MemorySessionVerification{}, err
	}
	now := s.now().UTC()
	return s.store.CreateMemorySessionVerification(ctx, MemorySessionVerification{
		ID:             s.newID("verification"),
		SessionID:      strings.TrimSpace(input.SessionID),
		TurnID:         strings.TrimSpace(input.TurnID),
		Scope:          input.Scope.Normalized(),
		Status:         ScopeProofStepStatusPending,
		Verdict:        ScopeProofVerdictPending,
		ExpectedRecall: append([]string(nil), input.ExpectedRecall...),
		Evidence:       map[string]any{},
		CreatedAt:      now,
		UpdatedAt:      now,
	})
}

type MemorySessionReport struct {
	Session           MemorySessionRun            `json:"session"`
	Evidence          LoopReportEvidence          `json:"evidence,omitempty"`
	FeedbackSummaries []UsefulnessFeedbackSummary `json:"feedback_summaries,omitempty"`
	NextActions       []string                    `json:"next_actions,omitempty"`
}

func (s *MemorySessionService) ReadSessionReport(ctx context.Context, input ReadMemorySessionRunInput) (MemorySessionReport, error) {
	session, err := s.ReadSession(ctx, input)
	if err != nil {
		return MemorySessionReport{}, err
	}
	evidence := loopReportEvidenceFromSession(session)
	feedbackSummaries, err := s.memorySessionFeedbackSummaries(ctx, session)
	if err != nil {
		return MemorySessionReport{}, err
	}
	return MemorySessionReport{
		Session:           session,
		Evidence:          evidence,
		FeedbackSummaries: feedbackSummaries,
		NextActions:       memorySessionNextActions(session, evidence),
	}, nil
}

func (s *MemorySessionService) memorySessionFeedbackSummaries(ctx context.Context, session MemorySessionRun) ([]UsefulnessFeedbackSummary, error) {
	if s.usefulnessSummarizer == nil {
		return nil, nil
	}
	subjects := memorySessionFeedbackSubjects(session)
	summaries := make([]UsefulnessFeedbackSummary, 0, len(subjects))
	for _, subject := range subjects {
		summary, err := s.usefulnessSummarizer.SummarizeUsefulnessFeedback(ctx, SummarizeUsefulnessFeedbackInput{
			Scope:   session.Scope,
			Subject: subject,
		})
		if err != nil {
			return nil, err
		}
		if summary.TotalActive == 0 {
			continue
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func memorySessionFeedbackSubjects(session MemorySessionRun) []UsefulnessFeedbackSubject {
	subjects := []UsefulnessFeedbackSubject{{Kind: UsefulnessFeedbackSubjectSession, ID: session.ID}}
	for _, turn := range session.Turns {
		subjects = append(subjects, UsefulnessFeedbackSubject{Kind: UsefulnessFeedbackSubjectTurn, ID: turn.ID})
		for _, memoryID := range stringSliceEvidenceValue(turn.ContextEvidence, "memory_ids") {
			subjects = append(subjects, UsefulnessFeedbackSubject{Kind: UsefulnessFeedbackSubjectMemory, ID: memoryID})
		}
		for _, citationID := range stringSliceEvidenceValue(turn.ContextEvidence, "citations") {
			subjects = append(subjects, UsefulnessFeedbackSubject{Kind: UsefulnessFeedbackSubjectCitation, ID: citationID})
		}
		for _, eventID := range turn.OutcomeEventIDs {
			subjects = append(subjects, UsefulnessFeedbackSubject{Kind: UsefulnessFeedbackSubjectRawEvent, ID: eventID})
		}
		for _, expectedID := range turn.ExpectedRecall {
			subjects = append(subjects, expectedRecallEventSubject(expectedID))
		}
	}
	for _, verification := range session.Verifications {
		subjects = append(subjects, UsefulnessFeedbackSubject{Kind: UsefulnessFeedbackSubjectVerification, ID: verification.ID})
		for _, expectedID := range verification.ExpectedRecall {
			subjects = append(subjects, expectedRecallEventSubject(expectedID))
		}
	}
	return uniqueUsefulnessFeedbackSubjects(subjects)
}

func expectedRecallEventSubject(id string) UsefulnessFeedbackSubject {
	return UsefulnessFeedbackSubject{
		Kind: UsefulnessFeedbackSubjectExpectedRecall,
		ExpectedRecallTarget: ExpectedRecallTarget{
			Kind: ExpectedRecallTargetEvent,
			ID:   strings.TrimSpace(id),
		},
	}
}

func uniqueUsefulnessFeedbackSubjects(subjects []UsefulnessFeedbackSubject) []UsefulnessFeedbackSubject {
	seen := make(map[string]struct{}, len(subjects))
	unique := make([]UsefulnessFeedbackSubject, 0, len(subjects))
	for _, subject := range subjects {
		if err := subject.Validate(); err != nil {
			continue
		}
		key := usefulnessFeedbackSubjectKey(subject)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, subject)
	}
	return unique
}

func usefulnessFeedbackSubjectKey(subject UsefulnessFeedbackSubject) string {
	if subject.Kind == UsefulnessFeedbackSubjectExpectedRecall {
		return string(subject.Kind) + ":" + string(subject.ExpectedRecallTarget.Kind) + ":" + subject.ExpectedRecallTarget.ID + ":" + subject.ExpectedRecallTarget.OpaqueToken
	}
	return string(subject.Kind) + ":" + subject.ID
}

func stringSliceEvidenceValue(evidence map[string]any, key string) []string {
	value, ok := evidence[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return uniqueStrings(typed)
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				items = append(items, text)
			}
		}
		return uniqueStrings(items)
	default:
		return nil
	}
}

func qualityFindingCodesFromEvidence(evidence map[string]any, key string) []QualityFindingCode {
	values := stringSliceEvidenceValue(evidence, key)
	if len(values) == 0 {
		return nil
	}
	codes := make([]QualityFindingCode, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			codes = append(codes, QualityFindingCode(trimmed))
		}
	}
	return uniqueQualityFindingCodes(codes)
}

func uniqueQualityFindingCodes(codes []QualityFindingCode) []QualityFindingCode {
	seen := make(map[QualityFindingCode]struct{}, len(codes))
	unique := make([]QualityFindingCode, 0, len(codes))
	for _, code := range codes {
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		unique = append(unique, code)
	}
	return unique
}

func memorySessionNextActions(session MemorySessionRun, evidence LoopReportEvidence) []string {
	if session.Verdict == ScopeProofVerdictPassed {
		return []string{"no_action_required"}
	}
	if session.Status == MemorySessionStatusVerifying {
		return []string{"wait_for_session_verification"}
	}
	actions := make([]string, 0)
	if session.FailureCategory != "" {
		actions = append(actions, nextActionsForFailureCategory(session.FailureCategory)...)
	}
	for _, category := range evidence.FailureCategories {
		actions = append(actions, nextActionsForFailureCategory(category)...)
	}
	if len(evidence.QualityEvaluationIDs) > 0 {
		actions = append(actions, "open_quality_evaluation")
	}
	if len(evidence.RepairPlanIDs) > 0 {
		actions = append(actions, "open_repair_plan")
	}
	if len(evidence.ReplayRunIDs) > 0 {
		actions = append(actions, "inspect_derived_insight_replay")
	}
	actions = uniqueStrings(actions)
	if len(actions) > 0 {
		return actions
	}
	return []string{"request_session_verification"}
}

func loopReportEvidenceFromSession(session MemorySessionRun) LoopReportEvidence {
	var evidence LoopReportEvidence
	if session.FailureCategory != "" {
		evidence.FailureCategories = append(evidence.FailureCategories, session.FailureCategory)
	}
	for _, turn := range session.Turns {
		if turn.FailureCategory != "" {
			evidence.FailureCategories = append(evidence.FailureCategories, turn.FailureCategory)
		}
		if id := strings.TrimSpace(stringEvidenceValue(turn.ContextEvidence, "evaluation_run_id")); id != "" {
			evidence.QualityEvaluationIDs = append(evidence.QualityEvaluationIDs, id)
		}
		evidence.QualityFindingIDs = append(evidence.QualityFindingIDs, stringSliceEvidenceValue(turn.ContextEvidence, "quality_finding_ids")...)
		evidence.QualityFindingCodes = append(evidence.QualityFindingCodes, qualityFindingCodesFromEvidence(turn.ContextEvidence, "quality_finding_codes")...)
		if id := strings.TrimSpace(stringEvidenceValue(turn.ContextEvidence, "replay_run_id")); id != "" {
			evidence.ReplayRunIDs = append(evidence.ReplayRunIDs, id)
		}
		if id := strings.TrimSpace(stringEvidenceValue(turn.ContextEvidence, "repair_plan_id")); id != "" {
			evidence.RepairPlanIDs = append(evidence.RepairPlanIDs, id)
		}
	}
	evidence.QualityEvaluationIDs = uniqueStrings(evidence.QualityEvaluationIDs)
	evidence.QualityFindingIDs = uniqueStrings(evidence.QualityFindingIDs)
	evidence.QualityFindingCodes = uniqueQualityFindingCodes(evidence.QualityFindingCodes)
	evidence.ReplayRunIDs = uniqueStrings(evidence.ReplayRunIDs)
	evidence.RepairPlanIDs = uniqueStrings(evidence.RepairPlanIDs)
	evidence.FailureCategories = uniqueFailureCategories(evidence.FailureCategories)
	return evidence
}

func normalizeMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
