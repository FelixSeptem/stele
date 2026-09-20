package memory

import (
	"errors"
)

var (
	// ErrTemporalIdentityRequiredForClass rejects a factual-class write that
	// carries no validity snapshot at all. Without one the fact would behave as
	// an eternal current fact: it could never be scoped to the time it was true
	// and could never be superseded by a correction.
	ErrTemporalIdentityRequiredForClass = errors.New("temporal identity is required for factual memory classes")
	// ErrTemporalValiditySourceNotExplicit rejects an inferred snapshot where the
	// caller is deliberately asserting a fact-valid interval. Accepting an
	// inferred source here would let a backfill default masquerade as a
	// deliberate correction.
	ErrTemporalValiditySourceNotExplicit = errors.New("temporal validity source must be explicit for this write")
	// ErrTemporalIdentityChangeRejected rejects a write that re-points an existing
	// fact at a new temporal identity. The correction ledger accounts for one
	// contiguous chain per identity, so rebinding would orphan every version
	// recorded against the previous identity.
	ErrTemporalIdentityChangeRejected = errors.New("temporal fact identity cannot change for an existing fact")
)

// ValidateTemporalForClass enforces the fact-validity rules that a write must
// satisfy before it reaches storage.
//
// A factual class (profile, episodic, procedural) must carry a well-formed
// validity snapshot: an identity, a recorded time and a valid-from. A snapshot
// with only some fields populated is malformed rather than legacy and fails
// closed, because the migration's legacy-compatible shape always fills all of
// them from the recorded time.
//
// requireExplicitSource is set by callers that are deliberately asserting a
// fact-valid interval: an inferred or backfilled source is then not sufficient,
// so a default cannot be laundered into a correction.
//
// Non-factual classes have no fact-valid time of their own and are left alone.
func ValidateTemporalForClass(class MemoryClass, validity TemporalValidity, requireExplicitSource bool) error {
	if !IsTemporalFactClass(class) {
		return nil
	}
	if validity.IsUnset() {
		return ErrTemporalIdentityRequiredForClass
	}
	if err := validity.Validate(); err != nil {
		return err
	}
	if requireExplicitSource && validity.ValiditySource != TemporalValiditySourceExplicit {
		return ErrTemporalValiditySourceNotExplicit
	}
	return nil
}

// ValidateTemporalTransition enforces the rules that only apply when an existing
// fact is being changed, on top of the snapshot rules above.
//
// The identity must be preserved: rebinding a fact to a new identity forks its
// lineage, and the correction ledger — which is contiguous per identity — could
// no longer account for the versions recorded against the old one.
//
// A moved valid_from is allowed and is the ordinary supersede path: a mutable
// fact changes over time within one identity. A retroactive correction into a
// past window is likewise allowed, since that is exactly what valid time is for.
func ValidateTemporalTransition(class MemoryClass, existing, proposed TemporalValidity, requireExplicitSource bool) error {
	if !IsTemporalFactClass(class) {
		return nil
	}
	if err := ValidateTemporalForClass(class, proposed, requireExplicitSource); err != nil {
		return err
	}
	if !existing.IsUnset() && existing.TemporalFactID != proposed.TemporalFactID {
		return ErrTemporalIdentityChangeRejected
	}
	return nil
}
