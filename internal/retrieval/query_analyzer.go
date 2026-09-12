package retrieval

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/FelixSeptem/stele/internal/memory"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// RuleBasedQueryAnalyzer is a pure, deterministic query analyzer. Its rules are
// deliberately finite: changing them requires a new query-analysis policy
// version so recorded evaluations remain replayable.
type RuleBasedQueryAnalyzer struct{}

// Analyze derives optional lexical signals while retaining AcceptedQuery as
// the exact mandatory first signal. It performs no I/O.
func (RuleBasedQueryAnalyzer) Analyze(input QueryAnalysisInput) (QueryAnalysisResult, error) {
	if err := input.Validate(); err != nil {
		return QueryAnalysisResult{}, err
	}
	if containsAdversarialControl(input.AcceptedQuery) {
		return NewQueryAnalysisResult(input, QueryAnalysisDispositionOriginalOnly, nil, nil,
			QueryAnalysisDiagnosticCount{Category: QueryAnalysisDiagnosticAdversarial, Count: 1})
	}

	derived := make([]QueryAnalysisSignal, 0, input.Limits.MaxSignals-1)
	hints := make([]QueryAnalysisHint, 0, input.Limits.MaxHints)
	seen := map[string]struct{}{input.AcceptedQuery: {}}
	workUnits := 0
	overBudgetCount := 0
	consumeWork := func() bool {
		if workUnits >= input.Limits.MaxAnalysisWork {
			overBudgetCount++
			return false
		}
		workUnits++
		return true
	}
	appendSignal := func(kind QueryAnalysisSignalKind, value string, maxBytes int) {
		if len(derived) >= input.Limits.MaxSignals-1 {
			overBudgetCount++
			return
		}
		value = truncateUTF8(value, maxBytes)
		if value == "" {
			return
		}
		if _, duplicate := seen[value]; duplicate {
			return
		}
		seen[value] = struct{}{}
		derived = append(derived, QueryAnalysisSignal{Kind: kind, Text: value})
	}
	finish := func() (QueryAnalysisResult, error) {
		disposition := QueryAnalysisDispositionComplete
		var categories []QueryAnalysisDiagnosticCount
		if overBudgetCount > 0 {
			disposition = QueryAnalysisDispositionPartial
			categories = []QueryAnalysisDiagnosticCount{{Category: QueryAnalysisDiagnosticOverBudget, Count: overBudgetCount}}
		} else if len(derived) == 0 {
			disposition = QueryAnalysisDispositionOriginalOnly
		}
		result, err := NewQueryAnalysisResult(input, disposition, hints, derived, categories...)
		if err != nil {
			return QueryAnalysisResult{}, err
		}
		result.WorkUnits = workUnits
		if err := result.Validate(input); err != nil {
			return QueryAnalysisResult{}, err
		}
		return result, nil
	}

	// One work unit authorizes one bounded stage before that stage executes.
	// The stages are normalization, aliases, the four individual hint kinds,
	// and decomposition, in that fixed order.
	if !consumeWork() {
		return finish()
	}
	normalized := normalizeQueryText(input.AcceptedQuery)
	appendSignal(QueryAnalysisSignalNormalized, normalized, input.Limits.MaxTermBytes)

	if input.Limits.MaxSignals > 1 {
		if !consumeWork() {
			return finish()
		}
		for _, term := range mixedLanguageTerms(normalized) {
			appendSignal(QueryAnalysisSignalTerm, term, input.Limits.MaxTermBytes)
		}
	}

	if input.Limits.MaxHints > 0 {
		hintStages := []func() QueryAnalysisHint{
			func() QueryAnalysisHint { return extractEntityHint(normalized, input.Limits.MaxTermBytes) },
			func() QueryAnalysisHint { return extractTemporalHint(normalized, input.Constraints) },
			func() QueryAnalysisHint { return extractMemoryClassHint(normalized, input.Constraints) },
			func() QueryAnalysisHint { return extractIntentHint(normalized) },
		}
		for _, extract := range hintStages {
			if !consumeWork() {
				return finish()
			}
			hint := boundHintValue(extract(), input.Limits.MaxTermBytes)
			if len(hints) >= input.Limits.MaxHints {
				overBudgetCount++
				continue
			}
			hints = append(hints, hint)
		}
	}

	if input.Limits.MaxSignals > 1 && input.Limits.MaxSubqueries > 0 {
		if !consumeWork() {
			return finish()
		}
		subqueries := decomposeQuery(normalized)
		retained := 0
		for _, subquery := range subqueries {
			if retained >= input.Limits.MaxSubqueries {
				overBudgetCount++
				continue
			}
			before := len(derived)
			appendSignal(QueryAnalysisSignalSubquery, subquery, input.Limits.MaxSubqueryBytes)
			if len(derived) > before {
				retained++
			}
		}
	}

	return finish()
}

