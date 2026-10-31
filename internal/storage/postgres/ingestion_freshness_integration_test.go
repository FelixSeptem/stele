package postgres

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
)

func TestIngestionFreshnessPostgresInitialAndRestartEvidence(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("STELE_TEST_POSTGRES_FRESHNESS_DSN"))
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_FRESHNESS_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	now := time.Now().UTC().Truncate(time.Microsecond)
	scope := memory.Scope{Tenant: "freshness-" + uuid.NewString(), Project: "assurance", Namespace: "scope-" + uuid.NewString()}
	eventID, candidateID, memoryID, provenanceID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	labels := `'{"version":"source-metadata-v1","trust":"untrusted","classification":"medium"}'::jsonb`
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO raw_events(id,tenant,project,namespace,event_type,content,source_metadata,governance_processed_at,created_at) VALUES($1,$2,$3,$4,'conversation.message','bounded freshness fixture',` + labels + `,$5,$5)`, []any{eventID, scope.Tenant, scope.Project, scope.Namespace, now}},
		{`INSERT INTO candidate_memories(id,source_raw_event_id,tenant,project,namespace,class,content,confidence,importance,freshness,sensitivity,mutability,retention_class,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'episodic','bounded freshness fixture',0.9,0.5,1,'medium','immutable','durable','promoted',$6,$6)`, []any{candidateID, eventID, scope.Tenant, scope.Project, scope.Namespace, now.Add(time.Second)}},
		{`INSERT INTO canonical_memories(id,tenant,project,namespace,class,state,content,source_metadata,created_at,updated_at) VALUES($1,$2,$3,$4,'episodic','active','bounded freshness fixture',` + labels + `,$5,$5)`, []any{memoryID, scope.Tenant, scope.Project, scope.Namespace, now.Add(2 * time.Second)}},
		{`INSERT INTO memory_versions(id,memory_id,version,state,content,source_metadata,created_at,modified_by) VALUES($1,$2,1,'active','bounded freshness fixture',` + labels + `,$3,'freshness-integration-test')`, []any{uuid.NewString(), memoryID, now.Add(2 * time.Second)}},
		{`INSERT INTO provenance_links(id,raw_event_id,candidate_memory_id,memory_id,tenant,project,namespace,operation,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,'promote_candidate',$8)`, []any{provenanceID, eventID, candidateID, memoryID, scope.Tenant, scope.Project, scope.Namespace, now.Add(2 * time.Second)}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("create freshness fixture: %v", err)
		}
	}
	repo := NewRepository(pool)
	if err := repo.RecordProviderMemoryRead(ctx, scope, []string{memoryID}, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	initialInput, err := repo.CollectIngestionFreshness(ctx, scope, eventID, now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	service := assurance.NewService(assurance.ServiceOptions{IngestionFreshnessCollector: repo, IngestionFreshnessEvidenceStore: repo, Now: func() time.Time { return now.Add(4 * time.Second) }})
	initial, err := service.InspectIngestionFreshness(ctx, assurance.IngestionFreshnessInspectionInput{Scope: scope, FixtureReference: eventID, ObservedAt: now.Add(4 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if initial.Result != assurance.IngestionFreshnessResultFresh || len(initial.Stages) != 4 || initial.Stages[3].Disposition != assurance.IngestionDispositionComplete {
		t.Fatalf("initial freshness report=%+v stages=%+v", initial, initialInput.Stages)
	}
	restartedRepo := NewRepository(pool)
	restartedService := assurance.NewService(assurance.ServiceOptions{IngestionFreshnessCollector: restartedRepo, IngestionFreshnessEvidenceStore: restartedRepo, Now: func() time.Time { return now.Add(4 * time.Second) }})
	restarted, err := restartedService.InspectIngestionFreshness(ctx, assurance.IngestionFreshnessInspectionInput{Scope: scope, FixtureReference: eventID, ObservedAt: now.Add(4 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if restarted.IdentityHash != initial.IdentityHash || restarted.FixtureHash != initial.FixtureHash {
		t.Fatalf("restart changed evidence identity: initial=%+v restarted=%+v", initial, restarted)
	}
	serialized, err := json.Marshal(initial)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{scope.Tenant, scope.Project, scope.Namespace, eventID, memoryID, "bounded freshness fixture"} {
		if strings.Contains(string(serialized), forbidden) {
			t.Fatalf("redacted report contains %q: %s", forbidden, serialized)
		}
	}
	if path := strings.TrimSpace(os.Getenv("STELE_FRESHNESS_EVIDENCE_PATH")); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(serialized, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var evidenceCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM assurance_ingestion_freshness_evidence WHERE tenant=$1 AND project=$2 AND namespace=$3 AND fixture_hash=$4`, scope.Tenant, scope.Project, scope.Namespace, initial.FixtureHash).Scan(&evidenceCount); err != nil {
		t.Fatal(err)
	}
	if evidenceCount != 1 {
		t.Fatalf("durable evidence count=%d, want one idempotent report", evidenceCount)
	}
}

