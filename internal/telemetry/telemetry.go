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

type MaintenanceEvent struct {
	JobClass        string
	Outcome         string
	LeaseOutcome    string
	Freshness       string
	SLO             string
	LatencyBucket   string
	CandidateBucket string
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
	// Deprecated compatibility fields are never exported or used by metrics.
	// New callers must leave them empty.
	Tenant   string
	Query    string
	MemoryID string
	Error    string
}

// RetrievalPlannerEvent carries only bounded planner execution categories.
// It intentionally excludes query text, scope values, identifiers, provider
// payloads, credentials, and raw scores.
type RetrievalPlannerEvent struct {
	PlannerVersion  string
	PolicyVersion   string
	Family          string
	Stage           string
	Disposition     string
	Pass            int
	BudgetBucket    string
	Evidence        string
	Fallback        string
	LatencyBucket   string
	Reranker        string
	GraphHopBucket  string
	GraphPathBucket string
	GraphTruncation string
	GraphFailure    string
}

type RetrievalPlannerChannelEvent struct {
	PlannerVersion string
	PolicyVersion  string
	Family         string
	Stage          string
	Channel        string
	Availability   string
}

type RetrievalPlannerChangedRankEvent struct {
	PlannerVersion string
	PolicyVersion  string
	Family         string
	Stage          string
	Bucket         string
	Count          int
}

type RetrievalPlannerDiagnosticEvent struct {
	FailureCategory string
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
