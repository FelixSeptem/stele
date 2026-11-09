package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/telemetry"
	"unicode/utf8"
)

// ErrNotFound is returned for a resource that is absent from the caller's
// exact scope. Callers must not distinguish an absent record from a foreign
// one.
var ErrNotFound = errors.New("memory resource not found")

var ErrProceduralRevalidationLeaseLost = errors.New("procedural revalidation lease ownership lost")

// The verification limits intentionally keep the append-only ledger compact.
// Source content remains at its authoritative source; the ledger carries only
// bounded identifiers, versions, and the operator's bounded correction.
const (
	ProceduralVerificationMaxIDBytes               = 256
	ProceduralVerificationMaxEvidence              = 32
	ProceduralVerificationMaxActorBytes            = 256
	ProceduralVerificationMaxReasonBytes           = 4096
	ProceduralVerificationMaxPolicyVersionBytes    = 256
	ProceduralVerificationMaxSourceWatermarkBytes  = 256
	ProceduralVerificationMaxCorrectedContentBytes = MemoryIntentMaxContentBytes
)

type ProceduralVerificationDisposition string

const (
	ProceduralVerificationDispositionSupported         ProceduralVerificationDisposition = "supported"
	ProceduralVerificationDispositionRefuted           ProceduralVerificationDisposition = "refuted"
	ProceduralVerificationDispositionInconclusive      ProceduralVerificationDisposition = "inconclusive"
	ProceduralVerificationDispositionNeedsRevalidation ProceduralVerificationDisposition = "needs_revalidation"
)

func (d ProceduralVerificationDisposition) Valid() bool {
	switch d {
	case ProceduralVerificationDispositionSupported,
		ProceduralVerificationDispositionRefuted,
		ProceduralVerificationDispositionInconclusive,
		ProceduralVerificationDispositionNeedsRevalidation:
		return true
	default:
		return false
	}
}

type ProceduralVerificationEvidenceKind string

const (
	ProceduralVerificationEvidenceToolObservation    ProceduralVerificationEvidenceKind = "tool_observation"
	ProceduralVerificationEvidenceUserCorrection     ProceduralVerificationEvidenceKind = "user_correction"
	ProceduralVerificationEvidenceExternalEvaluation ProceduralVerificationEvidenceKind = "external_evaluation"
	ProceduralVerificationEvidenceTranscript         ProceduralVerificationEvidenceKind = "transcript"
	ProceduralVerificationEvidenceModelAssertion     ProceduralVerificationEvidenceKind = "model_assertion"
)

func (k ProceduralVerificationEvidenceKind) Valid() bool {
	switch k {
	case ProceduralVerificationEvidenceToolObservation,
		ProceduralVerificationEvidenceUserCorrection,
		ProceduralVerificationEvidenceExternalEvaluation,
		ProceduralVerificationEvidenceTranscript,
		ProceduralVerificationEvidenceModelAssertion:
		return true
	default:
		return false
	}
}

// IndependentlyCorroborating reports whether this kind can be the independent
// source required to support a claim. A caller cannot elevate a transcript or
// model output by setting Corroborates to true.
func (k ProceduralVerificationEvidenceKind) IndependentlyCorroborating() bool {
	switch k {
	case ProceduralVerificationEvidenceToolObservation,
		ProceduralVerificationEvidenceUserCorrection,
		ProceduralVerificationEvidenceExternalEvaluation:
		return true
	default:
		return false
	}
}

type ProceduralVerificationEvidencePolarity string

const (
	ProceduralVerificationEvidenceSupporting ProceduralVerificationEvidencePolarity = "supporting"
	ProceduralVerificationEvidenceContrary   ProceduralVerificationEvidencePolarity = "contrary"
	ProceduralVerificationEvidenceNeutral    ProceduralVerificationEvidencePolarity = "neutral"
)

func (p ProceduralVerificationEvidencePolarity) Valid() bool {
	switch p {
	case ProceduralVerificationEvidenceSupporting,
		ProceduralVerificationEvidenceContrary,
		ProceduralVerificationEvidenceNeutral:
		return true
	default:
		return false
	}
}

// ProceduralVerificationEvidenceSourceKind identifies the durable record that
// supplied an observation. It is separate from evidence Kind (the method used
// to observe it), allowing the revalidator to re-resolve exactly this source
// and version without selecting new evidence.
type ProceduralVerificationEvidenceSourceKind string

const (
	ProceduralVerificationEvidenceSourceRawEvent           ProceduralVerificationEvidenceSourceKind = "raw_event"
	ProceduralVerificationEvidenceSourceMemoryVersion      ProceduralVerificationEvidenceSourceKind = "memory_version"
	ProceduralVerificationEvidenceSourceUsefulnessFeedback ProceduralVerificationEvidenceSourceKind = "usefulness_feedback"
	ProceduralVerificationEvidenceSourceExternalEvaluation ProceduralVerificationEvidenceSourceKind = "external_evaluation"
)

