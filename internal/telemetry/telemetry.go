package telemetry

import (
	"context"
	"log"
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

type RetrievalIntegrityEvent struct {
	Operation        string
	Result           string
	Component        string
	Verdict          string
	FindingCategory  string
	ArtifactCategory string
}

// LogRetrievalIntegrityLifecycle records one bounded lifecycle entry for the
// evaluation-only evidence path. It deliberately accepts no scope, query,
// identifier, provider payload, credential, or free-form reason fields.
func LogRetrievalIntegrityLifecycle(logger *log.Logger, event RetrievalIntegrityEvent) {
	if logger == nil {
		return
	}
	logger.Printf(
		"component=retrieval_integrity operation=%s result=%s subject=%s verdict=%s finding_category=%s artifact_category=%s",
		retrievalIntegrityOperation(event.Operation),
		retrievalIntegrityResult(event.Result),
		retrievalIntegrityComponent(event.Component),
		retrievalIntegrityVerdict(event.Verdict),
		retrievalIntegrityFinding(event.FindingCategory),
		retrievalIntegrityArtifact(event.ArtifactCategory),
	)
}

func retrievalIntegrityOperation(value string) string {
	return retrievalIntegrityLabel(value, "collect", "evaluate", "replay", "cleanup")
}

func retrievalIntegrityResult(value string) string {
	return retrievalIntegrityLabel(value, "completed", "degraded", "failed", "skipped", "deleted")
}

func retrievalIntegrityComponent(value string) string {
	return retrievalIntegrityLabel(value, "trajectory", "integrity", "retention")
}

func retrievalIntegrityVerdict(value string) string {
	return retrievalIntegrityLabel(value, "passed", "degraded", "skipped", "rejected")
}

func retrievalIntegrityFinding(value string) string {
	return retrievalIntegrityLabel(value, "expected-recall", "missing", "altered", "unexpected-duplicate", "misplaced", "foreign-scope", "hidden-lifecycle", "stale-watermark", "nondeterministic", "rollback-incomplete")
}

func retrievalIntegrityArtifact(value string) string {
	return retrievalIntegrityLabel(value, "trajectory", "integrity_report", "retention_outcome")
}

func retrievalIntegrityLabel(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return candidate
		}
	}
	return "unknown"
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
