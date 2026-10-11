package retrieval

import (
	"context"
	"reflect"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

type testContextAdmission struct{ visited []string }

func (a *testContextAdmission) AdmitContextItem(_ string, h SearchHit) bool {
	a.visited = append(a.visited, h.Memory.ID)
	return h.Memory.ID != "oversized"
}

type boundedContextAdmission struct {
	remaining int
	visited   []string
}

func (a *boundedContextAdmission) AdmitContextItem(section string, h SearchHit) bool {
	a.visited = append(a.visited, section+":"+h.Memory.ID)
	if len(h.Memory.Content) > a.remaining {
		return false
	}
	a.remaining -= len(h.Memory.Content)
	return true
}
func (a *boundedContextAdmission) NoteContextItemLimit(h SearchHit) {}

func TestContextAdmissionServicePreservesSectionsAndSummaryPreference(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	classes := []memory.MemoryClass{memory.MemoryClassProfile, memory.MemoryClassSummary, memory.MemoryClassRelation, memory.MemoryClassEpisodic, memory.MemoryClassProcedural}
	hits := make([]ScoredMemory, 0, len(classes))
	for i, class := range classes {
		hits = append(hits, ScoredMemory{Memory: memory.CanonicalMemory{ID: string(class), Scope: scope, Class: class, State: memory.MemoryStateActive, Content: "fit"}, LexicalScore: float64(10 - i)})
	}
	lexical := &stubLexicalSource{hits: hits}
	s := NewService(ServiceDependencies{Lexical: lexical})
	input := AssembleContextInput{Scope: scope, Query: "q", Budget: 5, IncludeRelations: true}
	native, err := s.AssembleContext(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	ample := &boundedContextAdmission{remaining: 100}
	input.FinalAdmission = ample
	got, err := s.AssembleContext(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(native, got) {
		t.Fatal("ample final admission changed baseline assembly")
	}
	want := []string{"profile:profile", "relevant_summaries:summary", "related_entities:relation", "recent_episodes:procedural", "recent_session:episodic"}
	if !reflect.DeepEqual(ample.visited, want) {
		t.Fatalf("order=%v", ample.visited)
	}
	tight := &boundedContextAdmission{remaining: 6}
	input.FinalAdmission = tight
	got, err = s.AssembleContext(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Profile) != 1 || len(got.RelevantSummaries) != 1 || len(got.RelatedEntities)+len(got.RecentSession)+len(got.RecentEpisodes) != 0 {
		t.Fatalf("protected section preference lost: %+v", got)
	}
	if !reflect.DeepEqual(tight.visited, want) || lexical.gotInput.TopK != 15 {
		t.Fatal("admission broadened or reordered bounded retrieval")
	}
}

func TestContextAdmissionServiceContinuesPastOversizedProfile(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	lexical := &stubLexicalSource{hits: []ScoredMemory{
		{Memory: memory.CanonicalMemory{ID: "oversized", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "too large"}, LexicalScore: 10},
		{Memory: memory.CanonicalMemory{ID: "fit", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "ok"}, LexicalScore: 1},
	}}
	a := &boundedContextAdmission{remaining: 2}
	out, err := NewService(ServiceDependencies{Lexical: lexical}).AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "q", Budget: 1, FinalAdmission: a})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Profile) != 1 || out.Profile[0].Memory.ID != "fit" || !reflect.DeepEqual(a.visited, []string{"profile:oversized", "profile:fit"}) {
		t.Fatalf("selection=%+v visits=%v", out.Profile, a.visited)
	}
}

