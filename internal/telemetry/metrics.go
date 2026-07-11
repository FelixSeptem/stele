package telemetry

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/FelixSeptem/stele/internal/diagnostics"
)

type AdmissionEvent struct {
	Component string
	Operation string
	Decision  string
	Blockers  []diagnostics.Finding
	Warnings  []diagnostics.Finding
}

type CutoverPlanStateEvent struct {
	Status string
	Count  int64
}

type CutoverItemStateEvent struct {
	Status string
	Count  int64
}

type ProviderProbeEvent struct {
	Mode     string
	Provider string
	Model    string
	Result   string
}

type CutoverWaveDispatchEvent struct {
	Result     string
	Dispatched int
}

type InsightFeedbackEvent struct {
	Operation    string
	Result       string
	FeedbackType string
	InsightType  string
	Decision     string
}

type DerivedInsightReplayEvent struct {
	Mode        string
	Result      string
	InsightType string
	Decision    string
	Reason      string
	Count       int
}

type QualityEvaluationEvent struct {
	Status          string
	CheckCategory   string
	FindingCategory string
	Severity        string
	Component       string
}

type RepairActionEvent struct {
	ActionCategory string
	Result         string
	ReasonCategory string
}

type RepairVerificationEvent struct {
	Status                  string
	ResidualFindingCategory string
}

type ScopeProofRunEvent struct {
	Status          string
	Verdict         string
	FailureCategory string
}

type ScopeProofStepEvent struct {
	Step            string
	Status          string
	Verdict         string
	Component       string
	FailureCategory string
}

type MemorySessionRunEvent struct {
	Status          string
	Verdict         string
	FailureCategory string
}

type MemorySessionTurnEvent struct {
	Status             string
	VerificationStatus string
	FailureCategory    string
}

type MemorySessionVerificationEvent struct {
	Status          string
	Verdict         string
	FailureCategory string
}

type MetricsObserver struct {
	mu       sync.Mutex
	counters map[string]float64
	gauges   map[string]float64
}

func NewMetricsObserver() *MetricsObserver {
	return &MetricsObserver{
		counters: make(map[string]float64),
		gauges:   make(map[string]float64),
	}
}

func (o *MetricsObserver) RecordOperation(ctx context.Context, event OperationEvent) {
	if o == nil {
		return
	}
	labels := map[string]string{
		"mode":      event.Mode,
		"component": event.Component,
		"operation": event.Operation,
		"status":    event.Status,
	}
	o.addCounter("stele_operations_total", labels, 1)
	o.addCounter("stele_operation_processed_total", labels, float64(event.Count))
}

func (o *MetricsObserver) RecordBacklog(ctx context.Context, event BacklogEvent) {
	if o == nil {
		return
	}
	labels := map[string]string{
		"mode":      event.Mode,
		"component": event.Component,
		"queue":     event.Queue,
		"status":    event.Status,
	}
	o.setGauge("stele_backlog_pending", labels, float64(event.Pending))
	o.setGauge("stele_backlog_leased", labels, float64(event.Leased))
	o.setGauge("stele_backlog_processed", labels, float64(event.Processed))
}

func (o *MetricsObserver) RecordAdmission(ctx context.Context, event AdmissionEvent) {
	if o == nil {
		return
	}
	decisionLabels := map[string]string{
		"component": event.Component,
		"operation": event.Operation,
		"decision":  event.Decision,
	}
	o.addCounter("stele_admission_decisions_total", decisionLabels, 1)
	for _, finding := range event.Blockers {
		o.addCounter("stele_admission_findings_total", map[string]string{
			"component": event.Component,
			"operation": event.Operation,
			"severity":  string(diagnostics.SeverityBlocker),
			"code":      finding.Code,
		}, 1)
	}
	for _, finding := range event.Warnings {
		o.addCounter("stele_admission_findings_total", map[string]string{
			"component": event.Component,
			"operation": event.Operation,
			"severity":  string(diagnostics.SeverityWarning),
			"code":      finding.Code,
		}, 1)
	}
}

func (o *MetricsObserver) RecordCutoverPlanState(ctx context.Context, event CutoverPlanStateEvent) {
	if o == nil {
		return
	}
	o.setGauge("stele_embedding_cutover_plans", map[string]string{
		"status": event.Status,
	}, float64(event.Count))
}

func (o *MetricsObserver) RecordCutoverItemState(ctx context.Context, event CutoverItemStateEvent) {
	if o == nil {
		return
	}
	o.setGauge("stele_embedding_cutover_items", map[string]string{
		"status": event.Status,
	}, float64(event.Count))
}

func (o *MetricsObserver) RecordProviderProbe(ctx context.Context, event ProviderProbeEvent) {
	if o == nil {
		return
	}
	o.addCounter("stele_embedding_provider_probe_total", map[string]string{
		"mode":     event.Mode,
		"provider": event.Provider,
		"model":    event.Model,
		"result":   event.Result,
	}, 1)
}

