package telemetry

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/diagnostics"
)

func TestMetricsObserverExportsOperationsBacklogsAndAdmission(t *testing.T) {
	observer := NewMetricsObserver()
	ctx := context.Background()
	observer.RecordOperation(ctx, OperationEvent{
		Mode:      "scheduler",
		Component: "embedding_rebuild_job",
		Operation: "embedding_rebuild",
		Status:    "ok",
		Count:     3,
		Duration:  2 * time.Second,
	})
	observer.RecordBacklog(ctx, BacklogEvent{
		Mode:      "scheduler",
		Component: "embedding_rebuild_job",
		Queue:     "embedding_rebuilds",
		Status:    "ok",
		Pending:   5,
	})
	observer.RecordAdmission(ctx, AdmissionEvent{
		Component: "embedding_cutover",
		Operation: "preflight",
		Decision:  "deny",
		Blockers: []diagnostics.Finding{
			{Code: "zero_eligible_memory"},
		},
		Warnings: []diagnostics.Finding{
			{Code: "many_waves"},
		},
	})

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_operations_total{component="embedding_rebuild_job",mode="scheduler",operation="embedding_rebuild",status="ok"} 1`,
		`stele_operation_processed_total{component="embedding_rebuild_job",mode="scheduler",operation="embedding_rebuild",status="ok"} 3`,
		`stele_backlog_pending{component="embedding_rebuild_job",mode="scheduler",queue="embedding_rebuilds",status="ok"} 5`,
		`stele_admission_decisions_total{component="embedding_cutover",decision="deny",operation="preflight"} 1`,
		`stele_admission_findings_total{code="zero_eligible_memory",component="embedding_cutover",operation="preflight",severity="blocker"} 1`,
		`stele_admission_findings_total{code="many_waves",component="embedding_cutover",operation="preflight",severity="warning"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
}

func TestMetricsObserverExportsEmbeddingCutoverAndProviderReadinessSignals(t *testing.T) {
	observer := NewMetricsObserver()
	ctx := context.Background()

	observer.RecordCutoverPlanState(ctx, CutoverPlanStateEvent{
		Status: "active",
		Count:  2,
	})
	observer.RecordCutoverItemState(ctx, CutoverItemStateEvent{
		Status: "queued",
		Count:  7,
	})
	observer.RecordProviderProbe(ctx, ProviderProbeEvent{
		Mode:     "scheduler",
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Result:   "failure",
	})
	observer.RecordCutoverWaveDispatch(ctx, CutoverWaveDispatchEvent{
		Result:     "ok",
		Dispatched: 3,
	})

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_embedding_cutover_plans{status="active"} 2`,
		`stele_embedding_cutover_items{status="queued"} 7`,
		`stele_embedding_provider_probe_total{mode="scheduler",model="text-embedding-3-small",provider="openai",result="failure"} 1`,
		`stele_embedding_cutover_wave_dispatch_total{result="ok"} 1`,
		`stele_embedding_cutover_wave_dispatched_total{result="ok"} 3`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
}

func TestMetricsObserverExportsInsightFeedbackSignalsWithoutHighCardinalityLabels(t *testing.T) {
	observer := NewMetricsObserver()
	ctx := context.Background()

	observer.RecordInsightFeedback(ctx, InsightFeedbackEvent{
		Operation:    "create",
		Result:       "ok",
		FeedbackType: "noisy",
		InsightType:  "failure_pattern",
		Decision:     "none",
	})
	observer.RecordInsightFeedback(ctx, InsightFeedbackEvent{
		Operation:    "policy",
		Result:       "ok",
		FeedbackType: "noisy",
		InsightType:  "failure_pattern",
		Decision:     "suppress",
	})

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_insight_feedback_total{decision="none",feedback_type="noisy",insight_type="failure_pattern",operation="create",result="ok"} 1`,
		`stele_insight_feedback_total{decision="suppress",feedback_type="noisy",insight_type="failure_pattern",operation="policy",result="ok"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
	for _, forbidden := range []string{"tenant", "project", "namespace", "actor", "reason", "insight_id"} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("metrics contain high-cardinality label %q\n%s", forbidden, metrics)
		}
	}
}

func TestMetricsObserverExportsUsefulnessFeedbackSignalsWithoutHighCardinalityLabels(t *testing.T) {
	observer := NewMetricsObserver()
	ctx := context.Background()

	observer.RecordUsefulnessFeedback(ctx, UsefulnessFeedbackEvent{
		Operation:     "create",
		Result:        "ok",
		FeedbackType:  "noisy",
		SubjectKind:   "memory",
		SourceSurface: "search",
		Decision:      "active",
	})
	observer.RecordUsefulnessFeedback(ctx, UsefulnessFeedbackEvent{
		Operation:     "supersede",
		Result:        "ok",
		FeedbackType:  "unknown",
		SubjectKind:   "unknown",
		SourceSurface: "admin",
		Decision:      "superseded",
	})

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_usefulness_feedback_total{decision="active",feedback_type="noisy",operation="create",result="ok",source_surface="search",subject_kind="memory"} 1`,
		`stele_usefulness_feedback_total{decision="superseded",feedback_type="unknown",operation="supersede",result="ok",source_surface="admin",subject_kind="unknown"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
	for _, forbidden := range []string{"tenant", "project", "namespace", "memory_id", "event_id", "session_id", "feedback_id", "actor", "reason_text"} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("metrics contain high-cardinality label %q\n%s", forbidden, metrics)
		}
	}
}

func TestMetricsObserverExportsTaskEvaluationAndRankingRolloutSignalsWithoutHighCardinalityLabels(t *testing.T) {
	observer := NewMetricsObserver()
	ctx := context.Background()

	observer.RecordTaskEvaluation(ctx, TaskEvaluationEvent{
		Operation:            "create",
		Result:               "ok",
		Verdict:              "failed",
		ContributionCategory: "memory_missing",
		CorrectionState:      "active",
	})
	observer.RecordRankingRollout(ctx, RankingRolloutEvent{
		Operation:       "dry_run",
		Result:          "ok",
		Surface:         "search",
		SignalSource:    "task_evaluations",
		ThresholdStatus: "satisfied",
		PolicyStatus:    "dry_run",
		ReasonCode:      "subject_boosted",
	})

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_task_evaluation_total{contribution_category="memory_missing",correction_state="active",operation="create",result="ok",verdict="failed"} 1`,
		`stele_ranking_rollout_total{operation="dry_run",policy_status="dry_run",reason_code="subject_boosted",result="ok",signal_source="task_evaluations",surface="search",threshold_status="satisfied"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
	for _, forbidden := range []string{"tenant", "project", "namespace", "memory_id", "event_id", "session_id", "task_id", "policy_id", "query", "actor", "reason_text"} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("metrics contain high-cardinality label %q\n%s", forbidden, metrics)
		}
	}
}

func TestMetricsObserverExportsDerivedInsightReplaySignalsWithoutHighCardinalityLabels(t *testing.T) {
	observer := NewMetricsObserver()
	ctx := context.Background()

	observer.RecordDerivedInsightReplay(ctx, DerivedInsightReplayEvent{
		Mode:        "dry_run",
		Result:      "completed",
		InsightType: "failure_pattern",
		Decision:    "create",
		Reason:      "repeated_evidence",
		Count:       2,
	})
	observer.RecordDerivedInsightReplay(ctx, DerivedInsightReplayEvent{
		Mode:        "apply",
		Result:      "continuation_required",
		InsightType: "lesson",
		Decision:    "skip",
		Reason:      "insufficient_evidence",
		Count:       1,
	})

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_derived_insight_replay_total{decision="create",insight_type="failure_pattern",mode="dry_run",reason="repeated_evidence",result="completed"} 2`,
		`stele_derived_insight_replay_total{decision="skip",insight_type="lesson",mode="apply",reason="insufficient_evidence",result="continuation_required"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
	for _, forbidden := range []string{"tenant", "project", "namespace", "replay_run_id", "insight_id", "actor", "reason_text"} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("metrics contain high-cardinality label %q\n%s", forbidden, metrics)
		}
	}
}

func TestMetricsObserverExportsScopeProofAndSessionSignalsWithoutHighCardinalityLabels(t *testing.T) {
	observer := NewMetricsObserver()
	ctx := context.Background()

	observer.RecordScopeProofRun(ctx, ScopeProofRunEvent{
		Status:          "completed",
		Verdict:         "passed_degraded",
		FailureCategory: "context",
	})
	observer.RecordScopeProofStep(ctx, ScopeProofStepEvent{
		Step:            "context_assembled",
		Status:          "completed",
		Verdict:         "passed_degraded",
		Component:       "context",
		FailureCategory: "context",
	})
	observer.RecordMemorySessionRun(ctx, MemorySessionRunEvent{
		Status:          "completed",
		Verdict:         "passed_degraded",
		FailureCategory: "retrieval",
	})
	observer.RecordMemorySessionTurn(ctx, MemorySessionTurnEvent{
		Status:             "verified",
		VerificationStatus: "passed_degraded",
		FailureCategory:    "retrieval",
	})
	observer.RecordMemorySessionVerification(ctx, MemorySessionVerificationEvent{
		Status:          "completed",
		Verdict:         "passed_degraded",
		FailureCategory: "retrieval",
	})

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_scope_proof_runs_total{failure_category="context",status="completed",verdict="passed_degraded"} 1`,
		`stele_scope_proof_steps_total{component="context",failure_category="context",status="completed",step="context_assembled",verdict="passed_degraded"} 1`,
		`stele_memory_session_runs_total{failure_category="retrieval",status="completed",verdict="passed_degraded"} 1`,
		`stele_memory_session_turns_total{failure_category="retrieval",status="verified",verification_status="passed_degraded"} 1`,
		`stele_memory_session_verifications_total{failure_category="retrieval",status="completed",verdict="passed_degraded"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
	for _, forbidden := range []string{"tenant", "project", "namespace", "proof_id", "session_id", "turn_id", "event_id", "memory_id", "actor", "reason_text"} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("metrics contain high-cardinality label %q\n%s", forbidden, metrics)
		}
	}
}
