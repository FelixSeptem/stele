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
	if len(l.Verifications) > 100 || len(l.Runs) > 100 {
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
	return nil
}

func (s ProceduralVerificationService) Record(ctx context.Context, record ProceduralMemoryVerification) (ProceduralMemoryVerification, error) {
	if err := record.Validate(); err != nil {
		return ProceduralMemoryVerification{}, err
	}
	if s.Store == nil || s.Targets == nil || s.Evidence == nil {
		return ProceduralMemoryVerification{}, fmt.Errorf("procedural verification service is not configured")
	}
	if err := s.Targets.ValidateProceduralVerificationTarget(ctx, record); err != nil {
		return ProceduralMemoryVerification{}, fmt.Errorf("validate procedural verification target: %w", err)
	}
	for index, evidence := range record.Evidence {
		if evidence.Scope == (Scope{}) {
			evidence.Scope = record.Scope.Normalized()
		}
		if err := s.Evidence.ValidateProceduralVerificationEvidence(ctx, record.Scope, evidence); err != nil {
			return ProceduralMemoryVerification{}, fmt.Errorf("validate procedural verification evidence %d: %w", index, err)
		}
	}
	stored, err := s.Store.AppendProceduralMemoryVerification(ctx, record)
	if err != nil {
		return ProceduralMemoryVerification{}, fmt.Errorf("append procedural verification: %w", err)
	}
	return stored, nil
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

type ClaimProceduralRevalidationRunsInput struct {
	Scope         Scope
	WorkerID      string
	Now           time.Time
	LeaseDuration time.Duration
	Limit         int
}

type CompleteProceduralRevalidationRunInput struct {
	Scope                  Scope
	RunID                  string
	WorkerID               string
	Status                 ProceduralRevalidationRunStatus
	TerminalVerificationID string
	CompletedAt            time.Time
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