func (k ProceduralVerificationEvidenceSourceKind) Valid() bool {
	switch k {
	case ProceduralVerificationEvidenceSourceRawEvent,
		ProceduralVerificationEvidenceSourceMemoryVersion,
		ProceduralVerificationEvidenceSourceUsefulnessFeedback,
		ProceduralVerificationEvidenceSourceExternalEvaluation:
		return true
	default:
		return false
	}
}

type ProceduralVerificationEvidence struct {
	Scope         Scope                                    `json:"scope"`
	Kind          ProceduralVerificationEvidenceKind       `json:"kind"`
	SourceKind    ProceduralVerificationEvidenceSourceKind `json:"source_kind"`
	Polarity      ProceduralVerificationEvidencePolarity   `json:"polarity"`
	ID            string                                   `json:"id"`
	SourceVersion int64                                    `json:"source_version"`
	Corroborates  bool                                     `json:"corroborates"`
}

func (e ProceduralVerificationEvidence) Validate(scope Scope) error {
	// Evidence inherits the record's scope when omitted. That keeps the public
	// assertion concise while a caller that does provide a scope must prove the
	// exact same isolation boundary.
	if e.Scope != (Scope{}) {
		if err := e.Scope.Validate(); err != nil {
			return fmt.Errorf("evidence scope: %w", err)
		}
		if e.Scope.Normalized() != scope.Normalized() {
			return fmt.Errorf("evidence scope must match verification scope")
		}
	}
	if !e.Kind.Valid() {
		return fmt.Errorf("evidence kind %q is invalid", e.Kind)
	}
	if !e.SourceKind.Valid() {
		return fmt.Errorf("evidence source kind %q is invalid", e.SourceKind)
	}
	if e.Polarity != "" && !e.Polarity.Valid() {
		return fmt.Errorf("evidence polarity %q is invalid", e.Polarity)
	}
	if err := boundedRequiredText("evidence id", e.ID, ProceduralVerificationMaxIDBytes); err != nil {
		return err
	}
	if e.SourceVersion <= 0 {
		return fmt.Errorf("evidence source version must be greater than zero")
	}
	return nil
}

type ProceduralMemoryVerification struct {
	ID                       string                            `json:"id"`
	Scope                    Scope                             `json:"scope"`
	MemoryID                 string                            `json:"memory_id"`
	TargetVersion            int64                             `json:"target_version"`
	ClaimFingerprint         string                            `json:"claim_fingerprint"`
	ApplicabilityFingerprint string                            `json:"applicability_fingerprint"`
	Disposition              ProceduralVerificationDisposition `json:"disposition"`
	Evidence                 []ProceduralVerificationEvidence  `json:"evidence"`
	Actor                    string                            `json:"actor"`
	Reason                   string                            `json:"reason"`
	PolicyVersion            string                            `json:"policy_version"`
	SourceWatermark          string                            `json:"source_watermark"`
	CorrectedContent         string                            `json:"corrected_content,omitempty"`
	CreatedAt                time.Time                         `json:"created_at"`
}

// ProceduralVerificationStore owns append-only persistence. Target and
// evidence validation remain separate because a generic caller must never be
// able to turn an arbitrary identifier into proof by writing it to the ledger.
type ProceduralVerificationStore interface {
	AppendProceduralMemoryVerification(context.Context, ProceduralMemoryVerification) (ProceduralMemoryVerification, error)
}

type ProceduralVerificationTargetResolver interface {
	ValidateProceduralVerificationTarget(context.Context, ProceduralMemoryVerification) error
}

type ProceduralVerificationEvidenceResolver interface {
	ValidateProceduralVerificationEvidence(context.Context, Scope, ProceduralVerificationEvidence) error
}

type ProceduralVerificationPolicyDecision struct {
	Enabled bool
	Version string
}

type ProceduralVerificationPolicy interface {
	EvaluateProceduralVerification(context.Context, Scope) (ProceduralVerificationPolicyDecision, error)
}

// StaticProceduralVerificationPolicy is the process-local policy adapter used
// by configuration wiring. It is exact-scope so enabling an installation never
// silently turns the workflow on for every tenant.
type StaticProceduralVerificationPolicy struct {
	Scope   Scope
	Enabled bool
	Version string
}

func (p StaticProceduralVerificationPolicy) EvaluateProceduralVerification(_ context.Context, scope Scope) (ProceduralVerificationPolicyDecision, error) {
	if err := scope.Validate(); err != nil {
		return ProceduralVerificationPolicyDecision{}, err
	}
	if err := p.Scope.Validate(); err != nil || p.Scope.Normalized() != scope.Normalized() {
		return ProceduralVerificationPolicyDecision{Enabled: false, Version: strings.TrimSpace(p.Version)}, nil
	}
	return ProceduralVerificationPolicyDecision{Enabled: p.Enabled, Version: strings.TrimSpace(p.Version)}, nil
}

