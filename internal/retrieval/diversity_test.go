package retrieval

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestDiversityPolicyValidateRejectsUnboundedParameters(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DiversityPolicy)
	}{
		{name: "unsupported name", mutate: func(policy *DiversityPolicy) { policy.Name = "other" }},
		{name: "empty version", mutate: func(policy *DiversityPolicy) { policy.Version = " " }},
		{name: "version longer than 64", mutate: func(policy *DiversityPolicy) { policy.Version = strings.Repeat("v", 65) }},
		{name: "MMR lambda below zero", mutate: func(policy *DiversityPolicy) { policy.MMRLambda = -0.01 }},
		{name: "MMR lambda above one", mutate: func(policy *DiversityPolicy) { policy.MMRLambda = 1.01 }},
		{name: "MMR lambda NaN", mutate: func(policy *DiversityPolicy) { policy.MMRLambda = math.NaN() }},
		{name: "MMR lambda infinity", mutate: func(policy *DiversityPolicy) { policy.MMRLambda = math.Inf(1) }},
		{name: "semantic threshold below zero", mutate: func(policy *DiversityPolicy) { policy.SemanticThreshold = -0.01 }},
		{name: "semantic threshold above one", mutate: func(policy *DiversityPolicy) { policy.SemanticThreshold = 1.01 }},
		{name: "semantic threshold NaN", mutate: func(policy *DiversityPolicy) { policy.SemanticThreshold = math.NaN() }},
		{name: "semantic threshold infinity", mutate: func(policy *DiversityPolicy) { policy.SemanticThreshold = math.Inf(1) }},
		{name: "candidate limit below minimum", mutate: func(policy *DiversityPolicy) { policy.MaxCandidates = 0 }},
		{name: "candidate limit above maximum", mutate: func(policy *DiversityPolicy) { policy.MaxCandidates = 5001 }},
		{name: "pairwise limit below minimum", mutate: func(policy *DiversityPolicy) { policy.MaxPairwiseComparisons = 0 }},
		{name: "pairwise limit above maximum", mutate: func(policy *DiversityPolicy) { policy.MaxPairwiseComparisons = 100001 }},
		{name: "embedding dimensions below minimum", mutate: func(policy *DiversityPolicy) { policy.MaxEmbeddingDimensions = 0 }},
		{name: "embedding dimensions above maximum", mutate: func(policy *DiversityPolicy) { policy.MaxEmbeddingDimensions = 4097 }},
		{name: "citation limit below minimum", mutate: func(policy *DiversityPolicy) { policy.MaxCitationsPerCandidate = 0 }},
		{name: "citation limit above maximum", mutate: func(policy *DiversityPolicy) { policy.MaxCitationsPerCandidate = 65 }},
		{name: "memory class weight below zero", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.MemoryClass = -0.01 }},
		{name: "memory class weight above one", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.MemoryClass = 1.01 }},
		{name: "memory class weight NaN", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.MemoryClass = math.NaN() }},
		{name: "memory class weight infinity", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.MemoryClass = math.Inf(1) }},
		{name: "session weight below zero", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.Session = -0.01 }},
		{name: "session weight above one", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.Session = 1.01 }},
		{name: "session weight NaN", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.Session = math.NaN() }},
		{name: "session weight infinity", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.Session = math.Inf(1) }},
		{name: "entity weight below zero", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.Entity = -0.01 }},
		{name: "entity weight above one", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.Entity = 1.01 }},
		{name: "entity weight NaN", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.Entity = math.NaN() }},
		{name: "entity weight infinity", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.Entity = math.Inf(1) }},
		{name: "time slice weight below zero", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.TimeSlice = -0.01 }},
		{name: "time slice weight above one", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.TimeSlice = 1.01 }},
		{name: "time slice weight NaN", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.TimeSlice = math.NaN() }},
		{name: "time slice weight infinity", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.TimeSlice = math.Inf(1) }},
		{name: "unknown weight below zero", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.Unknown = -0.01 }},
		{name: "unknown weight above one", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.Unknown = 1.01 }},
		{name: "unknown weight NaN", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.Unknown = math.NaN() }},
		{name: "unknown weight infinity", mutate: func(policy *DiversityPolicy) { policy.CoverageWeights.Unknown = math.Inf(1) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := DefaultDiversityPolicy()
			tc.mutate(&policy)
			if err := policy.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want bounded parameter error")
			}
		})
	}
}

