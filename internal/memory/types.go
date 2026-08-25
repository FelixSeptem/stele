package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type MemoryClass string

const (
	MemoryClassProfile    MemoryClass = "profile"
	MemoryClassEpisodic   MemoryClass = "episodic"
	MemoryClassProcedural MemoryClass = "procedural"
	MemoryClassSummary    MemoryClass = "summary"
	MemoryClassRelation   MemoryClass = "relation"
)

type MemoryState string

const (
	MemoryStateCandidate  MemoryState = "candidate"
	MemoryStateActive     MemoryState = "active"
	MemoryStateSuppressed MemoryState = "suppressed"
	MemoryStateForgotten  MemoryState = "forgotten"
	MemoryStateDeleted    MemoryState = "deleted"
)

type Scope struct {
	Tenant    string `json:"tenant"`
	Project   string `json:"project"`
	Namespace string `json:"namespace"`
}

func (s Scope) Normalized() Scope {
	return Scope{
		Tenant:    strings.TrimSpace(s.Tenant),
		Project:   strings.TrimSpace(s.Project),
		Namespace: strings.TrimSpace(s.Namespace),
	}
}

func (s Scope) Validate() error {
	normalized := s.Normalized()
	switch {
	case normalized.Tenant == "":
		return fmt.Errorf("tenant is required")
	case normalized.Project == "":
		return fmt.Errorf("project is required")
	case normalized.Namespace == "":
		return fmt.Errorf("namespace is required")
	default:
		return nil
	}
}

type RawEvent struct {
	ID              string                   `json:"id"`
	Scope           Scope                    `json:"scope"`
	EventType       string                   `json:"event_type"`
	Content         string                   `json:"content"`
	Metadata        map[string]any           `json:"metadata"`
	SourceTimestamp time.Time                `json:"source_timestamp"`
	CreatedAt       time.Time                `json:"created_at"`
	Admission       *AdmissionPressureReport `json:"admission,omitempty"`
}

type CanonicalMemory struct {
	ID         string      `json:"id"`
	Scope      Scope       `json:"scope"`
	Class      MemoryClass `json:"class"`
	State      MemoryState `json:"state"`
	Content    string      `json:"content"`
	CreatedAt  time.Time   `json:"created_at"`
	ModifiedAt time.Time   `json:"modified_at"`
}

type MemoryVersion struct {
	ID         string      `json:"id"`
	MemoryID   string      `json:"memory_id"`
	Version    int64       `json:"version"`
	State      MemoryState `json:"state"`
	Content    string      `json:"content"`
	CreatedAt  time.Time   `json:"created_at"`
	ModifiedBy string      `json:"modified_by"`
}

type EmbeddingRebuildStatus string

const (
	EmbeddingRebuildStatusPending    EmbeddingRebuildStatus = "pending"
	EmbeddingRebuildStatusRebuilding EmbeddingRebuildStatus = "rebuilding"
	EmbeddingRebuildStatusFailed     EmbeddingRebuildStatus = "failed"
	EmbeddingRebuildStatusCurrent    EmbeddingRebuildStatus = "current"
)

type VectorRevisionStatus string

const (
	VectorRevisionStatusGenerated  VectorRevisionStatus = "generated"
	VectorRevisionStatusActive     VectorRevisionStatus = "active"
	VectorRevisionStatusSuperseded VectorRevisionStatus = "superseded"
	VectorRevisionStatusFailed     VectorRevisionStatus = "failed"
)

type EmbeddingRebuildRecord struct {
	MemoryID             string
	Scope                Scope
	Class                MemoryClass
	Content              string
	SourceVersion        int64
	ContentHash          string
	RequestedProvider    string
	RequestedModel       string
	RequestedDimensions  int
	Status               EmbeddingRebuildStatus
	FailureReason        string
	RequestedAt          time.Time
	LastAttemptedAt      time.Time
	ActiveVectorRevision string
}

type EmbeddingLifecycleCandidate struct {
	MemoryID             string
	Scope                Scope
	Class                MemoryClass
	CurrentSourceVersion int64
	CurrentContentHash   string
	RebuildStatus        EmbeddingRebuildStatus
	RequestedProvider    string
	RequestedModel       string
	RequestedDimensions  int
	ActiveVectorRevision string
	ActiveProvider       string
	ActiveModel          string
	ActiveDimensions     int
}

type VectorRevision struct {
	ID                 string
	MemoryID           string
	Scope              Scope
	SourceVersion      int64
	ContentHash        string
	Provider           string
	Model              string
	Dimensions         int
	Embedding          []float32
	Status             VectorRevisionStatus
	FailureReason      string
	SupersededBy       string
	GeneratedAt        time.Time
	ActivatedAt        time.Time
	LastRebuildRequest time.Time
}

