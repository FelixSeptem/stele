package retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const retrievalTrajectorySchemaVersion = "retrieval-trajectory-v1"

type TrajectoryChannelAggregate struct {
	Channel              string `json:"channel"`
	Availability         string `json:"availability"`
	CandidateCountBucket string `json:"candidate_count_bucket"`
}

type TrajectoryCategoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

type RetrievalTrajectory struct {
	SchemaVersion         string                       `json:"schema_version"`
	ReportVersion         string                       `json:"report_version"`
	PolicyVersion         string                       `json:"policy_version"`
	Channels              []TrajectoryChannelAggregate `json:"channels,omitempty"`
	ParentExpansionBucket string                       `json:"parent_expansion_bucket,omitempty"`
	ChildExpansionBucket  string                       `json:"child_expansion_bucket,omitempty"`
	Dispositions          []TrajectoryCategoryCount    `json:"dispositions,omitempty"`
	Fallbacks             []TrajectoryCategoryCount    `json:"fallbacks,omitempty"`
	LatencyBucket         string                       `json:"latency_bucket"`
}

func (t RetrievalTrajectory) Validate() error {
	if t.SchemaVersion != retrievalTrajectorySchemaVersion || strings.TrimSpace(t.ReportVersion) == "" || strings.TrimSpace(t.PolicyVersion) == "" {
		return fmt.Errorf("trajectory schema, report, and policy versions are required")
	}
	if strings.TrimSpace(t.LatencyBucket) == "" {
		return fmt.Errorf("trajectory latency bucket is required")
	}
	for _, c := range t.Channels {
		if strings.TrimSpace(c.Channel) == "" || strings.TrimSpace(c.Availability) == "" || strings.TrimSpace(c.CandidateCountBucket) == "" {
			return fmt.Errorf("trajectory channel aggregate is incomplete")
		}
	}
	for _, group := range [][]TrajectoryCategoryCount{t.Dispositions, t.Fallbacks} {
		for _, item := range group {
			if strings.TrimSpace(item.Category) == "" || item.Count < 0 || item.Count > 1_000_000 {
				return fmt.Errorf("trajectory category is invalid")
			}
			lower := strings.ToLower(item.Category)
			for _, forbidden := range []string{"postgres://", "query", "tenant", "project", "namespace", "memory", "event", "score", "credential", "provider_error"} {
				if strings.Contains(lower, forbidden) {
					return fmt.Errorf("trajectory category contains forbidden material")
				}
			}
		}
	}
	return nil
}

