package memory

import (
	"errors"
	"testing"
	"time"
)

func TestTemporalValidityValidateUsesHalfOpenIntervals(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := now.Add(24 * time.Hour)
	tests := []struct {
		name    string
		input   TemporalValidity
		wantErr error
	}{
		{name: "valid open ended", input: TemporalValidity{TemporalFactID: "fact-1", IngestedAt: now, ValidFrom: now, ValiditySource: TemporalValiditySourceExplicit}},
		{name: "valid bounded", input: TemporalValidity{TemporalFactID: "fact-1", IngestedAt: now, ValidFrom: now, ValidTo: &end, ValiditySource: TemporalValiditySourceExplicit}},
		{name: "missing identity", input: TemporalValidity{IngestedAt: now, ValidFrom: now}, wantErr: ErrTemporalFactIdentityRequired},
		{name: "missing recorded time", input: TemporalValidity{TemporalFactID: "fact-1", ValidFrom: now}, wantErr: ErrTemporalIngestedAtRequired},
		{name: "missing valid from", input: TemporalValidity{TemporalFactID: "fact-1", IngestedAt: now}, wantErr: ErrTemporalValidFromRequired},
		{name: "closed interval", input: TemporalValidity{TemporalFactID: "fact-1", IngestedAt: now, ValidFrom: now, ValidTo: func() *time.Time { v := now; return &v }()}, wantErr: ErrTemporalIntervalInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestTemporalValidityContainsUsesInclusiveStartExclusiveEnd(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	validity := TemporalValidity{TemporalFactID: "fact-1", IngestedAt: start, ValidFrom: start, ValidTo: &end, ValiditySource: TemporalValiditySourceExplicit}
	if !validity.Contains(start) {
		t.Fatal("validity does not contain inclusive start")
	}
	if validity.Contains(end) {
		t.Fatal("validity contains exclusive end")
	}
}

func TestTemporalConstraintValidateRejectsAmbiguousSelectors(t *testing.T) {
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	validTo := asOf.Add(time.Hour)
	tests := []struct {
		name    string
		input   TemporalConstraint
		wantErr error
	}{
		{name: "current", input: TemporalConstraint{Mode: TemporalSelectionCurrent}},
		{name: "as of", input: TemporalConstraint{Mode: TemporalSelectionAsOf, AsOf: &asOf}},
		{name: "interval", input: TemporalConstraint{Mode: TemporalSelectionDuring, ValidFrom: &asOf, ValidTo: &validTo}},
		{name: "missing as of", input: TemporalConstraint{Mode: TemporalSelectionAsOf}, wantErr: ErrTemporalAsOfRequired},
		{name: "ambiguous", input: TemporalConstraint{Mode: TemporalSelectionAsOf, AsOf: &asOf, ValidFrom: &asOf, ValidTo: &validTo}, wantErr: ErrTemporalConstraintAmbiguous},
		{name: "invalid interval", input: TemporalConstraint{Mode: TemporalSelectionDuring, ValidFrom: &validTo, ValidTo: &asOf}, wantErr: ErrTemporalIntervalInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestTemporalValidityIsUnsetDetectsAbsentSnapshot(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		input TemporalValidity
		want  bool
	}{
		{name: "zero value", input: TemporalValidity{}, want: true},
		{name: "identity only", input: TemporalValidity{TemporalFactID: "fact-1"}},
		{name: "recorded time only", input: TemporalValidity{IngestedAt: start}},
		{name: "valid from only", input: TemporalValidity{ValidFrom: start}},
		{name: "bounded interval only", input: TemporalValidity{ValidFrom: start, ValidTo: &start}},
		{name: "source only", input: TemporalValidity{ValiditySource: TemporalValiditySourceMigrated}},
		{name: "fully populated", input: TemporalValidity{TemporalFactID: "fact-1", IngestedAt: start, ValidFrom: start, ValiditySource: TemporalValiditySourceExplicit}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.IsUnset(); got != tt.want {
				t.Fatalf("IsUnset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTemporalValidityLegacyCurrentCompatibleBackfillsOpenInterval(t *testing.T) {
	reference := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("unset uses fallback reference", func(t *testing.T) {
		got := TemporalValidity{}.LegacyCurrentCompatible(reference)
		if err := got.Validate(); err != nil {
			t.Fatalf("backfilled validity is invalid: %v", err)
		}
		if got.ValiditySource != TemporalValiditySourceLegacyCompatible {
			t.Fatalf("ValiditySource = %q, want %q", got.ValiditySource, TemporalValiditySourceLegacyCompatible)
		}
		if !got.ValidFrom.Equal(reference) || !got.IngestedAt.Equal(reference) {
			t.Fatalf("backfill did not anchor on reference: %+v", got)
		}
		if got.ValidTo != nil {
			t.Fatalf("backfill must leave the interval open, got ValidTo = %v", *got.ValidTo)
		}
	})

	t.Run("partial snapshot is treated as malformed not legacy", func(t *testing.T) {
		recorded := reference.Add(-48 * time.Hour)
		partial := TemporalValidity{IngestedAt: recorded}
		if partial.IsUnset() {
			t.Fatal("a partially populated snapshot must not report IsUnset")
		}
		if got := partial.LegacyCurrentCompatible(reference); got != partial {
			t.Fatalf("partial snapshot was rewritten: %+v", got)
		}
		if partial.Validate() == nil {
			t.Fatal("partial snapshot unexpectedly validates")
		}
		if partial.Contains(reference) {
			t.Fatal("partial snapshot must fail closed")
		}
	})

	t.Run("unset without any reference stays unset", func(t *testing.T) {
		got := TemporalValidity{}.LegacyCurrentCompatible(time.Time{})
		if !got.IsUnset() {
			t.Fatalf("backfill invented a snapshot without a reference: %+v", got)
		}
	})

	t.Run("populated snapshot is never rewritten", func(t *testing.T) {
		end := reference.Add(time.Hour)
		original := TemporalValidity{TemporalFactID: "fact-1", IngestedAt: reference, ValidFrom: reference, ValidTo: &end, ValiditySource: TemporalValiditySourceExplicit}
		if got := original.LegacyCurrentCompatible(reference); got != original {
			t.Fatalf("backfill mutated an explicit snapshot: %+v", got)
		}
	})

	t.Run("malformed snapshot is not silently repaired", func(t *testing.T) {
		malformed := TemporalValidity{TemporalFactID: "fact-1", IngestedAt: reference, ValidTo: &reference}
		if got := malformed.LegacyCurrentCompatible(reference); got != malformed {
			t.Fatalf("malformed interval was repaired: %+v", got)
		}
		if malformed.Validate() == nil {
			t.Fatal("malformed interval unexpectedly validates")
		}
	})
}

func TestTemporalConstraintMatchesTreatsUnsetValidityAsLegacyCurrent(t *testing.T) {
	reference := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	past := reference.Add(-24 * time.Hour)
	future := reference.Add(24 * time.Hour)
	malformed := TemporalValidity{TemporalFactID: "fact-1", IngestedAt: reference, ValidTo: &reference}

	tests := []struct {
		name       string
		constraint TemporalConstraint
		validity   TemporalValidity
		at         time.Time
		want       bool
	}{
		{
			name:       "unset is current valid",
			constraint: TemporalConstraint{Mode: TemporalSelectionCurrent},
			validity:   TemporalValidity{},
			at:         reference,
			want:       true,
		},
		{
			name:       "unset resolves from the evaluation instant itself",
			constraint: TemporalConstraint{Mode: TemporalSelectionCurrent},
			validity:   TemporalValidity{},
			at:         future,
			want:       true,
		},
		{
			name:       "unset fails closed without evaluation instant",
			constraint: TemporalConstraint{Mode: TemporalSelectionCurrent},
			validity:   TemporalValidity{},
			at:         time.Time{},
		},
		{
			name:       "malformed interval fails closed",
			constraint: TemporalConstraint{Mode: TemporalSelectionCurrent},
			validity:   malformed,
			at:         reference,
		},
		{
			name:       "expired explicit interval is excluded",
			constraint: TemporalConstraint{Mode: TemporalSelectionCurrent},
			validity:   TemporalValidity{TemporalFactID: "fact-2", IngestedAt: past, ValidFrom: past, ValidTo: func() *time.Time { v := reference.Add(-time.Hour); return &v }(), ValiditySource: TemporalValiditySourceExplicit},
			at:         reference,
		},
		{
			name:       "invalid constraint rejects everything",
			constraint: TemporalConstraint{Mode: TemporalSelectionAsOf},
			validity:   TemporalValidity{},
			at:         reference,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.constraint.Matches(tt.validity, tt.at); got != tt.want {
				t.Fatalf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTemporalFactClassPolicy(t *testing.T) {
	for _, class := range []MemoryClass{MemoryClassProfile, MemoryClassEpisodic, MemoryClassProcedural} {
		if !IsTemporalFactClass(class) {
			t.Fatalf("IsTemporalFactClass(%q) = false", class)
		}
	}
	for _, class := range []MemoryClass{MemoryClassSummary, MemoryClassRelation} {
		if IsTemporalFactClass(class) {
			t.Fatalf("IsTemporalFactClass(%q) = true", class)
		}
	}
}
