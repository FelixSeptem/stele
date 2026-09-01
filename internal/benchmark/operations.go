package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type CapacityBudget struct {
	MaxBytes   int64 `json:"max_bytes,omitempty"`
	MaxEvents  int   `json:"max_events,omitempty"`
	MaxQueries int   `json:"max_queries,omitempty"`
}

type CapacityReport struct {
	Status         Status   `json:"status"`
	EstimatedBytes int64    `json:"estimated_bytes"`
	EventCount     int      `json:"event_count"`
	QueryCount     int      `json:"query_count"`
	Diagnostics    []string `json:"diagnostics,omitempty"`
}

// CheckBenchmarkCapacity performs a deterministic local preflight before a
// corpus import. It does not inspect or mutate PostgreSQL and never reports
// host-specific paths beyond the caller-provided cache root.
func CheckBenchmarkCapacity(dataDir string, corpus NormalizedCorpus, budget CapacityBudget) (CapacityReport, error) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return CapacityReport{Status: StatusPrerequisiteMissing, Diagnostics: []string{"benchmark data directory is required"}}, &StatusError{Status: StatusPrerequisiteMissing, Message: "benchmark data directory is required"}
	}
	if _, err := os.Stat(dataDir); err != nil {
		return CapacityReport{Status: StatusPrerequisiteMissing, Diagnostics: []string{"benchmark data directory is unavailable"}}, &StatusError{Status: StatusPrerequisiteMissing, Message: "benchmark data directory is unavailable", Cause: err}
	}
	if err := corpus.Validate(); err != nil {
		return CapacityReport{Status: StatusInvalidManifest, Diagnostics: []string{"normalized corpus validation failed"}}, &StatusError{Status: StatusInvalidManifest, Message: "validate normalized corpus", Cause: err}
	}
	encoded, err := json.Marshal(corpus)
	if err != nil {
		return CapacityReport{Status: StatusInternalError, Diagnostics: []string{"estimate normalized corpus size failed"}}, err
	}
	report := CapacityReport{Status: StatusSuccess, EstimatedBytes: int64(len(encoded)), EventCount: len(corpus.Events), QueryCount: len(corpus.Queries)}
	if budget.MaxBytes > 0 && report.EstimatedBytes > budget.MaxBytes {
		report.Status = StatusCapacityRefused
		report.Diagnostics = append(report.Diagnostics, fmt.Sprintf("estimated corpus size %d bytes exceeds budget %d bytes", report.EstimatedBytes, budget.MaxBytes))
	}
	if budget.MaxEvents > 0 && report.EventCount > budget.MaxEvents {
		report.Status = StatusCapacityRefused
		report.Diagnostics = append(report.Diagnostics, fmt.Sprintf("event count %d exceeds budget %d", report.EventCount, budget.MaxEvents))
	}
	if budget.MaxQueries > 0 && report.QueryCount > budget.MaxQueries {
		report.Status = StatusCapacityRefused
		report.Diagnostics = append(report.Diagnostics, fmt.Sprintf("query count %d exceeds budget %d", report.QueryCount, budget.MaxQueries))
	}
	if report.Status == StatusCapacityRefused {
		return report, &StatusError{Status: StatusCapacityRefused, Message: "benchmark capacity budget refused"}
	}
	return report, nil
}

func PlanBenchmarkBatches(events []MemoryEventRecord, batchSize, maxEvents int) ([][]MemoryEventRecord, error) {
	if batchSize <= 0 {
		return nil, &StatusError{Status: StatusCapacityRefused, Message: "benchmark batch size must be greater than zero"}
	}
	if maxEvents > 0 && len(events) > maxEvents {
		return nil, &StatusError{Status: StatusCapacityRefused, Message: "benchmark event count exceeds import limit"}
	}
	if len(events) == 0 {
		return [][]MemoryEventRecord{}, nil
	}
	batches := make([][]MemoryEventRecord, 0, (len(events)+batchSize-1)/batchSize)
	for start := 0; start < len(events); start += batchSize {
		end := start + batchSize
		if end > len(events) {
			end = len(events)
		}
		batch := append([]MemoryEventRecord(nil), events[start:end]...)
		batches = append(batches, batch)
	}
	return batches, nil
}
