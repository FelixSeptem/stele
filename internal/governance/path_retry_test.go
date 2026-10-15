package governance

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestSummaryProcessorSeparatesNormalizedExactPaths(t *testing.T) {
	items := []CandidateMemory{
		newPromotedEpisodicCandidate("a1", "event-a1", "first task"),
		newPromotedEpisodicCandidate("b1", "event-b1", "second task"),
		newPromotedEpisodicCandidate("a2", "event-a2", "first task result"),
		newPromotedEpisodicCandidate("b2", "event-b2", "second task result"),
	}
	items[0].MemoryPath, items[2].MemoryPath = "/tasks/a/", "tasks/a"
	items[1].MemoryPath, items[3].MemoryPath = "tasks/b", "tasks/b"
	repo := &stubSummaryRepository{}
	processor := SummaryProcessor{
		Source: stubCompactionSource{candidates: items}, Repository: repo,
		Summarizer:  DeterministicSummarizer{},
		NewMemoryID: func() string { return "summary" }, NewVersionID: func() string { return "version" },
	}
	if err := processor.CompactScope(context.Background(), items[0].Scope, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(repo.created) != 2 {
		t.Fatalf("created %d summaries; want two exact-path clusters", len(repo.created))
	}
	for i, want := range [][]string{{"event-a1", "event-a2"}, {"event-b1", "event-b2"}} {
		if repo.created[i].MemoryPath != []string{"tasks/a", "tasks/b"}[i] {
			t.Fatalf("summary path = %q", repo.created[i].MemoryPath)
		}
		got := repo.created[i].EvidenceRawEventIDs
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("summary %d evidence = %v; want %v", i, got, want)
		}
	}
}

type pathCanonicalRepository struct {
	stubCanonicalRepository
	path      string
	canonical memory.CanonicalMemory
}

func (r *pathCanonicalRepository) GetLatestCanonicalByScopePathAndClass(ctx context.Context, scope memory.Scope, path string, class memory.MemoryClass) (memory.CanonicalMemory, bool, error) {
	r.path = path
	return r.canonical, true, nil
}

func TestConsolidationSelectsLatestCanonicalAtNormalizedCandidatePath(t *testing.T) {
	candidate := newPromotedEpisodicCandidate("candidate", "event", "new profile")
	candidate.Status, candidate.Class, candidate.Mutability = CandidateStatusPending, memory.MemoryClassProfile, MutabilityMutable
	candidate.MemoryPath = "/tasks/a/"
	repo := &pathCanonicalRepository{canonical: memory.CanonicalMemory{ID: "same-path-memory", Scope: candidate.Scope, MemoryPath: "tasks/a", Class: candidate.Class, Content: "old profile"}}
	processor := ConsolidationProcessor{Candidates: &stubCandidateRepository{listByRawEventResult: []CandidateMemory{candidate}}, Canonicals: repo, Consolidator: RuleBasedConsolidator{}, NewMemoryID: func() string { return "wrong-memory" }, NewVersionID: func() string { return "version" }, NewProvenanceID: func() string { return "provenance" }}
	if err := processor.ProcessByRawEvent(context.Background(), candidate.SourceRawEventID); err != nil {
		t.Fatal(err)
	}
	if repo.path != "tasks/a" || len(repo.promoted) != 1 || repo.promoted[0].MemoryID != "same-path-memory" {
		t.Fatalf("path=%q promotions=%+v", repo.path, repo.promoted)
	}
}

func TestSummaryRejectsForeignScopeAndMixedPaths(t *testing.T) {
	for _, foreignScope := range []bool{false, true} {
		first := newPromotedEpisodicCandidate("a", "event-a", "first")
		second := newPromotedEpisodicCandidate("b", "event-b", "second")
		first.MemoryPath = "tasks/a"
		second.MemoryPath = first.MemoryPath
		if foreignScope {
			second.Scope.Tenant = "foreign"
		} else {
			second.MemoryPath = "tasks/b"
		}
		if _, err := (DeterministicSummarizer{}).Summarize(context.Background(), SummaryCluster{Scope: first.Scope, Candidates: []CandidateMemory{first, second}}); err == nil {
			t.Fatalf("accepted mixed summary cluster (foreign scope=%v)", foreignScope)
		}
	}
}

func TestConsolidationDoesNotSupersedeCanonicalAtAnotherPath(t *testing.T) {
	candidate := newPromotedEpisodicCandidate("candidate", "event", "changed profile")
	candidate.Status, candidate.Class, candidate.Mutability = CandidateStatusPending, memory.MemoryClassProfile, MutabilityMutable
	candidate.MemoryPath = "tasks/a"
	latest := memory.CanonicalMemory{ID: "foreign-path-memory", Scope: candidate.Scope, Class: candidate.Class, MemoryPath: "tasks/b", Content: "prior profile"}
	decision, err := (RuleBasedConsolidator{}).Decide(context.Background(), candidate, latest, true)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ConsolidationActionPromote {
		t.Fatalf("action = %q; want promotion independent of another path", decision.Action)
	}
}

