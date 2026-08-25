package assurance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/telemetry"
)

type AssuranceStore interface {
	CreateHealthEvaluation(ctx context.Context, evaluation HealthEvaluation) (HealthEvaluation, error)
	ListHealthEvaluations(ctx context.Context, scope memory.Scope) ([]HealthEvaluation, error)
	ReadHealthEvaluation(ctx context.Context, input ReadHealthEvaluationInput) (HealthEvaluation, error)
	ReadIncident(ctx context.Context, input ReadIncidentInput) (Incident, error)
	ListIncidents(ctx context.Context, input ListIncidentsInput) ([]Incident, error)
	CreateIncident(ctx context.Context, incident Incident) (Incident, error)
	TransitionIncident(ctx context.Context, transition IncidentTransition) (Incident, error)
	ListAlertCandidates(ctx context.Context, scope memory.Scope) ([]AlertCandidate, error)
	ReadAlertCandidate(ctx context.Context, input ReadAlertCandidateInput) (AlertCandidate, error)
	ListAlertDeliveryAttempts(ctx context.Context, input ListAlertDeliveryAttemptsInput) ([]AlertDeliveryAttempt, error)
	CreateAlertCandidate(ctx context.Context, candidate AlertCandidate) (AlertCandidate, error)
	CreateAlertDeliveryAttempt(ctx context.Context, attempt AlertDeliveryAttempt) (AlertDeliveryAttempt, error)
	ClaimAlertCandidatesForDelivery(ctx context.Context, input ClaimAlertCandidatesForDeliveryInput) ([]AlertDeliveryClaim, error)
	CreateConformanceProfile(ctx context.Context, profile ConformanceProfile) (ConformanceProfile, error)
	ReadConformanceProfile(ctx context.Context, input ReadConformanceProfileInput) (ConformanceProfile, error)
	ListConformanceProfiles(ctx context.Context, input ListConformanceProfilesInput) ([]ConformanceProfile, error)
	UpdateConformanceProfile(ctx context.Context, input UpdateConformanceProfileInput) (ConformanceProfile, error)
	DisableConformanceProfile(ctx context.Context, input DisableConformanceProfileInput) (ConformanceProfile, error)
	InspectConformanceEvidence(ctx context.Context, input ConformanceEvidenceInspectionInput) ([]ConformanceEvidenceObservation, error)
	CreateConformanceRun(ctx context.Context, run ConformanceRun) (ConformanceRun, error)
	ListConformanceRuns(ctx context.Context, input ListConformanceRunsInput) ([]ConformanceRun, error)
	ReadConformanceRun(ctx context.Context, input ReadConformanceRunInput) (ConformanceRun, error)
	CreateMissingEvidenceDiagnostic(ctx context.Context, diagnostic MissingEvidenceDiagnostic) (MissingEvidenceDiagnostic, error)
	ListOperationalProofs(ctx context.Context, scope memory.Scope) ([]OperationalProof, error)
	CreateReadinessReport(ctx context.Context, report ReadinessReport) (ReadinessReport, error)
	ListReadinessReports(ctx context.Context, scope memory.Scope) ([]ReadinessReport, error)
	ReadReadinessReport(ctx context.Context, input ReadReadinessReportInput) (ReadinessReport, error)
	CreateRecoveryVerification(ctx context.Context, verification RecoveryVerification) (RecoveryVerification, error)
	ListRecoveryVerifications(ctx context.Context, scope memory.Scope) ([]RecoveryVerification, error)
	ReadRecoveryVerification(ctx context.Context, input ReadRecoveryVerificationInput) (RecoveryVerification, error)
	CreateRetentionRun(ctx context.Context, run RetentionRun) (RetentionRun, error)
}

// WorkflowHealthReader exposes only scoped workflow health aggregates to assurance.
// It deliberately excludes workflow, evidence, and actor identifiers.
type WorkflowHealthReader interface {
	ReadWorkflowHealth(ctx context.Context, scope memory.Scope, observedAt time.Time) (WorkflowHealthSnapshot, error)
}

type WorkflowHealthSnapshot struct {
	Scope               memory.Scope
	CompletedRuns       int
	IncompleteRuns      int
	StaleRuns           int
	BlockingDiagnostics int
	LatestObservedAt    time.Time
	Status              HealthStatus
	Reason              ReasonCategory
}

func (s WorkflowHealthSnapshot) Validate() error {
	if err := s.Scope.Validate(); err != nil {
		return err
	}
	if s.CompletedRuns < 0 || s.IncompleteRuns < 0 || s.StaleRuns < 0 || s.BlockingDiagnostics < 0 {
		return fmt.Errorf("workflow health counts must not be negative")
	}
	if s.Status != "" && !s.Status.Valid() {
		return fmt.Errorf("workflow health status %q is invalid", s.Status)
	}
	if s.Reason != "" && !s.Reason.Valid() {
		return fmt.Errorf("workflow health reason %q is invalid", s.Reason)
	}
	return nil
}

type ServiceOptions struct {
	Store    AssuranceStore
	Workflow WorkflowHealthReader
	Now      func() time.Time
	NewID    func(prefix string) string
	Observer telemetry.Observer
	Logger   *log.Logger
}

type Service struct {
	store    AssuranceStore
	workflow WorkflowHealthReader
	now      func() time.Time
	newID    func(prefix string) string
	observer telemetry.Observer
	logger   *log.Logger
}

type HealthObservation struct {
	Status       HealthStatus
	Severity     Severity
	Reason       ReasonCategory
	Evidence     map[string]any
	ObservedAt   time.Time
	FreshThrough time.Time
}

type HealthEvaluationInput struct {
	Scope                 memory.Scope
	EvaluationID          string
	ObservedAt            time.Time
	RuntimeReadiness      HealthObservation
	BacklogState          HealthObservation
	EmbeddingHealth       HealthObservation
	ProofSessionVerdict   HealthObservation
	UsefulnessFeedback    HealthObservation
	TaskEvaluationSummary HealthObservation
	RepairStatus          HealthObservation
	RankingRolloutState   HealthObservation
	ConformanceStatus     HealthObservation
	WorkflowHealth        HealthObservation
	CapacityLoadProof     HealthObservation
	BackupRestoreProof    HealthObservation
}

type IncidentActionInput struct {
	Scope        memory.Scope
	IncidentID   string
	TransitionID string
	Action       IncidentAction
	Actor        string
	Reason       string
	OccurredAt   time.Time
}

type AlertCandidateGenerationInput struct {
	Scope               memory.Scope
	Evaluation          HealthEvaluation
	DeliveryPolicy      string
	DeduplicationWindow time.Duration
	CreatedAt           time.Time
}

type AlertDeliveryInput struct {
	Scope        memory.Scope
	Candidate    AlertCandidate
	Config       AlertDeliveryConfig
	MaxAttempts  int
	RetryBackoff time.Duration
	WorkerID     string
	Now          time.Time
	HTTPClient   *http.Client
	Output       io.Writer
}

type ConformanceRunInput struct {
	Scope     memory.Scope
	ProfileID string
	RunID     string
	StartedAt time.Time
}

type ReadinessReportInput struct {
	Scope       memory.Scope
	ReportID    string
	GeneratedAt time.Time
}

type RecoveryVerificationInput struct {
	Scope           memory.Scope
	RecordID        string
	Target          RecoveryVerificationTarget
	TargetID        string
	Status          HealthStatus
	CheckedSurfaces []string
	ResultCategory  string
	LinkedEvidence  map[string]any
	Actor           string
	Reason          string
	VerifiedAt      time.Time
}

func (i RecoveryVerificationInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.RecordID != "" {
		if err := validateID(i.RecordID, "recovery verification id"); err != nil {
			return err
		}
	}
	if !i.Target.Valid() {
		return fmt.Errorf("recovery verification target %q is invalid", i.Target)
	}
	if err := validateID(i.TargetID, "target id"); err != nil {
		return err
	}
	if !i.Status.Valid() {
		return fmt.Errorf("health status %q is invalid", i.Status)
	}
	if strings.TrimSpace(i.ResultCategory) == "" {
		return fmt.Errorf("result category is required")
	}
	if err := validateActorReason(i.Actor, i.Reason); err != nil {
		return err
	}
	if err := validateMetadata(i.LinkedEvidence, "linked evidence"); err != nil {
		return err
	}
	return nil
}

func (i ReadinessReportInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.ReportID != "" {
		if err := validateID(i.ReportID, "readiness report id"); err != nil {
			return err
		}
	}
	if i.GeneratedAt.IsZero() {
		return fmt.Errorf("generated at is required")
	}
	return nil
}

func (i ConformanceRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := validateID(i.ProfileID, "conformance profile id"); err != nil {
		return err
	}
	if i.RunID != "" {
		if err := validateID(i.RunID, "conformance run id"); err != nil {
			return err
		}
	}
	if i.StartedAt.IsZero() {
		return fmt.Errorf("started at is required")
	}
	return nil
}

type ConformanceEvidenceInspectionInput struct {
	Scope            memory.Scope
	ExpectedEvidence []ExpectedEvidence
	ObservedAt       time.Time
}

func (i ConformanceEvidenceInspectionInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.ObservedAt.IsZero() {
		return fmt.Errorf("observed at is required")
	}
	for _, evidence := range i.ExpectedEvidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type ConformanceEvidenceObservation struct {
	Kind          ExpectedEvidenceKind
	Count         int
	FreshestAt    time.Time
	Stale         bool
	Contradictory bool
	OpaqueOnly    bool
	Hidden        bool
	OutOfScope    bool
}

type UpdateConformanceProfileInput struct {
	Scope            memory.Scope
	ProfileID        string
	ExpectedEvidence []ExpectedEvidence
	Actor            string
	Reason           string
	UpdatedAt        time.Time
}

func (i UpdateConformanceProfileInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := validateID(i.ProfileID, "conformance profile id"); err != nil {
		return err
	}
	if len(i.ExpectedEvidence) == 0 {
		return fmt.Errorf("expected evidence is required")
	}
	for _, evidence := range i.ExpectedEvidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
	}
	if err := validateActorReason(i.Actor, i.Reason); err != nil {
		return err
	}
	if i.UpdatedAt.IsZero() {
		return fmt.Errorf("updated at is required")
	}
	return nil
}

type DisableConformanceProfileInput struct {
	Scope      memory.Scope
	ProfileID  string
	Actor      string
	Reason     string
	DisabledAt time.Time
}

func (i DisableConformanceProfileInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := validateID(i.ProfileID, "conformance profile id"); err != nil {
		return err
	}
	if err := validateActorReason(i.Actor, i.Reason); err != nil {
		return err
	}
	if i.DisabledAt.IsZero() {
		return fmt.Errorf("disabled at is required")
	}
	return nil
}

func (i AlertCandidateGenerationInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.DeliveryPolicy) == "" {
		return fmt.Errorf("delivery policy is required")
	}
	if i.DeduplicationWindow < 0 {
		return fmt.Errorf("deduplication window must be greater than or equal to zero")
	}
	if i.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	if i.Evaluation.ID != "" && i.Evaluation.Scope != (memory.Scope{}) && i.Evaluation.Scope.Normalized() != i.Scope.Normalized() {
		return fmt.Errorf("health evaluation scope does not match alert generation scope")
	}
	return nil
}

func (i AlertDeliveryInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := i.Candidate.Validate(); err != nil {
		return err
	}
	if i.Candidate.Scope.Normalized() != i.Scope.Normalized() {
		return fmt.Errorf("alert candidate scope does not match delivery scope")
	}
	if strings.TrimSpace(i.WorkerID) == "" {
		return fmt.Errorf("worker id is required")
	}
	if i.MaxAttempts <= 0 {
		return fmt.Errorf("max attempts must be greater than zero")
	}
	if i.RetryBackoff < 0 {
		return fmt.Errorf("retry backoff must be greater than or equal to zero")
	}
	return i.Config.Validate()
}

func (i IncidentActionInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if err := validateID(i.IncidentID, "incident id"); err != nil {
		return err
	}
	if i.TransitionID != "" {
		if err := validateID(i.TransitionID, "incident transition id"); err != nil {
			return err
		}
	}
	if !i.Action.Valid() {
		return fmt.Errorf("incident action %q is invalid", i.Action)
	}
	if err := validateActorReason(i.Actor, i.Reason); err != nil {
		return err
	}
	if i.OccurredAt.IsZero() {
		return fmt.Errorf("occurred at is required")
	}
	return nil
}

type componentSignal struct {
	component HealthComponent
	source    string
	obs       HealthObservation
}

func NewService(options ServiceOptions) *Service {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	newID := options.NewID
	if newID == nil {
		newID = func(prefix string) string {
			return fmt.Sprintf("%s_%d", strings.TrimSpace(prefix), now().UnixNano())
		}
	}
	return &Service{store: options.Store, workflow: options.Workflow, now: now, newID: newID, observer: options.Observer, logger: options.Logger}
}

func (s *Service) CreateHealthEvaluation(ctx context.Context, input HealthEvaluationInput) (HealthEvaluation, error) {
	if err := input.Scope.Validate(); err != nil {
		return HealthEvaluation{}, err
	}
	if s.store == nil {
		return HealthEvaluation{}, fmt.Errorf("assurance store is not configured")
	}

	observedAt := input.ObservedAt
	if observedAt.IsZero() {
		observedAt = s.now().UTC()
	}
	evaluationID := strings.TrimSpace(input.EvaluationID)
	if evaluationID == "" {
		evaluationID = s.newID("health_evaluation")
	}

	signals := []componentSignal{
		{component: ComponentRuntime, source: "runtime_readiness", obs: input.RuntimeReadiness},
		{component: ComponentBacklog, source: "backlog_state", obs: input.BacklogState},
		{component: ComponentDependency, source: "embedding_health", obs: input.EmbeddingHealth},
		{component: ComponentProof, source: "proof_session_verdict", obs: input.ProofSessionVerdict},
		{component: ComponentFeedback, source: "usefulness_feedback", obs: input.UsefulnessFeedback},
		{component: ComponentTask, source: "task_evaluation_summary", obs: input.TaskEvaluationSummary},
		{component: ComponentRepair, source: "repair_status", obs: input.RepairStatus},
		{component: ComponentRanking, source: "ranking_rollout_state", obs: input.RankingRolloutState},
		{component: ComponentConformance, source: "conformance_status", obs: input.ConformanceStatus},
		{component: ComponentWorkflow, source: "workflow_health", obs: input.WorkflowHealth},
		{component: ComponentCapacityLoad, source: "capacity_load_proof", obs: input.CapacityLoadProof},
		{component: ComponentBackupRestore, source: "backup_restore_proof", obs: input.BackupRestoreProof},
	}

	evaluation := HealthEvaluation{
		ID:        evaluationID,
		Scope:     input.Scope.Normalized(),
		Severity:  SeverityInfo,
		Reason:    ReasonUnknown,
		CreatedAt: observedAt,
	}
	for _, signal := range signals {
		if isZeroHealthObservation(signal.obs) {
			continue
		}
		component := buildHealthComponentSummary(input.Scope.Normalized(), evaluationID, signal, observedAt)
		if component.Status == "" {
			continue
		}
		evaluation.Components = append(evaluation.Components, component)
		evaluation.Status, evaluation.Severity, evaluation.Reason = aggregateHealthEvaluation(evaluation.Status, evaluation.Severity, evaluation.Reason, component)
	}
	if len(evaluation.Components) == 0 {
		evaluation.Status = HealthStatusUnknown
		evaluation.Severity = SeverityInfo
		evaluation.Reason = ReasonUnknown
	}

	created, err := s.store.CreateHealthEvaluation(ctx, evaluation)
	if err != nil {
		return HealthEvaluation{}, err
	}
	for _, component := range created.Components {
		s.recordHealthEvaluation(ctx, "create", "ok", component)
		s.recordOperationalProof(ctx, component)
	}

	for _, component := range created.Components {
		if !shouldCreateIncident(component) {
			continue
		}
		if _, err := s.ensureIncident(ctx, created, component, observedAt); err != nil {
			return HealthEvaluation{}, err
		}
	}
	return created, nil
}

func (s *Service) ListHealthEvaluations(ctx context.Context, scope memory.Scope) ([]HealthEvaluation, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("assurance store is not configured")
	}
	return s.store.ListHealthEvaluations(ctx, scope.Normalized())
}

