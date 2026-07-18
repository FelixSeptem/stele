package assurance

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

const (
	maxReasonLength           = 256
	maxActorLength            = 256
	maxIdentifierLength       = 512
	maxDeduplicationKeyLength = 512
	maxIdempotencyKeyLength   = 256
	maxMetadataEntries        = 32
	maxMetadataKeyLength      = 128
	maxMetadataValueLength    = 1024
	defaultWebhookMaxBytes    = 64 * 1024
	minDeliveryTimeout        = time.Second
	defaultDeliveryTimeout    = 10 * time.Second
	minFreshnessWindow        = time.Minute
	defaultFreshnessWindow    = 24 * time.Hour
	defaultHistoryRetention   = 7 * 24 * time.Hour
)

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
	HealthStatusStale     HealthStatus = "stale"
)

func (s HealthStatus) Valid() bool {
	switch s {
	case HealthStatusHealthy, HealthStatusDegraded, HealthStatusUnhealthy, HealthStatusUnknown, HealthStatusStale:
		return true
	default:
		return false
	}
}

type ReadinessStatus string

const (
	ReadinessStatusReady    ReadinessStatus = "ready"
	ReadinessStatusDegraded ReadinessStatus = "degraded"
	ReadinessStatusUnknown  ReadinessStatus = "unknown"
	ReadinessStatusBlocked  ReadinessStatus = "blocked"
)

func (s ReadinessStatus) Valid() bool {
	switch s {
	case ReadinessStatusReady, ReadinessStatusDegraded, ReadinessStatusUnknown, ReadinessStatusBlocked:
		return true
	default:
		return false
	}
}

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

func (s Severity) Valid() bool {
	switch s {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	default:
		return false
	}
}

type HealthComponent string

const (
	ComponentRuntime       HealthComponent = "runtime"
	ComponentBacklog       HealthComponent = "backlog"
	ComponentDependency    HealthComponent = "dependency"
	ComponentProof         HealthComponent = "proof"
	ComponentSession       HealthComponent = "session"
	ComponentFeedback      HealthComponent = "feedback"
	ComponentTask          HealthComponent = "task"
	ComponentRepair        HealthComponent = "repair"
	ComponentRanking       HealthComponent = "ranking_rollout"
	ComponentConformance   HealthComponent = "conformance"
	ComponentCapacityLoad  HealthComponent = "capacity_load"
	ComponentBackupRestore HealthComponent = "backup_restore"
)

func (c HealthComponent) Valid() bool {
	switch c {
	case ComponentRuntime, ComponentBacklog, ComponentDependency, ComponentProof, ComponentSession,
		ComponentFeedback, ComponentTask, ComponentRepair, ComponentRanking, ComponentConformance,
		ComponentCapacityLoad, ComponentBackupRestore:
		return true
	default:
		return false
	}
}

type IncidentStatus string

const (
	IncidentStatusOpen         IncidentStatus = "open"
	IncidentStatusAcknowledged IncidentStatus = "acknowledged"
	IncidentStatusSuppressed   IncidentStatus = "suppressed"
	IncidentStatusResolved     IncidentStatus = "resolved"
)

func (s IncidentStatus) Valid() bool {
	switch s {
	case IncidentStatusOpen, IncidentStatusAcknowledged, IncidentStatusSuppressed, IncidentStatusResolved:
		return true
	default:
		return false
	}
}

type IncidentAction string

const (
	IncidentActionAcknowledge IncidentAction = "acknowledge"
	IncidentActionSuppress    IncidentAction = "suppress"
	IncidentActionResolve     IncidentAction = "resolve"
	IncidentActionReopen      IncidentAction = "reopen"
	IncidentActionVerify      IncidentAction = "verify"
)

func (a IncidentAction) Valid() bool {
	switch a {
	case IncidentActionAcknowledge, IncidentActionSuppress, IncidentActionResolve, IncidentActionReopen, IncidentActionVerify:
		return true
	default:
		return false
	}
}

type ReasonCategory string

const (
	ReasonRuntimeReady               ReasonCategory = "runtime_ready"
	ReasonBacklogPressure            ReasonCategory = "backlog_pressure"
	ReasonCapacityWithinThresholds   ReasonCategory = "capacity_within_thresholds"
	ReasonCapacityThresholdExceeded  ReasonCategory = "capacity_threshold_exceeded"
	ReasonBackupRestoreFresh         ReasonCategory = "backup_restore_fresh"
	ReasonBackupRestoreStale         ReasonCategory = "backup_restore_stale"
	ReasonConformanceMissingEvidence ReasonCategory = "conformance_missing_evidence"
	ReasonUnknown                    ReasonCategory = "unknown"
)

func (r ReasonCategory) Valid() bool {
	switch r {
	case ReasonRuntimeReady, ReasonBacklogPressure, ReasonCapacityWithinThresholds, ReasonCapacityThresholdExceeded,
		ReasonBackupRestoreFresh, ReasonBackupRestoreStale, ReasonConformanceMissingEvidence, ReasonUnknown:
		return true
	default:
		return false
	}
}