// ProceduralVerificationService admits only attributable administrator
// assertions. It contains no retrieval or model invocation: each cited source
// must be resolved by the repository-owned resolver before persistence.
type ProceduralVerificationService struct {
	Store    ProceduralVerificationStore
	Targets  ProceduralVerificationTargetResolver
	Evidence ProceduralVerificationEvidenceResolver
	Policy   ProceduralVerificationPolicy
	Observer interface {
		RecordProceduralVerification(context.Context, telemetry.ProceduralVerificationEvent)
	}
}

// ProceduralVerificationLineage is the bounded admin audit projection. It is
// intentionally not embedded in MemoryHistory or QueryStore: ordinary memory
// history and Provider retrieval must not acquire a verification disclosure
// path merely because the ledger exists.
type ProceduralVerificationLineage struct {
	Scope         Scope
	MemoryID      string
	TargetVersion int64
	Verifications []ProceduralMemoryVerification
	Runs          []ProceduralRevalidationRun
	Intents       []ProceduralVerificationIntentLink
}

// ProceduralVerificationIntentLink is a bounded, identifier-only audit edge
// from immutable evidence through governed work. It deliberately omits intent
// content, candidate content, and provenance payloads; those are not needed
// to follow correction history and must not become a new disclosure surface.
type ProceduralVerificationIntentLink struct {
	VerificationID    string
	IntentID          string
	Status            MemoryIntentStatus
	CandidateID       string
	SuccessorMemoryID string
	SuccessorVersion  int64
}

func (l ProceduralVerificationLineage) Validate() error {
	if err := l.Scope.Validate(); err != nil {
		return fmt.Errorf("procedural verification lineage scope: %w", err)
	}
	if err := boundedRequiredText("procedural verification lineage memory id", l.MemoryID, ProceduralVerificationMaxIDBytes); err != nil {
		return err
	}
	if l.TargetVersion <= 0 {
		return fmt.Errorf("procedural verification lineage target version must be greater than zero")
	}
	if len(l.Verifications) > 100 || len(l.Runs) > 100 || len(l.Intents) > 100 {
		return fmt.Errorf("procedural verification lineage exceeds bounded history")
	}
	for index, verification := range l.Verifications {
		if verification.Scope.Normalized() != l.Scope.Normalized() || verification.MemoryID != l.MemoryID || verification.TargetVersion != l.TargetVersion {
			return fmt.Errorf("procedural verification lineage verification %d has mismatched scope or target", index)
		}
	}
	for index, run := range l.Runs {
		// A redacted run projection may omit fields in a future list view. When
		// present, identity fields must still bind exactly to this lineage.
		if run.ID == "" {
			return fmt.Errorf("procedural verification lineage run %d is invalid", index)
		}
		if run.Scope != (Scope{}) && (run.Scope.Normalized() != l.Scope.Normalized() || run.MemoryID != l.MemoryID || run.TargetVersion != l.TargetVersion) {
			return fmt.Errorf("procedural verification lineage run %d has mismatched scope or target", index)
		}
	}
	for index, link := range l.Intents {
		if err := boundedRequiredText("procedural verification lineage intent verification id", link.VerificationID, ProceduralVerificationMaxIDBytes); err != nil {
			return fmt.Errorf("procedural verification lineage intent %d: %w", index, err)
		}
		if err := boundedRequiredText("procedural verification lineage intent id", link.IntentID, ProceduralVerificationMaxIDBytes); err != nil {
			return fmt.Errorf("procedural verification lineage intent %d: %w", index, err)
		}
		if !link.Status.Valid() {
			return fmt.Errorf("procedural verification lineage intent %d has invalid status", index)
		}
		if link.CandidateID != "" && len(link.CandidateID) > ProceduralVerificationMaxIDBytes {
			return fmt.Errorf("procedural verification lineage intent %d candidate id exceeds bound", index)
		}
		if link.SuccessorMemoryID != "" && len(link.SuccessorMemoryID) > ProceduralVerificationMaxIDBytes {
			return fmt.Errorf("procedural verification lineage intent %d successor memory id exceeds bound", index)
		}
		if link.SuccessorVersion < 0 || (link.SuccessorMemoryID == "" && link.SuccessorVersion != 0) {
			return fmt.Errorf("procedural verification lineage intent %d has invalid successor", index)
		}
	}
	return nil
}

