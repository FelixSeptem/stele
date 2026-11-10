package retrieval

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// Costs and verdicts are supplied independently by an external harness. Nil
// always means unobserved, including when other metrics were observed as zero.
type ResidentExternalMetrics struct {
	Observed          bool   `json:"observed"`
	TaskSucceeded     *bool  `json:"task_succeeded,omitempty"`
	RepeatedErrors    *int   `json:"repeated_errors,omitempty"`
	StaleEvidenceHarm *int   `json:"stale_evidence_harm,omitempty"`
	MaintenanceCost   *int64 `json:"maintenance_cost,omitempty"`
	ToolCalls         *int   `json:"tool_calls,omitempty"`
	ModelTokens       *int64 `json:"model_tokens,omitempty"`
}

type ResidentExternalDelta struct {
	TaskSucceeded     *int   `json:"task_succeeded,omitempty"`
	RepeatedErrors    *int   `json:"repeated_errors,omitempty"`
	StaleEvidenceHarm *int   `json:"stale_evidence_harm,omitempty"`
	MaintenanceCost   *int64 `json:"maintenance_cost,omitempty"`
	ToolCalls         *int   `json:"tool_calls,omitempty"`
	ModelTokens       *int64 `json:"model_tokens,omitempty"`
}

type ResidentConditionObservation struct {
	Condition         string `json:"condition"`
	SourceDigest      string `json:"source_digest"`
	OutputDigest      string `json:"output_digest"`
	Actor             string `json:"actor"`
	Stage             int    `json:"stage"`
	TaskSucceeded     *bool  `json:"task_succeeded,omitempty"`
	RepeatedErrors    *int   `json:"repeated_errors,omitempty"`
	StaleEvidenceHarm *int   `json:"stale_evidence_harm,omitempty"`
	MaintenanceCost   *int64 `json:"maintenance_cost,omitempty"`
	ToolCalls         *int   `json:"tool_calls,omitempty"`
	ModelTokens       *int64 `json:"model_tokens,omitempty"`
}

