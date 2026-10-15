package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestSummaryPersistsNormalizedPathOnCanonicalAndVersion(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	input := governance.SummaryMemoryRecord{MemoryID: "memory", VersionID: "version", Scope: scope, MemoryPath: "tasks/run-1", Content: "summary", EvidenceRawEventIDs: []string{"event"}, CreatedAt: now}
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO canonical_memories").WithArgs(input.MemoryID, scope.Tenant, scope.Project, scope.Namespace, input.MemoryPath, memory.MemoryClassSummary, memory.MemoryStateActive, policy.RetentionClassDurable, input.Content, now, now).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "memory_path", "class", "state", "content", "created_at", "updated_at", "source_metadata"}).AddRow(input.MemoryID, scope.Tenant, scope.Project, scope.Namespace, input.MemoryPath, memory.MemoryClassSummary, memory.MemoryStateActive, input.Content, now, now, memory.SourceMetadata{}.Effective()))
	mock.ExpectQuery("INSERT INTO memory_versions").WithArgs(input.VersionID, input.MemoryID, input.MemoryPath, int64(1), memory.MemoryStateActive, input.Content, now, "summary_compactor").
		WillReturnRows(pgxmock.NewRows([]string{"id", "memory_id", "memory_path", "version", "state", "content", "created_at", "modified_by", "source_metadata"}).AddRow(input.VersionID, input.MemoryID, input.MemoryPath, int64(1), memory.MemoryStateActive, input.Content, now, "summary_compactor", memory.SourceMetadata{}.Effective()))
	mock.ExpectExec("INSERT INTO provenance_links").WithArgs(summaryProvenanceID(input.VersionID, 0), "event", nil, input.MemoryID, scope.Tenant, scope.Project, scope.Namespace, "create_summary_memory", nil, nil, pgxmock.AnyArg(), now).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	canonical, version, err := NewRepository(mock).CreateSummaryMemory(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if canonical.MemoryPath != input.MemoryPath || version.MemoryPath != input.MemoryPath {
		t.Fatalf("canonical path=%q version path=%q", canonical.MemoryPath, version.MemoryPath)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConsolidationRepositoryLooksUpOnlyExactNormalizedPath(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	repo, ok := any(NewRepository(mock)).(governance.CanonicalPathRepository)
	if !ok {
		t.Fatal("repository does not support exact-path consolidation lookup")
	}
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	now := time.Now().UTC()
	mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories[\\s\\S]*tenant = \\$1[\\s\\S]*project = \\$2[\\s\\S]*namespace = \\$3[\\s\\S]*memory_path = \\$4[\\s\\S]*class = \\$5[\\s\\S]*state = \\$6").WithArgs(scope.Tenant, scope.Project, scope.Namespace, "tasks/run-1", memory.MemoryClassProfile, memory.MemoryStateActive).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "memory_path", "class", "state", "content", "created_at", "updated_at", "temporal_fact_id", "ingested_at", "valid_from", "valid_to", "validity_source"}).AddRow("memory", scope.Tenant, scope.Project, scope.Namespace, "tasks/run-1", memory.MemoryClassProfile, memory.MemoryStateActive, "old profile", now, now, "fact", now, now, nil, "legacy_current_compatible"))
	canonical, found, err := repo.GetLatestCanonicalByScopePathAndClass(context.Background(), scope, "/tasks/run-1/", memory.MemoryClassProfile)
	if err != nil || !found || canonical.MemoryPath != "tasks/run-1" {
		t.Fatalf("canonical=%+v found=%v err=%v", canonical, found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimedReferenceLoadsDurableAttributionWithinExactScope(t *testing.T) {
	for _, kind := range []string{"provider.reference.task", "provider.reference.run", "provider.reference.handoff", "provider.reference.compilation"} {
		t.Run(kind, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			now := time.Now().UTC()
			input := governance.ClaimPendingRawEventsInput{WorkerID: "worker", BatchSize: 1, LeaseDuration: time.Minute, Now: now}
			mock.ExpectQuery("WITH claimed AS").WithArgs(input.WorkerID, now, now.Add(input.LeaseDuration), input.BatchSize).
				WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "memory_path", "event_type", "content", "source_timestamp", "created_at", "governance_worker_id", "governance_claimed_at", "governance_lease_until", "governance_attempt", "source_metadata"}).AddRow("event", "tenant", "project", "namespace", "tasks/run-1", kind, "artifact", now, now, input.WorkerID, now, now.Add(input.LeaseDuration), 1, memory.SourceMetadata{}.Effective()))
			mock.ExpectQuery("SELECT metadata FROM raw_events WHERE id=\\$1 AND tenant=\\$2 AND project=\\$3 AND namespace=\\$4").WithArgs("event", "tenant", "project", "namespace").
				WillReturnRows(pgxmock.NewRows([]string{"metadata"}).AddRow([]byte(`{"reference_contract_version":"durable-reference-v1","reference_id":"ref-1"}`)))
			claims, err := NewRepository(mock).ClaimPendingRawEvents(context.Background(), input)
			if err != nil || len(claims) != 1 {
				t.Fatalf("claims=%+v err=%v", claims, err)
			}
			event := claims[0].Event
			if event.MemoryPath != "tasks/run-1" || event.Metadata["reference_contract_version"] != memory.DurableReferenceVersion || event.Metadata["reference_id"] != "ref-1" {
				t.Fatalf("durable attribution lost: %+v", event)
			}
			candidates, err := (governance.RuleBasedExtractor{}).Extract(context.Background(), event)
			if err != nil || len(candidates) != 1 {
				t.Fatalf("claimed reference not promotable: %+v %v", candidates, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
