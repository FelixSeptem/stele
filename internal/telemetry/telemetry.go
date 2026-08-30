package telemetry

import (
	"context"
	"time"
)

type OperationEvent struct {
	Mode       string
	Component  string
	Operation  string
	Status     string
	Count      int
	Duration   time.Duration
	Error      string
	ObservedAt time.Time
}

type BacklogEvent struct {
	Mode       string
	Component  string
	Queue      string
	Status     string
	Pending    int64
	Leased     int64
	Processed  int64
	OldestAge  time.Duration
	Error      string
	ObservedAt time.Time
}

// RetrievalEvaluationEvent is intentionally low-cardinality. It contains no scope,
// query, memory, source, credential, DSN, or raw error payload.
type RetrievalEvaluationEvent struct {
	Status          string
	FixtureVersion  string
	RankingVersion  string
	PolicyVersion   string
	FailureCategory string
	Decision        string
	CaseCount       int
	Duration        time.Duration
	Tenant          string
	Query           string
	MemoryID        string
	Error           string
}

type Observer interface {
	RecordOperation(ctx context.Context, event OperationEvent)
	RecordBacklog(ctx context.Context, event BacklogEvent)
}

type noopObserver struct{}

func (noopObserver) RecordOperation(ctx context.Context, event OperationEvent) {}

func (noopObserver) RecordBacklog(ctx context.Context, event BacklogEvent) {}

func (noopObserver) RecordRetrievalEvaluation(ctx context.Context, event RetrievalEvaluationEvent) {}

func NoopObserver() Observer {
	return noopObserver{}
}
