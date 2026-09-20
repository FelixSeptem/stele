package retrieval

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestSearchExcludesExpiredCandidateBeforeFusion(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	now := time.Now().UTC()
	expiredAt := now.Add(-time.Hour)
	service := NewService(ServiceDependencies{Lexical: &stubLexicalSource{hits: []ScoredMemory{
		{Memory: memory.CanonicalMemory{ID: "expired", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, TemporalValidity: memory.TemporalValidity{TemporalFactID: "fact", IngestedAt: now.Add(-2 * time.Hour), ValidFrom: now.Add(-2 * time.Hour), ValidTo: &expiredAt, ValiditySource: memory.TemporalValiditySourceExplicit}}, LexicalScore: 10},
		// The current version supersedes the expired one for the same fact.
		{Memory: memory.CanonicalMemory{ID: "current", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, TemporalValidity: memory.TemporalValidity{TemporalFactID: "fact", IngestedAt: now.Add(-time.Hour), ValidFrom: expiredAt, ValiditySource: memory.TemporalValiditySourceExplicit}}, LexicalScore: 1},
	}}})
	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "fact", TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 1 || result.Hits[0].Memory.ID != "current" {
		t.Fatalf("hits = %+v, want current only", result.Hits)
	}

	// The excluded version must be attributed to a bounded category so operators
	// can tell "superseded by a newer version" from "malformed snapshot".
	report := result.TemporalOmissionReport()
	if err := report.Validate(); err != nil {
		t.Fatalf("TemporalOmissionReport() invalid: %v", err)
	}
	if report.Total != 1 || len(report.Categories) != 1 {
		t.Fatalf("omission report = %+v, want exactly one recorded omission", report)
	}
	if report.Categories[0].Category != TemporalOmissionExpiredVersion {
		t.Fatalf("category = %q, want %q", report.Categories[0].Category, TemporalOmissionExpiredVersion)
	}
}

// TestSearchReportsUnsupportedClassOmission proves a non-factual class is
// attributed separately from an expired fact, so diagnostics never conflate
// "derived artifact" with "stale fact".
func TestSearchReportsUnsupportedClassOmission(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	now := time.Now().UTC()
	closed := now.Add(-time.Hour)
	service := NewService(ServiceDependencies{Lexical: &stubLexicalSource{hits: []ScoredMemory{
		{Memory: memory.CanonicalMemory{
			ID: "relation", Scope: scope, Class: memory.MemoryClassRelation, State: memory.MemoryStateActive,
			TemporalValidity: memory.TemporalValidity{TemporalFactID: "fact", IngestedAt: now.Add(-2 * time.Hour), ValidFrom: now.Add(-2 * time.Hour), ValidTo: &closed, ValiditySource: memory.TemporalValiditySourceExplicit},
		}, LexicalScore: 5},
	}}})

	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "fact", TopK: 10, IncludeRelations: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 0 {
		t.Fatalf("hits = %+v, want none", result.Hits)
	}
	report := result.TemporalOmissionReport()
	if report.Total != 1 {
		t.Fatalf("omission report = %+v, want one omission", report)
	}
	if report.Categories[0].Category != TemporalOmissionUnsupportedClass {
		t.Fatalf("category = %q, want %q", report.Categories[0].Category, TemporalOmissionUnsupportedClass)
	}
}

// TestSearchWithoutOmissionsReportsNothing keeps the happy path clean: a search
// that filters nothing must not fabricate omission counts.
func TestSearchWithoutOmissionsReportsNothing(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	service := NewService(ServiceDependencies{Lexical: &stubLexicalSource{hits: []ScoredMemory{
		{Memory: memory.CanonicalMemory{ID: "current", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive}, LexicalScore: 3},
	}}})

	result, err := service.Search(context.Background(), SearchInput{Scope: scope, Query: "fact", TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 1 {
		t.Fatalf("hits = %+v, want the retrievable version", result.Hits)
	}
	report := result.TemporalOmissionReport()
	if report.Total != 0 || len(report.Categories) != 0 {
		t.Fatalf("omission report = %+v, want empty", report)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("empty report must validate: %v", err)
	}
}

func TestSearchValidatesExplicitHistoricalConstraint(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	err := (SearchInput{Scope: scope, Query: "fact", TemporalConstraint: memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf}}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want missing as_of rejection")
	}
}
