package retrieval

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestRuleBasedQueryAnalyzerNormalizesDeterministically(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{name: "ASCII whitespace and case", query: "  PostgreSQL\t MIGRATION \n policy  ", want: "postgresql migration policy"},
		{name: "Unicode compatibility and whitespace", query: "  ＰｏｓｔｇｒｅＳＱＬ\u3000迁移  ", want: "postgresql 迁移"},
		{name: "Unicode canonical equivalence", query: "Cafe\u0301", want: "café"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validQueryAnalysisInput()
			input.AcceptedQuery = tt.query

			result, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if got := signalTexts(result, QueryAnalysisSignalNormalized); !reflect.DeepEqual(got, []string{tt.want}) {
				t.Fatalf("normalized signals = %#v, want %#v", got, []string{tt.want})
			}
			if result.Signals[0].Text != tt.query {
				t.Fatalf("original query = %q, want exact %q", result.Signals[0].Text, tt.query)
			}
		})
	}
}

func TestRuleBasedQueryAnalyzerBoundsAndOrdersMixedLanguageTerms(t *testing.T) {
	input := validQueryAnalysisInput()
	input.AcceptedQuery = "PostgreSQL migration 数据库 迁移 postgres"
	input.Limits.MaxSignals = 5

	want := []QueryAnalysisSignal{
		{Kind: QueryAnalysisSignalOriginal, Text: input.AcceptedQuery, Mandatory: true},
		{Kind: QueryAnalysisSignalNormalized, Text: "postgresql migration 数据库 迁移 postgres"},
		{Kind: QueryAnalysisSignalTerm, Text: "postgres"},
		{Kind: QueryAnalysisSignalTerm, Text: "迁移"},
		{Kind: QueryAnalysisSignalTerm, Text: "database"},
	}
	first, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	second, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
	if err != nil {
		t.Fatalf("second Analyze() error = %v", err)
	}
	if !reflect.DeepEqual(first.Signals, want) {
		t.Fatalf("signals = %#v, want %#v", first.Signals, want)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("replayed analysis differs:\nfirst:  %#v\nsecond: %#v", first, second)
	}
}

func TestRuleBasedQueryAnalyzerUsesUTF8SafeLengthTruncation(t *testing.T) {
	input := validQueryAnalysisInput()
	input.AcceptedQuery = "数据库迁移"
	input.Limits.MaxTermBytes = 7

	result, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	for _, signal := range result.Signals[1:] {
		if len(signal.Text) > input.Limits.MaxTermBytes {
			t.Fatalf("signal %q has %d bytes, max %d", signal.Text, len(signal.Text), input.Limits.MaxTermBytes)
		}
		if !utf8.ValidString(signal.Text) {
			t.Fatalf("signal is malformed UTF-8: %q", signal.Text)
		}
	}
	if got := result.Signals[1].Text; got != "数据" {
		t.Fatalf("truncated normalized signal = %q, want %q", got, "数据")
	}
}