func (s *Service) ReadHealthEvaluation(ctx context.Context, input ReadHealthEvaluationInput) (HealthEvaluation, error) {
	if err := input.Validate(); err != nil {
		return HealthEvaluation{}, err
	}
	if s.store == nil {
		return HealthEvaluation{}, fmt.Errorf("assurance store is not configured")
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ReadHealthEvaluation(ctx, input)
}

func (s *Service) ApplyIncidentAction(ctx context.Context, input IncidentActionInput) (Incident, error) {
	if err := input.Validate(); err != nil {
		return Incident{}, err
	}
	if s.store == nil {
		return Incident{}, fmt.Errorf("assurance store is not configured")
	}
	incident, err := s.store.ReadIncident(ctx, ReadIncidentInput{
		Scope:      input.Scope.Normalized(),
		IncidentID: input.IncidentID,
	})
	if err != nil {
		return Incident{}, err
	}
	toStatus := incidentActionTargetStatus(input.Action, incident.Status)
	transitionID := strings.TrimSpace(input.TransitionID)
	if transitionID == "" {
		transitionID = s.newID("incident_transition")
	}
	updated, err := s.store.TransitionIncident(ctx, IncidentTransition{
		ID:         transitionID,
		IncidentID: input.IncidentID,
		Scope:      input.Scope.Normalized(),
		FromStatus: incident.Status,
		ToStatus:   toStatus,
		Action:     input.Action,
		Actor:      strings.TrimSpace(input.Actor),
		Reason:     strings.TrimSpace(input.Reason),
		OccurredAt: input.OccurredAt.UTC(),
	})
	if err != nil {
		return Incident{}, err
	}
	s.recordIncidentLifecycle(ctx, string(input.Action), "ok", updated)
	return updated, nil
}

func (s *Service) ListIncidents(ctx context.Context, input ListIncidentsInput) ([]Incident, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("assurance store is not configured")
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ListIncidents(ctx, input)
}

func (s *Service) ReadIncident(ctx context.Context, input ReadIncidentInput) (Incident, error) {
	if err := input.Validate(); err != nil {
		return Incident{}, err
	}
	if s.store == nil {
		return Incident{}, fmt.Errorf("assurance store is not configured")
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ReadIncident(ctx, input)
}

func (s *Service) GenerateAlertCandidates(ctx context.Context, input AlertCandidateGenerationInput) ([]AlertCandidate, error) {
	if input.CreatedAt.IsZero() {
		input.CreatedAt = s.now().UTC()
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("assurance store is not configured")
	}

	scope := input.Scope.Normalized()
	existing, err := s.store.ListAlertCandidates(ctx, scope)
	if err != nil {
		return nil, err
	}
	existingByKey := make(map[string]AlertCandidate, len(existing))
	for _, candidate := range existing {
		existingByKey[candidate.DeduplicationKey] = candidate
	}

	created := make([]AlertCandidate, 0)
	incidents, err := s.store.ListIncidents(ctx, ListIncidentsInput{Scope: scope})
	if err != nil {
		return nil, err
	}
	for _, incident := range incidents {
		if !incidentAlertEligible(incident) {
			continue
		}
		candidate := alertCandidateFromIncident(input, incident)
		if alertCandidateDeduped(candidate, existingByKey, input.CreatedAt, input.DeduplicationWindow) {
			continue
		}
		stored, err := s.store.CreateAlertCandidate(ctx, candidate)
		if err != nil {
			return nil, err
		}
		created = append(created, stored)
		existingByKey[stored.DeduplicationKey] = stored
		s.recordAlertCandidate(ctx, "generate", "ok", stored)
	}

	if input.Evaluation.ID != "" {
		for _, component := range input.Evaluation.Components {
			if !componentAlertEligible(component) {
				continue
			}
			candidate := alertCandidateFromComponent(input, component)
			if alertCandidateDeduped(candidate, existingByKey, input.CreatedAt, input.DeduplicationWindow) {
				continue
			}
			stored, err := s.store.CreateAlertCandidate(ctx, candidate)
			if err != nil {
				return nil, err
			}
			created = append(created, stored)
			existingByKey[stored.DeduplicationKey] = stored
			s.recordAlertCandidate(ctx, "generate", "ok", stored)
		}
	}
	return created, nil
}

func (s *Service) ListAlertCandidates(ctx context.Context, scope memory.Scope) ([]AlertCandidate, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("assurance store is not configured")
	}
	return s.store.ListAlertCandidates(ctx, scope.Normalized())
}

func (s *Service) ReadAlertCandidate(ctx context.Context, input ReadAlertCandidateInput) (AlertCandidate, error) {
	if err := input.Validate(); err != nil {
		return AlertCandidate{}, err
	}
	if s.store == nil {
		return AlertCandidate{}, fmt.Errorf("assurance store is not configured")
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ReadAlertCandidate(ctx, input)
}

func (s *Service) ListAlertDeliveryAttempts(ctx context.Context, input ListAlertDeliveryAttemptsInput) ([]AlertDeliveryAttempt, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("assurance store is not configured")
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ListAlertDeliveryAttempts(ctx, input)
}

func (s *Service) ClaimAlertCandidatesForDelivery(ctx context.Context, input ClaimAlertCandidatesForDeliveryInput) ([]AlertDeliveryClaim, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("assurance store is not configured")
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ClaimAlertCandidatesForDelivery(ctx, input)
}

func (s *Service) DeliverAlertCandidate(ctx context.Context, input AlertDeliveryInput) ([]AlertDeliveryAttempt, error) {
	if input.Now.IsZero() {
		input.Now = s.now().UTC()
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("assurance store is not configured")
	}

	now := input.Now.UTC()
	payloadBytes, payloadHash, err := boundedAlertPayloadBytes(input.Candidate.Payload, input.Config.MaxPayloadBytes)
	if err != nil {
		attempt, recordErr := s.recordAlertDeliveryAttempt(ctx, input, AlertDeliveryAttempt{
			ID:               s.newID("alert_delivery_attempt"),
			AlertCandidateID: input.Candidate.ID,
			Scope:            input.Scope.Normalized(),
			Adapter:          input.Config.Mode,
			Result:           AlertDeliveryResultFailed,
			FailureCategory:  "payload_too_large",
			Attempt:          1,
			WorkerID:         input.WorkerID,
			PayloadHash:      payloadHash,
			AttemptedAt:      now,
			CompletedAt:      now,
		})
		if recordErr != nil {
			return nil, recordErr
		}
		return []AlertDeliveryAttempt{attempt}, err
	}

	attempts := make([]AlertDeliveryAttempt, 0, input.MaxAttempts)
	switch input.Config.Mode {
	case AlertAdapterDisabled:
		attempt, err := s.recordAlertDeliveryAttempt(ctx, input, AlertDeliveryAttempt{
			ID:               s.newID("alert_delivery_attempt"),
			AlertCandidateID: input.Candidate.ID,
			Scope:            input.Scope.Normalized(),
			Adapter:          input.Config.Mode,
			Result:           AlertDeliveryResultDisabled,
			Attempt:          1,
			WorkerID:         input.WorkerID,
			PayloadHash:      payloadHash,
			AttemptedAt:      now,
			CompletedAt:      now,
		})
		if err != nil {
			return nil, err
		}
		return []AlertDeliveryAttempt{attempt}, nil
	case AlertAdapterStdout:
		out := input.Output
		if out == nil {
			out = os.Stdout
		}
		if _, err := fmt.Fprintln(out, string(payloadBytes)); err != nil {
			attempt, recordErr := s.recordAlertDeliveryAttempt(ctx, input, AlertDeliveryAttempt{
				ID:               s.newID("alert_delivery_attempt"),
				AlertCandidateID: input.Candidate.ID,
				Scope:            input.Scope.Normalized(),
				Adapter:          input.Config.Mode,
				Result:           AlertDeliveryResultFailed,
				FailureCategory:  "stdout_write_failed",
				Attempt:          1,
				WorkerID:         input.WorkerID,
				PayloadHash:      payloadHash,
				AttemptedAt:      now,
				CompletedAt:      now,
			})
			if recordErr != nil {
				return nil, recordErr
			}
			return []AlertDeliveryAttempt{attempt}, err
		}
		attempt, err := s.recordAlertDeliveryAttempt(ctx, input, AlertDeliveryAttempt{
			ID:               s.newID("alert_delivery_attempt"),
			AlertCandidateID: input.Candidate.ID,
			Scope:            input.Scope.Normalized(),
			Adapter:          input.Config.Mode,
			Result:           AlertDeliveryResultSuccess,
			Attempt:          1,
			WorkerID:         input.WorkerID,
			PayloadHash:      payloadHash,
			AttemptedAt:      now,
			CompletedAt:      now,
		})
		if err != nil {
			return nil, err
		}
		return []AlertDeliveryAttempt{attempt}, nil
	case AlertAdapterWebhook:
		client := input.HTTPClient
		if client == nil {
			client = &http.Client{Timeout: input.Config.Timeout}
		} else if client.Timeout <= 0 {
			client.Timeout = input.Config.Timeout
		}
		url := strings.TrimSpace(input.Config.WebhookURL)
		for attemptNumber := 1; attemptNumber <= input.MaxAttempts; attemptNumber++ {
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
			if reqErr != nil {
				attempt, recordErr := s.recordAlertDeliveryAttempt(ctx, input, AlertDeliveryAttempt{
					ID:               s.newID("alert_delivery_attempt"),
					AlertCandidateID: input.Candidate.ID,
					Scope:            input.Scope.Normalized(),
					Adapter:          input.Config.Mode,
					Result:           AlertDeliveryResultFailed,
					FailureCategory:  "request_build_failed",
					Attempt:          attemptNumber,
					WorkerID:         input.WorkerID,
					PayloadHash:      payloadHash,
					AttemptedAt:      now,
					CompletedAt:      now,
				})
				if recordErr != nil {
					return nil, recordErr
				}
				return []AlertDeliveryAttempt{attempt}, reqErr
			}
			req.Header.Set("Content-Type", "application/json")
			for key, value := range input.Config.WebhookHeaders {
				req.Header.Set(key, value)
			}
			resp, err := client.Do(req)
			if err == nil && resp != nil && resp.Body != nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
			success := err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300
			result := AlertDeliveryResultSuccess
			failureCategory := ""
			if !success {
				result = AlertDeliveryResultRetry
				failureCategory = alertDeliveryFailureCategory(err, resp)
				if attemptNumber == input.MaxAttempts {
					result = AlertDeliveryResultFailed
				}
			}
			attempt, recordErr := s.recordAlertDeliveryAttempt(ctx, input, AlertDeliveryAttempt{
				ID:               s.newID("alert_delivery_attempt"),
				AlertCandidateID: input.Candidate.ID,
				Scope:            input.Scope.Normalized(),
				Adapter:          input.Config.Mode,
				Result:           result,
				FailureCategory:  failureCategory,
				Attempt:          attemptNumber,
				WorkerID:         input.WorkerID,
				PayloadHash:      payloadHash,
				AttemptedAt:      now,
				CompletedAt:      now,
			})
			if recordErr != nil {
				return nil, recordErr
			}
			attempts = append(attempts, attempt)
			if success || attemptNumber == input.MaxAttempts {
				return attempts, err
			}
			attempts[len(attempts)-1].Result = AlertDeliveryResultRetry
		}
	}
	return attempts, nil
}

func (s *Service) CreateConformanceProfile(ctx context.Context, profile ConformanceProfile) (ConformanceProfile, error) {
	if err := profile.Validate(); err != nil {
		return ConformanceProfile{}, err
	}
	if s.store == nil {
		return ConformanceProfile{}, fmt.Errorf("assurance store is not configured")
	}
	return s.store.CreateConformanceProfile(ctx, profile)
}

func (s *Service) UpdateConformanceProfile(ctx context.Context, input UpdateConformanceProfileInput) (ConformanceProfile, error) {
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = s.now().UTC()
	}
	if err := input.Validate(); err != nil {
		return ConformanceProfile{}, err
	}
	if s.store == nil {
		return ConformanceProfile{}, fmt.Errorf("assurance store is not configured")
	}
	return s.store.UpdateConformanceProfile(ctx, input)
}

func (s *Service) DisableConformanceProfile(ctx context.Context, input DisableConformanceProfileInput) (ConformanceProfile, error) {
	if input.DisabledAt.IsZero() {
		input.DisabledAt = s.now().UTC()
	}
	if err := input.Validate(); err != nil {
		return ConformanceProfile{}, err
	}
	if s.store == nil {
		return ConformanceProfile{}, fmt.Errorf("assurance store is not configured")
	}
	return s.store.DisableConformanceProfile(ctx, input)
}

func (s *Service) ReadConformanceProfile(ctx context.Context, input ReadConformanceProfileInput) (ConformanceProfile, error) {
	if err := input.Validate(); err != nil {
		return ConformanceProfile{}, err
	}
	if s.store == nil {
		return ConformanceProfile{}, fmt.Errorf("assurance store is not configured")
	}
	return s.store.ReadConformanceProfile(ctx, input)
}

func (s *Service) ListConformanceProfiles(ctx context.Context, input ListConformanceProfilesInput) ([]ConformanceProfile, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("assurance store is not configured")
	}
	return s.store.ListConformanceProfiles(ctx, input)
}

