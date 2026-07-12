package memory

import (
	"strings"
	"testing"
	"time"
)

func TestTaskEvaluationValidateAcceptsBoundedEvidenceAndVerdict(t *testing.T) {
	evaluation := TaskEvaluation{
		ID:        "task_eval_1",
		Scope:     Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Objective: "Verify memory recall stays scoped",
		SuccessCriteria: []string{
			"returns only scoped memory",
			"preserves hidden content isolation",
		},
		Verdict: TaskEvaluationVerdictSucceeded,
		Evidence: []TaskEvidenceLink{{
			Kind: TaskEvidenceTargetSession,
			ID:   "session_1",
		}},
		ContributionCategories: []TaskContributionCategory{
			TaskContributionCategoryUnknown,
		},
		Actor:     "operator-a",
		Reason:    "external task completed",
		CreatedAt: time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC),
	}

	if err := evaluation.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	evaluation.Verdict = TaskEvaluationVerdict("free_form")
	if err := evaluation.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid verdict")
	}
}

func TestTaskEvaluationValidateAcceptsOpaqueEvidenceTokens(t *testing.T) {
	evaluation := TaskEvaluation{
		ID:              "task_eval_opaque",
		Scope:           Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Objective:       "Check opaque evidence handling",
		SuccessCriteria: []string{"opaque evidence is persisted"},
		Verdict:         TaskEvaluationVerdictInconclusive,
		Evidence: []TaskEvidenceLink{{
			Kind:        TaskEvidenceTargetOpaque,
			OpaqueToken: "caller-evidence-token",
		}},
		Actor:     "operator-a",
		Reason:    "caller provided opaque evidence",
		CreatedAt: time.Date(2026, 7, 12, 8, 5, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 12, 8, 5, 0, 0, time.UTC),
	}

	if err := evaluation.Validate(); err != nil {
		t.Fatalf("Validate() opaque error = %v", err)
	}
}

func TestTaskEvaluationValidateBoundsMetadataAndWhitespace(t *testing.T) {
	evaluation := validTaskEvaluation()
	evaluation.Metadata = map[string]any{strings.Repeat("k", 129): "value"}
	if err := evaluation.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want metadata key bound error")
	}

	evaluation = validTaskEvaluation()
	evaluation.SuccessCriteria = []string{strings.Repeat("c", 1025)}
	if err := evaluation.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want success criteria bound error")
	}
}

func TestSummarizeTaskEvaluationsExcludesSupersededRecords(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	subject := TaskEvidenceLink{Kind: TaskEvidenceTargetMemory, ID: "mem_1"}
	now := time.Date(2026, 7, 12, 8, 10, 0, 0, time.UTC)

	summary := SummarizeTaskEvaluations(SummarizeTaskEvaluationsInput{
		Scope:              scope,
		EvidenceTargetKind: TaskEvidenceTargetMemory,
		EvidenceTargetID:   "mem_1",
	}, []TaskEvaluation{
		{
			ID:              "task_eval_1",
			Scope:           scope,
			Objective:       "objective",
			SuccessCriteria: []string{"criteria"},
			Verdict:         TaskEvaluationVerdictSucceeded,
			Evidence:        []TaskEvidenceLink{subject},
			Actor:           "operator-a",
			Reason:          "ok",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:                 "task_eval_2",
			Scope:              scope,
			Objective:          "objective",
			SuccessCriteria:    []string{"criteria"},
			Verdict:            TaskEvaluationVerdictFailed,
			Evidence:           []TaskEvidenceLink{subject},
			Actor:              "operator-a",
			Reason:             "superseded",
			CreatedAt:          now.Add(time.Minute),
			UpdatedAt:          now.Add(time.Minute),
			CorrectionState:    TaskEvaluationCorrectionStateSuperseded,
			SupersededAt:       now.Add(2 * time.Minute),
			SupersededByActor:  "operator-b",
			SupersededByReason: "corrected",
		},
	})

	if summary.ActiveEvaluations != 1 || summary.VerdictCounts[TaskEvaluationVerdictSucceeded] != 1 || summary.VerdictCounts[TaskEvaluationVerdictFailed] != 0 {
		t.Fatalf("summary = %+v, want superseded evaluation excluded", summary)
	}
	if len(summary.TaskEvaluationIDs) != 1 || summary.TaskEvaluationIDs[0] != "task_eval_1" || summary.LastTaskEvaluationID != "task_eval_1" {
		t.Fatalf("summary task ids = %+v last=%q, want active task_eval_1", summary.TaskEvaluationIDs, summary.LastTaskEvaluationID)
	}
}