func (s ProceduralVerificationService) Record(ctx context.Context, record ProceduralMemoryVerification) (ProceduralMemoryVerification, error) {
	if err := record.Validate(); err != nil {
		s.recordTelemetry(ctx, "denied", "none", "validation")
		return ProceduralMemoryVerification{}, err
	}
	if s.Store == nil || s.Targets == nil || s.Evidence == nil || s.Policy == nil {
		s.recordTelemetry(ctx, "failed", "none", "dependency")
		return ProceduralMemoryVerification{}, fmt.Errorf("procedural verification service is not configured")
	}
	decision, err := s.Policy.EvaluateProceduralVerification(ctx, record.Scope)
	if err != nil {
		s.recordTelemetry(ctx, "failed", "none", "dependency")
		return ProceduralMemoryVerification{}, fmt.Errorf("evaluate procedural verification policy: %w", err)
	}
	if !decision.Enabled {
		s.recordTelemetry(ctx, "disabled", "none", "policy_disabled")
		return ProceduralMemoryVerification{}, fmt.Errorf("procedural verification policy is disabled")
	}
	if err := s.Targets.ValidateProceduralVerificationTarget(ctx, record); err != nil {
		s.recordTelemetry(ctx, "denied", "none", "target_ineligible")
		return ProceduralMemoryVerification{}, fmt.Errorf("validate procedural verification target: %w", err)
	}
	for index, evidence := range record.Evidence {
		if evidence.Scope == (Scope{}) {
			evidence.Scope = record.Scope.Normalized()
		}
		if err := s.Evidence.ValidateProceduralVerificationEvidence(ctx, record.Scope, evidence); err != nil {
			s.recordTelemetry(ctx, "denied", "none", "source_ineligible")
			return ProceduralMemoryVerification{}, fmt.Errorf("validate procedural verification evidence %d: %w", index, err)
		}
	}
	stored, err := s.Store.AppendProceduralMemoryVerification(ctx, record)
	if err != nil {
		s.recordTelemetry(ctx, "failed", "none", "dependency")
		return ProceduralMemoryVerification{}, fmt.Errorf("append procedural verification: %w", err)
	}
	s.recordTelemetry(ctx, "accepted", string(stored.Disposition), "none")
	return stored, nil
}

func (s ProceduralVerificationService) recordTelemetry(ctx context.Context, result, disposition, failureCategory string) {
	if s.Observer == nil {
		return
	}
	s.Observer.RecordProceduralVerification(ctx, telemetry.ProceduralVerificationEvent{
		Operation: "record", Result: result, Disposition: disposition, Trigger: "administrator", Queue: "none", Recovery: "none", FailureCategory: failureCategory,
	})
}

// ProceduralRevalidationTrigger identifies why a previously recorded
// verification must be revisited. It is deliberately a closed set: queue
// consumers must never turn access volume or model output into a review.
type ProceduralRevalidationTrigger string

const (
	ProceduralRevalidationTriggerAdministrator       ProceduralRevalidationTrigger = "administrator"
	ProceduralRevalidationTriggerSourceDrift         ProceduralRevalidationTrigger = "source_drift"
	ProceduralRevalidationTriggerCorrectionSignal    ProceduralRevalidationTrigger = "correction_signal"
	ProceduralRevalidationTriggerStaleFeedback       ProceduralRevalidationTrigger = "stale_feedback"
	ProceduralRevalidationTriggerNeedsReviewFeedback ProceduralRevalidationTrigger = "needs_review_feedback"
)

func (t ProceduralRevalidationTrigger) Valid() bool {
	switch t {
	case ProceduralRevalidationTriggerAdministrator,
		ProceduralRevalidationTriggerSourceDrift,
		ProceduralRevalidationTriggerCorrectionSignal,
		ProceduralRevalidationTriggerStaleFeedback,
		ProceduralRevalidationTriggerNeedsReviewFeedback:
		return true
	default:
		return false
	}
}

type ProceduralRevalidationRunStatus string

const (
	ProceduralRevalidationRunStatusPending   ProceduralRevalidationRunStatus = "pending"
	ProceduralRevalidationRunStatusRunning   ProceduralRevalidationRunStatus = "running"
	ProceduralRevalidationRunStatusCompleted ProceduralRevalidationRunStatus = "completed"
	ProceduralRevalidationRunStatusFailed    ProceduralRevalidationRunStatus = "failed"
	ProceduralRevalidationRunStatusExhausted ProceduralRevalidationRunStatus = "exhausted"
)

func (s ProceduralRevalidationRunStatus) Valid() bool {
	switch s {
	case ProceduralRevalidationRunStatusPending,
		ProceduralRevalidationRunStatusRunning,
		ProceduralRevalidationRunStatusCompleted,
		ProceduralRevalidationRunStatusFailed,
		ProceduralRevalidationRunStatusExhausted:
		return true
	default:
		return false
	}
}

// ProceduralRevalidationRun contains durable mutable work state only. Its
// terminal decision is linked back to an immutable verification record rather
// than being stored as a mutable assertion in this row.
type ProceduralRevalidationRun struct {
	ID                     string                          `json:"id"`
	Scope                  Scope                           `json:"scope"`
	VerificationID         string                          `json:"verification_id"`
	MemoryID               string                          `json:"memory_id"`
	TargetVersion          int64                           `json:"target_version"`
	SourceWatermark        string                          `json:"source_watermark"`
	Trigger                ProceduralRevalidationTrigger   `json:"trigger"`
	WorkIdentity           string                          `json:"work_identity"`
	Status                 ProceduralRevalidationRunStatus `json:"status"`
	Attempt                int                             `json:"attempt"`
	MaxAttempts            int                             `json:"max_attempts"`
	LeaseOwner             string                          `json:"lease_owner,omitempty"`
	LeaseExpiresAt         time.Time                       `json:"lease_expires_at,omitempty"`
	NextAttemptAt          time.Time                       `json:"next_attempt_at,omitempty"`
	TerminalVerificationID string                          `json:"terminal_verification_id,omitempty"`
	FailureCategory        string                          `json:"failure_category,omitempty"`
	CreatedAt              time.Time                       `json:"created_at"`
	UpdatedAt              time.Time                       `json:"updated_at,omitempty"`
}

