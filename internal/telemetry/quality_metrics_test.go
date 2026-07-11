package telemetry

import (
	"context"
	"strings"
	"testing"
)

func TestMetricsObserverExportsQualityRepairSignalsWithoutHighCardinalityLabels(t *testing.T) {
	observer := NewMetricsObserver()
	ctx := context.Background()

	observer.RecordQualityEvaluation(ctx, QualityEvaluationEvent{
		Status:          "completed",
		CheckCategory:   "retrieval",
		FindingCategory: "semantic_projection",
		Severity:        "warning",
		Component:       "embedding",
	})
	observer.RecordRepairAction(ctx, RepairActionEvent{
		ActionCategory: "embedding_retry",
		Result:         "completed",
		ReasonCategory: "semantic_projection_degraded",
	})
	observer.RecordRepairVerification(ctx, RepairVerificationEvent{
		Status:                  "manual_review",
		ResidualFindingCategory: "semantic_projection",
	})

	metrics := observer.RenderPrometheus()
	for _, want := range []string{
		`stele_quality_evaluation_total{check_category="retrieval",component="embedding",finding_category="semantic_projection",severity="warning",status="completed"} 1`,
		`stele_quality_repair_actions_total{action_category="embedding_retry",reason_category="semantic_projection_degraded",result="completed"} 1`,
		`stele_quality_repair_verification_total{residual_finding_category="semantic_projection",status="manual_review"} 1`,
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q\n%s", want, metrics)
		}
	}
	for _, forbidden := range []string{`tenant="`, `project="`, `namespace="`, `memory_id="`, `event_id="`, `repair_plan_id="`, `actor="`, `reason_text="`} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("metrics contain high-cardinality label %q\n%s", forbidden, metrics)
		}
	}
}