type HealthEvaluation struct {
	ID         string                   `json:"id"`
	Scope      memory.Scope             `json:"scope"`
	Status     HealthStatus             `json:"status"`
	Severity   Severity                 `json:"severity"`
	Reason     ReasonCategory           `json:"reason,omitempty"`
	Components []HealthComponentSummary `json:"components,omitempty"`
	CreatedAt  time.Time                `json:"created_at"`
}

type Incident struct {
	ID                 string                `json:"id"`
	Scope              memory.Scope          `json:"scope"`
	Status             IncidentStatus        `json:"status"`
	Severity           Severity              `json:"severity"`
	Component          HealthComponent       `json:"component"`
	Reason             ReasonCategory        `json:"reason"`
	DeduplicationKey   string                `json:"deduplication_key"`
	LatestEvaluationID string                `json:"latest_evaluation_id,omitempty"`
	RunbookHints       []RunbookHintCategory `json:"runbook_hints,omitempty"`
	Metadata           map[string]any        `json:"metadata,omitempty"`
	OpenedAt           time.Time             `json:"opened_at"`
	UpdatedAt          time.Time             `json:"updated_at"`
	ResolvedAt         time.Time             `json:"resolved_at,omitempty"`
}

func (i Incident) Validate() error {
	if err := validateID(i.ID, "incident id"); err != nil {
		return err
	}
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if !i.Status.Valid() {
		return fmt.Errorf("incident status %q is invalid", i.Status)
	}
	if !i.Severity.Valid() {
		return fmt.Errorf("severity %q is invalid", i.Severity)
	}
	if !i.Component.Valid() {
		return fmt.Errorf("health component %q is invalid", i.Component)
	}
	if !i.Reason.Valid() {
		return fmt.Errorf("reason category %q is invalid", i.Reason)
	}
	if err := validateDeduplicationKey(i.DeduplicationKey); err != nil {
		return err
	}
	for _, hint := range i.RunbookHints {
		if !hint.Valid() {
			return fmt.Errorf("runbook hint %q is invalid", hint)
		}
	}
	if err := validateMetadata(i.Metadata, "metadata"); err != nil {
		return err
	}
	if i.OpenedAt.IsZero() || i.UpdatedAt.IsZero() {
		return fmt.Errorf("opened at and updated at are required")
	}
	return nil
}

type IncidentTransition struct {
	ID         string         `json:"id"`
	IncidentID string         `json:"incident_id"`
	Scope      memory.Scope   `json:"scope"`
	FromStatus IncidentStatus `json:"from_status,omitempty"`
	ToStatus   IncidentStatus `json:"to_status"`
	Action     IncidentAction `json:"action"`
	Actor      string         `json:"actor"`
	Reason     string         `json:"reason"`
	OccurredAt time.Time      `json:"occurred_at"`
}

func (t IncidentTransition) Validate() error {
	if err := validateID(t.ID, "incident transition id"); err != nil {
		return err
	}
	if err := validateID(t.IncidentID, "incident id"); err != nil {
		return err
	}
	if err := t.Scope.Validate(); err != nil {
		return err
	}
	if t.FromStatus != "" && !t.FromStatus.Valid() {
		return fmt.Errorf("from incident status %q is invalid", t.FromStatus)
	}
	if !t.ToStatus.Valid() {
		return fmt.Errorf("to incident status %q is invalid", t.ToStatus)
	}
	if !t.Action.Valid() {
		return fmt.Errorf("incident action %q is invalid", t.Action)
	}
	if err := validateActorReason(t.Actor, t.Reason); err != nil {
		return err
	}
	if t.OccurredAt.IsZero() {
		return fmt.Errorf("occurred at is required")
	}
	return nil
}

