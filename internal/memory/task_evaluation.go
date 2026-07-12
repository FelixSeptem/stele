package memory

import (
	"fmt"
	"strings"
	"time"
)

const (
	maxTaskEvaluationObjectiveLength      = 4096
	maxTaskEvaluationCriteriaLength       = 1024
	maxTaskEvaluationActorLength          = 256
	maxTaskEvaluationReasonLength         = 2048
	maxTaskEvaluationIdempotencyKeyLength = 256
	maxTaskEvaluationMetadataEntries      = 32
	maxTaskEvaluationMetadataKeyLength    = 128
	maxTaskEvaluationMetadataValueLength  = 1024
	maxTaskEvaluationEvidenceIDLength     = 512
)

type TaskEvaluationVerdict string

const (
	TaskEvaluationVerdictSucceeded    TaskEvaluationVerdict = "succeeded"
	TaskEvaluationVerdictFailed       TaskEvaluationVerdict = "failed"
	TaskEvaluationVerdictPartial      TaskEvaluationVerdict = "partial"
	TaskEvaluationVerdictInconclusive TaskEvaluationVerdict = "inconclusive"
)

func (v TaskEvaluationVerdict) Valid() bool {
	switch v {
	case TaskEvaluationVerdictSucceeded,
		TaskEvaluationVerdictFailed,
		TaskEvaluationVerdictPartial,
		TaskEvaluationVerdictInconclusive:
		return true
	default:
		return false
	}
}

func (v TaskEvaluationVerdict) Negative() bool {
	switch v {
	case TaskEvaluationVerdictFailed, TaskEvaluationVerdictPartial:
		return true
	default:
		return false
	}
}

type TaskContributionCategory string

const (
	TaskContributionCategoryMemoryMissing    TaskContributionCategory = "memory_missing"
	TaskContributionCategoryMemoryNoisy      TaskContributionCategory = "memory_noisy"
	TaskContributionCategoryMemoryStale      TaskContributionCategory = "memory_stale"
	TaskContributionCategoryMemoryIrrelevant TaskContributionCategory = "memory_irrelevant"
	TaskContributionCategoryHiddenMemory     TaskContributionCategory = "hidden_memory"
	TaskContributionCategoryExternalTool     TaskContributionCategory = "external_tool"
	TaskContributionCategoryAgentRuntime     TaskContributionCategory = "agent_runtime"
	TaskContributionCategoryUnknown          TaskContributionCategory = "unknown"
)

func (c TaskContributionCategory) Valid() bool {
	switch c {
	case TaskContributionCategoryMemoryMissing,
		TaskContributionCategoryMemoryNoisy,
		TaskContributionCategoryMemoryStale,
		TaskContributionCategoryMemoryIrrelevant,
		TaskContributionCategoryHiddenMemory,
		TaskContributionCategoryExternalTool,
		TaskContributionCategoryAgentRuntime,
		TaskContributionCategoryUnknown:
		return true
	default:
		return false
	}
}

func (c TaskContributionCategory) MemoryRelated() bool {
	switch c {
	case TaskContributionCategoryMemoryMissing,
		TaskContributionCategoryMemoryNoisy,
		TaskContributionCategoryMemoryStale,
		TaskContributionCategoryMemoryIrrelevant,
		TaskContributionCategoryHiddenMemory:
		return true
	default:
		return false
	}
}

type TaskEvaluationCorrectionState string

const (
	TaskEvaluationCorrectionStateActive     TaskEvaluationCorrectionState = "active"
	TaskEvaluationCorrectionStateSuperseded TaskEvaluationCorrectionState = "superseded"
)

func (s TaskEvaluationCorrectionState) Valid() bool {
	switch s {
	case TaskEvaluationCorrectionStateActive, TaskEvaluationCorrectionStateSuperseded:
		return true
	default:
		return false
	}
}

type TaskEvidenceLink struct {
	Kind        TaskEvidenceTargetKind `json:"kind"`
	ID          string                 `json:"id,omitempty"`
	OpaqueToken string                 `json:"opaque_token,omitempty"`
	Metadata    map[string]any         `json:"metadata,omitempty"`
}

