package assurance

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

type stubProviderFixtureExecutor struct {
	outcomes map[ProviderFixtureKind]ProviderFixtureOutcome
	bindings []provider.RuntimeBinding
	calls    int
}

func (s *stubProviderFixtureExecutor) ExecuteProviderFixture(_ context.Context, binding provider.RuntimeBinding, fixture ProviderFixture) (ProviderFixtureOutcome, error) {
	s.calls++
	s.bindings = append(s.bindings, binding)
	return s.outcomes[fixture.Kind], nil
}

func providerProfile(scope memory.Scope) ProviderConformanceProfile {
	return ProviderConformanceProfile{
		ProfileID:     "profile_provider",
		Scope:         scope,
		SchemaVersion: "schema-v1",
		Fixtures: []ProviderFixture{
			{ID: "capability", Kind: ProviderFixtureCapability, Operation: "capability", Scope: scope, RequiredEvidence: []ProviderEvidenceKind{ProviderEvidenceCompatibility}},
			{ID: "scope", Kind: ProviderFixtureScope, Operation: "retrieve", Scope: scope, RequiredEvidence: []ProviderEvidenceKind{ProviderEvidenceScope}},
			{ID: "replay", Kind: ProviderFixtureReplay, Operation: "ingest", Scope: scope, RequiredEvidence: []ProviderEvidenceKind{ProviderEvidenceReplay}},
			{ID: "lifecycle", Kind: ProviderFixtureLifecycle, Operation: "retrieve", Scope: scope, RequiredEvidence: []ProviderEvidenceKind{ProviderEvidenceLifecycle}},
			{ID: "citation", Kind: ProviderFixtureCitation, Operation: "context", Scope: scope, RequiredEvidence: []ProviderEvidenceKind{ProviderEvidenceCitation}},
			{ID: "restart", Kind: ProviderFixtureRestartFallback, Operation: "ingest", Scope: scope, RequiredEvidence: []ProviderEvidenceKind{ProviderEvidenceRestart}},
			{ID: "freshness", Kind: ProviderFixtureFreshness, Operation: "context", Scope: scope, RequiredEvidence: []ProviderEvidenceKind{ProviderEvidenceFreshness}},
		},
	}
}

func TestProviderConformanceProfileRejectsUnsupportedEvidenceAndForeignScope(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	profile := providerProfile(scope)
	profile.Fixtures[0].RequiredEvidence = []ProviderEvidenceKind{"raw_payload"}
	if err := profile.Validate(); err == nil {
		t.Fatal("unsupported provider evidence accepted")
	}
	profile = providerProfile(scope)
	profile.Fixtures[0].Scope.Tenant = "tenant-b"
	if err := profile.Validate(); err == nil {
		t.Fatal("foreign fixture scope accepted")
	}
}

func TestServiceRunsProviderFixturesInExactScopeAndPersistsBoundedOutcome(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	profile := providerProfile(scope)
	store := &stubAssuranceStore{conformanceProfiles: []ConformanceProfile{{ID: profile.ProfileID, Scope: scope, Status: ConformanceProfileStatusActive, ExpectedEvidence: []ExpectedEvidence{{Kind: ExpectedEvidenceProof, MinimumCount: 1, FreshnessWindow: time.Hour}}, Actor: "operator", Reason: "provider check", CreatedAt: now, UpdatedAt: now}}}
	executor := &stubProviderFixtureExecutor{outcomes: map[ProviderFixtureKind]ProviderFixtureOutcome{}}
	for _, fixture := range profile.Fixtures {
		executor.outcomes[fixture.Kind] = ProviderFixtureOutcome{Passed: true, Evidence: fixture.RequiredEvidence, References: []string{"evidence-1"}}
	}
	service := NewService(ServiceOptions{Store: store, ProviderConformance: executor, Now: func() time.Time { return now }, NewID: func(prefix string) string { return prefix + "_1" }})
	run, diagnostics, err := service.RunProviderConformance(context.Background(), ProviderConformanceRunInput{Profile: profile, Binding: provider.RuntimeBinding{BindingID: "rb-1", PrincipalID: "principal-1", Scope: scope, AgentID: "agent-1", SessionID: "session-1", ProviderInstanceID: "instance-1", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}, Dependencies: ProviderDependencySnapshot{Compatible: true, PostgreSQLReady: true, ProjectionFresh: true, WorkerReady: true}, StartedAt: now})
	if err != nil {
		t.Fatalf("RunProviderConformance() error = %v", err)
	}
	if run.Result != ConformanceResultPassed || len(diagnostics) != 0 || executor.calls != len(profile.Fixtures) {
		t.Fatalf("run=%+v diagnostics=%+v calls=%d", run, diagnostics, executor.calls)
	}
	for _, binding := range executor.bindings {
		if binding.Scope != scope {
			t.Fatalf("executor binding scope = %+v, want %+v", binding.Scope, scope)
		}
	}
	providerEvidence, ok := run.EvidenceCounts["provider"].(map[string]any)
	if !ok || providerEvidence["schema_version"] != "schema-v1" || providerEvidence["fixture_count"] != len(profile.Fixtures) {
		t.Fatalf("provider evidence = %#v", run.EvidenceCounts["provider"])
	}
	if len(store.createdConformanceRuns) != 1 {
		t.Fatalf("persisted runs = %d, want 1", len(store.createdConformanceRuns))
	}
}

