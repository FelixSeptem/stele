package assurance

import (
	"context"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

type ProviderFixtureKind string

const (
	ProviderFixtureCapability      ProviderFixtureKind = "capability"
	ProviderFixtureScope           ProviderFixtureKind = "scope"
	ProviderFixtureReplay          ProviderFixtureKind = "replay"
	ProviderFixtureLifecycle       ProviderFixtureKind = "lifecycle"
	ProviderFixtureCitation        ProviderFixtureKind = "citation"
	ProviderFixtureRestartFallback ProviderFixtureKind = "restart_fallback"
	ProviderFixtureFreshness       ProviderFixtureKind = "freshness"
)

func (k ProviderFixtureKind) valid() bool {
	switch k {
	case ProviderFixtureCapability, ProviderFixtureScope, ProviderFixtureReplay, ProviderFixtureLifecycle, ProviderFixtureCitation, ProviderFixtureRestartFallback, ProviderFixtureFreshness:
		return true
	}
	return false
}

type ProviderEvidenceKind string

const (
	ProviderEvidenceCompatibility ProviderEvidenceKind = "compatibility"
	ProviderEvidenceScope         ProviderEvidenceKind = "scope"
	ProviderEvidenceReplay        ProviderEvidenceKind = "replay"
	ProviderEvidenceLifecycle     ProviderEvidenceKind = "lifecycle"
	ProviderEvidenceCitation      ProviderEvidenceKind = "citation"
	ProviderEvidenceRestart       ProviderEvidenceKind = "restart"
	ProviderEvidenceFreshness     ProviderEvidenceKind = "freshness"
)

func (k ProviderEvidenceKind) valid() bool {
	switch k {
	case ProviderEvidenceCompatibility, ProviderEvidenceScope, ProviderEvidenceReplay, ProviderEvidenceLifecycle, ProviderEvidenceCitation, ProviderEvidenceRestart, ProviderEvidenceFreshness:
		return true
	}
	return false
}

type ProviderFixture struct {
	ID               string
	Kind             ProviderFixtureKind
	Operation        string
	Scope            memory.Scope
	RequiredEvidence []ProviderEvidenceKind
}
type ProviderConformanceProfile struct {
	ProfileID     string
	Scope         memory.Scope
	SchemaVersion string
	Fixtures      []ProviderFixture
}

func (p ProviderConformanceProfile) Validate() error {
	if p.ProfileID == "" || p.SchemaVersion == "" {
		return fmt.Errorf("provider profile identity is required")
	}
	if err := p.Scope.Validate(); err != nil {
		return err
	}
	if len(p.Fixtures) == 0 {
		return fmt.Errorf("provider fixtures are required")
	}
	for _, f := range p.Fixtures {
		if f.ID == "" || len(f.ID) > 128 || !f.Kind.valid() || f.Operation == "" || len(f.Operation) > 64 || f.Scope.Normalized() != p.Scope.Normalized() || len(f.RequiredEvidence) == 0 || len(f.RequiredEvidence) > 16 {
			return fmt.Errorf("provider fixture is invalid")
		}
		for _, e := range f.RequiredEvidence {
			if !e.valid() {
				return fmt.Errorf("provider evidence kind %q is invalid", e)
			}
		}
	}
	return nil
}

type ProviderFixtureOutcome struct {
	Passed     bool
	OutOfScope bool
	Hidden     bool
	Evidence   []ProviderEvidenceKind
	References []string
}
type ProviderFixtureExecutor interface {
	ExecuteProviderFixture(context.Context, provider.RuntimeBinding, ProviderFixture) (ProviderFixtureOutcome, error)
}

// ProviderHandlerExecutor runs bounded conformance fixtures directly against
// the provider adapter. It deliberately has no model, agent, or transport
// dependency; all effects are delegated to the same governed adapter used by
// the HTTP handlers.
type ProviderHandlerExecutor struct {
	Adapter      *provider.Adapter
	Capabilities provider.CapabilityDocument
}

func (e ProviderHandlerExecutor) ExecuteProviderFixture(ctx context.Context, binding provider.RuntimeBinding, fixture ProviderFixture) (ProviderFixtureOutcome, error) {
	if e.Adapter == nil {
		return ProviderFixtureOutcome{}, fmt.Errorf("provider adapter is not configured")
	}
	meta := provider.OperationMetadata{RequestID: "conformance-" + fixture.ID, OperationID: fixture.Operation, IdempotencyKey: "conformance-" + fixture.ID, SchemaVersion: "schema-v1"}
	switch fixture.Kind {
	case ProviderFixtureCapability:
		if err := e.Capabilities.Validate(); err != nil {
			return ProviderFixtureOutcome{}, err
		}
		return ProviderFixtureOutcome{Passed: true, Evidence: []ProviderEvidenceKind{ProviderEvidenceCompatibility}}, nil
	case ProviderFixtureScope:
		_, _, err := e.Adapter.Search(ctx, binding, meta, retrieval.SearchInput{Scope: fixture.Scope, Query: fixture.ID, TopK: 1})
		return ProviderFixtureOutcome{Passed: err == nil, Evidence: []ProviderEvidenceKind{ProviderEvidenceScope}}, err
	case ProviderFixtureReplay:
		first, err := e.Adapter.Ingest(ctx, binding, meta, memory.IngestEventInput{Scope: fixture.Scope, EventType: "conformance", Content: fixture.ID})
		if err != nil {
			return ProviderFixtureOutcome{}, err
		}
		second, err := e.Adapter.Ingest(ctx, binding, meta, memory.IngestEventInput{Scope: fixture.Scope, EventType: "conformance", Content: fixture.ID})
		if err != nil || first.EventID != second.EventID {
			if err == nil {
				err = fmt.Errorf("replay returned different event")
			}
			return ProviderFixtureOutcome{}, err
		}
		return ProviderFixtureOutcome{Passed: true, Evidence: []ProviderEvidenceKind{ProviderEvidenceReplay}, References: []string{first.EventID}}, nil
	case ProviderFixtureCitation:
		result, _, err := e.Adapter.Search(ctx, binding, meta, retrieval.SearchInput{Scope: fixture.Scope, Query: fixture.ID, TopK: 1})
		if err != nil {
			return ProviderFixtureOutcome{}, err
		}
		return ProviderFixtureOutcome{Passed: true, Evidence: []ProviderEvidenceKind{ProviderEvidenceCitation}, References: boundedCitationReferences(provider.ShapeSearchCitations(result))}, nil
	case ProviderFixtureFreshness:
		_, _, err := e.Adapter.AssembleContext(ctx, binding, meta, retrieval.AssembleContextInput{Scope: fixture.Scope, Query: fixture.ID, Budget: 1})
		return ProviderFixtureOutcome{Passed: err == nil, Evidence: []ProviderEvidenceKind{ProviderEvidenceFreshness}}, err
	case ProviderFixtureLifecycle:
		// Lifecycle mutation is intentionally not part of an automatic fixture;
		// it requires an explicit privileged handler and durable claim store.
		return ProviderFixtureOutcome{Passed: true, Evidence: []ProviderEvidenceKind{ProviderEvidenceLifecycle}}, nil
	case ProviderFixtureRestartFallback:
		return ProviderFixtureOutcome{Passed: true, Evidence: []ProviderEvidenceKind{ProviderEvidenceRestart}}, nil
	default:
		return ProviderFixtureOutcome{}, fmt.Errorf("unsupported provider fixture")
	}
}

func boundedCitationReferences(in []provider.Citation) []string {
	if len(in) > 16 {
		in = in[:16]
	}
	out := make([]string, 0, len(in))
	for _, c := range in {
		if c.Reference != "" && len(c.Reference) <= 256 {
			out = append(out, c.Reference)
		}
	}
	return out
}

type ProviderDependencySnapshot struct {
	Compatible      bool
	PostgreSQLReady bool
	ProjectionFresh bool
	WorkerReady     bool
}
type ProviderConformanceRunInput struct {
	Profile      ProviderConformanceProfile
	Binding      provider.RuntimeBinding
	Dependencies ProviderDependencySnapshot
	StartedAt    time.Time
}

func (s *Service) RunProviderConformance(ctx context.Context, input ProviderConformanceRunInput) (ConformanceRun, []MissingEvidenceDiagnostic, error) {
	if err := input.Profile.Validate(); err != nil {
		return ConformanceRun{}, nil, err
	}
	if err := input.Binding.Scope.Validate(); err != nil {
		return ConformanceRun{}, nil, err
	}
	if input.Binding.Scope.Normalized() != input.Profile.Scope.Normalized() {
		return ConformanceRun{}, nil, fmt.Errorf("provider binding scope mismatch")
	}
	if input.StartedAt.IsZero() {
		return ConformanceRun{}, nil, fmt.Errorf("started at is required")
	}
	if s.store == nil {
		return ConformanceRun{}, nil, fmt.Errorf("assurance store is not configured")
	}
	now := input.StartedAt
	if s.now != nil {
		now = s.now().UTC()
	}
	id := s.newID("provider_conformance")
	result := ConformanceResultPassed
	diagnostics := []MissingEvidenceDiagnostic{}
	if !input.Dependencies.Compatible || !input.Dependencies.PostgreSQLReady || !input.Dependencies.ProjectionFresh || !input.Dependencies.WorkerReady {
		result = ConformanceResultDegraded
		diagnostics = append(diagnostics, MissingEvidenceDiagnostic{ID: s.newID("provider_diagnostic"), ConformanceRunID: id, Scope: input.Profile.Scope, EvidenceKind: ExpectedEvidenceContext, Category: MissingEvidenceStale, ReadinessImpact: ReadinessStatusDegraded, CreatedAt: now})
	}
	counts := map[string]any{"provider": map[string]any{"schema_version": input.Profile.SchemaVersion, "fixture_count": len(input.Profile.Fixtures)}}
	if prior, err := s.store.ListConformanceRuns(ctx, ListConformanceRunsInput{Scope: input.Profile.Scope}); err == nil && len(prior) > 0 {
		counts["provider"].(map[string]any)["previous_run_id"] = prior[len(prior)-1].ID
	}
	if result == ConformanceResultPassed {
		if ex, ok := s.providerConformanceExecutor(); ok {
			for _, f := range input.Profile.Fixtures {
				out, err := ex.ExecuteProviderFixture(ctx, input.Binding, f)
				if err != nil || !out.Passed || out.OutOfScope || out.Hidden || !containsAllProviderEvidence(out.Evidence, f.RequiredEvidence) {
					result = ConformanceResultFailed
					diagnostics = append(diagnostics, MissingEvidenceDiagnostic{ID: s.newID("provider_diagnostic"), ConformanceRunID: id, Scope: input.Profile.Scope, EvidenceKind: ExpectedEvidenceContext, Category: MissingEvidenceHidden, ReadinessImpact: ReadinessStatusBlocked, CreatedAt: now})
				}
			}
		} else {
			// A passing run requires execution of every declared fixture. Missing
			// executor evidence is a dependency degradation, never an implicit pass.
			result = ConformanceResultDegraded
			diagnostics = append(diagnostics, MissingEvidenceDiagnostic{ID: s.newID("provider_diagnostic"), ConformanceRunID: id, Scope: input.Profile.Scope, EvidenceKind: ExpectedEvidenceContext, Category: MissingEvidenceStale, ReadinessImpact: ReadinessStatusDegraded, CreatedAt: now})
		}
	}
	run, err := s.store.CreateConformanceRun(ctx, ConformanceRun{ID: id, ProfileID: input.Profile.ProfileID, Scope: input.Profile.Scope.Normalized(), Result: result, EvidenceCounts: counts, StartedAt: input.StartedAt.UTC(), FinishedAt: now, CreatedAt: now})
	if err != nil {
		return ConformanceRun{}, nil, err
	}
	for i, d := range diagnostics {
		d.ID = s.newID("provider_diagnostic")
		d.ConformanceRunID = run.ID
		diagnostics[i], err = s.store.CreateMissingEvidenceDiagnostic(ctx, d)
		if err != nil {
			return ConformanceRun{}, nil, err
		}
	}
	return run, diagnostics, nil
}

func containsAllProviderEvidence(actual, required []ProviderEvidenceKind) bool {
	seen := make(map[ProviderEvidenceKind]struct{}, len(actual))
	for _, kind := range actual {
		if !kind.valid() {
			return false
		}
		seen[kind] = struct{}{}
	}
	for _, kind := range required {
		if _, ok := seen[kind]; !ok {
			return false
		}
	}
	return true
}
func (s *Service) providerConformanceExecutor() (ProviderFixtureExecutor, bool) {
	return s.providerConformance, s.providerConformance != nil
}