func (l TaskEvidenceLink) Validate() error {
	if !l.Kind.Valid() {
		return fmt.Errorf("task evidence target kind %q is invalid", l.Kind)
	}
	if l.Kind == TaskEvidenceTargetOpaque {
		if strings.TrimSpace(l.OpaqueToken) == "" {
			return fmt.Errorf("opaque task evidence token is required")
		}
		if strings.TrimSpace(l.ID) != "" {
			return fmt.Errorf("opaque task evidence must not include an internal id")
		}
		return nil
	}
	if strings.TrimSpace(l.ID) == "" {
		return fmt.Errorf("task evidence id is required")
	}
	if len(l.ID) > maxTaskEvaluationEvidenceIDLength {
		return fmt.Errorf("task evidence id must be at most %d bytes", maxTaskEvaluationEvidenceIDLength)
	}
	if strings.TrimSpace(l.OpaqueToken) != "" {
		return fmt.Errorf("known task evidence must not include an opaque token")
	}
	return nil
}

type TaskEvaluation struct {
	ID                           string                        `json:"id"`
	Scope                        Scope                         `json:"scope"`
	Objective                    string                        `json:"objective"`
	SuccessCriteria              []string                      `json:"success_criteria,omitempty"`
	Verdict                      TaskEvaluationVerdict         `json:"verdict"`
	ContributionCategories       []TaskContributionCategory    `json:"contribution_categories,omitempty"`
	Evidence                     []TaskEvidenceLink            `json:"evidence,omitempty"`
	Actor                        string                        `json:"actor"`
	Reason                       string                        `json:"reason"`
	IdempotencyKey               string                        `json:"idempotency_key,omitempty"`
	Metadata                     map[string]any                `json:"metadata,omitempty"`
	CorrectionState              TaskEvaluationCorrectionState `json:"correction_state,omitempty"`
	SupersededAt                 time.Time                     `json:"superseded_at,omitempty"`
	SupersededByTaskEvaluationID string                        `json:"superseded_by_task_evaluation_id,omitempty"`
	SupersededByActor            string                        `json:"superseded_by_actor,omitempty"`
	SupersededByReason           string                        `json:"superseded_by_reason,omitempty"`
	CreatedAt                    time.Time                     `json:"created_at"`
	UpdatedAt                    time.Time                     `json:"updated_at"`
}

func (e TaskEvaluation) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("task evaluation id is required")
	}
	if err := e.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(e.Objective) == "" {
		return fmt.Errorf("task objective is required")
	}
	if len(e.Objective) > maxTaskEvaluationObjectiveLength {
		return fmt.Errorf("task objective must be at most %d bytes", maxTaskEvaluationObjectiveLength)
	}
	if len(e.SuccessCriteria) == 0 {
		return fmt.Errorf("at least one task success criteria item is required")
	}
	for _, criterion := range e.SuccessCriteria {
		if strings.TrimSpace(criterion) == "" {
			return fmt.Errorf("task success criteria item is required")
		}
		if len(criterion) > maxTaskEvaluationCriteriaLength {
			return fmt.Errorf("task success criteria item must be at most %d bytes", maxTaskEvaluationCriteriaLength)
		}
	}
	if !e.Verdict.Valid() {
		return fmt.Errorf("task verdict %q is invalid", e.Verdict)
	}
	for _, category := range e.ContributionCategories {
		if !category.Valid() {
			return fmt.Errorf("task contribution category %q is invalid", category)
		}
	}
	if len(e.Evidence) == 0 {
		return fmt.Errorf("at least one task evidence link is required")
	}
	for _, evidence := range e.Evidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
	}
	if strings.TrimSpace(e.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if len(e.Actor) > maxTaskEvaluationActorLength {
		return fmt.Errorf("actor must be at most %d bytes", maxTaskEvaluationActorLength)
	}
	if strings.TrimSpace(e.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if len(e.Reason) > maxTaskEvaluationReasonLength {
		return fmt.Errorf("reason must be at most %d bytes", maxTaskEvaluationReasonLength)
	}
	if e.IdempotencyKey != "" && len(e.IdempotencyKey) > maxTaskEvaluationIdempotencyKeyLength {
		return fmt.Errorf("idempotency key must be at most %d bytes", maxTaskEvaluationIdempotencyKeyLength)
	}
	if err := validateTaskEvaluationMetadata(e.Metadata); err != nil {
		return err
	}
	if e.CorrectionState != "" && !e.CorrectionState.Valid() {
		return fmt.Errorf("task evaluation correction state %q is invalid", e.CorrectionState)
	}
	if e.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	if e.UpdatedAt.IsZero() {
		return fmt.Errorf("updated at is required")
	}
	if !e.SupersededAt.IsZero() {
		if strings.TrimSpace(e.SupersededByActor) == "" {
			return fmt.Errorf("superseded by actor is required")
		}
		if strings.TrimSpace(e.SupersededByReason) == "" {
			return fmt.Errorf("superseded by reason is required")
		}
	}
	return nil
}

func validateTaskEvaluationMetadata(metadata map[string]any) error {
	if len(metadata) > maxTaskEvaluationMetadataEntries {
		return fmt.Errorf("metadata must contain at most %d entries", maxTaskEvaluationMetadataEntries)
	}
	for key, value := range metadata {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("metadata key is required")
		}
		if len(key) > maxTaskEvaluationMetadataKeyLength {
			return fmt.Errorf("metadata key must be at most %d bytes", maxTaskEvaluationMetadataKeyLength)
		}
		if len(fmt.Sprint(value)) > maxTaskEvaluationMetadataValueLength {
			return fmt.Errorf("metadata value must be at most %d bytes", maxTaskEvaluationMetadataValueLength)
		}
	}
	return nil
}