type AlertCandidate struct {
	ID               string          `json:"id"`
	Scope            memory.Scope    `json:"scope"`
	IncidentID       string          `json:"incident_id,omitempty"`
	EvaluationID     string          `json:"evaluation_id,omitempty"`
	Severity         Severity        `json:"severity"`
	Component        HealthComponent `json:"component"`
	Reason           ReasonCategory  `json:"reason"`
	DeduplicationKey string          `json:"deduplication_key"`
	DeliveryPolicy   string          `json:"delivery_policy"`
	Payload          map[string]any  `json:"payload,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	NextAttemptAt    time.Time       `json:"next_attempt_at,omitempty"`
	SuppressedUntil  time.Time       `json:"suppressed_until,omitempty"`
}

func (a AlertCandidate) Validate() error {
	if err := validateID(a.ID, "alert candidate id"); err != nil {
		return err
	}
	if err := a.Scope.Validate(); err != nil {
		return err
	}
	if !a.Severity.Valid() {
		return fmt.Errorf("severity %q is invalid", a.Severity)
	}
	if !a.Component.Valid() {
		return fmt.Errorf("health component %q is invalid", a.Component)
	}
	if !a.Reason.Valid() {
		return fmt.Errorf("reason category %q is invalid", a.Reason)
	}
	if err := validateDeduplicationKey(a.DeduplicationKey); err != nil {
		return err
	}
	if strings.TrimSpace(a.DeliveryPolicy) == "" {
		return fmt.Errorf("delivery policy is required")
	}
	if err := validateMetadata(a.Payload, "payload"); err != nil {
		return err
	}
	if a.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	return nil
}

type AlertDeliveryAttempt struct {
	ID               string              `json:"id"`
	AlertCandidateID string              `json:"alert_candidate_id"`
	Scope            memory.Scope        `json:"scope"`
	Adapter          AlertAdapterKind    `json:"adapter"`
	Result           AlertDeliveryResult `json:"result"`
	FailureCategory  string              `json:"failure_category,omitempty"`
	Attempt          int                 `json:"attempt"`
	WorkerID         string              `json:"worker_id,omitempty"`
	LeaseUntil       time.Time           `json:"lease_until,omitempty"`
	NextAttemptAt    time.Time           `json:"next_attempt_at,omitempty"`
	PayloadHash      string              `json:"payload_hash,omitempty"`
	AttemptedAt      time.Time           `json:"attempted_at"`
	CompletedAt      time.Time           `json:"completed_at,omitempty"`
}

func (a AlertDeliveryAttempt) Validate() error {
	if err := validateID(a.ID, "alert delivery attempt id"); err != nil {
		return err
	}
	if err := validateID(a.AlertCandidateID, "alert candidate id"); err != nil {
		return err
	}
	if err := a.Scope.Validate(); err != nil {
		return err
	}
	if !a.Adapter.Valid() {
		return fmt.Errorf("alert adapter %q is invalid", a.Adapter)
	}
	if !a.Result.Valid() {
		return fmt.Errorf("alert delivery result %q is invalid", a.Result)
	}
	if a.Attempt < 0 {
		return fmt.Errorf("attempt must be greater than or equal to zero")
	}
	if a.AttemptedAt.IsZero() {
		return fmt.Errorf("attempted at is required")
	}
	return nil
}

type ClaimAlertCandidatesForDeliveryInput struct {
	Scope         memory.Scope
	WorkerID      string
	Now           time.Time
	LeaseDuration time.Duration
	Limit         int
	MaxAttempts   int
}

func (i ClaimAlertCandidatesForDeliveryInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.WorkerID) == "" {
		return fmt.Errorf("worker id is required")
	}
	if i.Now.IsZero() {
		return fmt.Errorf("now is required")
	}
	if i.LeaseDuration <= 0 {
		return fmt.Errorf("lease duration must be greater than zero")
	}
	if i.Limit <= 0 {
		return fmt.Errorf("limit must be greater than zero")
	}
	if i.MaxAttempts <= 0 {
		return fmt.Errorf("max attempts must be greater than zero")
	}
	return nil
}

type AlertDeliveryClaim struct {
	Candidate  AlertCandidate `json:"candidate"`
	Attempt    int            `json:"attempt"`
	WorkerID   string         `json:"worker_id"`
	ClaimedAt  time.Time      `json:"claimed_at"`
	LeaseUntil time.Time      `json:"lease_until"`
}

func (c AlertDeliveryClaim) Validate() error {
	if err := c.Candidate.Validate(); err != nil {
		return err
	}
	if c.Attempt <= 0 {
		return fmt.Errorf("attempt must be greater than zero")
	}
	if strings.TrimSpace(c.WorkerID) == "" {
		return fmt.Errorf("worker id is required")
	}
	if c.ClaimedAt.IsZero() || c.LeaseUntil.IsZero() {
		return fmt.Errorf("claimed at and lease until are required")
	}
	return nil
}

type OperationalProof struct {
	ID           string                 `json:"id"`
	Scope        memory.Scope           `json:"scope"`
	Target       OperationalProofTarget `json:"target"`
	Status       HealthStatus           `json:"status"`
	Severity     Severity               `json:"severity"`
	Reason       ReasonCategory         `json:"reason"`
	ObservedAt   time.Time              `json:"observed_at"`
	FreshThrough time.Time              `json:"fresh_through,omitempty"`
	Evidence     map[string]any         `json:"evidence,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

func (p OperationalProof) Validate() error {
	if err := validateID(p.ID, "operational proof id"); err != nil {
		return err
	}
	if err := p.Scope.Validate(); err != nil {
		return err
	}
	if !p.Target.Valid() {
		return fmt.Errorf("operational proof target %q is invalid", p.Target)
	}
	if !p.Status.Valid() {
		return fmt.Errorf("health status %q is invalid", p.Status)
	}
	if !p.Severity.Valid() {
		return fmt.Errorf("severity %q is invalid", p.Severity)
	}
	if !p.Reason.Valid() {
		return fmt.Errorf("reason category %q is invalid", p.Reason)
	}
	if p.ObservedAt.IsZero() || p.CreatedAt.IsZero() {
		return fmt.Errorf("observed at and created at are required")
	}
	return validateMetadata(p.Evidence, "evidence")
}

type ConformanceRun struct {
	ID             string            `json:"id"`
	ProfileID      string            `json:"profile_id"`
	Scope          memory.Scope      `json:"scope"`
	Result         ConformanceResult `json:"result"`
	EvidenceCounts map[string]any    `json:"evidence_counts,omitempty"`
	StartedAt      time.Time         `json:"started_at"`
	FinishedAt     time.Time         `json:"finished_at,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

func (r ConformanceRun) Validate() error {
	if err := validateID(r.ID, "conformance run id"); err != nil {
		return err
	}
	if err := validateID(r.ProfileID, "conformance profile id"); err != nil {
		return err
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if !r.Result.Valid() {
		return fmt.Errorf("conformance result %q is invalid", r.Result)
	}
	if r.StartedAt.IsZero() || r.CreatedAt.IsZero() {
		return fmt.Errorf("started at and created at are required")
	}
	return validateMetadata(r.EvidenceCounts, "evidence counts")
}

type MissingEvidenceDiagnostic struct {
	ID               string                  `json:"id"`
	ConformanceRunID string                  `json:"conformance_run_id"`
	Scope            memory.Scope            `json:"scope"`
	EvidenceKind     ExpectedEvidenceKind    `json:"evidence_kind"`
	Category         MissingEvidenceCategory `json:"category"`
	ReadinessImpact  ReadinessStatus         `json:"readiness_impact"`
	Metadata         map[string]any          `json:"metadata,omitempty"`
	CreatedAt        time.Time               `json:"created_at"`
}

func (d MissingEvidenceDiagnostic) Validate() error {
	if err := validateID(d.ID, "missing evidence diagnostic id"); err != nil {
		return err
	}
	if err := validateID(d.ConformanceRunID, "conformance run id"); err != nil {
		return err
	}
	if err := d.Scope.Validate(); err != nil {
		return err
	}
	if !d.EvidenceKind.Valid() {
		return fmt.Errorf("evidence kind %q is invalid", d.EvidenceKind)
	}
	if !d.Category.Valid() {
		return fmt.Errorf("missing evidence category %q is invalid", d.Category)
	}
	if !d.ReadinessImpact.Valid() {
		return fmt.Errorf("readiness impact %q is invalid", d.ReadinessImpact)
	}
	if d.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	return validateMetadata(d.Metadata, "metadata")
}

type ReadinessReport struct {
	ID                 string                `json:"id"`
	Scope              memory.Scope          `json:"scope"`
	Status             ReadinessStatus       `json:"status"`
	HealthEvaluationID string                `json:"health_evaluation_id,omitempty"`
	ConformanceRunID   string                `json:"conformance_run_id,omitempty"`
	ComponentSummary   map[string]any        `json:"component_summary,omitempty"`
	RecommendedActions []RunbookHintCategory `json:"recommended_actions,omitempty"`
	GeneratedAt        time.Time             `json:"generated_at"`
	CreatedAt          time.Time             `json:"created_at"`
}

func (r ReadinessReport) Validate() error {
	if err := validateID(r.ID, "readiness report id"); err != nil {
		return err
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if !r.Status.Valid() {
		return fmt.Errorf("readiness status %q is invalid", r.Status)
	}
	for _, action := range r.RecommendedActions {
		if !action.Valid() {
			return fmt.Errorf("runbook hint %q is invalid", action)
		}
	}
	if r.GeneratedAt.IsZero() || r.CreatedAt.IsZero() {
		return fmt.Errorf("generated at and created at are required")
	}
	return validateMetadata(r.ComponentSummary, "component summary")
}

type RecoveryVerification struct {
	ID              string                     `json:"id"`
	Scope           memory.Scope               `json:"scope"`
	Target          RecoveryVerificationTarget `json:"target"`
	TargetID        string                     `json:"target_id"`
	Status          HealthStatus               `json:"status"`
	CheckedSurfaces []string                   `json:"checked_surfaces,omitempty"`
	ResultCategory  string                     `json:"result_category"`
	LinkedEvidence  map[string]any             `json:"linked_evidence,omitempty"`
	Actor           string                     `json:"actor"`
	Reason          string                     `json:"reason"`
	CreatedAt       time.Time                  `json:"created_at"`
	VerifiedAt      time.Time                  `json:"verified_at,omitempty"`
}

func (v RecoveryVerification) Validate() error {
	if err := validateID(v.ID, "recovery verification id"); err != nil {
		return err
	}
	if err := v.Scope.Validate(); err != nil {
		return err
	}
	if !v.Target.Valid() {
		return fmt.Errorf("recovery verification target %q is invalid", v.Target)
	}
	if err := validateID(v.TargetID, "target id"); err != nil {
		return err
	}
	if !v.Status.Valid() {
		return fmt.Errorf("health status %q is invalid", v.Status)
	}
	if strings.TrimSpace(v.ResultCategory) == "" {
		return fmt.Errorf("result category is required")
	}
	if err := validateActorReason(v.Actor, v.Reason); err != nil {
		return err
	}
	if v.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	return validateMetadata(v.LinkedEvidence, "linked evidence")
}

type RetentionRun struct {
	ID             string         `json:"id"`
	Scope          memory.Scope   `json:"scope"`
	RecordCategory RetentionClass `json:"record_category"`
	Cutoff         time.Time      `json:"cutoff"`
	DeletedCount   int            `json:"deleted_count"`
	Status         HealthStatus   `json:"status"`
	StartedAt      time.Time      `json:"started_at"`
	FinishedAt     time.Time      `json:"finished_at,omitempty"`
}

type ReadRetentionRunInput struct {
	Scope memory.Scope
	RunID string
}

func (i ReadRetentionRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.RunID, "retention run id")
}

type ListRetentionRunsInput struct {
	Scope          memory.Scope
	RecordCategory RetentionClass
}

func (i ListRetentionRunsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.RecordCategory != "" && !i.RecordCategory.Valid() {
		return fmt.Errorf("retention class %q is invalid", i.RecordCategory)
	}
	return nil
}

func (r RetentionRun) Validate() error {
	if err := validateID(r.ID, "retention run id"); err != nil {
		return err
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if !r.RecordCategory.Valid() {
		return fmt.Errorf("retention class %q is invalid", r.RecordCategory)
	}
	if r.Cutoff.IsZero() || r.StartedAt.IsZero() {
		return fmt.Errorf("cutoff and started at are required")
	}
	if r.DeletedCount < 0 {
		return fmt.Errorf("deleted count must be greater than or equal to zero")
	}
	if !r.Status.Valid() {
		return fmt.Errorf("health status %q is invalid", r.Status)
	}
	return nil
}

func (e HealthEvaluation) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("health evaluation id is required")
	}
	if err := e.Scope.Validate(); err != nil {
		return err
	}
	if !e.Status.Valid() {
		return fmt.Errorf("health status %q is invalid", e.Status)
	}
	if !e.Severity.Valid() {
		return fmt.Errorf("severity %q is invalid", e.Severity)
	}
	if e.Reason != "" && !e.Reason.Valid() {
		return fmt.Errorf("reason category %q is invalid", e.Reason)
	}
	if e.CreatedAt.IsZero() {
		return fmt.Errorf("created at is required")
	}
	for _, component := range e.Components {
		if component.EvaluationID == "" {
			component.EvaluationID = e.ID
		}
		if component.Scope == (memory.Scope{}) {
			component.Scope = e.Scope
		}
		if err := component.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type HealthComponentSummary struct {
	ID           string          `json:"id"`
	EvaluationID string          `json:"evaluation_id"`
	Scope        memory.Scope    `json:"scope"`
	Component    HealthComponent `json:"component"`
	Status       HealthStatus    `json:"status"`
	Severity     Severity        `json:"severity"`
	Reason       ReasonCategory  `json:"reason,omitempty"`
	ObservedAt   time.Time       `json:"observed_at"`
	FreshThrough time.Time       `json:"fresh_through,omitempty"`
	Evidence     map[string]any  `json:"evidence,omitempty"`
}

func (s HealthComponentSummary) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("health component id is required")
	}
	if strings.TrimSpace(s.EvaluationID) == "" {
		return fmt.Errorf("health evaluation id is required")
	}
	if err := s.Scope.Validate(); err != nil {
		return err
	}
	if !s.Component.Valid() {
		return fmt.Errorf("health component %q is invalid", s.Component)
	}
	if !s.Status.Valid() {
		return fmt.Errorf("health status %q is invalid", s.Status)
	}
	if !s.Severity.Valid() {
		return fmt.Errorf("severity %q is invalid", s.Severity)
	}
	if s.Reason != "" && !s.Reason.Valid() {
		return fmt.Errorf("reason category %q is invalid", s.Reason)
	}
	if s.ObservedAt.IsZero() {
		return fmt.Errorf("observed at is required")
	}
	if err := validateMetadata(s.Evidence, "evidence"); err != nil {
		return err
	}
	return nil
}

type ExpectedEvidenceKind string

const (
	ExpectedEvidenceSession            ExpectedEvidenceKind = "session"
	ExpectedEvidenceContext            ExpectedEvidenceKind = "context"
	ExpectedEvidenceOutcome            ExpectedEvidenceKind = "outcome"
	ExpectedEvidenceVerification       ExpectedEvidenceKind = "verification"
	ExpectedEvidenceUsefulnessFeedback ExpectedEvidenceKind = "usefulness_feedback"
	ExpectedEvidenceTaskEvaluation     ExpectedEvidenceKind = "task_evaluation"
	ExpectedEvidenceProof              ExpectedEvidenceKind = "proof"
	ExpectedEvidenceRepair             ExpectedEvidenceKind = "repair"
	ExpectedEvidenceRankingRollout     ExpectedEvidenceKind = "ranking_rollout"
)

func (k ExpectedEvidenceKind) Valid() bool {
	switch k {
	case ExpectedEvidenceSession, ExpectedEvidenceContext, ExpectedEvidenceOutcome, ExpectedEvidenceVerification,
		ExpectedEvidenceUsefulnessFeedback, ExpectedEvidenceTaskEvaluation, ExpectedEvidenceProof,
		ExpectedEvidenceRepair, ExpectedEvidenceRankingRollout:
		return true
	default:
		return false
	}
}

type MissingEvidenceCategory string

const (
	MissingEvidenceSessionWithoutOutcome         MissingEvidenceCategory = "session_without_outcome"
	MissingEvidenceTurnWithoutContext            MissingEvidenceCategory = "turn_without_context"
	MissingEvidenceVerificationMissing           MissingEvidenceCategory = "verification_missing"
	MissingEvidenceFeedbackWithoutSubject        MissingEvidenceCategory = "feedback_without_subject"
	MissingEvidenceTaskEvaluationMissingEvidence MissingEvidenceCategory = "task_evaluation_missing_evidence"
	MissingEvidenceRepairWithoutVerification     MissingEvidenceCategory = "repair_without_verification"
	MissingEvidenceRolloutWithoutDryRun          MissingEvidenceCategory = "rollout_without_dry_run"
	MissingEvidenceOutOfScope                    MissingEvidenceCategory = "out_of_scope"
	MissingEvidenceStale                         MissingEvidenceCategory = "stale"
	MissingEvidenceOpaqueOnly                    MissingEvidenceCategory = "opaque_only"
	MissingEvidenceContradictory                 MissingEvidenceCategory = "contradictory"
	MissingEvidenceHidden                        MissingEvidenceCategory = "hidden"
)

func (c MissingEvidenceCategory) Valid() bool {
	switch c {
	case MissingEvidenceSessionWithoutOutcome, MissingEvidenceTurnWithoutContext, MissingEvidenceVerificationMissing,
		MissingEvidenceFeedbackWithoutSubject, MissingEvidenceTaskEvaluationMissingEvidence,
		MissingEvidenceRepairWithoutVerification, MissingEvidenceRolloutWithoutDryRun, MissingEvidenceOutOfScope,
		MissingEvidenceStale, MissingEvidenceOpaqueOnly, MissingEvidenceContradictory, MissingEvidenceHidden:
		return true
	default:
		return false
	}
}

type ConformanceResult string

const (
	ConformanceResultPassed   ConformanceResult = "passed"
	ConformanceResultDegraded ConformanceResult = "degraded"
	ConformanceResultFailed   ConformanceResult = "failed"
	ConformanceResultUnknown  ConformanceResult = "unknown"
)

func (r ConformanceResult) Valid() bool {
	switch r {
	case ConformanceResultPassed, ConformanceResultDegraded, ConformanceResultFailed, ConformanceResultUnknown:
		return true
	default:
		return false
	}
}

type ConformanceProfileStatus string

const (
	ConformanceProfileStatusActive   ConformanceProfileStatus = "active"
	ConformanceProfileStatusDisabled ConformanceProfileStatus = "disabled"
)

func (s ConformanceProfileStatus) Valid() bool {
	switch s {
	case ConformanceProfileStatusActive, ConformanceProfileStatusDisabled:
		return true
	default:
		return false
	}
}

type ExpectedEvidence struct {
	Kind            ExpectedEvidenceKind `json:"kind"`
	MinimumCount    int                  `json:"minimum_count"`
	FreshnessWindow time.Duration        `json:"freshness_window"`
}

func (e ExpectedEvidence) Validate() error {
	if !e.Kind.Valid() {
		return fmt.Errorf("evidence kind %q is invalid", e.Kind)
	}
	if e.MinimumCount <= 0 {
		return fmt.Errorf("minimum evidence count must be greater than zero")
	}
	if e.FreshnessWindow < minFreshnessWindow {
		return fmt.Errorf("freshness window must be at least %s", minFreshnessWindow)
	}
	return nil
}

type ConformanceProfile struct {
	ID               string                   `json:"id"`
	Scope            memory.Scope             `json:"scope"`
	Status           ConformanceProfileStatus `json:"status"`
	ExpectedEvidence []ExpectedEvidence       `json:"expected_evidence"`
	Actor            string                   `json:"actor"`
	Reason           string                   `json:"reason"`
	CreatedAt        time.Time                `json:"created_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
	DisabledAt       time.Time                `json:"disabled_at,omitempty"`
}

func (p ConformanceProfile) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("conformance profile id is required")
	}
	if err := p.Scope.Validate(); err != nil {
		return err
	}
	if !p.Status.Valid() {
		return fmt.Errorf("conformance profile status %q is invalid", p.Status)
	}
	if len(p.ExpectedEvidence) == 0 {
		return fmt.Errorf("expected evidence is required")
	}
	for _, evidence := range p.ExpectedEvidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
	}
	if err := validateActorReason(p.Actor, p.Reason); err != nil {
		return err
	}
	if p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		return fmt.Errorf("created at and updated at are required")
	}
	return nil
}

