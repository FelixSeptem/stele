package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type MemoryIntentType string

const (
	MemoryIntentRemember      MemoryIntentType = "remember"
	MemoryIntentUpdate        MemoryIntentType = "update"
	MemoryIntentForget        MemoryIntentType = "forget"
	MemoryIntentContradiction MemoryIntentType = "contradiction"
	MemoryIntentFeedback      MemoryIntentType = "feedback"
)

func (t MemoryIntentType) Valid() bool {
	switch t {
	case MemoryIntentRemember, MemoryIntentUpdate, MemoryIntentForget, MemoryIntentContradiction, MemoryIntentFeedback:
		return true
	default:
		return false
	}
}

type MemoryIntentStatus string

const (
	MemoryIntentStatusPending    MemoryIntentStatus = "pending"
	MemoryIntentStatusAccepted   MemoryIntentStatus = "accepted"
	MemoryIntentStatusCandidate  MemoryIntentStatus = "candidate"
	MemoryIntentStatusActive     MemoryIntentStatus = "active"
	MemoryIntentStatusSuppressed MemoryIntentStatus = "suppressed"
	MemoryIntentStatusRejected   MemoryIntentStatus = "rejected"
	MemoryIntentStatusFailed     MemoryIntentStatus = "failed"
	MemoryIntentStatusReplayed   MemoryIntentStatus = "replayed"
)

func (s MemoryIntentStatus) Valid() bool {
	switch s {
	case MemoryIntentStatusAccepted, MemoryIntentStatusCandidate, MemoryIntentStatusActive, MemoryIntentStatusSuppressed, MemoryIntentStatusRejected, MemoryIntentStatusFailed:
		return true
	case MemoryIntentStatusPending, MemoryIntentStatusReplayed:
		return true
	default:
		return false
	}
}

type MemoryIntentInput struct {
	Scope           Scope
	MemoryPath      string
	Type            MemoryIntentType
	TargetMemoryID  string
	TargetVersion   int64
	Content         string
	Actor           string
	Reason          string
	Provenance      map[string]any
	RequestID       string
	OperationID     string
	IdempotencyKey  string
	TargetInsightID string
	Evidence        []MemoryIntentEvidence
}

func (i MemoryIntentInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if _, err := NormalizeMemoryPath(i.MemoryPath); err != nil {
		return fmt.Errorf("memory path: %w", err)
	}
	switch {
	case !i.Type.Valid():
		return fmt.Errorf("memory intent type %q is invalid", i.Type)
	case strings.TrimSpace(i.Actor) == "":
		return fmt.Errorf("actor is required")
	case strings.TrimSpace(i.Reason) == "":
		return fmt.Errorf("reason is required")
	case strings.TrimSpace(i.RequestID) == "":
		return fmt.Errorf("request id is required")
	case strings.TrimSpace(i.OperationID) == "":
		return fmt.Errorf("operation id is required")
	case strings.TrimSpace(i.IdempotencyKey) == "":
		return fmt.Errorf("idempotency key is required")
	case len(i.IdempotencyKey) > 256:
		return fmt.Errorf("idempotency key must be at most 256 bytes")
	}
	if len(i.Content) > MemoryIntentMaxContentBytes {
		return fmt.Errorf("intent content exceeds %d bytes", MemoryIntentMaxContentBytes)
	}
	if i.Type == MemoryIntentRemember || i.Type == MemoryIntentUpdate || i.Type == MemoryIntentFeedback {
		if strings.TrimSpace(i.Content) == "" {
			return fmt.Errorf("content is required for %s intent", i.Type)
		}
	}
	if i.Type == MemoryIntentUpdate || i.Type == MemoryIntentForget || i.Type == MemoryIntentContradiction {
		if strings.TrimSpace(i.TargetMemoryID) == "" {
			return fmt.Errorf("target memory id is required for %s intent", i.Type)
		}
		if i.TargetVersion <= 0 {
			return fmt.Errorf("target version must be greater than zero for %s intent", i.Type)
		}
	}
	if i.Type == MemoryIntentFeedback && strings.TrimSpace(i.TargetInsightID) == "" {
		return fmt.Errorf("feedback target insight id is required")
	}
	return nil
}

type MemoryIntentRecord struct {
	ID                 string
	Scope              Scope
	MemoryPath         string
	Type               MemoryIntentType
	TargetMemoryID     string
	TargetVersion      int64
	Content            string
	Actor              string
	Reason             string
	Provenance         map[string]any
	RequestID          string
	OperationID        string
	IdempotencyKey     string
	TargetInsightID    string
	Evidence           []MemoryIntentEvidence
	OutcomeReference   string
	RequestFingerprint string
	PolicyVersion      string
	PrecedenceVersion  string
	PrecedenceStage    OperationPrecedenceStage
	PrecedenceOutcome  OperationPrecedenceOutcome
	Status             MemoryIntentStatus
	CreatedAt          time.Time
}

type MemoryIntentProcessor interface {
	AppendMemoryIntent(context.Context, MemoryIntentRecord) (MemoryIntentRecord, error)
}

// MemoryIntentEnqueuer is implemented by stores that can hand an accepted
// intent to the asynchronous candidate/governance pipeline. It is optional so
// append-only ledger persistence remains independently testable.
type MemoryIntentEnqueuer interface {
	EnqueueMemoryIntent(context.Context, MemoryIntentRecord) error
}