func (o *MetricsObserver) RecordCutoverWaveDispatch(ctx context.Context, event CutoverWaveDispatchEvent) {
	if o == nil {
		return
	}
	labels := map[string]string{
		"result": event.Result,
	}
	o.addCounter("stele_embedding_cutover_wave_dispatch_total", labels, 1)
	o.addCounter("stele_embedding_cutover_wave_dispatched_total", labels, float64(event.Dispatched))
}

func (o *MetricsObserver) RecordInsightFeedback(ctx context.Context, event InsightFeedbackEvent) {
	if o == nil {
		return
	}
	decision := event.Decision
	if decision == "" {
		decision = "none"
	}
	o.addCounter("stele_insight_feedback_total", map[string]string{
		"operation":     event.Operation,
		"result":        event.Result,
		"feedback_type": event.FeedbackType,
		"insight_type":  event.InsightType,
		"decision":      decision,
	}, 1)
}

func (o *MetricsObserver) RecordDerivedInsightReplay(ctx context.Context, event DerivedInsightReplayEvent) {
	if o == nil {
		return
	}
	count := event.Count
	if count <= 0 {
		count = 1
	}
	o.addCounter("stele_derived_insight_replay_total", map[string]string{
		"mode":         labelOrUnknown(event.Mode),
		"result":       labelOrUnknown(event.Result),
		"insight_type": labelOrUnknown(event.InsightType),
		"decision":     labelOrUnknown(event.Decision),
		"reason":       labelOrUnknown(event.Reason),
	}, float64(count))
}

func (o *MetricsObserver) RecordQualityEvaluation(ctx context.Context, event QualityEvaluationEvent) {
	if o == nil {
		return
	}
	o.addCounter("stele_quality_evaluation_total", map[string]string{
		"status":           labelOrUnknown(event.Status),
		"check_category":   labelOrUnknown(event.CheckCategory),
		"finding_category": labelOrUnknown(event.FindingCategory),
		"severity":         labelOrUnknown(event.Severity),
		"component":        labelOrUnknown(event.Component),
	}, 1)
}

func (o *MetricsObserver) RecordRepairAction(ctx context.Context, event RepairActionEvent) {
	if o == nil {
		return
	}
	o.addCounter("stele_quality_repair_actions_total", map[string]string{
		"action_category": labelOrUnknown(event.ActionCategory),
		"result":          labelOrUnknown(event.Result),
		"reason_category": labelOrUnknown(event.ReasonCategory),
	}, 1)
}

func (o *MetricsObserver) RecordRepairVerification(ctx context.Context, event RepairVerificationEvent) {
	if o == nil {
		return
	}
	o.addCounter("stele_quality_repair_verification_total", map[string]string{
		"status":                    labelOrUnknown(event.Status),
		"residual_finding_category": labelOrUnknown(event.ResidualFindingCategory),
	}, 1)
}

func (o *MetricsObserver) RecordScopeProofRun(ctx context.Context, event ScopeProofRunEvent) {
	if o == nil {
		return
	}
	o.addCounter("stele_scope_proof_runs_total", map[string]string{
		"status":           labelOrUnknown(event.Status),
		"verdict":          labelOrUnknown(event.Verdict),
		"failure_category": labelOrUnknown(event.FailureCategory),
	}, 1)
}

func (o *MetricsObserver) RecordScopeProofStep(ctx context.Context, event ScopeProofStepEvent) {
	if o == nil {
		return
	}
	o.addCounter("stele_scope_proof_steps_total", map[string]string{
		"step":             labelOrUnknown(event.Step),
		"status":           labelOrUnknown(event.Status),
		"verdict":          labelOrUnknown(event.Verdict),
		"component":        labelOrUnknown(event.Component),
		"failure_category": labelOrUnknown(event.FailureCategory),
	}, 1)
}

func (o *MetricsObserver) RecordMemorySessionRun(ctx context.Context, event MemorySessionRunEvent) {
	if o == nil {
		return
	}
	o.addCounter("stele_memory_session_runs_total", map[string]string{
		"status":           labelOrUnknown(event.Status),
		"verdict":          labelOrUnknown(event.Verdict),
		"failure_category": labelOrUnknown(event.FailureCategory),
	}, 1)
}

func (o *MetricsObserver) RecordMemorySessionTurn(ctx context.Context, event MemorySessionTurnEvent) {
	if o == nil {
		return
	}
	o.addCounter("stele_memory_session_turns_total", map[string]string{
		"status":              labelOrUnknown(event.Status),
		"verification_status": labelOrUnknown(event.VerificationStatus),
		"failure_category":    labelOrUnknown(event.FailureCategory),
	}, 1)
}

func (o *MetricsObserver) RecordMemorySessionVerification(ctx context.Context, event MemorySessionVerificationEvent) {
	if o == nil {
		return
	}
	o.addCounter("stele_memory_session_verifications_total", map[string]string{
		"status":           labelOrUnknown(event.Status),
		"verdict":          labelOrUnknown(event.Verdict),
		"failure_category": labelOrUnknown(event.FailureCategory),
	}, 1)
}