// ProceduralRevalidationRequest is an administrator or eligible maintenance
// signal asking to revisit one already-recorded verification. It contains no
// claim or evidence content: the service re-reads the exact immutable ledger
// record, and workers later re-resolve only its cited sources.
type ProceduralRevalidationRequest struct {
	Scope          Scope
	VerificationID string
	Trigger        ProceduralRevalidationTrigger
	Actor          string
	Reason         string
}

func (r ProceduralRevalidationRequest) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return fmt.Errorf("revalidation request scope: %w", err)
	}
	if err := boundedRequiredText("revalidation verification id", r.VerificationID, ProceduralVerificationMaxIDBytes); err != nil {
		return err
	}
	if !r.Trigger.Valid() {
		return fmt.Errorf("revalidation request trigger %q is invalid", r.Trigger)
	}
	if err := boundedRequiredText("revalidation actor", r.Actor, ProceduralVerificationMaxActorBytes); err != nil {
		return err
	}
	if err := boundedRequiredText("revalidation reason", r.Reason, ProceduralVerificationMaxReasonBytes); err != nil {
		return err
	}
	return nil
}

// ProceduralRevalidationDispatch is the narrow handoff to a derived-work
// adapter. It purposely carries only durable references and queue identity,
// never evidence, claim, correction, actor, or reason content.
type ProceduralRevalidationDispatch struct {
	Scope     Scope
	RunID     string
	Watermark string
	Identity  string
}

func (d ProceduralRevalidationDispatch) Validate() error {
	if err := d.Scope.Validate(); err != nil {
		return fmt.Errorf("revalidation dispatch scope: %w", err)
	}
	for name, value := range map[string]string{
		"revalidation dispatch run id":    d.RunID,
		"revalidation dispatch watermark": d.Watermark,
		"revalidation dispatch identity":  d.Identity,
	} {
		if err := boundedRequiredText(name, value, ProceduralVerificationMaxIDBytes); err != nil {
			return err
		}
	}
	return nil
}

// ProceduralRevalidationRequestStore joins the immutable ledger read and
// durable, idempotent run creation without exposing a generic mutation API.
type ProceduralRevalidationRequestStore interface {
	ReadProceduralMemoryVerification(context.Context, Scope, string) (ProceduralMemoryVerification, error)
	CreateProceduralRevalidationRun(context.Context, ProceduralRevalidationRun) (ProceduralRevalidationRun, error)
}

// ProceduralRevalidationDispatchSink is implemented at the jobs boundary.
// Defining it here keeps the memory domain independent from queue and runtime
// packages, so neither admission nor tests introduce an import cycle.
type ProceduralRevalidationDispatchSink interface {
	DispatchProceduralRevalidation(context.Context, ProceduralRevalidationDispatch) error
}

// ProceduralRevalidationService records a bounded, replay-safe work request.
// It never selects evidence, invokes a model, or decides truth. If durable
// creation succeeds but dispatch fails, the pending run is intentionally
// retained for the bounded scheduler/recovery path rather than being silently
// discarded.
type ProceduralRevalidationService struct {
	Store       ProceduralRevalidationRequestStore
	Policy      ProceduralVerificationPolicy
	Dispatcher  ProceduralRevalidationDispatchSink
	Now         func() time.Time
	NewID       func() string
	MaxAttempts int
	Observer    interface {
		RecordProceduralVerification(context.Context, telemetry.ProceduralVerificationEvent)
	}
}

