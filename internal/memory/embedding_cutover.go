package memory

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrEmbeddingCutoverConflict = errors.New("embedding cutover conflict")
var ErrEmbeddingCutoverRejected = errors.New("embedding cutover rejected")

type EmbeddingCutoverPlanStatus string

const (
	EmbeddingCutoverPlanStatusDraft     EmbeddingCutoverPlanStatus = "draft"
	EmbeddingCutoverPlanStatusActive    EmbeddingCutoverPlanStatus = "active"
	EmbeddingCutoverPlanStatusPaused    EmbeddingCutoverPlanStatus = "paused"
	EmbeddingCutoverPlanStatusCancelled EmbeddingCutoverPlanStatus = "cancelled"
	EmbeddingCutoverPlanStatusCompleted EmbeddingCutoverPlanStatus = "completed"
)

func (s EmbeddingCutoverPlanStatus) Valid() bool {
	switch s {
	case EmbeddingCutoverPlanStatusDraft,
		EmbeddingCutoverPlanStatusActive,
		EmbeddingCutoverPlanStatusPaused,
		EmbeddingCutoverPlanStatusCancelled,
		EmbeddingCutoverPlanStatusCompleted:
		return true
	default:
		return false
	}
}

type EmbeddingCutoverPlanAction string

const (
	EmbeddingCutoverPlanActionActivate EmbeddingCutoverPlanAction = "activate"
	EmbeddingCutoverPlanActionPause    EmbeddingCutoverPlanAction = "pause"
	EmbeddingCutoverPlanActionCancel   EmbeddingCutoverPlanAction = "cancel"
)

func (a EmbeddingCutoverPlanAction) Valid() bool {
	switch a {
	case EmbeddingCutoverPlanActionActivate,
		EmbeddingCutoverPlanActionPause,
		EmbeddingCutoverPlanActionCancel:
		return true
	default:
		return false
	}
}

type EmbeddingCutoverItemStatus string

const (
	EmbeddingCutoverItemStatusQueued     EmbeddingCutoverItemStatus = "queued"
	EmbeddingCutoverItemStatusRebuilding EmbeddingCutoverItemStatus = "rebuilding"
	EmbeddingCutoverItemStatusCurrent    EmbeddingCutoverItemStatus = "current"
	EmbeddingCutoverItemStatusFailed     EmbeddingCutoverItemStatus = "failed"
	EmbeddingCutoverItemStatusSkipped    EmbeddingCutoverItemStatus = "skipped"
	EmbeddingCutoverItemStatusPaused     EmbeddingCutoverItemStatus = "paused"
	EmbeddingCutoverItemStatusCancelled  EmbeddingCutoverItemStatus = "cancelled"
)

func (s EmbeddingCutoverItemStatus) Valid() bool {
	switch s {
	case EmbeddingCutoverItemStatusQueued,
		EmbeddingCutoverItemStatusRebuilding,
		EmbeddingCutoverItemStatusCurrent,
		EmbeddingCutoverItemStatusFailed,
		EmbeddingCutoverItemStatusSkipped,
		EmbeddingCutoverItemStatusPaused,
		EmbeddingCutoverItemStatusCancelled:
		return true
	default:
		return false
	}
}

type EmbeddingCutoverTarget struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
}

func (t EmbeddingCutoverTarget) Validate() error {
	switch {
	case strings.TrimSpace(t.Provider) == "":
		return fmt.Errorf("provider is required")
	case strings.TrimSpace(t.Model) == "":
		return fmt.Errorf("model is required")
	case t.Dimensions <= 0:
		return fmt.Errorf("dimensions must be greater than zero")
	default:
		return nil
	}
}

type EmbeddingCutoverProgress struct {
	Total      int `json:"total"`
	Queued     int `json:"queued"`
	Rebuilding int `json:"rebuilding"`
	Current    int `json:"current"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`
	Paused     int `json:"paused"`
	Cancelled  int `json:"cancelled"`
}