type ProvenanceRecord struct {
	ID                string         `json:"id"`
	Scope             Scope          `json:"scope"`
	RawEventID        string         `json:"raw_event_id"`
	CandidateMemoryID string         `json:"candidate_memory_id"`
	MemoryID          string         `json:"memory_id"`
	RequestID         string         `json:"request_id"`
	Actor             string         `json:"actor"`
	Operation         string         `json:"operation"`
	CreatedAt         time.Time      `json:"created_at"`
	SourceContext     map[string]any `json:"source_context"`
}

type MemoryHistory struct {
	Memory     CanonicalMemory    `json:"memory"`
	Versions   []MemoryVersion    `json:"versions"`
	Provenance []ProvenanceRecord `json:"provenance"`
}

type IngestEventInput struct {
	Scope           Scope
	EventType       string
	Content         string
	Metadata        map[string]any
	SourceTimestamp time.Time
}

func (i IngestEventInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(i.EventType) == "" {
		return fmt.Errorf("event type is required")
	}

	if strings.TrimSpace(i.Content) == "" {
		return fmt.Errorf("content is required")
	}

	return nil
}

type EventIngestor interface {
	Ingest(ctx context.Context, input IngestEventInput) (RawEvent, error)
}

var (
	ErrIdempotencyConflict   = fmt.Errorf("idempotency key conflicts with an existing request")
	ErrIdempotencyInProgress = fmt.Errorf("idempotency request is in progress")
)

type IdempotentEventIngestInput struct {
	PrincipalID        string
	IdempotencyKey     string
	RequestFingerprint string
}

func (i IdempotentEventIngestInput) Validate() error {
	if strings.TrimSpace(i.PrincipalID) == "" {
		return fmt.Errorf("principal id is required")
	}
	if strings.TrimSpace(i.IdempotencyKey) == "" || len(i.IdempotencyKey) > 256 {
		return fmt.Errorf("idempotency key is required and must be at most 256 bytes")
	}
	if strings.TrimSpace(i.RequestFingerprint) == "" {
		return fmt.Errorf("request fingerprint is required")
	}
	return nil
}

type IdempotentEventIngestResult struct {
	Event    RawEvent
	Replayed bool
}

type IdempotentEventIngestor interface {
	IngestIdempotent(ctx context.Context, input IngestEventInput, principalID, idempotencyKey string) (IdempotentEventIngestResult, error)
}

func EventRequestFingerprint(input IngestEventInput) (string, error) {
	payload := struct {
		EventType       string         `json:"event_type"`
		Content         string         `json:"content"`
		Metadata        map[string]any `json:"metadata,omitempty"`
		SourceTimestamp string         `json:"source_timestamp,omitempty"`
	}{
		EventType: strings.TrimSpace(input.EventType), Content: input.Content, Metadata: input.Metadata,
	}
	if !input.SourceTimestamp.IsZero() {
		payload.SourceTimestamp = input.SourceTimestamp.UTC().Format(time.RFC3339Nano)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal event request fingerprint: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

type TaskEvidenceTargetKind string

const (
	TaskEvidenceTargetSession            TaskEvidenceTargetKind = "session"
	TaskEvidenceTargetTurn               TaskEvidenceTargetKind = "turn"
	TaskEvidenceTargetRawEvent           TaskEvidenceTargetKind = "raw_event"
	TaskEvidenceTargetOutcomeEvent       TaskEvidenceTargetKind = "outcome_event"
	TaskEvidenceTargetVerification       TaskEvidenceTargetKind = "verification"
	TaskEvidenceTargetExpectedRecall     TaskEvidenceTargetKind = "expected_recall"
	TaskEvidenceTargetUsefulnessFeedback TaskEvidenceTargetKind = "usefulness_feedback"
	TaskEvidenceTargetContextCitation    TaskEvidenceTargetKind = "context_citation"
	TaskEvidenceTargetDerivedInsight     TaskEvidenceTargetKind = "derived_insight"
	TaskEvidenceTargetMemory             TaskEvidenceTargetKind = "memory"
	TaskEvidenceTargetQualityFinding     TaskEvidenceTargetKind = "quality_finding"
	TaskEvidenceTargetRepairPlan         TaskEvidenceTargetKind = "repair_plan"
	TaskEvidenceTargetOpaque             TaskEvidenceTargetKind = "opaque"
)

func (k TaskEvidenceTargetKind) Valid() bool {
	switch k {
	case TaskEvidenceTargetSession,
		TaskEvidenceTargetTurn,
		TaskEvidenceTargetRawEvent,
		TaskEvidenceTargetOutcomeEvent,
		TaskEvidenceTargetVerification,
		TaskEvidenceTargetExpectedRecall,
		TaskEvidenceTargetUsefulnessFeedback,
		TaskEvidenceTargetContextCitation,
		TaskEvidenceTargetDerivedInsight,
		TaskEvidenceTargetMemory,
		TaskEvidenceTargetQualityFinding,
		TaskEvidenceTargetRepairPlan,
		TaskEvidenceTargetOpaque:
		return true
	default:
		return false
	}
}
