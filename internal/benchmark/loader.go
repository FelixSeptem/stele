package benchmark

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type BenchmarkRunScope struct {
	ID      string       `json:"id"`
	Dataset string       `json:"dataset"`
	Scope   memory.Scope `json:"scope"`
}

func NewRunScope(base memory.Scope, dataset, runID string) (BenchmarkRunScope, error) {
	if err := base.Validate(); err != nil {
		return BenchmarkRunScope{}, err
	}
	dataset = strings.TrimSpace(dataset)
	runID = strings.TrimSpace(runID)
	if dataset == "" || runID == "" {
		return BenchmarkRunScope{}, fmt.Errorf("benchmark dataset and run id are required")
	}
	digest := sha256.Sum256([]byte(dataset + "\x00" + runID))
	suffix := hex.EncodeToString(digest[:6])
	return BenchmarkRunScope{ID: runID, Dataset: dataset, Scope: memory.Scope{Tenant: base.Tenant, Project: "benchmark-" + benchmarkIdentifier(dataset), Namespace: "run-" + benchmarkIdentifier(runID) + "-" + suffix}}, nil
}

func benchmarkIdentifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
			builder.WriteRune(character)
			continue
		}
		builder.WriteByte('-')
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "benchmark"
	}
	return result
}

type EvidenceMapping struct {
	EvidenceID   string       `json:"evidence_id"`
	RawEventID   string       `json:"raw_event_id"`
	SourceTurnID string       `json:"source_turn_id"`
	SessionID    string       `json:"session_id"`
	Scope        memory.Scope `json:"scope"`
}

type CorpusLoader struct {
	ingestor memory.EventIngestor
}

func NewCorpusLoader(ingestor memory.EventIngestor) CorpusLoader {
	return CorpusLoader{ingestor: ingestor}
}

// LongMemEvalImport is the audit boundary between normalized LongMemEval
// inputs and Stele's ingestion path. Its run scope is always freshly derived
// from the caller tenant; it cannot retain a production project or namespace.
type LongMemEvalImport struct {
	Run      BenchmarkRunScope          `json:"run"`
	Evidence map[string]EvidenceMapping `json:"evidence"`
}

type LongMemEvalImporter struct {
	loader CorpusLoader
}

// BenchmarkScopeCleaner is implemented by the database-backed benchmark
// runtime. It intentionally accepts only a derived benchmark scope so cache
// cleanup can never request deletion of a caller's production scope.
type BenchmarkScopeCleaner interface {
	CleanBenchmarkScope(context.Context, memory.Scope) error
}

type LongMemEvalCleanupResult struct {
	DatabaseScopeCleaned bool             `json:"database_scope_cleaned"`
	Artifacts            RunCleanupResult `json:"artifacts"`
}

type LongMemEvalRunCleaner struct {
	cache        Cache
	scopeCleaner BenchmarkScopeCleaner
}

func NewLongMemEvalRunCleaner(cache Cache, scopeCleaner BenchmarkScopeCleaner) LongMemEvalRunCleaner {
	return LongMemEvalRunCleaner{cache: cache, scopeCleaner: scopeCleaner}
}

func NewLongMemEvalImporter(ingestor memory.EventIngestor) LongMemEvalImporter {
	return LongMemEvalImporter{loader: NewCorpusLoader(ingestor)}
}

func (i LongMemEvalImporter) Import(ctx context.Context, base memory.Scope, runID string, corpus NormalizedCorpus) (LongMemEvalImport, error) {
	run, err := NewRunScope(base, "longmemeval", runID)
	if err != nil {
		return LongMemEvalImport{}, fmt.Errorf("create LongMemEval benchmark scope: %w", err)
	}
	evidence, err := i.loader.Load(ctx, run, corpus)
	if err != nil {
		return LongMemEvalImport{}, fmt.Errorf("load LongMemEval benchmark corpus: %w", err)
	}
	return LongMemEvalImport{Run: run, Evidence: evidence}, nil
}

func (c LongMemEvalRunCleaner) Clean(ctx context.Context, manifest DatasetManifest, run BenchmarkRunScope, retainReport bool) (LongMemEvalCleanupResult, error) {
	if c.scopeCleaner == nil {
		return LongMemEvalCleanupResult{}, fmt.Errorf("benchmark database scope cleaner is required")
	}
	if run.Dataset != "longmemeval" || run.Scope.Project != "benchmark-longmemeval" || !strings.HasPrefix(run.Scope.Namespace, "run-") {
		return LongMemEvalCleanupResult{}, &StatusError{Status: StatusInvalidManifest, Message: "LongMemEval cleanup requires a derived benchmark run scope"}
	}
	if manifest.Name != "longmemeval" {
		return LongMemEvalCleanupResult{}, &StatusError{Status: StatusInvalidManifest, Message: "LongMemEval cleanup requires a LongMemEval manifest"}
	}
	if err := c.scopeCleaner.CleanBenchmarkScope(ctx, run.Scope); err != nil {
		return LongMemEvalCleanupResult{}, fmt.Errorf("clean LongMemEval database scope: %w", err)
	}
	artifacts, err := c.cache.CleanRunArtifacts(manifest, run.ID, retainReport)
	if err != nil {
		return LongMemEvalCleanupResult{}, fmt.Errorf("clean LongMemEval run artifacts: %w", err)
	}
	return LongMemEvalCleanupResult{DatabaseScopeCleaned: true, Artifacts: artifacts}, nil
}

func (l CorpusLoader) Load(ctx context.Context, run BenchmarkRunScope, corpus NormalizedCorpus) (map[string]EvidenceMapping, error) {
	if l.ingestor == nil {
		return nil, fmt.Errorf("benchmark event ingestor is required")
	}
	if err := run.Scope.Validate(); err != nil {
		return nil, err
	}
	if err := corpus.Validate(); err != nil {
		return nil, err
	}
	mappings := make(map[string]EvidenceMapping, len(corpus.Events))
	for _, event := range corpus.Events {
		observedAt, err := parseObservedAt(event.ObservedAt)
		if err != nil {
			return nil, fmt.Errorf("event %s: %w", event.ID, err)
		}
		input := memory.IngestEventInput{Scope: run.Scope, EventType: "benchmark." + string(event.Class), Content: event.Text, SourceTimestamp: observedAt, Metadata: map[string]any{"benchmark_dataset": run.Dataset, "benchmark_run_id": run.ID, "benchmark_event_id": event.ID, "source_turn_id": event.SourceTurnID, "session_id": event.SessionID, "expected_lifecycle": event.ExpectedState}}
		var rawEvent memory.RawEvent
		if idempotent, ok := l.ingestor.(memory.IdempotentEventIngestor); ok {
			result, err := idempotent.IngestIdempotent(ctx, input, "benchmark:"+run.ID, "benchmark/"+run.Dataset+"/"+run.ID+"/"+event.ID)
			if err != nil {
				return nil, fmt.Errorf("ingest benchmark event %s: %w", event.ID, err)
			}
			rawEvent = result.Event
		} else {
			var err error
			rawEvent, err = l.ingestor.Ingest(ctx, input)
			if err != nil {
				return nil, fmt.Errorf("ingest benchmark event %s: %w", event.ID, err)
			}
		}
		mappings[event.ID] = EvidenceMapping{EvidenceID: event.ID, RawEventID: rawEvent.ID, SourceTurnID: event.SourceTurnID, SessionID: event.SessionID, Scope: run.Scope}
	}
	return mappings, nil
}

func parseObservedAt(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006/01/02 (Mon) 15:04"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse observed_at %q: unsupported timestamp format", value)
}
