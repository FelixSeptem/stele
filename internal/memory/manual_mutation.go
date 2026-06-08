package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrManualMutationVersionConflict = errors.New("manual memory version conflict")
var ErrManualMutationRejected = errors.New("manual memory mutation rejected")

type ManualCreateMemoryInput struct {
	Scope     Scope
	Class     MemoryClass
	Content   string
	Reason    string
	Actor     string
	RequestID string
}

func (i ManualCreateMemoryInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case !isManualCreateClassAllowed(i.Class):
		return fmt.Errorf("manual create class %q is not allowed", i.Class)
	case strings.TrimSpace(i.Content) == "":
		return fmt.Errorf("content is required")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	default:
		return nil
	}
}

type ManualUpdateMemoryInput struct {
	Scope           Scope
	MemoryID        string
	Content         string
	ExpectedVersion int64
	Reason          string
	Actor           string
	RequestID       string
}

func (i ManualUpdateMemoryInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case strings.TrimSpace(i.MemoryID) == "":
		return fmt.Errorf("memory id is required")
	case strings.TrimSpace(i.Content) == "":
		return fmt.Errorf("content is required")
	case i.ExpectedVersion <= 0:
		return fmt.Errorf("expected version must be greater than zero")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	default:
		return nil
	}
}

type ManualMergeMemoryInput struct {
	Scope           Scope
	TargetMemoryID  string
	SourceMemoryID  string
	Content         string
	ExpectedVersion int64
	Reason          string
	Actor           string
	RequestID       string
}

func (i ManualMergeMemoryInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case strings.TrimSpace(i.TargetMemoryID) == "":
		return fmt.Errorf("target memory id is required")
	case strings.TrimSpace(i.SourceMemoryID) == "":
		return fmt.Errorf("source memory id is required")
	case strings.TrimSpace(i.TargetMemoryID) == strings.TrimSpace(i.SourceMemoryID):
		return fmt.Errorf("source memory id must differ from target memory id")
	case strings.TrimSpace(i.Content) == "":
		return fmt.Errorf("content is required")
	case i.ExpectedVersion <= 0:
		return fmt.Errorf("expected version must be greater than zero")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	default:
		return nil
	}
}

type ManualReclassifyMemoryInput struct {
	Scope           Scope
	MemoryID        string
	TargetClass     MemoryClass
	ExpectedVersion int64
	Reason          string
	Actor           string
	RequestID       string
}

func (i ManualReclassifyMemoryInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case strings.TrimSpace(i.MemoryID) == "":
		return fmt.Errorf("memory id is required")
	case !isManualReclassifyClassAllowed(i.TargetClass):
		return fmt.Errorf("manual reclassify target class %q is not allowed", i.TargetClass)
	case i.ExpectedVersion <= 0:
		return fmt.Errorf("expected version must be greater than zero")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	default:
		return nil
	}
}

type ManualCreateMemoryRecord struct {
	MemoryID  string
	VersionID string
	Scope     Scope
	Class     MemoryClass
	Content   string
	Reason    string
	Actor     string
	RequestID string
	CreatedAt time.Time
}

type ManualUpdateMemoryRecord struct {
	MemoryID        string
	VersionID       string
	Scope           Scope
	Content         string
	ExpectedVersion int64
	Reason          string
	Actor           string
	RequestID       string
	UpdatedAt       time.Time
}

type ManualMergeMemoryRecord struct {
	TargetMemoryID  string
	SourceMemoryID  string
	VersionID       string
	Scope           Scope
	Content         string
	ExpectedVersion int64
	Reason          string
	Actor           string
	RequestID       string
	AppliedAt       time.Time
}

type ManualReclassifyMemoryRecord struct {
	MemoryID        string
	VersionID       string
	Scope           Scope
	TargetClass     MemoryClass
	ExpectedVersion int64
	Reason          string
	Actor           string
	RequestID       string
	AppliedAt       time.Time
}

type ManualMutationProcessor interface {
	CreateMemory(ctx context.Context, record ManualCreateMemoryRecord) (CanonicalMemory, error)
	UpdateMemory(ctx context.Context, record ManualUpdateMemoryRecord) (CanonicalMemory, error)
	MergeMemory(ctx context.Context, record ManualMergeMemoryRecord) (CanonicalMemory, error)
	ReclassifyMemory(ctx context.Context, record ManualReclassifyMemoryRecord) (CanonicalMemory, error)
}

type ManualMutationService struct {
	Processor    ManualMutationProcessor
	Now          func() time.Time
	NewMemoryID  func() string
	NewVersionID func() string
}