func TestDiversityPolicyValidateReportsCoverageErrorsInStableOrder(t *testing.T) {
	policy := DefaultDiversityPolicy()
	policy.CoverageWeights = DiversityCoverageWeights{
		MemoryClass: -1,
		Session:     -1,
		Entity:      -1,
		TimeSlice:   -1,
		Unknown:     -1,
	}
	for attempt := 0; attempt < 100; attempt++ {
		err := policy.Validate()
		if err == nil || err.Error() != "memory class coverage weight must be between 0 and 1" {
			t.Fatalf("Validate() error = %v, want deterministic memory class error", err)
		}
	}
}

func TestDeduplicateDiversityCandidatesPreservesFusedOrderAndMergesBoundedCitations(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	first := diversityCandidate(scope, "mem-first", 0.1, "event-shared", "", nil)
	first.Citations = []Citation{{MemoryID: "mem-first", RawEventID: "event-a", Operation: "lexical"}}
	second := diversityCandidate(scope, "mem-second", 99, "event-shared", "", nil)
	second.Citations = []Citation{
		{MemoryID: "mem-first", RawEventID: "event-a", Operation: "lexical"},
		{MemoryID: "mem-second", RawEventID: "event-b", Operation: "semantic"},
	}

	result := DeduplicateDiversityCandidates(scope, []DiversityCandidate{first, second})
	if len(result.Candidates) != 1 || result.Candidates[0].Memory.ID != "mem-first" {
		t.Fatalf("candidates = %+v, want first fused candidate as representative", result.Candidates)
	}
	wantCitations := []Citation{
		{MemoryID: "mem-first", RawEventID: "event-a", Operation: "lexical"},
		{MemoryID: "mem-second", RawEventID: "event-b", Operation: "semantic"},
	}
	if !reflect.DeepEqual(result.Candidates[0].Citations, wantCitations) {
		t.Fatalf("citations = %+v, want %+v", result.Candidates[0].Citations, wantCitations)
	}
}

func TestDeduplicateDiversityCandidatesUnionsTransitiveLineage(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	items := []DiversityCandidate{
		diversityCandidate(scope, "mem-first", 0.1, "event-shared", "", nil),
		diversityCandidate(scope, "mem-bridge", 2, "event-shared", "parent-shared", nil),
		diversityCandidate(scope, "mem-last", 3, "event-other", "parent-shared", nil),
	}

	result := DeduplicateDiversityCandidates(scope, items)
	if len(result.Candidates) != 1 || result.Candidates[0].Memory.ID != "mem-first" {
		t.Fatalf("candidates = %+v, want one transitive group represented by first fused candidate", result.Candidates)
	}
	if result.Dispositions.Duplicate != 2 {
		t.Fatalf("duplicate disposition = %d, want 2", result.Dispositions.Duplicate)
	}
}

func TestDeduplicateDiversityCandidatesKeepsStableFirstLineageRepresentative(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	items := []DiversityCandidate{
		diversityCandidate(scope, "mem-low", 0.2, "event-1", "", nil),
		diversityCandidate(scope, "mem-high", 0.9, "event-1", "", nil),
		diversityCandidate(scope, "mem-independent", 0.8, "event-2", "", nil),
	}

	result := DeduplicateDiversityCandidates(scope, items)
	if len(result.Candidates) != 2 {
		t.Fatalf("candidates = %+v, want 2", result.Candidates)
	}
	if result.Candidates[0].Memory.ID != "mem-low" || result.Candidates[1].Memory.ID != "mem-independent" {
		t.Fatalf("candidate ids = %q, %q; want stable input order mem-low, mem-independent", result.Candidates[0].Memory.ID, result.Candidates[1].Memory.ID)
	}
	if result.Dispositions.Duplicate != 1 || result.Dispositions.Invalid != 0 {
		t.Fatalf("dispositions = %+v, want one duplicate and no invalid", result.Dispositions)
	}
}

func TestDeduplicateDiversityCandidatesBoundsMergedCitations(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	first := diversityCandidate(scope, "mem-first", 1, "event-shared", "", nil)
	second := diversityCandidate(scope, "mem-second", 0.9, "event-shared", "", nil)
	for index := 0; index < defaultMaxCitationsPerCandidate; index++ {
		first.Citations = append(first.Citations, Citation{MemoryID: "mem-first", RawEventID: "event-a", Operation: string(rune('a' + index))})
	}
	second.Citations = []Citation{{MemoryID: "mem-second", RawEventID: "event-b", Operation: "overflow"}}

	result := DeduplicateDiversityCandidates(scope, []DiversityCandidate{first, second})
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want one representative", result.Candidates)
	}
	if got := len(result.Candidates[0].Citations); got != defaultMaxCitationsPerCandidate {
		t.Fatalf("citation count = %d, want bounded %d", got, defaultMaxCitationsPerCandidate)
	}
}