func TestRuleBasedQueryAnalyzerRejectsEmptyOversizedAndMalformedInput(t *testing.T) {
	tests := []struct {
		name  string
		query string
		limit int
		want  string
	}{
		{name: "empty", query: "", limit: 32, want: "accepted query"},
		{name: "oversized", query: "123456", limit: 5, want: "max query bytes"},
		{name: "malformed UTF-8", query: string([]byte{0xff, 'x'}), limit: 32, want: "UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validQueryAnalysisInput()
			input.AcceptedQuery = tt.query
			input.Limits.MaxQueryBytes = tt.limit
			if _, err := (RuleBasedQueryAnalyzer{}).Analyze(input); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Analyze() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRuleBasedQueryAnalyzerFallsBackOnAdversarialInput(t *testing.T) {
	input := validQueryAnalysisInput()
	input.AcceptedQuery = "postgresql\x00migration"

	result, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Disposition != QueryAnalysisDispositionOriginalOnly || len(result.Signals) != 1 {
		t.Fatalf("result = %#v, want original-only adversarial fallback", result)
	}
	assertAnalysisCategory(t, result, QueryAnalysisDiagnosticAdversarial, 1)
}

func TestRuleBasedQueryAnalyzerExtractsBoundedHints(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []QueryAnalysisHint
	}{
		{
			name:  "recognized hints",
			query: "find procedural memory about entity:PostgreSQL on 2026-09-11",
			want: []QueryAnalysisHint{
				{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "postgresql"},
				{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintPresent, Value: "2026-09-11"},
				{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintPresent, Value: "procedural"},
				{Kind: QueryAnalysisHintIntent, Disposition: QueryAnalysisHintPresent, Value: "lookup"},
			},
		},
		{
			name:  "unknown and absent hints",
			query: "compare entity:PostgreSQL with entity:pgvector on 2026-09-11 or 2026-09-12",
			want: []QueryAnalysisHint{
				{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintUnknown},
				{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintUnknown},
				{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintAbsent},
				{Kind: QueryAnalysisHintIntent, Disposition: QueryAnalysisHintUnknown},
			},
		},
		{
			name:  "mixed-language explicit rules",
			query: "查找 entity:数据库 的流程记忆",
			want: []QueryAnalysisHint{
				{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "数据库"},
				{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintAbsent},
				{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintPresent, Value: "procedural"},
				{Kind: QueryAnalysisHintIntent, Disposition: QueryAnalysisHintPresent, Value: "lookup"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validQueryAnalysisInput()
			input.AcceptedQuery = tt.query
			result, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if !reflect.DeepEqual(result.Hints, tt.want) {
				t.Fatalf("hints = %#v, want %#v", result.Hints, tt.want)
			}
		})
	}
}

func TestRuleBasedQueryAnalyzerDiscardsHintsConflictingWithExplicitFilters(t *testing.T) {
	input := validQueryAnalysisInput()
	input.AcceptedQuery = "find procedural memory on 2026-09-11"
	input.Constraints.Classes = []memory.MemoryClass{memory.MemoryClassProfile}
	input.Constraints.TimeFrom = time.Date(2026, time.September, 12, 0, 0, 0, 0, time.UTC)
	input.Constraints.TimeTo = time.Date(2026, time.September, 13, 0, 0, 0, 0, time.UTC)

	result, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	wantByKind := map[QueryAnalysisHintKind]QueryAnalysisHint{
		QueryAnalysisHintTemporal:    {Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintUnknown},
		QueryAnalysisHintMemoryClass: {Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintUnknown},
	}
	for _, hint := range result.Hints {
		if want, ok := wantByKind[hint.Kind]; ok && hint != want {
			t.Fatalf("conflicting %s hint = %#v, want %#v", hint.Kind, hint, want)
		}
	}
	if !reflect.DeepEqual(input.Constraints.Classes, []memory.MemoryClass{memory.MemoryClassProfile}) {
		t.Fatalf("classes widened or mutated: %#v", input.Constraints.Classes)
	}
	if got := input.Constraints.TimeFrom; !got.Equal(time.Date(2026, time.September, 12, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("time constraint mutated: %s", got)
	}
}

func TestRuleBasedQueryAnalyzerNeverInventsEntityIdentifiers(t *testing.T) {
	input := validQueryAnalysisInput()
	input.AcceptedQuery = "find memories about PostgreSQL"

	result, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if got := result.Hints[0]; got != (QueryAnalysisHint{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintAbsent}) {
		t.Fatalf("implicit entity hint = %#v, want absent instead of invented identifier", got)
	}
}

func TestRuleBasedQueryAnalyzerRequiresEntityMarkerBoundaryAndPreservesSemanticValue(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		maxTerm int
		want    QueryAnalysisHint
	}{
		{
			name:    "embedded marker",
			query:   "find nonentity:postgresql",
			maxTerm: 256,
			want:    QueryAnalysisHint{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintAbsent},
		},
		{
			name:    "overlong semantic entity",
			query:   "find entity:postgresql",
			maxTerm: 5,
			want:    QueryAnalysisHint{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintUnknown},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validQueryAnalysisInput()
			input.AcceptedQuery = tt.query
			input.Limits.MaxTermBytes = tt.maxTerm
			result, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if got := result.Hints[0]; got != tt.want {
				t.Fatalf("entity hint = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRuleBasedQueryAnalyzerUsesMemoryClassPhraseBoundariesAndNegation(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  QueryAnalysisHint
	}{
		{
			name:  "embedded phrase",
			query: "find nonprocedural memory",
			want:  QueryAnalysisHint{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintAbsent},
		},
		{
			name:  "recognized negation",
			query: "find not procedural memory",
			want:  QueryAnalysisHint{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintUnknown},
		},
		{
			name:  "Chinese positive phrase preserved",
			query: "查找流程记忆",
			want:  QueryAnalysisHint{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintPresent, Value: "procedural"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validQueryAnalysisInput()
			input.AcceptedQuery = tt.query
			result, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if got := result.Hints[2]; got != tt.want {
				t.Fatalf("memory class hint = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRuleBasedQueryAnalyzerTruncatesHintsInStableKindOrder(t *testing.T) {
	input := validQueryAnalysisInput()
	input.AcceptedQuery = "find procedural memory about entity:PostgreSQL on 2026-09-11"
	input.Limits.MaxHints = 2

	result, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	want := []QueryAnalysisHint{
		{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: "postgresql"},
		{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintPresent, Value: "2026-09-11"},
	}
	if !reflect.DeepEqual(result.Hints, want) {
		t.Fatalf("hints = %#v, want stable truncation %#v", result.Hints, want)
	}
	if result.Disposition != QueryAnalysisDispositionPartial {
		t.Fatalf("disposition = %q, want partial for hint truncation", result.Disposition)
	}
	assertAnalysisCategory(t, result, QueryAnalysisDiagnosticOverBudget, 2)
}

func TestRuleBasedQueryAnalyzerDecomposesRecognizableMultiHopQueries(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "ordered English clauses",
			query: "what policy was selected and what migration followed",
			want:  []string{"what policy was selected", "what migration followed"},
		},
		{
			name:  "ordered Chinese clauses",
			query: "查找数据库策略，然后查找迁移步骤",
			want:  []string{"查找数据库策略", "查找迁移步骤"},
		},
		{
			name:  "duplicate clauses",
			query: "what policy was selected and what policy was selected",
			want:  []string{"what policy was selected"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validQueryAnalysisInput()
			input.AcceptedQuery = tt.query
			result, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if got := signalTexts(result, QueryAnalysisSignalSubquery); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("subqueries = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRuleBasedQueryAnalyzerBoundsSubqueriesAndTotalSignals(t *testing.T) {
	input := validQueryAnalysisInput()
	input.AcceptedQuery = "find alpha and find beta and find gamma and find delta"
	input.Limits.MaxSubqueries = 2
	input.Limits.MaxSignals = 4

	result, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if got := signalTexts(result, QueryAnalysisSignalSubquery); !reflect.DeepEqual(got, []string{"find alpha", "find beta"}) {
		t.Fatalf("subqueries = %#v, want stable first two", got)
	}
	if len(result.Signals) != 3 || len(result.Signals) > input.Limits.MaxSignals {
		t.Fatalf("signal count = %d, want three valid signals within max %d", len(result.Signals), input.Limits.MaxSignals)
	}
	if result.Disposition != QueryAnalysisDispositionPartial {
		t.Fatalf("disposition = %q, want partial for subquery truncation", result.Disposition)
	}
	assertAnalysisCategory(t, result, QueryAnalysisDiagnosticOverBudget, 2)
}

func TestRuleBasedQueryAnalyzerRecordsEachCountTruncation(t *testing.T) {
	tests := []struct {
		name  string
		input QueryAnalysisInput
		count int
	}{
		{
			name: "maximum signals",
			input: func() QueryAnalysisInput {
				input := validQueryAnalysisInput()
				input.AcceptedQuery = "PostgreSQL migration"
				input.Limits.MaxSignals = 2
				input.Limits.MaxSubqueries = 1
				return input
			}(),
			count: 2,
		},
		{
			name: "maximum subqueries",
			input: func() QueryAnalysisInput {
				input := validQueryAnalysisInput()
				input.AcceptedQuery = "find alpha and find beta and find gamma"
				input.Limits.MaxSubqueries = 1
				return input
			}(),
			count: 2,
		},
		{
			name: "maximum hints",
			input: func() QueryAnalysisInput {
				input := validQueryAnalysisInput()
				input.AcceptedQuery = "find procedural memory about entity:PostgreSQL on 2026-09-11"
				input.Limits.MaxHints = 1
				return input
			}(),
			count: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := (RuleBasedQueryAnalyzer{}).Analyze(tt.input)
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			if result.Disposition != QueryAnalysisDispositionPartial {
				t.Fatalf("disposition = %q, want partial", result.Disposition)
			}
			assertAnalysisCategory(t, result, QueryAnalysisDiagnosticOverBudget, tt.count)
		})
	}
}

func TestRuleBasedQueryAnalyzerStopsAtWorkBudget(t *testing.T) {
	input := validQueryAnalysisInput()
	input.AcceptedQuery = "find procedural PostgreSQL migration on 2026-09-11 and find database steps"
	input.Limits.MaxAnalysisWork = 1

	result, err := (RuleBasedQueryAnalyzer{}).Analyze(input)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.WorkUnits != input.Limits.MaxAnalysisWork {
		t.Fatalf("work units = %d, want exhausted bound %d", result.WorkUnits, input.Limits.MaxAnalysisWork)
	}
	if got := signalTexts(result, QueryAnalysisSignalSubquery); len(got) != 0 {
		t.Fatalf("subqueries = %#v, want none after work exhaustion", got)
	}
	if got := signalTexts(result, QueryAnalysisSignalTerm); len(got) != 0 {
		t.Fatalf("terms = %#v, want alias stage not executed", got)
	}
	if len(result.Hints) != 0 {
		t.Fatalf("hints = %#v, want hint stages not executed", result.Hints)
	}
	if got := signalTexts(result, QueryAnalysisSignalNormalized); !reflect.DeepEqual(got, []string{"find procedural postgresql migration on 2026-09-11 and find database steps"}) {
		t.Fatalf("normalized signals = %#v, want normalization-only output", got)
	}
	if result.Disposition != QueryAnalysisDispositionPartial {
		t.Fatalf("disposition = %q, want partial", result.Disposition)
	}
	assertAnalysisCategory(t, result, QueryAnalysisDiagnosticOverBudget, 1)
}

func TestRuleBasedQueryAnalyzerHasNoIOOrProviderState(t *testing.T) {
	typeOfAnalyzer := reflect.TypeOf(RuleBasedQueryAnalyzer{})
	if typeOfAnalyzer.NumField() != 0 {
		t.Fatalf("RuleBasedQueryAnalyzer has %d fields, want stateless pure analyzer", typeOfAnalyzer.NumField())
	}
	method, ok := typeOfAnalyzer.MethodByName("Analyze")
	if !ok || method.Type.NumIn() != 2 || method.Type.In(1) != reflect.TypeOf(QueryAnalysisInput{}) {
		t.Fatalf("Analyze method = %#v, want only value input and no repository/network/provider dependency", method)
	}
}

func FuzzRuleBasedQueryAnalyzerIsBoundedDeterministicAndImmutable(f *testing.F) {
	seeds := []struct {
		query      []byte
		maxHints   uint8
		maxSignals uint8
		maxSubs    uint8
		maxTerm    uint16
		maxSub     uint16
		maxWork    uint16
	}{
		{query: []byte("   \t\n"), maxHints: 4, maxSignals: 8, maxSubs: 4, maxTerm: 256, maxSub: 1024, maxWork: 1000},
		{query: []byte("PostgreSQL migration 数据库 迁移 postgres"), maxHints: 2, maxSignals: 5, maxSubs: 2, maxTerm: 7, maxSub: 19, maxWork: 8},
		{query: []byte("find entity:PostgreSQL on 2026-09-11"), maxHints: 4, maxSignals: 4, maxSubs: 1, maxTerm: 1, maxSub: 8, maxWork: 20},
		{query: []byte("what policy was selected and what migration followed"), maxHints: 0, maxSignals: 3, maxSubs: 2, maxTerm: 13, maxSub: 17, maxWork: 4},
		{query: []byte{0xff, 0xfe, 0x00}, maxHints: 4, maxSignals: 8, maxSubs: 4, maxTerm: 256, maxSub: 1024, maxWork: 1000},
	}
	for _, seed := range seeds {
		f.Add(seed.query, seed.maxHints, seed.maxSignals, seed.maxSubs, seed.maxTerm, seed.maxSub, seed.maxWork)
	}

	f.Fuzz(func(t *testing.T, query []byte, maxHints, maxSignals, maxSubs uint8, maxTerm, maxSub, maxWork uint16) {
		originalBytes := append([]byte(nil), query...)
		input := validQueryAnalysisInput()
		input.AcceptedQuery = string(query)
		input.Limits.MaxQueryBytes = min(max(1, len(query)), QueryAnalysisHardMaxQueryBytes)
		input.Limits.MaxHints = int(maxHints) % (QueryAnalysisHardMaxHints + 1)
		input.Limits.MaxSignals = int(maxSignals)%QueryAnalysisHardMaxSignals + 1
		input.Limits.MaxSubqueries = min(int(maxSubs)%(QueryAnalysisHardMaxSubqueries+1), input.Limits.MaxSignals-1)
		input.Limits.MaxTermBytes = int(maxTerm)%QueryAnalysisHardMaxTermBytes + 1
		input.Limits.MaxSubqueryBytes = int(maxSub)%QueryAnalysisHardMaxSubqueryBytes + 1
		input.Limits.MaxAnalysisWork = int(maxWork)%QueryAnalysisHardMaxAnalysisWork + 1

		inputErr := input.Validate()
		first, firstErr := (RuleBasedQueryAnalyzer{}).Analyze(input)
		second, secondErr := (RuleBasedQueryAnalyzer{}).Analyze(input)
		if !bytes.Equal(query, originalBytes) || input.AcceptedQuery != string(originalBytes) {
			t.Fatalf("Analyze() mutated original input")
		}
		if (firstErr == nil) != (secondErr == nil) || (firstErr != nil && firstErr.Error() != secondErr.Error()) {
			t.Fatalf("nondeterministic errors: first=%v second=%v", firstErr, secondErr)
		}
		if firstErr != nil {
			if inputErr == nil {
				t.Fatalf("valid bounded input produced analyzer error: %v", firstErr)
			}
			return
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("nondeterministic result:\nfirst:  %#v\nsecond: %#v", first, second)
		}
		if err := first.Validate(input); err != nil {
			t.Fatalf("result violates configured bounds: %v; result=%#v", err, first)
		}
		if first.Signals[0].Text != string(originalBytes) || !first.Signals[0].Mandatory {
			t.Fatalf("original signal changed: %#v", first.Signals[0])
		}
		if len(first.Hints) > input.Limits.MaxHints || len(first.Signals) > input.Limits.MaxSignals || first.WorkUnits > input.Limits.MaxAnalysisWork {
			t.Fatalf("count/work bound exceeded: hints=%d signals=%d work=%d", len(first.Hints), len(first.Signals), first.WorkUnits)
		}
		seen := make(map[string]struct{}, len(first.Signals))
		subqueries := 0
		for _, signal := range first.Signals {
			if _, duplicate := seen[signal.Text]; duplicate {
				t.Fatalf("duplicate signal: %q", signal.Text)
			}
			seen[signal.Text] = struct{}{}
			if signal.Kind == QueryAnalysisSignalSubquery {
				subqueries++
				if len(signal.Text) > input.Limits.MaxSubqueryBytes {
					t.Fatalf("subquery byte bound exceeded: %d > %d", len(signal.Text), input.Limits.MaxSubqueryBytes)
				}
			} else if signal.Kind != QueryAnalysisSignalOriginal && len(signal.Text) > input.Limits.MaxTermBytes {
				t.Fatalf("term byte bound exceeded: %d > %d", len(signal.Text), input.Limits.MaxTermBytes)
			}
		}
		if subqueries > input.Limits.MaxSubqueries {
			t.Fatalf("subquery count exceeded: %d > %d", subqueries, input.Limits.MaxSubqueries)
		}
		if len(first.Signals) == 1 && first.Disposition != QueryAnalysisDispositionOriginalOnly && first.Disposition != QueryAnalysisDispositionPartial {
			t.Fatalf("no derived signals but disposition = %q", first.Disposition)
		}
	})
}

func signalTexts(result QueryAnalysisResult, kind QueryAnalysisSignalKind) []string {
	var texts []string
	for _, signal := range result.Signals {
		if signal.Kind == kind {
			texts = append(texts, signal.Text)
		}
	}
	return texts
}

func assertAnalysisCategory(t *testing.T, result QueryAnalysisResult, category QueryAnalysisDiagnosticCategory, count int) {
	t.Helper()
	for _, got := range result.Categories {
		if got.Category == category {
			if got.Count != count {
				t.Fatalf("category %q count = %d, want %d; categories=%#v", category, got.Count, count, result.Categories)
			}
			return
		}
	}
	t.Fatalf("categories = %#v, want %q count %d", result.Categories, category, count)
}