type AlertAdapterKind string

const (
	AlertAdapterDisabled AlertAdapterKind = "disabled"
	AlertAdapterStdout   AlertAdapterKind = "stdout"
	AlertAdapterWebhook  AlertAdapterKind = "webhook"
)

func (k AlertAdapterKind) Valid() bool {
	switch k {
	case AlertAdapterDisabled, AlertAdapterStdout, AlertAdapterWebhook:
		return true
	default:
		return false
	}
}

type AlertDeliveryResult string

const (
	AlertDeliveryResultDisabled AlertDeliveryResult = "disabled"
	AlertDeliveryResultSkipped  AlertDeliveryResult = "skipped"
	AlertDeliveryResultSuccess  AlertDeliveryResult = "success"
	AlertDeliveryResultRetry    AlertDeliveryResult = "retry"
	AlertDeliveryResultFailed   AlertDeliveryResult = "failed"
)

func (r AlertDeliveryResult) Valid() bool {
	switch r {
	case AlertDeliveryResultDisabled, AlertDeliveryResultSkipped, AlertDeliveryResultSuccess,
		AlertDeliveryResultRetry, AlertDeliveryResultFailed:
		return true
	default:
		return false
	}
}

type OperationalProofTarget string

const (
	OperationalProofCapacityLoad  OperationalProofTarget = "capacity_load"
	OperationalProofBackupRestore OperationalProofTarget = "backup_restore"
)

