package benchmark

import (
	"encoding/json"
	"fmt"
	"math"
)

type TaskSeriesCostReport struct {
	ObservedStages int      `json:"observed_stages"`
	Coverage       float64  `json:"coverage"`
	Total          int64    `json:"total"`
	Average        *float64 `json:"average"`
}

type TaskSeriesGateCounts struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Missing int `json:"missing"`
}

type TaskSeriesV3ConditionReport struct {
	ModelTokens     TaskSeriesCostReport `json:"model_tokens"`
	ToolCalls       TaskSeriesCostReport `json:"tool_calls"`
	Latency         TaskSeriesGateCounts `json:"latency"`
	Disclosure      TaskSeriesGateCounts `json:"disclosure"`
	StageVisibility TaskSeriesGateCounts `json:"stage_visibility"`
	Rollback        TaskSeriesGateCounts `json:"rollback"`
}

type TaskSeriesV3Report struct {
	ExperimentDigest   string `json:"experiment_digest"`
	InputDigest        string `json:"input_digest"`
	ObservationDigest  string `json:"observation_digest"`
	GateStatus         string `json:"gate_status"`
	ComparisonComplete bool   `json:"comparison_complete"`
}

func taskSeriesValueDigest(v any) string { b, _ := json.Marshal(v); return checksumBytes(b) }

func taskSeriesGate(value *bool) TaskSeriesGateCounts {
	if value == nil {
		return TaskSeriesGateCounts{Missing: 1}
	}
	if *value {
		return TaskSeriesGateCounts{Passed: 1}
	}
	return TaskSeriesGateCounts{Failed: 1}
}
func (g *TaskSeriesGateCounts) add(v TaskSeriesGateCounts) {
	g.Passed += v.Passed
	g.Failed += v.Failed
	g.Missing += v.Missing
}

func taskSeriesCost(values []*int64, current int) TaskSeriesCostReport {
	var r TaskSeriesCostReport
	for _, v := range values {
		if v != nil {
			r.ObservedStages++
			r.Total += *v
		}
	}
	if current > 0 {
		r.Coverage = float64(r.ObservedStages) / float64(current)
	}
	if r.ObservedStages > 0 {
		n := float64(r.Total) / float64(r.ObservedStages)
		r.Average = &n
	}
	return r
}

func evaluateTaskSeriesV3Condition(c TaskSeriesCondition, obs map[string]TaskSeriesStageObservation, current int, l TaskSeriesLimits) *TaskSeriesV3ConditionReport {
	r := &TaskSeriesV3ConditionReport{}
	var tokens, tools []*int64
	for _, s := range c.Stages {
		if !s.Superseded {
			tokens = append(tokens, s.ModelTokens)
			tools = append(tools, s.ToolCalls)
			o := obs[s.ID]
			var latency *bool
			if o.LatencyMicros != nil {
				ok := *o.LatencyMicros >= 0 && *o.LatencyMicros <= int64(l.MaxLatencyMS)*1000
				latency = &ok
			}
			r.Latency.add(taskSeriesGate(latency))
			r.Disclosure.add(taskSeriesGate(o.DisclosureSafe))
			r.StageVisibility.add(taskSeriesGate(o.StageVisible))
			r.Rollback.add(taskSeriesGate(o.RollbackMatched))
		}
	}
	r.ModelTokens = taskSeriesCost(tokens, current)
	r.ToolCalls = taskSeriesCost(tools, current)
	return r
}

func evaluateTaskSeriesV3Report(m TaskSeriesManifest, obs TaskSeriesObservations) *TaskSeriesV3Report {
	r := &TaskSeriesV3Report{ExperimentDigest: taskSeriesValueDigest(m.Experiment), InputDigest: taskSeriesValueDigest(m), ObservationDigest: taskSeriesValueDigest(obs), GateStatus: "passed", ComparisonComplete: true}
	missing, failed := false, false
	for _, c := range m.Conditions {
		current := 0
		for _, s := range c.Stages {
			if !s.Superseded {
				current++
				if s.Verdict == "" || s.Verdict == TaskSeriesVerdictInconclusive || s.ModelTokens == nil || s.ToolCalls == nil || s.RepeatedError == nil || s.MaintenanceCost == nil {
					r.ComparisonComplete = false
				}
			}
		}
		metrics := evaluateTaskSeriesV3Condition(c, obs[c.ID], current, m.Experiment.Limits)
		for _, g := range []TaskSeriesGateCounts{metrics.Latency, metrics.Disclosure, metrics.StageVisibility, metrics.Rollback} {
			failed = failed || g.Failed > 0
			missing = missing || g.Missing > 0
		}
	}
	if failed {
		r.GateStatus = "failed"
	} else if missing {
		r.GateStatus = "inconclusive"
	}
	return r
}

func validateTaskSeriesV3Report(r TaskSeriesReport, v3 bool) error {
	if !v3 {
		if r.V3 != nil {
			return fmt.Errorf("v3 report requires v3 schema")
		}
		for _, c := range r.Conditions {
			if c.V3 != nil {
				return fmt.Errorf("v3 condition metrics require v3 schema")
			}
		}
		return nil
	}
	if r.V3 == nil || !sha256Pattern.MatchString(r.V3.ExperimentDigest) || !sha256Pattern.MatchString(r.V3.InputDigest) || !sha256Pattern.MatchString(r.V3.ObservationDigest) || len(r.Conditions) != 5 {
		return fmt.Errorf("v3 report identities are invalid")
	}
	gateStatus := "passed"
	for _, c := range r.Conditions {
		current := c.StageCount - c.SupersededStages
		if c.V3 == nil || current < 0 || c.StageCount > 64 {
			return fmt.Errorf("v3 condition metrics are missing")
		}
		for _, cost := range []TaskSeriesCostReport{c.V3.ModelTokens, c.V3.ToolCalls} {
			if cost.ObservedStages < 0 || cost.ObservedStages > current || cost.Total < 0 || cost.Total > int64(cost.ObservedStages)*maxTaskSeriesMaintenanceCost || math.IsNaN(cost.Coverage) || math.IsInf(cost.Coverage, 0) {
				return fmt.Errorf("v3 costs exceed bounds")
			}
			expected := 0.0
			if current > 0 {
				expected = float64(cost.ObservedStages) / float64(current)
			}
			if cost.Coverage != expected || (cost.ObservedStages == 0) != (cost.Average == nil) {
				return fmt.Errorf("v3 cost coverage is inconsistent")
			}
			if cost.Average != nil && *cost.Average != float64(cost.Total)/float64(cost.ObservedStages) {
				return fmt.Errorf("v3 average is inconsistent")
			}
		}
		for _, g := range []TaskSeriesGateCounts{c.V3.Latency, c.V3.Disclosure, c.V3.StageVisibility, c.V3.Rollback} {
			if g.Passed < 0 || g.Failed < 0 || g.Missing < 0 || g.Passed+g.Failed+g.Missing != current {
				return fmt.Errorf("v3 gate coverage is inconsistent")
			}
			if g.Failed > 0 {
				gateStatus = "failed"
			} else if g.Missing > 0 && gateStatus != "failed" {
				gateStatus = "inconclusive"
			}
		}
	}
	if r.V3.GateStatus != gateStatus || (r.Status == StatusSuccess && (gateStatus != "passed" || !r.V3.ComparisonComplete || r.SafetyStatus != TaskSeriesSafetyPassed)) {
		return fmt.Errorf("v3 gate status is inconsistent")
	}
	return nil
}