func (s *Service) RunConformance(ctx context.Context, input ConformanceRunInput) (ConformanceRun, []MissingEvidenceDiagnostic, error) {
	if input.StartedAt.IsZero() {
		input.StartedAt = s.now().UTC()
	}
	if err := input.Validate(); err != nil {
		return ConformanceRun{}, nil, err
	}
	if s.store == nil {
		return ConformanceRun{}, nil, fmt.Errorf("assurance store is not configured")
	}

	scope := input.Scope.Normalized()
	profile, err := s.store.ReadConformanceProfile(ctx, ReadConformanceProfileInput{
		Scope:     scope,
		ProfileID: input.ProfileID,
	})
	if err != nil {
		return ConformanceRun{}, nil, err
	}
	if profile.Scope.Normalized() != scope {
		return ConformanceRun{}, nil, fmt.Errorf("conformance profile scope does not match run scope")
	}
	if profile.Status != ConformanceProfileStatusActive {
		return ConformanceRun{}, nil, fmt.Errorf("conformance profile %q is not active", profile.ID)
	}

	observations, err := s.store.InspectConformanceEvidence(ctx, ConformanceEvidenceInspectionInput{
		Scope:            scope,
		ExpectedEvidence: append([]ExpectedEvidence(nil), profile.ExpectedEvidence...),
		ObservedAt:       input.StartedAt.UTC(),
	})
	if err != nil {
		return ConformanceRun{}, nil, err
	}
	observedByKind := make(map[ExpectedEvidenceKind]ConformanceEvidenceObservation, len(observations))
	for _, observation := range observations {
		observedByKind[observation.Kind] = observation
	}

	runID := strings.TrimSpace(input.RunID)
	if runID == "" {
		runID = s.newID("conformance_run")
	}
	evidenceCounts := make(map[string]any, len(profile.ExpectedEvidence))
	diagnostics := make([]MissingEvidenceDiagnostic, 0)
	result := ConformanceResultPassed
	for _, expected := range profile.ExpectedEvidence {
		observation := observedByKind[expected.Kind]
		observation.Kind = expected.Kind
		if observation.Count < expected.MinimumCount {
			diagnostics = append(diagnostics, s.conformanceDiagnostic(runID, scope, expected, observation, missingEvidenceCategoryForKind(expected.Kind), input.StartedAt))
		}
		if observation.Count > 0 && observation.FreshestAt.Before(input.StartedAt.UTC().Add(-expected.FreshnessWindow)) {
			observation.Stale = true
		}
		if observation.Stale {
			diagnostics = append(diagnostics, s.conformanceDiagnostic(runID, scope, expected, observation, MissingEvidenceStale, input.StartedAt))
		}
		if observation.Contradictory {
			diagnostics = append(diagnostics, s.conformanceDiagnostic(runID, scope, expected, observation, MissingEvidenceContradictory, input.StartedAt))
		}
		if observation.OpaqueOnly {
			diagnostics = append(diagnostics, s.conformanceDiagnostic(runID, scope, expected, observation, MissingEvidenceOpaqueOnly, input.StartedAt))
		}
		if observation.Hidden {
			diagnostics = append(diagnostics, s.conformanceDiagnostic(runID, scope, expected, observation, MissingEvidenceHidden, input.StartedAt))
		}
		if observation.OutOfScope {
			diagnostics = append(diagnostics, s.conformanceDiagnostic(runID, scope, expected, observation, MissingEvidenceOutOfScope, input.StartedAt))
		}
		evidenceCounts[string(expected.Kind)] = map[string]any{
			"count":         observation.Count,
			"minimum_count": expected.MinimumCount,
			"freshness_ms":  expected.FreshnessWindow.Milliseconds(),
			"freshest_at":   formatOptionalTime(observation.FreshestAt),
		}
	}
	if len(diagnostics) > 0 {
		result = ConformanceResultDegraded
	}

	run, err := s.store.CreateConformanceRun(ctx, ConformanceRun{
		ID:             runID,
		ProfileID:      profile.ID,
		Scope:          scope,
		Result:         result,
		EvidenceCounts: evidenceCounts,
		StartedAt:      input.StartedAt.UTC(),
		FinishedAt:     input.StartedAt.UTC(),
		CreatedAt:      input.StartedAt.UTC(),
	})
	if err != nil {
		return ConformanceRun{}, nil, err
	}
	createdDiagnostics := make([]MissingEvidenceDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		created, err := s.store.CreateMissingEvidenceDiagnostic(ctx, diagnostic)
		if err != nil {
			return ConformanceRun{}, nil, err
		}
		createdDiagnostics = append(createdDiagnostics, created)
		s.recordMissingEvidenceDiagnostic(ctx, created)
	}
	s.recordConformanceRun(ctx, run, profile.Status, createdDiagnostics)
	return run, createdDiagnostics, nil
}

func (s *Service) ListConformanceRuns(ctx context.Context, input ListConformanceRunsInput) ([]ConformanceRun, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("assurance store is not configured")
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ListConformanceRuns(ctx, input)
}

