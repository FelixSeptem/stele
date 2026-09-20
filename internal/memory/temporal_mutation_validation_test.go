package memory

import (
	"errors"
	"testing"
	"time"
)

func temporalMutationTimes() (time.Time, time.Time, time.Time) {
	recorded := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	validFrom := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	validTo := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	return recorded, validFrom, validTo
}

func explicitValidity(factID string, recorded, validFrom time.Time, validTo *time.Time) TemporalValidity {
	return TemporalValidity{
		TemporalFactID: factID,
		IngestedAt:     recorded,
		ValidFrom:      validFrom,
		ValidTo:        validTo,
		ValiditySource: TemporalValiditySourceExplicit,
	}
}

// TestTemporalFactClassRequiresExplicitIdentity proves the central 3.3 rule: a
// factual class must carry a temporal identity and a validity interval, because
// without them a fact cannot be scoped to the time it was true and instead
// silently behaves like an eternal current fact.
func TestTemporalFactClassRequiresExplicitIdentity(t *testing.T) {
	recorded, validFrom, _ := temporalMutationTimes()

	for _, class := range []MemoryClass{MemoryClassProfile, MemoryClassEpisodic, MemoryClassProcedural} {
		t.Run(string(class)+" without validity", func(t *testing.T) {
			err := ValidateTemporalForClass(class, TemporalValidity{}, false)
			if !errors.Is(err, ErrTemporalIdentityRequiredForClass) {
				t.Fatalf("error = %v, want ErrTemporalIdentityRequiredForClass", err)
			}
		})

		t.Run(string(class)+" with explicit validity", func(t *testing.T) {
			validity := explicitValidity("fact-1", recorded, validFrom, nil)
			if err := ValidateTemporalForClass(class, validity, false); err != nil {
				t.Fatalf("error = %v, want nil for a well-formed factual snapshot", err)
			}
		})
	}
}

// TestTemporalFactClassAcceptsLegacyCompatibleAfterBackfill proves the migration
// path stays open: a row whose validity came from the backfill is legitimate for
// a factual class, because the backfill derives identity and interval from the
// recorded time rather than inventing them.
func TestTemporalFactClassAcceptsLegacyCompatibleAfterBackfill(t *testing.T) {
	recorded, _, _ := temporalMutationTimes()
	validity := TemporalValidity{
		TemporalFactID: "memory-1",
		IngestedAt:     recorded,
		ValidFrom:      recorded,
		ValiditySource: TemporalValiditySourceLegacyCompatible,
	}

	for _, class := range []MemoryClass{MemoryClassProfile, MemoryClassEpisodic, MemoryClassProcedural} {
		if err := ValidateTemporalForClass(class, validity, false); err != nil {
			t.Fatalf("%s: error = %v, want nil for a backfilled snapshot", class, err)
		}
	}
}

// TestNonTemporalFactClassToleratesUnsetValidity proves the requirement is
// scoped to factual classes: derived artifacts such as summaries and relations
// have no fact-valid time of their own and must not be forced to invent one.
func TestNonTemporalFactClassToleratesUnsetValidity(t *testing.T) {
	for _, class := range []MemoryClass{MemoryClassSummary, MemoryClassRelation} {
		if IsTemporalFactClass(class) {
			t.Fatalf("%s must not be a temporal fact class", class)
		}
		if err := ValidateTemporalForClass(class, TemporalValidity{}, false); err != nil {
			t.Fatalf("%s: error = %v, want nil for an unset snapshot", class, err)
		}
	}
}