func (t OperationalProofTarget) Valid() bool {
	switch t {
	case OperationalProofCapacityLoad, OperationalProofBackupRestore:
		return true
	default:
		return false
	}
}

type RecoveryVerificationTarget string

const (
	RecoveryVerificationTargetIncident            RecoveryVerificationTarget = "incident"
	RecoveryVerificationTargetAlertCandidate      RecoveryVerificationTarget = "alert_candidate"
	RecoveryVerificationTargetConformanceRun      RecoveryVerificationTarget = "conformance_run"
	RecoveryVerificationTargetRepairResult        RecoveryVerificationTarget = "repair_result"
	RecoveryVerificationTargetRankingRollback     RecoveryVerificationTarget = "ranking_rollback"
	RecoveryVerificationTargetProofRun            RecoveryVerificationTarget = "proof_run"
	RecoveryVerificationTargetSessionVerification RecoveryVerificationTarget = "session_verification"
	RecoveryVerificationTargetCapacityLoadProof   RecoveryVerificationTarget = "capacity_load_proof"
	RecoveryVerificationTargetBackupRestoreProof  RecoveryVerificationTarget = "backup_restore_proof"
)

func (t RecoveryVerificationTarget) Valid() bool {
	switch t {
	case RecoveryVerificationTargetIncident, RecoveryVerificationTargetAlertCandidate,
		RecoveryVerificationTargetConformanceRun, RecoveryVerificationTargetRepairResult,
		RecoveryVerificationTargetRankingRollback, RecoveryVerificationTargetProofRun,
		RecoveryVerificationTargetSessionVerification, RecoveryVerificationTargetCapacityLoadProof,
		RecoveryVerificationTargetBackupRestoreProof:
		return true
	default:
		return false
	}
}