type EmbeddingCutoverPlan struct {
	ID               string                     `json:"id"`
	Scope            Scope                      `json:"scope"`
	Target           EmbeddingCutoverTarget     `json:"target"`
	Classes          []MemoryClass              `json:"classes,omitempty"`
	WaveSize         int                        `json:"wave_size"`
	Status           EmbeddingCutoverPlanStatus `json:"status"`
	Reason           string                     `json:"reason"`
	CreatedBy        string                     `json:"created_by"`
	CreatedAt        time.Time                  `json:"created_at"`
	LastActionBy     string                     `json:"last_action_by,omitempty"`
	LastActionReason string                     `json:"last_action_reason,omitempty"`
	LastActionAt     time.Time                  `json:"last_action_at,omitempty"`
	ActivatedAt      time.Time                  `json:"activated_at,omitempty"`
	PausedAt         time.Time                  `json:"paused_at,omitempty"`
	CancelledAt      time.Time                  `json:"cancelled_at,omitempty"`
	CompletedAt      time.Time                  `json:"completed_at,omitempty"`
	Progress         EmbeddingCutoverProgress   `json:"progress"`
	Items            []EmbeddingCutoverItem     `json:"items,omitempty"`
}

type EmbeddingCutoverItem struct {
	PlanID               string                     `json:"plan_id"`
	MemoryID             string                     `json:"memory_id"`
	Scope                Scope                      `json:"scope"`
	Class                MemoryClass                `json:"class"`
	Status               EmbeddingCutoverItemStatus `json:"status"`
	FailureReason        string                     `json:"failure_reason,omitempty"`
	ActiveVectorRevision string                     `json:"active_vector_revision_id,omitempty"`
	ActiveProvider       string                     `json:"active_provider,omitempty"`
	ActiveModel          string                     `json:"active_model,omitempty"`
	ActiveDimensions     int                        `json:"active_dimensions,omitempty"`
	RequestedAt          time.Time                  `json:"requested_at,omitempty"`
	LastAttemptedAt      time.Time                  `json:"last_attempted_at,omitempty"`
	UpdatedAt            time.Time                  `json:"updated_at,omitempty"`
}

type CreateEmbeddingCutoverPlanInput struct {
	Scope     Scope                  `json:"scope"`
	Target    EmbeddingCutoverTarget `json:"target"`
	Classes   []MemoryClass          `json:"classes,omitempty"`
	WaveSize  int                    `json:"wave_size"`
	Actor     string                 `json:"actor"`
	Reason    string                 `json:"reason"`
	CreatedAt time.Time              `json:"created_at"`
}

func (i CreateEmbeddingCutoverPlanInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case i.Target.Validate() != nil:
		return i.Target.Validate()
	case validateEmbeddingCutoverClasses(i.Classes) != nil:
		return validateEmbeddingCutoverClasses(i.Classes)
	case i.WaveSize <= 0:
		return fmt.Errorf("wave size must be greater than zero")
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	case i.CreatedAt.IsZero():
		return fmt.Errorf("created at is required")
	default:
		return nil
	}
}

type ListEmbeddingCutoverPlansInput struct {
	Scope  Scope                      `json:"scope"`
	Status EmbeddingCutoverPlanStatus `json:"status,omitempty"`
	Limit  int                        `json:"limit"`
}

func (i ListEmbeddingCutoverPlansInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case i.Status != "" && !i.Status.Valid():
		return fmt.Errorf("embedding cutover plan status %q is invalid", i.Status)
	case i.Limit <= 0:
		return fmt.Errorf("limit must be greater than zero")
	default:
		return nil
	}
}

type ReadEmbeddingCutoverPlanInput struct {
	Scope  Scope  `json:"scope"`
	PlanID string `json:"plan_id"`
}

func (i ReadEmbeddingCutoverPlanInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case strings.TrimSpace(i.PlanID) == "":
		return fmt.Errorf("cutover plan id is required")
	default:
		return nil
	}
}

type ApplyEmbeddingCutoverPlanActionInput struct {
	Scope     Scope                      `json:"scope"`
	PlanID    string                     `json:"plan_id"`
	Action    EmbeddingCutoverPlanAction `json:"action"`
	Actor     string                     `json:"actor"`
	Reason    string                     `json:"reason"`
	AppliedAt time.Time                  `json:"applied_at"`
}

func (i ApplyEmbeddingCutoverPlanActionInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case strings.TrimSpace(i.PlanID) == "":
		return fmt.Errorf("cutover plan id is required")
	case !i.Action.Valid():
		return fmt.Errorf("embedding cutover plan action %q is invalid", i.Action)
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	case i.AppliedAt.IsZero():
		return fmt.Errorf("applied at is required")
	default:
		return nil
	}
}