// CompareResidentOutcomes attaches subsequent condition observations. Promotion
// utility is earlier evidence: it cannot be recycled as a comparison outcome.
// The report is observational and never serves as a release/activation ticket.
func CompareResidentOutcomes(result ResidentEvaluationResult, events []memory.RawEvent, observedAt time.Time) (ResidentEvaluationReport, error) {
	if err := result.Report.Validate(); err != nil {
		return ResidentEvaluationReport{}, err
	}
	if observedAt.IsZero() || observedAt.Before(result.evaluatedAt) || len(events) > 2 {
		return ResidentEvaluationReport{}, fmt.Errorf("resident comparison bounds are invalid")
	}
	if err := residentEventBounds(events); err != nil {
		return ResidentEvaluationReport{}, err
	}
	report := result.Report
	report.BaselineOutcome = ResidentExternalMetrics{}
	report.CandidateOutcome = ResidentExternalMetrics{}
	seen := map[string]bool{}
	for _, event := range events {
		var body ResidentConditionObservation
		if !decodeResidentObservation(event.Content, &body) || !residentRecordSafe(event, result.scope, observedAt) || event.EventType != "external_evaluation" || !event.SourceTimestamp.After(result.evaluatedAt) || !residentID(body.Actor) || body.Stage <= result.latestStage || body.Stage > 1_000_000 || body.SourceDigest != report.SourceDigest || body.OutputDigest != report.OutputDigest || (body.Condition != "baseline" && body.Condition != "candidate") || seen[body.Condition] {
			return ResidentEvaluationReport{}, fmt.Errorf("resident comparison observation is incompatible")
		}
		seen[body.Condition] = true
		metric := ResidentExternalMetrics{Observed: true, TaskSucceeded: body.TaskSucceeded, RepeatedErrors: body.RepeatedErrors, StaleEvidenceHarm: body.StaleEvidenceHarm, MaintenanceCost: body.MaintenanceCost, ToolCalls: body.ToolCalls, ModelTokens: body.ModelTokens}
		if !metric.valid() {
			return ResidentEvaluationReport{}, fmt.Errorf("resident external metrics are invalid")
		}
		if body.Condition == "baseline" {
			report.BaselineOutcome = metric
		} else {
			report.CandidateOutcome = metric
		}
	}
	report.Delta = residentExternalDelta(report.BaselineOutcome, report.CandidateOutcome)
	report.OutcomeStatus = residentOutcomeStatus(report)
	ordered := append([]memory.RawEvent{}, events...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	var err error
	report.OutcomeDigest, err = residentDigest(struct {
		At     time.Time
		Events []memory.RawEvent
	}{observedAt, ordered})
	if err != nil {
		return ResidentEvaluationReport{}, err
	}
	report.RunHash = ""
	report.RunHash, err = residentDigest(report)
	if err != nil {
		return ResidentEvaluationReport{}, err
	}
	return report, report.Validate()
}

func (m ResidentExternalMetrics) valid() bool {
	if !m.Observed && (m.TaskSucceeded != nil || m.RepeatedErrors != nil || m.StaleEvidenceHarm != nil || m.MaintenanceCost != nil || m.ToolCalls != nil || m.ModelTokens != nil) {
		return false
	}
	for _, v := range []*int{m.RepeatedErrors, m.StaleEvidenceHarm, m.ToolCalls} {
		if v != nil && (*v < 0 || *v > 1_000_000_000) {
			return false
		}
	}
	for _, v := range []*int64{m.MaintenanceCost, m.ModelTokens} {
		if v != nil && (*v < 0 || *v > 1_000_000_000_000) {
			return false
		}
	}
	return true
}

func residentExternalDelta(b, c ResidentExternalMetrics) ResidentExternalDelta {
	d := ResidentExternalDelta{}
	if b.TaskSucceeded != nil && c.TaskSucceeded != nil {
		v := 0
		if *c.TaskSucceeded {
			v++
		}
		if *b.TaskSucceeded {
			v--
		}
		d.TaskSucceeded = &v
	}
	d.RepeatedErrors = residentIntDelta(b.RepeatedErrors, c.RepeatedErrors)
	d.StaleEvidenceHarm = residentIntDelta(b.StaleEvidenceHarm, c.StaleEvidenceHarm)
	d.ToolCalls = residentIntDelta(b.ToolCalls, c.ToolCalls)
	d.MaintenanceCost = residentCostDelta(b.MaintenanceCost, c.MaintenanceCost)
	d.ModelTokens = residentCostDelta(b.ModelTokens, c.ModelTokens)
	return d
}
func residentIntDelta(b, c *int) *int {
	if b == nil || c == nil {
		return nil
	}
	v := *c - *b
	return &v
}
func residentCostDelta(b, c *int64) *int64 {
	if b == nil || c == nil {
		return nil
	}
	v := *c - *b
	return &v
}
func residentOutcomeStatus(r ResidentEvaluationReport) string {
	if r.BaselineOutcome.TaskSucceeded != nil && r.CandidateOutcome.TaskSucceeded != nil {
		return "observational"
	}
	return "inconclusive"
}
func residentReportDigest(s string) bool {
	_, err := hex.DecodeString(s)
	return len(s) == 64 && strings.ToLower(s) == s && err == nil
}

// Validate rejects altered or internally inconsistent aggregates. SHA-256 is
// an integrity identifier, not authentication of an external judge or report.
func (r ResidentEvaluationReport) Validate() error {
	if r.Schema != "resident-evaluation-v1" || r.Status != "shadow" || !r.RollbackTested || r.Limits.validate() != nil || !residentReportDigest(r.SourceDigest) || !residentReportDigest(r.PolicyDigest) || !residentReportDigest(r.OutputDigest) || !residentReportDigest(r.RunHash) || (r.OutcomeDigest != "" && !residentReportDigest(r.OutcomeDigest)) {
		return fmt.Errorf("resident report identity is invalid")
	}
	for _, m := range []ResidentPlanMetrics{r.Baseline, r.Candidate} {
		if m.Items < 0 || m.Items > r.Limits.MaxItems || m.ContentBytes < 0 || m.ContentBytes > r.Limits.MaxContentBytes || m.Tokens < 0 || m.Tokens > r.Limits.MaxTokens || m.Citations != m.Items || m.Citations > r.Limits.MaxCitations || (m.Items == 0 && (m.ContentBytes != 0 || m.Tokens != 0)) {
			return fmt.Errorf("resident report plan metrics are invalid")
		}
	}
	if r.Promoted < 0 || r.Retained < 0 || r.Demoted < 0 || r.Demoted > residentMaxSources || r.Promoted+r.Retained != r.Candidate.Items {
		return fmt.Errorf("resident report selection is invalid")
	}
	total := 0
	for reason, count := range r.Omissions {
		switch reason {
		case ResidentOmissionSafety, ResidentOmissionPolicy, ResidentOmissionSupport, ResidentOmissionUtility, ResidentOmissionBudget:
		default:
			return fmt.Errorf("resident report omission is invalid")
		}
		if count < 1 || count > residentMaxSources {
			return fmt.Errorf("resident report omission count is invalid")
		}
		total += count
	}
	if total > residentMaxSources {
		return fmt.Errorf("resident report omission total is invalid")
	}
	if !r.BaselineOutcome.valid() || !r.CandidateOutcome.valid() || r.OutcomeStatus != residentOutcomeStatus(r) || !reflect.DeepEqual(r.Delta, residentExternalDelta(r.BaselineOutcome, r.CandidateOutcome)) || ((r.BaselineOutcome.Observed || r.CandidateOutcome.Observed) && r.OutcomeDigest == "") {
		return fmt.Errorf("resident report outcomes are invalid")
	}
	expected := r.RunHash
	r.RunHash = ""
	actual, err := residentDigest(r)
	if err != nil || actual != expected {
		return fmt.Errorf("resident report integrity mismatch")
	}
	return nil
}