func TestDeduplicateDiversityCandidatesDoesNotCollideOnNULInCitationFields(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	representative := diversityCandidate(scope, "mem-first", 1, "event-shared", "", nil)
	representative.Citations = []Citation{{MemoryID: "memory\x00event", RawEventID: "raw", Operation: "operation"}}
	duplicate := diversityCandidate(scope, "mem-second", 0.9, "event-shared", "", nil)
	duplicate.Citations = []Citation{{MemoryID: "memory", RawEventID: "event\x00raw", Operation: "operation"}}

	result := DeduplicateDiversityCandidates(scope, []DiversityCandidate{representative, duplicate})
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want one representative", result.Candidates)
	}
	if got := len(result.Candidates[0].Citations); got != 2 {
		t.Fatalf("citation count = %d, want both structurally distinct citations", got)
	}
}

func TestSelectDiverseCandidatesPrioritizesIndependentCompatibleEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	items := []DiversityCandidate{
		diversityCandidate(scope, "mem-a", 1.0, "event-a", "", []float32{1, 0}),
		diversityCandidate(scope, "mem-a-near", 0.9, "event-b", "", []float32{0.8, 0.6}),
		diversityCandidate(scope, "mem-b", 0.8, "event-c", "", []float32{0, 1}),
	}
	for index := range items {
		items[index].EmbeddingRevision = "embedding-v1"
		items[index].EmbeddingRevisionActive = true
	}
	policy := DefaultDiversityPolicy()
	policy.SemanticThreshold = 0.9

	result, err := SelectDiverseCandidates(scope, policy, items, 2)
	if err != nil {
		t.Fatalf("SelectDiverseCandidates() error = %v", err)
	}
	if len(result.Candidates) != 2 || result.Candidates[0].Memory.ID != "mem-a" || result.Candidates[1].Memory.ID != "mem-b" {
		t.Fatalf("candidate ids = %+v, want independent mem-a and mem-b", result.Candidates)
	}
	if result.Dispositions.Diversity != 1 {
		t.Fatalf("dispositions = %+v, want one diversity omission", result.Dispositions)
	}
}

func TestSelectDiverseCandidatesDegradesToIdentityDedupWhenEmbeddingsAreIncompatible(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	items := []DiversityCandidate{
		diversityCandidate(scope, "mem-a", 1.0, "event-a", "", []float32{1, 0}),
		diversityCandidate(scope, "mem-b", 0.9, "event-b", "", []float32{0.99, 0.01}),
	}
	items[0].EmbeddingRevision = "embedding-v1"
	items[1].EmbeddingRevision = "embedding-v2"
	items[0].EmbeddingRevisionActive = true
	items[1].EmbeddingRevisionActive = true

	result, err := SelectDiverseCandidates(scope, DefaultDiversityPolicy(), items, 2)
	if err != nil {
		t.Fatalf("SelectDiverseCandidates() error = %v", err)
	}
	if len(result.Candidates) != 2 || result.Candidates[1].Memory.ID != "mem-b" {
		t.Fatalf("candidate ids = %+v, want stable identity-deduplicated order", result.Candidates)
	}
	if result.Dispositions.SemanticUnavailable != 1 {
		t.Fatalf("dispositions = %+v, want semantic unavailable status", result.Dispositions)
	}
}

