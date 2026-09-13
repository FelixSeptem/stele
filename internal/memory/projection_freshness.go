package memory

import "time"

type ProjectionFreshnessCategory string

const (
	ProjectionFreshnessFresh     ProjectionFreshnessCategory = "fresh"
	ProjectionFreshnessStale     ProjectionFreshnessCategory = "stale"
	ProjectionFreshnessMissing   ProjectionFreshnessCategory = "missing"
	ProjectionFreshnessDivergent ProjectionFreshnessCategory = "divergent"
	ProjectionFreshnessForeign   ProjectionFreshnessCategory = "foreign_scope"
	ProjectionFreshnessHidden    ProjectionFreshnessCategory = "lifecycle_hidden"
)

func (c ProjectionFreshnessCategory) Valid() bool {
	switch c {
	case ProjectionFreshnessFresh, ProjectionFreshnessStale, ProjectionFreshnessMissing, ProjectionFreshnessDivergent, ProjectionFreshnessForeign, ProjectionFreshnessHidden:
		return true
	default:
		return false
	}
}

type ProjectionSLOBucket string

const (
	ProjectionSLOWithinBudget ProjectionSLOBucket = "within_budget"
	ProjectionSLOOverBudget   ProjectionSLOBucket = "over_budget"
	ProjectionSLOUnknown      ProjectionSLOBucket = "unknown"
)

type ProjectionFreshnessEvidence struct {
	Scope               Scope
	SourceWatermark     string
	ProjectionWatermark string
	PolicyVersion       string
	RendererVersion     string
	Category            ProjectionFreshnessCategory
	SLO                 ProjectionSLOBucket
	Age                 time.Duration
	Duration            time.Duration
	Eligible            bool
	RebuildRequired     bool
}

func EvaluateProjectionFreshness(scope Scope, sourceWatermark, projectionWatermark, policyVersion, rendererVersion string, age, duration, freshnessWindow, maxDuration time.Duration, lifecycleVisible bool, evidenceScope Scope) (ProjectionFreshnessEvidence, error) {
	if err := scope.Validate(); err != nil {
		return ProjectionFreshnessEvidence{}, err
	}
	e := ProjectionFreshnessEvidence{Scope: scope.Normalized(), SourceWatermark: sourceWatermark, ProjectionWatermark: projectionWatermark, PolicyVersion: policyVersion, RendererVersion: rendererVersion, Age: age, Duration: duration, Eligible: true}
	if evidenceScope.Normalized() != scope.Normalized() {
		e.Category, e.Eligible = ProjectionFreshnessForeign, false
	} else if !lifecycleVisible {
		e.Category, e.Eligible = ProjectionFreshnessHidden, false
	} else if sourceWatermark == "" || projectionWatermark == "" {
		e.Category, e.Eligible, e.RebuildRequired = ProjectionFreshnessMissing, false, true
	} else if sourceWatermark != projectionWatermark {
		e.Category, e.Eligible, e.RebuildRequired = ProjectionFreshnessDivergent, false, true
	} else if freshnessWindow <= 0 || age > freshnessWindow {
		e.Category, e.Eligible, e.RebuildRequired = ProjectionFreshnessStale, false, true
	} else {
		e.Category = ProjectionFreshnessFresh
	}
	if maxDuration <= 0 {
		e.SLO = ProjectionSLOUnknown
	} else if duration <= maxDuration {
		e.SLO = ProjectionSLOWithinBudget
	} else {
		e.SLO, e.Eligible = ProjectionSLOOverBudget, false
	}
	return e, nil
}
