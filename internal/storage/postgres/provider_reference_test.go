package postgres

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

var referenceTestScope = memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}

const referenceTestEvent = "10000000-0000-0000-0000-000000000001"

func referenceRows(visible, processed, failed, exhausted, complete bool) *pgxmock.Rows {
	return pgxmock.NewRows([]string{"event_id", "reference_id", "memory_path", "event_type", "visible", "processed", "failed", "exhausted", "complete", "candidate_id", "memory_id", "version_id", "provenance_id", "version", "value"}).AddRow(referenceTestEvent, "task-1", "tasks/demo", "provider.reference.task", visible, processed, failed, exhausted, complete, "20000000-0000-0000-0000-000000000002", "30000000-0000-0000-0000-000000000003", "40000000-0000-0000-0000-000000000004", "50000000-0000-0000-0000-000000000005", int64(2), "current value")
}

func TestRepositoryReferenceStatusGovernanceAndVisibility(t *testing.T) {
	for _, tc := range []struct {
		name                                            string
		visible, processed, failed, exhausted, complete bool
		status                                          string
		retry                                           bool
	}{
		{"pending", true, false, false, false, false, provider.ReferenceStatusPending, true},
		{"durable chain before processing commit", true, false, false, false, true, provider.ReferenceStatusPending, true},
		{"retryable failure", true, false, true, false, false, provider.ReferenceStatusFailed, true},
		{"terminal failure", true, false, true, true, false, provider.ReferenceStatusFailed, false},
		{"completed", true, true, false, false, true, provider.ReferenceStatusCompleted, false},
		{"missing durable completion", true, true, false, false, false, provider.ReferenceStatusUnavailable, false},
		{"hidden completion", false, true, false, false, true, provider.ReferenceStatusUnavailable, false},
		{"hidden failure", false, false, true, true, false, provider.ReferenceStatusUnavailable, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			mock.ExpectQuery(regexp.QuoteMeta(referenceStatusQuery)).WithArgs("t", "p", "n", referenceTestEvent).WillReturnRows(referenceRows(tc.visible, tc.processed, tc.failed, tc.exhausted, tc.complete))
			got, err := NewRepository(mock).ReferenceStatus(context.Background(), referenceTestScope, "principal", referenceTestEvent, "memory-content-v1", "bounded_content")
			if err != nil || got.Status != tc.status || got.Retryable != tc.retry {
				t.Fatalf("got=%+v err=%v", got, err)
			}
			if tc.status == provider.ReferenceStatusCompleted {
				if got.EventID != "" {
					t.Fatalf("completed event identity must be in lineage only: %+v", got)
				}
				if got.Reference == nil || got.Reference.Value != "current value" || got.Reference.Version != 2 {
					t.Fatalf("completion=%+v", got)
				}
			} else if got.Reference != nil {
				t.Fatalf("incomplete reference leaked: %+v", got)
			}
			if tc.status == provider.ReferenceStatusUnavailable && (got.EventID != "" || got.ErrorCategory != "") {
				t.Fatalf("hidden state leaked: %+v", got)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRepositoryReferenceLookupUsesExactSelectorAndOmittedContent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectQuery(regexp.QuoteMeta(referenceLookupQuery)).WithArgs("t", "p", "n", "task-1", "tasks/demo").WillReturnRows(referenceRows(true, true, false, false, true))
	got, err := NewRepository(mock).ReferenceLookup(context.Background(), referenceTestScope, "principal", "task-1", "/tasks/demo/", "memory-reference-v1", "reference_only")
	if err != nil || got.Status != provider.ReferenceStatusCompleted || got.Reference == nil || got.Reference.Value != "" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReferenceRecordRejectsMalformedOrOversizedDurableIdentity(t *testing.T) {
	base := provider.ReferenceRecord{ReferenceID: "task-1", MemoryPath: "tasks/demo", EventType: "provider.reference.task", EventID: referenceTestEvent, CandidateID: referenceTestEvent, MemoryID: referenceTestEvent, VersionID: referenceTestEvent, ProvenanceID: referenceTestEvent, Version: 1, Value: "value"}
	for _, tc := range []struct {
		name   string
		mutate func(*provider.ReferenceRecord)
	}{
		{"nil UUID", func(r *provider.ReferenceRecord) { r.VersionID = "00000000-0000-0000-0000-000000000000" }},
		{"noncanonical UUID", func(r *provider.ReferenceRecord) { r.EventID = "10000000000000000000000000000001" }},
		{"invalid reference token", func(r *provider.ReferenceRecord) { r.ReferenceID = "task!" }},
		{"oversized value", func(r *provider.ReferenceRecord) { r.Value = strings.Repeat("x", 4097) }},
		{"unnormalized path", func(r *provider.ReferenceRecord) { r.MemoryPath = "/tasks/demo/" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			tc.mutate(&r)
			if validReferenceRecord(r) {
				t.Fatalf("invalid record accepted: %+v", r)
			}
		})
	}
}

func TestRepositoryReferenceMissingAndDependencyFailure(t *testing.T) {
	for _, failure := range []error{pgx.ErrNoRows, errors.New("database unavailable")} {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectQuery(regexp.QuoteMeta(referenceStatusQuery)).WithArgs("t", "p", "n", referenceTestEvent).WillReturnError(failure)
		got, err := NewRepository(mock).ReferenceStatus(context.Background(), referenceTestScope, "principal", referenceTestEvent, "memory-reference-v1", "reference_only")
		if errors.Is(failure, pgx.ErrNoRows) {
			if err != nil || got.Status != provider.ReferenceStatusUnavailable || got.EventID != "" {
				t.Fatalf("missing=%+v err=%v", got, err)
			}
		} else if err == nil {
			t.Fatal("dependency returned success")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		mock.Close()
	}
}

func TestReferenceLookupSelectsNewestBeforeDisclosureAndLifecycle(t *testing.T) {
	split := strings.Index(referenceLookupQuery, "SELECT e.id::text")
	if split < 0 {
		t.Fatal("missing final projection")
	}
	selector := referenceLookupQuery[:split]
	for _, want := range []string{"WITH selected AS MATERIALIZED", "tenant=$1", "project=$2", "namespace=$3", "metadata->>'reference_id'=$4", "memory_path=$5", "ORDER BY created_at DESC, id DESC", "LIMIT 1", "reference_contract_version"} {
		if !strings.Contains(selector, want) {
			t.Errorf("selector missing %s", want)
		}
	}
	for _, forbidden := range []string{"classification", "state='active'", "status='promoted'"} {
		if strings.Contains(selector, forbidden) {
			t.Errorf("selector filters %s before newest identity", forbidden)
		}
	}
	for _, want := range []string{"ORDER BY version DESC", "v.memory_path=e.memory_path", "c.memory_path=e.memory_path", "m.memory_path=e.memory_path", "p.operation='promote_candidate'", "c.source_raw_event_id=e.id", "v.state='active'", "m.state='active'", "c.status='promoted'"} {
		if !strings.Contains(referenceLookupQuery, want) {
			t.Errorf("completion missing %s", want)
		}
	}
}
