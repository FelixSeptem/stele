package retrieval

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestNewQueryAnalysisResultRetainsImmutableOriginalFirst(t *testing.T) {
	input := QueryAnalysisInput{
		AcceptedQuery: "  PostgreSQL migration policy?  ",
		PolicyVersion: QueryAnalysisPolicyVersionV1,
		Limits:        DefaultQueryAnalysisLimits(),
	}
	derived := []QueryAnalysisSignal{{Kind: QueryAnalysisSignalNormalized, Text: "postgresql migration policy"}}

	result, err := NewQueryAnalysisResult(input, QueryAnalysisDispositionComplete, nil, derived)
	if err != nil {
		t.Fatalf("NewQueryAnalysisResult() error = %v", err)
	}
	derived[0].Text = "mutated after construction"

	if got := result.Signals[0]; got.Kind != QueryAnalysisSignalOriginal || got.Text != input.AcceptedQuery || !got.Mandatory {
		t.Fatalf("first signal = %+v, want exact mandatory original query", got)
	}
	if got := result.Signals[1].Text; got != "postgresql migration policy" {
		t.Fatalf("derived signal changed through caller slice: %q", got)
	}
	if got := input.AcceptedQuery; got != "  PostgreSQL migration policy?  " {
		t.Fatalf("accepted query mutated: %q", got)
	}
}