// TestTemporalValidationRejectsPartiallyPopulatedSnapshot proves a half-filled
// snapshot fails closed. IsUnset() is true only when every field is empty, so a
// snapshot with some fields set is malformed rather than legacy; treating it as
// legacy would let a caller bypass the identity requirement by filling in a
// single field.
func TestTemporalValidationRejectsPartiallyPopulatedSnapshot(t *testing.T) {
	recorded, validFrom, _ := temporalMutationTimes()

	tests := []struct {
		name     string
		validity TemporalValidity
		wantErr  error
		class    MemoryClass
	}{
		{
			name:     "identity only",
			class:    MemoryClassProfile,
			validity: TemporalValidity{TemporalFactID: "fact-1"},
			wantErr:  ErrTemporalIngestedAtRequired,
		},
		{
			name:     "recorded time only",
			class:    MemoryClassProfile,
			validity: TemporalValidity{IngestedAt: recorded},
			wantErr:  ErrTemporalFactIdentityRequired,
		},
		{
			name:     "missing valid_from",
			class:    MemoryClassProfile,
			validity: TemporalValidity{TemporalFactID: "fact-1", IngestedAt: recorded, ValiditySource: TemporalValiditySourceExplicit},
			wantErr:  ErrTemporalValidFromRequired,
		},
		{
			name:  "closed interval",
			class: MemoryClassProfile,
			validity: func() TemporalValidity {
				closed := validFrom
				return explicitValidity("fact-1", recorded, validFrom, &closed)
			}(),
			wantErr: ErrTemporalIntervalInvalid,
		},
		{
			name:  "invalid validity source",
			class: MemoryClassProfile,
			validity: TemporalValidity{
				TemporalFactID: "fact-1",
				IngestedAt:     recorded,
				ValidFrom:      validFrom,
				ValiditySource: "guessed",
			},
			wantErr: ErrTemporalValiditySourceInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTemporalForClass(tt.class, tt.validity, false)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestTemporalValidationRequiresExplicitSourceWhenRequested proves the strict
// mode used by consolidation: an inferred or backfilled snapshot is not enough
// when the caller is asserting a fact-valid interval deliberately.
func TestTemporalValidationRequiresExplicitSourceWhenRequested(t *testing.T) {
	recorded, validFrom, _ := temporalMutationTimes()
	backfilled := TemporalValidity{
		TemporalFactID: "memory-1",
		IngestedAt:     recorded,
		ValidFrom:      recorded,
		ValiditySource: TemporalValiditySourceLegacyCompatible,
	}

	err := ValidateTemporalForClass(MemoryClassProfile, backfilled, true)
	if !errors.Is(err, ErrTemporalValiditySourceNotExplicit) {
		t.Fatalf("error = %v, want ErrTemporalValiditySourceNotExplicit", err)
	}

	explicit := explicitValidity("fact-1", recorded, validFrom, nil)
	if err := ValidateTemporalForClass(MemoryClassProfile, explicit, true); err != nil {
		t.Fatalf("error = %v, want nil for an explicit snapshot", err)
	}
}

// TestTemporalValidationRejectsClosedIntervalForFactualClass pins half-open
// semantics on the write path: [from, to) with to <= from is not an interval.
func TestTemporalValidationRejectsClosedIntervalForFactualClass(t *testing.T) {
	recorded, validFrom, _ := temporalMutationTimes()
	before := validFrom.Add(-time.Hour)

	err := ValidateTemporalForClass(MemoryClassProfile, explicitValidity("fact-1", recorded, validFrom, &before), false)
	if !errors.Is(err, ErrTemporalIntervalInvalid) {
		t.Fatalf("error = %v, want ErrTemporalIntervalInvalid for an inverted interval", err)
	}
}

// TestTemporalValidationRejectsIdentityChangeForExistingFact proves an existing
// fact cannot be re-pointed at a different temporal identity: that would fork
// the lineage, and the correction ledger could no longer account for the
// versions between the two identities.
func TestTemporalValidationRejectsIdentityChangeForExistingFact(t *testing.T) {
	recorded, validFrom, _ := temporalMutationTimes()
	existing := explicitValidity("fact-original", recorded, validFrom, nil)

	err := ValidateTemporalTransition(MemoryClassProfile, existing, explicitValidity("fact-rebound", recorded, validFrom, nil), false)
	if !errors.Is(err, ErrTemporalIdentityChangeRejected) {
		t.Fatalf("error = %v, want ErrTemporalIdentityChangeRejected", err)
	}
}

// TestTemporalValidationAllowsSupersedeWithSameIdentity proves the ordinary
// supersede path stays open: keeping the identity and moving valid_from forward
// is exactly how a mutable fact changes over time.
func TestTemporalValidationAllowsSupersedeWithSameIdentity(t *testing.T) {
	recorded, validFrom, _ := temporalMutationTimes()
	existing := explicitValidity("fact-1", recorded, validFrom, nil)
	next := explicitValidity("fact-1", recorded.Add(time.Hour), validFrom.Add(24*time.Hour), nil)

	if err := ValidateTemporalTransition(MemoryClassProfile, existing, next, false); err != nil {
		t.Fatalf("error = %v, want nil for a same-identity supersede", err)
	}
}

// TestTemporalValidationRejectsBackdatedInvalidBeforeStateChange proves a
// correction that would be refused is refused by the pure validator, so the
// caller can reject it before opening a transaction. The companion guarantee —
// that canonical and provenance state stay unchanged on rejection — is proven at
// the repository level.
func TestTemporalValidationRejectsBackdatedInvalidBeforeStateChange(t *testing.T) {
	recorded, validFrom, validTo := temporalMutationTimes()

	// A well-formed but already-expired interval is still valid, so a retroactive
	// correction to a past window must be accepted rather than mistaken for error.
	expired := explicitValidity("fact-1", recorded, validFrom, &validTo)
	if err := ValidateTemporalForClass(MemoryClassProfile, expired, true); err != nil {
		t.Fatalf("error = %v, want nil: a past valid window is a legitimate retroactive correction", err)
	}

	// An interval that never had a start is not.
	unstartable := TemporalValidity{
		TemporalFactID: "fact-1",
		IngestedAt:     recorded,
		ValiditySource: TemporalValiditySourceExplicit,
	}
	if err := ValidateTemporalForClass(MemoryClassProfile, unstartable, true); !errors.Is(err, ErrTemporalValidFromRequired) {
		t.Fatalf("error = %v, want ErrTemporalValidFromRequired", err)
	}
}

// TestTemporalValidationIgnoresNonFactualTransitions proves the transition rule
// is scoped like the snapshot rule: derived artifacts are not blocked by
// identity churn they cannot have.
func TestTemporalValidationIgnoresNonFactualTransitions(t *testing.T) {
	if err := ValidateTemporalTransition(MemoryClassRelation, TemporalValidity{}, TemporalValidity{}, false); err != nil {
		t.Fatalf("error = %v, want nil for a non-factual class", err)
	}
}
