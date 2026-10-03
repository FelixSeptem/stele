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

// SchedulerRunEvent carries only fixed categories or buckets. It deliberately
// has no scope, run id, attempt id, raw error, payload, or reason fields.
type SchedulerRunEvent struct {
	Operation      string
	Result         string
	State          string
	Lease          string
	Retry          string
	Recovery       string
	Record         string
	Freshness      string
	SLO            string
	DurationBucket string
}

// DerivedWorkEvent contains only fixed queue lifecycle categories. It is safe
// to emit from workers without exposing work keys, scope values or payloads.
type DerivedWorkEvent struct {
	Operation string
	Result    string
	Kind      string
	State     string
	Retry     string
	Loss      string
	Freshness string
	SLO       string
}

func LogSchedulerRunLifecycle(logger *log.Logger, event SchedulerRunEvent) {
	if logger == nil {
		return
	}
	logger.Printf("component=scheduler_run operation=%s result=%s state=%s lease=%s retry=%s recovery=%s record=%s freshness=%s slo=%s duration_bucket=%s",
		boundedSchedulerLabel(event.Operation, "dispatch", "lease", "retry", "recovery", "duplicate", "checkpoint", "terminal", "retention_cleanup", "admin_inspection"),
		boundedSchedulerLabel(event.Result, "accepted", "acquired", "renewed", "reclaimed", "retrying", "exhausted", "completed", "cancelled", "duplicate", "skipped", "deleted", "failure", "denied"),
		boundedSchedulerLabel(event.State, "pending", "running", "retrying", "recovered", "completed", "failed", "duplicate", "skipped", "exhausted", "cancelled"),
		boundedSchedulerLabel(event.Lease, "acquired", "renewed", "reclaimed", "conflict", "none"),
		boundedSchedulerLabel(event.Retry, "none", "eligible", "backoff", "exhausted"),
		boundedSchedulerLabel(event.Recovery, "none", "reclaimed", "resumed", "manual_review", "cancelled"),
		boundedSchedulerLabel(event.Record, "run_summary", "attempt", "attempt_detail", "admin_page"),
		boundedSchedulerLabel(event.Freshness, "fresh", "stale", "divergent", "missing", "unknown"),
		boundedSchedulerLabel(event.SLO, "within_budget", "over_budget", "unknown"),
		boundedSchedulerLabel(event.DurationBucket, "lt_1s", "1s_10s", "gt_10s", "unknown"),
	)
}

func boundedSchedulerLabel(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return candidate
		}
	}
	return "unknown"
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

// ProgressiveExperimentEvent carries only fixed-category experiment health.
// It intentionally has no scope, query, source, identifier, DSN, or score
// fields, so callers cannot accidentally turn experiment telemetry into a
// high-cardinality or sensitive channel.
type ProgressiveExperimentEvent struct {
	Level       string
	Strategy    string
	Mode        string
	Result      string
	Freshness   string
	Fallback    string
	Budget      string
	Rollback    string
	Eligibility string
}

// ReasoningInsightEvent carries only bounded categories for provider-backed
// insight derivation and replay. It deliberately excludes scope, prompts,
// candidate identifiers, source content, provider payloads, and raw errors.
type ReasoningInsightEvent struct {
	Operation           string
	Mode                string
	InsightType         string
	Result              string
	Eligibility         string
	Freshness           string
	Fallback            string
	Duration            string
	TemporalDisposition string
	Review              string
}

// MemoryIntentEvent is deliberately category-only. It never accepts scope,
// payload, request identifiers, claims, credentials, or provider errors.
type MemoryIntentEvent struct {
	Operation string
	Type      string
	Status    string
	Outcome   string
	Retry     string
	Rollback  string
}

func LogMemoryIntentLifecycle(logger *log.Logger, event MemoryIntentEvent) {
	if logger == nil {
		return
	}
	logger.Printf("component=memory_intent operation=%s type=%s status=%s outcome=%s retry=%s rollback=%s",
		boundedIntentLabel(event.Operation, "submit", "replay", "queue", "outcome", "retry", "rollback", "inspect"),
		boundedIntentLabel(event.Type, "remember", "update", "forget", "contradiction", "feedback"),
		boundedIntentLabel(event.Status, "pending", "accepted", "candidate", "active", "suppressed", "rejected", "failed", "replayed"),
		boundedIntentLabel(event.Outcome, "accepted", "rejected", "suppressed", "failed", "scope_denied", "target_stale", "evidence_incomplete", "policy_disabled", "retry_exhausted", "rolled_back"),
		boundedIntentLabel(event.Retry, "none", "eligible", "backoff", "exhausted"),
		boundedIntentLabel(event.Rollback, "none", "disabled", "held", "resumed"),
	)
}

// IntentConformanceEvent carries only the fixed vocabulary used by the
// product-verification report. It deliberately excludes scopes, identifiers,
// payloads, credentials, DSNs, and raw dependency errors.
type IntentConformanceEvent struct {
	Operation       string
	Phase           string
	Result          string
	Prerequisite    string
	Recovery        string
	Duration        string
	FailureCategory string
	Rollback        string
	Consumable      string
}