func (s ProceduralRevalidationService) Request(ctx context.Context, request ProceduralRevalidationRequest) (ProceduralRevalidationRun, error) {
	if err := request.Validate(); err != nil {
		s.recordTelemetry(ctx, request.Trigger, "denied", "none", "validation")
		return ProceduralRevalidationRun{}, err
	}
	if s.Policy == nil {
		s.recordTelemetry(ctx, request.Trigger, "failed", "none", "dependency")
		return ProceduralRevalidationRun{}, fmt.Errorf("procedural revalidation policy is not configured")
	}
	decision, err := s.Policy.EvaluateProceduralVerification(ctx, request.Scope)
	if err != nil {
		s.recordTelemetry(ctx, request.Trigger, "failed", "none", "dependency")
		return ProceduralRevalidationRun{}, fmt.Errorf("evaluate procedural revalidation policy: %w", err)
	}
	if !decision.Enabled {
		s.recordTelemetry(ctx, request.Trigger, "disabled", "none", "policy_disabled")
		return ProceduralRevalidationRun{}, fmt.Errorf("procedural verification policy is disabled")
	}
	if s.Store == nil || s.Dispatcher == nil || s.NewID == nil {
		s.recordTelemetry(ctx, request.Trigger, "failed", "none", "dependency")
		return ProceduralRevalidationRun{}, fmt.Errorf("procedural revalidation service is not configured")
	}
	verification, err := s.Store.ReadProceduralMemoryVerification(ctx, request.Scope, request.VerificationID)
	if err != nil {
		s.recordTelemetry(ctx, request.Trigger, "failed", "none", "dependency")
		return ProceduralRevalidationRun{}, fmt.Errorf("read procedural verification for revalidation: %w", err)
	}
	if err := verification.Validate(); err != nil {
		s.recordTelemetry(ctx, request.Trigger, "denied", "none", "target_ineligible")
		return ProceduralRevalidationRun{}, fmt.Errorf("stored procedural verification is invalid: %w", err)
	}
	if verification.Scope.Normalized() != request.Scope.Normalized() {
		s.recordTelemetry(ctx, request.Trigger, "denied", "none", "scope_denied")
		return ProceduralRevalidationRun{}, ErrNotFound
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	maxAttempts := s.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	run := ProceduralRevalidationRun{
		ID:              strings.TrimSpace(s.NewID()),
		Scope:           request.Scope.Normalized(),
		VerificationID:  verification.ID,
		MemoryID:        verification.MemoryID,
		TargetVersion:   verification.TargetVersion,
		SourceWatermark: verification.SourceWatermark,
		Trigger:         request.Trigger,
		WorkIdentity: proceduralRevalidationWorkIdentity(
			request.Scope, verification.ID, verification.TargetVersion, verification.SourceWatermark, request.Trigger,
		),
		Status:        ProceduralRevalidationRunStatusPending,
		MaxAttempts:   maxAttempts,
		NextAttemptAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := run.Validate(); err != nil {
		s.recordTelemetry(ctx, request.Trigger, "denied", "none", "validation")
		return ProceduralRevalidationRun{}, fmt.Errorf("construct procedural revalidation run: %w", err)
	}
	stored, err := s.Store.CreateProceduralRevalidationRun(ctx, run)
	if err != nil {
		s.recordTelemetry(ctx, request.Trigger, "failed", "none", "dependency")
		return ProceduralRevalidationRun{}, fmt.Errorf("create procedural revalidation run: %w", err)
	}
	if err := s.Dispatcher.DispatchProceduralRevalidation(ctx, ProceduralRevalidationDispatch{
		Scope: stored.Scope, RunID: stored.ID, Watermark: stored.SourceWatermark, Identity: stored.WorkIdentity,
	}); err != nil {
		s.recordTelemetry(ctx, request.Trigger, "failed", "durable", "dependency")
		return ProceduralRevalidationRun{}, fmt.Errorf("dispatch procedural revalidation run: %w", err)
	}
	s.recordTelemetry(ctx, stored.Trigger, "accepted", "durable", "none")
	return stored, nil
}

func (s ProceduralRevalidationService) recordTelemetry(ctx context.Context, trigger ProceduralRevalidationTrigger, result, queue, failureCategory string) {
	if s.Observer == nil {
		return
	}
	s.Observer.RecordProceduralVerification(ctx, telemetry.ProceduralVerificationEvent{
		Operation: "request", Result: result, Disposition: "none", Trigger: string(trigger), Queue: queue, Recovery: "none", FailureCategory: failureCategory,
	})
}

func proceduralRevalidationWorkIdentity(scope Scope, verificationID string, targetVersion int64, sourceWatermark string, trigger ProceduralRevalidationTrigger) string {
	scope = scope.Normalized()
	material := strings.Join([]string{
		scope.Tenant, scope.Project, scope.Namespace, strings.TrimSpace(verificationID),
		fmt.Sprintf("%d", targetVersion), strings.TrimSpace(sourceWatermark), string(trigger),
	}, "\x00")
	digest := sha256.Sum256([]byte(material))
	return hex.EncodeToString(digest[:])
}

type ClaimProceduralRevalidationRunsInput struct {
	Scope         Scope
	WorkerID      string
	Now           time.Time
	LeaseDuration time.Duration
	Limit         int
}

// ListPendingProceduralRevalidationRunsInput is used by the dispatch recovery
// job. It intentionally observes only durable pending runs; queue delivery is
// idempotent through the run's work identity and does not claim the child run.
type ListPendingProceduralRevalidationRunsInput struct {
	Scope Scope
	Now   time.Time
	Limit int
}

func (i ListPendingProceduralRevalidationRunsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Now.IsZero() {
		return fmt.Errorf("pending revalidation list time is required")
	}
	if i.Limit <= 0 || i.Limit > ProceduralVerificationMaxEvidence {
		return fmt.Errorf("pending revalidation list limit must be between one and %d", ProceduralVerificationMaxEvidence)
	}
	return nil
}

// ClaimProceduralRevalidationRunInput binds derived queue delivery to the
// durable run it references. The same run may be redelivered after a queue
// restart, but only one worker can own its lease at a time.
type ClaimProceduralRevalidationRunInput struct {
	Scope         Scope
	RunID         string
	WorkerID      string
	Now           time.Time
	LeaseDuration time.Duration
}

func (i ClaimProceduralRevalidationRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := boundedRequiredText("revalidation run id", i.RunID, ProceduralVerificationMaxIDBytes); err != nil {
		return err
	}
	if err := boundedRequiredText("revalidation worker id", i.WorkerID, ProceduralVerificationMaxIDBytes); err != nil {
		return err
	}
	if i.Now.IsZero() || i.LeaseDuration <= 0 || i.LeaseDuration > time.Hour {
		return fmt.Errorf("revalidation run lease input is invalid")
	}
	return nil
}

type CompleteProceduralRevalidationRunInput struct {
	Scope                  Scope
	RunID                  string
	WorkerID               string
	Status                 ProceduralRevalidationRunStatus
	TerminalVerificationID string
	CompletedAt            time.Time
}

// ExhaustProceduralRevalidationRunInput settles a durable run after its
// corresponding derived-work delivery has consumed its bounded retry budget.
// The repository derives the original verification link itself so a queue
// caller can never forge a terminal evidence reference.
type ExhaustProceduralRevalidationRunInput struct {
	Scope       Scope
	RunID       string
	WorkerID    string
	ExhaustedAt time.Time
}

func (i ExhaustProceduralRevalidationRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := boundedRequiredText("revalidation run id", i.RunID, ProceduralVerificationMaxIDBytes); err != nil {
		return err
	}
	if err := boundedRequiredText("revalidation worker id", i.WorkerID, ProceduralVerificationMaxIDBytes); err != nil {
		return err
	}
	if i.ExhaustedAt.IsZero() {
		return fmt.Errorf("revalidation exhaustion time is required")
	}
	return nil
}

func (i CompleteProceduralRevalidationRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"revalidation run id": i.RunID, "revalidation worker id": i.WorkerID,
		"terminal verification id": i.TerminalVerificationID,
	} {
		if err := boundedRequiredText(name, value, ProceduralVerificationMaxIDBytes); err != nil {
			return err
		}
	}
	if i.Status != ProceduralRevalidationRunStatusCompleted && i.Status != ProceduralRevalidationRunStatusFailed && i.Status != ProceduralRevalidationRunStatusExhausted {
		return fmt.Errorf("revalidation completion status %q is invalid", i.Status)
	}
	if i.CompletedAt.IsZero() {
		return fmt.Errorf("revalidation completion time is required")
	}
	return nil
}

func (i ClaimProceduralRevalidationRunsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := boundedRequiredText("revalidation worker id", i.WorkerID, ProceduralVerificationMaxIDBytes); err != nil {
		return err
	}
	if i.Now.IsZero() {
		return fmt.Errorf("revalidation claim time is required")
	}
	if i.LeaseDuration <= 0 || i.LeaseDuration > time.Hour {
		return fmt.Errorf("revalidation lease duration must be between zero and one hour")
	}
	if i.Limit <= 0 || i.Limit > ProceduralVerificationMaxEvidence {
		return fmt.Errorf("revalidation claim limit must be between one and %d", ProceduralVerificationMaxEvidence)
	}
	return nil
}

func (r ProceduralRevalidationRun) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return fmt.Errorf("revalidation scope: %w", err)
	}
	for name, value := range map[string]string{
		"revalidation run id": r.ID, "verification id": r.VerificationID,
		"target memory id": r.MemoryID, "source watermark": r.SourceWatermark,
		"work identity": r.WorkIdentity,
	} {
		if err := boundedRequiredText(name, value, ProceduralVerificationMaxIDBytes); err != nil {
			return err
		}
	}
	if r.TargetVersion <= 0 {
		return fmt.Errorf("revalidation target version must be greater than zero")
	}
	if !r.Trigger.Valid() {
		return fmt.Errorf("revalidation trigger %q is invalid", r.Trigger)
	}
	if !r.Status.Valid() {
		return fmt.Errorf("revalidation status %q is invalid", r.Status)
	}
	if r.MaxAttempts <= 0 || r.MaxAttempts > 100 {
		return fmt.Errorf("revalidation max attempts must be between one and 100")
	}
	if r.Attempt < 0 || r.Attempt > r.MaxAttempts {
		return fmt.Errorf("revalidation attempt must be between zero and max attempts")
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("revalidation created at is required")
	}
	return nil
}

