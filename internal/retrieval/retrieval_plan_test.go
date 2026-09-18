package retrieval

import (
	"reflect"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestBuildRetrievalPlanClassifiesQueryFamilies(t *testing.T) {
	tests := []struct {
		name               string
		hints              []QueryAnalysisHint
		signals            []QueryAnalysisSignal
		embeddingAvailable bool
		want               RetrievalQueryFamily
	}{
		{name: "exact lookup", hints: []QueryAnalysisHint{{Kind: QueryAnalysisHintIntent, Disposition: QueryAnalysisHintPresent, Value: "lookup"}}, want: RetrievalQueryFamilyExactLookup},
		{name: "semantic", embeddingAvailable: true, want: RetrievalQueryFamilySemantic},
		{name: "temporal", hints: []QueryAnalysisHint{{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintPresent, Value: "recent"}}, want: RetrievalQueryFamilyTemporal},
		{name: "entity relation", hints: []QueryAnalysisHint{{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "stele"}}, want: RetrievalQueryFamilyEntityRelation},
		{name: "multi hop", signals: []QueryAnalysisSignal{{Kind: QueryAnalysisSignalSubquery, Text: "first"}, {Kind: QueryAnalysisSignalSubquery, Text: "second"}}, want: RetrievalQueryFamilyMultiHop},
		{name: "procedural", hints: []QueryAnalysisHint{{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintPresent, Value: string(memory.MemoryClassProcedural)}}, want: RetrievalQueryFamilyProcedural},
		{name: "general fallback", want: RetrievalQueryFamilyGeneral},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := plannerInputForTest(test.hints, test.signals, test.embeddingAvailable)
			plan, err := BuildRetrievalPlan(input)
			if err != nil {
				t.Fatalf("BuildRetrievalPlan() error = %v", err)
			}
			if plan.Family != test.want {
				t.Fatalf("BuildRetrievalPlan().Family = %q, want %q", plan.Family, test.want)
			}
			if err := plan.Validate(input.Policy.HardLimits); err != nil {
				t.Fatalf("plan.Validate() error = %v", err)
			}
		})
	}
}

func TestBuildRetrievalPlanUsesDeterministicPrecedenceAndIdentity(t *testing.T) {
	input := plannerInputForTest(
		[]QueryAnalysisHint{
			{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "stele"},
			{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintPresent, Value: "recent"},
			{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintPresent, Value: string(memory.MemoryClassProcedural)},
		},
		[]QueryAnalysisSignal{{Kind: QueryAnalysisSignalSubquery, Text: "first"}},
		true,
	)

	first, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan(first) error = %v", err)
	}
	second, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan(second) error = %v", err)
	}
	if first.Family != RetrievalQueryFamilyProcedural {
		t.Fatalf("precedence family = %q, want %q", first.Family, RetrievalQueryFamilyProcedural)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated plans differ:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if first.Identity.String() != "retrieval-planner-v1:retrieval-plan-policy-v1:procedural" {
		t.Fatalf("plan identity = %q", first.Identity.String())
	}
}

func TestBuildRetrievalPlanPreservesConfiguredContextPriority(t *testing.T) {
	input := plannerInputForTest(
		[]QueryAnalysisHint{{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintPresent, Value: string(memory.MemoryClassProcedural)}},
		nil,
		false,
	)

	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if len(plan.ContextPriorities) == 0 || plan.ContextPriorities[0] != memory.MemoryClassProcedural {
		t.Fatalf("context priorities = %v, want procedural first", plan.ContextPriorities)
	}
}

func TestBuildRetrievalPlanRejectsUnknownVersionsAndIncompleteTemplates(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RetrievalPlanInput)
	}{
		{name: "unknown planner", mutate: func(input *RetrievalPlanInput) { input.Policy.PlannerVersion = "unknown" }},
		{name: "unknown policy", mutate: func(input *RetrievalPlanInput) { input.Policy.Version = "unknown" }},
		{name: "missing template", mutate: func(input *RetrievalPlanInput) { delete(input.Policy.Templates, RetrievalQueryFamilyGeneral) }},
		{name: "too many candidates", mutate: func(input *RetrievalPlanInput) {
			template := input.Policy.Templates[RetrievalQueryFamilyGeneral]
			template.TotalCandidates = input.Policy.HardLimits.MaxCandidates + 1
			input.Policy.Templates[RetrievalQueryFamilyGeneral] = template
		}},
		{name: "too many passes", mutate: func(input *RetrievalPlanInput) {
			template := input.Policy.Templates[RetrievalQueryFamilyGeneral]
			template.MaxPasses = 3
			input.Policy.Templates[RetrievalQueryFamilyGeneral] = template
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := plannerInputForTest(nil, nil, false)
			test.mutate(&input)
			if _, err := BuildRetrievalPlan(input); err == nil {
				t.Fatal("BuildRetrievalPlan() error = nil, want validation error")
			}
		})
	}
}

