package retrieval

import (
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
)

// TemporalOmissionCategory is the bounded, low-cardinality reason a candidate
// version was left out of a temporal search result. It is deliberately a closed
// set: diagnostics may serialize these labels and a count, and nothing else, so
// an omission report can never leak content, identifiers, or interval details.
type TemporalOmissionCategory string

const (
	// TemporalOmissionExpiredVersion is a version whose validity interval does
	// not contain the evaluation instant under current selection.
	TemporalOmissionExpiredVersion TemporalOmissionCategory = "expired_version"
	// TemporalOmissionOutOfInterval is a version excluded because it does not
	// overlap an explicitly requested historical interval.
	TemporalOmissionOutOfInterval TemporalOmissionCategory = "out_of_interval"
	// TemporalOmissionMissingValidityIdentity is a version whose temporal
	// snapshot is present but malformed, so it fails closed instead of being
	// treated as legacy current-compatible.
	TemporalOmissionMissingValidityIdentity TemporalOmissionCategory = "missing_validity_identity"
	// TemporalOmissionUnsupportedClass is a memory class that carries no fact
	// validity, such as a derived summary or relation.
	TemporalOmissionUnsupportedClass TemporalOmissionCategory = "unsupported_class"
)

func (category TemporalOmissionCategory) Valid() bool {
	switch category {
	case TemporalOmissionExpiredVersion, TemporalOmissionOutOfInterval,
		TemporalOmissionMissingValidityIdentity, TemporalOmissionUnsupportedClass:
		return true
	default:
		return false
	}
}

// TemporalOmissionCount is an allowlisted aggregate: a bounded label plus a
// count. It has no field able to carry query text, scope values, memory
// identifiers, intervals, or raw scores.
type TemporalOmissionCount struct {
	Category TemporalOmissionCategory `json:"category"`
	Count    int                      `json:"count"`
}

// TemporalOmissionReport summarizes how many candidate versions a temporal
// predicate removed and why. Counts are non-negative and categories are sorted
// by declaration order so repeated evaluation is deterministic.
type TemporalOmissionReport struct {
	Categories []TemporalOmissionCount `json:"categories,omitempty"`
	Total      int                     `json:"total"`
}

// Add records one omitted candidate under a validated category. An unknown
// category is ignored rather than widening the label set, so a caller cannot
// smuggle an unbounded string into a diagnostics surface.
func (report *TemporalOmissionReport) Add(category TemporalOmissionCategory) {
	if !category.Valid() {
		return
	}
	for i := range report.Categories {
		if report.Categories[i].Category == category {
			report.Categories[i].Count++
			report.Total++
			return
		}
	}
	report.Categories = append(report.Categories, TemporalOmissionCount{Category: category, Count: 1})
	report.Total++
}

// Normalize orders categories deterministically and drops zero counts.
func (report TemporalOmissionReport) Normalize() TemporalOmissionReport {
	ordered := make([]TemporalOmissionCount, 0, len(temporalOmissionCategoryOrder))
	for _, category := range temporalOmissionCategoryOrder {
		for _, entry := range report.Categories {
			if entry.Category == category && entry.Count > 0 {
				ordered = append(ordered, entry)
				break
			}
		}
	}
	return TemporalOmissionReport{Categories: ordered, Total: report.Total}
}

// Validate rejects a report that could not have been produced by Add: unknown
// categories, negative or zero counts, duplicates, or a total that disagrees
// with the sum of its categories.
func (report TemporalOmissionReport) Validate() error {
	sum := 0
	seen := make(map[TemporalOmissionCategory]struct{}, len(report.Categories))
	for _, entry := range report.Categories {
		if !entry.Category.Valid() {
			return fmt.Errorf("unknown temporal omission category %q", entry.Category)
		}
		if entry.Count <= 0 {
			return fmt.Errorf("temporal omission count for %q must be positive", entry.Category)
		}
		if _, duplicate := seen[entry.Category]; duplicate {
			return fmt.Errorf("duplicate temporal omission category %q", entry.Category)
		}
		seen[entry.Category] = struct{}{}
		sum += entry.Count
	}
	if sum != report.Total {
		return fmt.Errorf("temporal omission total %d does not match category sum %d", report.Total, sum)
	}
	return nil
}

// temporalOmissionCategoryOrder fixes the serialization order so two equivalent
// reports are byte-identical regardless of the order candidates were omitted.
var temporalOmissionCategoryOrder = []TemporalOmissionCategory{
	TemporalOmissionExpiredVersion,
	TemporalOmissionOutOfInterval,
	TemporalOmissionMissingValidityIdentity,
	TemporalOmissionUnsupportedClass,
}

// classifyTemporalOmission returns the bounded reason a version is not
// selectable under the supplied constraint. It is derived only from the
// version's memory class and validity snapshot; it never inspects content or
// identity, so the resulting category carries no hidden detail.
//
// The evaluation instant is passed in rather than read here so every omission in
// one request is attributed against the same clock.
func classifyTemporalOmission(constraint memory.TemporalConstraint, class memory.MemoryClass, validity memory.TemporalValidity) TemporalOmissionCategory {
	if !memory.IsTemporalFactClass(class) {
		return TemporalOmissionUnsupportedClass
	}
	// A partially populated snapshot is malformed, not legacy: only a wholly
	// unset snapshot is allowed to resolve as legacy current-compatible.
	if !validity.IsUnset() && validity.Validate() != nil {
		return TemporalOmissionMissingValidityIdentity
	}
	if constraint.Mode == memory.TemporalSelectionDuring {
		return TemporalOmissionOutOfInterval
	}
	return TemporalOmissionExpiredVersion
}