func TestSelectDiverseCandidatesFormsTransitiveSemanticClustersAtInclusiveThreshold(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	items := []DiversityCandidate{
		diversityCandidate(scope, "mem-a", 1.0, "event-a", "", []float32{1, 0}),
		diversityCandidate(scope, "mem-b", 0.9, "event-b", "", []float32{0.8660254, 0.5}),
		diversityCandidate(scope, "mem-c", 0.8, "event-c", "", []float32{0.5, 0.8660254}),
	}
	for index := range items {
		items[index].EmbeddingRevision = "embedding-v1"
		items[index].EmbeddingRevisionActive = true
		items[index].Citations = []Citation{{MemoryID: items[index].Memory.ID, Operation: "semantic"}}
	}
	policy := DefaultDiversityPolicy()
	policy.SemanticThreshold = 0.86

	result, err := SelectDiverseCandidates(scope, policy, items, 3)
	if err != nil {
		t.Fatalf("SelectDiverseCandidates() error = %v", err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Memory.ID != "mem-a" {
		t.Fatalf("candidates = %+v, want stable representative mem-a", result.Candidates)
	}
	if got := len(result.Candidates[0].Citations); got != 3 {
		t.Fatalf("citation count = %d, want citations retained from all semantic-cluster members", got)
	}
	if result.Dispositions.Diversity != 2 || result.PairwiseComparisons != 3 {
		t.Fatalf("selection = %+v, want two clustered omissions and three comparisons", result)
	}
}

func TestSelectDiverseCandidatesSemanticThresholdBoundary(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	tests := []struct {
		name       string
		second     []float32
		wantLength int
	}{
		{name: "equal similarity clusters", second: []float32{1, 0}, wantLength: 1},
		{name: "below threshold remains independent", second: []float32{1, 0.01}, wantLength: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items := []DiversityCandidate{
				diversityCandidate(scope, "mem-a", 1, "event-a", "", []float32{1, 0}),
				diversityCandidate(scope, "mem-b", 0.9, "event-b", "", tc.second),
			}
			for index := range items {
				items[index].EmbeddingRevision = "embedding-v1"
				items[index].EmbeddingRevisionActive = true
			}
			policy := DefaultDiversityPolicy()
			policy.SemanticThreshold = 1

			result, err := SelectDiverseCandidates(scope, policy, items, 2)
			if err != nil {
				t.Fatalf("SelectDiverseCandidates() error = %v", err)
			}
			if len(result.Candidates) != tc.wantLength {
				t.Fatalf("candidate count = %d, want %d", len(result.Candidates), tc.wantLength)
			}
			if result.PairwiseComparisons != 1 {
				t.Fatalf("pairwise comparisons = %d, want 1", result.PairwiseComparisons)
			}
		})
	}
}

func TestSelectDiverseCandidatesRequiresCompatibleActiveEmbeddingRevision(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	tests := []struct {
		name   string
		mutate func([]DiversityCandidate)
	}{
		{name: "inactive revision", mutate: func(items []DiversityCandidate) { items[1].EmbeddingRevisionActive = false }},
		{name: "revision mismatch", mutate: func(items []DiversityCandidate) { items[1].EmbeddingRevision = "embedding-v2" }},
		{name: "missing embedding", mutate: func(items []DiversityCandidate) { items[1].Embedding = nil }},
		{name: "dimension mismatch", mutate: func(items []DiversityCandidate) { items[1].Embedding = []float32{1} }},
		{name: "non-finite embedding", mutate: func(items []DiversityCandidate) { items[1].Embedding[0] = float32(math.NaN()) }},
		{name: "zero norm embedding", mutate: func(items []DiversityCandidate) { items[1].Embedding = []float32{0, 0} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items := []DiversityCandidate{
				diversityCandidate(scope, "mem-a", 1, "event-a", "", []float32{1, 0}),
				diversityCandidate(scope, "mem-a-near", 0.9, "event-b", "", []float32{0.99, 0.01}),
				diversityCandidate(scope, "mem-b", 0.8, "event-c", "", []float32{0, 1}),
			}
			for index := range items {
				items[index].EmbeddingRevision = "embedding-v1"
				items[index].EmbeddingRevisionActive = true
			}
			tc.mutate(items)

			result, err := SelectDiverseCandidates(scope, DefaultDiversityPolicy(), items, 2)
			if err != nil {
				t.Fatalf("SelectDiverseCandidates() error = %v", err)
			}
			if got := diversityCandidateIDs(result.Candidates); !reflect.DeepEqual(got, []string{"mem-a", "mem-a-near"}) {
				t.Fatalf("candidate ids = %v, want identity-only baseline", got)
			}
			if result.Dispositions.SemanticUnavailable != 1 || result.PairwiseComparisons != 0 {
				t.Fatalf("selection = %+v, want bounded semantic-unavailable fallback without partial comparisons", result)
			}
		})
	}
}

func TestSelectDiverseCandidatesDegradesWhenEmbeddingDimensionsExceedPolicyBound(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	items := []DiversityCandidate{
		diversityCandidate(scope, "mem-a", 1, "event-a", "", []float32{1, 0, 0}),
		diversityCandidate(scope, "mem-a-near", 0.9, "event-b", "", []float32{0.99, 0.01, 0}),
		diversityCandidate(scope, "mem-b", 0.8, "event-c", "", []float32{0, 1, 0}),
	}
	for index := range items {
		items[index].EmbeddingRevision = "embedding-v1"
		items[index].EmbeddingRevisionActive = true
	}
	policy := DefaultDiversityPolicy()
	policy.MaxEmbeddingDimensions = 2

	result, err := SelectDiverseCandidates(scope, policy, items, 2)
	if err != nil {
		t.Fatalf("SelectDiverseCandidates() error = %v", err)
	}
	if got := diversityCandidateIDs(result.Candidates); !reflect.DeepEqual(got, []string{"mem-a", "mem-a-near"}) {
		t.Fatalf("candidate ids = %v, want identity-only baseline", got)
	}
	if result.Dispositions.SemanticUnavailable != 1 || result.PairwiseComparisons != 0 {
		t.Fatalf("selection = %+v, want bounded semantic-unavailable fallback", result)
	}
}

func TestSelectDiverseCandidatesAppliesCoverageBonuses(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	tests := []struct {
		name      string
		configure func(*DiversityPolicy, []DiversityCandidate)
	}{
		{name: "memory class", configure: func(policy *DiversityPolicy, items []DiversityCandidate) {
			policy.CoverageWeights.MemoryClass = 1
			items[2].Memory.Class = memory.MemoryClassProfile
		}},
		{name: "session", configure: func(policy *DiversityPolicy, items []DiversityCandidate) {
			policy.CoverageWeights.Session = 1
			items[0].SessionID, items[1].SessionID, items[2].SessionID = "session-a", "session-a", "session-b"
		}},
		{name: "entity", configure: func(policy *DiversityPolicy, items []DiversityCandidate) {
			policy.CoverageWeights.Entity = 1
			items[0].EntityKey, items[1].EntityKey, items[2].EntityKey = "entity-a", "entity-a", "entity-b"
		}},
		{name: "time slice", configure: func(policy *DiversityPolicy, items []DiversityCandidate) {
			policy.CoverageWeights.TimeSlice = 1
			items[0].TimeSlice, items[1].TimeSlice, items[2].TimeSlice = "2026-09", "2026-09", "2026-08"
		}},
		{name: "unknown", configure: func(policy *DiversityPolicy, items []DiversityCandidate) {
			policy.CoverageWeights.Unknown = 1
			items[0].SessionID, items[1].SessionID, items[2].SessionID = "session-a", "session-a", ""
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items := []DiversityCandidate{
				diversityCandidate(scope, "mem-a", 1, "event-a", "", []float32{1, 0, 0}),
				diversityCandidate(scope, "mem-b", 0.9, "event-b", "", []float32{0, 1, 0}),
				diversityCandidate(scope, "mem-c", 0.8, "event-c", "", []float32{0, 0, 1}),
			}
			for index := range items {
				items[index].EmbeddingRevision = "embedding-v1"
				items[index].EmbeddingRevisionActive = true
			}
			policy := DefaultDiversityPolicy()
			policy.MMRLambda = 0.5
			policy.SemanticThreshold = 1
			policy.CoverageWeights = DiversityCoverageWeights{}
			tc.configure(&policy, items)

			result, err := SelectDiverseCandidates(scope, policy, items, 2)
			if err != nil {
				t.Fatalf("SelectDiverseCandidates() error = %v", err)
			}
			if got := diversityCandidateIDs(result.Candidates); !reflect.DeepEqual(got, []string{"mem-a", "mem-c"}) {
				t.Fatalf("candidate ids = %v, want highest relevance then coverage-diverse mem-c", got)
			}
		})
	}
}

func TestSelectDiverseCandidatesUsesStableFusionTieBreak(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	tests := []struct {
		name      string
		configure func([]DiversityCandidate)
		want      string
	}{
		{name: "memory class", configure: func(items []DiversityCandidate) {
			items[1].Memory.Class = memory.MemoryClassRelation
			items[2].Memory.Class = memory.MemoryClassProfile
		}, want: "mem-c"},
		{name: "modified timestamp", configure: func(items []DiversityCandidate) {
			items[2].Memory.ModifiedAt = items[1].Memory.ModifiedAt.Add(time.Second)
		}, want: "mem-c"},
		{name: "canonical id", configure: func(items []DiversityCandidate) {
			items[1].Memory.ID = "mem-z"
			items[2].Memory.ID = "mem-b"
		}, want: "mem-b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items := []DiversityCandidate{
				diversityCandidate(scope, "mem-a", 1, "event-a", "", []float32{1, 0, 0}),
				diversityCandidate(scope, "mem-b", 0.8, "event-b", "", []float32{0, 1, 0}),
				diversityCandidate(scope, "mem-c", 0.8, "event-c", "", []float32{0, 0, 1}),
			}
			tc.configure(items)
			for index := range items {
				items[index].EmbeddingRevision = "embedding-v1"
				items[index].EmbeddingRevisionActive = true
			}
			policy := DefaultDiversityPolicy()
			policy.SemanticThreshold = 1
			policy.CoverageWeights = DiversityCoverageWeights{}

			result, err := SelectDiverseCandidates(scope, policy, items, 2)
			if err != nil {
				t.Fatalf("SelectDiverseCandidates() error = %v", err)
			}
			if got := result.Candidates[1].Memory.ID; got != tc.want {
				t.Fatalf("second candidate id = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSelectDiverseCandidatesEnforcesCandidateAndPairwiseBounds(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	newItems := func() []DiversityCandidate {
		items := []DiversityCandidate{
			diversityCandidate(scope, "mem-a", 1, "event-a", "", []float32{1, 0}),
			diversityCandidate(scope, "mem-a-near", 0.9, "event-b", "", []float32{0.99, 0.01}),
			diversityCandidate(scope, "mem-b", 0.8, "event-c", "", []float32{0, 1}),
		}
		for index := range items {
			items[index].EmbeddingRevision = "embedding-v1"
			items[index].EmbeddingRevisionActive = true
		}
		return items
	}

	t.Run("candidate cap", func(t *testing.T) {
		policy := DefaultDiversityPolicy()
		policy.MaxCandidates = 2
		policy.SemanticThreshold = 1
		result, err := SelectDiverseCandidates(scope, policy, newItems(), 3)
		if err != nil {
			t.Fatalf("SelectDiverseCandidates() error = %v", err)
		}
		if got := diversityCandidateIDs(result.Candidates); !reflect.DeepEqual(got, []string{"mem-a", "mem-a-near"}) {
			t.Fatalf("candidate ids = %v, want capped first two candidates", got)
		}
		if result.Dispositions.Diversity != 1 || result.PairwiseComparisons != 1 {
			t.Fatalf("selection = %+v, want one capped omission and one comparison", result)
		}
	})

	t.Run("exact pairwise budget", func(t *testing.T) {
		policy := DefaultDiversityPolicy()
		policy.MaxPairwiseComparisons = 3
		result, err := SelectDiverseCandidates(scope, policy, newItems(), 2)
		if err != nil {
			t.Fatalf("SelectDiverseCandidates() error = %v", err)
		}
		if got := diversityCandidateIDs(result.Candidates); !reflect.DeepEqual(got, []string{"mem-a", "mem-b"}) {
			t.Fatalf("candidate ids = %v, want semantic selection at exact comparison budget", got)
		}
		if result.PairwiseComparisons != 3 || result.Dispositions.SemanticUnavailable != 0 {
			t.Fatalf("selection = %+v, want exactly three successful comparisons", result)
		}
	})

	t.Run("insufficient pairwise budget", func(t *testing.T) {
		policy := DefaultDiversityPolicy()
		policy.MaxPairwiseComparisons = 2
		result, err := SelectDiverseCandidates(scope, policy, newItems(), 2)
		if err != nil {
			t.Fatalf("SelectDiverseCandidates() error = %v", err)
		}
		if got := diversityCandidateIDs(result.Candidates); !reflect.DeepEqual(got, []string{"mem-a", "mem-a-near"}) {
			t.Fatalf("candidate ids = %v, want complete identity-only fallback", got)
		}
		if result.PairwiseComparisons != 0 || result.Dispositions.SemanticUnavailable != 1 {
			t.Fatalf("selection = %+v, want no partial comparisons and semantic-unavailable status", result)
		}
	})
}

func TestSelectDiverseCandidatesReplaysDeterministically(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	items := []DiversityCandidate{
		diversityCandidate(scope, "mem-a", 1, "event-a", "", []float32{1, 0, 0}),
		diversityCandidate(scope, "mem-b", 0.9, "event-b", "", []float32{0.7, 0.7, 0}),
		diversityCandidate(scope, "mem-c", 0.8, "event-c", "", []float32{0, 1, 0}),
		diversityCandidate(scope, "mem-d", 0.7, "event-d", "", []float32{0, 0, 1}),
	}
	for index := range items {
		items[index].EmbeddingRevision = "embedding-v1"
		items[index].EmbeddingRevisionActive = true
		items[index].SessionID = []string{"session-a", "session-a", "session-b", ""}[index]
	}
	policy := DefaultDiversityPolicy()
	var want DiversitySelection
	for attempt := 0; attempt < 100; attempt++ {
		result, err := SelectDiverseCandidates(scope, policy, items, 3)
		if err != nil {
			t.Fatalf("SelectDiverseCandidates() error = %v", err)
		}
		if attempt == 0 {
			want = result
			continue
		}
		if !reflect.DeepEqual(result, want) {
			t.Fatalf("attempt %d selection = %+v, want %+v", attempt, result, want)
		}
	}
}

func TestSelectDiverseCandidatesDoesNotMutateInput(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	tests := []struct {
		name      string
		threshold float64
	}{
		{name: "semantic cluster", threshold: 0.9},
		{name: "ordinary MMR", threshold: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items := []DiversityCandidate{
				diversityCandidate(scope, "mem-a", 1, "event-a", "", []float32{1, 0}),
				diversityCandidate(scope, "mem-b", 0.9, "event-b", "", []float32{0.99, 0.01}),
				diversityCandidate(scope, "mem-c", 0.8, "event-c", "", []float32{0, 1}),
			}
			for index := range items {
				items[index].EmbeddingRevision = "embedding-v1"
				items[index].EmbeddingRevisionActive = true
				items[index].Citations = []Citation{{MemoryID: items[index].Memory.ID, RawEventID: items[index].SourceEventID, Operation: "semantic"}}
			}
			before := cloneDiversityCandidates(items)
			policy := DefaultDiversityPolicy()
			policy.SemanticThreshold = tc.threshold

			if _, err := SelectDiverseCandidates(scope, policy, items, 2); err != nil {
				t.Fatalf("SelectDiverseCandidates() error = %v", err)
			}
			if !reflect.DeepEqual(items, before) {
				t.Fatalf("input mutated:\n got: %+v\nwant: %+v", items, before)
			}
		})
	}
}

func TestSelectDiverseCandidatesHandlesLimitAndPoolBoundaries(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	compatible := func(count int) []DiversityCandidate {
		items := make([]DiversityCandidate, 0, count)
		vectors := [][]float32{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
		for index := 0; index < count; index++ {
			candidate := diversityCandidate(scope, "mem-"+string(rune('a'+index)), 1-float64(index)/10, "event-"+string(rune('a'+index)), "", vectors[index])
			candidate.EmbeddingRevision = "embedding-v1"
			candidate.EmbeddingRevisionActive = true
			items = append(items, candidate)
		}
		return items
	}
	policy := DefaultDiversityPolicy()
	policy.SemanticThreshold = 1
	tests := []struct {
		name                string
		items               []DiversityCandidate
		limit               int
		wantIDs             []string
		wantError           bool
		wantDiversity       int
		wantUnavailable     int
		wantPairComparisons int
	}{
		{name: "negative limit", items: compatible(2), limit: -1, wantError: true},
		{name: "empty pool", items: nil, limit: 2, wantIDs: []string{}},
		{name: "singleton pool", items: compatible(1), limit: 2, wantIDs: []string{"mem-a"}},
		{name: "zero limit", items: compatible(3), limit: 0, wantIDs: []string{}, wantDiversity: 3},
		{name: "limit exceeds pool", items: compatible(2), limit: 3, wantIDs: []string{"mem-a", "mem-b"}, wantPairComparisons: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := SelectDiverseCandidates(scope, policy, tc.items, tc.limit)
			if (err != nil) != tc.wantError {
				t.Fatalf("SelectDiverseCandidates() error = %v, wantError %t", err, tc.wantError)
			}
			if tc.wantError {
				return
			}
			if got := diversityCandidateIDs(result.Candidates); !reflect.DeepEqual(got, tc.wantIDs) {
				t.Fatalf("candidate ids = %v, want %v", got, tc.wantIDs)
			}
			if result.Dispositions.Diversity != tc.wantDiversity || result.Dispositions.SemanticUnavailable != tc.wantUnavailable || result.PairwiseComparisons != tc.wantPairComparisons {
				t.Fatalf("selection = %+v, want diversity=%d unavailable=%d comparisons=%d", result, tc.wantDiversity, tc.wantUnavailable, tc.wantPairComparisons)
			}
		})
	}
}

func TestDiversitySelectionFailsClosedForUnresolvedScope(t *testing.T) {
	validScope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	tests := []struct {
		name  string
		scope memory.Scope
	}{
		{name: "missing tenant", scope: memory.Scope{Project: "project-a", Namespace: "namespace-a"}},
		{name: "missing project", scope: memory.Scope{Tenant: "tenant-a", Namespace: "namespace-a"}},
		{name: "missing namespace", scope: memory.Scope{Tenant: "tenant-a", Project: "project-a"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := diversityCandidate(tc.scope, "mem-a", 1, "event-a", "", []float32{1, 0})
			candidate.EmbeddingRevision = "embedding-v1"
			candidate.EmbeddingRevisionActive = true

			deduplicated := DeduplicateDiversityCandidates(tc.scope, []DiversityCandidate{candidate})
			if len(deduplicated.Candidates) != 0 || deduplicated.Dispositions.Invalid != 1 {
				t.Fatalf("deduplicated = %+v, want fail-closed invalid disposition", deduplicated)
			}
			if _, err := SelectDiverseCandidates(tc.scope, DefaultDiversityPolicy(), []DiversityCandidate{candidate}, 1); err == nil {
				t.Fatal("SelectDiverseCandidates() error = nil, want unresolved-scope error")
			}
		})
	}

	valid := diversityCandidate(validScope, "mem-valid", 1, "event-valid", "", nil)
	result := DeduplicateDiversityCandidates(validScope, []DiversityCandidate{valid})
	if len(result.Candidates) != 1 || result.Dispositions.Invalid != 0 {
		t.Fatalf("valid deduplication = %+v, want candidate retained", result)
	}
}

func TestDeduplicateDiversityCandidatesDoesNotLetInvalidCandidateSuppressVisibleEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	tests := []struct {
		name   string
		mutate func(*DiversityCandidate)
	}{
		{name: "empty canonical id", mutate: func(candidate *DiversityCandidate) { candidate.Memory.ID = " " }},
		{name: "candidate state", mutate: func(candidate *DiversityCandidate) { candidate.Memory.State = memory.MemoryStateCandidate }},
		{name: "suppressed state", mutate: func(candidate *DiversityCandidate) { candidate.Memory.State = memory.MemoryStateSuppressed }},
		{name: "forgotten state", mutate: func(candidate *DiversityCandidate) { candidate.Memory.State = memory.MemoryStateForgotten }},
		{name: "deleted state", mutate: func(candidate *DiversityCandidate) { candidate.Memory.State = memory.MemoryStateDeleted }},
		{name: "NaN score", mutate: func(candidate *DiversityCandidate) { candidate.Score = math.NaN() }},
		{name: "positive infinite score", mutate: func(candidate *DiversityCandidate) { candidate.Score = math.Inf(1) }},
		{name: "negative infinite score", mutate: func(candidate *DiversityCandidate) { candidate.Score = math.Inf(-1) }},
		{name: "foreign tenant", mutate: func(candidate *DiversityCandidate) { candidate.Memory.Scope.Tenant = "tenant-b" }},
		{name: "foreign project", mutate: func(candidate *DiversityCandidate) { candidate.Memory.Scope.Project = "project-b" }},
		{name: "foreign namespace", mutate: func(candidate *DiversityCandidate) { candidate.Memory.Scope.Namespace = "namespace-b" }},
	}
	for _, tc := range tests {
		for _, identity := range []string{"source", "parent"} {
			t.Run(tc.name+"/"+identity, func(t *testing.T) {
				invalid := diversityCandidate(scope, "mem-invalid", 1, "", "", nil)
				visible := diversityCandidate(scope, "mem-visible", 0.5, "", "", nil)
				if identity == "source" {
					invalid.SourceEventID = "shared"
					visible.SourceEventID = "shared"
				} else {
					invalid.ParentMemoryID = "shared"
					visible.ParentMemoryID = "shared"
				}
				tc.mutate(&invalid)

				result := DeduplicateDiversityCandidates(scope, []DiversityCandidate{invalid, visible})
				if len(result.Candidates) != 1 || result.Candidates[0].Memory.ID != "mem-visible" {
					t.Fatalf("candidates = %+v, want only visible candidate", result.Candidates)
				}
				if result.Dispositions.Invalid != 1 || result.Dispositions.Duplicate != 0 {
					t.Fatalf("dispositions = %+v, want one invalid and no duplicate", result.Dispositions)
				}
			})
		}
	}
}

func diversityCandidate(scope memory.Scope, id string, score float64, sourceEventID, parentMemoryID string, embedding []float32) DiversityCandidate {
	return DiversityCandidate{
		FusedCandidate: FusedCandidate{Memory: memory.CanonicalMemory{
			ID: id, Scope: scope, State: memory.MemoryStateActive, Class: memory.MemoryClassEpisodic,
			ModifiedAt: time.Unix(100, 0).UTC(),
		}, Score: score},
		SourceEventID: sourceEventID, ParentMemoryID: parentMemoryID, Embedding: embedding,
	}
}

func diversityCandidateIDs(candidates []DiversityCandidate) []string {
	ids := make([]string, len(candidates))
	for index := range candidates {
		ids[index] = candidates[index].Memory.ID
	}
	return ids
}

func cloneDiversityCandidates(candidates []DiversityCandidate) []DiversityCandidate {
	cloned := make([]DiversityCandidate, len(candidates))
	for index := range candidates {
		cloned[index] = candidates[index]
		cloned[index].Embedding = append([]float32(nil), candidates[index].Embedding...)
		cloned[index].Citations = append([]Citation(nil), candidates[index].Citations...)
		if candidates[index].ChannelRanks != nil {
			cloned[index].ChannelRanks = make(map[FusionChannel]int, len(candidates[index].ChannelRanks))
			for channel, rank := range candidates[index].ChannelRanks {
				cloned[index].ChannelRanks[channel] = rank
			}
		}
	}
	return cloned
}