func (s *Service) ReadConformanceRun(ctx context.Context, input ReadConformanceRunInput) (ConformanceRun, error) {
	if err := input.Validate(); err != nil {
		return ConformanceRun{}, err
	}
	if s.store == nil {
		return ConformanceRun{}, fmt.Errorf("assurance store is not configured")
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ReadConformanceRun(ctx, input)
}

func (s *Service) CreateReadinessReport(ctx context.Context, input ReadinessReportInput) (ReadinessReport, error) {
	if input.GeneratedAt.IsZero() {
		input.GeneratedAt = s.now().UTC()
	}
	if err := input.Validate(); err != nil {
		return ReadinessReport{}, err
	}
	if s.store == nil {
		return ReadinessReport{}, fmt.Errorf("assurance store is not configured")
	}
	scope := input.Scope.Normalized()
	healthEvaluations, err := s.store.ListHealthEvaluations(ctx, scope)
	if err != nil {
		return ReadinessReport{}, err
	}
	conformanceRuns, err := s.store.ListConformanceRuns(ctx, ListConformanceRunsInput{Scope: scope})
	if err != nil {
		return ReadinessReport{}, err
	}
	proofs, err := s.store.ListOperationalProofs(ctx, scope)
	if err != nil {
		return ReadinessReport{}, err
	}
	incidents, err := s.store.ListIncidents(ctx, ListIncidentsInput{Scope: scope})
	if err != nil {
		return ReadinessReport{}, err
	}
	alerts, err := s.store.ListAlertCandidates(ctx, scope)
	if err != nil {
		return ReadinessReport{}, err
	}
	workflowHealth := WorkflowHealthSnapshot{Scope: scope, Status: HealthStatusUnknown, Reason: ReasonWorkflowGap}
	if s.workflow != nil {
		workflowHealth, err = s.workflow.ReadWorkflowHealth(ctx, scope, input.GeneratedAt.UTC())
		if err != nil {
			return ReadinessReport{}, err
		}
		if err := workflowHealth.Validate(); err != nil {
			return ReadinessReport{}, err
		}
		if workflowHealth.Scope.Normalized() != scope {
			return ReadinessReport{}, fmt.Errorf("workflow health scope does not match readiness report scope")
		}
	}

	latestHealth := latestHealthEvaluation(healthEvaluations)
	latestConformance := latestConformanceRun(conformanceRuns)
	capacityProof, backupProof := latestOperationalProofs(proofs)
	activeIncidentCount := countActiveIncidents(incidents)
	alertCount := len(alerts)
	status := readinessStatus(latestHealth, latestConformance, capacityProof, backupProof)
	if workflowHealth.Status == HealthStatusUnhealthy {
		status = ReadinessStatusBlocked
	} else if workflowHealth.Status == HealthStatusDegraded || workflowHealth.Status == HealthStatusStale {
		if status != ReadinessStatusBlocked {
			status = ReadinessStatusDegraded
		}
	} else if workflowHealth.Status == HealthStatusUnknown && status == ReadinessStatusReady {
		status = ReadinessStatusUnknown
	}
	actions := readinessRecommendedActions(status, latestConformance, capacityProof, backupProof, activeIncidentCount, alertCount)
	if workflowHealth.Status != HealthStatusHealthy {
		actions = appendUniqueRunbookHint(actions, RunbookHintReviewWorkflow)
	}
	reportID := strings.TrimSpace(input.ReportID)
	if reportID == "" {
		reportID = s.newID("readiness_report")
	}
	report := ReadinessReport{
		ID:                 reportID,
		Scope:              scope,
		Status:             status,
		HealthEvaluationID: latestHealth.ID,
		ConformanceRunID:   latestConformance.ID,
		ComponentSummary: map[string]any{
			"health_status":                 string(latestHealth.Status),
			"conformance_status":            string(latestConformance.Result),
			"capacity_load_status":          string(capacityProof.Status),
			"backup_restore_status":         string(backupProof.Status),
			"active_incidents":              activeIncidentCount,
			"alert_candidates":              alertCount,
			"repair_health":                 string(componentStatus(latestHealth, ComponentRepair)),
			"ranking_rollout_health":        string(componentStatus(latestHealth, ComponentRanking)),
			"proof_session_health":          string(componentStatus(latestHealth, ComponentProof)),
			"workflow_status":               string(workflowHealth.Status),
			"workflow_completed_runs":       workflowHealth.CompletedRuns,
			"workflow_incomplete_runs":      workflowHealth.IncompleteRuns,
			"workflow_stale_runs":           workflowHealth.StaleRuns,
			"workflow_blocking_diagnostics": workflowHealth.BlockingDiagnostics,
		},
		RecommendedActions: actions,
		GeneratedAt:        input.GeneratedAt.UTC(),
		CreatedAt:          input.GeneratedAt.UTC(),
	}
	created, err := s.store.CreateReadinessReport(ctx, report)
	if err != nil {
		return ReadinessReport{}, err
	}
	s.recordReadinessReport(ctx, created, latestHealth, latestConformance, activeIncidentCount)
	return created, nil
}

func (s *Service) ListReadinessReports(ctx context.Context, scope memory.Scope) ([]ReadinessReport, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("assurance store is not configured")
	}
	return s.store.ListReadinessReports(ctx, scope.Normalized())
}

func (s *Service) ReadReadinessReport(ctx context.Context, input ReadReadinessReportInput) (ReadinessReport, error) {
	if err := input.Validate(); err != nil {
		return ReadinessReport{}, err
	}
	if s.store == nil {
		return ReadinessReport{}, fmt.Errorf("assurance store is not configured")
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ReadReadinessReport(ctx, input)
}

func (s *Service) CreateRecoveryVerification(ctx context.Context, input RecoveryVerificationInput) (RecoveryVerification, error) {
	if err := input.Validate(); err != nil {
		return RecoveryVerification{}, err
	}
	if s.store == nil {
		return RecoveryVerification{}, fmt.Errorf("assurance store is not configured")
	}
	now := s.now().UTC()
	recordID := strings.TrimSpace(input.RecordID)
	if recordID == "" {
		recordID = s.newID("recovery_verification")
	}
	verifiedAt := input.VerifiedAt
	if verifiedAt.IsZero() && input.Status == HealthStatusHealthy {
		verifiedAt = now
	}
	created, err := s.store.CreateRecoveryVerification(ctx, RecoveryVerification{
		ID:              recordID,
		Scope:           input.Scope.Normalized(),
		Target:          input.Target,
		TargetID:        input.TargetID,
		Status:          input.Status,
		CheckedSurfaces: append([]string(nil), input.CheckedSurfaces...),
		ResultCategory:  strings.TrimSpace(input.ResultCategory),
		LinkedEvidence:  cloneAnyMap(input.LinkedEvidence),
		Actor:           strings.TrimSpace(input.Actor),
		Reason:          strings.TrimSpace(input.Reason),
		CreatedAt:       now,
		VerifiedAt:      verifiedAt.UTC(),
	})
	if err != nil {
		return RecoveryVerification{}, err
	}
	s.recordRecoveryVerification(ctx, created)
	return created, nil
}

func (s *Service) ListRecoveryVerifications(ctx context.Context, scope memory.Scope) ([]RecoveryVerification, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("assurance store is not configured")
	}
	return s.store.ListRecoveryVerifications(ctx, scope.Normalized())
}

func (s *Service) ReadRecoveryVerification(ctx context.Context, input ReadRecoveryVerificationInput) (RecoveryVerification, error) {
	if err := input.Validate(); err != nil {
		return RecoveryVerification{}, err
	}
	if s.store == nil {
		return RecoveryVerification{}, fmt.Errorf("assurance store is not configured")
	}
	input.Scope = input.Scope.Normalized()
	return s.store.ReadRecoveryVerification(ctx, input)
}

func (s *Service) CreateRetentionRun(ctx context.Context, run RetentionRun) (RetentionRun, error) {
	if run.Scope != (memory.Scope{}) {
		run.Scope = run.Scope.Normalized()
	}
	if err := run.Validate(); err != nil {
		return RetentionRun{}, err
	}
	if s.store == nil {
		return RetentionRun{}, fmt.Errorf("assurance store is not configured")
	}
	created, err := s.store.CreateRetentionRun(ctx, run)
	if err != nil {
		return RetentionRun{}, err
	}
	s.recordAssuranceCleanup(ctx, created)
	return created, nil
}

func (s *Service) conformanceDiagnostic(runID string, scope memory.Scope, expected ExpectedEvidence, observation ConformanceEvidenceObservation, category MissingEvidenceCategory, createdAt time.Time) MissingEvidenceDiagnostic {
	return MissingEvidenceDiagnostic{
		ID:               s.newID("missing_evidence_diagnostic"),
		ConformanceRunID: runID,
		Scope:            scope.Normalized(),
		EvidenceKind:     expected.Kind,
		Category:         category,
		ReadinessImpact:  ReadinessStatusDegraded,
		Metadata: map[string]any{
			"count":         observation.Count,
			"minimum_count": expected.MinimumCount,
			"freshness_ms":  expected.FreshnessWindow.Milliseconds(),
			"freshest_at":   formatOptionalTime(observation.FreshestAt),
		},
		CreatedAt: createdAt.UTC(),
	}
}

func (s *Service) recordAlertDeliveryAttempt(ctx context.Context, input AlertDeliveryInput, attempt AlertDeliveryAttempt) (AlertDeliveryAttempt, error) {
	if attempt.ID == "" {
		attempt.ID = s.newID("alert_delivery_attempt")
	}
	if attempt.Scope == (memory.Scope{}) {
		attempt.Scope = input.Scope.Normalized()
	}
	if attempt.AttemptedAt.IsZero() {
		attempt.AttemptedAt = input.Now.UTC()
	}
	if attempt.CompletedAt.IsZero() {
		attempt.CompletedAt = attempt.AttemptedAt
	}
	recorded, err := s.store.CreateAlertDeliveryAttempt(ctx, attempt)
	if err != nil {
		return AlertDeliveryAttempt{}, err
	}
	s.recordAlertDelivery(ctx, input.Candidate, recorded)
	return recorded, nil
}

func (s *Service) recordHealthEvaluation(ctx context.Context, operation, result string, component HealthComponentSummary) {
	s.logAssuranceLifecycle(operation, result, string(component.Status), string(component.Component), string(component.Severity), string(component.Reason), "", "")
	observer, ok := s.observer.(interface {
		RecordAssuranceHealthEvaluation(context.Context, telemetry.AssuranceHealthEvaluationEvent)
	})
	if !ok {
		return
	}
	observer.RecordAssuranceHealthEvaluation(ctx, telemetry.AssuranceHealthEvaluationEvent{
		Operation:        operation,
		Result:           result,
		Status:           string(component.Status),
		Component:        string(component.Component),
		Severity:         string(component.Severity),
		OperationalProof: operationalProofCategory(component.Component),
		ReasonCategory:   string(component.Reason),
	})
}

func (s *Service) recordOperationalProof(ctx context.Context, component HealthComponentSummary) {
	target := operationalProofCategory(component.Component)
	if target == "none" {
		return
	}
	observer, ok := s.observer.(interface {
		RecordOperationalProof(context.Context, telemetry.OperationalProofEvent)
	})
	if !ok {
		return
	}
	observer.RecordOperationalProof(ctx, telemetry.OperationalProofEvent{
		Target:         target,
		Status:         string(component.Status),
		Severity:       string(component.Severity),
		ReasonCategory: string(component.Reason),
	})
}