func (v ProceduralMemoryVerification) Validate() error {
	if err := v.Scope.Validate(); err != nil {
		return fmt.Errorf("verification scope: %w", err)
	}
	if err := boundedRequiredText("verification id", v.ID, ProceduralVerificationMaxIDBytes); err != nil {
		return err
	}
	if err := boundedRequiredText("target memory id", v.MemoryID, ProceduralVerificationMaxIDBytes); err != nil {
		return err
	}
	if v.TargetVersion <= 0 {
		return fmt.Errorf("target version must be greater than zero")
	}
	if !sha256Hex(v.ClaimFingerprint) || !sha256Hex(v.ApplicabilityFingerprint) {
		return fmt.Errorf("claim and applicability fingerprints must be SHA-256 hex")
	}
	if !v.Disposition.Valid() {
		return fmt.Errorf("verification disposition %q is invalid", v.Disposition)
	}
	if len(v.Evidence) == 0 || len(v.Evidence) > ProceduralVerificationMaxEvidence {
		return fmt.Errorf("verification evidence count must be between one and %d", ProceduralVerificationMaxEvidence)
	}
	independentCorroboration := false
	for index, evidence := range v.Evidence {
		if err := evidence.Validate(v.Scope); err != nil {
			return fmt.Errorf("evidence %d: %w", index, err)
		}
		if evidence.Corroborates && evidence.Kind.IndependentlyCorroborating() {
			independentCorroboration = true
		}
	}
	if v.Disposition == ProceduralVerificationDispositionSupported && !independentCorroboration {
		return fmt.Errorf("supported verification requires independent corroboration")
	}
	if v.Disposition != ProceduralVerificationDispositionRefuted && strings.TrimSpace(v.CorrectedContent) != "" {
		return fmt.Errorf("corrected content is only permitted for a refuted verification")
	}
	if len([]byte(v.CorrectedContent)) > ProceduralVerificationMaxCorrectedContentBytes {
		return fmt.Errorf("corrected content exceeds %d bytes", ProceduralVerificationMaxCorrectedContentBytes)
	}
	if err := boundedRequiredText("actor", v.Actor, ProceduralVerificationMaxActorBytes); err != nil {
		return err
	}
	if err := boundedRequiredText("reason", v.Reason, ProceduralVerificationMaxReasonBytes); err != nil {
		return err
	}
	if err := boundedRequiredText("policy version", v.PolicyVersion, ProceduralVerificationMaxPolicyVersionBytes); err != nil {
		return err
	}
	if err := boundedRequiredText("source watermark", v.SourceWatermark, ProceduralVerificationMaxSourceWatermarkBytes); err != nil {
		return err
	}
	if v.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	return nil
}