type ReadTaskEvaluationInput struct {
	Scope        Scope
	EvaluationID string
}

func (i ReadTaskEvaluationInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.EvaluationID) == "" {
		return fmt.Errorf("task evaluation id is required")
	}
	return nil
}

type ListTaskEvaluationsInput struct {
	Scope                Scope
	Verdict              TaskEvaluationVerdict
	ContributionCategory TaskContributionCategory
	EvidenceTargetKind   TaskEvidenceTargetKind
	EvidenceTargetID     string
	IncludeSuperseded    bool
	Limit                int
}

func (i ListTaskEvaluationsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Verdict != "" && !i.Verdict.Valid() {
		return fmt.Errorf("task verdict %q is invalid", i.Verdict)
	}
	if i.ContributionCategory != "" && !i.ContributionCategory.Valid() {
		return fmt.Errorf("task contribution category %q is invalid", i.ContributionCategory)
	}
	if i.EvidenceTargetKind != "" && !i.EvidenceTargetKind.Valid() {
		return fmt.Errorf("task evidence target kind %q is invalid", i.EvidenceTargetKind)
	}
	if i.Limit < 0 {
		return fmt.Errorf("limit must be greater than or equal to zero")
	}
	return nil
}

type SupersedeTaskEvaluationInput struct {
	Scope         Scope
	EvaluationID  string
	SupersedingID string
	Actor         string
	Reason        string
	SupersededAt  time.Time
}

func (i SupersedeTaskEvaluationInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.EvaluationID) == "" {
		return fmt.Errorf("task evaluation id is required")
	}
	if strings.TrimSpace(i.Actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if strings.TrimSpace(i.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if i.SupersededAt.IsZero() {
		return fmt.Errorf("superseded at is required")
	}
	return nil
}

type SummarizeTaskEvaluationsInput struct {
	Scope              Scope
	EvidenceTargetKind TaskEvidenceTargetKind
	EvidenceTargetID   string
}

func (i SummarizeTaskEvaluationsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.EvidenceTargetKind != "" && !i.EvidenceTargetKind.Valid() {
		return fmt.Errorf("task evidence target kind %q is invalid", i.EvidenceTargetKind)
	}
	return nil
}

type TaskEvaluationSummary struct {
	Scope                Scope                            `json:"scope"`
	EvidenceTargetKind   TaskEvidenceTargetKind           `json:"evidence_target_kind,omitempty"`
	EvidenceTargetID     string                           `json:"evidence_target_id,omitempty"`
	TotalEvaluations     int                              `json:"total_evaluations"`
	ActiveEvaluations    int                              `json:"active_evaluations"`
	VerdictCounts        map[TaskEvaluationVerdict]int    `json:"verdict_counts"`
	ContributionCounts   map[TaskContributionCategory]int `json:"contribution_counts"`
	TaskEvaluationIDs    []string                         `json:"task_evaluation_ids,omitempty"`
	LastTaskEvaluationID string                           `json:"last_task_evaluation_id,omitempty"`
	LastEvaluationAt     time.Time                        `json:"last_evaluation_at,omitempty"`
}

type TaskSummarySignal struct {
	Scope                  Scope                            `json:"scope"`
	EvidenceTargetKind     TaskEvidenceTargetKind           `json:"evidence_target_kind,omitempty"`
	EvidenceTargetID       string                           `json:"evidence_target_id,omitempty"`
	TaskEvaluationIDs      []string                         `json:"task_evaluation_ids,omitempty"`
	TaskVerdictCounts      map[TaskEvaluationVerdict]int    `json:"task_verdict_counts,omitempty"`
	TaskContributionCounts map[TaskContributionCategory]int `json:"task_contribution_counts,omitempty"`
	QualityFindingCodes    []QualityFindingCode             `json:"quality_finding_codes,omitempty"`
	NextActions            []string                         `json:"next_actions,omitempty"`
	LastTaskEvaluationAt   time.Time                        `json:"last_task_evaluation_at,omitempty"`
	LatestTaskEvaluationID string                           `json:"latest_task_evaluation_id,omitempty"`
}

type TaskEvaluationReport struct {
	Evaluation                   TaskEvaluation             `json:"evaluation"`
	Summary                      TaskEvaluationSummary      `json:"summary"`
	Evidence                     []TaskEvidenceLink         `json:"evidence,omitempty"`
	LinkedSessionIDs             []string                   `json:"linked_session_ids,omitempty"`
	LinkedTurnIDs                []string                   `json:"linked_turn_ids,omitempty"`
	LinkedRawEventIDs            []string                   `json:"linked_raw_event_ids,omitempty"`
	LinkedOutcomeEventIDs        []string                   `json:"linked_outcome_event_ids,omitempty"`
	LinkedVerificationIDs        []string                   `json:"linked_verification_ids,omitempty"`
	LinkedExpectedRecallIDs      []string                   `json:"linked_expected_recall_ids,omitempty"`
	LinkedFeedbackIDs            []string                   `json:"linked_feedback_ids,omitempty"`
	LinkedContextCitationIDs     []string                   `json:"linked_context_citation_ids,omitempty"`
	LinkedDerivedInsightIDs      []string                   `json:"linked_derived_insight_ids,omitempty"`
	LinkedMemoryIDs              []string                   `json:"linked_memory_ids,omitempty"`
	LinkedQualityFindingIDs      []string                   `json:"linked_quality_finding_ids,omitempty"`
	LinkedRepairPlanIDs          []string                   `json:"linked_repair_plan_ids,omitempty"`
	MemoryContributionCategories []TaskContributionCategory `json:"memory_contribution_categories,omitempty"`
	NextActions                  []string                   `json:"next_actions,omitempty"`
}

func BuildTaskEvaluationReport(evaluation TaskEvaluation, summary TaskEvaluationSummary) TaskEvaluationReport {
	sanitizedEvaluation := evaluation
	sanitizedEvaluation.Evidence = sanitizeTaskEvidenceLinks(evaluation.Evidence)

	report := TaskEvaluationReport{
		Evaluation: sanitizedEvaluation,
		Summary:    summary,
		Evidence:   sanitizeTaskEvidenceLinks(evaluation.Evidence),
	}
	report.LinkedSessionIDs = taskEvidenceIDs(report.Evidence, TaskEvidenceTargetSession)
	report.LinkedTurnIDs = taskEvidenceIDs(report.Evidence, TaskEvidenceTargetTurn)
	report.LinkedRawEventIDs = taskEvidenceIDs(report.Evidence, TaskEvidenceTargetRawEvent)
	report.LinkedOutcomeEventIDs = taskEvidenceIDs(report.Evidence, TaskEvidenceTargetOutcomeEvent)
	report.LinkedVerificationIDs = taskEvidenceIDs(report.Evidence, TaskEvidenceTargetVerification)
	report.LinkedExpectedRecallIDs = taskEvidenceIDs(report.Evidence, TaskEvidenceTargetExpectedRecall)
	report.LinkedFeedbackIDs = taskEvidenceIDs(report.Evidence, TaskEvidenceTargetUsefulnessFeedback)
	report.LinkedContextCitationIDs = taskEvidenceIDs(report.Evidence, TaskEvidenceTargetContextCitation)
	report.LinkedDerivedInsightIDs = taskEvidenceIDs(report.Evidence, TaskEvidenceTargetDerivedInsight)
	report.LinkedMemoryIDs = append(taskEvidenceIDs(report.Evidence, TaskEvidenceTargetExpectedRecall), taskEvidenceIDs(report.Evidence, TaskEvidenceTargetMemory)...)
	report.LinkedQualityFindingIDs = taskEvidenceIDs(report.Evidence, TaskEvidenceTargetQualityFinding)
	report.LinkedRepairPlanIDs = taskEvidenceIDs(report.Evidence, TaskEvidenceTargetRepairPlan)
	report.MemoryContributionCategories = append([]TaskContributionCategory(nil), evaluation.ContributionCategories...)
	report.NextActions = taskEvaluationNextActions(evaluation)
	return report
}

func sanitizeTaskEvidenceLinks(links []TaskEvidenceLink) []TaskEvidenceLink {
	sanitized := make([]TaskEvidenceLink, 0, len(links))
	for _, link := range links {
		copy := link
		if copy.Kind == TaskEvidenceTargetOpaque {
			copy.ID = ""
			copy.OpaqueToken = ""
		}
		sanitized = append(sanitized, copy)
	}
	return sanitized
}

func taskEvidenceIDs(links []TaskEvidenceLink, kind TaskEvidenceTargetKind) []string {
	ids := make([]string, 0)
	for _, link := range links {
		if link.Kind != kind {
			continue
		}
		if strings.TrimSpace(link.ID) != "" {
			ids = append(ids, strings.TrimSpace(link.ID))
		}
	}
	return ids
}

func taskEvaluationNextActions(evaluation TaskEvaluation) []string {
	nextActions := make([]string, 0, 3)
	if containsTaskContributionCategory(evaluation.ContributionCategories, TaskContributionCategoryMemoryMissing) {
		nextActions = append(nextActions, "review_missing_memory")
	}
	if containsTaskContributionCategory(evaluation.ContributionCategories, TaskContributionCategoryMemoryNoisy) ||
		containsTaskContributionCategory(evaluation.ContributionCategories, TaskContributionCategoryMemoryStale) ||
		containsTaskContributionCategory(evaluation.ContributionCategories, TaskContributionCategoryMemoryIrrelevant) {
		nextActions = append(nextActions, "review_memory_contribution")
	}
	if containsTaskContributionCategory(evaluation.ContributionCategories, TaskContributionCategoryHiddenMemory) {
		nextActions = append(nextActions, "review_hidden_evidence")
	}
	if evaluation.Verdict == TaskEvaluationVerdictFailed || evaluation.Verdict == TaskEvaluationVerdictPartial {
		nextActions = append(nextActions, "consider_task_followup")
	}
	return uniqueStrings(nextActions)
}

func containsTaskContributionCategory(values []TaskContributionCategory, want TaskContributionCategory) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func SummarizeTaskEvaluations(input SummarizeTaskEvaluationsInput, records []TaskEvaluation) TaskEvaluationSummary {
	summary := TaskEvaluationSummary{
		Scope:              input.Scope.Normalized(),
		EvidenceTargetKind: input.EvidenceTargetKind,
		EvidenceTargetID:   strings.TrimSpace(input.EvidenceTargetID),
		VerdictCounts: map[TaskEvaluationVerdict]int{
			TaskEvaluationVerdictSucceeded:    0,
			TaskEvaluationVerdictFailed:       0,
			TaskEvaluationVerdictPartial:      0,
			TaskEvaluationVerdictInconclusive: 0,
		},
		ContributionCounts: map[TaskContributionCategory]int{
			TaskContributionCategoryMemoryMissing:    0,
			TaskContributionCategoryMemoryNoisy:      0,
			TaskContributionCategoryMemoryStale:      0,
			TaskContributionCategoryMemoryIrrelevant: 0,
			TaskContributionCategoryHiddenMemory:     0,
			TaskContributionCategoryExternalTool:     0,
			TaskContributionCategoryAgentRuntime:     0,
			TaskContributionCategoryUnknown:          0,
		},
	}
	for _, record := range records {
		if record.Scope.Normalized() != summary.Scope {
			continue
		}
		if input.EvidenceTargetKind != "" && !taskEvaluationMatchesEvidenceTarget(record, input) {
			continue
		}
		summary.TotalEvaluations++
		if record.CorrectionState != TaskEvaluationCorrectionStateSuperseded {
			summary.ActiveEvaluations++
			summary.TaskEvaluationIDs = append(summary.TaskEvaluationIDs, record.ID)
			summary.VerdictCounts[record.Verdict]++
			for _, category := range record.ContributionCategories {
				summary.ContributionCounts[category]++
			}
			if record.CreatedAt.After(summary.LastEvaluationAt) {
				summary.LastEvaluationAt = record.CreatedAt
				summary.LastTaskEvaluationID = record.ID
			}
		}
	}
	summary.TaskEvaluationIDs = uniqueStrings(summary.TaskEvaluationIDs)
	return summary
}

func taskEvaluationMatchesEvidenceTarget(record TaskEvaluation, input SummarizeTaskEvaluationsInput) bool {
	if input.EvidenceTargetKind == "" {
		return true
	}
	for _, evidence := range record.Evidence {
		if evidence.Kind != input.EvidenceTargetKind {
			continue
		}
		if strings.TrimSpace(input.EvidenceTargetID) != "" {
			if strings.TrimSpace(evidence.ID) == strings.TrimSpace(input.EvidenceTargetID) {
				return true
			}
			if strings.TrimSpace(evidence.OpaqueToken) == strings.TrimSpace(input.EvidenceTargetID) {
				return true
			}
			continue
		}
		return true
	}
	return false
}

func taskQualityFindingCodes(summary TaskEvaluationSummary) []QualityFindingCode {
	codes := make([]QualityFindingCode, 0, 5)
	if summary.ContributionCounts[TaskContributionCategoryMemoryMissing] > 0 {
		codes = append(codes, QualityFindingExpectedRecallMissing)
	}
	if summary.ContributionCounts[TaskContributionCategoryMemoryNoisy] > 0 {
		codes = append(codes, QualityFindingFeedbackNoisyRepeated)
	}
	if summary.ContributionCounts[TaskContributionCategoryMemoryStale] > 0 {
		codes = append(codes, QualityFindingFeedbackStaleRepeated)
	}
	if summary.ContributionCounts[TaskContributionCategoryMemoryIrrelevant] > 0 {
		codes = append(codes, QualityFindingFeedbackIrrelevantRepeated)
	}
	if summary.ContributionCounts[TaskContributionCategoryHiddenMemory] > 0 {
		codes = append(codes, QualityFindingFeedbackUnsafeOrHidden)
	}
	if summary.VerdictCounts[TaskEvaluationVerdictInconclusive] > 0 {
		codes = append(codes, QualityFindingFeedbackNeedsReview)
	}
	return uniqueQualityFindingCodes(codes)
}

func taskSummaryNextActions(summary TaskEvaluationSummary) []string {
	actions := make([]string, 0, 4)
	if summary.ContributionCounts[TaskContributionCategoryMemoryMissing] > 0 {
		actions = append(actions, "review_missing_memory")
	}
	if summary.ContributionCounts[TaskContributionCategoryMemoryNoisy] > 0 ||
		summary.ContributionCounts[TaskContributionCategoryMemoryStale] > 0 ||
		summary.ContributionCounts[TaskContributionCategoryMemoryIrrelevant] > 0 {
		actions = append(actions, "review_memory_contribution")
	}
	if summary.ContributionCounts[TaskContributionCategoryHiddenMemory] > 0 {
		actions = append(actions, "review_hidden_evidence")
	}
	if summary.VerdictCounts[TaskEvaluationVerdictFailed] > 0 ||
		summary.VerdictCounts[TaskEvaluationVerdictPartial] > 0 ||
		summary.VerdictCounts[TaskEvaluationVerdictInconclusive] > 0 {
		actions = append(actions, "consider_task_followup")
	}
	return uniqueStrings(actions)
}