func (s *Service) recordIncidentLifecycle(ctx context.Context, operation, result string, incident Incident) {
	s.logAssuranceLifecycle(operation, result, string(incident.Status), string(incident.Component), string(incident.Severity), string(incident.Reason), "", "")
	observer, ok := s.observer.(interface {
		RecordAssuranceIncidentLifecycle(context.Context, telemetry.AssuranceIncidentLifecycleEvent)
	})
	if !ok {
		return
	}
	observer.RecordAssuranceIncidentLifecycle(ctx, telemetry.AssuranceIncidentLifecycleEvent{
		Operation:      operation,
		Result:         result,
		Status:         string(incident.Status),
		Component:      string(incident.Component),
		Severity:       string(incident.Severity),
		ReasonCategory: string(incident.Reason),
	})
}

func (s *Service) recordAlertCandidate(ctx context.Context, operation, result string, candidate AlertCandidate) {
	s.logAssuranceLifecycle(operation, result, alertCandidateMetricStatus(candidate), string(candidate.Component), string(candidate.Severity), string(candidate.Reason), "", "")
	observer, ok := s.observer.(interface {
		RecordAssuranceAlertCandidate(context.Context, telemetry.AssuranceAlertCandidateEvent)
	})
	if !ok {
		return
	}
	observer.RecordAssuranceAlertCandidate(ctx, telemetry.AssuranceAlertCandidateEvent{
		Operation:      operation,
		Result:         result,
		Status:         alertCandidateMetricStatus(candidate),
		Component:      string(candidate.Component),
		Severity:       string(candidate.Severity),
		ReasonCategory: string(candidate.Reason),
	})
}

func (s *Service) recordAlertDelivery(ctx context.Context, candidate AlertCandidate, attempt AlertDeliveryAttempt) {
	s.logAssuranceLifecycle("deliver", string(attempt.Result), string(attempt.Result), string(candidate.Component), string(candidate.Severity), "none", string(attempt.Adapter), metricFailureCategory(attempt.FailureCategory))
	observer, ok := s.observer.(interface {
		RecordAssuranceAlertDelivery(context.Context, telemetry.AssuranceAlertDeliveryEvent)
	})
	if !ok {
		return
	}
	observer.RecordAssuranceAlertDelivery(ctx, telemetry.AssuranceAlertDeliveryEvent{
		Adapter:         string(attempt.Adapter),
		Result:          string(attempt.Result),
		Severity:        string(candidate.Severity),
		Component:       string(candidate.Component),
		FailureCategory: metricFailureCategory(attempt.FailureCategory),
	})
}

func (s *Service) recordConformanceRun(ctx context.Context, run ConformanceRun, profileStatus ConformanceProfileStatus, diagnostics []MissingEvidenceDiagnostic) {
	observer, ok := s.observer.(interface {
		RecordConformanceRun(context.Context, telemetry.ConformanceRunEvent)
	})
	if len(diagnostics) == 0 {
		s.logConformanceLifecycle("run", string(run.Result), "all", readinessImpactForConformanceResult(run.Result), "none")
		if !ok {
			return
		}
		observer.RecordConformanceRun(ctx, telemetry.ConformanceRunEvent{
			Result:                  string(run.Result),
			ProfileStatus:           string(profileStatus),
			EvidenceCategory:        "all",
			MissingEvidenceCategory: "none",
			ReadinessImpact:         readinessImpactForConformanceResult(run.Result),
		})
		return
	}
	for _, diagnostic := range diagnostics {
		s.logConformanceLifecycle("run", string(run.Result), string(diagnostic.EvidenceKind), string(diagnostic.ReadinessImpact), string(diagnostic.Category))
		if !ok {
			continue
		}
		observer.RecordConformanceRun(ctx, telemetry.ConformanceRunEvent{
			Result:                  string(run.Result),
			ProfileStatus:           string(profileStatus),
			EvidenceCategory:        string(diagnostic.EvidenceKind),
			MissingEvidenceCategory: string(diagnostic.Category),
			ReadinessImpact:         string(diagnostic.ReadinessImpact),
		})
	}
}

func (s *Service) recordMissingEvidenceDiagnostic(ctx context.Context, diagnostic MissingEvidenceDiagnostic) {
	s.logConformanceLifecycle("diagnostic", string(diagnostic.ReadinessImpact), string(diagnostic.EvidenceKind), string(diagnostic.ReadinessImpact), string(diagnostic.Category))
	observer, ok := s.observer.(interface {
		RecordMissingEvidenceDiagnostic(context.Context, telemetry.MissingEvidenceDiagnosticEvent)
	})
	if !ok {
		return
	}
	observer.RecordMissingEvidenceDiagnostic(ctx, telemetry.MissingEvidenceDiagnosticEvent{
		EvidenceCategory:        string(diagnostic.EvidenceKind),
		MissingEvidenceCategory: string(diagnostic.Category),
		ReadinessImpact:         string(diagnostic.ReadinessImpact),
	})
}

func (s *Service) recordReadinessReport(ctx context.Context, report ReadinessReport, health HealthEvaluation, conformance ConformanceRun, activeIncidents int) {
	s.logConformanceLifecycle("readiness", string(report.Status), "unknown", string(report.Status), "unknown")
	observer, ok := s.observer.(interface {
		RecordReadinessReport(context.Context, telemetry.ReadinessReportEvent)
	})
	if !ok {
		return
	}
	observer.RecordReadinessReport(ctx, telemetry.ReadinessReportEvent{
		ReadinessStatus:           string(report.Status),
		RuntimeCategory:           metricHealthStatus(health.Status),
		ConformanceCategory:       metricConformanceResult(conformance.Result),
		IncidentCategory:          incidentMetricCategory(activeIncidents),
		RecommendedActionCategory: firstRunbookHintCategory(report.RecommendedActions),
	})
}

func (s *Service) recordRecoveryVerification(ctx context.Context, verification RecoveryVerification) {
	s.logAssuranceLifecycle("recovery_verify", verification.ResultCategory, string(verification.Status), "unknown", "unknown", "none", "", "")
	observer, ok := s.observer.(interface {
		RecordRecoveryVerification(context.Context, telemetry.RecoveryVerificationEvent)
	})
	if !ok {
		return
	}
	observer.RecordRecoveryVerification(ctx, telemetry.RecoveryVerificationEvent{
		Target:         string(verification.Target),
		Status:         string(verification.Status),
		ResultCategory: verification.ResultCategory,
	})
}

func (s *Service) recordAssuranceCleanup(ctx context.Context, run RetentionRun) {
	s.logAssuranceLifecycle("cleanup", retentionRunMetricResult(run.Status), string(run.Status), "unknown", "unknown", "none", "", "")
	observer, ok := s.observer.(interface {
		RecordAssuranceCleanup(context.Context, telemetry.AssuranceCleanupEvent)
	})
	if !ok {
		return
	}
	observer.RecordAssuranceCleanup(ctx, telemetry.AssuranceCleanupEvent{
		RecordCategory:  string(run.RecordCategory),
		Result:          retentionRunMetricResult(run.Status),
		DeletedCategory: deletedMetricCategory(run.DeletedCount),
	})
}

func (s *Service) logAssuranceLifecycle(operation, result, status, component, severity, reasonCategory, adapter, failureCategory string) {
	if s.logger == nil {
		return
	}
	s.logger.Printf(
		"mode=service component=assurance event=lifecycle operation=%s result=%s status=%s health_component=%s severity=%s reason_category=%s adapter=%s failure_category=%s",
		boundedLifecycleLogLabel(operation),
		boundedLifecycleLogLabel(result),
		boundedLifecycleLogLabel(status),
		boundedLifecycleLogLabel(component),
		boundedLifecycleLogLabel(severity),
		boundedLifecycleLogLabel(reasonCategory),
		boundedLifecycleLogLabel(adapter),
		boundedLifecycleLogLabel(failureCategory),
	)
}

func (s *Service) logConformanceLifecycle(operation, result, evidenceCategory, readinessStatus, missingEvidenceCategory string) {
	if s.logger == nil {
		return
	}
	s.logger.Printf(
		"mode=service component=conformance event=lifecycle operation=%s result=%s evidence_category=%s readiness_status=%s missing_evidence_category=%s",
		boundedLifecycleLogLabel(operation),
		boundedLifecycleLogLabel(result),
		boundedLifecycleLogLabel(evidenceCategory),
		boundedLifecycleLogLabel(readinessStatus),
		boundedLifecycleLogLabel(missingEvidenceCategory),
	)
}

func boundedAlertPayloadBytes(payload map[string]any, maxBytes int) ([]byte, string, error) {
	encoded, err := json.Marshal(normalizeAnyMapLocal(payload))
	if err != nil {
		return nil, "", fmt.Errorf("marshal alert payload: %w", err)
	}
	sum := sha256.Sum256(encoded)
	if maxBytes > 0 && len(encoded) > maxBytes {
		return nil, hex.EncodeToString(sum[:]), fmt.Errorf("alert payload exceeds %d bytes", maxBytes)
	}
	return encoded, hex.EncodeToString(sum[:]), nil
}

func normalizeAnyMapLocal(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	normalized := make(map[string]any, len(input))
	for key, value := range input {
		normalized[key] = value
	}
	return normalized
}

