package postgres_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var residentOwnedScope = memory.Scope{Tenant: "resident-owned", Project: "resident-evaluation", Namespace: "conformance-v1"}

// Only the verifier's fresh, uniquely owned database is accepted. The seed
// phase deliberately leaves both projections and immutable source history for
// an independent process to read after a real PostgreSQL container restart.
func TestResidentEvaluationPostgresOwnedReplayAndScope(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("STELE_TEST_RESIDENT_EVALUATION_DSN"))
	if dsn == "" {
		t.Skip("owned resident evaluation DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := postgres.NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := postgres.NewRepository(pool)
	scope := residentOwnedScope
	if os.Getenv("STELE_TEST_RESIDENT_PHASE") == "post_restart" {
		residentVerifyStoredPlans(t, ctx, pool, repo)
		return
	}
	if os.Getenv("STELE_TEST_RESIDENT_PHASE") != "seed" {
		t.Fatal("owned resident evaluation phase must be seed or post_restart")
	}
	eligible := residentSeedSource(t, ctx, pool, repo, scope, "Prefer concise answers with exact value 47.")
	popular := residentSeedSource(t, ctx, pool, repo, scope, "Prefer unsupported popular value 999.")
	popular.AccessCount = 1_000_000
	var foreignSources []retrieval.ResidentSource
	for dimension := 0; dimension < 3; dimension++ {
		foreign := residentForeignScope(dimension)
		foreignSources = append(foreignSources, residentSeedSource(t, ctx, pool, repo, foreign, eligible.Memory.Content))
		if _, err := repo.ReadMemoryHistory(ctx, foreign, eligible.Memory.ID, true); err == nil {
			t.Fatal("source history crossed isolation boundary")
		}
	}
	// Source time follows persisted canonical creation; held-out utility time
	// follows support. The fixed evaluation clock is captured after all writes.
	supportAt := time.Now().UTC().Add(-time.Microsecond)
	support := retrieval.ResidentSupportObservation{MemoryID: eligible.Memory.ID, SourceVersion: eligible.Memory.SourceVersion, ClaimFingerprint: retrieval.ResidentClaimFingerprint(eligible.Memory.Content), Actor: "fixture-operator", Stable: true, Applicable: true, Stage: 1, ExpiresAt: supportAt.Add(time.Hour)}
	supportEvent := residentWriteObservation(t, ctx, repo, scope, "user_correction", support, supportAt)
	utilityAt := time.Now().UTC()
	utility := retrieval.ResidentUtilityObservation{MemoryID: eligible.Memory.ID, SourceVersion: eligible.Memory.SourceVersion, ClaimFingerprint: support.ClaimFingerprint, Actor: "fixture-held-out-judge", Stage: 2, ExpiresAt: utilityAt.Add(time.Hour), Useful: true}
	utilityEvent := residentWriteObservation(t, ctx, repo, scope, "external_evaluation", utility, utilityAt)
	in := retrieval.ResidentEvaluationInput{Scope: scope, EvaluatedAt: time.Now().UTC(), Policy: memory.DefaultContextProjectionPolicy("resident-owned-v1"), Limits: retrieval.DefaultResidentEvaluationLimits(), Sources: append([]retrieval.ResidentSource{eligible, popular}, foreignSources...), CurrentResidentIDs: []string{popular.Memory.ID}, Support: []memory.RawEvent{supportEvent}, Utility: []memory.RawEvent{utilityEvent}}
	before := residentSourceChecksum(t, ctx, pool)
	result, err := retrieval.EvaluateResidentContext(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if result.Report.Baseline.Items != 2 || result.Report.Candidate.Items != 1 || result.Report.Promoted != 1 || result.Report.Demoted != 1 || result.Report.Omissions[retrieval.ResidentOmissionSafety] != 3 || result.Report.OutcomeStatus != "inconclusive" {
		t.Fatalf("unexpected resident conformance aggregates: %+v", result.Report)
	}
	baseline, candidate := result.BaselineProjection(), result.CandidateProjection()
	residentAssertPlanSources(t, baseline, []retrieval.ResidentSource{eligible, popular})
	residentAssertPlanSources(t, candidate, []retrieval.ResidentSource{eligible})
	if !reflect.DeepEqual(result.SelectedProjection(true), baseline) || !reflect.DeepEqual(result.SelectedProjection(false), candidate) {
		t.Fatal("actual disabled selection differs from baseline")
	}
	if err := result.Report.Validate(); err != nil {
		t.Fatal(err)
	}
	// Persist both conditions with exactly the same policy identity and limits.
	// The candidate supersedes the baseline; scoped SQL below proves the older
	// condition's immutable rows remain accessible for maintenance comparison.
	for _, plan := range []memory.ContextProjection{baseline, candidate} {
		if _, err := repo.CreateContextProjection(ctx, plan); err != nil {
			t.Fatal(err)
		}
	}
	residentVerifyStoredPlans(t, ctx, pool, repo)
	if residentSourceChecksum(t, ctx, pool) != before {
		t.Fatal("evaluation or projection persistence changed canonical history or raw evidence")
	}
	// Append a real immutable version inside a transaction. Refreshing the
	// canonical source version while keeping the old resolved snapshot fails
	// closed; resolving the new version also rejects the old support claim.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO memory_versions (id,memory_id,version,state,content,created_at,modified_by,temporal_fact_id,ingested_at,valid_from,validity_source) SELECT $1,memory_id,2,state,content,$3,'resident-owned-drift',temporal_fact_id,ingested_at,valid_from,validity_source FROM memory_versions WHERE memory_id=$2 AND version=1`, uuid.NewString(), eligible.Memory.ID, in.EvaluatedAt); err != nil {
		t.Fatal(err)
	}
	drift := in
	drift.Sources = append([]retrieval.ResidentSource{}, in.Sources...)
	if err := tx.QueryRow(ctx, `SELECT max(v.version) FROM memory_versions v JOIN canonical_memories m ON m.id=v.memory_id WHERE m.id=$1 AND m.tenant=$2 AND m.project=$3 AND m.namespace=$4`, eligible.Memory.ID, scope.Tenant, scope.Project, scope.Namespace).Scan(&drift.Sources[0].Memory.SourceVersion); err != nil {
		t.Fatal(err)
	}
	unsafe, err := retrieval.EvaluateResidentContext(ctx, drift)
	if err != nil || unsafe.Report.Candidate.Items != 0 || unsafe.Report.Omissions[retrieval.ResidentOmissionSafety] != 4 {
		t.Fatalf("source drift did not fail closed: %+v err=%v", unsafe.Report, err)
	}
	var driftMetadata []byte
	if err := tx.QueryRow(ctx, `SELECT v.id::text,v.version,v.created_at,v.content,v.state,v.source_metadata FROM memory_versions v JOIN canonical_memories m ON m.id=v.memory_id WHERE v.memory_id=$1 AND v.version=2 AND m.tenant=$2 AND m.project=$3 AND m.namespace=$4`, eligible.Memory.ID, scope.Tenant, scope.Project, scope.Namespace).Scan(&drift.Sources[0].Version.ID, &drift.Sources[0].Version.Version, &drift.Sources[0].Version.CreatedAt, &drift.Sources[0].Version.Content, &drift.Sources[0].Version.State, &driftMetadata); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(driftMetadata, &drift.Sources[0].Version.SourceMetadata); err != nil {
		t.Fatal(err)
	}
	unsafe, err = retrieval.EvaluateResidentContext(ctx, drift)
	if err != nil || unsafe.Report.Candidate.Items != 0 || unsafe.Report.Omissions[retrieval.ResidentOmissionSupport] != 2 {
		t.Fatalf("stale support admitted new version: %+v err=%v", unsafe.Report, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if residentSourceChecksum(t, ctx, pool) != before {
		t.Fatal("drift fault injection escaped rollback")
	}
	if reportPath := os.Getenv("STELE_TEST_RESIDENT_REPORT_PATH"); reportPath != "" {
		data, err := json.Marshal(result.Report)
		if err != nil || len(data) > 4096 {
			t.Fatal("resident aggregate report exceeds bound")
		}
		if err := os.WriteFile(reportPath, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func residentForeignScope(dimension int) memory.Scope {
	s := residentOwnedScope
	switch dimension {
	case 0:
		s.Tenant += "-foreign"
	case 1:
		s.Project += "-foreign"
	case 2:
		s.Namespace += "-foreign"
	}
	return s
}

func residentWriteObservation(t *testing.T, ctx context.Context, repo *postgres.Repository, scope memory.Scope, kind string, body any, at time.Time) memory.RawEvent {
	t.Helper()
	content, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	event, err := repo.IngestEvent(ctx, memory.IngestEventInput{Scope: scope, EventType: kind, Content: string(content), SourceTimestamp: at, SourceMetadata: memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustAttestedData, Classification: memory.SourceClassificationLow}}, memory.ProvenanceRecord{ID: uuid.NewString(), Scope: scope, Operation: "resident_owned_observation", Actor: "resident-owned-harness", CreatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func residentSeedSource(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *postgres.Repository, scope memory.Scope, content string) retrieval.ResidentSource {
	t.Helper()
	// This synthetic source explicitly starts with an attributable low-class
	// event. Canonical metadata is copied from that actual scoped raw row and
	// the database trigger snapshots it onto the immutable version.
	event := residentWriteObservation(t, ctx, repo, scope, "tool_observation", content, time.Now().UTC().Add(-time.Second))
	id := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO canonical_memories (id,tenant,project,namespace,class,state,content,source_metadata,temporal_fact_id,ingested_at,valid_from,validity_source) SELECT $1::uuid,tenant,project,namespace,'profile','active',$2,source_metadata,($1::uuid)::text,now(),now(),$7 FROM raw_events WHERE id=$3 AND tenant=$4 AND project=$5 AND namespace=$6`, id, content, event.ID, scope.Tenant, scope.Project, scope.Namespace, string(memory.TemporalValiditySourceLegacyCompatible)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO memory_versions (id,memory_id,version,state,content,modified_by,temporal_fact_id,ingested_at,valid_from,validity_source) SELECT $1,id,1,'active',content,'resident-owned-harness',temporal_fact_id,ingested_at,valid_from,validity_source FROM canonical_memories WHERE id=$2`, uuid.NewString(), id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO provenance_links (id,raw_event_id,memory_id,tenant,project,namespace,operation,actor,source_context) VALUES ($1,$2,$3,$4,$5,$6,'resident_owned_source','resident-owned-harness','{}'::jsonb)`, uuid.NewString(), event.ID, id, scope.Tenant, scope.Project, scope.Namespace); err != nil {
		t.Fatal(err)
	}
	history, err := repo.ReadMemoryHistory(ctx, scope, id, true)
	if err != nil || len(history.Versions) != 1 || len(history.Provenance) != 1 || history.Provenance[0].RawEventID != event.ID {
		t.Fatalf("persisted source history unresolved: versions=%d provenance=%d err=%v", len(history.Versions), len(history.Provenance), err)
	}
	s := retrieval.ResidentSource{Memory: history.Memory, Version: history.Versions[0], Confidence: 1}
	var canonicalMetadata, versionMetadata, rawMetadata []byte
	// ReadMemoryHistory intentionally omits disclosure labels. Resolve the
	// actual persisted lineage instead of assigning a permissive default.
	if err := pool.QueryRow(ctx, `SELECT m.source_metadata,v.source_metadata,e.source_metadata,v.version FROM canonical_memories m JOIN memory_versions v ON v.memory_id=m.id JOIN provenance_links p ON p.memory_id=m.id AND p.tenant=m.tenant AND p.project=m.project AND p.namespace=m.namespace JOIN raw_events e ON e.id=p.raw_event_id AND e.tenant=m.tenant AND e.project=m.project AND e.namespace=m.namespace WHERE m.id=$1 AND m.tenant=$2 AND m.project=$3 AND m.namespace=$4 ORDER BY v.version DESC LIMIT 1`, id, scope.Tenant, scope.Project, scope.Namespace).Scan(&canonicalMetadata, &versionMetadata, &rawMetadata, &s.Memory.SourceVersion); err != nil {
		t.Fatal(err)
	}
	var raw memory.SourceMetadata
	if json.Unmarshal(canonicalMetadata, &s.Memory.SourceMetadata) != nil || json.Unmarshal(versionMetadata, &s.Version.SourceMetadata) != nil || json.Unmarshal(rawMetadata, &raw) != nil || !raw.Disclosable() || s.Memory.SourceMetadata != raw || s.Version.SourceMetadata != raw {
		t.Fatal("persisted disclosure lineage mismatch")
	}
	return s
}

func residentAssertPlanSources(t *testing.T, plan memory.ContextProjection, sources []retrieval.ResidentSource) {
	t.Helper()
	if len(plan.Items) != len(sources) {
		t.Fatalf("selected items=%d want=%d", len(plan.Items), len(sources))
	}
	byID := map[string]retrieval.ResidentSource{}
	for _, source := range sources {
		byID[source.Memory.ID] = source
	}
	for _, item := range plan.Items {
		source, ok := byID[item.Source.MemoryID]
		if !ok || item.Source.Scope != plan.Scope || item.Source.ID != source.Version.ID || item.Source.Version != source.Memory.SourceVersion || item.Text != source.Memory.Content || item.Citation.MemoryID != source.Memory.ID || item.Citation.SourceVersion != source.Version.Version || item.Citation.Operation != "context_projection" {
			t.Fatal("selected reference or citation differs from persisted immutable source")
		}
		delete(byID, source.Memory.ID)
	}
	if len(byID) != 0 {
		t.Fatal("selection omitted exact source")
	}
}

func residentVerifyStoredPlans(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *postgres.Repository) {
	t.Helper()
	s := residentOwnedScope
	latest, err := repo.ReadLatestContextProjectionForMaintenance(ctx, s, memory.ContextProjectionKindAlwaysVisible)
	if err != nil || latest.Version != 2 || len(latest.Items) != 1 {
		t.Fatalf("persisted candidate unavailable: version=%d items=%d err=%v", latest.Version, len(latest.Items), err)
	}
	for dimension := 0; dimension < 3; dimension++ {
		if _, err := repo.ReadContextProjection(ctx, residentForeignScope(dimension), latest.ID); err == nil {
			t.Fatal("projection crossed isolation boundary")
		}
	}
	var projections, baselineItems, candidateItems, references, sources, events int
	if err := pool.QueryRow(ctx, `SELECT count(*), (SELECT count(*) FROM context_projection_items i JOIN context_projections p ON p.id=i.projection_id WHERE p.tenant=$1 AND p.project=$2 AND p.namespace=$3 AND p.version=1 AND p.status='superseded'), (SELECT count(*) FROM context_projection_items i JOIN context_projections p ON p.id=i.projection_id WHERE p.tenant=$1 AND p.project=$2 AND p.namespace=$3 AND p.version=2), (SELECT count(*) FROM context_projection_items i JOIN memory_versions v ON v.id=i.source_id AND v.version=i.source_version AND v.memory_id=i.memory_id JOIN canonical_memories m ON m.id=v.memory_id AND m.tenant=i.tenant AND m.project=i.project AND m.namespace=i.namespace WHERE i.tenant=$1 AND i.project=$2 AND i.namespace=$3 AND i.rendered_text=v.content AND i.citation->>'memory_id'=m.id::text AND (i.citation->>'source_version')::bigint=v.version), (SELECT count(*) FROM canonical_memories m JOIN memory_versions v ON v.memory_id=m.id JOIN provenance_links l ON l.memory_id=m.id JOIN raw_events e ON e.id=l.raw_event_id WHERE m.tenant=$1 AND m.project=$2 AND m.namespace=$3 AND m.source_metadata=v.source_metadata AND v.source_metadata=e.source_metadata), (SELECT count(*) FROM raw_events WHERE tenant=$1 AND project=$2 AND namespace=$3) FROM context_projections WHERE tenant=$1 AND project=$2 AND namespace=$3`, s.Tenant, s.Project, s.Namespace).Scan(&projections, &baselineItems, &candidateItems, &references, &sources, &events); err != nil || projections != 2 || baselineItems != 2 || candidateItems != 1 || references != 3 || sources != 2 || events != 4 {
		t.Fatalf("persisted history incomplete: projections=%d baseline=%d candidate=%d references=%d sources=%d raw=%d err=%v", projections, baselineItems, candidateItems, references, sources, events, err)
	}
	for _, item := range latest.Items {
		history, err := repo.ReadMemoryHistory(ctx, s, item.Source.MemoryID, true)
		if err != nil || len(history.Versions) != 1 || len(history.Provenance) != 1 || history.Versions[0].ID != item.Source.ID || history.Versions[0].Version != item.Citation.SourceVersion {
			t.Fatal("candidate source history or provenance unavailable after persistence")
		}
	}
}

func residentSourceChecksum(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	var checksum string
	if err := pool.QueryRow(ctx, `SELECT md5((SELECT string_agg(to_jsonb(m)::text,',' ORDER BY id) FROM canonical_memories m)|| (SELECT string_agg(to_jsonb(v)::text,',' ORDER BY id) FROM memory_versions v)|| (SELECT string_agg(to_jsonb(e)::text,',' ORDER BY id) FROM raw_events e)|| (SELECT string_agg(to_jsonb(p)::text,',' ORDER BY id) FROM provenance_links p))`).Scan(&checksum); err != nil {
		t.Fatal(err)
	}
	return checksum
}
