package retrieval

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestTemporalOmissionCategoryIsAClosedSet(t *testing.T) {
	for _, category := range temporalOmissionCategoryOrder {
		if !category.Valid() {
			t.Fatalf("declared category %q must be valid", category)
		}
	}
	for _, category := range []TemporalOmissionCategory{"", "unknown", "expired_version ", "EXPIRED_VERSION", "expired"} {
		if category.Valid() {
			t.Fatalf("category %q must be rejected", category)
		}
	}
}

// TestTemporalOmissionReportIgnoresUnknownCategory proves an unvalidated label
// cannot enter a diagnostics surface; otherwise an arbitrary string could leak
// through a "bounded" report.
func TestTemporalOmissionReportIgnoresUnknownCategory(t *testing.T) {
	var report TemporalOmissionReport
	report.Add(TemporalOmissionExpiredVersion)
	report.Add(TemporalOmissionCategory("expired version: memory_id=abc"))

	if report.Total != 1 {
		t.Fatalf("Total = %d, want 1; unknown categories must not be counted", report.Total)
	}
	if len(report.Categories) != 1 || report.Categories[0].Category != TemporalOmissionExpiredVersion {
		t.Fatalf("Categories = %+v, want only the valid category", report.Categories)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestTemporalOmissionReportAggregatesAndOrdersDeterministically(t *testing.T) {
	var report TemporalOmissionReport
	// Deliberately add out of declaration order to prove Normalize sorts.
	report.Add(TemporalOmissionUnsupportedClass)
	report.Add(TemporalOmissionExpiredVersion)
	report.Add(TemporalOmissionMissingValidityIdentity)
	report.Add(TemporalOmissionExpiredVersion)
	report.Add(TemporalOmissionOutOfInterval)
	report.Add(TemporalOmissionUnsupportedClass)

	if report.Total != 6 {
		t.Fatalf("Total = %d, want 6", report.Total)
	}

	normalized := report.Normalize()
	if err := normalized.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	want := []TemporalOmissionCategory{
		TemporalOmissionExpiredVersion,
		TemporalOmissionOutOfInterval,
		TemporalOmissionMissingValidityIdentity,
		TemporalOmissionUnsupportedClass,
	}
	if len(normalized.Categories) != len(want) {
		t.Fatalf("Categories = %+v, want %d entries", normalized.Categories, len(want))
	}
	for i, category := range want {
		if normalized.Categories[i].Category != category {
			t.Fatalf("Categories[%d] = %q, want %q", i, normalized.Categories[i].Category, category)
		}
	}
	if normalized.Categories[0].Count != 2 || normalized.Categories[3].Count != 2 {
		t.Fatalf("counts = %+v, want 2 for the repeated categories", normalized.Categories)
	}

	// Normalization is idempotent: repeated evaluation is deterministic.
	again := normalized.Normalize()
	if len(again.Categories) != len(normalized.Categories) {
		t.Fatalf("Normalize is not idempotent: %+v vs %+v", again.Categories, normalized.Categories)
	}
	for i := range again.Categories {
		if again.Categories[i] != normalized.Categories[i] {
			t.Fatalf("Normalize is not idempotent at %d: %+v vs %+v", i, again.Categories[i], normalized.Categories[i])
		}
	}
}

func TestTemporalOmissionReportValidateRejectsInconsistentAccounting(t *testing.T) {
	tests := []struct {
		name   string
		report TemporalOmissionReport
	}{
		{
			name:   "unknown category",
			report: TemporalOmissionReport{Categories: []TemporalOmissionCount{{Category: "nope", Count: 1}}, Total: 1},
		},
		{
			name:   "zero count",
			report: TemporalOmissionReport{Categories: []TemporalOmissionCount{{Category: TemporalOmissionExpiredVersion, Count: 0}}, Total: 0},
		},
		{
			name:   "negative count",
			report: TemporalOmissionReport{Categories: []TemporalOmissionCount{{Category: TemporalOmissionExpiredVersion, Count: -1}}, Total: -1},
		},
		{
			name: "duplicate category",
			report: TemporalOmissionReport{Categories: []TemporalOmissionCount{
				{Category: TemporalOmissionExpiredVersion, Count: 1},
				{Category: TemporalOmissionExpiredVersion, Count: 1},
			}, Total: 2},
		},
		{
			name:   "total disagrees with sum",
			report: TemporalOmissionReport{Categories: []TemporalOmissionCount{{Category: TemporalOmissionExpiredVersion, Count: 2}}, Total: 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.report.Validate(); err == nil {
				t.Fatalf("Validate() = nil for %+v, want rejection", tt.report)
			}
		})
	}
}

// TestTemporalOmissionReportCarriesNoIdentifiers is the redaction guarantee: the
// serialized shape must be incapable of holding content, scope, or interval data.
func TestTemporalOmissionReportCarriesNoIdentifiers(t *testing.T) {
	var report TemporalOmissionReport
	report.Add(TemporalOmissionExpiredVersion)
	normalized := report.Normalize()

	if len(normalized.Categories) != 1 {
		t.Fatalf("Categories = %+v", normalized.Categories)
	}
	entry := normalized.Categories[0]
	if entry.Category != TemporalOmissionExpiredVersion || entry.Count != 1 {
		t.Fatalf("entry = %+v, want a bounded label and count only", entry)
	}
}

func TestClassifyTemporalOmissionUsesBoundedReasons(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	current := memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}
	during := memory.TemporalConstraint{Mode: memory.TemporalSelectionDuring, ValidFrom: &start, ValidTo: &end}
	valid := memory.TemporalValidity{TemporalFactID: "fact-1", IngestedAt: start, ValidFrom: start, ValidTo: &end, ValiditySource: memory.TemporalValiditySourceExplicit}

	tests := []struct {
		name       string
		constraint memory.TemporalConstraint
		class      memory.MemoryClass
		validity   memory.TemporalValidity
		want       TemporalOmissionCategory
	}{
		{
			name:       "derived summary class",
			constraint: current,
			class:      memory.MemoryClassSummary,
			validity:   valid,
			want:       TemporalOmissionUnsupportedClass,
		},
		{
			name:       "derived relation class",
			constraint: current,
			class:      memory.MemoryClassRelation,
			validity:   valid,
			want:       TemporalOmissionUnsupportedClass,
		},
		{
			name:       "partially populated snapshot is malformed",
			constraint: current,
			class:      memory.MemoryClassEpisodic,
			validity:   memory.TemporalValidity{IngestedAt: start},
			want:       TemporalOmissionMissingValidityIdentity,
		},
		{
			name:       "invalid interval",
			constraint: current,
			class:      memory.MemoryClassEpisodic,
			validity:   memory.TemporalValidity{TemporalFactID: "f", IngestedAt: start, ValidFrom: start, ValidTo: &start, ValiditySource: memory.TemporalValiditySourceExplicit},
			want:       TemporalOmissionMissingValidityIdentity,
		},
		{
			name:       "expired under current selection",
			constraint: current,
			class:      memory.MemoryClassProfile,
			validity:   valid,
			want:       TemporalOmissionExpiredVersion,
		},
		{
			name:       "outside requested interval",
			constraint: during,
			class:      memory.MemoryClassProcedural,
			validity:   valid,
			want:       TemporalOmissionOutOfInterval,
		},
		{
			name:       "legacy unset snapshot is not malformed",
			constraint: current,
			class:      memory.MemoryClassEpisodic,
			validity:   memory.TemporalValidity{},
			want:       TemporalOmissionExpiredVersion,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyTemporalOmission(tt.constraint, tt.class, tt.validity)
			if got != tt.want {
				t.Fatalf("classifyTemporalOmission() = %q, want %q", got, tt.want)
			}
			if !got.Valid() {
				t.Fatalf("classifier produced an unvalidated category %q", got)
			}
		})
	}
}