func alertDeliveryFailureCategory(err error, resp *http.Response) string {
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "timeout") {
			return "timeout"
		}
		return "request_failed"
	}
	if resp != nil {
		return fmt.Sprintf("http_status_%d", resp.StatusCode)
	}
	return "request_failed"
}

func missingEvidenceCategoryForKind(kind ExpectedEvidenceKind) MissingEvidenceCategory {
	switch kind {
	case ExpectedEvidenceSession:
		return MissingEvidenceSessionWithoutOutcome
	case ExpectedEvidenceContext:
		return MissingEvidenceTurnWithoutContext
	case ExpectedEvidenceOutcome:
		return MissingEvidenceSessionWithoutOutcome
	case ExpectedEvidenceVerification:
		return MissingEvidenceVerificationMissing
	case ExpectedEvidenceUsefulnessFeedback:
		return MissingEvidenceFeedbackWithoutSubject
	case ExpectedEvidenceTaskEvaluation:
		return MissingEvidenceTaskEvaluationMissingEvidence
	case ExpectedEvidenceRepair:
		return MissingEvidenceRepairWithoutVerification
	case ExpectedEvidenceRankingRollout:
		return MissingEvidenceRolloutWithoutDryRun
	case ExpectedEvidenceWorkflow:
		return MissingEvidenceWorkflowIncomplete
	default:
		return MissingEvidenceOutOfScope
	}
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func latestHealthEvaluation(evaluations []HealthEvaluation) HealthEvaluation {
	var latest HealthEvaluation
	for _, evaluation := range evaluations {
		if latest.ID == "" || evaluation.CreatedAt.After(latest.CreatedAt) {
			latest = evaluation
		}
	}
	return latest
}

func latestConformanceRun(runs []ConformanceRun) ConformanceRun {
	var latest ConformanceRun
	for _, run := range runs {
		if latest.ID == "" || run.CreatedAt.After(latest.CreatedAt) {
			latest = run
		}
	}
	return latest
}

func latestOperationalProofs(proofs []OperationalProof) (OperationalProof, OperationalProof) {
	var capacity OperationalProof
	var backup OperationalProof
	for _, proof := range proofs {
		switch proof.Target {
		case OperationalProofCapacityLoad:
			if capacity.ID == "" || proof.ObservedAt.After(capacity.ObservedAt) {
				capacity = proof
			}
		case OperationalProofBackupRestore:
			if backup.ID == "" || proof.ObservedAt.After(backup.ObservedAt) {
				backup = proof
			}
		}
	}
	return capacity, backup
}

func operationalProofCategory(component HealthComponent) string {
	switch component {
	case ComponentCapacityLoad:
		return string(OperationalProofCapacityLoad)
	case ComponentBackupRestore:
		return string(OperationalProofBackupRestore)
	default:
		return "none"
	}
}

func countActiveIncidents(incidents []Incident) int {
	count := 0
	for _, incident := range incidents {
		if incident.Status != IncidentStatusResolved {
			count++
		}
	}
	return count
}

func readinessStatus(health HealthEvaluation, conformance ConformanceRun, capacity OperationalProof, backup OperationalProof) ReadinessStatus {
	if health.ID == "" || conformance.ID == "" {
		return ReadinessStatusUnknown
	}
	if health.Status == HealthStatusUnhealthy || conformance.Result == ConformanceResultFailed {
		return ReadinessStatusBlocked
	}
	if health.Status != HealthStatusHealthy || conformance.Result == ConformanceResultDegraded ||
		proofDegradesReadiness(capacity) || proofDegradesReadiness(backup) {
		return ReadinessStatusDegraded
	}
	return ReadinessStatusReady
}

func proofDegradesReadiness(proof OperationalProof) bool {
	if proof.ID == "" {
		return true
	}
	return proof.Status != HealthStatusHealthy
}

func readinessRecommendedActions(status ReadinessStatus, conformance ConformanceRun, capacity OperationalProof, backup OperationalProof, activeIncidents int, alerts int) []RunbookHintCategory {
	actions := make([]RunbookHintCategory, 0)
	appendAction := func(action RunbookHintCategory) {
		for _, existing := range actions {
			if existing == action {
				return
			}
		}
		actions = append(actions, action)
	}
	if status == ReadinessStatusUnknown || conformance.ID == "" || conformance.Result != ConformanceResultPassed {
		appendAction(RunbookHintReviewConformanceProfile)
	}
	if proofDegradesReadiness(capacity) {
		appendAction(RunbookHintReviewCapacityProof)
	}
	if proofDegradesReadiness(backup) {
		appendAction(RunbookHintReviewBackupRestoreProof)
	}
	if activeIncidents > 0 {
		appendAction(RunbookHintReviewBacklog)
	}
	if alerts > 0 {
		appendAction(RunbookHintReviewAlertDelivery)
	}
	if len(actions) == 0 {
		appendAction(RunbookHintReviewConformanceProfile)
	}
	return actions
}

func appendUniqueRunbookHint(actions []RunbookHintCategory, action RunbookHintCategory) []RunbookHintCategory {
	for _, existing := range actions {
		if existing == action {
			return actions
		}
	}
	return append(actions, action)
}

func readinessImpactForConformanceResult(result ConformanceResult) string {
	switch result {
	case ConformanceResultPassed:
		return string(ReadinessStatusReady)
	case ConformanceResultDegraded:
		return string(ReadinessStatusDegraded)
	case ConformanceResultFailed:
		return string(ReadinessStatusBlocked)
	default:
		return string(ReadinessStatusUnknown)
	}
}

func alertCandidateMetricStatus(candidate AlertCandidate) string {
	if !candidate.SuppressedUntil.IsZero() {
		return "suppressed"
	}
	if candidate.NextAttemptAt.IsZero() {
		return "created"
	}
	return "queued"
}

func metricFailureCategory(category string) string {
	if strings.TrimSpace(category) == "" {
		return "none"
	}
	return strings.TrimSpace(category)
}

func retentionRunMetricResult(status HealthStatus) string {
	switch status {
	case HealthStatusHealthy:
		return "ok"
	case HealthStatusDegraded, HealthStatusStale, HealthStatusUnknown:
		return "degraded"
	case HealthStatusUnhealthy:
		return "failed"
	default:
		return "unknown"
	}
}

func deletedMetricCategory(count int) string {
	if count <= 0 {
		return "none"
	}
	return "some"
}

func metricHealthStatus(status HealthStatus) string {
	if status == "" {
		return "unknown"
	}
	return string(status)
}

func metricConformanceResult(result ConformanceResult) string {
	if result == "" {
		return "unknown"
	}
	return string(result)
}

func incidentMetricCategory(active int) string {
	if active > 0 {
		return "active"
	}
	return "none"
}

func firstRunbookHintCategory(actions []RunbookHintCategory) string {
	if len(actions) == 0 {
		return "none"
	}
	return string(actions[0])
}

func boundedLifecycleLogLabel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return strings.TrimSpace(value)
}

func componentStatus(evaluation HealthEvaluation, component HealthComponent) HealthStatus {
	for _, summary := range evaluation.Components {
		if summary.Component == component {
			return summary.Status
		}
	}
	return ""
}

func (s *Service) ensureIncident(ctx context.Context, evaluation HealthEvaluation, component HealthComponentSummary, observedAt time.Time) (Incident, error) {
	dedupKey := incidentDeduplicationKey(component)
	existing, err := s.store.ListIncidents(ctx, ListIncidentsInput{Scope: evaluation.Scope})
	if err != nil {
		return Incident{}, err
	}
	for _, incident := range existing {
		if incident.DeduplicationKey == dedupKey {
			return incident, nil
		}
	}
	incident := Incident{
		ID:                 s.newID("incident"),
		Scope:              evaluation.Scope,
		Status:             IncidentStatusOpen,
		Severity:           incidentSeverity(component),
		Component:          component.Component,
		Reason:             component.Reason,
		DeduplicationKey:   dedupKey,
		LatestEvaluationID: evaluation.ID,
		RunbookHints:       runbookHintsForComponent(component.Component),
		Metadata: map[string]any{
			"health_evaluation_id": evaluation.ID,
			"component":            string(component.Component),
			"component_status":     string(component.Status),
			"observed_at":          observedAt.UTC().Format(time.RFC3339Nano),
			"fresh_through":        component.FreshThrough.UTC().Format(time.RFC3339Nano),
		},
		OpenedAt:  observedAt,
		UpdatedAt: observedAt,
	}
	created, err := s.store.CreateIncident(ctx, incident)
	if err != nil {
		return Incident{}, err
	}
	s.recordIncidentLifecycle(ctx, "open", "ok", created)
	return created, nil
}

func alertCandidateFromIncident(input AlertCandidateGenerationInput, incident Incident) AlertCandidate {
	key := "incident:" + incident.ID + ":" + string(incident.Component) + ":" + string(incident.Reason)
	return AlertCandidate{
		ID:               generatedAlertCandidateID(key, input.CreatedAt),
		Scope:            input.Scope.Normalized(),
		IncidentID:       incident.ID,
		Severity:         incident.Severity,
		Component:        incident.Component,
		Reason:           incident.Reason,
		DeduplicationKey: key,
		DeliveryPolicy:   strings.TrimSpace(input.DeliveryPolicy),
		Payload:          boundedAlertPayload("incident", incident.Severity, incident.Component, incident.Reason),
		CreatedAt:        input.CreatedAt.UTC(),
		NextAttemptAt:    input.CreatedAt.UTC(),
	}
}

