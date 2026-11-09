package memory

import (
	"context"
	"fmt"
	"strings"
)

// VerificationCorrectionRequest contains only the administrator's governed
// request metadata. Target, content, evidence, and policy attribution are
// resolved from the immutable verification record, never accepted from this
// caller-controlled request.
type VerificationCorrectionRequest struct {
	Scope          Scope
	VerificationID string
	Actor          string
	Reason         string
	RequestID      string
	OperationID    string
	IdempotencyKey string
}

func (r VerificationCorrectionRequest) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"verification correction verification id": r.VerificationID,
		"verification correction actor":           r.Actor,
		"verification correction reason":          r.Reason,
		"verification correction request id":      r.RequestID,
		"verification correction operation id":    r.OperationID,
		"verification correction idempotency key": r.IdempotencyKey,
	} {
		if !boundedCorrectionIdentity(value) {
			return fmt.Errorf("%s is required and bounded", name)
		}
	}
	return nil
}

type VerificationCorrectionReader interface {
	ReadProceduralMemoryVerification(context.Context, Scope, string) (ProceduralMemoryVerification, error)
}

// VerificationAttributedIntentSubmitter is deliberately distinct from the
// generic intent service. Its only input attribution is server-resolved by
// VerificationCorrectionAdmissionService, preventing an ordinary caller from
// forging a verification correction path.
type VerificationAttributedIntentSubmitter interface {
	SubmitVerificationAttributedIntent(context.Context, MemoryIntentInput, VerificationAttribution) (MemoryIntentRecord, error)
}

// VerificationCorrectionAdmissionService is the sole domain admission path
// from a refuted procedural verification to a governed update. It has neither
// candidate nor canonical-memory mutation capability.
type VerificationCorrectionAdmissionService struct {
	Reader    VerificationCorrectionReader
	Targets   ProceduralVerificationTargetResolver
	Evidence  ProceduralVerificationEvidenceResolver
	Policy    ProceduralVerificationPolicy
	Submitter VerificationAttributedIntentSubmitter
}

func (s VerificationCorrectionAdmissionService) Submit(ctx context.Context, request VerificationCorrectionRequest) (MemoryIntentRecord, error) {
	if err := request.Validate(); err != nil {
		return MemoryIntentRecord{}, err
	}
	if s.Reader == nil || s.Targets == nil || s.Evidence == nil || s.Policy == nil || s.Submitter == nil {
		return MemoryIntentRecord{}, fmt.Errorf("verification correction admission is not configured")
	}
	decision, err := s.Policy.EvaluateProceduralVerification(ctx, request.Scope)
	if err != nil {
		return MemoryIntentRecord{}, fmt.Errorf("evaluate verification correction policy: %w", err)
	}
	if !decision.Enabled {
		return MemoryIntentRecord{}, fmt.Errorf("procedural verification policy is disabled")
	}
	verification, err := s.Reader.ReadProceduralMemoryVerification(ctx, request.Scope, request.VerificationID)
	if err != nil {
		return MemoryIntentRecord{}, fmt.Errorf("read verification correction source: %w", err)
	}
	if err := verification.Validate(); err != nil {
		return MemoryIntentRecord{}, fmt.Errorf("verification correction source is invalid: %w", err)
	}
	if verification.Scope.Normalized() != request.Scope.Normalized() {
		return MemoryIntentRecord{}, ErrNotFound
	}
	if verification.Disposition != ProceduralVerificationDispositionRefuted || strings.TrimSpace(verification.CorrectedContent) == "" {
		return MemoryIntentRecord{}, fmt.Errorf("verification correction is unavailable")
	}
	if strings.TrimSpace(decision.Version) == "" || strings.TrimSpace(decision.Version) != strings.TrimSpace(verification.PolicyVersion) {
		return MemoryIntentRecord{}, fmt.Errorf("verification correction policy is unavailable")
	}
	if err := s.Targets.ValidateProceduralVerificationTarget(ctx, verification); err != nil {
		return MemoryIntentRecord{}, fmt.Errorf("validate verification correction target: %w", err)
	}
	for index, evidence := range verification.Evidence {
		if evidence.Scope == (Scope{}) {
			evidence.Scope = verification.Scope.Normalized()
		}
		if err := s.Evidence.ValidateProceduralVerificationEvidence(ctx, request.Scope, evidence); err != nil {
			return MemoryIntentRecord{}, fmt.Errorf("validate verification correction evidence %d: %w", index, err)
		}
	}
	input := MemoryIntentInput{
		Scope: request.Scope.Normalized(), Type: MemoryIntentUpdate, TargetMemoryID: verification.MemoryID, TargetVersion: verification.TargetVersion,
		Content: verification.CorrectedContent, Actor: strings.TrimSpace(request.Actor), Reason: strings.TrimSpace(request.Reason),
		RequestID: strings.TrimSpace(request.RequestID), OperationID: strings.TrimSpace(request.OperationID), IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
		Evidence: []MemoryIntentEvidence{{Scope: request.Scope.Normalized(), Kind: "procedural_verification", ID: verification.ID, Version: verification.TargetVersion}},
	}
	attribution := VerificationAttribution{VerificationID: verification.ID, TargetMemoryID: verification.MemoryID, TargetVersion: verification.TargetVersion, SourceWatermark: verification.SourceWatermark, PolicyVersion: strings.TrimSpace(decision.Version)}
	if err := attribution.Validate(); err != nil {
		return MemoryIntentRecord{}, err
	}
	return s.Submitter.SubmitVerificationAttributedIntent(ctx, input, attribution)
}