func TestQueryAnalysisResultValidateRequiresMatchingVersionedIdentity(t *testing.T) {
	input := validQueryAnalysisInput()
	result, err := NewQueryAnalysisResult(input, QueryAnalysisDispositionComplete, nil, nil)
	if err != nil {
		t.Fatalf("NewQueryAnalysisResult() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*QueryAnalysisResult)
		want   string
	}{
		{name: "unknown policy version", mutate: func(result *QueryAnalysisResult) { result.Identity.PolicyVersion = "query-analysis-v999" }, want: "policy version"},
		{name: "unknown limits version", mutate: func(result *QueryAnalysisResult) { result.Identity.LimitsVersion = "query-analysis-limits-v999" }, want: "limits version"},
		{name: "mismatched policy version", mutate: func(result *QueryAnalysisResult) { result.Identity.PolicyVersion = "" }, want: "policy version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invalid := result
			tt.mutate(&invalid)
			if err := invalid.Validate(input); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestQueryAnalysisHintUsesExplicitAbsentAndUnknownDispositions(t *testing.T) {
	input := validQueryAnalysisInput()
	hints := []QueryAnalysisHint{
		{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintAbsent},
		{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintUnknown},
		{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintPresent, Value: "procedural"},
	}
	result, err := NewQueryAnalysisResult(input, QueryAnalysisDispositionComplete, hints, nil)
	if err != nil {
		t.Fatalf("NewQueryAnalysisResult() error = %v", err)
	}
	if err := result.Validate(input); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalid := result
	invalid.Hints = append([]QueryAnalysisHint(nil), result.Hints...)
	invalid.Hints[0].Disposition = QueryAnalysisHintDisposition("invented")
	if err := invalid.Validate(input); err == nil || !strings.Contains(err.Error(), "hint disposition") {
		t.Fatalf("Validate() error = %v, want unknown hint disposition rejection", err)
	}
}

func TestQueryAnalysisResultConstructionIsDeterministic(t *testing.T) {
	input := validQueryAnalysisInput()
	hints := []QueryAnalysisHint{
		{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintUnknown},
		{Kind: QueryAnalysisHintIntent, Disposition: QueryAnalysisHintPresent, Value: "lookup"},
	}
	signals := []QueryAnalysisSignal{
		{Kind: QueryAnalysisSignalTerm, Text: "migration"},
		{Kind: QueryAnalysisSignalSubquery, Text: "What rules govern migration?"},
	}

	first, err := NewQueryAnalysisResult(input, QueryAnalysisDispositionComplete, hints, signals)
	if err != nil {
		t.Fatalf("first NewQueryAnalysisResult() error = %v", err)
	}
	second, err := NewQueryAnalysisResult(input, QueryAnalysisDispositionComplete, hints, signals)
	if err != nil {
		t.Fatalf("second NewQueryAnalysisResult() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("equivalent replay differs:\nfirst:  %+v\nsecond: %+v", first, second)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first result: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second result: %v", err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("equivalent replay JSON differs:\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
}

func TestNewQueryAnalysisResultDefensivelyCopiesAndValidatesCategories(t *testing.T) {
	input := validQueryAnalysisInput()
	categories := []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticOverBudget, Count: 2}}
	result, err := NewQueryAnalysisResult(input, QueryAnalysisDispositionPartial, nil, nil, categories...)
	if err != nil {
		t.Fatalf("NewQueryAnalysisResult() error = %v", err)
	}
	categories[0].Count = 99
	if got := result.Categories; !reflect.DeepEqual(got, []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticOverBudget, Count: 2}}) {
		t.Fatalf("categories changed through caller slice: %#v", got)
	}

	tests := []struct {
		name        string
		disposition QueryAnalysisDisposition
		categories  []QueryAnalysisDiagnosticCount
		want        string
	}{
		{name: "unknown", disposition: QueryAnalysisDispositionPartial, categories: []QueryAnalysisDiagnosticCount{{Category: "invented", Count: 1}}, want: "diagnostic category"},
		{name: "duplicate", disposition: QueryAnalysisDispositionPartial, categories: []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticOverBudget, Count: 1}, {Category: QueryAnalysisDiagnosticOverBudget, Count: 1}}, want: "duplicate"},
		{name: "zero count", disposition: QueryAnalysisDispositionPartial, categories: []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticOverBudget}}, want: "diagnostic count"},
		{name: "unbounded count", disposition: QueryAnalysisDispositionPartial, categories: []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticOverBudget, Count: QueryAnalysisHardMaxDiagnosticCount + 1}}, want: "diagnostic count"},
		{name: "over budget must be partial", disposition: QueryAnalysisDispositionComplete, categories: []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticOverBudget, Count: 1}}, want: "partial"},
		{name: "adversarial must be original only", disposition: QueryAnalysisDispositionPartial, categories: []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticAdversarial, Count: 1}}, want: "original-only"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewQueryAnalysisResult(input, tt.disposition, nil, nil, tt.categories...)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewQueryAnalysisResult() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestQueryAnalysisInputAndLimitsRejectUnknownVersionsAndUnsafeValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*QueryAnalysisInput)
		want   string
	}{
		{name: "empty query", mutate: func(input *QueryAnalysisInput) { input.AcceptedQuery = "" }, want: "accepted query"},
		{name: "invalid UTF-8 query", mutate: func(input *QueryAnalysisInput) { input.AcceptedQuery = string([]byte{0xff}) }, want: "UTF-8"},
		{name: "unknown policy", mutate: func(input *QueryAnalysisInput) { input.PolicyVersion = "query-analysis-v999" }, want: "policy version"},
		{name: "unknown limits", mutate: func(input *QueryAnalysisInput) { input.Limits.Version = "query-analysis-limits-v999" }, want: "limits version"},
		{name: "zero signals", mutate: func(input *QueryAnalysisInput) { input.Limits.MaxSignals = 0 }, want: "max signals"},
		{name: "zero term bytes", mutate: func(input *QueryAnalysisInput) { input.Limits.MaxTermBytes = 0 }, want: "max term bytes"},
		{name: "zero subquery bytes", mutate: func(input *QueryAnalysisInput) { input.Limits.MaxSubqueryBytes = 0 }, want: "max subquery bytes"},
		{name: "zero work", mutate: func(input *QueryAnalysisInput) { input.Limits.MaxAnalysisWork = 0 }, want: "max analysis work"},
		{name: "work exceeds fixed stages", mutate: func(input *QueryAnalysisInput) { input.Limits.MaxAnalysisWork = QueryAnalysisStageCount + 1 }, want: "max analysis work"},
		{name: "zero candidates per signal", mutate: func(input *QueryAnalysisInput) { input.Limits.MaxCandidatesPerSignal = 0 }, want: "max candidates per signal"},
		{name: "zero aggregate candidates", mutate: func(input *QueryAnalysisInput) { input.Limits.MaxAggregateCandidates = 0 }, want: "max aggregate candidates"},
		{name: "zero elapsed", mutate: func(input *QueryAnalysisInput) { input.Limits.MaxElapsed = 0 }, want: "max elapsed"},
		{name: "unbounded signals", mutate: func(input *QueryAnalysisInput) { input.Limits.MaxSignals = QueryAnalysisHardMaxSignals + 1 }, want: "max signals"},
		{name: "unbounded elapsed", mutate: func(input *QueryAnalysisInput) {
			input.Limits.MaxElapsed = QueryAnalysisHardMaxElapsed + time.Nanosecond
		}, want: "max elapsed"},
		{name: "unknown constrained class", mutate: func(input *QueryAnalysisInput) { input.Constraints.Classes = []memory.MemoryClass{"invented"} }, want: "memory class"},
		{name: "duplicate constrained class", mutate: func(input *QueryAnalysisInput) {
			input.Constraints.Classes = []memory.MemoryClass{memory.MemoryClassProfile, memory.MemoryClassProfile}
		}, want: "duplicate memory class"},
		{name: "reversed constrained time", mutate: func(input *QueryAnalysisInput) {
			input.Constraints.TimeFrom = time.Date(2026, time.September, 12, 0, 0, 0, 0, time.UTC)
			input.Constraints.TimeTo = time.Date(2026, time.September, 11, 0, 0, 0, 0, time.UTC)
		}, want: "time constraint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validQueryAnalysisInput()
			tt.mutate(&input)
			if err := input.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestQueryAnalysisLimitsAllowOptionalAnalysisFeaturesDisabled(t *testing.T) {
	t.Run("original only", func(t *testing.T) {
		input := validQueryAnalysisInput()
		input.Limits.MaxSignals = 1
		input.Limits.MaxSubqueries = 0

		result, err := NewQueryAnalysisResult(input, QueryAnalysisDispositionOriginalOnly, nil, nil)
		if err != nil {
			t.Fatalf("NewQueryAnalysisResult() error = %v", err)
		}
		if len(result.Signals) != 1 || result.Signals[0].Kind != QueryAnalysisSignalOriginal {
			t.Fatalf("signals = %+v, want original only", result.Signals)
		}
	})

	t.Run("hints disabled", func(t *testing.T) {
		input := validQueryAnalysisInput()
		input.Limits.MaxHints = 0

		if err := input.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})

	t.Run("subqueries disabled independently", func(t *testing.T) {
		input := validQueryAnalysisInput()
		input.Limits.MaxSignals = 2
		input.Limits.MaxSubqueries = 0

		result, err := NewQueryAnalysisResult(input, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "migration"}})
		if err != nil {
			t.Fatalf("NewQueryAnalysisResult() error = %v", err)
		}
		if len(result.Signals) != 2 || result.Signals[1].Kind != QueryAnalysisSignalTerm {
			t.Fatalf("signals = %+v, want original plus derived term", result.Signals)
		}
	})
}