func alertCandidateFromComponent(input AlertCandidateGenerationInput, component HealthComponentSummary) AlertCandidate {
	key := "evaluation:" + input.Evaluation.ID + ":" + string(component.Component) + ":" + string(component.Reason)
	return AlertCandidate{
		ID:               generatedAlertCandidateID(key, input.CreatedAt),
		Scope:            input.Scope.Normalized(),
		EvaluationID:     input.Evaluation.ID,
		Severity:         component.Severity,
		Component:        component.Component,
		Reason:           component.Reason,
		DeduplicationKey: key,
		DeliveryPolicy:   strings.TrimSpace(input.DeliveryPolicy),
		Payload:          boundedAlertPayload("health_evaluation", component.Severity, component.Component, component.Reason),
		CreatedAt:        input.CreatedAt.UTC(),
		NextAttemptAt:    input.CreatedAt.UTC(),
	}
}

func generatedAlertCandidateID(key string, at time.Time) string {
	sanitized := strings.NewReplacer(":", "_", "/", "_", " ", "_").Replace(key)
	id := "alert_" + sanitized + "_" + fmt.Sprintf("%d", at.UTC().UnixNano())
	if len(id) <= maxIdentifierLength {
		return id
	}
	return id[:maxIdentifierLength]
}

func incidentAlertEligible(incident Incident) bool {
	return incident.Status != IncidentStatusResolved && (incident.Severity == SeverityCritical || incident.Status == IncidentStatusOpen)
}

func componentAlertEligible(component HealthComponentSummary) bool {
	return component.Severity == SeverityCritical || component.Status == HealthStatusUnhealthy
}

func alertCandidateDeduped(candidate AlertCandidate, existing map[string]AlertCandidate, now time.Time, window time.Duration) bool {
	if window <= 0 {
		return false
	}
	prior, ok := existing[candidate.DeduplicationKey]
	if !ok {
		return false
	}
	return !prior.CreatedAt.IsZero() && !prior.CreatedAt.Before(now.UTC().Add(-window))
}

func boundedAlertPayload(source string, severity Severity, component HealthComponent, reason ReasonCategory) map[string]any {
	hints := runbookHintsForComponent(component)
	hint := ""
	if len(hints) > 0 {
		hint = string(hints[0])
	}
	return map[string]any{
		"source":        source,
		"severity":      string(severity),
		"component":     string(component),
		"reason":        string(reason),
		"runbook_hint":  hint,
		"admin_surface": adminSurfaceForComponent(component),
	}
}

func adminSurfaceForComponent(component HealthComponent) string {
	switch component {
	case ComponentCapacityLoad:
		return "admin.operational_proofs.capacity_load"
	case ComponentBackupRestore:
		return "admin.operational_proofs.backup_restore"
	case ComponentConformance, ComponentRanking:
		return "admin.conformance"
	case ComponentRepair:
		return "admin.memory_quality.repair"
	default:
		return "admin.assurance"
	}
}

func incidentActionTargetStatus(action IncidentAction, current IncidentStatus) IncidentStatus {
	switch action {
	case IncidentActionAcknowledge:
		return IncidentStatusAcknowledged
	case IncidentActionSuppress:
		return IncidentStatusSuppressed
	case IncidentActionResolve:
		return IncidentStatusResolved
	case IncidentActionReopen:
		return IncidentStatusOpen
	case IncidentActionVerify:
		return current
	default:
		return current
	}
}

func buildHealthComponentSummary(scope memory.Scope, evaluationID string, signal componentSignal, observedAt time.Time) HealthComponentSummary {
	obs := signal.obs
	if obs.ObservedAt.IsZero() {
		obs.ObservedAt = observedAt
	}
	if obs.Severity == "" {
		obs.Severity = severityForStatus(obs.Status)
	}
	if obs.Reason == "" {
		obs.Reason = defaultReasonForComponent(signal.component, obs.Status)
	}
	component := HealthComponentSummary{
		ID:           componentSummaryID(evaluationID, signal.source, signal.component),
		EvaluationID: evaluationID,
		Scope:        scope.Normalized(),
		Component:    signal.component,
		Status:       obs.Status,
		Severity:     obs.Severity,
		Reason:       obs.Reason,
		ObservedAt:   obs.ObservedAt.UTC(),
		FreshThrough: obs.FreshThrough.UTC(),
		Evidence:     cloneAnyMap(obs.Evidence),
	}
	if component.Evidence == nil {
		component.Evidence = map[string]any{}
	}
	component.Evidence["source"] = signal.source
	return component
}

func aggregateHealthEvaluation(currentStatus HealthStatus, currentSeverity Severity, currentReason ReasonCategory, component HealthComponentSummary) (HealthStatus, Severity, ReasonCategory) {
	if component.Status == HealthStatusUnhealthy {
		return HealthStatusUnhealthy, SeverityCritical, component.Reason
	}
	if component.Status == HealthStatusDegraded || component.Status == HealthStatusStale {
		if currentStatus != HealthStatusUnhealthy {
			return HealthStatusDegraded, severityForStatus(component.Status), component.Reason
		}
		return currentStatus, currentSeverity, currentReason
	}
	if component.Status == HealthStatusUnknown && currentStatus == HealthStatusHealthy {
		return HealthStatusUnknown, severityForStatus(component.Status), component.Reason
	}
	if component.Status == HealthStatusUnknown && currentStatus == "" {
		return HealthStatusUnknown, severityForStatus(component.Status), component.Reason
	}
	if currentStatus == HealthStatusUnknown {
		return HealthStatusUnknown, currentSeverity, currentReason
	}
	if currentStatus == "" || currentStatus == HealthStatusHealthy {
		return HealthStatusHealthy, severityForStatus(component.Status), component.Reason
	}
	return currentStatus, currentSeverity, currentReason
}

func shouldCreateIncident(component HealthComponentSummary) bool {
	return component.Status == HealthStatusDegraded || component.Status == HealthStatusUnhealthy || component.Status == HealthStatusStale
}

func incidentDeduplicationKey(component HealthComponentSummary) string {
	return string(component.Component) + ":" + string(component.Reason)
}

func incidentSeverity(component HealthComponentSummary) Severity {
	if component.Status == HealthStatusUnhealthy || component.Severity == SeverityCritical {
		return SeverityCritical
	}
	return SeverityWarning
}

func severityForStatus(status HealthStatus) Severity {
	switch status {
	case HealthStatusUnhealthy:
		return SeverityCritical
	case HealthStatusDegraded, HealthStatusStale, HealthStatusUnknown:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}

func defaultReasonForComponent(component HealthComponent, status HealthStatus) ReasonCategory {
	switch component {
	case ComponentRuntime:
		return ReasonRuntimeReady
	case ComponentBacklog:
		return ReasonBacklogPressure
	case ComponentDependency:
		return ReasonUnknown
	case ComponentProof:
		return ReasonUnknown
	case ComponentFeedback:
		return ReasonUnknown
	case ComponentTask:
		return ReasonUnknown
	case ComponentRepair:
		return ReasonUnknown
	case ComponentRanking:
		return ReasonUnknown
	case ComponentConformance:
		return ReasonConformanceMissingEvidence
	case ComponentWorkflow:
		return ReasonWorkflowGap
	case ComponentCapacityLoad:
		if status == HealthStatusHealthy {
			return ReasonCapacityWithinThresholds
		}
		return ReasonCapacityThresholdExceeded
	case ComponentBackupRestore:
		if status == HealthStatusHealthy {
			return ReasonBackupRestoreFresh
		}
		return ReasonBackupRestoreStale
	default:
		return ReasonUnknown
	}
}

func runbookHintsForComponent(component HealthComponent) []RunbookHintCategory {
	switch component {
	case ComponentBacklog, ComponentRuntime, ComponentDependency, ComponentProof, ComponentFeedback, ComponentTask:
		return []RunbookHintCategory{RunbookHintReviewBacklog}
	case ComponentRepair:
		return []RunbookHintCategory{RunbookHintReviewRepair}
	case ComponentRanking, ComponentConformance:
		return []RunbookHintCategory{RunbookHintReviewConformanceProfile}
	case ComponentWorkflow:
		return []RunbookHintCategory{RunbookHintReviewWorkflow}
	case ComponentCapacityLoad:
		return []RunbookHintCategory{RunbookHintReviewCapacityProof}
	case ComponentBackupRestore:
		return []RunbookHintCategory{RunbookHintReviewBackupRestoreProof}
	default:
		return []RunbookHintCategory{RunbookHintReviewBacklog}
	}
}

func isZeroHealthObservation(obs HealthObservation) bool {
	return obs.Status == "" && obs.Severity == "" && obs.Reason == "" && obs.ObservedAt.IsZero() && obs.FreshThrough.IsZero() && len(obs.Evidence) == 0
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func componentSummaryID(evaluationID string, source string, component HealthComponent) string {
	suffix := ":" + source + ":" + string(component)
	id := evaluationID + suffix
	if len(id) <= maxIdentifierLength {
		return id
	}
	limit := maxIdentifierLength - len(suffix)
	if limit <= 0 {
		return id[:maxIdentifierLength]
	}
	return evaluationID[:limit] + suffix
}