func TestServiceProviderConformanceCannotClaimReadyWhenDependencyIsDegraded(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	profile := providerProfile(scope)
	store := &stubAssuranceStore{conformanceProfiles: []ConformanceProfile{{ID: profile.ProfileID, Scope: scope, Status: ConformanceProfileStatusActive, ExpectedEvidence: []ExpectedEvidence{{Kind: ExpectedEvidenceProof, MinimumCount: 1, FreshnessWindow: time.Hour}}, Actor: "operator", Reason: "provider check", CreatedAt: now, UpdatedAt: now}}}
	executor := &stubProviderFixtureExecutor{outcomes: map[ProviderFixtureKind]ProviderFixtureOutcome{}}
	service := NewService(ServiceOptions{Store: store, ProviderConformance: executor, Now: func() time.Time { return now }})
	run, diagnostics, err := service.RunProviderConformance(context.Background(), ProviderConformanceRunInput{Profile: profile, Binding: provider.RuntimeBinding{BindingID: "rb-1", PrincipalID: "principal-1", Scope: scope, AgentID: "agent-1", SessionID: "session-1", ProviderInstanceID: "instance-1", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}, Dependencies: ProviderDependencySnapshot{Compatible: true, PostgreSQLReady: true, ProjectionFresh: false, WorkerReady: true}, StartedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if run.Result != ConformanceResultDegraded || len(diagnostics) == 0 || executor.calls != 0 {
		t.Fatalf("run=%+v diagnostics=%+v calls=%d", run, diagnostics, executor.calls)
	}
	if diagnostics[0].Category != MissingEvidenceStale || diagnostics[0].ReadinessImpact != ReadinessStatusDegraded {
		t.Fatalf("diagnostic=%+v", diagnostics[0])
	}
}

func TestServiceProviderConformanceFailsClosedOnHiddenOrForeignEvidence(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	profile := providerProfile(scope)
	store := &stubAssuranceStore{conformanceProfiles: []ConformanceProfile{{ID: profile.ProfileID, Scope: scope, Status: ConformanceProfileStatusActive, ExpectedEvidence: []ExpectedEvidence{{Kind: ExpectedEvidenceProof, MinimumCount: 1, FreshnessWindow: time.Hour}}, Actor: "operator", Reason: "provider check", CreatedAt: now, UpdatedAt: now}}}
	executor := &stubProviderFixtureExecutor{outcomes: map[ProviderFixtureKind]ProviderFixtureOutcome{ProviderFixtureScope: {Passed: false, OutOfScope: true, References: []string{"foreign-secret"}}, ProviderFixtureLifecycle: {Passed: false, Hidden: true, References: []string{"hidden-secret"}}}}
	service := NewService(ServiceOptions{Store: store, ProviderConformance: executor, Now: func() time.Time { return now }})
	run, diagnostics, err := service.RunProviderConformance(context.Background(), ProviderConformanceRunInput{Profile: profile, Binding: provider.RuntimeBinding{BindingID: "rb-1", PrincipalID: "principal-1", Scope: scope, AgentID: "agent-1", SessionID: "session-1", ProviderInstanceID: "instance-1", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}, Dependencies: ProviderDependencySnapshot{Compatible: true, PostgreSQLReady: true, ProjectionFresh: true, WorkerReady: true}, StartedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if run.Result != ConformanceResultFailed {
		t.Fatalf("result=%s, want failed", run.Result)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Metadata["reference"] == "foreign-secret" || diagnostic.Metadata["reference"] == "hidden-secret" {
			t.Fatalf("diagnostic leaked hidden reference: %+v", diagnostic)
		}
	}
}

func TestServiceProviderConformanceRerunLinksPriorHistory(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	profile := providerProfile(scope)
	store := &stubAssuranceStore{conformanceProfiles: []ConformanceProfile{{ID: profile.ProfileID, Scope: scope, Status: ConformanceProfileStatusActive, ExpectedEvidence: []ExpectedEvidence{{Kind: ExpectedEvidenceProof, MinimumCount: 1, FreshnessWindow: time.Hour}}, Actor: "operator", Reason: "provider check", CreatedAt: now, UpdatedAt: now}}, conformanceRuns: []ConformanceRun{{ID: "prior-run", ProfileID: profile.ProfileID, Scope: scope, Result: ConformanceResultPassed, StartedAt: now.Add(-time.Hour), CreatedAt: now.Add(-time.Hour)}}}
	executor := &stubProviderFixtureExecutor{outcomes: map[ProviderFixtureKind]ProviderFixtureOutcome{}}
	for _, fixture := range profile.Fixtures {
		executor.outcomes[fixture.Kind] = ProviderFixtureOutcome{Passed: true, Evidence: fixture.RequiredEvidence}
	}
	service := NewService(ServiceOptions{Store: store, ProviderConformance: executor, Now: func() time.Time { return now }, NewID: func(prefix string) string { return prefix + "_new" }})
	run, _, err := service.RunProviderConformance(context.Background(), ProviderConformanceRunInput{Profile: profile, Binding: provider.RuntimeBinding{BindingID: "rb-1", PrincipalID: "principal-1", Scope: scope, AgentID: "agent-1", SessionID: "session-1", ProviderInstanceID: "instance-1", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}, Dependencies: ProviderDependencySnapshot{Compatible: true, PostgreSQLReady: true, ProjectionFresh: true, WorkerReady: true}, StartedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	providerEvidence := run.EvidenceCounts["provider"].(map[string]any)
	if providerEvidence["previous_run_id"] != "prior-run" {
		t.Fatalf("previous_run_id=%v", providerEvidence["previous_run_id"])
	}
}