func normalizeQueryText(value string) string {
	return strings.Join(strings.Fields(cases.Fold().String(norm.NFKC.String(value))), " ")
}

func containsAdversarialControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return true
		}
	}
	folded := cases.Fold().String(value)
	return strings.Contains(folded, "../") ||
		(strings.Contains(folded, "ignore bounds") && strings.Contains(folded, "tenant"))
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return strings.TrimSpace(value)
}

var queryTermAliases = map[string][]string{
	"postgresql": {"postgres"},
	"postgres":   {"postgresql"},
	"migration":  {"迁移"},
	"迁移":         {"migration"},
	"database":   {"数据库"},
	"数据库":        {"database"},
}

func mixedLanguageTerms(normalized string) []string {
	var terms []string
	seen := make(map[string]struct{})
	for _, token := range strings.FieldsFunc(normalized, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	}) {
		for _, alias := range queryTermAliases[token] {
			if _, duplicate := seen[alias]; duplicate {
				continue
			}
			seen[alias] = struct{}{}
			terms = append(terms, alias)
		}
	}
	return terms
}

var isoDatePattern = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)

func boundHintValue(hint QueryAnalysisHint, maxBytes int) QueryAnalysisHint {
	if hint.Disposition == QueryAnalysisHintPresent && len(hint.Value) > maxBytes {
		hint.Disposition = QueryAnalysisHintUnknown
		hint.Value = ""
	}
	return hint
}

func extractEntityHint(normalized string, maxBytes int) QueryAnalysisHint {
	values := boundedMarkerMatches(normalized, "entity:", func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) && r != '-' && r != '_'
	})
	if len(values) == 0 {
		return QueryAnalysisHint{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintAbsent}
	}
	if len(values) != 1 {
		return QueryAnalysisHint{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintUnknown}
	}
	value := values[0]
	if value == "" || len(value) > maxBytes {
		return QueryAnalysisHint{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintUnknown}
	}
	return QueryAnalysisHint{Kind: QueryAnalysisHintEntity, Disposition: QueryAnalysisHintPresent, Value: value}
}

func boundedMarkerMatches(text, prefix string, stop func(rune) bool) []string {
	var values []string
	seen := make(map[string]struct{})
	for offset := 0; offset < len(text); {
		index := strings.Index(text[offset:], prefix)
		if index < 0 {
			break
		}
		markerStart := offset + index
		start := markerStart + len(prefix)
		if markerStart > 0 {
			previous, _ := utf8.DecodeLastRuneInString(text[:markerStart])
			if unicode.IsLetter(previous) || unicode.IsNumber(previous) || previous == '_' {
				offset = start
				continue
			}
		}
		end := start
		for end < len(text) {
			r, size := utf8.DecodeRuneInString(text[end:])
			if stop(r) {
				break
			}
			end += size
		}
		value := strings.TrimSpace(text[start:end])
		if value != "" {
			if _, duplicate := seen[value]; !duplicate {
				seen[value] = struct{}{}
				values = append(values, value)
			}
		}
		offset = start
		if end > start {
			offset = end
		}
	}
	return values
}

func extractTemporalHint(normalized string, constraints QueryAnalysisConstraints) QueryAnalysisHint {
	matches := isoDatePattern.FindAllString(normalized, -1)
	values := dedupeStrings(matches)
	if len(values) == 0 {
		return QueryAnalysisHint{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintAbsent}
	}
	if len(values) != 1 {
		return QueryAnalysisHint{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintUnknown}
	}
	date, err := time.Parse("2006-01-02", values[0])
	if err != nil || (!constraints.TimeFrom.IsZero() && date.Before(constraints.TimeFrom)) ||
		(!constraints.TimeTo.IsZero() && date.After(constraints.TimeTo)) {
		return QueryAnalysisHint{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintUnknown}
	}
	return QueryAnalysisHint{Kind: QueryAnalysisHintTemporal, Disposition: QueryAnalysisHintPresent, Value: values[0]}
}

