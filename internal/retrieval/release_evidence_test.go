package retrieval

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMarshalRetrievalTrajectoryAllowsOnlyBoundedAggregates(t *testing.T) {
	report := RetrievalTrajectory{
		SchemaVersion: "retrieval-trajectory-v1",
		ReportVersion: "release-report-v1",
		PolicyVersion: "release-policy-v1",
		Channels: []TrajectoryChannelAggregate{{
			Channel: "semantic", Availability: "available", CandidateCountBucket: "11-25",
		}},
		ParentExpansionBucket: "1-5",
		ChildExpansionBucket:  "6-10",
		Dispositions:          []TrajectoryCategoryCount{{Category: "selected", Count: 4}},
		Fallbacks:             []TrajectoryCategoryCount{{Category: "reranker_unavailable", Count: 1}},
		LatencyBucket:         "50-99ms",
	}

	encoded, err := MarshalRetrievalTrajectory(report)
	if err != nil {
		t.Fatalf("MarshalRetrievalTrajectory() error = %v", err)
	}
	for _, forbidden := range []string{"query", "tenant", "project", "namespace", "memory_id", "event_id", "raw_score", "credential", "provider_error"} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			t.Fatalf("trajectory contains forbidden field %q: %s", forbidden, encoded)
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded["latency_bucket"]; got != "50-99ms" {
		t.Fatalf("latency_bucket = %v", got)
	}
}

func TestMarshalRetrievalTrajectoryRejectsUnsafeCategory(t *testing.T) {
	report := RetrievalTrajectory{
		SchemaVersion: "retrieval-trajectory-v1",
		ReportVersion: "release-report-v1",
		PolicyVersion: "release-policy-v1",
		Channels:      []TrajectoryChannelAggregate{{Channel: "semantic", Availability: "available", CandidateCountBucket: "1-5"}},
		Dispositions:  []TrajectoryCategoryCount{{Category: "postgres://operator:secret@db/internal", Count: 1}},
		LatencyBucket: "0-9ms",
	}
	if _, err := MarshalRetrievalTrajectory(report); err == nil {
		t.Fatal("MarshalRetrievalTrajectory() error = nil, want unsafe category rejected")
	}
}

type derivedArtifactRepositoryStub struct {
	deletedKinds []DerivedEvaluationArtifactKind
	before       time.Time
}

func (s *derivedArtifactRepositoryStub) DeleteExpiredEvaluationArtifacts(_ context.Context, before time.Time, kinds []DerivedEvaluationArtifactKind) (map[DerivedEvaluationArtifactKind]int, error) {
	s.before = before
	s.deletedKinds = append([]DerivedEvaluationArtifactKind(nil), kinds...)
	return map[DerivedEvaluationArtifactKind]int{
		DerivedEvaluationArtifactTrajectory: 2,
		DerivedEvaluationArtifactReport:     1,
	}, nil
}

func TestEvaluationArtifactRetentionDeletesOnlyDerivedKinds(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	repo := &derivedArtifactRepositoryStub{}
	result, err := (EvaluationArtifactRetention{
		Repository: repo,
		Window:     7 * 24 * time.Hour,
		Now:        func() time.Time { return now },
	}).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !repo.before.Equal(now.Add(-7 * 24 * time.Hour)) {
		t.Fatalf("before = %s", repo.before)
	}
	if len(repo.deletedKinds) != 4 {
		t.Fatalf("deleted kinds = %v, want four derived kinds", repo.deletedKinds)
	}
	for _, kind := range repo.deletedKinds {
		if kind == DerivedEvaluationArtifactKind("canonical_memory") {
			t.Fatal("retention attempted to delete canonical memory")
		}
	}
	if result.TotalDeleted != 3 || result.Category != "deleted" {
		t.Fatalf("result = %+v", result)
	}
}

func TestBuildMemoryOrganizationIntegritySeparatesActionFromInformation(t *testing.T) {
	report, err := BuildMemoryOrganizationIntegrity(MemoryOrganizationIntegrityInput{
		SchemaVersion: "memory-integrity-v1",
		Operation:     "merge",
		ActionSuccess: true,
		Expected: []IntegrityEvidence{
			{Alias: "fact-a", Placement: "profile", Digest: "sha256:a"},
			{Alias: "fact-b", Placement: "episodic", Digest: "sha256:b"},
		},
		Actual: []IntegrityEvidence{
			{Alias: "fact-a", Placement: "procedural", Digest: "sha256:changed"},
			{Alias: "fact-a", Placement: "procedural", Digest: "sha256:changed"},
			{Alias: "unexpected", Placement: "summary", Digest: "sha256:x"},
		},
	})
	if err != nil {
		t.Fatalf("BuildMemoryOrganizationIntegrity() error = %v", err)
	}
	if !report.ActionSuccess || report.InformationIntegrityPassed {
		t.Fatalf("report = %+v, want action success with integrity failure", report)
	}
	if report.MissingCount != 1 || report.AlteredCount != 1 || report.MisplacedCount != 1 || report.DuplicateCount != 1 || report.UnexpectedCount != 1 {
		t.Fatalf("unexpected integrity counts: %+v", report)
	}
	if report.FactEvidenceRecall != 0.5 || report.PlacementAccuracy != 0 {
		t.Fatalf("recall/placement = %v/%v", report.FactEvidenceRecall, report.PlacementAccuracy)
	}
}

func TestMemoryOrganizationIntegritySafetyFailureOverridesPreservedFacts(t *testing.T) {
	report, err := BuildMemoryOrganizationIntegrity(MemoryOrganizationIntegrityInput{
		SchemaVersion:  "memory-integrity-v1",
		Operation:      "projection",
		ActionSuccess:  true,
		Expected:       []IntegrityEvidence{{Alias: "fact-a", Placement: "summary", Digest: "sha256:a"}},
		Actual:         []IntegrityEvidence{{Alias: "fact-a", Placement: "summary", Digest: "sha256:a"}},
		SafetyFailures: []string{"isolation_violation"},
	})
	if err != nil {
		t.Fatalf("BuildMemoryOrganizationIntegrity() error = %v", err)
	}
	if report.InformationIntegrityPassed {
		t.Fatalf("report = %+v, want hard safety failure", report)
	}
}
