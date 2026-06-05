package governance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

type Sensitivity string

const (
	SensitivityLow        Sensitivity = "low"
	SensitivityMedium     Sensitivity = "medium"
	SensitivityHigh       Sensitivity = "high"
	SensitivityRestricted Sensitivity = "restricted"
)

func (s Sensitivity) Validate() error {
	switch Sensitivity(strings.TrimSpace(string(s))) {
	case SensitivityLow, SensitivityMedium, SensitivityHigh, SensitivityRestricted:
		return nil
	default:
		return fmt.Errorf("invalid sensitivity %q", s)
	}
}

type Mutability string

const (
	MutabilityMutable   Mutability = "mutable"
	MutabilityImmutable Mutability = "immutable"
)

func (m Mutability) Validate() error {
	switch Mutability(strings.TrimSpace(string(m))) {
	case MutabilityMutable, MutabilityImmutable:
		return nil
	default:
		return fmt.Errorf("invalid mutability %q", m)
	}
}

type CandidateStatus string

const (
	CandidateStatusPending    CandidateStatus = "pending"
	CandidateStatusPromoted   CandidateStatus = "promoted"
	CandidateStatusSuppressed CandidateStatus = "suppressed"
	CandidateStatusExpired    CandidateStatus = "expired"
	CandidateStatusDeleted    CandidateStatus = "deleted"
)

func (s CandidateStatus) Validate() error {
	switch CandidateStatus(strings.TrimSpace(string(s))) {
	case CandidateStatusPending, CandidateStatusPromoted, CandidateStatusSuppressed, CandidateStatusExpired, CandidateStatusDeleted:
		return nil
	default:
		return fmt.Errorf("invalid candidate status %q", s)
	}
}

type CandidateMemory struct {
	ID               string
	SourceRawEventID string
	Scope            memory.Scope
	Class            memory.MemoryClass
	Content          string
	Confidence       float64
	Importance       float64
	Freshness        float64
	Sensitivity      Sensitivity
	Mutability       Mutability
	RetentionClass   policy.RetentionClass
	Status           CandidateStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (c CandidateMemory) Validate() error {
	switch {
	case strings.TrimSpace(c.ID) == "":
		return fmt.Errorf("candidate id is required")
	case strings.TrimSpace(c.SourceRawEventID) == "":
		return fmt.Errorf("source raw event id is required")
	case c.Scope.Validate() != nil:
		return c.Scope.Validate()
	case validateCandidateClass(c.Class) != nil:
		return validateCandidateClass(c.Class)
	case strings.TrimSpace(c.Content) == "":
		return fmt.Errorf("candidate content is required")
	case validateScore("confidence", c.Confidence) != nil:
		return validateScore("confidence", c.Confidence)
	case validateScore("importance", c.Importance) != nil:
		return validateScore("importance", c.Importance)
	case validateScore("freshness", c.Freshness) != nil:
		return validateScore("freshness", c.Freshness)
	case c.Sensitivity.Validate() != nil:
		return c.Sensitivity.Validate()
	case c.Mutability.Validate() != nil:
		return c.Mutability.Validate()
	case c.RetentionClass.Validate() != nil:
		return c.RetentionClass.Validate()
	case c.Status.Validate() != nil:
		return c.Status.Validate()
	case c.CreatedAt.IsZero():
		return fmt.Errorf("candidate created at is required")
	case c.UpdatedAt.IsZero():
		return fmt.Errorf("candidate updated at is required")
	case c.UpdatedAt.Before(c.CreatedAt):
		return fmt.Errorf("candidate updated at must not be before created at")
	default:
		return nil
	}
}

type CandidateStatusTransition struct {
	CandidateID string
	ToStatus    CandidateStatus
	UpdatedAt   time.Time
}

func (t CandidateStatusTransition) Validate() error {
	switch {
	case strings.TrimSpace(t.CandidateID) == "":
		return fmt.Errorf("candidate id is required")
	case t.ToStatus.Validate() != nil:
		return t.ToStatus.Validate()
	case t.UpdatedAt.IsZero():
		return fmt.Errorf("candidate updated at is required")
	default:
		return nil
	}
}

type CandidateRepository interface {
	CreateCandidate(ctx context.Context, candidate CandidateMemory, provenance memory.ProvenanceRecord) (CandidateMemory, error)
	ListCandidatesByRawEvent(ctx context.Context, rawEventID string) ([]CandidateMemory, error)
	TransitionCandidateStatus(ctx context.Context, transition CandidateStatusTransition, provenance memory.ProvenanceRecord) (CandidateMemory, error)
}

func validateCandidateClass(class memory.MemoryClass) error {
	switch class {
	case memory.MemoryClassProfile,
		memory.MemoryClassEpisodic,
		memory.MemoryClassProcedural,
		memory.MemoryClassSummary,
		memory.MemoryClassRelation:
		return nil
	default:
		return fmt.Errorf("invalid candidate class %q", class)
	}
}

func validateScore(name string, value float64) error {
	if value < 0 || value > 1 {
		return fmt.Errorf("%s must be between 0 and 1", name)
	}

	return nil
}
