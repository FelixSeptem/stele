package app

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

func TestProviderCorrectionRiverWorkerAppendsGovernedSuccessor(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_PROVIDER_CORRECTION_GOVERNED_DSN")
	if dsn == "" {
		t.Skip("fresh disposable Provider correction governance PostgreSQL DSN not configured")
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
		t.Fatalf("governed correction fixture requires an empty disposable database: tables=%d err=%v", tables, err)
	}
	if err = postgres.NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	if err = postgres.MigrateRiverDatabase(ctx, dsn, "auto"); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "correction-governed-tenant", Project: "correction-governed-project", Namespace: "correction-governed-namespace"}
	binding := provider.RuntimeBinding{BindingID: "correction-governed-binding", PrincipalID: "correction-governed-principal", Scope: scope, AgentID: "correction-governed-agent", SessionID: "correction-governed-session", ProviderInstanceID: "correction-governed-provider", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	execSQL := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	labels := `'{"version":"source-metadata-v1","trust":"untrusted","classification":"low"}'::jsonb`
	eventID, candidateID, memoryID, versionID, provenanceID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	execSQL(`INSERT INTO access_principals(id,role,status,label) VALUES($1,'agent','active','correction governance fixture')`, binding.PrincipalID)
	execSQL(`INSERT INTO access_scope_grants(id,principal_id,tenant,project,namespace,status,allowed_disclosure_profiles,default_disclosure_profile) VALUES('correction-governed-grant',$1,$2,$3,$4,'active',ARRAY['memory-reference-v1'],'memory-reference-v1')`, binding.PrincipalID, scope.Tenant, scope.Project, scope.Namespace)
	execSQL(`INSERT INTO provider_runtime_bindings(binding_id,principal_id,tenant,project,namespace,agent_id,session_id,conversation_id,provider_instance_id,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,'',$8,$9,$10)`, binding.BindingID, binding.PrincipalID, scope.Tenant, scope.Project, scope.Namespace, binding.AgentID, binding.SessionID, binding.ProviderInstanceID, binding.CreatedAt, binding.ExpiresAt)
	execSQL(`INSERT INTO raw_events(id,tenant,project,namespace,event_type,content,memory_path,metadata,source_metadata,governance_processed_at) VALUES($1,$2,$3,$4,'provider.reference.task','governed correction reference','tasks/correction-governed',jsonb_build_object('reference_contract_version','durable-reference-v1','reference_id','correction-governed-task','reference_binding_id',$5::text,'reference_principal_id',$6::text),`+labels+`,clock_timestamp())`, eventID, scope.Tenant, scope.Project, scope.Namespace, binding.BindingID, binding.PrincipalID)
	execSQL(`INSERT INTO candidate_memories(id,source_raw_event_id,tenant,project,namespace,class,content,confidence,importance,freshness,sensitivity,mutability,retention_class,status,memory_path,source_metadata) VALUES($1,$2,$3,$4,$5,'episodic','governed correction reference',0.8,0.5,1,'low','mutable','durable','promoted','tasks/correction-governed',`+labels+`)`, candidateID, eventID, scope.Tenant, scope.Project, scope.Namespace)
	execSQL(`INSERT INTO canonical_memories(id,tenant,project,namespace,class,state,content,memory_path,source_metadata) VALUES($1,$2,$3,$4,'episodic','active','original governed fact','tasks/correction-governed',`+labels+`)`, memoryID, scope.Tenant, scope.Project, scope.Namespace)
	execSQL(`INSERT INTO memory_versions(id,memory_id,version,state,content,modified_by,memory_path,source_metadata) VALUES($1,$2,1,'active','original governed fact','fixture','tasks/correction-governed',`+labels+`)`, versionID, memoryID)
	execSQL(`INSERT INTO provenance_links(id,raw_event_id,candidate_memory_id,memory_id,tenant,project,namespace,operation) VALUES($1,$2,$3,$4,$5,$6,$7,'promote_candidate')`, provenanceID, eventID, candidateID, memoryID, scope.Tenant, scope.Project, scope.Namespace)

	repo := postgres.NewRepository(pool)
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: postgres.RiverSchemaName})
	if err != nil {
		t.Fatal(err)
	}
	repo.SetMemoryIntentRiverInserter(riverClient)
	projection := provider.ProjectionRequest{Metadata: provider.OperationMetadata{RequestID: "governed-projection-1", OperationID: "governed-projection-op-1", SchemaVersion: "provider-v1", IdempotencyKey: "governed-projection-1"}, Input: provider.ProjectionInput{ContractVersion: provider.ProjectionContractVersion, Kind: "outcome", EffectID: "governed-correction-effect", SourceRevision: 1, OccurredAt: now.Truncate(time.Second).Format(time.RFC3339), ReportedOutcome: "succeeded", References: []provider.ProjectionReferenceSelector{{EventID: eventID}}}}
	first, err := repo.AdmitProjection(ctx, binding, projection)
	if err != nil || first.Disposition != "accepted" {
		t.Fatalf("first correction projection: %+v err=%v", first, err)
	}
	projection.Metadata.RequestID, projection.Metadata.OperationID, projection.Metadata.IdempotencyKey = "governed-projection-2", "governed-projection-op-2", "governed-projection-2"
	projection.Input.SourceRevision = 2
	second, err := repo.AdmitProjection(ctx, binding, projection)
	if err != nil || second.Disposition != "corrected" {
		t.Fatalf("corrected projection: %+v err=%v", second, err)
	}
	request := provider.CorrectionRequest{Metadata: provider.OperationMetadata{RequestID: "governed-correction-request", OperationID: "governed-correction-operation", IdempotencyKey: "governed-correction-idempotency", SchemaVersion: "provider-v1"}, Input: provider.CorrectionInput{ContractVersion: provider.CorrectionContractVersion, ProjectionID: second.ProjectionID, TargetMemoryID: memoryID, ExpectedVersion: 1, Content: "corrected governed fact", Reason: "source correction"}, PolicyVersion: "correction-policy-v1"}
	accepted, err := repo.SubmitCorrection(ctx, binding, request)
	if err != nil || accepted.Status != memory.MemoryIntentStatusAccepted {
		t.Fatalf("submit governed correction: %+v err=%v", accepted, err)
	}

	execution := jobs.MemoryIntentRiverExecution{
		History: repo,
		Worker: memory.MemoryIntentWorker{
			Router: memoryIntentGovernanceRouter{repo: repo}, Validator: repo, CorrectionValidator: repo,
		},
		Recorder: repo,
		Policy:   memory.StaticMemoryIntentPolicy{Scope: scope, PolicyVersion: request.PolicyVersion, Enabled: true},
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, &jobs.RiverMemoryIntentWorker{Store: repo, Execute: execution.Execute})
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: postgres.RiverSchemaName, Workers: workers, Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	stopRiverClient := func(client interface{ Stop(context.Context) error }) {
		t.Helper()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		if err := client.Stop(stopCtx); err != nil {
			t.Fatalf("stop governed correction River client: %v", err)
		}
	}
	var jobID int64
	if err = pool.QueryRow(ctx, `SELECT river_job_id FROM memory_intent_river_jobs WHERE intent_id=$1::uuid`, accepted.IntentID).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	waitForCorrectionRiverJob(t, ctx, pool, jobID)

	var versionCount, promotionAttributionCount int
	var previousContent, successorContent, intentStatus string
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_versions WHERE memory_id=$1::uuid`, memoryID).Scan(&versionCount); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT content FROM memory_versions WHERE memory_id=$1::uuid AND version=1`, memoryID).Scan(&previousContent); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT content FROM memory_versions WHERE memory_id=$1::uuid AND version=2`, memoryID).Scan(&successorContent); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM provenance_links WHERE memory_id=$1::uuid AND operation='promote_candidate' AND source_context->>'correction_projection_id'=$2`, memoryID, second.ProjectionID).Scan(&promotionAttributionCount); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT to_status FROM memory_intent_transitions WHERE intent_id=$1::uuid ORDER BY sequence DESC LIMIT 1`, accepted.IntentID).Scan(&intentStatus); err != nil {
		t.Fatal(err)
	}
	if versionCount != 2 || previousContent != "original governed fact" || successorContent != request.Input.Content || promotionAttributionCount != 1 || intentStatus != string(memory.MemoryIntentStatusActive) {
		t.Fatalf("governed correction outcome: versions=%d prior=%q successor=%q promotion_attribution=%d intent=%q", versionCount, previousContent, successorContent, promotionAttributionCount, intentStatus)
	}
	stopRiverClient(client)

	// A new repository and worker client model API/worker restart while the
	// completed request remains replayable without appending another version.
	restartedRepo := postgres.NewRepository(pool)
	restartedRepo.SetMemoryIntentRiverInserter(riverClient)
	restartedWorkers := river.NewWorkers()
	river.AddWorker(restartedWorkers, &jobs.RiverMemoryIntentWorker{Store: restartedRepo, Execute: execution.Execute})
	restartedClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: postgres.RiverSchemaName, Workers: restartedWorkers, Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := restartedClient.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	replayed, err := restartedRepo.SubmitCorrection(ctx, binding, request)
	if err != nil || replayed.Status != memory.MemoryIntentStatusReplayed || replayed.IntentID != accepted.IntentID {
		t.Fatalf("post-completion correction replay after API/worker restart: %+v err=%v", replayed, err)
	}
	var replayJobCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_intent_river_jobs WHERE intent_id=$1::uuid`, accepted.IntentID).Scan(&replayJobCount); err != nil || replayJobCount != 1 {
		t.Fatalf("restart replay created duplicate execution job: count=%d err=%v", replayJobCount, err)
	}
	stopRiverClient(restartedClient)

	// Admit a second correction, then supersede its source before a restarted
	// worker can claim it. The stale job must complete without a new candidate
	// or canonical successor.
	secondEventID, secondCandidateID, successorTargetID, successorVersionID, secondProvenanceID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	execSQL(`INSERT INTO raw_events(id,tenant,project,namespace,event_type,content,memory_path,metadata,source_metadata,governance_processed_at) VALUES($1,$2,$3,$4,'provider.reference.task','successor version reference','tasks/correction-governed',jsonb_build_object('reference_contract_version','durable-reference-v1','reference_id','correction-governed-successor','reference_binding_id',$5::text,'reference_principal_id',$6::text),`+labels+`,clock_timestamp())`, secondEventID, scope.Tenant, scope.Project, scope.Namespace, binding.BindingID, binding.PrincipalID)
	execSQL(`INSERT INTO candidate_memories(id,source_raw_event_id,tenant,project,namespace,class,content,confidence,importance,freshness,sensitivity,mutability,retention_class,status,memory_path,source_metadata) VALUES($1,$2,$3,$4,$5,'episodic','successor version reference',0.8,0.5,1,'low','mutable','durable','promoted','tasks/correction-governed',`+labels+`)`, secondCandidateID, secondEventID, scope.Tenant, scope.Project, scope.Namespace)
	execSQL(`INSERT INTO canonical_memories(id,tenant,project,namespace,class,state,content,memory_path,source_metadata) VALUES($1,$2,$3,$4,'episodic','active','independent correction target','tasks/correction-governed',`+labels+`)`, successorTargetID, scope.Tenant, scope.Project, scope.Namespace)
	execSQL(`INSERT INTO memory_versions(id,memory_id,version,state,content,modified_by,memory_path,source_metadata) VALUES($1,$2,1,'active','independent correction target','fixture','tasks/correction-governed',`+labels+`)`, successorVersionID, successorTargetID)
	execSQL(`INSERT INTO provenance_links(id,raw_event_id,candidate_memory_id,memory_id,tenant,project,namespace,operation) VALUES($1,$2,$3,$4,$5,$6,$7,'promote_candidate')`, secondProvenanceID, secondEventID, secondCandidateID, successorTargetID, scope.Tenant, scope.Project, scope.Namespace)
	secondReference, err := restartedRepo.ReferenceStatus(ctx, scope, binding.PrincipalID, secondEventID, "memory-reference-v1", "reference_only")
	if err != nil || secondReference.Status != provider.ReferenceStatusCompleted || secondReference.Reference == nil || secondReference.Reference.MemoryID != successorTargetID || secondReference.Reference.Version != 1 {
		t.Fatalf("successor durable reference = %+v err=%v", secondReference, err)
	}
	projection.Metadata.RequestID, projection.Metadata.OperationID, projection.Metadata.IdempotencyKey = "governed-projection-3", "governed-projection-op-3", "governed-projection-3"
	projection.Input.SourceRevision = 3
	projection.Input.References = []provider.ProjectionReferenceSelector{{EventID: secondEventID}}
	third, err := restartedRepo.AdmitProjection(ctx, binding, projection)
	if err != nil || third.Disposition != "corrected" {
		t.Fatalf("third correction projection: %+v err=%v", third, err)
	}
	staleRequest := request
	staleRequest.Metadata.RequestID, staleRequest.Metadata.OperationID, staleRequest.Metadata.IdempotencyKey = "stale-correction-request", "stale-correction-operation", "stale-correction-idempotency"
	staleRequest.Input.ProjectionID = third.ProjectionID
	staleRequest.Input.TargetMemoryID = successorTargetID
	staleRequest.Input.ExpectedVersion = 1
	staleRequest.Input.Content = "superseded correction must not promote"
	staleAccepted, err := restartedRepo.SubmitCorrection(ctx, binding, staleRequest)
	if err != nil || staleAccepted.Status != memory.MemoryIntentStatusAccepted {
		t.Fatalf("submit correction before source supersession: %+v err=%v", staleAccepted, err)
	}
	projection.Metadata.RequestID, projection.Metadata.OperationID, projection.Metadata.IdempotencyKey = "governed-projection-4", "governed-projection-op-4", "governed-projection-4"
	projection.Input.SourceRevision = 4
	fourth, err := restartedRepo.AdmitProjection(ctx, binding, projection)
	if err != nil || fourth.Disposition != "corrected" {
		t.Fatalf("fourth correction projection: %+v err=%v", fourth, err)
	}
	staleWorkers := river.NewWorkers()
	river.AddWorker(staleWorkers, &jobs.RiverMemoryIntentWorker{Store: restartedRepo, Execute: execution.Execute})
	staleClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: postgres.RiverSchemaName, Workers: staleWorkers, Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := staleClient.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	var staleJobID int64
	if err = pool.QueryRow(ctx, `SELECT river_job_id FROM memory_intent_river_jobs WHERE intent_id=$1::uuid`, staleAccepted.IntentID).Scan(&staleJobID); err != nil {
		t.Fatal(err)
	}
	waitForCorrectionRiverJob(t, ctx, pool, staleJobID)
	var staleStatus string
	if err = pool.QueryRow(ctx, `SELECT to_status FROM memory_intent_transitions WHERE intent_id=$1::uuid ORDER BY sequence DESC LIMIT 1`, staleAccepted.IntentID).Scan(&staleStatus); err != nil {
		t.Fatal(err)
	}
	if staleStatus != string(memory.MemoryIntentStatusSuppressed) {
		t.Fatalf("superseded correction worker status=%q, want suppressed", staleStatus)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM memory_versions WHERE memory_id=$1::uuid`, memoryID).Scan(&versionCount); err != nil || versionCount != 2 {
		t.Fatalf("stale correction created a canonical successor: versions=%d err=%v", versionCount, err)
	}
	stopRiverClient(staleClient)
	var completedJobs int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM `+postgres.RiverSchemaName+`.river_job WHERE id=$1 AND state=$2`, staleJobID, string(rivertype.JobStateCompleted)).Scan(&completedJobs); err != nil || completedJobs != 1 {
		t.Fatalf("stale correction River job completion=%d err=%v", completedJobs, err)
	}
}

func waitForCorrectionRiverJob(t *testing.T, ctx context.Context, pool *pgxpool.Pool, jobID int64) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		var state string
		var errors string
		if err := pool.QueryRow(ctx, `SELECT state,COALESCE(array_to_string(errors, E'\n'),'') FROM `+postgres.RiverSchemaName+`.river_job WHERE id=$1`, jobID).Scan(&state, &errors); err != nil {
			t.Fatalf("read correction River job: %v", err)
		}
		if state == string(rivertype.JobStateCompleted) {
			return
		}
		if state == string(rivertype.JobStateRetryable) {
			t.Fatalf("correction River job became retryable: %s", errors)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("correction River job state=%q, want completed: %v", state, ctx.Err())
		case <-ticker.C:
		}
	}
}
