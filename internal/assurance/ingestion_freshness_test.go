package assurance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

func TestEvaluateIngestionFreshnessBucketsOrderedStagesDeterministically(t *testing.T) {
	base := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	input := IngestionFreshnessInput{
		Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		ReportKey:  "fixture-ordered",
		ObservedAt: base.Add(4 * time.Minute),
		Policy:     IngestionFreshnessPolicy{ContractVersion: IngestionFreshnessVersion, AdmissionWindow: 5 * time.Minute, GovernanceWindow: 2 * time.Minute, RetrievalWindow: 4 * time.Minute},
		Stages: []IngestionFreshnessStage{
			{Kind: IngestionStageAdmitted, Disposition: IngestionDispositionComplete, At: base},
			{Kind: IngestionStageGovernanceCompleted, Disposition: IngestionDispositionComplete, At: base.Add(30 * time.Second)},
			{Kind: IngestionStageCanonicalActive, Disposition: IngestionDispositionComplete, At: base.Add(time.Minute)},
			{Kind: IngestionStagePublicRetrievable, Disposition: IngestionDispositionComplete, At: base.Add(2 * time.Minute)},
		},
	}
	first, err := EvaluateIngestionFreshness(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EvaluateIngestionFreshness(input)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("evaluation is not deterministic:\n%s\n%s", firstJSON, secondJSON)
	}
	if first.Result != IngestionFreshnessResultFresh || first.Stages[len(first.Stages)-1].Disposition != IngestionDispositionComplete {
		t.Fatalf("result = %+v, want fresh complete evidence", first)
	}
	for _, stage := range first.Stages {
		if stage.LatencyBucket == "" {
			t.Fatalf("stage %q has no bounded latency bucket", stage.Kind)
		}
	}
}