type ListEmbeddingRecoveryHistoryInput struct {
	Scope         Scope                   `json:"scope"`
	MemoryID      string                  `json:"memory_id,omitempty"`
	Action        EmbeddingRecoveryAction `json:"action,omitempty"`
	Actor         string                  `json:"actor,omitempty"`
	CutoverPlanID string                  `json:"cutover_plan_id,omitempty"`
	OccurredFrom  time.Time               `json:"occurred_from,omitempty"`
	OccurredTo    time.Time               `json:"occurred_to,omitempty"`
	Limit         int                     `json:"limit"`
}

func (i ListEmbeddingRecoveryHistoryInput) Validate() error {
	switch {
	case i.Scope.Validate() != nil:
		return i.Scope.Validate()
	case i.Action != "" && !i.Action.Valid():
		return fmt.Errorf("embedding recovery action %q is invalid", i.Action)
	case !i.OccurredFrom.IsZero() && !i.OccurredTo.IsZero() && i.OccurredFrom.After(i.OccurredTo):
		return fmt.Errorf("occurred_from must be before or equal to occurred_to")
	case i.Limit <= 0:
		return fmt.Errorf("limit must be greater than zero")
	default:
		return nil
	}
}

func ApplyEmbeddingCutoverPlanAction(current EmbeddingCutoverPlan, input ApplyEmbeddingCutoverPlanActionInput) (EmbeddingCutoverPlan, error) {
	if err := input.Validate(); err != nil {
		return EmbeddingCutoverPlan{}, err
	}
	if current.Status != "" && !current.Status.Valid() {
		return EmbeddingCutoverPlan{}, fmt.Errorf("embedding cutover plan status %q is invalid", current.Status)
	}

	next := current
	switch input.Action {
	case EmbeddingCutoverPlanActionActivate:
		if current.Status != EmbeddingCutoverPlanStatusDraft && current.Status != EmbeddingCutoverPlanStatusPaused {
			return EmbeddingCutoverPlan{}, fmt.Errorf("%w: action %q is not allowed for status %q", ErrEmbeddingCutoverConflict, input.Action, current.Status)
		}
		next.Status = EmbeddingCutoverPlanStatusActive
		next.ActivatedAt = input.AppliedAt.UTC()
	case EmbeddingCutoverPlanActionPause:
		if current.Status != EmbeddingCutoverPlanStatusActive {
			return EmbeddingCutoverPlan{}, fmt.Errorf("%w: action %q is not allowed for status %q", ErrEmbeddingCutoverConflict, input.Action, current.Status)
		}
		next.Status = EmbeddingCutoverPlanStatusPaused
		next.PausedAt = input.AppliedAt.UTC()
	case EmbeddingCutoverPlanActionCancel:
		if current.Status != EmbeddingCutoverPlanStatusDraft &&
			current.Status != EmbeddingCutoverPlanStatusActive &&
			current.Status != EmbeddingCutoverPlanStatusPaused {
			return EmbeddingCutoverPlan{}, fmt.Errorf("%w: action %q is not allowed for status %q", ErrEmbeddingCutoverConflict, input.Action, current.Status)
		}
		next.Status = EmbeddingCutoverPlanStatusCancelled
		next.CancelledAt = input.AppliedAt.UTC()
	default:
		return EmbeddingCutoverPlan{}, fmt.Errorf("embedding cutover plan action %q is invalid", input.Action)
	}

	next.LastActionBy = strings.TrimSpace(input.Actor)
	next.LastActionReason = strings.TrimSpace(input.Reason)
	next.LastActionAt = input.AppliedAt.UTC()
	return next, nil
}

func validateEmbeddingCutoverClasses(classes []MemoryClass) error {
	for _, class := range classes {
		if !validEmbeddingCutoverClass(class) {
			return fmt.Errorf("memory class %q is invalid", class)
		}
	}

	return nil
}

func validEmbeddingCutoverClass(class MemoryClass) bool {
	switch class {
	case MemoryClassProfile,
		MemoryClassEpisodic,
		MemoryClassProcedural,
		MemoryClassSummary,
		MemoryClassRelation:
		return true
	default:
		return false
	}
}