type RetentionClass string

const (
	RetentionClassDiagnostic RetentionClass = "diagnostic"
	RetentionClassAudit      RetentionClass = "audit"
)

func (c RetentionClass) Valid() bool {
	switch c {
	case RetentionClassDiagnostic, RetentionClassAudit:
		return true
	default:
		return false
	}
}

type RunbookHintCategory string

const (
	RunbookHintReviewBacklog            RunbookHintCategory = "review_backlog"
	RunbookHintReviewRepair             RunbookHintCategory = "review_repair"
	RunbookHintReviewConformanceProfile RunbookHintCategory = "review_conformance_profile"
	RunbookHintReviewCapacityProof      RunbookHintCategory = "review_capacity_proof"
	RunbookHintReviewBackupRestoreProof RunbookHintCategory = "review_backup_restore_proof"
	RunbookHintReviewAlertDelivery      RunbookHintCategory = "review_alert_delivery"
)

func (c RunbookHintCategory) Valid() bool {
	switch c {
	case RunbookHintReviewBacklog, RunbookHintReviewRepair, RunbookHintReviewConformanceProfile,
		RunbookHintReviewCapacityProof, RunbookHintReviewBackupRestoreProof, RunbookHintReviewAlertDelivery:
		return true
	default:
		return false
	}
}