func (s ManualMutationService) CreateMemory(ctx context.Context, input ManualCreateMemoryInput) (MemoryResource, error) {
	if err := input.Validate(); err != nil {
		return MemoryResource{}, err
	}
	if s.Processor == nil {
		return MemoryResource{}, fmt.Errorf("manual mutation processor is not configured")
	}
	if s.NewMemoryID == nil {
		return MemoryResource{}, fmt.Errorf("manual memory id generator is not configured")
	}
	if s.NewVersionID == nil {
		return MemoryResource{}, fmt.Errorf("manual version id generator is not configured")
	}

	canonical, err := s.Processor.CreateMemory(ctx, ManualCreateMemoryRecord{
		MemoryID:  s.NewMemoryID(),
		VersionID: s.NewVersionID(),
		Scope:     input.Scope,
		Class:     input.Class,
		Content:   input.Content,
		Reason:    input.Reason,
		Actor:     input.Actor,
		RequestID: strings.TrimSpace(input.RequestID),
		CreatedAt: manualMutationNow(s.Now),
	})
	if err != nil {
		return MemoryResource{}, err
	}

	return NewMemoryResource(canonical), nil
}

func (s ManualMutationService) UpdateMemory(ctx context.Context, input ManualUpdateMemoryInput) (MemoryResource, error) {
	if err := input.Validate(); err != nil {
		return MemoryResource{}, err
	}
	if s.Processor == nil {
		return MemoryResource{}, fmt.Errorf("manual mutation processor is not configured")
	}
	if s.NewVersionID == nil {
		return MemoryResource{}, fmt.Errorf("manual version id generator is not configured")
	}

	canonical, err := s.Processor.UpdateMemory(ctx, ManualUpdateMemoryRecord{
		MemoryID:        input.MemoryID,
		VersionID:       s.NewVersionID(),
		Scope:           input.Scope,
		Content:         input.Content,
		ExpectedVersion: input.ExpectedVersion,
		Reason:          input.Reason,
		Actor:           input.Actor,
		RequestID:       strings.TrimSpace(input.RequestID),
		UpdatedAt:       manualMutationNow(s.Now),
	})
	if err != nil {
		return MemoryResource{}, err
	}

	return NewMemoryResource(canonical), nil
}

func (s ManualMutationService) MergeMemory(ctx context.Context, input ManualMergeMemoryInput) (MemoryResource, error) {
	if err := input.Validate(); err != nil {
		return MemoryResource{}, err
	}
	if s.Processor == nil {
		return MemoryResource{}, fmt.Errorf("manual mutation processor is not configured")
	}
	if s.NewVersionID == nil {
		return MemoryResource{}, fmt.Errorf("manual version id generator is not configured")
	}

	canonical, err := s.Processor.MergeMemory(ctx, ManualMergeMemoryRecord{
		TargetMemoryID:  input.TargetMemoryID,
		SourceMemoryID:  input.SourceMemoryID,
		VersionID:       s.NewVersionID(),
		Scope:           input.Scope,
		Content:         input.Content,
		ExpectedVersion: input.ExpectedVersion,
		Reason:          input.Reason,
		Actor:           input.Actor,
		RequestID:       strings.TrimSpace(input.RequestID),
		AppliedAt:       manualMutationNow(s.Now),
	})
	if err != nil {
		return MemoryResource{}, err
	}

	return NewMemoryResource(canonical), nil
}

func (s ManualMutationService) ReclassifyMemory(ctx context.Context, input ManualReclassifyMemoryInput) (MemoryResource, error) {
	if err := input.Validate(); err != nil {
		return MemoryResource{}, err
	}
	if s.Processor == nil {
		return MemoryResource{}, fmt.Errorf("manual mutation processor is not configured")
	}
	if s.NewVersionID == nil {
		return MemoryResource{}, fmt.Errorf("manual version id generator is not configured")
	}

	canonical, err := s.Processor.ReclassifyMemory(ctx, ManualReclassifyMemoryRecord{
		MemoryID:        input.MemoryID,
		VersionID:       s.NewVersionID(),
		Scope:           input.Scope,
		TargetClass:     input.TargetClass,
		ExpectedVersion: input.ExpectedVersion,
		Reason:          input.Reason,
		Actor:           input.Actor,
		RequestID:       strings.TrimSpace(input.RequestID),
		AppliedAt:       manualMutationNow(s.Now),
	})
	if err != nil {
		return MemoryResource{}, err
	}

	return NewMemoryResource(canonical), nil
}

func manualMutationNow(now func() time.Time) time.Time {
	if now == nil {
		now = time.Now
	}
	return now().UTC()
}

func isManualCreateClassAllowed(class MemoryClass) bool {
	switch class {
	case MemoryClassProfile, MemoryClassEpisodic, MemoryClassProcedural, MemoryClassRelation:
		return true
	default:
		return false
	}
}

func isManualReclassifyClassAllowed(class MemoryClass) bool {
	switch class {
	case MemoryClassProfile, MemoryClassEpisodic, MemoryClassProcedural:
		return true
	default:
		return false
	}
}