func (o *MetricsObserver) RenderPrometheus() string {
	if o == nil {
		return ""
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	var builder strings.Builder
	writeMetricFamilyHeader(&builder, "stele_operations_total", "counter", "Stele operation executions.")
	writeMetricFamilyHeader(&builder, "stele_operation_processed_total", "counter", "Stele processed item totals.")
	writeMetricFamilyHeader(&builder, "stele_backlog_pending", "gauge", "Stele pending backlog count.")
	writeMetricFamilyHeader(&builder, "stele_backlog_leased", "gauge", "Stele leased backlog count.")
	writeMetricFamilyHeader(&builder, "stele_backlog_processed", "gauge", "Stele processed backlog count.")
	writeMetricFamilyHeader(&builder, "stele_admission_decisions_total", "counter", "Stele admission decisions.")
	writeMetricFamilyHeader(&builder, "stele_admission_findings_total", "counter", "Stele admission findings.")
	writeMetricFamilyHeader(&builder, "stele_embedding_cutover_plans", "gauge", "Embedding cutover plan counts by status.")
	writeMetricFamilyHeader(&builder, "stele_embedding_cutover_items", "gauge", "Embedding cutover item counts by status.")
	writeMetricFamilyHeader(&builder, "stele_embedding_provider_probe_total", "counter", "Embedding provider readiness probe results.")
	writeMetricFamilyHeader(&builder, "stele_embedding_cutover_wave_dispatch_total", "counter", "Embedding cutover wave dispatch attempts.")
	writeMetricFamilyHeader(&builder, "stele_embedding_cutover_wave_dispatched_total", "counter", "Embedding cutover items dispatched by scheduler waves.")
	writeMetricFamilyHeader(&builder, "stele_insight_feedback_total", "counter", "Derived insight feedback operations and policy decisions.")
	writeMetricFamilyHeader(&builder, "stele_derived_insight_replay_total", "counter", "Derived insight replay outcomes by low-cardinality categories.")
	writeMetricFamilyHeader(&builder, "stele_quality_evaluation_total", "counter", "Memory quality evaluation outcomes.")
	writeMetricFamilyHeader(&builder, "stele_quality_repair_actions_total", "counter", "Memory quality repair action outcomes.")
	writeMetricFamilyHeader(&builder, "stele_quality_repair_verification_total", "counter", "Memory quality repair verification outcomes.")
	writeMetricFamilyHeader(&builder, "stele_scope_proof_runs_total", "counter", "Scope proof run outcomes by bounded categories.")
	writeMetricFamilyHeader(&builder, "stele_scope_proof_steps_total", "counter", "Scope proof step outcomes by bounded categories.")
	writeMetricFamilyHeader(&builder, "stele_memory_session_runs_total", "counter", "Memory session run outcomes by bounded categories.")
	writeMetricFamilyHeader(&builder, "stele_memory_session_turns_total", "counter", "Memory session turn outcomes by bounded categories.")
	writeMetricFamilyHeader(&builder, "stele_memory_session_verifications_total", "counter", "Memory session verification outcomes by bounded categories.")

	writeMetricMap(&builder, o.counters)
	writeMetricMap(&builder, o.gauges)
	return builder.String()
}

func labelOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

func (o *MetricsObserver) addCounter(name string, labels map[string]string, value float64) {
	if value == 0 {
		return
	}
	key, ok := metricKey(name, labels)
	if !ok {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.counters[key] += value
}

func (o *MetricsObserver) setGauge(name string, labels map[string]string, value float64) {
	key, ok := metricKey(name, labels)
	if !ok {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.gauges[key] = value
}

func metricKey(name string, labels map[string]string) (string, bool) {
	normalized, err := diagnostics.MetricLabels(labels)
	if err != nil {
		return "", false
	}
	keys := make([]string, 0, len(normalized))
	for key := range normalized {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, key, normalized[key]))
	}
	return fmt.Sprintf("%s{%s}", name, strings.Join(parts, ",")), true
}

func writeMetricFamilyHeader(builder *strings.Builder, name, kind, help string) {
	builder.WriteString("# HELP ")
	builder.WriteString(name)
	builder.WriteString(" ")
	builder.WriteString(help)
	builder.WriteString("\n# TYPE ")
	builder.WriteString(name)
	builder.WriteString(" ")
	builder.WriteString(kind)
	builder.WriteString("\n")
}

func writeMetricMap(builder *strings.Builder, values map[string]float64) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString(" ")
		builder.WriteString(formatMetricValue(values[key]))
		builder.WriteString("\n")
	}
}

func formatMetricValue(value float64) string {
	if value == float64(int64(value)) {
		return fmt.Sprintf("%d", int64(value))
	}
	return fmt.Sprintf("%g", value)
}
