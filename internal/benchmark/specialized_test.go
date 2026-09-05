package benchmark

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestSpecializedFixtureNormalizesTemporalAndMultiHopEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "specialized", Namespace: "run-1"}
	fixture := SpecializedFixture{SchemaVersion: SchemaVersion, Subfamily: "temporal", Events: []MemoryEventRecord{
		{ID: "old", Scope: scope, Text: "Alex lived in Paris", ExpectedState: memory.MemoryStateSuppressed},
		{ID: "current", Scope: scope, Text: "Alex lives in Berlin", ExpectedState: memory.MemoryStateActive},
	}, Queries: []BenchmarkQuery{{ID: "q", Scope: scope, Text: "Where does Alex live?", MustNotReturnIDs: []string{"old"}}}, QRELs: []QREL{{QueryID: "q", EvidenceID: "current", Grade: 2}}}
	corpus, err := fixture.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	report := EvaluateSpecializedFor(fixture.Subfamily, corpus, []RetrievedEvidence{{EvidenceID: "current", Rank: 1}}, time.Millisecond)
	if report.Family != "specialized_retrieval" || report.Subfamily != "temporal" || report.Metrics.TemporalUpdatePrecedence != 1 || report.Metrics.StaleFactSuppression != 1 {
		t.Fatalf("unexpected specialized report: %#v", report)
	}
}

func TestStressAdmissionRefusesVisualOrOverBudget(t *testing.T) {
	if status := AdmitStress(StressConfig{ContextLength: 100, MaxContextLength: 10, Mode: "text"}); status != StatusCapacityRefused {
		t.Fatalf("got %s", status)
	}
	if status := AdmitStress(StressConfig{ContextLength: 10, MaxContextLength: 10, Mode: "visual"}); status != StatusPrerequisiteMissing {
		t.Fatalf("got %s", status)
	}
	if status := AdmitStress(StressConfig{ContextLength: 10, MaxContextLength: 10, Mode: "text"}); status != StatusSuccess {
		t.Fatalf("got %s", status)
	}
}

func TestProfilePreferenceFixturePreservesCurrentAndHistoricalFacts(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "specialized", Namespace: "run-1"}
	fixture := ProfilePreferenceFixture{Facts: []ProfilePreferenceFact{
		{ID: "pref-old", SessionID: "session-a", Text: "Mina prefers tea", Current: false},
		{ID: "pref-new", SessionID: "session-b", Text: "Mina prefers coffee", Current: true},
	}, Queries: []ProfilePreferenceQuery{
		{ID: "current", Text: "What does Mina prefer now?", FactID: "pref-new", Historical: false},
		{ID: "historical", Text: "What did Mina prefer before?", FactID: "pref-old", Historical: true},
	}}
	corpus, err := fixture.Normalize(scope)
	if err != nil {
		t.Fatal(err)
	}
	if corpus.Events[0].Class != memory.MemoryClassProfile || corpus.Events[0].ExpectedState != memory.MemoryStateSuppressed || corpus.Events[1].ExpectedState != memory.MemoryStateActive {
		t.Fatalf("profile lifecycle was not preserved: %#v", corpus.Events)
	}
	if corpus.Queries[0].MustNotReturnIDs[0] != "pref-old" || corpus.Queries[1].QueryType != "profile_history" || corpus.Events[1].Provenance["session_id"] != "session-b" {
		t.Fatalf("profile facts lost session/current-history metadata: %#v", corpus)
	}
}

func TestEvaluateSpecializedQueriesReportsProfileCurrentAndHistoricalConsistency(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "specialized", Namespace: "run-1"}
	corpus, err := (ProfilePreferenceFixture{Facts: []ProfilePreferenceFact{
		{ID: "old", SessionID: "a", Text: "tea", Current: false},
		{ID: "new", SessionID: "b", Text: "coffee", Current: true},
	}, Queries: []ProfilePreferenceQuery{
		{ID: "current", Text: "now?", FactID: "new"},
		{ID: "history", Text: "before?", FactID: "old", Historical: true},
	}}).Normalize(scope)
	if err != nil {
		t.Fatal(err)
	}
	report := EvaluateSpecializedQueries("profile_preference", corpus, map[string][]RetrievedEvidence{
		"current": {{EvidenceID: "new", Rank: 1}}, "history": {{EvidenceID: "old", Rank: 1}},
	}, time.Millisecond)
	if report.Metrics.ProfileRecall != 1 || report.Metrics.PreferenceConsistency != 1 || report.Metrics.HistoricalPreferenceRecall != 1 {
		t.Fatalf("unexpected profile metrics: %#v", report.Metrics)
	}
}

