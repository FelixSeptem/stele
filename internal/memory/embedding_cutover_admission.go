package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/diagnostics"
)

const embeddingCutoverAdmissionComponent = "embedding_cutover"

type EmbeddingCutoverPreflightInput struct {
	Scope      Scope     `json:"scope"`
	PlanID     string    `json:"plan_id"`
	ObservedAt time.Time `json:"observed_at"`
}

func (i EmbeddingCutoverPreflightInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case strings.TrimSpace(i.PlanID) == "":
		return fmt.Errorf("cutover plan id is required")
	default:
		return nil
	}
}

type EmbeddingCutoverPlanSummary struct {
	ID     string                     `json:"id"`
	Status EmbeddingCutoverPlanStatus `json:"status"`
}

type EmbeddingCutoverClassBreakdown struct {
	Class               MemoryClass `json:"class"`
	Eligible            int         `json:"eligible"`
	Drifted             int         `json:"drifted"`
	MissingActiveVector int         `json:"missing_active_vector"`
	MissingRoute        int         `json:"missing_route"`
}

type EmbeddingCutoverAdmissionSnapshot struct {
	Plan            EmbeddingCutoverPlan             `json:"plan"`
	EligibleTotal   int                              `json:"eligible_total"`
	ClassBreakdown  []EmbeddingCutoverClassBreakdown `json:"class_breakdown,omitempty"`
	ConflictingPlan *EmbeddingCutoverPlanSummary     `json:"conflicting_plan,omitempty"`
}

type EmbeddingCutoverPreflightReport struct {
	Component       string                           `json:"component"`
	Decision        diagnostics.AdmissionDecision    `json:"decision"`
	Blockers        []diagnostics.Finding            `json:"blockers,omitempty"`
	Warnings        []diagnostics.Finding            `json:"warnings,omitempty"`
	Target          EmbeddingCutoverTarget           `json:"target"`
	Scope           Scope                            `json:"scope"`
	EligibleTotal   int                              `json:"eligible_total"`
	ClassBreakdown  []EmbeddingCutoverClassBreakdown `json:"class_breakdown,omitempty"`
	ConflictingPlan *EmbeddingCutoverPlanSummary     `json:"conflicting_plan,omitempty"`
	ObservedAt      time.Time                        `json:"observed_at"`
}

type EmbeddingCutoverAdmissionError struct {
	Report EmbeddingCutoverPreflightReport
}

func (e EmbeddingCutoverAdmissionError) Error() string {
	return ErrEmbeddingCutoverRejected.Error() + ": cutover admission denied"
}

func (e EmbeddingCutoverAdmissionError) Unwrap() error {
	return ErrEmbeddingCutoverRejected
}

func EvaluateEmbeddingCutoverAdmission(runtime EmbeddingRuntimeStatus, snapshot EmbeddingCutoverAdmissionSnapshot, observedAt time.Time) EmbeddingCutoverPreflightReport {
	findings := make([]diagnostics.Finding, 0)
	plan := snapshot.Plan

	if plan.Status != EmbeddingCutoverPlanStatusDraft && plan.Status != EmbeddingCutoverPlanStatusPaused {
		findings = append(findings, diagnostics.Finding{
			Severity: diagnostics.SeverityBlocker,
			Code:     "plan_not_activatable",
			Message:  fmt.Sprintf("cutover plan status %q cannot be activated", plan.Status),
		})
	}
	if err := validateEmbeddingCutoverRuntimeSupport(runtime, plan.Target); err != nil {
		findings = append(findings, diagnostics.Finding{
			Severity: diagnostics.SeverityBlocker,
			Code:     "target_unresolved",
			Message:  err.Error(),
		})
	}
	if snapshot.ConflictingPlan != nil {
		findings = append(findings, diagnostics.Finding{
			Severity: diagnostics.SeverityBlocker,
			Code:     "scoped_plan_conflict",
			Message:  "scope already has an active or paused embedding cutover plan",
			Metadata: map[string]string{"conflict_status": string(snapshot.ConflictingPlan.Status)},
		})
	}
	if snapshot.EligibleTotal == 0 {
		findings = append(findings, diagnostics.Finding{
			Severity: diagnostics.SeverityBlocker,
			Code:     "zero_eligible_memory",
			Message:  "cutover plan has no eligible memories",
		})
	}

	var drifted, missingActive, missingRoute int
	for _, item := range snapshot.ClassBreakdown {
		drifted += item.Drifted
		missingActive += item.MissingActiveVector
		missingRoute += item.MissingRoute
	}
	if missingRoute > 0 {
		findings = append(findings, diagnostics.Finding{
			Severity: diagnostics.SeverityBlocker,
			Code:     "unsupported_class_route",
			Message:  "one or more class routes cannot satisfy the cutover target",
		})
	}
	if drifted > 0 {
		findings = append(findings, diagnostics.Finding{
			Severity: diagnostics.SeverityWarning,
			Code:     "semantic_drift",
			Message:  "one or more memories currently differ from the cutover target",
		})
	}
	if missingActive > 0 {
		findings = append(findings, diagnostics.Finding{
			Severity: diagnostics.SeverityWarning,
			Code:     "missing_active_vector",
			Message:  "one or more memories currently lack an active vector revision",
		})
	}
	if plan.WaveSize > 0 && snapshot.EligibleTotal > plan.WaveSize {
		findings = append(findings, diagnostics.Finding{
			Severity: diagnostics.SeverityWarning,
			Code:     "many_waves",
			Message:  "cutover will require multiple scheduler waves",
		})
	}

	admission := diagnostics.NewAdmissionReport(embeddingCutoverAdmissionComponent, observedAt, findings...)
	return EmbeddingCutoverPreflightReport{
		Component:       admission.Component,
		Decision:        admission.Decision,
		Blockers:        admission.Blockers,
		Warnings:        admission.Warnings,
		Target:          plan.Target,
		Scope:           plan.Scope,
		EligibleTotal:   snapshot.EligibleTotal,
		ClassBreakdown:  append([]EmbeddingCutoverClassBreakdown(nil), snapshot.ClassBreakdown...),
		ConflictingPlan: snapshot.ConflictingPlan,
		ObservedAt:      admission.ObservedAt,
	}
}

func (s *EmbeddingAdminQueryService) PreflightEmbeddingCutoverPlan(ctx context.Context, input EmbeddingCutoverPreflightInput) (EmbeddingCutoverPreflightReport, error) {
	if err := input.Validate(); err != nil {
		return EmbeddingCutoverPreflightReport{}, err
	}
	if s.store == nil {
		return EmbeddingCutoverPreflightReport{}, fmt.Errorf("embedding admin store is not configured")
	}
	snapshot, err := s.store.ReadEmbeddingCutoverAdmission(ctx, input)
	if err != nil {
		return EmbeddingCutoverPreflightReport{}, err
	}
	return EvaluateEmbeddingCutoverAdmission(s.runtime, snapshot, input.ObservedAt), nil
}