func TestBuildRetrievalPlanRejectsInvalidFollowUpChannels(t *testing.T) {
	tests := []struct {
		name     string
		channels []FusionChannel
	}{
		{name: "unknown", channels: []FusionChannel{"unknown"}},
		{name: "undeclared", channels: []FusionChannel{FusionChannelSemantic}},
		{name: "duplicate", channels: []FusionChannel{FusionChannelLexical, FusionChannelLexical}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := plannerInputForTest(nil, nil, false)
			template := input.Policy.Templates[RetrievalQueryFamilyGeneral]
			template.Channels = []FusionChannel{FusionChannelLexical}
			template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 2}
			template.TotalCandidates = 2
			template.Fusion.PerChannelCandidate = 2
			template.Fusion.TotalCandidates = 2
			template.MaxPasses = 2
			template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 1, CandidateAllocation: 1, Channels: test.channels}
			input.Policy.Templates[RetrievalQueryFamilyGeneral] = template
			if _, err := BuildRetrievalPlan(input); err == nil {
				t.Fatal("BuildRetrievalPlan() error = nil, want invalid follow-up rejected")
			}
		})
	}
	t.Run("zero allocation", func(t *testing.T) {
		input := plannerInputForTest(nil, nil, false)
		template := input.Policy.Templates[RetrievalQueryFamilyGeneral]
		template.Channels = []FusionChannel{FusionChannelLexical}
		template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 0}
		template.TotalCandidates = 1
		template.Fusion.PerChannelCandidate = 1
		template.Fusion.TotalCandidates = 1
		template.MaxPasses = 2
		template.FollowUp = RetrievalPlanFollowUpRule{Enabled: true, MinimumVisible: 1, CandidateAllocation: 1, Channels: []FusionChannel{FusionChannelLexical}}
		input.Policy.Templates[RetrievalQueryFamilyGeneral] = template
		if _, err := BuildRetrievalPlan(input); err == nil {
			t.Fatal("BuildRetrievalPlan() error = nil, want zero allocation rejected")
		}
	})
}

func TestBuildRetrievalPlanRejectsFusionOutsidePlanEnvelope(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RetrievalPlanTemplate)
	}{
		{name: "fusion total exceeds plan total", mutate: func(template *RetrievalPlanTemplate) { template.Fusion.TotalCandidates = template.TotalCandidates + 1 }},
		{name: "fusion channel limit exceeds allocation", mutate: func(template *RetrievalPlanTemplate) {
			template.Fusion.PerChannelCandidate = template.ChannelCandidates[FusionChannelSemantic] + 1
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := plannerInputForTest(nil, nil, false)
			template := input.Policy.Templates[RetrievalQueryFamilyGeneral]
			template.Channels = []FusionChannel{FusionChannelLexical, FusionChannelSemantic}
			template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 3, FusionChannelSemantic: 2}
			template.TotalCandidates = 5
			template.Fusion.PerChannelCandidate = 2
			template.Fusion.TotalCandidates = 5
			test.mutate(&template)
			input.Policy.Templates[RetrievalQueryFamilyGeneral] = template
			if _, err := BuildRetrievalPlan(input); err == nil {
				t.Fatal("BuildRetrievalPlan() error = nil, want fusion envelope rejected")
			}
		})
	}
}

func TestBuildRetrievalPlanRejectsChannelAllocationAboveTotalCandidates(t *testing.T) {
	input := plannerInputForTest(nil, nil, false)
	template := input.Policy.Templates[RetrievalQueryFamilyGeneral]
	template.Channels = []FusionChannel{FusionChannelLexical, FusionChannelSemantic}
	template.ChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 3, FusionChannelSemantic: 3}
	template.TotalCandidates = 5
	template.Fusion.PerChannelCandidate = 3
	template.Fusion.TotalCandidates = 5
	input.Policy.Templates[RetrievalQueryFamilyGeneral] = template

	if _, err := BuildRetrievalPlan(input); err == nil {
		t.Fatal("BuildRetrievalPlan() error = nil, want channel allocation sum above total rejected")
	}
}

func TestBuildRetrievalPlanCarriesFallbackAllocationInStableIdentity(t *testing.T) {
	input := plannerInputForTest(nil, nil, true)
	template := input.Policy.Templates[RetrievalQueryFamilySemantic]
	template.Channels = []FusionChannel{FusionChannelSemantic}
	template.ChannelCandidates = map[FusionChannel]int{FusionChannelSemantic: 4}
	template.FallbackChannelCandidates = map[FusionChannel]int{FusionChannelLexical: 1}
	template.TotalCandidates = 4
	template.Fusion.PerChannelCandidate = 4
	template.Fusion.TotalCandidates = 4
	input.Policy.Templates[RetrievalQueryFamilySemantic] = template

	plan, err := BuildRetrievalPlan(input)
	if err != nil {
		t.Fatalf("BuildRetrievalPlan() error = %v", err)
	}
	if got := plan.Identity.String(); got != "retrieval-planner-v1:retrieval-plan-policy-v1:semantic:fallback=lexical:1" {
		t.Fatalf("plan identity = %q", got)
	}
	if !reflect.DeepEqual(plan.FallbackChannelCandidates, map[FusionChannel]int{FusionChannelLexical: 1}) {
		t.Fatalf("fallback allocations = %v", plan.FallbackChannelCandidates)
	}

	plan.FallbackChannelCandidates[FusionChannelLexical] = 2
	if err := plan.Validate(input.Policy.HardLimits); err == nil {
		t.Fatal("plan.Validate() error = nil after identity-bound fallback allocation changed")
	}
}

