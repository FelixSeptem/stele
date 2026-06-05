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
	Tenant    string
	Project   string
	Namespace string
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
	ID              string
	Scope           Scope
	EventType       string
	Content         string
	Metadata        map[string]any
	SourceTimestamp time.Time
	CreatedAt       time.Time
}

type CanonicalMemory struct {
	ID         string
	Scope      Scope
	Class      MemoryClass
	State      MemoryState
	Content    string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

type MemoryVersion struct {
	ID         string
	MemoryID   string
	Version    int64
	State      MemoryState
	Content    string
	CreatedAt  time.Time
	ModifiedBy string
}

type ProvenanceRecord struct {
	ID                string
	Scope             Scope
	RawEventID        string
	CandidateMemoryID string
	MemoryID          string
	RequestID         string
	Actor             string
	Operation         string
	CreatedAt         time.Time
	SourceContext     map[string]any
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