var memoryClassPhrases = []struct {
	class   memory.MemoryClass
	phrases []string
}{
	{class: memory.MemoryClassProfile, phrases: []string{"profile memory", "profile memories", "档案记忆"}},
	{class: memory.MemoryClassEpisodic, phrases: []string{"episodic memory", "episodic memories", "事件记忆"}},
	{class: memory.MemoryClassProcedural, phrases: []string{"procedural memory", "procedural memories", "流程记忆"}},
	{class: memory.MemoryClassSummary, phrases: []string{"summary memory", "summary memories", "摘要记忆"}},
	{class: memory.MemoryClassRelation, phrases: []string{"relation memory", "relation memories", "关系记忆"}},
}

func extractMemoryClassHint(normalized string, constraints QueryAnalysisConstraints) QueryAnalysisHint {
	for _, candidate := range memoryClassPhrases {
		for _, phrase := range candidate.phrases {
			if isASCIIPhrase(phrase) && containsBoundedPhrase(normalized, "not "+phrase) {
				return QueryAnalysisHint{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintUnknown}
			}
		}
	}
	var classes []memory.MemoryClass
	for _, candidate := range memoryClassPhrases {
		for _, phrase := range candidate.phrases {
			if phraseMatches(normalized, phrase) {
				classes = append(classes, candidate.class)
				break
			}
		}
	}
	if len(classes) == 0 {
		return QueryAnalysisHint{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintAbsent}
	}
	if len(classes) != 1 || classConflicts(classes[0], constraints.Classes) {
		return QueryAnalysisHint{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintUnknown}
	}
	return QueryAnalysisHint{Kind: QueryAnalysisHintMemoryClass, Disposition: QueryAnalysisHintPresent, Value: string(classes[0])}
}

func phraseMatches(text, phrase string) bool {
	if !isASCIIPhrase(phrase) {
		return strings.Contains(text, phrase)
	}
	return containsBoundedPhrase(text, phrase)
}

func isASCIIPhrase(phrase string) bool {
	for _, r := range phrase {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func containsBoundedPhrase(text, phrase string) bool {
	for offset := 0; offset <= len(text)-len(phrase); {
		index := strings.Index(text[offset:], phrase)
		if index < 0 {
			return false
		}
		start := offset + index
		end := start + len(phrase)
		leftBounded := start == 0
		if !leftBounded {
			previous, _ := utf8.DecodeLastRuneInString(text[:start])
			leftBounded = !unicode.IsLetter(previous) && !unicode.IsNumber(previous) && previous != '_'
		}
		rightBounded := end == len(text)
		if !rightBounded {
			next, _ := utf8.DecodeRuneInString(text[end:])
			rightBounded = !unicode.IsLetter(next) && !unicode.IsNumber(next) && next != '_'
		}
		if leftBounded && rightBounded {
			return true
		}
		offset = start + 1
	}
	return false
}

func classConflicts(class memory.MemoryClass, explicit []memory.MemoryClass) bool {
	if len(explicit) == 0 {
		return false
	}
	for _, allowed := range explicit {
		if allowed == class {
			return false
		}
	}
	return true
}

func extractIntentHint(normalized string) QueryAnalysisHint {
	lookup := containsWord(normalized, "find") || containsWord(normalized, "lookup") || strings.Contains(normalized, "查找")
	compare := containsWord(normalized, "compare") || strings.Contains(normalized, "比较")
	if lookup && !compare {
		return QueryAnalysisHint{Kind: QueryAnalysisHintIntent, Disposition: QueryAnalysisHintPresent, Value: "lookup"}
	}
	if compare || lookup {
		return QueryAnalysisHint{Kind: QueryAnalysisHintIntent, Disposition: QueryAnalysisHintUnknown}
	}
	return QueryAnalysisHint{Kind: QueryAnalysisHintIntent, Disposition: QueryAnalysisHintAbsent}
}

func containsWord(text, word string) bool {
	for _, field := range strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
		if field == word {
			return true
		}
	}
	return false
}

func dedupeStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func decomposeQuery(normalized string) []string {
	separators := []string{"，然后", ",然后", ", then ", "; then ", " and "}
	parts := []string{normalized}
	for _, separator := range separators {
		var split []string
		for _, part := range parts {
			split = append(split, strings.Split(part, separator)...)
		}
		parts = split
	}
	if len(parts) < 2 {
		return nil
	}
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), "?!。！？")
		if part == "" {
			continue
		}
		if _, duplicate := seen[part]; duplicate {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	return result
}
