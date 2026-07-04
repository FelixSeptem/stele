package insights

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/FelixSeptem/stele/internal/memory"
)

type FailureEvidence struct {
	Kind       memory.DerivedInsightEvidenceKind
	ID         string
	FailureKey string
	Message    string
	ObservedAt time.Time
	Metadata   map[string]any
}

func (e FailureEvidence) Validate() error {
	switch {
	case !e.Kind.Valid():
		return fmt.Errorf("failure evidence kind %q is invalid", e.Kind)
	case strings.TrimSpace(e.ID) == "":
		return fmt.Errorf("failure evidence id is required")
	case NormalizeFailureKey(e.FailureKey, e.Message) == "":
		return fmt.Errorf("failure evidence key is required")
	default:
		return nil
	}
}

type FailurePatternEvaluator struct {
	MinimumEvidence int
	Window          time.Duration
	Now             func() time.Time
}

func (e FailurePatternEvaluator) Evaluate(scope memory.Scope, evidence []FailureEvidence) ([]memory.DerivedInsight, error) {
	scope = scope.Normalized()
	if err := scope.Validate(); err != nil {
		return nil, err
	}

	minimum := e.MinimumEvidence
	if minimum <= 0 {
		minimum = 2
	}
	window := e.Window
	if window <= 0 {
		window = 24 * time.Hour
	}

	now := time.Now
	if e.Now != nil {
		now = e.Now
	}
	current := now().UTC()

	groups := map[string][]FailureEvidence{}
	windows := map[string]failureEvidenceWindow{}
	for _, item := range evidence {
		if err := item.Validate(); err != nil {
			return nil, err
		}
		observedAt := item.ObservedAt.UTC()
		if observedAt.IsZero() {
			observedAt = current
			item.ObservedAt = observedAt
		}

		key := NormalizeFailureKey(item.FailureKey, item.Message)
		windowStart := observedAt.Truncate(window)
		windowEnd := windowStart.Add(window)
		groupKey := failurePatternGroupKey(scope, item.Kind, key, windowStart, windowEnd)
		groups[groupKey] = append(groups[groupKey], item)
		windows[groupKey] = failureEvidenceWindow{
			normalizedKey: key,
			kind:          item.Kind,
			start:         windowStart,
			end:           windowEnd,
		}
	}

	fingerprints := make([]string, 0, len(groups))
	for groupKey, items := range groups {
		if len(uniqueFailureEvidence(items)) < minimum {
			continue
		}
		fingerprints = append(fingerprints, groupKey)
	}
	sort.Strings(fingerprints)

	insights := make([]memory.DerivedInsight, 0, len(fingerprints))
	for _, groupKey := range fingerprints {
		windowInfo := windows[groupKey]
		items := uniqueFailureEvidence(groups[groupKey])
		sort.Slice(items, func(i, j int) bool {
			if items[i].ObservedAt.Equal(items[j].ObservedAt) {
				return items[i].ID < items[j].ID
			}
			return items[i].ObservedAt.Before(items[j].ObservedAt)
		})

		fingerprint := FailurePatternFingerprint(scope, windowInfo.kind, windowInfo.normalizedKey, windowInfo.start, windowInfo.end)
		evidenceRefs := make([]memory.DerivedInsightEvidenceRef, 0, len(items))
		lastObservedAt := time.Time{}
		for _, item := range items {
			evidenceRefs = append(evidenceRefs, memory.DerivedInsightEvidenceRef{
				Kind:       item.Kind,
				ID:         item.ID,
				Relation:   memory.DerivedInsightEvidenceRelationSupports,
				ObservedAt: item.ObservedAt.UTC(),
				Metadata:   item.Metadata,
			})
			if item.ObservedAt.After(lastObservedAt) {
				lastObservedAt = item.ObservedAt.UTC()
			}
		}

		confidence := float64(len(items)) / float64(minimum*2)
		if confidence > 1 {
			confidence = 1
		}

		insight := memory.DerivedInsight{
			ID:    stableInsightID("insight", fingerprint),
			Scope: scope,
			Type:  memory.DerivedInsightTypeFailurePattern,
			State: memory.DerivedInsightStateActive,
			Title: fmt.Sprintf("Repeated failure: %s", strings.ReplaceAll(windowInfo.normalizedKey, "_", " ")),
			Summary: fmt.Sprintf(
				"%d %s failure records matched %q in the evidence window.",
				len(items),
				windowInfo.kind,
				windowInfo.normalizedKey,
			),
			Confidence: memory.DerivedInsightConfidence{
				Score:  confidence,
				Method: "repeated_evidence_ratio",
			},
			Payload: map[string]any{
				"normalized_key": windowInfo.normalizedKey,
				"evidence_kind":  string(windowInfo.kind),
				"evidence_count": float64(len(items)),
			},
			Derivation: memory.DerivedInsightDerivation{
				Source:              "failure_pattern_evaluator",
				Fingerprint:         fingerprint,
				EvidenceWindowStart: windowInfo.start,
				EvidenceWindowEnd:   windowInfo.end,
				DerivedAt:           current,
				Metadata: map[string]any{
					"minimum_evidence": float64(minimum),
				},
			},
			Evidence:       evidenceRefs,
			LastObservedAt: lastObservedAt,
			CreatedAt:      current,
			UpdatedAt:      current,
		}
		if err := insight.Validate(); err != nil {
			return nil, err
		}
		insights = append(insights, insight)
	}

	return insights, nil
}