func TestTemporalOmissionCategoryOrderCoversEveryValidCategory(t *testing.T) {
	covered := make(map[TemporalOmissionCategory]struct{}, len(temporalOmissionCategoryOrder))
	for _, category := range temporalOmissionCategoryOrder {
		if _, duplicate := covered[category]; duplicate {
			t.Fatalf("category %q appears twice in the declaration order", category)
		}
		covered[category] = struct{}{}
	}
	for _, category := range []TemporalOmissionCategory{
		TemporalOmissionExpiredVersion, TemporalOmissionOutOfInterval,
		TemporalOmissionMissingValidityIdentity, TemporalOmissionUnsupportedClass,
	} {
		if _, ok := covered[category]; !ok {
			t.Fatalf("category %q is valid but missing from the declaration order", category)
		}
	}
	if len(covered) != 4 {
		t.Fatalf("declaration order has %d categories, want 4", len(covered))
	}
}

// TestTemporalOmissionReportIsNotPartOfTheJSONResponseShape keeps ordinary API
// responses free of temporal accounting: the field is unexported, so encoding a
// SearchResult can never serialize omission detail.
func TestTemporalOmissionReportIsNotPartOfTheJSONResponseShape(t *testing.T) {
	result := SearchResult{Hits: []SearchHit{}}
	if err := result.temporalOmissions.Validate(); err != nil {
		t.Fatalf("zero-value omission report must validate: %v", err)
	}
	report := result.TemporalOmissionReport()
	if report.Total != 0 || len(report.Categories) != 0 {
		t.Fatalf("empty result exposed omissions: %+v", report)
	}
}

func TestTemporalOmissionReportAddIsIdempotentPerCandidate(t *testing.T) {
	var report TemporalOmissionReport
	for i := 0; i < 5; i++ {
		report.Add(TemporalOmissionExpiredVersion)
	}
	if report.Total != 5 {
		t.Fatalf("Total = %d, want 5", report.Total)
	}
	if len(report.Categories) != 1 || report.Categories[0].Count != 5 {
		t.Fatalf("Categories = %+v, want a single entry with count 5", report.Categories)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