func TestConsolidationRetrySkipsCandidatesWithFinalStatus(t *testing.T) {
	for _, status := range []CandidateStatus{CandidateStatusPromoted, CandidateStatusSuppressed, CandidateStatusExpired, CandidateStatusDeleted} {
		t.Run(string(status), func(t *testing.T) {
			candidate := newPromotedEpisodicCandidate("candidate", "event", "reference value")
			candidate.Status = status
			candidates := &stubCandidateRepository{listByRawEventResult: []CandidateMemory{candidate}}
			canonicals := &stubCanonicalRepository{}
			processor := ConsolidationProcessor{
				Candidates: candidates, Canonicals: canonicals, Consolidator: RuleBasedConsolidator{},
				NewMemoryID: func() string { return "unexpected-memory" }, NewVersionID: func() string { return "unexpected-version" }, NewProvenanceID: func() string { return "unexpected-provenance" },
			}
			if err := processor.ProcessByRawEvent(context.Background(), candidate.SourceRawEventID); err != nil {
				t.Fatal(err)
			}
			if len(canonicals.promoted) != 0 || len(candidates.transitions) != 0 {
				t.Fatalf("retried finalized candidate: promotions=%d transitions=%d", len(canonicals.promoted), len(candidates.transitions))
			}
		})
	}
}

func TestExtractionRetryReusesCommittedCandidates(t *testing.T) {
	now := time.Now().UTC()
	candidate := newPromotedEpisodicCandidate("existing", "event", "durable reference")
	repo := &stubCandidateRepository{listByRawEventResult: []CandidateMemory{candidate}}
	recorder := &stubRawEventCompletionRecorder{}
	processor := ExtractionProcessor{
		Extractor:  stubExtractor{outputs: []ExtractedCandidate{{Class: memory.MemoryClassEpisodic, Content: "durable reference", Confidence: .9, Importance: .9, Freshness: .9, Sensitivity: SensitivityLow, Mutability: MutabilityImmutable, RetentionClass: candidate.RetentionClass}}},
		Candidates: repo, RawEvents: recorder, Now: func() time.Time { return now },
		NewCandidateID: func() string { return "new" }, NewProvenanceID: func() string { return "new-prov" },
	}
	claim := ClaimedRawEvent{Event: memory.RawEvent{ID: "event", Scope: candidate.Scope, Content: "durable reference"}, WorkerID: "worker", ClaimedAt: now, LeaseUntil: now.Add(time.Minute), Attempt: 2}
	if err := processor.ProcessClaimedRawEvent(context.Background(), claim); err != nil {
		t.Fatal(err)
	}
	if len(repo.created) != 0 {
		t.Fatalf("created %d duplicate candidates", len(repo.created))
	}
	if len(recorder.inputs) != 1 {
		t.Fatalf("completion calls = %d", len(recorder.inputs))
	}
}

func TestExtractionRetryCompletesPartiallyCommittedCandidates(t *testing.T) {
	now := time.Now().UTC()
	candidate := newPromotedEpisodicCandidate("existing", "event", "first")
	repo := &stubCandidateRepository{listByRawEventResult: []CandidateMemory{candidate}}
	first := ExtractedCandidate{Class: memory.MemoryClassEpisodic, Content: "first", Confidence: .9, Importance: .9, Freshness: .9, Sensitivity: SensitivityLow, Mutability: MutabilityImmutable, RetentionClass: candidate.RetentionClass}
	second := first
	second.Content = "second"
	processor := ExtractionProcessor{
		Extractor:  stubExtractor{outputs: []ExtractedCandidate{first, second}},
		Candidates: repo, RawEvents: &stubRawEventCompletionRecorder{}, Now: func() time.Time { return now },
		NewCandidateID: func() string { return "new" }, NewProvenanceID: func() string { return "new-prov" },
	}
	claim := ClaimedRawEvent{Event: memory.RawEvent{ID: "event", Scope: candidate.Scope, Content: "source"}, WorkerID: "worker", ClaimedAt: now, LeaseUntil: now.Add(time.Minute), Attempt: 2}
	if err := processor.ProcessClaimedRawEvent(context.Background(), claim); err != nil {
		t.Fatal(err)
	}
	if len(repo.created) != 1 || repo.created[0].Content != "second" {
		t.Fatalf("retry candidates = %+v; want only uncommitted second candidate", repo.created)
	}
}
