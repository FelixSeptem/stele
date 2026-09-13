package jobs

import (
	"context"
	"github.com/FelixSeptem/stele/internal/memory"
	"testing"
	"time"
)

func TestMaintenanceRetentionAllowlistExcludesCanonicalAndIncidentRecords(t *testing.T) {
	for _, category := range []string{"job_execution", "projection_evidence", "conformance_evidence", "redacted_trajectory"} {
		if !IsDerivedMaintenanceRetentionCategory(category) {
			t.Fatalf("category %q should be allowed", category)
		}
	}
	for _, category := range []string{"canonical_memory", "raw_event", "memory_version", "incident_audit"} {
		if IsDerivedMaintenanceRetentionCategory(category) {
			t.Fatalf("category %q must not be allowed", category)
		}
	}
}

type derivedRetentionStub struct {
	projection, conformance int
	cutoff                  time.Time
	limit                   int
}

func (s *derivedRetentionStub) DeleteContextProjectionEvidenceBefore(_ context.Context, _ memory.Scope, cutoff time.Time, limit int) (int, error) {
	s.projection++
	s.cutoff = cutoff
	s.limit = limit
	return 2, nil
}
func (s *derivedRetentionStub) DeleteConformanceEvidenceBefore(_ context.Context, _ memory.Scope, _ time.Time, _ int) (int, error) {
	s.conformance++
	return 3, nil
}

func TestDerivedArtifactRetentionJobDeletesOnlyBoundedDerivedArtifacts(t *testing.T) {
	s := &derivedRetentionStub{}
	now := time.Unix(1000, 0).UTC()
	job := DerivedArtifactRetentionJob{Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Store: s, Now: func() time.Time { return now }, RetentionWindow: time.Hour, Limit: 7}
	deleted, err := job.Run(context.Background())
	if err != nil || deleted != 5 {
		t.Fatalf("deleted=%d err=%v", deleted, err)
	}
	if s.projection != 1 || s.conformance != 1 || s.limit != 7 || !s.cutoff.Equal(now.Add(-time.Hour)) {
		t.Fatalf("stub=%+v", s)
	}
}

func TestMaintenanceRetentionCategoriesAreBoundedAndDeterministic(t *testing.T) {
	if got := NormalizeMaintenanceRetentionCategories([]string{"job_execution", "job_execution", "canonical_memory", "projection_evidence"}); len(got) != 2 || got[0] != "job_execution" || got[1] != "projection_evidence" {
		t.Fatalf("normalized categories = %v, want sorted unique allowlist", got)
	}
	if got := NormalizeMaintenanceRetentionCategories([]string{"canonical_memory", "incident_audit"}); len(got) != 0 {
		t.Fatalf("unsafe categories = %v, want empty", got)
	}
}