func TestQueryAnalysisResultRejectsUnsafeSignalsAndHints(t *testing.T) {
	input := validQueryAnalysisInput()
	tests := []struct {
		name    string
		hints   []QueryAnalysisHint
		signals []QueryAnalysisSignal
		want    string
	}{
		{name: "unknown signal", signals: []QueryAnalysisSignal{{Kind: QueryAnalysisSignalKind("invented"), Text: "safe"}}, want: "signal kind"},
		{name: "empty signal", signals: []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm}}, want: "signal text"},
		{name: "duplicate signal", signals: []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "repeat"}, {Kind: QueryAnalysisSignalSubquery, Text: "repeat"}}, want: "duplicate signal"},
		{name: "derived mandatory", signals: []QueryAnalysisSignal{{Kind: QueryAnalysisSignalTerm, Text: "safe", Mandatory: true}}, want: "derived signal"},
		{name: "unknown hint", hints: []QueryAnalysisHint{{Kind: QueryAnalysisHintKind("invented"), Disposition: QueryAnalysisHintAbsent}}, want: "hint kind"},
		{name: "absent hint with value", hints: []QueryAnalysisHint{{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintAbsent, Value: "invented-id"}}, want: "absent"},
		{name: "present hint without value", hints: []QueryAnalysisHint{{Kind: QueryAnalysisHintIntent, Disposition: QueryAnalysisHintPresent}}, want: "present"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewQueryAnalysisResult(input, QueryAnalysisDispositionComplete, tt.hints, tt.signals)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewQueryAnalysisResult() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestQueryAnalysisFallbackCategoriesAreStable(t *testing.T) {
	for _, category := range []QueryAnalysisFallbackCategory{
		QueryAnalysisFallbackNone,
		QueryAnalysisFallbackUnavailable,
		QueryAnalysisFallbackMalformed,
		QueryAnalysisFallbackAdversarial,
		QueryAnalysisFallbackDuplicate,
		QueryAnalysisFallbackOverBudget,
	} {
		diagnostics := validQueryAnalysisDiagnostics()
		diagnostics.Fallback = category
		if err := diagnostics.Validate(DefaultQueryAnalysisLimits()); err != nil {
			t.Errorf("Validate(%q) error = %v", category, err)
		}
	}
}

func TestQueryAnalysisDiagnosticsRejectUnknownAndUnboundedValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*QueryAnalysisDiagnostics)
		want   string
	}{
		{name: "unknown fallback", mutate: func(d *QueryAnalysisDiagnostics) { d.Fallback = "invented" }, want: "fallback category"},
		{name: "unknown category", mutate: func(d *QueryAnalysisDiagnostics) { d.Categories[0].Category = "invented" }, want: "diagnostic category"},
		{name: "duplicate category", mutate: func(d *QueryAnalysisDiagnostics) { d.Categories = append(d.Categories, d.Categories[0]) }, want: "duplicate diagnostic category"},
		{name: "unbounded category count", mutate: func(d *QueryAnalysisDiagnostics) { d.Categories[0].Count = QueryAnalysisHardMaxDiagnosticCount + 1 }, want: "diagnostic count"},
		{name: "unbounded signal count", mutate: func(d *QueryAnalysisDiagnostics) { d.SignalCount = QueryAnalysisHardMaxSignals + 1 }, want: "signal count"},
		{name: "unbounded subquery count", mutate: func(d *QueryAnalysisDiagnostics) { d.SubqueryCount = QueryAnalysisHardMaxSubqueries + 1 }, want: "subquery count"},
		{name: "unbounded candidate count", mutate: func(d *QueryAnalysisDiagnostics) { d.CandidateCount = QueryAnalysisHardMaxAggregateCandidates + 1 }, want: "candidate count"},
		{name: "negative elapsed", mutate: func(d *QueryAnalysisDiagnostics) { d.Elapsed = -time.Nanosecond }, want: "elapsed"},
		{name: "unbounded elapsed", mutate: func(d *QueryAnalysisDiagnostics) { d.Elapsed = QueryAnalysisHardMaxElapsed + time.Nanosecond }, want: "elapsed"},
		{name: "original not retained", mutate: func(d *QueryAnalysisDiagnostics) { d.OriginalRetained = false }, want: "original retained"},
		{name: "missing original signal", mutate: func(d *QueryAnalysisDiagnostics) { d.SignalCount = 0 }, want: "signal count"},
		{name: "subqueries exceed derived signals", mutate: func(d *QueryAnalysisDiagnostics) { d.SubqueryCount = d.SignalCount }, want: "subquery count"},
		{name: "original only has derived signal", mutate: func(d *QueryAnalysisDiagnostics) { d.Disposition = QueryAnalysisDispositionOriginalOnly }, want: "original-only"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnostics := validQueryAnalysisDiagnostics()
			tt.mutate(&diagnostics)
			if err := diagnostics.Validate(DefaultQueryAnalysisLimits()); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
			if _, err := MarshalQueryAnalysisDiagnostics(diagnostics, DefaultQueryAnalysisLimits()); err == nil {
				t.Fatal("MarshalQueryAnalysisDiagnostics() error = nil, want invalid diagnostics rejected")
			}
		})
	}
}