func ProjectLesson(pattern memory.DerivedInsight) (memory.DerivedInsight, error) {
	if err := pattern.Validate(); err != nil {
		return memory.DerivedInsight{}, err
	}
	if pattern.Type != memory.DerivedInsightTypeFailurePattern {
		return memory.DerivedInsight{}, fmt.Errorf("source insight must be a failure_pattern")
	}

	derivedAt := pattern.UpdatedAt
	if derivedAt.IsZero() {
		derivedAt = pattern.Derivation.DerivedAt
	}
	if derivedAt.IsZero() {
		derivedAt = time.Now().UTC()
	}

	lesson := memory.DerivedInsight{
		ID:      stableInsightID("lesson", pattern.ID),
		Scope:   pattern.Scope,
		Type:    memory.DerivedInsightTypeLesson,
		State:   memory.DerivedInsightStateActive,
		Title:   "Lesson from " + pattern.Title,
		Summary: "Evidence-backed guidance derived from failure pattern " + pattern.ID + ".",
		Confidence: memory.DerivedInsightConfidence{
			Score:  pattern.Confidence.Score,
			Method: pattern.Confidence.Method,
		},
		Lesson: &memory.DerivedInsightLesson{
			SourceFailurePatternID: pattern.ID,
			Guidance:               "Before repeating this workflow, account for the known failure pattern: " + pattern.Summary,
		},
		Derivation: memory.DerivedInsightDerivation{
			Source:      "lesson_projection",
			Fingerprint: "lesson:" + pattern.Derivation.Fingerprint,
			DerivedAt:   derivedAt,
			Metadata: map[string]any{
				"source_failure_pattern_id": pattern.ID,
			},
		},
		Evidence:       append([]memory.DerivedInsightEvidenceRef(nil), pattern.Evidence...),
		LastObservedAt: pattern.LastObservedAt,
		CreatedAt:      derivedAt,
		UpdatedAt:      derivedAt,
	}
	if err := lesson.Validate(); err != nil {
		return memory.DerivedInsight{}, err
	}

	return lesson, nil
}

func FailurePatternFingerprint(scope memory.Scope, kind memory.DerivedInsightEvidenceKind, failureKey string, windowStart, windowEnd time.Time) string {
	normalized := NormalizeFailureKey(failureKey, "")
	parts := []string{
		"failure_pattern",
		scope.Normalized().Tenant,
		scope.Normalized().Project,
		scope.Normalized().Namespace,
		string(kind),
		normalized,
		windowStart.UTC().Format(time.RFC3339),
		windowEnd.UTC().Format(time.RFC3339),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "failure_pattern:" + hex.EncodeToString(sum[:])
}

func NormalizeFailureKey(values ...string) string {
	for _, value := range values {
		normalized := normalizeFailureKey(value)
		if normalized != "" {
			return normalized
		}
	}
	return ""
}

type failureEvidenceWindow struct {
	normalizedKey string
	kind          memory.DerivedInsightEvidenceKind
	start         time.Time
	end           time.Time
}

func failurePatternGroupKey(scope memory.Scope, kind memory.DerivedInsightEvidenceKind, failureKey string, windowStart, windowEnd time.Time) string {
	return strings.Join([]string{
		scope.Tenant,
		scope.Project,
		scope.Namespace,
		string(kind),
		failureKey,
		windowStart.UTC().Format(time.RFC3339),
		windowEnd.UTC().Format(time.RFC3339),
	}, "\x00")
}

func uniqueFailureEvidence(evidence []FailureEvidence) []FailureEvidence {
	seen := map[string]struct{}{}
	items := make([]FailureEvidence, 0, len(evidence))
	for _, item := range evidence {
		key := string(item.Kind) + "\x00" + item.ID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, item)
	}
	return items
}

func stableInsightID(prefix, seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return prefix + "_" + hex.EncodeToString(sum[:16])
}

func normalizeFailureKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}

	var builder strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}

	return strings.Trim(builder.String(), "_")
}
