package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/internal/workqueue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

// The caller must provide one empty, disposable PostgreSQL database. This test
// proves the public correction resolver against production migrations rather
// than manufacturing a parallel schema.
func TestProviderCorrectionPostgresResolutionReplayAndImmutableAttribution(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_PROVIDER_CORRECTION_DSN")
	if dsn == "" {
		t.Skip("fresh disposable Provider correction PostgreSQL DSN not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var tables int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public'`).Scan(&tables); err != nil || tables != 0 {
		t.Fatalf("correction fixture requires an empty disposable database: tables=%d err=%v", tables, err)
	}
	if err = NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	if err = MigrateRiverDatabase(ctx, dsn, "auto"); err != nil {
		t.Fatal(err)
	}
	if err = MigrateRiverDatabase(ctx, dsn, "validate"); err != nil {
		t.Fatalf("validate River migration state: %v", err)
	}
	if err = MigrateRiverDatabase(ctx, dsn, "auto"); err != nil {
		t.Fatalf("reapply River migrations: %v", err)
	}
	if err = MigrateDatabase(ctx, dsn, "validate"); err != nil {
		t.Fatalf("validate combined Stele and River migration state: %v", err)
	}
	if err = MigrateDatabase(ctx, dsn, "off"); err != nil {
		t.Fatalf("validate no-migration startup policy: %v", err)
	}
	status, err := NewMigrationRunner().Status(ctx, dsn)
	if err != nil || status.Dirty || status.CurrentVersion != 43 {
		t.Fatalf("migration not clean: %+v err=%v", status, err)
	}

	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "correction-tenant", Project: "correction-project", Namespace: "correction-namespace"}
	binding := provider.RuntimeBinding{BindingID: "correction-binding", PrincipalID: "correction-principal", Scope: scope, AgentID: "correction-agent", SessionID: "correction-session", ConversationID: "correction-conversation", ProviderInstanceID: "correction-provider", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO access_principals(id,role,status,label) VALUES($1,'agent','active','correction fixture')`, binding.PrincipalID)
	exec(`INSERT INTO access_scope_grants(id,principal_id,tenant,project,namespace,status,allowed_disclosure_profiles,default_disclosure_profile) VALUES($1,$2,$3,$4,$5,'active',ARRAY['memory-reference-v1'],'memory-reference-v1')`, "correction-grant", binding.PrincipalID, scope.Tenant, scope.Project, scope.Namespace)
	exec(`INSERT INTO provider_runtime_bindings(binding_id,principal_id,tenant,project,namespace,agent_id,session_id,conversation_id,provider_instance_id,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, append(receiptIdentityArgs(binding), binding.CreatedAt, binding.ExpiresAt)...)

	eventID, candidateID, memoryID, versionID, provenanceID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	const labels = `'{"version":"source-metadata-v1","trust":"untrusted","classification":"low"}'::jsonb`
	exec(`INSERT INTO raw_events(id,tenant,project,namespace,event_type,content,memory_path,metadata,source_metadata,governance_processed_at)
VALUES($1,$2,$3,$4,'provider.reference.task','correction durable reference','tasks/correction',jsonb_build_object('reference_contract_version','durable-reference-v1','reference_id','correction-task','reference_binding_id',$5::text,'reference_principal_id',$6::text),`+labels+`,clock_timestamp())`, eventID, scope.Tenant, scope.Project, scope.Namespace, binding.BindingID, binding.PrincipalID)
	exec(`INSERT INTO candidate_memories(id,source_raw_event_id,tenant,project,namespace,class,content,confidence,importance,freshness,sensitivity,mutability,retention_class,status,memory_path,source_metadata)
VALUES($1,$2,$3,$4,$5,'episodic','correction durable reference',0.8,0.5,1,'low','mutable','durable','promoted','tasks/correction',`+labels+`)`, candidateID, eventID, scope.Tenant, scope.Project, scope.Namespace)
	exec(`INSERT INTO canonical_memories(id,tenant,project,namespace,class,state,content,memory_path,source_metadata)
VALUES($1,$2,$3,$4,'episodic','active','original correction target','tasks/correction',`+labels+`)`, memoryID, scope.Tenant, scope.Project, scope.Namespace)
	exec(`INSERT INTO memory_versions(id,memory_id,version,state,content,modified_by,memory_path,source_metadata)
VALUES($1,$2,1,'active','original correction target','fixture','tasks/correction',`+labels+`)`, versionID, memoryID)
	exec(`INSERT INTO provenance_links(id,raw_event_id,candidate_memory_id,memory_id,tenant,project,namespace,operation)
VALUES($1,$2,$3,$4,$5,$6,$7,'promote_candidate')`, provenanceID, eventID, candidateID, memoryID, scope.Tenant, scope.Project, scope.Namespace)
	hiddenEventID, hiddenCandidateID, hiddenMemoryID, hiddenVersionID, hiddenProvenanceID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	const restrictedLabels = `'{"version":"source-metadata-v1","trust":"untrusted","classification":"restricted"}'::jsonb`
	exec(`INSERT INTO raw_events(id,tenant,project,namespace,event_type,content,memory_path,metadata,source_metadata,governance_processed_at)
VALUES($1,$2,$3,$4,'provider.reference.task','restricted correction reference','tasks/restricted',jsonb_build_object('reference_contract_version','durable-reference-v1','reference_id','restricted-correction-task','reference_binding_id',$5::text,'reference_principal_id',$6::text),`+restrictedLabels+`,clock_timestamp())`, hiddenEventID, scope.Tenant, scope.Project, scope.Namespace, binding.BindingID, binding.PrincipalID)
	exec(`INSERT INTO candidate_memories(id,source_raw_event_id,tenant,project,namespace,class,content,confidence,importance,freshness,sensitivity,mutability,retention_class,status,memory_path,source_metadata)
VALUES($1,$2,$3,$4,$5,'episodic','restricted correction reference',0.8,0.5,1,'restricted','mutable','durable','promoted','tasks/restricted',`+restrictedLabels+`)`, hiddenCandidateID, hiddenEventID, scope.Tenant, scope.Project, scope.Namespace)
	exec(`INSERT INTO canonical_memories(id,tenant,project,namespace,class,state,content,memory_path,source_metadata)
VALUES($1,$2,$3,$4,'episodic','active','restricted correction target','tasks/restricted',`+restrictedLabels+`)`, hiddenMemoryID, scope.Tenant, scope.Project, scope.Namespace)
	exec(`INSERT INTO memory_versions(id,memory_id,version,state,content,modified_by,memory_path,source_metadata)
VALUES($1,$2,1,'active','restricted correction target','fixture','tasks/restricted',`+restrictedLabels+`)`, hiddenVersionID, hiddenMemoryID)
	exec(`INSERT INTO provenance_links(id,raw_event_id,candidate_memory_id,memory_id,tenant,project,namespace,operation)
VALUES($1,$2,$3,$4,$5,$6,$7,'promote_candidate')`, hiddenProvenanceID, hiddenEventID, hiddenCandidateID, hiddenMemoryID, scope.Tenant, scope.Project, scope.Namespace)
	repo := NewRepository(pool)
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: RiverSchemaName})
	if err != nil {
		t.Fatal(err)
	}
	repo.SetMemoryIntentRiverInserter(riverClient)
	testRiverMemoryIntentExecutionRollbackAndRetry(t, ctx, pool, repo)
	repo.SetMemoryIntentRiverInserter(riverClient)
	intentService := memory.MemoryIntentService{Processor: repo, NewID: uuid.NewString}
	queueScope := memory.Scope{Tenant: "queue-tenant", Project: "queue-project", Namespace: "queue-namespace"}
	atomicInput := memory.MemoryIntentInput{Scope: queueScope, MemoryPath: "tasks/atomic", Type: memory.MemoryIntentRemember, Content: "atomic acceptance fixture", Actor: binding.PrincipalID, Reason: "verify durable queue handoff", RequestID: "atomic-request", OperationID: "atomic-operation", IdempotencyKey: "atomic-idempotency"}
	atomicIntent, err := intentService.Submit(ctx, atomicInput)
	if err != nil || atomicIntent.Status != memory.MemoryIntentStatusAccepted {
		t.Fatalf("atomic intent admission: %+v err=%v", atomicIntent, err)
	}
	atomicReplay, err := intentService.Submit(ctx, atomicInput)
	if err != nil || atomicReplay.Status != memory.MemoryIntentStatusReplayed || atomicReplay.ID != atomicIntent.ID {
		t.Fatalf("atomic intent replay: %+v err=%v", atomicReplay, err)
	}
	var queued int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_intent_river_jobs WHERE intent_id=$1::uuid`, atomicIntent.ID).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("atomic intent River job association=%d err=%v", queued, err)
	}
	if _, err = pool.Exec(ctx, `CREATE FUNCTION fail_memory_intent_river_link() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced link failure'; END $$; CREATE TRIGGER fail_memory_intent_river_link BEFORE INSERT ON memory_intent_river_jobs FOR EACH ROW EXECUTE FUNCTION fail_memory_intent_river_link()`); err != nil {
		t.Fatal(err)
	}
	failedRecord := memory.MemoryIntentRecord{ID: uuid.NewString(), Scope: queueScope, MemoryPath: "tasks/rollback", Type: memory.MemoryIntentRemember, Content: "must roll back", Actor: binding.PrincipalID, Reason: "transaction rollback fixture", RequestID: "rollback-request", OperationID: "rollback-operation", IdempotencyKey: "rollback-idempotency", PrecedenceVersion: "governed-operation-precedence-v1", PrecedenceStage: "mutation", PrecedenceOutcome: "accepted", Status: memory.MemoryIntentStatusAccepted, CreatedAt: now}
	if _, err = repo.AcceptMemoryIntent(ctx, failedRecord); err == nil || !strings.Contains(err.Error(), "forced link failure") {
		t.Fatalf("AcceptMemoryIntent() error = %v, want forced association failure", err)
	}
	if _, err = pool.Exec(ctx, `DROP TRIGGER fail_memory_intent_river_link ON memory_intent_river_jobs; DROP FUNCTION fail_memory_intent_river_link()`); err != nil {
		t.Fatal(err)
	}
	var rolledBackIntents, rolledBackJobs int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_intents WHERE id=$1::uuid`, failedRecord.ID).Scan(&rolledBackIntents); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM `+RiverSchemaName+`.river_job WHERE args->>'intent_id'=$1`, failedRecord.ID).Scan(&rolledBackJobs); err != nil || rolledBackIntents != 0 || rolledBackJobs != 0 {
		t.Fatalf("failed acceptance persisted intent=%d River job=%d err=%v", rolledBackIntents, rolledBackJobs, err)
	}
	legacyIntentID := uuid.NewString()
	legacyRecord := memory.MemoryIntentRecord{ID: legacyIntentID, Scope: queueScope, MemoryPath: "tasks/legacy", Type: memory.MemoryIntentRemember, Content: "legacy pending intent", Actor: binding.PrincipalID, Reason: "legacy handoff fixture", RequestID: "legacy-request", OperationID: "legacy-operation", IdempotencyKey: "legacy-idempotency", PrecedenceVersion: "governed-operation-precedence-v1", PrecedenceStage: "mutation", PrecedenceOutcome: "accepted", Status: memory.MemoryIntentStatusAccepted, CreatedAt: now}
	if _, err = repo.AppendMemoryIntent(ctx, legacyRecord); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO derived_work_items(work_key,kind,tenant,project,namespace,watermark,idempotency_key,reference,state,max_attempts,next_attempt_at,detail_expires_at) VALUES('legacy-intent-work','memory_intent',$1,$2,$3,'legacy-watermark','legacy-idempotency',$4,'queued',3,clock_timestamp(),clock_timestamp()+interval '24 hours') RETURNING id::text`, queueScope.Tenant, queueScope.Project, queueScope.Namespace, legacyIntentID).Scan(new(string)); err != nil {
		t.Fatal(err)
	}
	if err = repo.HandoffLegacyMemoryIntentJobs(ctx); err != nil {
		t.Fatal(err)
	}
	if err = repo.HandoffLegacyMemoryIntentJobs(ctx); err != nil {
		t.Fatal(err)
	}
	var legacyLinks, legacyJobs, handedOff int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_intent_river_jobs WHERE intent_id=$1::uuid`, legacyIntentID).Scan(&legacyLinks); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM `+RiverSchemaName+`.river_job WHERE args->>'intent_id'=$1`, legacyIntentID).Scan(&legacyJobs); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM derived_work_items WHERE reference=$1 AND state='completed' AND failure_category='river_handoff'`, legacyIntentID).Scan(&handedOff); err != nil || legacyLinks != 1 || legacyJobs != 1 || handedOff != 1 {
		t.Fatalf("legacy handoff links=%d jobs=%d oldRows=%d err=%v", legacyLinks, legacyJobs, handedOff, err)
	}
	terminalIntentID := uuid.NewString()
	terminalRecord := legacyRecord
	terminalRecord.ID = terminalIntentID
	terminalRecord.OperationID, terminalRecord.IdempotencyKey = "terminal-legacy-operation", "terminal-legacy-idempotency"
	if _, err = repo.AppendMemoryIntent(ctx, terminalRecord); err != nil {
		t.Fatal(err)
	}
	terminalWorkID := uuid.NewString()
	if _, err = pool.Exec(ctx, `INSERT INTO derived_work_items(id,work_key,kind,tenant,project,namespace,watermark,idempotency_key,reference,state,max_attempts,next_attempt_at,terminal_at,detail_expires_at) VALUES($1::uuid,'legacy-terminal-intent-work','memory_intent',$2,$3,$4,'terminal-watermark','terminal-idempotency',$5,'completed',3,clock_timestamp(),clock_timestamp(),clock_timestamp()+interval '24 hours')`, terminalWorkID, queueScope.Tenant, queueScope.Project, queueScope.Namespace, terminalIntentID); err != nil {
		t.Fatal(err)
	}
	otherWorkID := uuid.NewString()
	if _, err = pool.Exec(ctx, `INSERT INTO derived_work_items(id,work_key,kind,tenant,project,namespace,watermark,idempotency_key,reference,state,max_attempts,next_attempt_at,detail_expires_at) VALUES($1::uuid,'legacy-other-kind-work',$2,$3,$4,$5,'other-kind-watermark','other-kind-idempotency','other-kind-reference','queued',3,clock_timestamp(),clock_timestamp()+interval '24 hours')`, otherWorkID, string(workqueue.WorkKindReflection), queueScope.Tenant, queueScope.Project, queueScope.Namespace); err != nil {
		t.Fatal(err)
	}
	if err = repo.HandoffLegacyMemoryIntentJobs(ctx); err != nil {
		t.Fatal(err)
	}
	var terminalJobCount int
	var terminalState, otherKindState string
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM `+RiverSchemaName+`.river_job WHERE args->>'intent_id'=$1`, terminalIntentID).Scan(&terminalJobCount); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT state FROM derived_work_items WHERE id=$1::uuid`, terminalWorkID).Scan(&terminalState); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT state FROM derived_work_items WHERE id=$1::uuid`, otherWorkID).Scan(&otherKindState); err != nil || terminalJobCount != 0 || terminalState != "completed" || otherKindState != "queued" {
		t.Fatalf("handoff changed terminal or unrelated work: terminal_jobs=%d terminal_state=%q other_kind_state=%q err=%v", terminalJobCount, terminalState, otherKindState, err)
	}
	projection := provider.ProjectionRequest{Metadata: provider.OperationMetadata{RequestID: "projection-request", OperationID: "projection-operation", SchemaVersion: "provider-v1", IdempotencyKey: "projection-v1"}, Input: provider.ProjectionInput{ContractVersion: provider.ProjectionContractVersion, Kind: "outcome", EffectID: "correction-effect", SourceRevision: 1, OccurredAt: "2026-10-31T00:00:00Z", ReportedOutcome: "succeeded", References: []provider.ProjectionReferenceSelector{{EventID: eventID}}}}
	first, err := repo.AdmitProjection(ctx, binding, projection)
	if err != nil || first.Disposition != "accepted" {
		t.Fatalf("first projection: %+v err=%v", first, err)
	}
	projection.Metadata.IdempotencyKey, projection.Input.SourceRevision = "projection-v2", 2
	second, err := repo.AdmitProjection(ctx, binding, projection)
	if err != nil || second.Disposition != "corrected" || second.SupersedesID != first.ProjectionID {
		t.Fatalf("corrected projection: %+v err=%v", second, err)
	}
	projection.Metadata.IdempotencyKey, projection.Input.SourceRevision = "projection-v3", 3
	third, err := repo.AdmitProjection(ctx, binding, projection)
	if err != nil || third.Disposition != "corrected" || third.SupersedesID != second.ProjectionID {
		t.Fatalf("latest correction projection: %+v err=%v", third, err)
	}
	hiddenProjection := provider.ProjectionRequest{Metadata: provider.OperationMetadata{RequestID: "hidden-projection-request-1", OperationID: "hidden-projection-operation", SchemaVersion: "provider-v1", IdempotencyKey: "hidden-projection-v1"}, Input: provider.ProjectionInput{ContractVersion: provider.ProjectionContractVersion, Kind: "outcome", EffectID: "hidden-correction-effect", SourceRevision: 1, OccurredAt: "2026-10-31T00:00:00Z", ReportedOutcome: "succeeded", References: []provider.ProjectionReferenceSelector{{EventID: hiddenEventID}}}}
	if _, err = repo.AdmitProjection(ctx, binding, hiddenProjection); err != nil {
		t.Fatalf("first restricted-source projection: %v", err)
	}
	hiddenProjection.Metadata.IdempotencyKey, hiddenProjection.Input.SourceRevision = "hidden-projection-v2", 2
	hiddenCurrent, err := repo.AdmitProjection(ctx, binding, hiddenProjection)
	if err != nil || hiddenCurrent.Disposition != "corrected" {
		t.Fatalf("latest restricted-source projection: %+v err=%v", hiddenCurrent, err)
	}

	wrongBinding := binding
	wrongBinding.BindingID = "another-current-binding"
	exec(`INSERT INTO provider_runtime_bindings(binding_id,principal_id,tenant,project,namespace,agent_id,session_id,conversation_id,provider_instance_id,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, append(receiptIdentityArgs(wrongBinding), wrongBinding.CreatedAt, wrongBinding.ExpiresAt)...)
	deniedCases := []struct {
		name          string
		binding       provider.RuntimeBinding
		projectionID  string
		targetID      string
		targetVersion int64
		category      error
	}{
		{name: "wrong current binding", binding: wrongBinding, projectionID: third.ProjectionID, targetID: memoryID, targetVersion: 1, category: provider.ErrCorrectionUnavailable},
		{name: "missing projection", binding: binding, projectionID: uuid.NewString(), targetID: memoryID, targetVersion: 1, category: provider.ErrCorrectionUnavailable},
		{name: "first projection revision", binding: binding, projectionID: first.ProjectionID, targetID: memoryID, targetVersion: 1, category: provider.ErrCorrectionUnavailable},
		{name: "superseded projection revision", binding: binding, projectionID: second.ProjectionID, targetID: memoryID, targetVersion: 1, category: provider.ErrCorrectionStale},
		{name: "mismatched target lineage", binding: binding, projectionID: third.ProjectionID, targetID: uuid.NewString(), targetVersion: 1, category: provider.ErrCorrectionUnavailable},
		{name: "mismatched target version", binding: binding, projectionID: third.ProjectionID, targetID: memoryID, targetVersion: 2, category: provider.ErrCorrectionUnavailable},
	}
	for index, denied := range deniedCases {
		request := provider.CorrectionRequest{
			Metadata:      provider.OperationMetadata{RequestID: fmt.Sprintf("denied-correction-request-%d", index), OperationID: fmt.Sprintf("denied-correction-operation-%d", index), IdempotencyKey: fmt.Sprintf("denied-correction-idempotency-%d", index), SchemaVersion: "provider-v1"},
			Input:         provider.CorrectionInput{ContractVersion: provider.CorrectionContractVersion, ProjectionID: denied.projectionID, TargetMemoryID: denied.targetID, ExpectedVersion: denied.targetVersion, Content: "denied correction", Reason: "denied fixture"},
			PolicyVersion: "correction-policy-v1",
		}
		_, submitErr := repo.SubmitCorrection(ctx, denied.binding, request)
		if !errors.Is(submitErr, denied.category) {
			t.Fatalf("%s error = %v, want non-disclosing %v", denied.name, submitErr, denied.category)
		}
	}
	if _, err = pool.Exec(ctx, `UPDATE access_scope_grants SET status='suspended' WHERE id='correction-grant'`); err != nil {
		t.Fatal(err)
	}
	grantDenied := provider.CorrectionRequest{Metadata: provider.OperationMetadata{RequestID: "denied-grant-request", OperationID: "denied-grant-operation", IdempotencyKey: "denied-grant-idempotency", SchemaVersion: "provider-v1"}, Input: provider.CorrectionInput{ContractVersion: provider.CorrectionContractVersion, ProjectionID: third.ProjectionID, TargetMemoryID: memoryID, ExpectedVersion: 1, Content: "denied grant correction", Reason: "denied fixture"}, PolicyVersion: "correction-policy-v1"}
	if _, err = repo.SubmitCorrection(ctx, binding, grantDenied); !errors.Is(err, provider.ErrCorrectionUnavailable) {
		t.Fatalf("revoked grant error = %v, want non-disclosing unavailable", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE access_scope_grants SET status='active' WHERE id='correction-grant'`); err != nil {
		t.Fatal(err)
	}
	hiddenSource := grantDenied
	hiddenSource.Metadata.RequestID, hiddenSource.Metadata.OperationID, hiddenSource.Metadata.IdempotencyKey = "denied-source-request", "denied-source-operation", "denied-source-idempotency"
	hiddenSource.Input.ProjectionID, hiddenSource.Input.TargetMemoryID = hiddenCurrent.ProjectionID, hiddenMemoryID
	if _, err = repo.SubmitCorrection(ctx, binding, hiddenSource); !errors.Is(err, provider.ErrCorrectionUnavailable) {
		t.Fatalf("restricted source error = %v, want non-disclosing unavailable", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE canonical_memories SET state='suppressed' WHERE id=$1::uuid`, memoryID); err != nil {
		t.Fatal(err)
	}
	hiddenTarget := grantDenied
	hiddenTarget.Metadata.RequestID, hiddenTarget.Metadata.OperationID, hiddenTarget.Metadata.IdempotencyKey = "denied-hidden-request", "denied-hidden-operation", "denied-hidden-idempotency"
	if _, err = repo.SubmitCorrection(ctx, binding, hiddenTarget); !errors.Is(err, provider.ErrCorrectionUnavailable) {
		t.Fatalf("hidden target error = %v, want non-disclosing unavailable", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE canonical_memories SET state='active' WHERE id=$1::uuid`, memoryID); err != nil {
		t.Fatal(err)
	}
	var deniedIntentCount, deniedJobCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_intents WHERE request_id LIKE 'denied-%'`).Scan(&deniedIntentCount); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM `+RiverSchemaName+`.river_job WHERE args->>'intent_id' IN (SELECT id::text FROM memory_intents WHERE request_id LIKE 'denied-%')`).Scan(&deniedJobCount); err != nil || deniedIntentCount != 0 || deniedJobCount != 0 {
		t.Fatalf("denied correction persisted intents=%d River jobs=%d err=%v", deniedIntentCount, deniedJobCount, err)
	}
	request := provider.CorrectionRequest{Metadata: provider.OperationMetadata{RequestID: "correction-request", OperationID: "correction-operation", IdempotencyKey: "correction-idempotency", SchemaVersion: "provider-v1"}, Input: provider.CorrectionInput{ContractVersion: provider.CorrectionContractVersion, ProjectionID: third.ProjectionID, TargetMemoryID: memoryID, ExpectedVersion: 1, Content: "corrected governed content", Reason: "corrected source evidence"}, PolicyVersion: "correction-policy-v1"}
	accepted, err := repo.SubmitCorrection(ctx, binding, request)
	if err != nil || accepted.Status != memory.MemoryIntentStatusAccepted || accepted.AttributionState != "verified" || accepted.IntentID == "" {
		t.Fatalf("submit correction: %+v err=%v", accepted, err)
	}
	var attributed int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_intent_correction_attributions WHERE intent_id=$1::uuid AND projection_id=$2::uuid AND target_memory_id=$3::uuid AND target_version=1`, accepted.IntentID, third.ProjectionID, memoryID).Scan(&attributed); err != nil || attributed != 1 {
		t.Fatalf("attribution=%d err=%v", attributed, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_intent_river_jobs WHERE intent_id=$1::uuid`, accepted.IntentID).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("River job association=%d err=%v", queued, err)
	}
	var jobID int64
	var jobKind, intentArg string
	if err = pool.QueryRow(ctx, `SELECT j.id,j.kind,j.args->>'intent_id' FROM `+RiverSchemaName+`.river_job j JOIN memory_intent_river_jobs a ON a.river_job_id=j.id WHERE a.intent_id=$1::uuid`, accepted.IntentID).Scan(&jobID, &jobKind, &intentArg); err != nil || jobID <= 0 || jobKind != "stele_memory_intent" || intentArg != accepted.IntentID {
		t.Fatalf("River job payload id=%d kind=%q intent=%q err=%v", jobID, jobKind, intentArg, err)
	}
	history, err := repo.ReadMemoryIntentHistory(ctx, scope, accepted.IntentID)
	if err != nil || history.Intent.Correction == nil || history.Intent.Correction.ProjectionID != third.ProjectionID || history.Intent.Correction.SourceRevision != 3 {
		t.Fatalf("intent history correction attribution = %+v err=%v", history.Intent.Correction, err)
	}
	if history.Intent.PolicyVersion != "correction-policy-v1" {
		t.Fatalf("intent policy version = %q, want persisted admission policy", history.Intent.PolicyVersion)
	}
	ordinary, err := repo.AppendMemoryIntent(ctx, memory.MemoryIntentRecord{Scope: scope, MemoryPath: "tasks/ordinary", Type: memory.MemoryIntentRemember, Actor: "fixture", Reason: "ordinary intent", RequestID: "ordinary-request", OperationID: "ordinary-operation", IdempotencyKey: "ordinary-idempotency", Content: "ordinary", PrecedenceVersion: "governed-operation-precedence-v1", PrecedenceStage: "mutation", PrecedenceOutcome: "accepted", Status: memory.MemoryIntentStatusAccepted})
	if err != nil {
		t.Fatal(err)
	}
	listed, err := repo.ListMemoryIntents(ctx, memory.MemoryIntentListInput{Scope: scope, Limit: 10})
	if err != nil || len(listed) != 2 {
		t.Fatalf("listed correction attribution = %+v err=%v", listed, err)
	}
	var foundCorrection, foundOrdinary bool
	for _, item := range listed {
		switch item.ID {
		case accepted.IntentID:
			foundCorrection = item.Correction != nil && item.Correction.ProjectionID == third.ProjectionID
		case ordinary.ID:
			foundOrdinary = item.Correction == nil
		}
	}
	if !foundCorrection || !foundOrdinary {
		t.Fatalf("list did not preserve ordinary and attributed intents: %+v", listed)
	}
	replayed, err := repo.SubmitCorrection(ctx, binding, request)
	if err != nil || replayed.Status != memory.MemoryIntentStatusReplayed || replayed.IntentID != accepted.IntentID {
		t.Fatalf("replay: %+v err=%v", replayed, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_intent_river_jobs WHERE intent_id=$1::uuid`, accepted.IntentID).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("River job association after replay=%d err=%v", queued, err)
	}
	conflict := request
	conflict.Input.Content = "different correction content"
	if _, err = repo.SubmitCorrection(ctx, binding, conflict); !errors.Is(err, provider.ErrCorrectionConflict) {
		t.Fatalf("idempotency collision error=%v", err)
	}
	policyConflict := request
	policyConflict.PolicyVersion = "correction-policy-v2"
	if _, err = repo.SubmitCorrection(ctx, binding, policyConflict); !errors.Is(err, provider.ErrCorrectionConflict) {
		t.Fatalf("policy-version idempotency collision error=%v", err)
	}
	foreign := binding
	foreign.BindingID = "foreign-binding"
	if _, err = repo.SubmitCorrection(ctx, foreign, request); !errors.Is(err, provider.ErrCorrectionUnavailable) {
		t.Fatalf("foreign binding error=%v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE memory_intent_correction_attributions SET source_revision=3 WHERE intent_id=$1::uuid`, accepted.IntentID); err == nil {
		t.Fatal("correction attribution update was allowed")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM memory_intent_correction_attributions WHERE intent_id=$1::uuid`, accepted.IntentID); err == nil {
		t.Fatal("correction attribution delete was allowed")
	}

}

func testRiverMemoryIntentExecutionRollbackAndRetry(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *Repository) {
	t.Helper()
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "river-worker-tenant", Project: "river-worker-project", Namespace: "river-worker-namespace"}
	intent := memory.MemoryIntentRecord{
		ID: uuid.NewString(), Scope: scope, MemoryPath: "tasks/river-worker", Type: memory.MemoryIntentRemember,
		Content: "River retry transaction fixture", Actor: "river-worker-test", Reason: "verify retry rollback",
		RequestID: uuid.NewString(), OperationID: uuid.NewString(), IdempotencyKey: uuid.NewString(),
		PrecedenceVersion: "governed-operation-precedence-v1", PrecedenceStage: "mutation", PrecedenceOutcome: "accepted",
		Status: memory.MemoryIntentStatusAccepted, CreatedAt: now,
	}
	if _, err := repo.AcceptMemoryIntent(ctx, intent); err != nil {
		t.Fatalf("accept River worker intent: %v", err)
	}

	var attempts atomic.Int32
	firstAttemptStarted := make(chan struct{}, 1)
	secondAttemptStarted := make(chan struct{}, 1)
	releaseFirstAttempt := make(chan struct{}, 1)
	releaseSecondAttempt := make(chan struct{}, 1)
	defer func() {
		releaseFirstAttempt <- struct{}{}
		releaseSecondAttempt <- struct{}{}
	}()
	execution := &jobs.RiverMemoryIntentWorker{Store: repo, Execute: func(execCtx context.Context, record memory.MemoryIntentRecord) error {
		event, err := repo.WriteRawEvent(execCtx, memory.IngestEventInput{
			Scope: record.Scope, EventType: "stele.river_worker.fixture", Content: "transactional governance side effect",
			Metadata: map[string]any{"river_worker_intent_id": record.ID}, SourceTimestamp: now,
		})
		if err != nil {
			return err
		}
		if err := repo.AppendMemoryIntentTransition(execCtx, memory.MemoryIntentTransition{
			IntentID: record.ID, Scope: record.Scope, Sequence: 2, From: memory.MemoryIntentStatusAccepted,
			To: memory.MemoryIntentStatusCandidate, Actor: record.Actor, Reason: "River worker fixture",
			DiagnosticCategory: memory.MemoryIntentDiagnosticAccepted, WorkReference: event.ID, OccurredAt: now,
		}); err != nil {
			return err
		}
		switch attempts.Add(1) {
		case 1:
			firstAttemptStarted <- struct{}{}
			<-releaseFirstAttempt
			return errors.New("force River retry after transactional writes")
		case 2:
			secondAttemptStarted <- struct{}{}
			<-releaseSecondAttempt
		}
		return nil
	}}
	workers := river.NewWorkers()
	river.AddWorker(workers, execution)
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema: RiverSchemaName, Workers: workers, Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 1}},
	})
	if err != nil {
		t.Fatalf("initialize River test worker: %v", err)
	}
	repo.SetMemoryIntentRiverInserter(client)
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("start River test worker: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.Stop(stopCtx); err != nil {
			t.Errorf("stop River test worker: %v", err)
		}
	}()

	var jobID int64
	if err := pool.QueryRow(ctx, `SELECT river_job_id FROM memory_intent_river_jobs WHERE intent_id=$1::uuid`, intent.ID).Scan(&jobID); err != nil {
		t.Fatalf("read River job id: %v", err)
	}
	waitForSignal(t, ctx, firstAttemptStarted, "first River attempt")
	var eventCount, transitionCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM raw_events WHERE metadata->>'river_worker_intent_id'=$1`, intent.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM memory_intent_transitions WHERE intent_id=$1::uuid`, intent.ID).Scan(&transitionCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 0 || transitionCount != 1 {
		t.Fatalf("uncommitted River writes became visible: raw events=%d transitions=%d, want 0 and initial transition only", eventCount, transitionCount)
	}
	releaseFirstAttempt <- struct{}{}
	waitForSignal(t, ctx, secondAttemptStarted, "River retry attempt")
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM raw_events WHERE metadata->>'river_worker_intent_id'=$1`, intent.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM memory_intent_transitions WHERE intent_id=$1::uuid`, intent.ID).Scan(&transitionCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 0 || transitionCount != 1 {
		t.Fatalf("failed River attempt leaked writes after retry began: raw events=%d transitions=%d", eventCount, transitionCount)
	}
	releaseSecondAttempt <- struct{}{}
	waitForRiverJobState(t, ctx, pool, jobID, string(rivertype.JobStateCompleted))
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM raw_events WHERE metadata->>'river_worker_intent_id'=$1`, intent.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM memory_intent_transitions WHERE intent_id=$1::uuid`, intent.ID).Scan(&transitionCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 || transitionCount != 2 || attempts.Load() != 2 {
		t.Fatalf("successful retry state: events=%d transitions=%d attempts=%d, want 1, 2, 2", eventCount, transitionCount, attempts.Load())
	}
}

func waitForSignal(t *testing.T, ctx context.Context, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	case <-ctx.Done():
		t.Fatalf("timed out waiting for %s: %v", label, ctx.Err())
	}
}

func waitForRiverJobState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, jobID int64, want string) {
	t.Helper()
	deadline := time.NewTicker(25 * time.Millisecond)
	defer deadline.Stop()
	for {
		var state string
		if err := pool.QueryRow(ctx, `SELECT state FROM `+RiverSchemaName+`.river_job WHERE id=$1`, jobID).Scan(&state); err != nil {
			t.Fatalf("read River job state: %v", err)
		}
		if state == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("River job state=%q, want %q: %v", state, want, ctx.Err())
		case <-deadline.C:
		}
	}
}