func TestBuildTaskEvaluationReportSanitizesOpaqueEvidenceAndDerivesReferences(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	evaluation := TaskEvaluation{
		ID:              "task_eval_1",
		Scope:           scope,
		Objective:       "Validate task success reporting",
		SuccessCriteria: []string{"report linked evidence"},
		Verdict:         TaskEvaluationVerdictPartial,
		ContributionCategories: []TaskContributionCategory{
			TaskContributionCategoryMemoryMissing,
			TaskContributionCategoryHiddenMemory,
		},
		Evidence: []TaskEvidenceLink{
			{Kind: TaskEvidenceTargetSession, ID: "session_1"},
			{Kind: TaskEvidenceTargetTurn, ID: "turn_1"},
			{Kind: TaskEvidenceTargetRawEvent, ID: "raw_event_1"},
			{Kind: TaskEvidenceTargetOutcomeEvent, ID: "outcome_event_1"},
			{Kind: TaskEvidenceTargetVerification, ID: "verification_1"},
			{Kind: TaskEvidenceTargetExpectedRecall, ID: "mem_expected_1"},
			{Kind: TaskEvidenceTargetUsefulnessFeedback, ID: "feedback_1"},
			{Kind: TaskEvidenceTargetContextCitation, ID: "citation_1"},
			{Kind: TaskEvidenceTargetDerivedInsight, ID: "insight_1"},
			{Kind: TaskEvidenceTargetMemory, ID: "mem_1"},
			{Kind: TaskEvidenceTargetQualityFinding, ID: "finding_1"},
			{Kind: TaskEvidenceTargetRepairPlan, ID: "repair_1"},
			{Kind: TaskEvidenceTargetOpaque, OpaqueToken: "caller-opaque-evidence"},
		},
		Actor:     "operator-a",
		Reason:    "record external task outcome",
		CreatedAt: time.Date(2026, 7, 12, 8, 15, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 12, 8, 15, 0, 0, time.UTC),
	}
	summary := SummarizeTaskEvaluations(SummarizeTaskEvaluationsInput{Scope: scope}, []TaskEvaluation{evaluation})

	report := BuildTaskEvaluationReport(evaluation, summary)

	if len(report.Evidence) != len(evaluation.Evidence) {
		t.Fatalf("evidence count = %d, want %d", len(report.Evidence), len(evaluation.Evidence))
	}
	if report.Evidence[len(report.Evidence)-1].OpaqueToken != "" {
		t.Fatalf("report evidence leaked opaque token: %+v", report.Evidence[len(report.Evidence)-1])
	}
	if report.Evaluation.Evidence[len(report.Evaluation.Evidence)-1].OpaqueToken != "" {
		t.Fatalf("evaluation evidence leaked opaque token: %+v", report.Evaluation.Evidence[len(report.Evaluation.Evidence)-1])
	}
	if len(report.LinkedSessionIDs) != 1 || report.LinkedSessionIDs[0] != "session_1" {
		t.Fatalf("linked sessions = %+v, want session_1", report.LinkedSessionIDs)
	}
	if len(report.LinkedTurnIDs) != 1 || report.LinkedTurnIDs[0] != "turn_1" {
		t.Fatalf("linked turns = %+v, want turn_1", report.LinkedTurnIDs)
	}
	if len(report.LinkedVerificationIDs) != 1 || report.LinkedVerificationIDs[0] != "verification_1" {
		t.Fatalf("linked verifications = %+v, want verification_1", report.LinkedVerificationIDs)
	}
	if len(report.LinkedFeedbackIDs) != 1 || report.LinkedFeedbackIDs[0] != "feedback_1" {
		t.Fatalf("linked feedback = %+v, want feedback_1", report.LinkedFeedbackIDs)
	}
	if len(report.LinkedDerivedInsightIDs) != 1 || report.LinkedDerivedInsightIDs[0] != "insight_1" {
		t.Fatalf("linked derived insights = %+v, want insight_1", report.LinkedDerivedInsightIDs)
	}
	if len(report.LinkedMemoryIDs) != 2 || report.LinkedMemoryIDs[0] != "mem_expected_1" || report.LinkedMemoryIDs[1] != "mem_1" {
		t.Fatalf("linked memories = %+v, want expected recall and memory ids", report.LinkedMemoryIDs)
	}
	if !containsTaskEvaluationString(report.NextActions, "review_missing_memory") || !containsTaskEvaluationString(report.NextActions, "consider_task_followup") {
		t.Fatalf("next actions = %+v, want missing-memory and follow-up actions", report.NextActions)
	}
	if len(report.LinkedQualityFindingIDs) != 1 || report.LinkedQualityFindingIDs[0] != "finding_1" {
		t.Fatalf("linked quality findings = %+v, want finding_1", report.LinkedQualityFindingIDs)
	}
	if len(report.LinkedRepairPlanIDs) != 1 || report.LinkedRepairPlanIDs[0] != "repair_1" {
		t.Fatalf("linked repair plans = %+v, want repair_1", report.LinkedRepairPlanIDs)
	}
	if len(report.MemoryContributionCategories) != 2 {
		t.Fatalf("memory contribution categories = %+v, want task categories", report.MemoryContributionCategories)
	}
	if !containsTaskEvaluationString(report.NextActions, "review_missing_memory") || !containsTaskEvaluationString(report.NextActions, "review_hidden_evidence") || !containsTaskEvaluationString(report.NextActions, "consider_task_followup") {
		t.Fatalf("next actions = %+v, want bounded task actions", report.NextActions)
	}
}