func TestProfilePreferenceMetricsDetectForeignSessionEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "specialized", Namespace: "run-1"}
	corpus, err := (ProfilePreferenceFixture{Facts: []ProfilePreferenceFact{
		{ID: "session-a-fact", SessionID: "session-a", Text: "Mina likes tea", Current: true},
		{ID: "session-b-fact", SessionID: "session-b", Text: "Other user likes coffee", Current: true},
	}, Queries: []ProfilePreferenceQuery{{ID: "q", Text: "What does Mina like?", FactID: "session-a-fact", AllowedSessionIDs: []string{"session-a"}}}}).Normalize(scope)
	if err != nil {
		t.Fatal(err)
	}
	report := EvaluateSpecializedQueries("profile_preference", corpus, map[string][]RetrievedEvidence{"q": {
		{EvidenceID: "session-a-fact", Rank: 1}, {EvidenceID: "session-b-fact", Rank: 2},
	}}, time.Millisecond)
	if report.Metrics.SessionIsolationViolations != 1 {
		t.Fatalf("expected foreign session evidence detection, got %#v", report.Metrics)
	}
}

func TestTemporalFixtureRecordsDateBoundedUpdateAndStaleSuppression(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "specialized", Namespace: "run-1"}
	fixture := TemporalFixture{Facts: []TemporalFact{
		{ID: "old", Text: "Mina lives in Paris", ValidFrom: "2023-01-01", ValidTo: "2023-12-31", Current: false},
		{ID: "new", Text: "Mina lives in Berlin", ValidFrom: "2024-01-01", Current: true},
	}, Queries: []TemporalQuery{{ID: "q", Text: "Where does Mina live in 2024?", At: "2024-02-01", FactID: "new"}}}
	corpus, err := fixture.Normalize(scope)
	if err != nil {
		t.Fatal(err)
	}
	if corpus.Events[0].ExpectedState != memory.MemoryStateSuppressed || corpus.Queries[0].Metadata["at"] != "2024-02-01" || corpus.Queries[0].MustNotReturnIDs[0] != "old" {
		t.Fatalf("temporal update metadata/lifecycle lost: %#v", corpus)
	}
}

func TestMultiHopFixtureRequiresCompleteEvidenceGroup(t *testing.T) {
	scope := memory.Scope{Tenant: "benchmark", Project: "specialized", Namespace: "run-1"}
	corpus, err := (MultiHopFixture{Facts: []MultiHopFact{{ID: "city", Text: "Ada lives in Berlin"}, {ID: "country", Text: "Berlin is in Germany"}}, Queries: []MultiHopQuery{{ID: "q", Text: "Which country does Ada live in?", EvidenceIDs: []string{"city", "country"}}}}).Normalize(scope)
	if err != nil {
		t.Fatal(err)
	}
	report := EvaluateSpecializedQueries("multi_hop", corpus, map[string][]RetrievedEvidence{"q": {{EvidenceID: "city", Rank: 1}}}, time.Millisecond)
	if report.Metrics.EvidenceGroupCompleteness != 0 {
		t.Fatalf("partial multi-hop result must not count as complete: %#v", report)
	}
}