type MemoryIntentTransitionWriter interface {
	AppendMemoryIntentTransition(context.Context, MemoryIntentTransition) error
}

// MemoryIntentTargetValidator lets the canonical store enforce existence,
// scope, lifecycle, and expected-version checks before the ledger write is
// handed to downstream governance work.
type MemoryIntentTargetValidator interface {
	ValidateMemoryIntentTarget(context.Context, MemoryIntentInput) error
}

type MemoryIntentService struct {
	Processor MemoryIntentProcessor
	Policy    MemoryIntentPolicyGate
	Now       func() time.Time
	NewID     func() string
}

func (s MemoryIntentService) Submit(ctx context.Context, input MemoryIntentInput) (MemoryIntentRecord, error) {
	if err := input.ValidateGoverned(); err != nil {
		return MemoryIntentRecord{}, err
	}
	fingerprint, err := input.CanonicalFingerprint()
	if err != nil {
		return MemoryIntentRecord{}, err
	}
	if s.Processor == nil {
		return MemoryIntentRecord{}, fmt.Errorf("memory intent processor is not configured")
	}
	if validator, ok := s.Processor.(MemoryIntentTargetValidator); ok {
		if err := validator.ValidateMemoryIntentTarget(ctx, input); err != nil {
			return MemoryIntentRecord{}, fmt.Errorf("validate memory intent target: %w", err)
		}
	}
	policyVersion := ""
	policyEnabled := true
	if s.Policy != nil {
		decision, err := s.Policy.EvaluateMemoryIntent(ctx, input)
		if err != nil {
			return MemoryIntentRecord{}, fmt.Errorf("evaluate memory intent policy: %w", err)
		}
		policyEnabled = decision.Enabled
		policyVersion = strings.TrimSpace(decision.PolicyVersion)
	}
	precedence, err := EvaluateOperationPrecedence(OperationPrecedenceInput{
		Operation: "memory.intent.submit", Scope: input.Scope, GrantedScope: input.Scope,
		PrincipalID: strings.TrimSpace(input.Actor), GrantID: strings.TrimSpace(input.Actor),
		LifecycleChecked: true, LifecycleVisible: true, PrincipalGranted: strings.TrimSpace(input.Actor) != "",
		ApprovalRequired: s.Policy != nil, ApprovalEnabled: policyEnabled, PolicyVersion: policyVersion,
		RequireIdempotency: true, IdempotencyKey: input.IdempotencyKey, RequestFingerprint: fingerprint,
		HandoffAllowed: s.Processor != nil, MutationAllowed: true,
	})
	if err != nil {
		return MemoryIntentRecord{}, fmt.Errorf("evaluate memory intent precedence: %w", err)
	}
	if precedence.Outcome != OperationOutcomeAccepted {
		if precedence.Outcome == OperationOutcomePolicyDisabled {
			category := MemoryIntentDiagnosticPolicyDisabled
			return MemoryIntentRecord{}, fmt.Errorf("memory intent policy %s: processing disabled", category)
		}
		return MemoryIntentRecord{}, fmt.Errorf("memory intent precedence %s at %s", precedence.Outcome, precedence.Stage)
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	id := ""
	if s.NewID != nil {
		id = s.NewID()
	}
	record := MemoryIntentRecord{
		ID: id, Scope: input.Scope, MemoryPath: func() string { p, _ := NormalizeMemoryPath(input.MemoryPath); return p }(), Type: input.Type, TargetMemoryID: strings.TrimSpace(input.TargetMemoryID),
		TargetVersion: input.TargetVersion, Content: input.Content, Actor: strings.TrimSpace(input.Actor),
		Reason: input.Reason, Provenance: input.Provenance, RequestID: strings.TrimSpace(input.RequestID),
		OperationID: strings.TrimSpace(input.OperationID), IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
		TargetInsightID: strings.TrimSpace(input.TargetInsightID), Evidence: input.Evidence, RequestFingerprint: fingerprint, PolicyVersion: policyVersion,
		PrecedenceVersion: precedence.Version, PrecedenceStage: precedence.Stage, PrecedenceOutcome: precedence.Outcome,
		Status: MemoryIntentStatusAccepted, CreatedAt: now,
	}
	created, err := s.Processor.AppendMemoryIntent(ctx, record)
	if err != nil {
		return MemoryIntentRecord{}, err
	}
	acceptedOrReplay := created.Status == MemoryIntentStatusAccepted || created.Status == MemoryIntentStatusReplayed
	if writer, ok := s.Processor.(MemoryIntentTransitionWriter); ok && acceptedOrReplay {
		if err := writer.AppendMemoryIntentTransition(ctx, MemoryIntentTransition{
			IntentID: created.ID, Scope: created.Scope, Sequence: 1, To: MemoryIntentStatusAccepted,
			Actor: created.Actor, Reason: created.Reason, DiagnosticCategory: MemoryIntentDiagnosticAccepted,
			OccurredAt: created.CreatedAt,
		}); err != nil {
			return MemoryIntentRecord{}, fmt.Errorf("record memory intent transition: %w", err)
		}
	}
	if enqueuer, ok := s.Processor.(MemoryIntentEnqueuer); ok && acceptedOrReplay {
		if err := enqueuer.EnqueueMemoryIntent(ctx, created); err != nil {
			return MemoryIntentRecord{}, fmt.Errorf("enqueue memory intent: %w", err)
		}
	}
	return created, nil
}