type AlertDeliveryConfig struct {
	Mode               AlertAdapterKind
	WebhookURL         string
	WebhookHeaders     map[string]string
	AllowInsecureLocal bool
	Timeout            time.Duration
	MaxPayloadBytes    int
}

func (c AlertDeliveryConfig) Validate() error {
	if c.Mode == "" {
		c.Mode = AlertAdapterDisabled
	}
	if !c.Mode.Valid() {
		return fmt.Errorf("alert delivery mode %q is invalid", c.Mode)
	}
	if c.Timeout == 0 {
		c.Timeout = defaultDeliveryTimeout
	}
	if c.Timeout < minDeliveryTimeout {
		return fmt.Errorf("alert delivery timeout must be at least %s", minDeliveryTimeout)
	}
	if c.MaxPayloadBytes == 0 {
		c.MaxPayloadBytes = defaultWebhookMaxBytes
	}
	if c.MaxPayloadBytes <= 0 || c.MaxPayloadBytes > defaultWebhookMaxBytes {
		return fmt.Errorf("alert payload limit must be between 1 and %d bytes", defaultWebhookMaxBytes)
	}
	if c.Mode != AlertAdapterWebhook {
		return nil
	}
	return validateWebhook(c)
}

type ReadHealthEvaluationInput struct {
	Scope        memory.Scope
	EvaluationID string
}

func (i ReadHealthEvaluationInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.EvaluationID) == "" {
		return fmt.Errorf("health evaluation id is required")
	}
	return nil
}

type ListIncidentsInput struct {
	Scope  memory.Scope
	Status IncidentStatus
	Limit  int
}