func TestBuildRetrievalPlanRejectsInvalidFallbackAllocation(t *testing.T) {
	tests := []struct {
		name     string
		fallback map[FusionChannel]int
		reranker int
	}{
		{name: "overlaps planned channel", fallback: map[FusionChannel]int{FusionChannelSemantic: 1}},
		{name: "unknown channel", fallback: map[FusionChannel]int{"unknown": 1}},
		{name: "zero allocation", fallback: map[FusionChannel]int{FusionChannelLexical: 0}},
		{name: "exhausts planned capacity", fallback: map[FusionChannel]int{FusionChannelLexical: 3}, reranker: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := plannerInputForTest(nil, nil, true)
			template := input.Policy.Templates[RetrievalQueryFamilySemantic]
			template.Channels = []FusionChannel{FusionChannelSemantic}
			template.ChannelCandidates = map[FusionChannel]int{FusionChannelSemantic: 4}
			template.FallbackChannelCandidates = test.fallback
			template.TotalCandidates = 4
			template.Fusion.PerChannelCandidate = 4
			template.Fusion.TotalCandidates = 4
			template.RerankerEligible = test.reranker > 0
			template.RerankerHeadroom = test.reranker
			input.Policy.Templates[RetrievalQueryFamilySemantic] = template
			if _, err := BuildRetrievalPlan(input); err == nil {
				t.Fatal("BuildRetrievalPlan() error = nil, want invalid fallback allocation rejected")
			}
		})
	}
}

func plannerInputForTest(hints []QueryAnalysisHint, signals []QueryAnalysisSignal, embeddingAvailable bool) RetrievalPlanInput {
	analysisSignals := []QueryAnalysisSignal{{Kind: QueryAnalysisSignalOriginal, Text: "test query", Mandatory: true}}
	analysisSignals = append(analysisSignals, signals...)
	return RetrievalPlanInput{
		AcceptedQuery: "test query",
		Analysis: QueryAnalysisResult{
			Identity:    QueryAnalysisIdentity{PolicyVersion: QueryAnalysisPolicyVersionV1, LimitsVersion: QueryAnalysisLimitsVersionV1},
			Disposition: QueryAnalysisDispositionComplete,
			Hints:       append([]QueryAnalysisHint(nil), hints...),
			Signals:     analysisSignals,
		},
		EmbeddingAvailable: embeddingAvailable,
		Policy:             DefaultRetrievalPlanPolicy(),
		Now:                time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC),
	}
}

func FuzzBuildRetrievalPlanBoundsAndDeterminism(f *testing.F) {
	f.Add("query", uint8(0), true)
	f.Add("another query", uint8(6), false)
	f.Fuzz(func(t *testing.T, query string, familyIndex uint8, embeddingAvailable bool) {
		if query == "" {
			query = "fallback"
		}
		input := plannerInputForTest(nil, nil, embeddingAvailable)
		input.AcceptedQuery = query
		input.Analysis.Signals[0].Text = query
		families := append([]RetrievalQueryFamily(nil), retrievalQueryFamilies...)
		selected := families[int(familyIndex)%len(families)]
		template := input.Policy.Templates[selected]
		template.TotalCandidates = 1 + int(familyIndex)%input.Policy.HardLimits.MaxCandidates
		remaining := template.TotalCandidates
		for _, channel := range template.Channels {
			allocated := remaining
			if allocated > input.Policy.HardLimits.MaxCandidatesPerChannel {
				allocated = input.Policy.HardLimits.MaxCandidatesPerChannel
			}
			template.ChannelCandidates[channel] = allocated
			remaining -= allocated
			if remaining < 1 {
				remaining = 1
			}
		}
		input.Policy.Templates[selected] = template
		before := append([]FusionChannel(nil), template.Channels...)

		first, firstErr := BuildRetrievalPlan(input)
		second, secondErr := BuildRetrievalPlan(input)
		if (firstErr == nil) != (secondErr == nil) {
			t.Fatalf("non-deterministic errors: first=%v second=%v", firstErr, secondErr)
		}
		if firstErr != nil {
			return
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("non-deterministic plans: first=%+v second=%+v", first, second)
		}
		if !reflect.DeepEqual(before, template.Channels) {
			t.Fatalf("input channels mutated: before=%v after=%v", before, template.Channels)
		}
		if first.TotalCandidates > input.Policy.HardLimits.MaxCandidates || first.MaxPasses > 2 || first.LatencyBudget > input.Policy.HardLimits.MaxLatency || first.ContextItems > input.Policy.HardLimits.MaxContextItems {
			t.Fatalf("plan exceeds hard limits: %+v", first)
		}
	})
}