func TestMarshalQueryAnalysisDiagnosticsNeverSerializesQueryOrSubqueryText(t *testing.T) {
	query := "SECRET original query"
	subquery := "SECRET derived subquery"
	input := validQueryAnalysisInput()
	input.AcceptedQuery = query
	_, err := NewQueryAnalysisResult(input, QueryAnalysisDispositionComplete, nil, []QueryAnalysisSignal{{Kind: QueryAnalysisSignalSubquery, Text: subquery}})
	if err != nil {
		t.Fatalf("NewQueryAnalysisResult() error = %v", err)
	}

	encoded, err := MarshalQueryAnalysisDiagnostics(validQueryAnalysisDiagnostics(), input.Limits)
	if err != nil {
		t.Fatalf("MarshalQueryAnalysisDiagnostics() error = %v", err)
	}
	for _, prohibited := range []string{query, subquery, "accepted_query", "normalized_text", "subqueries\"", "reasoning", "candidate_id", "score"} {
		if strings.Contains(strings.ToLower(string(encoded)), strings.ToLower(prohibited)) {
			t.Fatalf("diagnostics contains prohibited value %q: %s", prohibited, encoded)
		}
	}
}

func validQueryAnalysisInput() QueryAnalysisInput {
	return QueryAnalysisInput{
		AcceptedQuery: "Which migration policy was selected?",
		PolicyVersion: QueryAnalysisPolicyVersionV1,
		Limits:        DefaultQueryAnalysisLimits(),
	}
}

func validQueryAnalysisDiagnostics() QueryAnalysisDiagnostics {
	return QueryAnalysisDiagnostics{
		PolicyVersion:    QueryAnalysisPolicyVersionV1,
		LimitsVersion:    QueryAnalysisLimitsVersionV1,
		Disposition:      QueryAnalysisDispositionComplete,
		Fallback:         QueryAnalysisFallbackNone,
		OriginalRetained: true,
		Categories: []QueryAnalysisDiagnosticCount{
			{Category: QueryAnalysisDiagnosticDuplicate, Count: 1},
			{Category: QueryAnalysisDiagnosticOverBudget, Count: 1},
		},
		HintCount:      1,
		SignalCount:    2,
		SubqueryCount:  1,
		CandidateCount: 4,
		Elapsed:        10 * time.Millisecond,
	}
}