func LogIntentConformance(logger *log.Logger, event IntentConformanceEvent) {
	if logger == nil {
		return
	}
	logger.Printf("component=intent_conformance operation=%s phase=%s result=%s prerequisite=%s recovery=%s duration_bucket=%s failure_category=%s rollback=%s consumable=%s",
		boundedConformanceLabel(event.Operation, "run", "phase", "replay", "inspect", "cleanup", "redaction"),
		boundedConformanceLabel(event.Phase, "prerequisite", "submission", "replay", "scope_isolation", "inspection", "queue_recovery", "worker_restart", "scheduler_restart", "rollback", "cleanup"),
		boundedConformanceLabel(event.Result, "pass", "skip", "degraded", "fail"),
		boundedConformanceLabel(event.Prerequisite, "available", "missing", "incompatible", "unknown"),
		boundedConformanceLabel(event.Recovery, "none", "reclaimed", "retried", "exhausted", "degraded", "unknown"),
		boundedConformanceLabel(event.Duration, "lt_1s", "1s_10s", "gt_10s", "unknown"),
		boundedConformanceLabel(event.FailureCategory, "none", "validation", "scope_denied", "conflict", "timeout", "dependency", "hidden_evidence", "unsafe_retry", "cleanup", "unknown"),
		boundedConformanceLabel(event.Rollback, "none", "held", "resumed", "failed", "unknown"),
		boundedConformanceLabel(event.Consumable, "yes", "no", "unknown"),
	)
}

func boundedConformanceLabel(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return candidate
		}
	}
	return "unknown"
}

func boundedIntentLabel(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return candidate
		}
	}
	return "unknown"
}

func LogReasoningInsightLifecycle(logger *log.Logger, event ReasoningInsightEvent) {
	if logger == nil {
		return
	}
	logger.Printf("component=reasoning_insight operation=%s mode=%s insight_type=%s result=%s eligibility=%s freshness=%s fallback=%s duration_bucket=%s temporal_disposition=%s review=%s",
		boundedReasoningLabel(event.Operation, "derive", "replay", "shadow", "handoff", "rollback"),
		boundedReasoningLabel(event.Mode, "offline", "shadow", "apply"),
		boundedReasoningLabel(event.InsightType, "hypothesis", "goal", "contradiction", "causal_link", "unknown"),
		boundedReasoningLabel(event.Result, "candidate", "would_activate", "rejected", "quarantined", "stale", "fallback", "completed", "failed"),
		boundedReasoningLabel(event.Eligibility, "eligible", "ineligible", "disabled", "unknown"),
		boundedReasoningLabel(event.Freshness, "fresh", "stale", "missing", "unknown"),
		boundedReasoningLabel(event.Fallback, "none", "provider", "budget", "compatibility", "validation", "unknown"),
		boundedReasoningLabel(event.Duration, "lt_1s", "1s_10s", "gt_10s", "unknown"),
		boundedReasoningLabel(event.TemporalDisposition, "contradiction", "temporal_coexistence", "unresolved_temporal", "unknown"),
		boundedReasoningLabel(event.Review, "required", "confirmed", "coexists", "incorrect", "stale", "unknown"),
	)
}

func boundedReasoningLabel(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return candidate
		}
	}
	return "unknown"
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

// ReleaseEvidenceOperationalEvent contains only fixed lifecycle categories.
// It is safe for metrics and structured operator logs.
type ReleaseEvidenceOperationalEvent struct {
	Operation      string
	Result         string
	State          string
	Cleanup        string
	Freshness      string
	Rollback       string
	DurationBucket string
}

func LogReleaseEvidenceOperationalLifecycle(logger *log.Logger, event ReleaseEvidenceOperationalEvent) {
	if logger == nil {
		return
	}
	logger.Printf("component=retrieval_release_evidence operation=%s result=%s state=%s cleanup=%s freshness=%s rollback=%s duration_bucket=%s",
		boundedReleaseEvidenceLabel(event.Operation, "preflight", "run", "cleanup", "attestation", "disablement", "rollback"),
		boundedReleaseEvidenceLabel(event.Result, "accepted", "completed", "skipped", "degraded", "failed", "mismatch", "deleted"),
		boundedReleaseEvidenceLabel(event.State, "skipped", "degraded", "failed", "timed_out", "completed"),
		boundedReleaseEvidenceLabel(event.Cleanup, "pending", "complete", "incomplete"),
		boundedReleaseEvidenceLabel(event.Freshness, "fresh", "stale", "unknown"),
		boundedReleaseEvidenceLabel(event.Rollback, "passed", "failed", "unknown"),
		boundedReleaseEvidenceLabel(event.DurationBucket, "lt_1s", "1s_10s", "gt_10s", "unknown"),
	)
}

func boundedReleaseEvidenceLabel(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return candidate
		}
	}
	return "unknown"
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
