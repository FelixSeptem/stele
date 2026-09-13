package jobs

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

func IsDerivedMaintenanceRetentionCategory(category string) bool {
	switch category {
	case "job_execution", "projection_evidence", "conformance_evidence", "redacted_trajectory":
		return true
	default:
		return false
	}
}

func NormalizeMaintenanceRetentionCategories(categories []string) []string {
	seen := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		if IsDerivedMaintenanceRetentionCategory(category) {
			seen[category] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for category := range seen {
		result = append(result, category)
	}
	sort.Strings(result)
	return result
}

type DerivedArtifactRetentionStore interface {
	DeleteContextProjectionEvidenceBefore(context.Context, memory.Scope, time.Time, int) (int, error)
	DeleteConformanceEvidenceBefore(context.Context, memory.Scope, time.Time, int) (int, error)
}

type DerivedArtifactRetentionJob struct {
	Scope           memory.Scope
	Store           DerivedArtifactRetentionStore
	Now             func() time.Time
	RetentionWindow time.Duration
	Limit           int
	Observer        telemetry.Observer
}

func (j DerivedArtifactRetentionJob) Name() string { return "derived_artifact_retention" }

func (j DerivedArtifactRetentionJob) Run(ctx context.Context) (int, error) {
	if err := j.Scope.Validate(); err != nil {
		return 0, err
	}
	if j.Store == nil {
		return 0, fmt.Errorf("derived artifact retention store is required")
	}
	now := time.Now().UTC()
	if j.Now != nil {
		now = j.Now().UTC()
	}
	window := j.RetentionWindow
	if window <= 0 {
		window = 7 * 24 * time.Hour
	}
	limit := j.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	deleted, err := j.Store.DeleteContextProjectionEvidenceBefore(ctx, j.Scope, now.Add(-window), limit)
	if err != nil {
		j.recordTelemetry(ctx, "failure")
		return deleted, err
	}
	count, err := j.Store.DeleteConformanceEvidenceBefore(ctx, j.Scope, now.Add(-window), limit)
	if err != nil {
		j.recordTelemetry(ctx, "failure")
		return deleted + count, err
	}
	j.recordTelemetry(ctx, "success")
	return deleted + count, nil
}

func (j DerivedArtifactRetentionJob) recordTelemetry(ctx context.Context, outcome string) {
	if observer, ok := j.Observer.(interface {
		RecordMaintenance(context.Context, telemetry.MaintenanceEvent)
	}); ok {
		observer.RecordMaintenance(ctx, telemetry.MaintenanceEvent{JobClass: "retention", Outcome: outcome, LeaseOutcome: "none", Freshness: "unknown", SLO: "unknown", LatencyBucket: "unknown", CandidateBucket: "unknown"})
	}
}
