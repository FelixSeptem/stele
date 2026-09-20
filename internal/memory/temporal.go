package memory

import (
	"errors"
	"fmt"
	"time"
)

type TemporalValiditySource string

const (
	TemporalValiditySourceExplicit         TemporalValiditySource = "explicit"
	TemporalValiditySourceLegacyCompatible TemporalValiditySource = "legacy_current_compatible"
	TemporalValiditySourceMigrated         TemporalValiditySource = "migrated"
)

var (
	ErrTemporalFactIdentityRequired  = errors.New("temporal fact identity is required")
	ErrTemporalIngestedAtRequired    = errors.New("temporal ingested_at is required")
	ErrTemporalValidFromRequired     = errors.New("temporal valid_from is required")
	ErrTemporalIntervalInvalid       = errors.New("temporal validity interval is invalid")
	ErrTemporalAsOfRequired          = errors.New("temporal as_of is required")
	ErrTemporalConstraintAmbiguous   = errors.New("temporal constraint selectors are ambiguous")
	ErrTemporalValiditySourceInvalid = errors.New("temporal validity source is invalid")
)

// TemporalValidity is the immutable fact-validity snapshot attached to a
// canonical version. ValidTo is exclusive; nil means the interval is open.
type TemporalValidity struct {
	TemporalFactID string                 `json:"temporal_fact_id"`
	IngestedAt     time.Time              `json:"ingested_at"`
	ValidFrom      time.Time              `json:"valid_from"`
	ValidTo        *time.Time             `json:"valid_to,omitempty"`
	ValiditySource TemporalValiditySource `json:"validity_source"`
}

func (v TemporalValidity) Validate() error {
	switch {
	case v.TemporalFactID == "":
		return ErrTemporalFactIdentityRequired
	case v.IngestedAt.IsZero():
		return ErrTemporalIngestedAtRequired
	case v.ValidFrom.IsZero():
		return ErrTemporalValidFromRequired
	case v.ValidTo != nil && !v.ValidTo.After(v.ValidFrom):
		return ErrTemporalIntervalInvalid
	case v.ValiditySource != "" && !validTemporalValiditySource(v.ValiditySource):
		return ErrTemporalValiditySourceInvalid
	default:
		return nil
	}
}

// IsUnset reports whether no validity snapshot has been attached at all. It is
// the in-memory equivalent of a pre-migration row, and such a version must stay
// retrievable as current-valid rather than vanishing from ordinary retrieval.
func (v TemporalValidity) IsUnset() bool {
	return v.TemporalFactID == "" && v.IngestedAt.IsZero() && v.ValidFrom.IsZero() && v.ValidTo == nil && v.ValiditySource == ""
}

// LegacyCurrentCompatible rewrites an unset snapshot into the same shape the
// migration backfills for existing rows: an open-ended interval that starts at
// the recorded creation time. It is used only when the row has no temporal
// identity, so an explicitly malformed interval is never silently repaired.
func (v TemporalValidity) LegacyCurrentCompatible(fallbackReference time.Time) TemporalValidity {
	if !v.IsUnset() {
		return v
	}
	reference := v.IngestedAt
	if reference.IsZero() {
		reference = fallbackReference
	}
	if reference.IsZero() {
		return v
	}
	return TemporalValidity{
		TemporalFactID: string(TemporalValiditySourceLegacyCompatible),
		IngestedAt:     reference,
		ValidFrom:      reference,
		ValiditySource: TemporalValiditySourceLegacyCompatible,
	}
}

func (v TemporalValidity) Contains(at time.Time) bool {
	if v.Validate() != nil {
		return false
	}
	if at.Before(v.ValidFrom) {
		return false
	}
	return v.ValidTo == nil || at.Before(*v.ValidTo)
}

func (v TemporalValidity) Overlaps(from time.Time, to *time.Time) bool {
	if v.Validate() != nil || from.IsZero() || (to != nil && !to.After(from)) {
		return false
	}
	if v.ValidTo != nil && !from.Before(*v.ValidTo) {
		return false
	}
	return to == nil || v.ValidFrom.Before(*to)
}

type TemporalSelectionMode string

const (
	TemporalSelectionCurrent TemporalSelectionMode = "current"
	TemporalSelectionAsOf    TemporalSelectionMode = "as_of"
	TemporalSelectionDuring  TemporalSelectionMode = "valid_during"
)

// TemporalConstraint is the validated selector carried by a retrieval plan.
// Current is evaluated at the request clock supplied to Matches.
type TemporalConstraint struct {
	Mode      TemporalSelectionMode `json:"mode"`
	AsOf      *time.Time            `json:"as_of,omitempty"`
	ValidFrom *time.Time            `json:"valid_from,omitempty"`
	ValidTo   *time.Time            `json:"valid_to,omitempty"`
}

func (c TemporalConstraint) Validate() error {
	if c.Mode == "" {
		return fmt.Errorf("temporal mode is required")
	}
	switch c.Mode {
	case TemporalSelectionCurrent:
		if c.AsOf != nil || c.ValidFrom != nil || c.ValidTo != nil {
			return ErrTemporalConstraintAmbiguous
		}
	case TemporalSelectionAsOf:
		if c.AsOf == nil {
			return ErrTemporalAsOfRequired
		}
		if c.ValidFrom != nil || c.ValidTo != nil {
			return ErrTemporalConstraintAmbiguous
		}
	case TemporalSelectionDuring:
		if c.ValidFrom == nil || c.ValidTo == nil {
			return ErrTemporalIntervalInvalid
		}
		if !c.ValidTo.After(*c.ValidFrom) {
			return ErrTemporalIntervalInvalid
		}
		if c.AsOf != nil {
			return ErrTemporalConstraintAmbiguous
		}
	default:
		return fmt.Errorf("unknown temporal mode %q", c.Mode)
	}
	return nil
}