func (i ListIncidentsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Status != "" && !i.Status.Valid() {
		return fmt.Errorf("incident status %q is invalid", i.Status)
	}
	return nil
}

type ReadIncidentInput struct {
	Scope      memory.Scope
	IncidentID string
}

func (i ReadIncidentInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.IncidentID, "incident id")
}

type ReadAlertCandidateInput struct {
	Scope            memory.Scope
	AlertCandidateID string
}

func (i ReadAlertCandidateInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.AlertCandidateID, "alert candidate id")
}

type ListAlertDeliveryAttemptsInput struct {
	Scope            memory.Scope
	AlertCandidateID string
}

func (i ListAlertDeliveryAttemptsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.AlertCandidateID, "alert candidate id")
}

type ReadConformanceProfileInput struct {
	Scope     memory.Scope
	ProfileID string
}

func (i ReadConformanceProfileInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.ProfileID, "conformance profile id")
}

type ListConformanceProfilesInput struct {
	Scope  memory.Scope
	Status ConformanceProfileStatus
}

func (i ListConformanceProfilesInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if i.Status != "" && !i.Status.Valid() {
		return fmt.Errorf("conformance profile status %q is invalid", i.Status)
	}
	return nil
}

type ReadConformanceRunInput struct {
	Scope memory.Scope
	RunID string
}

func (i ReadConformanceRunInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.RunID, "conformance run id")
}

type ListConformanceRunsInput struct {
	Scope     memory.Scope
	ProfileID string
}

func (i ListConformanceRunsInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.ProfileID) != "" {
		return validateID(i.ProfileID, "conformance profile id")
	}
	return nil
}

type ReadOperationalProofInput struct {
	Scope   memory.Scope
	ProofID string
}

func (i ReadOperationalProofInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.ProofID, "operational proof id")
}

type ReadReadinessReportInput struct {
	Scope    memory.Scope
	ReportID string
}

func (i ReadReadinessReportInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.ReportID, "readiness report id")
}

type ReadRecoveryVerificationInput struct {
	Scope    memory.Scope
	RecordID string
}

func (i ReadRecoveryVerificationInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateID(i.RecordID, "recovery verification id")
}

func validateWebhook(c AlertDeliveryConfig) error {
	if strings.TrimSpace(c.WebhookURL) == "" {
		return fmt.Errorf("webhook URL is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(c.WebhookURL))
	if err != nil {
		return fmt.Errorf("webhook URL is invalid: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("webhook URL scheme %q is invalid", parsed.Scheme)
	}
	host := parsed.Hostname()
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("webhook host is required")
	}
	if parsed.Scheme == "http" && !c.AllowInsecureLocal {
		return fmt.Errorf("unsafe webhook target: http requires explicit local override")
	}
	if isUnsafeWebhookHost(host) {
		return fmt.Errorf("unsafe webhook target: %s", host)
	}
	for key := range c.WebhookHeaders {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "" {
			return fmt.Errorf("webhook header name is required")
		}
		if normalized == "host" || strings.HasPrefix(normalized, "x-forwarded-") {
			return fmt.Errorf("webhook header %q is not allowed", key)
		}
	}
	return nil
}

func validateID(id string, label string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s is required", label)
	}
	if len(id) > maxIdentifierLength {
		return fmt.Errorf("%s must be at most %d bytes", label, maxIdentifierLength)
	}
	return nil
}

func validateDeduplicationKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("deduplication key is required")
	}
	if len(key) > maxDeduplicationKeyLength {
		return fmt.Errorf("deduplication key must be at most %d bytes", maxDeduplicationKeyLength)
	}
	return nil
}

func ValidateIdempotencyKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("idempotency key is required")
	}
	if len(key) > maxIdempotencyKeyLength {
		return fmt.Errorf("idempotency key must be at most %d bytes", maxIdempotencyKeyLength)
	}
	return nil
}

func isUnsafeWebhookHost(host string) bool {
	normalized := strings.ToLower(strings.Trim(host, "[]"))
	if normalized == "localhost" || strings.HasSuffix(normalized, ".localhost") {
		return false
	}
	ip := net.ParseIP(normalized)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return false
	}
	return ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified()
}

func validateActorReason(actor string, reason string) error {
	if strings.TrimSpace(actor) == "" {
		return fmt.Errorf("actor is required")
	}
	if len(actor) > maxActorLength {
		return fmt.Errorf("actor must be at most %d bytes", maxActorLength)
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if len(reason) > maxReasonLength {
		return fmt.Errorf("reason must be at most %d bytes", maxReasonLength)
	}
	return nil
}

func validateMetadata(metadata map[string]any, label string) error {
	if len(metadata) > maxMetadataEntries {
		return fmt.Errorf("%s must contain at most %d entries", label, maxMetadataEntries)
	}
	for key, value := range metadata {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%s key is required", label)
		}
		if len(key) > maxMetadataKeyLength {
			return fmt.Errorf("%s key must be at most %d bytes", label, maxMetadataKeyLength)
		}
		if len(fmt.Sprint(value)) > maxMetadataValueLength {
			return fmt.Errorf("%s value must be at most %d bytes", label, maxMetadataValueLength)
		}
	}
	return nil
}