func MarshalRetrievalTrajectory(t RetrievalTrajectory) ([]byte, error) {
	if t.SchemaVersion == "" {
		t.SchemaVersion = retrievalTrajectorySchemaVersion
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(t)
}

type DerivedEvaluationArtifactKind string

const (
	DerivedEvaluationArtifactTrajectory DerivedEvaluationArtifactKind = "trajectory"
	DerivedEvaluationArtifactDiagnostic DerivedEvaluationArtifactKind = "diagnostic"
	DerivedEvaluationArtifactReport     DerivedEvaluationArtifactKind = "report"
	DerivedEvaluationArtifactFixture    DerivedEvaluationArtifactKind = "fixture"
)

type DerivedEvaluationArtifactRepository interface {
	DeleteExpiredEvaluationArtifacts(context.Context, time.Time, []DerivedEvaluationArtifactKind) (map[DerivedEvaluationArtifactKind]int, error)
}

type EvaluationArtifactRetention struct {
	Repository DerivedEvaluationArtifactRepository
	Window     time.Duration
	Now        func() time.Time
}

type EvaluationArtifactRetentionResult struct {
	Category     string                                `json:"category"`
	Before       time.Time                             `json:"before"`
	Deleted      map[DerivedEvaluationArtifactKind]int `json:"deleted"`
	TotalDeleted int                                   `json:"total_deleted"`
}

func (r EvaluationArtifactRetention) Run(ctx context.Context) (EvaluationArtifactRetentionResult, error) {
	if r.Repository == nil {
		return EvaluationArtifactRetentionResult{}, fmt.Errorf("retention repository is required")
	}
	if r.Window <= 0 {
		return EvaluationArtifactRetentionResult{}, fmt.Errorf("retention window must be positive")
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	before := now().UTC().Add(-r.Window)
	kinds := []DerivedEvaluationArtifactKind{DerivedEvaluationArtifactTrajectory, DerivedEvaluationArtifactDiagnostic, DerivedEvaluationArtifactReport, DerivedEvaluationArtifactFixture}
	deleted, err := r.Repository.DeleteExpiredEvaluationArtifacts(ctx, before, kinds)
	if err != nil {
		return EvaluationArtifactRetentionResult{}, err
	}
	result := EvaluationArtifactRetentionResult{Category: "deleted", Before: before, Deleted: deleted}
	for _, count := range deleted {
		if count > 0 {
			result.TotalDeleted += count
		}
	}
	return result, nil
}

type IntegrityEvidence struct {
	Alias     string `json:"alias"`
	Placement string `json:"placement"`
	Digest    string `json:"digest"`
}

type MemoryOrganizationIntegrityInput struct {
	SchemaVersion  string
	Operation      string
	ActionSuccess  bool
	Expected       []IntegrityEvidence
	Actual         []IntegrityEvidence
	SafetyFailures []string
}

type MemoryOrganizationIntegrityReport struct {
	SchemaVersion              string   `json:"schema_version"`
	Operation                  string   `json:"operation"`
	ActionSuccess              bool     `json:"action_success"`
	InformationIntegrityPassed bool     `json:"information_integrity_passed"`
	FactEvidenceRecall         float64  `json:"fact_evidence_recall"`
	PlacementAccuracy          float64  `json:"placement_accuracy"`
	DuplicateCount             int      `json:"duplicate_count"`
	MissingCount               int      `json:"missing_count"`
	AlteredCount               int      `json:"altered_count"`
	MisplacedCount             int      `json:"misplaced_count"`
	UnexpectedCount            int      `json:"unexpected_count"`
	SafetyFailures             []string `json:"safety_failures,omitempty"`
}

func BuildMemoryOrganizationIntegrity(in MemoryOrganizationIntegrityInput) (MemoryOrganizationIntegrityReport, error) {
	if strings.TrimSpace(in.SchemaVersion) == "" || strings.TrimSpace(in.Operation) == "" {
		return MemoryOrganizationIntegrityReport{}, fmt.Errorf("integrity schema version and operation are required")
	}
	report := MemoryOrganizationIntegrityReport{SchemaVersion: in.SchemaVersion, Operation: in.Operation, ActionSuccess: in.ActionSuccess, SafetyFailures: append([]string(nil), in.SafetyFailures...)}
	expected := make(map[string]IntegrityEvidence, len(in.Expected))
	for _, e := range in.Expected {
		if strings.TrimSpace(e.Alias) == "" {
			return report, fmt.Errorf("expected evidence alias is required")
		}
		if _, ok := expected[e.Alias]; ok {
			return report, fmt.Errorf("duplicate expected evidence alias")
		}
		expected[e.Alias] = e
	}
	actual := make(map[string][]IntegrityEvidence, len(in.Actual))
	for _, e := range in.Actual {
		if strings.TrimSpace(e.Alias) == "" {
			return report, fmt.Errorf("actual evidence alias is required")
		}
		actual[e.Alias] = append(actual[e.Alias], e)
	}
	exactPlacement := 0
	presentEvidence := 0
	for alias, want := range expected {
		got, ok := actual[alias]
		if !ok {
			report.MissingCount++
			continue
		}
		presentEvidence++
		if len(got) > 1 {
			report.DuplicateCount += len(got) - 1
		}
		if got[0].Digest != want.Digest {
			report.AlteredCount++
		}
		if got[0].Placement != want.Placement {
			report.MisplacedCount++
		} else if got[0].Digest == want.Digest {
			exactPlacement++
		}
	}
	for alias := range actual {
		if _, ok := expected[alias]; !ok {
			report.UnexpectedCount += len(actual[alias])
		}
	}
	if len(in.Expected) > 0 {
		report.PlacementAccuracy = float64(exactPlacement) / float64(len(in.Expected))
	}
	if len(in.Expected) > 0 {
		report.FactEvidenceRecall = float64(presentEvidence) / float64(len(in.Expected))
	}
	report.InformationIntegrityPassed = report.MissingCount == 0 && report.DuplicateCount == 0 && report.AlteredCount == 0 && report.MisplacedCount == 0 && report.UnexpectedCount == 0 && len(report.SafetyFailures) == 0
	return report, nil
}

func (r MemoryOrganizationIntegrityReport) StableSafetyCategories() []string {
	out := append([]string(nil), r.SafetyFailures...)
	sort.Strings(out)
	return out
}