// Matches reports whether a version is selectable under this constraint at the
// supplied evaluation instant. A version with no validity snapshot is treated as
// legacy current-compatible so pre-migration rows remain retrievable; that
// fallback needs a recorded-time reference and therefore fails closed when the
// caller supplies none.
func (c TemporalConstraint) Matches(v TemporalValidity, evaluationTime time.Time) bool {
	if c.Validate() != nil {
		return false
	}
	if v.IsUnset() {
		v = v.LegacyCurrentCompatible(evaluationTime)
	}
	if v.Validate() != nil {
		return false
	}
	switch c.Mode {
	case TemporalSelectionCurrent:
		return v.Contains(evaluationTime)
	case TemporalSelectionAsOf:
		return v.Contains(*c.AsOf)
	case TemporalSelectionDuring:
		return v.Overlaps(*c.ValidFrom, c.ValidTo)
	default:
		return false
	}
}

func IsTemporalFactClass(class MemoryClass) bool {
	switch class {
	case MemoryClassProfile, MemoryClassEpisodic, MemoryClassProcedural:
		return true
	default:
		return false
	}
}

func validTemporalValiditySource(source TemporalValiditySource) bool {
	switch source {
	case TemporalValiditySourceExplicit, TemporalValiditySourceLegacyCompatible, TemporalValiditySourceMigrated:
		return true
	default:
		return false
	}
}

// TemporalConflictDisposition records how the service resolved a competing
// validity interval for one temporal fact identity. Ordinary current retrieval
// excludes any fact whose latest disposition is not none or resolved, so an
// unreviewed conflict can never be served as if it were authoritative.
type TemporalConflictDisposition string

const (
	// TemporalConflictNone means the submitted interval did not compete with an
	// existing current interval.
	TemporalConflictNone TemporalConflictDisposition = "none"
	// TemporalConflictRejected means the submission was refused outright and no
	// successor was created.
	TemporalConflictRejected TemporalConflictDisposition = "rejected"
	// TemporalConflictOpen means an overlap was recorded for audit and the fact
	// is withheld from ordinary current retrieval until it is resolved.
	TemporalConflictOpen TemporalConflictDisposition = "conflict_open"
	// TemporalConflictResolved means an authorized correction explicitly closed
	// the competing intervals and named this successor as the current one.
	TemporalConflictResolved TemporalConflictDisposition = "resolved"
)

func (disposition TemporalConflictDisposition) Valid() bool {
	switch disposition {
	case TemporalConflictNone, TemporalConflictRejected, TemporalConflictOpen, TemporalConflictResolved:
		return true
	default:
		return false
	}
}

// SuppressesCurrentRetrieval reports whether a fact carrying this disposition
// must be withheld from ordinary current-valid results.
func (disposition TemporalConflictDisposition) SuppressesCurrentRetrieval() bool {
	return disposition == TemporalConflictOpen
}

var (
	// ErrTemporalOverlapUnresolved is returned when a proposed interval would
	// overlap an existing current interval for the same fact identity without an
	// explicit conflict disposition. The caller must either reject the write or
	// record TemporalConflictOpen; silently accepting it would create two
	// simultaneously-valid versions of a mutually exclusive fact.
	ErrTemporalOverlapUnresolved = errors.New("temporal interval overlaps an existing current interval without an explicit conflict disposition")
	// ErrTemporalDispositionInvalid rejects an unknown disposition label so it
	// cannot be persisted as an untyped string.
	ErrTemporalDispositionInvalid = errors.New("temporal conflict disposition is invalid")
)

// TemporalIntervalOverlap reports whether two half-open validity intervals
// overlap. A nil end is open-ended. Malformed inputs never overlap: an interval
// that cannot be validated is not evidence of a conflict.
func TemporalIntervalOverlap(left, right TemporalValidity) bool {
	if left.Validate() != nil || right.Validate() != nil {
		return false
	}
	// Half-open [from, to): they overlap unless one ends at or before the other
	// begins. A nil end means open-ended, so it never ends first.
	leftEndsFirst := left.ValidTo != nil && !left.ValidTo.After(right.ValidFrom)
	rightEndsFirst := right.ValidTo != nil && !right.ValidTo.After(left.ValidFrom)
	return !leftEndsFirst && !rightEndsFirst
}

// TemporalOverlapConflict decides whether a proposed validity interval may
// replace the current one for a fact identity.
//
// It returns the disposition the caller must persist. An overlap without an
// explicit authorization is reported as ErrTemporalOverlapUnresolved so the
// write path fails closed rather than creating an ambiguous current interval.
func TemporalOverlapConflict(existing, proposed TemporalValidity, authorized bool) (TemporalConflictDisposition, error) {
	if err := existing.Validate(); err != nil {
		return TemporalConflictRejected, err
	}
	if err := proposed.Validate(); err != nil {
		return TemporalConflictRejected, err
	}
	// A different fact identity is never a conflict; the caller is writing an
	// unrelated fact.
	if existing.TemporalFactID != proposed.TemporalFactID {
		return TemporalConflictNone, nil
	}
	if !TemporalIntervalOverlap(existing, proposed) {
		return TemporalConflictNone, nil
	}
	if !authorized {
		return TemporalConflictOpen, ErrTemporalOverlapUnresolved
	}
	return TemporalConflictResolved, nil
}
