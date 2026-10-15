package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func expectCandidatePromotionLock(mock pgxmock.PgxPoolIface, input governance.CanonicalPromotion, existing any) {
	if existing == nil {
		existing = ""
	}
	mock.ExpectQuery("SELECT[\\s\\S]*c.status[\\s\\S]*FROM candidate_memories c[\\s\\S]*FOR UPDATE").
		WithArgs(input.Candidate.ID, input.Candidate.Scope.Tenant, input.Candidate.Scope.Project, input.Candidate.Scope.Namespace, input.Candidate.SourceRawEventID).
		WillReturnRows(pgxmock.NewRows([]string{"status", "memory_id"}).AddRow(input.Candidate.Status, existing))
}

func expectCandidatePromotionStatus(mock pgxmock.PgxPoolIface, input governance.CanonicalPromotion) {
	mock.ExpectExec("UPDATE candidate_memories SET status='promoted'").
		WithArgs(input.Candidate.ID, input.CreatedAt, input.Candidate.Scope.Tenant, input.Candidate.Scope.Project, input.Candidate.Scope.Namespace).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
}

func TestPromotionCandidateLockFailureRollsBackWithoutCanonicalWrite(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	input := governance.CanonicalPromotion{Candidate: governance.CandidateMemory{ID: "candidate", SourceRawEventID: "event", Scope: scope, MemoryPath: "tasks/run"}, MemoryID: "memory", VersionID: "version", CreatedAt: time.Now().UTC()}
	failure := errors.New("candidate lock unavailable")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT[\\s\\S]*c.status[\\s\\S]*FROM candidate_memories c[\\s\\S]*FOR UPDATE").WithArgs(input.Candidate.ID, scope.Tenant, scope.Project, scope.Namespace, input.Candidate.SourceRawEventID).WillReturnError(failure)
	mock.ExpectRollback()
	_, _, err = NewRepository(mock).PromoteCandidate(context.Background(), input)
	if !errors.Is(err, failure) {
		t.Fatalf("promotion error=%v; want candidate lock failure", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPromotionRetryReturnsCommittedLineageWithoutReactivatingLifecycle(t *testing.T) {
	for _, state := range []memory.MemoryState{memory.MemoryStateActive, memory.MemoryStateSuppressed, memory.MemoryStateForgotten, memory.MemoryStateDeleted} {
		for _, status := range []governance.CandidateStatus{governance.CandidateStatusPending, governance.CandidateStatusPromoted} {
			t.Run(string(state)+"/"+string(status), func(t *testing.T) {
				mock, err := pgxmock.NewPool()
				if err != nil {
					t.Fatal(err)
				}
				defer mock.Close()
				now := time.Now().UTC()
				scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
				input := governance.CanonicalPromotion{Candidate: governance.CandidateMemory{ID: "candidate", SourceRawEventID: "event", Scope: scope, MemoryPath: "tasks/run", Status: status}, MemoryID: "retry-must-not-create-memory", VersionID: "retry-must-not-create-version", CreatedAt: now}
				mock.ExpectBegin()
				expectCandidatePromotionLock(mock, input, "committed-memory")
				mock.ExpectQuery("SELECT[\\s\\S]*FROM canonical_memories c JOIN memory_versions v[\\s\\S]*v.modified_by=\\$5").
					WithArgs("committed-memory", scope.Tenant, scope.Project, scope.Namespace, input.Candidate.ID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "tenant", "project", "namespace", "memory_path", "class", "state", "content", "created_at", "updated_at", "temporal_fact_id", "ingested_at", "valid_from", "valid_to", "validity_source", "source_metadata",
						"version_id", "memory_id", "version_memory_path", "version", "version_state", "version_content", "version_created_at", "modified_by", "version_temporal_fact_id", "version_ingested_at", "version_valid_from", "version_valid_to", "version_validity_source", "version_source_metadata",
					}).AddRow(
						"committed-memory", scope.Tenant, scope.Project, scope.Namespace, "tasks/run", memory.MemoryClassEpisodic, state, "current content", now, now, "fact", now, now, nil, "legacy_current_compatible", memory.SourceMetadata{}.Effective(),
						"committed-version", "committed-memory", "tasks/run", int64(1), memory.MemoryStateActive, "original content", now, "candidate", "fact", now, now, nil, "legacy_current_compatible", memory.SourceMetadata{}.Effective(),
					))
				if status == governance.CandidateStatusPending {
					expectCandidatePromotionStatus(mock, input)
				}
				mock.ExpectCommit()
				canonical, version, err := NewRepository(mock).PromoteCandidate(context.Background(), input)
				if err != nil {
					t.Fatal(err)
				}
				if canonical.ID != "committed-memory" || version.ID != "committed-version" || canonical.State != state || canonical.MemoryPath != "tasks/run" || version.MemoryPath != "tasks/run" {
					t.Fatalf("recovery canonical=%+v version=%+v", canonical, version)
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestPromotionStatusFailureRollsBackCanonicalVersionAndProvenance(t *testing.T) {
	for _, previousVersion := range []int64{0, 1} {
		t.Run(string(rune('0'+previousVersion)), func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()
			now := time.Now().UTC()
			scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
			input := governance.CanonicalPromotion{Candidate: governance.CandidateMemory{ID: "candidate", SourceRawEventID: "event", Scope: scope, MemoryPath: "tasks/run", Class: memory.MemoryClassSummary, Content: "summary", Status: governance.CandidateStatusPending}, MemoryID: "memory", VersionID: "version", CreatedAt: now}
			failure := errors.New("candidate status unavailable")
			mock.ExpectBegin()
			expectCandidatePromotionLock(mock, input, nil)
			mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM memory_versions").WithArgs(input.MemoryID).WillReturnRows(pgxmock.NewRows([]string{"coalesce"}).AddRow(previousVersion))
			canonicalQuery := "INSERT INTO canonical_memories"
			canonicalArgCount := 16
			if previousVersion > 0 {
				canonicalQuery = "UPDATE canonical_memories"
				canonicalArgCount = 10
			}
			mock.ExpectQuery(canonicalQuery).WithArgs(promotionTestAnyArgs(canonicalArgCount)...).WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "memory_path", "class", "state", "content", "created_at", "updated_at", "temporal_fact_id", "ingested_at", "valid_from", "valid_to", "validity_source", "source_metadata"}).AddRow(input.MemoryID, scope.Tenant, scope.Project, scope.Namespace, input.Candidate.MemoryPath, input.Candidate.Class, memory.MemoryStateActive, input.Candidate.Content, now, now, "", time.Time{}, time.Time{}, nil, "", memory.SourceMetadata{}.Effective()))
			mock.ExpectQuery("INSERT INTO memory_versions").WithArgs(promotionTestAnyArgs(13)...).WillReturnRows(pgxmock.NewRows([]string{"id", "memory_id", "memory_path", "version", "state", "content", "created_at", "modified_by", "temporal_fact_id", "ingested_at", "valid_from", "valid_to", "validity_source", "source_metadata"}).AddRow(input.VersionID, input.MemoryID, input.Candidate.MemoryPath, previousVersion+1, memory.MemoryStateActive, input.Candidate.Content, now, input.Candidate.ID, "", time.Time{}, time.Time{}, nil, "", memory.SourceMetadata{}.Effective()))
			mock.ExpectExec("INSERT INTO provenance_links").WithArgs(promotionTestAnyArgs(12)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
			mock.ExpectExec("UPDATE candidate_memories SET status='promoted'").WithArgs(input.Candidate.ID, now, scope.Tenant, scope.Project, scope.Namespace).WillReturnError(failure)
			mock.ExpectRollback()
			_, _, err = NewRepository(mock).PromoteCandidate(context.Background(), input)
			if !errors.Is(err, failure) {
				t.Fatalf("promotion error=%v; want candidate-status failure", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func promotionTestAnyArgs(count int) []any {
	args := make([]any, count)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}
