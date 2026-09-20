package memory

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrTemporalCorrectionIdentityRequired rejects a correction that does not
	// name the temporal fact it corrects; without it the lineage cannot be
	// reconstructed deterministically.
	ErrTemporalCorrectionIdentityRequired = errors.New("temporal correction requires a fact identity")
	// ErrTemporalCorrectionProvenanceRequired rejects a correction without an
	// actor and reason, since an unattributed validity change is not auditable.
	ErrTemporalCorrectionProvenanceRequired = errors.New("temporal correction requires actor and reason provenance")
	// ErrTemporalCorrectionSuccessorRequired rejects a correction that does not
	// advance the version, which would be an in-place mutation rather than an
	// append.
	ErrTemporalCorrectionSuccessorRequired = errors.New("temporal correction requires a successor version")
	// ErrTemporalCorrectionLineageInvalid rejects a correction whose successor
	// does not immediately follow its predecessor.
	ErrTemporalCorrectionLineageInvalid = errors.New("temporal correction successor must follow its predecessor")
	// ErrTemporalCorrectionConflict rejects a replay whose recorded fields
	// disagree with the correction already stored for that successor version.
	// The ledger is append-only, so a disagreeing replay is refused rather than
	// allowed to overwrite the durable record.
	ErrTemporalCorrectionConflict = errors.New("temporal correction already recorded with different provenance")
	// ErrTemporalCorrectionGap rejects a stored lineage that skips a version,
	// which would mean a version exists that no correction accounts for.
	ErrTemporalCorrectionGap = errors.New("stored temporal correction lineage is not contiguous")
)

// TemporalCorrection is one append-only record of a validity change for a
// temporal fact. It never carries mutated memory payload: the correction names
// the successor version that a reader must load separately. Prior versions are
// never rewritten, so the full lineage stays auditable.
type TemporalCorrection struct {
	Scope              Scope
	TemporalFactID     string
	MemoryID           string
	PredecessorVersion *int64
	SuccessorVersion   int64
	Actor              string
	Reason             string
	Disposition        TemporalConflictDisposition
	CreatedAt          time.Time
}

func (c TemporalCorrection) Validate() error {
	switch {
	case strings.TrimSpace(c.TemporalFactID) == "":
		return ErrTemporalCorrectionIdentityRequired
	case strings.TrimSpace(c.MemoryID) == "":
		return ErrTemporalCorrectionIdentityRequired
	case c.Scope.Validate() != nil:
		return c.Scope.Validate()
	case strings.TrimSpace(c.Actor) == "" || strings.TrimSpace(c.Reason) == "":
		return ErrTemporalCorrectionProvenanceRequired
	case c.SuccessorVersion <= 0:
		return ErrTemporalCorrectionSuccessorRequired
	case c.PredecessorVersion != nil && *c.PredecessorVersion <= 0:
		return ErrTemporalCorrectionLineageInvalid
	case c.PredecessorVersion != nil && c.SuccessorVersion <= *c.PredecessorVersion:
		return ErrTemporalCorrectionLineageInvalid
	case !c.Disposition.Valid():
		return ErrTemporalDispositionInvalid
	default:
		return nil
	}
}

// SuppressesCurrentRetrieval reports whether the correction leaves the fact in a
// state that must be withheld from ordinary current results.
func (c TemporalCorrection) SuppressesCurrentRetrieval() bool {
	return c.Disposition.SuppressesCurrentRetrieval()
}

// TemporalLineage is the deterministic, ordered view of a fact's correction
// history: oldest first, with stable predecessor/successor links.
type TemporalLineage struct {
	TemporalFactID string
	Corrections    []TemporalCorrection
}

// Validate rejects a lineage that is not a single deterministic chain: a
// starting correction with no predecessor, strictly increasing successor
// versions that advance by exactly one, and every link naming the predecessor it
// follows. Requiring a contiguous chain is what makes "read the history in
// order" deterministic: a gap would mean a version exists that no correction
// accounts for.
func (lineage TemporalLineage) Validate() error {
	if strings.TrimSpace(lineage.TemporalFactID) == "" {
		return ErrTemporalCorrectionIdentityRequired
	}
	var previous *int64
	for i, correction := range lineage.Corrections {
		if err := correction.Validate(); err != nil {
			return fmt.Errorf("correction %d: %w", i, err)
		}
		if correction.TemporalFactID != lineage.TemporalFactID {
			return fmt.Errorf("correction %d references a foreign fact identity", i)
		}
		if i == 0 {
			if correction.PredecessorVersion != nil {
				return fmt.Errorf("lineage must start from a correction with no predecessor")
			}
		} else {
			if correction.PredecessorVersion == nil || previous == nil || *correction.PredecessorVersion != *previous {
				return fmt.Errorf("correction %d does not follow its predecessor", i)
			}
			if correction.SuccessorVersion != *previous+1 {
				return fmt.Errorf("correction %d skips a version: %d follows %d", i, correction.SuccessorVersion, *previous)
			}
		}
		successor := correction.SuccessorVersion
		previous = &successor
	}
	return nil
}

// OrderTemporalCorrections returns corrections in append order: by successor
// version ascending, then by creation time. It never mutates the input slice.
func OrderTemporalCorrections(corrections []TemporalCorrection) []TemporalCorrection {
	ordered := make([]TemporalCorrection, len(corrections))
	copy(ordered, corrections)
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0; j-- {
			left, right := ordered[j-1], ordered[j]
			if left.SuccessorVersion < right.SuccessorVersion ||
				(left.SuccessorVersion == right.SuccessorVersion && !right.CreatedAt.Before(left.CreatedAt)) {
				break
			}
			ordered[j-1], ordered[j] = ordered[j], ordered[j-1]
		}
	}
	return ordered
}
