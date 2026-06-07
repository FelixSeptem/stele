package memory

import (
	"context"
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
	ID              string         `json:"id"`
	Scope           Scope          `json:"scope"`
	EventType       string         `json:"event_type"`
	Content         string         `json:"content"`
	Metadata        map[string]any `json:"metadata"`
	SourceTimestamp time.Time      `json:"source_timestamp"`
	CreatedAt       time.Time      `json:"created_at"`
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