func TestSpecializedSuitesRetainRepresentativeLifecycleAndIsolationReports(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	manifest.Family = FamilySpecialized
	manifest.Name = "specialized-fixtures"
	manifest.Version = "v1"
	manifest.ConversionVersion = "specialized-v1"
	manifest.Splits = map[string]SplitSpec{
		"profile":   {Identity: "specialized/profile", Source: "profile.json"},
		"temporal":  {Identity: "specialized/temporal", Source: "temporal.json"},
		"multi-hop": {Identity: "specialized/multi-hop", Source: "multi-hop.json"},
	}
	base := memory.Scope{Tenant: "tenant-a", Project: "production", Namespace: "default"}

	profileRun, err := NewRunScope(base, manifest.Name, "profile-regression")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := (ProfilePreferenceFixture{Facts: []ProfilePreferenceFact{
		{ID: "old", SessionID: "session-a", Text: "Mina prefers tea", Current: false},
		{ID: "current", SessionID: "session-b", Text: "Mina prefers coffee", Current: true},
	}, Queries: []ProfilePreferenceQuery{
		{ID: "current-query", Text: "What does Mina prefer now?", FactID: "current", AllowedSessionIDs: []string{"session-b"}},
		{ID: "history", Text: "What did Mina prefer before?", FactID: "old", Historical: true, AllowedSessionIDs: []string{"session-a"}},
	}}).Normalize(profileRun.Scope)
	if err != nil {
		t.Fatal(err)
	}
	profileReport := EvaluateSpecializedQueries("profile_preference", profile, map[string][]RetrievedEvidence{
		"current-query": {{EvidenceID: "current", Rank: 1}},
		"history":       {{EvidenceID: "old", Rank: 1}},
	}, time.Millisecond)
	if profileReport.Metrics.ProfileRecall != 1 || profileReport.Metrics.HistoricalPreferenceRecall != 1 || profileReport.Metrics.SessionIsolationViolations != 0 {
		t.Fatalf("profile lifecycle/isolation regression: %#v", profileReport.Metrics)
	}

	temporalRun, err := NewRunScope(base, manifest.Name, "temporal-regression")
	if err != nil {
		t.Fatal(err)
	}
	temporal, err := (TemporalFixture{Facts: []TemporalFact{
		{ID: "old", Text: "Mina lived in Paris", ValidFrom: "2023-01-01", ValidTo: "2023-12-31", Current: false},
		{ID: "current", Text: "Mina lives in Berlin", ValidFrom: "2024-01-01", Current: true},
	}, Queries: []TemporalQuery{{ID: "at-2024", Text: "Where does Mina live in 2024?", At: "2024-02-01", FactID: "current"}}}).Normalize(temporalRun.Scope)
	if err != nil {
		t.Fatal(err)
	}
	temporalReport := EvaluateSpecializedQueries("temporal", temporal, map[string][]RetrievedEvidence{"at-2024": {{EvidenceID: "current", Rank: 1}}}, time.Millisecond)
	if temporalReport.Metrics.TemporalUpdatePrecedence != 1 || temporalReport.Metrics.StaleFactSuppression != 1 || temporalReport.Metrics.ScopeSafetyFailures != 0 {
		t.Fatalf("temporal lifecycle regression: %#v", temporalReport.Metrics)
	}

	multiHopRun, err := NewRunScope(base, manifest.Name, "multi-hop-regression")
	if err != nil {
		t.Fatal(err)
	}
	multiHop, err := (MultiHopFixture{Facts: []MultiHopFact{{ID: "city", Text: "Ada lives in Berlin"}, {ID: "country", Text: "Berlin is in Germany"}}, Queries: []MultiHopQuery{{ID: "country-query", Text: "Which country does Ada live in?", EvidenceIDs: []string{"city", "country"}}}}).Normalize(multiHopRun.Scope)
	if err != nil {
		t.Fatal(err)
	}
	multiHopReport := EvaluateSpecializedQueries("multi_hop", multiHop, map[string][]RetrievedEvidence{"country-query": {{EvidenceID: "city", Rank: 1}, {EvidenceID: "country", Rank: 2}}}, time.Millisecond)
	if multiHopReport.Metrics.EvidenceGroupCompleteness != 1 || multiHopReport.Metrics.ScopeSafetyFailures != 0 {
		t.Fatalf("multi-hop evidence regression: %#v", multiHopReport.Metrics)
	}

	for _, fixture := range []struct {
		split  string
		run    BenchmarkRunScope
		result SpecializedReport
	}{
		{split: "profile", run: profileRun, result: profileReport},
		{split: "temporal", run: temporalRun, result: temporalReport},
		{split: "multi-hop", run: multiHopRun, result: multiHopReport},
	} {
		report := NewFamilyReport(FamilySpecialized, manifest, fixture.split, fixture.run.Scope).WithExecutionProvenance(FamilyReportExecution{QRELVersion: "specialized-qrels-v1", StrategyProfile: "fixture-retrieval"})
		report.Metrics = fixture.result
		report.SafetyOutcomes = fixture.result.Metrics
		path, err := cache.WriteFamilyReport(manifest, fixture.run.ID, report)
		if err != nil {
			t.Fatalf("retain %s report: %v", fixture.split, err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("retained %s report missing: %v", fixture.split, err)
		}
		if got := filepath.Base(path); got != fixture.run.ID+".json" {
			t.Fatalf("retained %s report name = %q", fixture.split, got)
		}
	}
}