func TestContextAdmissionActivePlannerPreservesQuotaAndSummaryPreference(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "planner-admission"}
	policy := lexicalOnlyPlannerPolicy(10)
	for family, template := range policy.Templates {
		template.ContextItems = 2
		template.ContextPriorities = []memory.MemoryClass{memory.MemoryClassProcedural, memory.MemoryClassSummary, memory.MemoryClassProfile}
		template.MemoryClassQuotas = map[memory.MemoryClass]int{memory.MemoryClassProcedural: 1, memory.MemoryClassSummary: 1, memory.MemoryClassProfile: 1}
		policy.Templates[family] = template
	}
	makeService := func() *Service {
		lexical := &plannerChannelRecorder{hits: [][]ScoredMemory{{plannerTestHit(scope, "profile", memory.MemoryClassProfile), plannerTestHit(scope, "summary", memory.MemoryClassSummary), plannerTestHit(scope, "procedure", memory.MemoryClassProcedural)}}}
		return NewService(ServiceDependencies{Lexical: lexical, RankingRolloutPolicyReader: plannerPolicyReader{policy: plannerTestRollout(scope, memory.RankingRolloutSurfaceContext, memory.RankingRolloutPolicyStatusActiveForScope)}, RetrievalPlanPolicy: policy})
	}
	input := AssembleContextInput{Scope: scope, Query: "original", Budget: 10}
	native, err := makeService().AssembleContext(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	a := &boundedContextAdmission{remaining: 100}
	input.FinalAdmission = a
	got, err := makeService().AssembleContext(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(native, got) || len(got.RecentEpisodes) != 1 || len(got.RelevantSummaries) != 1 || len(got.Profile) != 0 {
		t.Fatalf("planner quota/order changed: %+v", got)
	}
	if !reflect.DeepEqual(a.visited, []string{"recent_episodes:procedure", "relevant_summaries:summary"}) {
		t.Fatalf("order=%v", a.visited)
	}
}

func TestContextAdmissionInternalChunkUsesWholeEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	candidate := testChunk(scope, "parent", "中文🙂", memory.MemoryStateActive)
	s := NewService(ServiceDependencies{Chunks: chunkSearcherStub{candidates: []ChunkCandidate{candidate}}, ChunkRollout: memory.ChunkRolloutModeActive})
	for _, cap := range []int{len(candidate.Chunk.Content), len(candidate.Chunk.Content) - 1} {
		a := &boundedContextAdmission{remaining: cap}
		out, err := s.AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "q", Budget: 1, CharacterBudget: 1, FinalAdmission: a})
		if err != nil {
			t.Fatal(err)
		}
		if len(a.visited) != 1 {
			t.Fatal("chunk was clipped before final admission")
		}
		if cap == len(candidate.Chunk.Content) {
			if len(out.Profile) != 1 || out.Profile[0].Memory.Content != candidate.Chunk.Content || len(out.Profile[0].Citations) == 0 {
				t.Fatal("whole chunk evidence or citation lost")
			}
		} else if len(out.Profile) != 0 {
			t.Fatal("oversized chunk admitted")
		}
	}
}

func TestContextAdmissionInternalProjectionUsesWholeEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	projection := memory.ContextProjection{Items: []memory.ContextProjectionItem{{Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: "parent", Scope: scope}, Class: memory.MemoryClassProfile, LifecycleState: memory.MemoryStateActive, Text: "中文🙂", Citation: memory.ProjectionCitation{MemoryID: "parent", Operation: "context_projection"}}}}
	s := NewService(ServiceDependencies{Projections: stubContextProjectionReader{projections: map[memory.ContextProjectionKind]memory.ContextProjection{memory.ContextProjectionKindAlwaysVisible: projection}}})
	for _, cap := range []int{len("中文🙂"), len("中文🙂") - 1} {
		a := &boundedContextAdmission{remaining: cap}
		out, err := s.AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "q", Budget: 1, CharacterBudget: 1, UseProjections: true, FinalAdmission: a})
		if err != nil {
			t.Fatal(err)
		}
		if len(a.visited) != 1 {
			t.Fatal("projection was clipped before final admission")
		}
		if cap == len("中文🙂") {
			if len(out.Profile) != 1 || out.Profile[0].Memory.Content != "中文🙂" || len(out.Profile[0].Citations) == 0 {
				t.Fatal("projection evidence or citation lost")
			}
		} else if len(out.Profile) != 0 {
			t.Fatal("oversized projection admitted")
		}
	}
}

func TestContextAdmissionDoesNotBypassWithUnsupportedDerivedSections(t *testing.T) {
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	insights := &stubDerivedInsightSource{}
	a := &boundedContextAdmission{remaining: 100}
	_, err := NewService(ServiceDependencies{Insights: insights}).AssembleContext(context.Background(), AssembleContextInput{Scope: scope, Query: "q", Budget: 2, FinalAdmission: a, IncludeExperienceInsights: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(insights.gotInputs) != 0 {
		t.Fatal("non-memory section bypassed canonical final admission")
	}
}
func (a *testContextAdmission) NoteContextItemLimit(h SearchHit) {
	a.visited = append(a.visited, h.Memory.ID)
}
func TestContextAdmissionContinuesBoundedStream(t *testing.T) {
	hits := make([]SearchHit, 3)
	for i, id := range []string{"oversized", "fit", "later"} {
		hits[i].Memory.ID = id
	}
	a := &testContextAdmission{}
	remaining := 1
	out := selectContextSection(hits, "recent_session", 1, &remaining, a)
	if len(out) != 1 || out[0].Memory.ID != "fit" || remaining != 0 {
		t.Fatalf("whole-item selection: %+v remaining %d", out, remaining)
	}
	if len(a.visited) != 2 {
		t.Fatalf("unexpected stream: %v", a.visited)
	}
	remaining = 1
	native := selectContextSection(hits, "recent_session", 1, &remaining, nil)
	if len(native) != 1 || native[0].Memory.ID != "oversized" {
		t.Fatal("native default changed")
	}
}