func TestEvaluateIngestionFreshnessDoesNotPromoteIncompleteEvidence(t *testing.T) {
	base := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	input := IngestionFreshnessInput{
		Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		ReportKey:  "fixture-incomplete",
		ObservedAt: base.Add(time.Minute),
		Policy:     IngestionFreshnessPolicy{ContractVersion: IngestionFreshnessVersion, AdmissionWindow: time.Minute, GovernanceWindow: time.Minute, RetrievalWindow: time.Minute},
		Stages:     []IngestionFreshnessStage{{Kind: IngestionStageAdmitted, Disposition: IngestionDispositionComplete, At: base}, {Kind: IngestionStageGovernanceCompleted, Disposition: IngestionDispositionRetrying}},
	}
	got, err := EvaluateIngestionFreshness(input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Result == IngestionFreshnessResultFresh || got.Stages[1].Disposition != IngestionDispositionRetrying || got.Stages[2].Disposition == IngestionDispositionComplete {
		t.Fatalf("incomplete governance was promoted: %+v", got)
	}
}

func TestEvaluateIngestionFreshnessClassifiesWindowAndSLOWithoutError(t *testing.T) {
	base := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	got, err := EvaluateIngestionFreshness(IngestionFreshnessInput{
		Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		ReportKey:  "fixture-window",
		ObservedAt: base.Add(10 * time.Minute),
		Policy:     IngestionFreshnessPolicy{ContractVersion: IngestionFreshnessVersion, AdmissionWindow: time.Minute, GovernanceWindow: time.Minute, RetrievalWindow: time.Minute},
		Stages: []IngestionFreshnessStage{
			{Kind: IngestionStageAdmitted, Disposition: IngestionDispositionComplete, At: base},
			{Kind: IngestionStageGovernanceCompleted, Disposition: IngestionDispositionComplete, At: base.Add(2 * time.Minute)},
			{Kind: IngestionStageCanonicalActive, Disposition: IngestionDispositionComplete, At: base.Add(3 * time.Minute)},
			{Kind: IngestionStagePublicRetrievable, Disposition: IngestionDispositionComplete, At: base.Add(4 * time.Minute)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != IngestionFreshnessResultDegraded || got.SLO != "outside_window" {
		t.Fatalf("got result=%q slo=%q, want degraded/outside_window", got.Result, got.SLO)
	}
}

func TestEvaluateIngestionFreshnessEnforcesAdmissionAgeWindow(t *testing.T) {
	base := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	got, err := EvaluateIngestionFreshness(IngestionFreshnessInput{
		Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		ReportKey:  "fixture-admission-age",
		ObservedAt: base.Add(10 * time.Minute),
		Policy:     IngestionFreshnessPolicy{ContractVersion: IngestionFreshnessVersion, AdmissionWindow: time.Minute, GovernanceWindow: time.Hour, RetrievalWindow: time.Hour},
		Stages: []IngestionFreshnessStage{
			{Kind: IngestionStageAdmitted, Disposition: IngestionDispositionComplete, At: base},
			{Kind: IngestionStageGovernanceCompleted, Disposition: IngestionDispositionComplete, At: base.Add(time.Second)},
			{Kind: IngestionStageCanonicalActive, Disposition: IngestionDispositionComplete, At: base.Add(2 * time.Second)},
			{Kind: IngestionStagePublicRetrievable, Disposition: IngestionDispositionComplete, At: base.Add(3 * time.Second)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != IngestionFreshnessResultDegraded || got.SLO != "outside_window" {
		t.Fatalf("got result=%q slo=%q, want degraded/outside_window", got.Result, got.SLO)
	}
}

func TestInspectIngestionFreshnessUsesServerCollectedExactScopeEvidence(t *testing.T) {
	base := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	collector := &stubIngestionFreshnessCollector{input: IngestionFreshnessInput{
		Scope: scope, ReportKey: "server-event-reference", ObservedAt: base,
		Policy: IngestionFreshnessPolicy{ContractVersion: IngestionFreshnessVersion, AdmissionWindow: time.Hour, GovernanceWindow: time.Hour, RetrievalWindow: time.Hour},
		Stages: []IngestionFreshnessStage{
			{Kind: IngestionStageAdmitted, Disposition: IngestionDispositionComplete, At: base.Add(-3 * time.Second)},
			{Kind: IngestionStageGovernanceCompleted, Disposition: IngestionDispositionComplete, At: base.Add(-2 * time.Second)},
			{Kind: IngestionStageCanonicalActive, Disposition: IngestionDispositionComplete, At: base.Add(-time.Second)},
			{Kind: IngestionStagePublicRetrievable, Disposition: IngestionDispositionComplete, At: base},
		},
	}}
	store := &stubIngestionFreshnessEvidenceStore{}
	service := NewService(ServiceOptions{IngestionFreshnessCollector: collector, IngestionFreshnessEvidenceStore: store, Now: func() time.Time { return base }})
	first, err := service.InspectIngestionFreshness(context.Background(), IngestionFreshnessInspectionInput{Scope: scope, FixtureReference: "client-opaque-reference"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.InspectIngestionFreshness(context.Background(), IngestionFreshnessInspectionInput{Scope: scope, FixtureReference: "client-opaque-reference"})
	if err != nil {
		t.Fatal(err)
	}
	if collector.scope != scope || collector.fixtureReference != "client-opaque-reference" || len(store.created) != 1 {
		t.Fatalf("collector/store invocation=%+v/%d, want exact scope and one durable replay", collector, len(store.created))
	}
	if first.IdentityHash != second.IdentityHash || first.Result != IngestionFreshnessResultFresh {
		t.Fatalf("replay report=%+v/%+v, want stable fresh report", first, second)
	}
}

type stubIngestionFreshnessCollector struct {
	input            IngestionFreshnessInput
	scope            memory.Scope
	fixtureReference string
}

func (s *stubIngestionFreshnessCollector) CollectIngestionFreshness(_ context.Context, scope memory.Scope, fixtureReference string, _ time.Time) (IngestionFreshnessInput, error) {
	s.scope = scope
	s.fixtureReference = fixtureReference
	return s.input, nil
}

type stubIngestionFreshnessEvidenceStore struct {
	created []IngestionFreshnessEvidence
}

func (s *stubIngestionFreshnessEvidenceStore) StoreIngestionFreshnessEvidence(_ context.Context, evidence IngestionFreshnessEvidence) (IngestionFreshnessEvidence, error) {
	for _, prior := range s.created {
		if prior.FixtureHash != evidence.FixtureHash {
			continue
		}
		if prior.IdentityHash == evidence.IdentityHash {
			return prior, nil
		}
		return IngestionFreshnessEvidence{}, ErrIngestionFreshnessReplayConflict
	}
	s.created = append(s.created, evidence)
	return evidence, nil
}

func TestEvaluateIngestionFreshnessRejectsRegressingStageTimes(t *testing.T) {
	base := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	input := IngestionFreshnessInput{
		Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		ReportKey:  "fixture-regression",
		ObservedAt: base.Add(time.Minute),
		Policy:     IngestionFreshnessPolicy{ContractVersion: IngestionFreshnessVersion, AdmissionWindow: time.Minute, GovernanceWindow: time.Minute, RetrievalWindow: time.Minute},
		Stages:     []IngestionFreshnessStage{{Kind: IngestionStageAdmitted, Disposition: IngestionDispositionComplete, At: base}, {Kind: IngestionStageGovernanceCompleted, Disposition: IngestionDispositionComplete, At: base.Add(-time.Second)}},
	}
	if _, err := EvaluateIngestionFreshness(input); err == nil {
		t.Fatal("regressing stage timestamps were accepted")
	}
}

func TestEvaluateIngestionFreshnessRejectsUnboundedOrInvalidInput(t *testing.T) {
	base := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	valid := IngestionFreshnessInput{
		Scope:      memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		ReportKey:  "fixture-a",
		ObservedAt: base.Add(time.Minute),
		Policy:     IngestionFreshnessPolicy{ContractVersion: IngestionFreshnessVersion, AdmissionWindow: time.Minute, GovernanceWindow: time.Minute, RetrievalWindow: time.Minute},
		Stages:     []IngestionFreshnessStage{{Kind: IngestionStageAdmitted, Disposition: IngestionDispositionComplete, At: base}},
	}
	cases := []struct {
		name   string
		mutate func(*IngestionFreshnessInput)
	}{
		{name: "missing fixture key", mutate: func(i *IngestionFreshnessInput) { i.ReportKey = " " }},
		{name: "oversized fixture key", mutate: func(i *IngestionFreshnessInput) { i.ReportKey = strings.Repeat("x", 129) }},
		{name: "unknown stage", mutate: func(i *IngestionFreshnessInput) { i.Stages[0].Kind = "other" }},
		{name: "unknown disposition", mutate: func(i *IngestionFreshnessInput) { i.Stages[0].Disposition = "success" }},
		{name: "missing completed timestamp", mutate: func(i *IngestionFreshnessInput) { i.Stages[0].At = time.Time{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := valid
			input.Stages = append([]IngestionFreshnessStage(nil), valid.Stages...)
			tc.mutate(&input)
			if _, err := EvaluateIngestionFreshness(input); err == nil {
				t.Fatal("invalid input was accepted")
			}
		})
	}
}

func TestIngestionFreshnessReportJSONOmitsExactScopeAndInternalIdentity(t *testing.T) {
	base := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	got, err := EvaluateIngestionFreshness(IngestionFreshnessInput{
		Scope:      memory.Scope{Tenant: "private-tenant", Project: "private-project", Namespace: "private-namespace"},
		ObservedAt: base.Add(time.Minute),
		ReportKey:  "internal-event-id",
		Policy:     IngestionFreshnessPolicy{ContractVersion: IngestionFreshnessVersion, AdmissionWindow: time.Minute, GovernanceWindow: time.Minute, RetrievalWindow: time.Minute},
		Stages:     []IngestionFreshnessStage{{Kind: IngestionStageAdmitted, Disposition: IngestionDispositionComplete, At: base}},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-tenant", "private-project", "private-namespace", "internal-event-id"} {
		if string(data) == "" || containsBytes(data, secret) {
			t.Fatalf("serialized report disclosed %q: %s", secret, data)
		}
	}
}

func TestIngestionFreshnessReportRejectsUnknownClosedBuckets(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	report, err := EvaluateIngestionFreshness(IngestionFreshnessInput{
		Scope: scope, ReportKey: "fixture-report-validate", ObservedAt: time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC),
		Policy: IngestionFreshnessPolicy{ContractVersion: IngestionFreshnessVersion, AdmissionWindow: time.Hour, GovernanceWindow: time.Hour, RetrievalWindow: time.Hour},
		Stages: []IngestionFreshnessStage{{Kind: IngestionStageAdmitted, Disposition: IngestionDispositionComplete, At: time.Date(2026, 10, 30, 7, 59, 57, 0, time.UTC)}, {Kind: IngestionStageGovernanceCompleted, Disposition: IngestionDispositionComplete, At: time.Date(2026, 10, 30, 7, 59, 58, 0, time.UTC)}, {Kind: IngestionStageCanonicalActive, Disposition: IngestionDispositionComplete, At: time.Date(2026, 10, 30, 7, 59, 59, 0, time.UTC)}, {Kind: IngestionStagePublicRetrievable, Disposition: IngestionDispositionComplete, At: time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	report.Stages[0].Freshness = "ad_hoc"
	if err := report.Validate(); err == nil {
		t.Fatal("unknown freshness bucket was accepted")
	}
}

func TestEvaluateIngestionFreshnessEmitsBoundedTelemetry(t *testing.T) {
	observer := telemetry.NewMetricsObserver()
	scope := memory.Scope{Tenant: "private-tenant", Project: "private-project", Namespace: "private-namespace"}
	base := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	service := NewService(ServiceOptions{Observer: observer})
	if _, err := service.EvaluateIngestionFreshness(context.Background(), IngestionFreshnessInput{
		Scope: scope, ReportKey: "private-fixture-reference", ObservedAt: base.Add(time.Second),
		Policy: IngestionFreshnessPolicy{ContractVersion: IngestionFreshnessVersion, AdmissionWindow: time.Hour, GovernanceWindow: time.Hour, RetrievalWindow: time.Hour},
		Stages: []IngestionFreshnessStage{{Kind: IngestionStageAdmitted, Disposition: IngestionDispositionComplete, At: base}},
	}); err != nil {
		t.Fatal(err)
	}
	metrics := observer.RenderPrometheus()
	want := `stele_operations_total{component="ingestion_freshness",mode="assurance",operation="evaluate",status="degraded"} 1`
	if !strings.Contains(metrics, want) {
		t.Fatalf("metrics missing %q\n%s", want, metrics)
	}
	for _, forbidden := range []string{scope.Tenant, scope.Project, scope.Namespace, "private-fixture-reference"} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("metrics disclosed %q: %s", forbidden, metrics)
		}
	}
}

func containsBytes(data []byte, value string) bool {
	for i := 0; i+len(value) <= len(data); i++ {
		if string(data[i:i+len(value)]) == value {
			return true
		}
	}
	return false
}