func TestSummarizeTaskEvaluationsMarksInconclusiveTasksForReview(t *testing.T) {
	scope := Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 12, 8, 20, 0, 0, time.UTC)
	summary := SummarizeTaskEvaluations(SummarizeTaskEvaluationsInput{Scope: scope}, []TaskEvaluation{{
		ID:              "task_eval_1",
		Scope:           scope,
		Objective:       "Check incomplete task evidence",
		SuccessCriteria: []string{"report the task result"},
		Verdict:         TaskEvaluationVerdictInconclusive,
		Evidence: []TaskEvidenceLink{{
			Kind: TaskEvidenceTargetSession,
			ID:   "session_1",
		}},
		Actor:     "operator-a",
		Reason:    "insufficient evidence",
		CreatedAt: now,
		UpdatedAt: now,
	}})

	if summary.ActiveEvaluations != 1 {
		t.Fatalf("summary active evaluations = %d, want 1", summary.ActiveEvaluations)
	}
	if !containsQualityFindingCode(taskQualityFindingCodes(summary), QualityFindingFeedbackNeedsReview) {
		t.Fatalf("task quality finding codes = %+v, want needs review", taskQualityFindingCodes(summary))
	}
	if !containsTaskEvaluationString(taskSummaryNextActions(summary), "consider_task_followup") {
		t.Fatalf("task summary next actions = %+v, want follow-up for inconclusive task", taskSummaryNextActions(summary))
	}
}

func containsTaskEvaluationString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsQualityFindingCode(values []QualityFindingCode, want QualityFindingCode) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validTaskEvaluation() TaskEvaluation {
	now := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)
	return TaskEvaluation{
		ID:              "task_eval_1",
		Scope:           Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Objective:       "Verify memory recall stays scoped",
		SuccessCriteria: []string{"returns only scoped memory"},
		Verdict:         TaskEvaluationVerdictSucceeded,
		Evidence: []TaskEvidenceLink{{
			Kind: TaskEvidenceTargetSession,
			ID:   "session_1",
		}},
		Actor:     "operator-a",
		Reason:    "external task completed",
		CreatedAt: now,
		UpdatedAt: now,
	}
}
