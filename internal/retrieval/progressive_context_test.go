package retrieval

import (
	"reflect"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestEvaluateProgressiveContextReportsComparableLevelsAndStableRebuildIdentity(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	projection := progressiveTestProjection(scope, now)
	input := ProgressiveContextEvaluationInput{
		Scope: scope, EvaluatedAt: now, BaselineIdentity: "flat-fusion-v1",
		Levels: []ProgressiveContextLevelInput{
			{Identity: "short-v1", Kind: ProgressiveContextLevelShortRetrieval, Projection: &projection, ExpectedWatermarkHash: projection.SourceWatermarkHash(), MaxAge: time.Hour, CharacterBudget: 100, TokenBudget: 20, ExpectedEvidence: 1},
			{Identity: "medium-v1", Kind: ProgressiveContextLevelMediumOverview, Projection: &projection, ExpectedWatermarkHash: projection.SourceWatermarkHash(), MaxAge: time.Hour, CharacterBudget: 100, TokenBudget: 20, ExpectedEvidence: 1},
			{Identity: "canonical-v1", Kind: ProgressiveContextLevelCanonicalChunk, Evidence: []ProgressiveContextEvidence{progressiveTestEvidence(scope, "a", "memory-a", "two words")}, CharacterBudget: 100, TokenBudget: 20, ExpectedEvidence: 1},
		},
	}

	first, err := EvaluateProgressiveContext(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EvaluateProgressiveContext(input)
	if err != nil {
		t.Fatal(err)
	}
	if first.BaselineIdentity != "flat-fusion-v1" || len(first.Levels) != 3 {
		t.Fatalf("report = %+v", first)
	}
	for index, level := range first.Levels {
		if !level.Eligible || level.Freshness != ProgressiveContextFreshnessFresh {
			t.Fatalf("level %d = %+v", index, level)
		}
		if level.CharacterCount == 0 || level.TokenCount == 0 || level.CitationCoverage != 1 || level.EvidenceCoverage != 1 {
			t.Fatalf("level metrics = %+v", level)
		}
		if level.RebuildIdentity == "" || level.RebuildIdentity != second.Levels[index].RebuildIdentity {
			t.Fatalf("unstable rebuild identity: %+v / %+v", level, second.Levels[index])
		}
	}
	if first.Levels[0].Kind != ProgressiveContextLevelShortRetrieval || first.Levels[1].Kind != ProgressiveContextLevelMediumOverview || first.Levels[2].Kind != ProgressiveContextLevelCanonicalChunk {
		t.Fatalf("level order = %+v", first.Levels)
	}
}

func TestEvaluateProgressiveContextFailsClosedForStaleHiddenAndForeignEvidence(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	foreign := memory.Scope{Tenant: "foreign", Project: "project", Namespace: "namespace"}
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	stale := progressiveTestProjection(scope, now.Add(-2*time.Hour))
	hidden := progressiveTestProjection(scope, now)
	hidden.Items[0].LifecycleState = memory.MemoryStateSuppressed
	foreignProjection := progressiveTestProjection(scope, now)
	foreignProjection.Items[0].Source.Scope = foreign

	report, err := EvaluateProgressiveContext(ProgressiveContextEvaluationInput{Scope: scope, EvaluatedAt: now, BaselineIdentity: "flat-v1", Levels: []ProgressiveContextLevelInput{
		{Identity: "stale-v1", Kind: ProgressiveContextLevelShortRetrieval, Projection: &stale, ExpectedWatermarkHash: stale.SourceWatermarkHash(), MaxAge: time.Hour, CharacterBudget: 100, TokenBudget: 20},
		{Identity: "hidden-v1", Kind: ProgressiveContextLevelMediumOverview, Projection: &hidden, ExpectedWatermarkHash: hidden.SourceWatermarkHash(), MaxAge: time.Hour, CharacterBudget: 100, TokenBudget: 20},
		{Identity: "foreign-v1", Kind: ProgressiveContextLevelMediumOverview, Projection: &foreignProjection, ExpectedWatermarkHash: foreignProjection.SourceWatermarkHash(), MaxAge: time.Hour, CharacterBudget: 100, TokenBudget: 20},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]ProgressiveContextFailureReason{{ProgressiveContextFailureStale}, {ProgressiveContextFailureLifecycle}, {ProgressiveContextFailureIsolation}}
	for index, level := range report.Levels {
		if level.Eligible || !reflect.DeepEqual(level.FailureReasons, want[index]) {
			t.Fatalf("level %d = %+v, want failures %v", index, level, want[index])
		}
	}
}

func TestEvaluateProgressiveContextFailsClosedForMissingWatermarkBudgetAndCitation(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	projection := progressiveTestProjection(scope, now)
	projection.SourceWatermark = memory.ContextProjectionWatermark{}
	projection.Items[0].Citation = memory.ProjectionCitation{}
	report, err := EvaluateProgressiveContext(ProgressiveContextEvaluationInput{Scope: scope, EvaluatedAt: now, BaselineIdentity: "flat-v1", Levels: []ProgressiveContextLevelInput{{Identity: "short-v1", Kind: ProgressiveContextLevelShortRetrieval, Projection: &projection, MaxAge: time.Hour, CharacterBudget: 1, TokenBudget: 1, ExpectedEvidence: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	got := report.Levels[0]
	if got.Eligible || !containsProgressiveFailure(got.FailureReasons, ProgressiveContextFailureWatermark) || !containsProgressiveFailure(got.FailureReasons, ProgressiveContextFailureBudget) || !containsProgressiveFailure(got.FailureReasons, ProgressiveContextFailureCitation) {
		t.Fatalf("level = %+v", got)
	}
}

func progressiveTestProjection(scope memory.Scope, updatedAt time.Time) memory.ContextProjection {
	return memory.ContextProjection{ID: "projection-a", Scope: scope, Kind: memory.ContextProjectionKindRetrieval, Version: 1, SchemaVersion: "schema-v1", PolicyVersion: "policy-v1", RendererVersion: "renderer-v1", SourceWatermark: memory.ContextProjectionWatermark{CanonicalVersionIDs: []string{"version-a"}}, Status: memory.ContextProjectionStatusActive, Items: []memory.ContextProjectionItem{{ID: "item-a", Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: "version-a", Version: 1, MemoryID: "memory-a", Scope: scope, LifecycleState: memory.MemoryStateActive}, Text: "two words", Class: memory.MemoryClassSummary, LifecycleState: memory.MemoryStateActive, SortKey: "a", Citation: memory.ProjectionCitation{MemoryID: "memory-a", Operation: "context_projection"}}}, CreatedAt: updatedAt, UpdatedAt: updatedAt}
}

func progressiveTestEvidence(scope memory.Scope, alias, memoryID, content string) ProgressiveContextEvidence {
	return ProgressiveContextEvidence{Alias: alias, Memory: memory.CanonicalMemory{ID: memoryID, Scope: scope, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: content}, Citations: []Citation{{MemoryID: memoryID, Operation: "canonical"}}}
}