// ProceduralMemoryVerificationFingerprint returns the server-owned identity
// for an immutable assertion. Evidence is sorted so harmless input ordering
// differences cannot create duplicate ledger facts.
func ProceduralMemoryVerificationFingerprint(v ProceduralMemoryVerification) (string, error) {
	if err := v.Validate(); err != nil {
		return "", err
	}
	evidence := append([]ProceduralVerificationEvidence(nil), v.Evidence...)
	for index := range evidence {
		if evidence[index].Scope == (Scope{}) {
			evidence[index].Scope = v.Scope.Normalized()
		}
	}
	sort.Slice(evidence, func(i, j int) bool {
		left, right := evidence[i], evidence[j]
		return strings.Join([]string{left.Scope.Normalized().Tenant, left.Scope.Normalized().Project, left.Scope.Normalized().Namespace, string(left.Kind), string(left.SourceKind), left.ID, fmt.Sprint(left.SourceVersion), string(left.Polarity), fmt.Sprint(left.Corroborates)}, "\x00") <
			strings.Join([]string{right.Scope.Normalized().Tenant, right.Scope.Normalized().Project, right.Scope.Normalized().Namespace, string(right.Kind), string(right.SourceKind), right.ID, fmt.Sprint(right.SourceVersion), string(right.Polarity), fmt.Sprint(right.Corroborates)}, "\x00")
	})
	payload := struct {
		Scope                    Scope                             `json:"scope"`
		MemoryID                 string                            `json:"memory_id"`
		TargetVersion            int64                             `json:"target_version"`
		ClaimFingerprint         string                            `json:"claim_fingerprint"`
		ApplicabilityFingerprint string                            `json:"applicability_fingerprint"`
		Disposition              ProceduralVerificationDisposition `json:"disposition"`
		Evidence                 []ProceduralVerificationEvidence  `json:"evidence"`
		CorrectedContent         string                            `json:"corrected_content,omitempty"`
		PolicyVersion            string                            `json:"policy_version"`
	}{
		Scope:                    v.Scope.Normalized(),
		MemoryID:                 strings.TrimSpace(v.MemoryID),
		TargetVersion:            v.TargetVersion,
		ClaimFingerprint:         strings.ToLower(strings.TrimSpace(v.ClaimFingerprint)),
		ApplicabilityFingerprint: strings.ToLower(strings.TrimSpace(v.ApplicabilityFingerprint)),
		Disposition:              v.Disposition,
		Evidence:                 evidence,
		CorrectedContent:         strings.TrimSpace(v.CorrectedContent),
		PolicyVersion:            strings.TrimSpace(v.PolicyVersion),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal procedural verification fingerprint: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func boundedRequiredText(name, value string, max int) error {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	if len([]byte(value)) > max {
		return fmt.Errorf("%s exceeds %d bytes", name, max)
	}
	return nil
}

func sha256Hex(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
