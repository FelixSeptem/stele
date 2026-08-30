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
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse observed_at: %w", err)
	}
	return parsed, nil
}
