package benchmark

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func TestLoadRepresentationFixtureLocksAllFamiliesAndRejectsTampering(t *testing.T) {
	fixture := validRepresentationFixture()
	qrelChecksum, sourceChecksum, err := RepresentationFixtureLocks(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fixture.QRELChecksum = qrelChecksum
	fixture.SourceChecksum = sourceChecksum
	encoded, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRepresentationFixture(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Cases) != 4 {
		t.Fatalf("case count = %d, want 4", len(loaded.Cases))
	}

	tampered := []byte(strings.ReplaceAll(string(encoded), "profile-source", "different-source"))
	if _, err := LoadRepresentationFixture(tampered); StatusOf(err) != StatusChecksumMismatch {
		t.Fatalf("tampered fixture status=%q err=%v", StatusOf(err), err)
	}
	unknown := []byte(strings.Replace(string(encoded), `"schema_version"`, `"unknown":true,"schema_version"`, 1))
	if _, err := LoadRepresentationFixture(unknown); StatusOf(err) != StatusInvalidManifest {
		t.Fatalf("unknown-field fixture status=%q err=%v", StatusOf(err), err)
	}
}

func TestTaskAppropriateRepresentationFixtureV1IsChecksumLocked(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("testdata", "task-appropriate-representation-fixture-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var unlocked RepresentationFixture
	if err := json.Unmarshal(source, &unlocked); err != nil {
		t.Fatal(err)
	}
	q, s, err := RepresentationFixtureLocks(unlocked)
	if err != nil {
		t.Fatal(err)
	}
	if unlocked.QRELChecksum != q || unlocked.SourceChecksum != s {
		t.Fatalf("fixture locks: qrel=%s source=%s", q, s)
	}
	fixture, err := LoadRepresentationFixture(source)
	if err != nil {
		t.Fatal(err)
	}
	qrelChecksum, sourceChecksum, err := RepresentationFixtureLocks(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if fixture.QRELChecksum != qrelChecksum || fixture.SourceChecksum != sourceChecksum {
		t.Fatalf("fixture locks qrel=%s source=%s, want qrel=%s source=%s", fixture.QRELChecksum, fixture.SourceChecksum, qrelChecksum, sourceChecksum)
	}
}

func TestRepresentationFixtureRejectsUnequalBudgetAndIncompatibleCandidate(t *testing.T) {
	fixture := validRepresentationFixture()
	fixture.Cases[0].Candidate = retrieval.RepresentationCandidateTopicProjectionWithSearch
	if _, _, err := RepresentationFixtureLocks(fixture); err == nil {
		t.Fatal("RepresentationFixtureLocks() error = nil, want incompatible candidate")
	}
	fixture = validRepresentationFixture()
	fixture.Cases[1].CandidateLimits.MaxTokens++
	if _, _, err := RepresentationFixtureLocks(fixture); err == nil {
		t.Fatal("RepresentationFixtureLocks() error = nil, want unequal budgets")
	}
}

func TestReplayRepresentationFixtureStaysShadowAndBuildsEachTaskFamily(t *testing.T) {
	fixture := validRepresentationFixture()
	reports, err := ReplayRepresentationFixture(context.Background(), fixture, "renderer-v1", timeForRepresentationFixture(), validRepresentationObserver{})
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 4 {
		t.Fatalf("report count=%d, want 4", len(reports))
	}
	for _, report := range reports {
		if report.Mode != retrieval.RepresentationEvaluationModeShadow || report.SelectedStrategy != "flat-fusion-v1" || !report.SourceSearchRetained || report.ActivationEligible {
			t.Fatalf("report escaped the evaluation boundary: %+v", report)
		}
	}
}

func TestRepresentationFixtureReplayRejectsDirectSourceTamper(t *testing.T) {
	fixture := validRepresentationFixture()
	fixture.SourceChecksum = strings.Repeat("0", 64)
	if _, err := ReplayRepresentationFixture(context.Background(), fixture, "renderer-v1", timeForRepresentationFixture(), validRepresentationObserver{}); err == nil {
		t.Fatal("direct replay bypassed the locked source checksum")
	}
}

func TestRepresentationFixtureActuallyRunsBothConditionsAndRollback(t *testing.T) {
	observer := &mutatingRepresentationObserver{}
	reports, err := ReplayRepresentationFixture(context.Background(), validRepresentationFixture(), "renderer-v1", timeForRepresentationFixture(), observer)
	if err != nil {
		t.Fatal(err)
	}
	if observer.baselineCalls != 8 || observer.candidateCalls != 8 {
		t.Fatalf("calls baseline=%d candidate=%d", observer.baselineCalls, observer.candidateCalls)
	}
	for _, report := range reports {
		if report.BaselineMetrics == nil || report.ComparisonDelta == nil || report.BaselineMetrics.CriticalFactRetention != 1 || report.RealStack || !report.RollbackTested {
			t.Fatalf("missing real comparison or promoted synthetic: %+v", report)
		}
	}
}

func TestRepresentationReplayDigestBindsEqualSizedDerivedExcerpts(t *testing.T) {
	fixture := validRepresentationFixture()
	for i := range fixture.Cases {
		for j := range fixture.Cases[i].Sources {
			fixture.Cases[i].Sources[j].Content = "left value=731 right"
		}
	}
	fixture.QRELChecksum, fixture.SourceChecksum, _ = RepresentationFixtureLocks(fixture)
	left, err := ReplayRepresentationFixture(context.Background(), fixture, "renderer-v1", timeForRepresentationFixture(), &mutatingRepresentationObserver{kind: "left-excerpt"})
	if err != nil {
		t.Fatal(err)
	}
	right, err := ReplayRepresentationFixture(context.Background(), fixture, "renderer-v1", timeForRepresentationFixture(), &mutatingRepresentationObserver{kind: "right-excerpt"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range left {
		if left[i].RunIdentity == right[i].RunIdentity || left[i].EvidenceDigest == right[i].EvidenceDigest {
			t.Fatal("distinct stable excerpts collapsed into identical report identity")
		}
	}
}

func TestRepresentationMetricsCannotReplaceFactsWithCounts(t *testing.T) {
	for _, kind := range []string{"wrong-value", "wrong-citation", "mixed-citation", "hidden", "version", "changed-source", "nondeterministic", "expired", "future"} {
		t.Run(kind, func(t *testing.T) {
			observer := &mutatingRepresentationObserver{kind: kind}
			reports, err := ReplayRepresentationFixture(context.Background(), validRepresentationFixture(), "renderer-v1", timeForRepresentationFixture(), observer)
			if err != nil {
				t.Fatal(err)
			}
			for _, report := range reports {
				if report.Comparable || report.Verdict != retrieval.RepresentationEvaluationNonPass {
					t.Fatalf("%s passed: %+v", kind, report)
				}
			}
		})
	}
}

type mutatingRepresentationObserver struct {
	kind                          string
	baselineCalls, candidateCalls int
}

func (o *mutatingRepresentationObserver) ObserveRepresentation(ctx context.Context, item RepresentationFixtureCase, strategy string) (RepresentationObservation, error) {
	result, err := (validRepresentationObserver{}).ObserveRepresentation(ctx, item, strategy)
	if strategy == retrieval.ParentFirstFlatStrategy {
		o.baselineCalls++
		return result, err
	}
	o.candidateCalls++
	result.CriticalFactsReturned = 999
	result.Evidence = append([]retrieval.ProgressiveContextEvidence(nil), result.Evidence...)
	switch o.kind {
	case "left-excerpt":
		for i := range result.Evidence {
			result.Evidence[i].Memory.Content = "left value=731"
		}
	case "right-excerpt":
		for i := range result.Evidence {
			result.Evidence[i].Memory.Content = "value=731 right"
		}
	case "wrong-value":
		result.Evidence[0].Memory.Content = "controlled exact value=999"
	case "wrong-citation":
		result.Evidence[0].Citations = []retrieval.Citation{{MemoryID: "foreign", RawEventID: "bogus"}}
	case "mixed-citation":
		result.Evidence[0].Citations = append(append([]retrieval.Citation(nil), result.Evidence[0].Citations...), retrieval.Citation{MemoryID: result.Evidence[0].Memory.ID, RawEventID: "bogus"})
	case "expired":
		end := timeForRepresentationFixture().Add(-time.Hour)
		result.Evidence[0].Memory.TemporalValidity = memory.TemporalValidity{TemporalFactID: "fact", IngestedAt: end.Add(-time.Hour), ValidFrom: end.Add(-time.Hour), ValidTo: &end}
	case "future":
		start := timeForRepresentationFixture().Add(time.Hour)
		result.Evidence[0].Memory.TemporalValidity = memory.TemporalValidity{TemporalFactID: "fact", IngestedAt: start, ValidFrom: start}
	case "hidden":
		result.Evidence[0].Memory.State = memory.MemoryStateSuppressed
	case "version":
		result.Evidence[0].Memory.SourceVersion++
	case "changed-source":
		result.SourceWatermark = strings.Repeat("b", 64)
	case "nondeterministic":
		if o.candidateCalls%2 == 0 {
			result.Evidence[0].Memory.Content = "controlled"
		}
	}
	return result, err
}

type validRepresentationObserver struct{}

func (validRepresentationObserver) ObserveRepresentation(_ context.Context, item RepresentationFixtureCase, strategy string) (RepresentationObservation, error) {
	observation := RepresentationObservation{SourceWatermark: strings.Repeat("a", 64), LatencyMS: 3, SourceSearchRetained: true}
	for _, source := range item.Sources {
		entry := retrieval.ProgressiveContextEvidence{Alias: source.Alias, Memory: memory.CanonicalMemory{ID: source.Alias, Scope: item.Scope, State: source.State, SourceVersion: 1, Content: source.Content}, Citations: []retrieval.Citation{{MemoryID: source.Alias, RawEventID: source.Alias + "-event"}}}
		observation.SourceEvidence = append(observation.SourceEvidence, entry)
		if source.State == memory.MemoryStateActive {
			observation.Evidence = append(observation.Evidence, entry)
		}
	}
	if strategy != retrieval.ParentFirstFlatStrategy {
		observation.SearchEvidence = observation.Evidence
	}
	return observation, nil
}

func validRepresentationFixture() RepresentationFixture {
	scope := memory.Scope{Tenant: "benchmark", Project: "benchmark-representations", Namespace: "ev3"}
	limits := retrieval.RepresentationEvaluationLimits{MaxCandidates: 4, MaxCharacters: 256, MaxTokens: 64, MaxLatencyMS: 100}
	fixture := RepresentationFixture{
		SchemaVersion: RepresentationFixtureSchemaVersion,
		Cases: []RepresentationFixtureCase{
			{ID: "exact", Family: retrieval.RepresentationTaskFamilyExactFacts, Candidate: retrieval.RepresentationCandidateExactExcerptWithSearch, Scope: scope, CriticalAlias: "exact-source", RequiredSourceAliases: []string{"exact-source"}, CandidateLimits: limits, BaselineLimits: limits},
			{ID: "cross", Family: retrieval.RepresentationTaskFamilyCrossDocument, Candidate: retrieval.RepresentationCandidateTopicProjectionWithSearch, Scope: scope, CriticalAlias: "cross-source", RequiredSourceAliases: []string{"cross-source", "cross-other"}, CandidateLimits: limits, BaselineLimits: limits},
			{ID: "profile", Family: retrieval.RepresentationTaskFamilyProfilePreference, Candidate: retrieval.RepresentationCandidateTopicProjectionWithSearch, Scope: scope, CriticalAlias: "profile-source", RequiredSourceAliases: []string{"profile-source"}, CandidateLimits: limits, BaselineLimits: limits},
			{ID: "procedure", Family: retrieval.RepresentationTaskFamilyProceduralLesson, Candidate: retrieval.RepresentationCandidateHierarchicalWithSearch, Scope: scope, CriticalAlias: "procedure-source", RequiredSourceAliases: []string{"procedure-source"}, CandidateLimits: limits, BaselineLimits: limits},
		},
	}
	for i := range fixture.Cases {
		item := &fixture.Cases[i]
		item.Query = "controlled"
		for _, alias := range item.RequiredSourceAliases {
			item.Sources = append(item.Sources, retrieval.EvaluationSource{Alias: alias, EventType: "fixture", Content: "controlled exact value=731", State: memory.MemoryStateActive})
			item.CriticalFacts = append(item.CriticalFacts, retrieval.RepresentationCriticalFact{Alias: alias, Value: "value=731"})
		}
	}
	fixture.QRELChecksum, fixture.SourceChecksum, _ = RepresentationFixtureLocks(fixture)
	return fixture
}

func timeForRepresentationFixture() time.Time {
	return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
}