func TestIngestionFreshnessPostgresNegativeLifecycleEvidence(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("STELE_TEST_POSTGRES_FRESHNESS_DSN"))
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_FRESHNESS_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := NewRepository(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)

	t.Run("pending governance never claims a canonical or public read", func(t *testing.T) {
		scope := memory.Scope{Tenant: "freshness-pending-" + uuid.NewString(), Project: "assurance", Namespace: "scope-" + uuid.NewString()}
		eventID := uuid.NewString()
		labels := `'{"version":"source-metadata-v1","trust":"untrusted","classification":"medium"}'::jsonb`
		if _, err := pool.Exec(ctx, `INSERT INTO raw_events(id,tenant,project,namespace,event_type,content,source_metadata,created_at) VALUES($1,$2,$3,$4,'conversation.message','pending freshness fixture',`+labels+`,$5)`, eventID, scope.Tenant, scope.Project, scope.Namespace, now); err != nil {
			t.Fatal(err)
		}
		input, err := repo.CollectIngestionFreshness(ctx, scope, eventID, now.Add(time.Second))
		if err != nil {
			t.Fatal(err)
		}
		if input.Stages[1].Disposition != assurance.IngestionDispositionPending || input.Stages[2].Disposition == assurance.IngestionDispositionComplete || input.Stages[3].Disposition == assurance.IngestionDispositionComplete {
			t.Fatalf("pending fixture stages=%+v", input.Stages)
		}
	})

	t.Run("restricted source never claims a public read", func(t *testing.T) {
		scope := memory.Scope{Tenant: "freshness-restricted-" + uuid.NewString(), Project: "assurance", Namespace: "scope-" + uuid.NewString()}
		eventID, candidateID, memoryID, versionID, provenanceID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
		labels := `'{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}'::jsonb`
		statements := []struct {
			query string
			args  []any
		}{
			{`INSERT INTO raw_events(id,tenant,project,namespace,event_type,content,source_metadata,governance_processed_at,created_at) VALUES($1,$2,$3,$4,'conversation.message','restricted freshness fixture',` + labels + `,$5,$5)`, []any{eventID, scope.Tenant, scope.Project, scope.Namespace, now}},
			{`INSERT INTO candidate_memories(id,source_raw_event_id,tenant,project,namespace,class,content,confidence,importance,freshness,sensitivity,mutability,retention_class,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'episodic','restricted freshness fixture',0.9,0.5,1,'medium','immutable','durable','promoted',$6,$6)`, []any{candidateID, eventID, scope.Tenant, scope.Project, scope.Namespace, now.Add(time.Second)}},
			{`INSERT INTO canonical_memories(id,tenant,project,namespace,class,state,content,source_metadata,created_at,updated_at) VALUES($1,$2,$3,$4,'episodic','active','restricted freshness fixture',` + labels + `,$5,$5)`, []any{memoryID, scope.Tenant, scope.Project, scope.Namespace, now.Add(2 * time.Second)}},
			{`INSERT INTO memory_versions(id,memory_id,version,state,content,source_metadata,created_at,modified_by) VALUES($1,$2,1,'active','restricted freshness fixture',` + labels + `,$3,'freshness-integration-test')`, []any{versionID, memoryID, now.Add(2 * time.Second)}},
			{`INSERT INTO provenance_links(id,raw_event_id,candidate_memory_id,memory_id,tenant,project,namespace,operation,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,'promote_candidate',$8)`, []any{provenanceID, eventID, candidateID, memoryID, scope.Tenant, scope.Project, scope.Namespace, now.Add(2 * time.Second)}},
		}
		for _, statement := range statements {
			if _, err := pool.Exec(ctx, statement.query, statement.args...); err != nil {
				t.Fatal(err)
			}
		}
		input, err := repo.CollectIngestionFreshness(ctx, scope, eventID, now.Add(3*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		if input.Stages[2].Disposition != assurance.IngestionDispositionComplete || input.Stages[3].Disposition != assurance.IngestionDispositionIneligible {
			t.Fatalf("restricted fixture stages=%+v", input.Stages)
		}
	})
}
