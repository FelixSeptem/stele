package diagnostics

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Severity string

const (
	SeverityBlocker Severity = "blocker"
	SeverityWarning Severity = "warning"
)

type AdmissionDecision string

const (
	AdmissionDecisionAllow AdmissionDecision = "allow"
	AdmissionDecisionDeny  AdmissionDecision = "deny"
)

type Finding struct {
	Severity  Severity          `json:"severity"`
	Code      string            `json:"code"`
	Message   string            `json:"message,omitempty"`
	Component string            `json:"component,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type AdmissionReport struct {
	Component  string            `json:"component"`
	Decision   AdmissionDecision `json:"decision"`
	Blockers   []Finding         `json:"blockers,omitempty"`
	Warnings   []Finding         `json:"warnings,omitempty"`
	ObservedAt time.Time         `json:"observed_at"`
}

func NewAdmissionReport(component string, observedAt time.Time, findings ...Finding) AdmissionReport {
	report := AdmissionReport{
		Component:  strings.TrimSpace(component),
		Decision:   AdmissionDecisionAllow,
		ObservedAt: observedAt.UTC(),
	}
	for _, finding := range findings {
		finding.Code = NormalizeMetricValue(finding.Code)
		finding.Component = strings.TrimSpace(finding.Component)
		if finding.Component == "" {
			finding.Component = report.Component
		}
		switch finding.Severity {
		case SeverityBlocker:
			report.Blockers = append(report.Blockers, finding)
		case SeverityWarning:
			report.Warnings = append(report.Warnings, finding)
		}
	}
	if len(report.Blockers) > 0 {
		report.Decision = AdmissionDecisionDeny
	}
	return report
}

type RuntimeMode string

const (
	RuntimeModeAPI       RuntimeMode = "api"
	RuntimeModeWorker    RuntimeMode = "worker"
	RuntimeModeScheduler RuntimeMode = "scheduler"
)

type ReadinessStatus string

const (
	ReadinessStatusReady    ReadinessStatus = "ready"
	ReadinessStatusNotReady ReadinessStatus = "not_ready"
)

type ReadinessCheck struct {
	Name     string
	Required bool
	Check    func(ctx context.Context) error
}

type ReadinessResult struct {
	Mode       RuntimeMode     `json:"mode"`
	Status     ReadinessStatus `json:"status"`
	Findings   []Finding       `json:"findings,omitempty"`
	ObservedAt time.Time       `json:"observed_at"`
}

type ReadinessEvaluator struct {
	Mode       RuntimeMode
	Checks     []ReadinessCheck
	ObservedAt time.Time
}

func (e ReadinessEvaluator) Evaluate(ctx context.Context) ReadinessResult {
	result := ReadinessResult{
		Mode:       e.Mode,
		Status:     ReadinessStatusReady,
		ObservedAt: e.ObservedAt.UTC(),
	}
	for _, check := range e.Checks {
		if check.Check == nil {
			continue
		}
		if err := check.Check(ctx); err != nil {
			severity := SeverityWarning
			if check.Required {
				severity = SeverityBlocker
				result.Status = ReadinessStatusNotReady
			}
			result.Findings = append(result.Findings, Finding{
				Severity:  severity,
				Code:      "check_failed",
				Message:   err.Error(),
				Component: NormalizeMetricValue(check.Name),
			})
		}
	}
	return result
}

var metricValuePattern = regexp.MustCompile(`^[a-zA-Z0-9_:.-]+$`)

var disallowedMetricLabels = map[string]struct{}{
	"memory_id":       {},
	"raw_event_id":    {},
	"cutover_plan_id": {},
	"plan_id":         {},
	"id":              {},
	"error":           {},
	"error_message":   {},
	"failure_reason":  {},
}

func MetricLabels(input map[string]string) (map[string]string, error) {
	labels := make(map[string]string, len(input))
	for key, value := range input {
		normalizedKey := NormalizeMetricValue(key)
		if _, disallowed := disallowedMetricLabels[normalizedKey]; disallowed {
			return nil, fmt.Errorf("metric label %q is not allowed", normalizedKey)
		}
		normalizedValue := NormalizeMetricValue(value)
		if !metricValuePattern.MatchString(normalizedValue) {
			return nil, fmt.Errorf("metric label %q has invalid value %q", normalizedKey, value)
		}
		labels[normalizedKey] = normalizedValue
	}
	return labels, nil
}

func NormalizeMetricValue(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}
