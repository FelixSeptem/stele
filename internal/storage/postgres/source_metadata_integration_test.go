package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestSourceMetadataPostgresGovernanceSnapshotsAndEligibility(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_DISCLOSURE_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_DISCLOSURE_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
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
	scope := memory.Scope{Tenant: uuid.NewString(), Project: "pc2", Namespace: "test"}
	now := time.Now().UTC()
	for _, classification := range []memory.SourceClassification{memory.SourceClassificationLow, memory.SourceClassificationHigh} {
		labels := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: classification}
		event, err := repo.WriteRawEvent(ctx, memory.IngestEventInput{Scope: scope, MemoryPath: "pc2/non-root", EventType: "conversation.message", Content: "pc2marker", SourceMetadata: labels})
		if err != nil {
			t.Fatal(err)
		}
		candidate := governance.CandidateMemory{ID: uuid.NewString(), SourceRawEventID: event.ID, Scope: scope, MemoryPath: event.MemoryPath, Class: memory.MemoryClassEpisodic, Content: event.Content, Confidence: 0.8, Importance: 0.5, Freshness: 0.5, Sensitivity: governance.SensitivityLow, Mutability: governance.MutabilityImmutable, RetentionClass: policy.RetentionClassSession, Status: governance.CandidateStatusPending, CreatedAt: now, UpdatedAt: now, SourceMetadata: labels}
		candidate, err = repo.CreateCandidate(ctx, candidate, memory.ProvenanceRecord{ID: uuid.NewString(), Scope: scope, Operation: "extract", CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		canonical, version, err := repo.PromoteCandidate(ctx, governance.CanonicalPromotion{MemoryID: uuid.NewString(), VersionID: uuid.NewString(), Candidate: candidate, CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"raw_events", "candidate_memories", "canonical_memories", "memory_versions"} {
			id := event.ID
			if table == "candidate_memories" {
				id = candidate.ID
			}
			if table == "canonical_memories" {
				id = canonical.ID
			}
			if table == "memory_versions" {
				id = version.ID
			}
			var stored memory.SourceMetadata
			if err := pool.QueryRow(ctx, "SELECT source_metadata FROM "+table+" WHERE id=$1", id).Scan(&stored); err != nil {
				t.Fatal(err)
			}
			if stored != labels {
				t.Fatalf("%s labels=%+v, want %+v", table, stored, labels)
			}
		}
	}
	hits, err := repo.SearchLexical(ctx, retrieval.SearchInput{Scope: scope, Query: "pc2marker", TopK: 1, DisclosureRequired: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Memory.SourceMetadata.Classification != memory.SourceClassificationLow || hits[0].Memory.MemoryPath != "pc2/non-root" || hits[0].Memory.SourceVersion != 1 {
		t.Fatalf("disclosed hits=%+v", hits)
	}
}

func TestSourceMetadataPostgresMixedDerivationsAndVersionHistory(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_DISCLOSURE_DSN")
	if dsn == "" {
		t.Skip("owned PostgreSQL disclosure fixture not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
	scope := memory.Scope{Tenant: uuid.NewString(), Project: "pc2", Namespace: "derived"}
	now := time.Now().UTC()
	attested := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustAttestedData, Classification: memory.SourceClassificationLow}
	untrusted := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationMedium}
	want := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationHigh}
	events := []memory.RawEvent{}
	for _, labels := range []memory.SourceMetadata{attested, untrusted} {
		event, err := repo.WriteRawEvent(ctx, memory.IngestEventInput{Scope: scope, MemoryPath: "pc2/mixed", EventType: "conversation.message", Content: "mixed source evidence", SourceMetadata: labels})
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
		candidate := governance.CandidateMemory{ID: uuid.NewString(), SourceRawEventID: event.ID, Scope: scope, MemoryPath: event.MemoryPath, Class: memory.MemoryClassEpisodic, Content: event.Content, Confidence: 0.8, Importance: 0.5, Freshness: 0.5, Sensitivity: governance.SensitivityHigh, Mutability: governance.MutabilityImmutable, RetentionClass: policy.RetentionClassSession, Status: governance.CandidateStatusPending, CreatedAt: now, UpdatedAt: now, SourceMetadata: labels}
		if _, err := repo.CreateCandidate(ctx, candidate, memory.ProvenanceRecord{ID: uuid.NewString(), Scope: scope, Operation: "extract", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	summary, version, err := repo.CreateSummaryMemory(ctx, governance.SummaryMemoryRecord{MemoryID: uuid.NewString(), VersionID: uuid.NewString(), Scope: scope, Class: memory.MemoryClassSummary, Content: "mixed source summary", EvidenceRawEventIDs: []string{events[0].ID, events[1].ID}, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("summary_inherits_candidate_floor", func(t *testing.T) {
		if summary.SourceMetadata != want || version.SourceMetadata != want {
			t.Fatalf("summary/version labels=%+v/%+v, want %+v", summary.SourceMetadata, version.SourceMetadata, want)
		}
	})
	check := func(t *testing.T, table, key, id string, expected memory.SourceMetadata) {
		t.Helper()
		var got memory.SourceMetadata
		if err := pool.QueryRow(ctx, "SELECT source_metadata FROM "+table+" WHERE "+key+"=$1", id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != expected {
			t.Fatalf("%s labels=%+v, want %+v", table, got, expected)
		}
	}
	t.Run("chunks_resolve_exact_version", func(t *testing.T) {
		chunks, err := memory.ChunkText(memory.ChunkingInput{Source: memory.ChunkSourceReference{Kind: memory.ChunkSourceKindCanonicalVersion, ID: version.ID, MemoryID: summary.ID, Version: 1, Scope: scope}, Scope: scope, Class: memory.MemoryClassSummary, Content: summary.Content, Policy: memory.DefaultChunkPolicy("pc2", "pc2")})
		if err != nil {
			t.Fatal(err)
		}
		stored, err := repo.CreateMemoryChunks(ctx, chunks, "whitespace-v1")
		if err != nil || len(stored) == 0 {
			t.Fatalf("chunk persistence: %v", err)
		}
		check(t, "memory_chunk_items", "id", stored[0].ID, want)
		var labels memory.SourceMetadata
		for _, source := range []struct {
			id      string
			version int64
			tenant  string
		}{{version.ID, 99, scope.Tenant}, {version.ID, 1, "foreign"}, {uuid.NewString(), 1, scope.Tenant}} {
			if err := pool.QueryRow(ctx, `SELECT stele_source_snapshot('canonical_version',$1,$2,$3,$4,$5)`, source.id, source.version, source.tenant, scope.Project, scope.Namespace).Scan(&labels); err != nil {
				t.Fatal(err)
			}
			if labels != (memory.SourceMetadata{}).Effective() {
				t.Fatal("missing, foreign or stale source snapshot became permissive")
			}
		}
	})
	t.Run("projection_joins_items_atomically", func(t *testing.T) {
		projection, err := memory.MaterializeContextProjection(ctx, memory.MaterializeContextProjectionInput{Scope: scope, Kind: memory.ContextProjectionKindRetrieval, Version: 1, SchemaVersion: "pc2", RendererVersion: "pc2", Policy: memory.DefaultContextProjectionPolicy("pc2"), Sources: []memory.ContextProjectionCandidate{{Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: version.ID, MemoryID: summary.ID, Version: 1, Scope: scope}, Class: memory.MemoryClassSummary, State: memory.MemoryStateActive, Content: summary.Content, Confidence: 1}}})
		if err != nil {
			t.Fatal(err)
		}
		stored, err := repo.CreateContextProjection(ctx, projection)
		if err != nil {
			t.Fatal(err)
		}
		check(t, "context_projections", "id", stored.ID, want)
		check(t, "context_projection_items", "projection_id", stored.ID, want)
	})
	for _, insightType := range []memory.DerivedInsightType{
		memory.DerivedInsightTypeFailurePattern, memory.DerivedInsightTypeLesson,
		memory.DerivedInsightTypeHypothesis, memory.DerivedInsightTypeGoal,
		memory.DerivedInsightTypeContradiction, memory.DerivedInsightTypeCausalLink,
	} {
		t.Run("insights_join_mixed_and_incomplete_evidence/"+string(insightType), func(t *testing.T) {
			insight := testDerivedInsight()
			insight.Type = insightType
			if !insightType.ActiveSupported() {
				insight.State = memory.DerivedInsightStateCandidate
			}
			if insightType == memory.DerivedInsightTypeLesson {
				insight.Lesson = &memory.DerivedInsightLesson{SourceFailurePatternID: uuid.NewString(), Guidance: "model-generated text cannot self-attest"}
			}
			insight.ID, insight.Scope = uuid.NewString(), scope
			insight.Derivation.Fingerprint = uuid.NewString()
			insight.Evidence = []memory.DerivedInsightEvidenceRef{{Kind: memory.DerivedInsightEvidenceKindRawEvent, ID: events[0].ID, Relation: memory.DerivedInsightEvidenceRelationSupports}, {Kind: memory.DerivedInsightEvidenceKindCanonicalMemory, ID: summary.ID, Relation: memory.DerivedInsightEvidenceRelationSupports}}
			stored, err := repo.UpsertDerivedInsight(ctx, insight)
			if err != nil {
				t.Fatal(err)
			}
			check(t, "derived_insights", "id", stored.ID, want)
			var evidenceVersion int64
			if err := pool.QueryRow(ctx, `SELECT source_version FROM derived_insight_evidence WHERE insight_id=$1 AND evidence_id=$2`, stored.ID, summary.ID).Scan(&evidenceVersion); err != nil || evidenceVersion != 1 {
				t.Fatalf("evidence version=%d err=%v", evidenceVersion, err)
			}
			insight.Evidence = append(insight.Evidence, memory.DerivedInsightEvidenceRef{Kind: memory.DerivedInsightEvidenceKindRawEvent, ID: uuid.NewString(), Relation: memory.DerivedInsightEvidenceRelationSupports})
			if _, err := repo.UpsertDerivedInsight(ctx, insight); err != nil {
				t.Fatal(err)
			}
			check(t, "derived_insights", "id", stored.ID, (memory.SourceMetadata{}).Effective())
		})
	}
	t.Run("canonical_update_cannot_weaken_old_versions", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE canonical_memories SET source_metadata=$2 WHERE id=$1`, summary.ID, attested)
		if err != nil {
			t.Fatal(err)
		}
		check(t, "canonical_memories", "id", summary.ID, want)
		check(t, "memory_versions", "id", version.ID, want)
		if _, err := pool.Exec(ctx, `UPDATE memory_versions SET source_metadata=$2 WHERE id=$1`, version.ID, attested); err == nil {
			t.Fatal("immutable version labels overwritten")
		}
	})
}

func TestSourceMetadataPostgresPC1UpgradeConservativeDefaults(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_DISCLOSURE_DSN")
	if dsn == "" {
		t.Skip("owned PostgreSQL disclosure fixture not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	schema := pgx.Identifier{"pc2_upgrade_" + strings.ReplaceAll(uuid.NewString(), "-", "")}.Sanitize()
	if _, err := tx.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SET LOCAL search_path TO "+schema+", public"); err != nil {
		t.Fatal(err)
	}
	manifest, err := MigrationManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range manifest[:34] {
		sql, err := migrationFS.ReadFile("migrations/" + asset.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("PC1 schema migration %d: %v", asset.Version, err)
		}
	}
	principalID, grantID, eventID, memoryID, versionID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, fixture := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO access_principals(id,role,status,label) VALUES($1,'public','active','legacy')`, []any{principalID}},
		{`INSERT INTO access_scope_grants(id,principal_id,tenant,project,namespace,status) VALUES($1,$2,'t','p','n','active')`, []any{grantID, principalID}},
		{`INSERT INTO raw_events(id,tenant,project,namespace,event_type,content) VALUES($1,'t','p','n','conversation.message','legacy')`, []any{eventID}},
		{`INSERT INTO canonical_memories(id,tenant,project,namespace,class,state,content) VALUES($1,'t','p','n','episodic','active','legacy')`, []any{memoryID}},
		{`INSERT INTO memory_versions(id,memory_id,version,state,content,modified_by) VALUES($1,$2,1,'active','legacy','legacy')`, []any{versionID, memoryID}},
	} {
		if _, err := tx.Exec(ctx, fixture.sql, fixture.args...); err != nil {
			t.Fatal(err)
		}
	}
	sql, err := migrationFS.ReadFile("migrations/" + manifest[34].Name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	var profiles []string
	var defaultProfile string
	var attestation bool
	if err := tx.QueryRow(ctx, `SELECT allowed_disclosure_profiles,default_disclosure_profile,source_attestation FROM access_scope_grants WHERE id=$1`, grantID).Scan(&profiles, &defaultProfile, &attestation); err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 || defaultProfile != "" || attestation {
		t.Fatal("upgrade implicitly granted disclosure or attestation")
	}
	for table, id := range map[string]string{"raw_events": eventID, "canonical_memories": memoryID, "memory_versions": versionID} {
		var labels memory.SourceMetadata
		if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT source_metadata FROM %s WHERE id=$1", table), id).Scan(&labels); err != nil {
			t.Fatal(err)
		}
		if labels != (memory.SourceMetadata{}).Effective() {
			t.Fatalf("legacy %s became permissive", table)
		}
	}
}

func TestSourceMetadataPostgresPromotionMutationAndHiddenSummary(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_DISCLOSURE_DSN")
	if dsn == "" {
		t.Skip("owned PostgreSQL disclosure fixture not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
	scope := memory.Scope{Tenant: uuid.NewString(), Project: "pc2", Namespace: "branches"}
	now := time.Now().UTC()
	low := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustAttestedData, Classification: memory.SourceClassificationLow}
	high := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationHigh}
	check := func(t *testing.T, table, id string, want memory.SourceMetadata) {
		t.Helper()
		var got memory.SourceMetadata
		if err := pool.QueryRow(ctx, "SELECT source_metadata FROM "+table+" WHERE id=$1", id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s labels=%+v want=%+v", table, got, want)
		}
	}
	promote := func(t *testing.T, labels memory.SourceMetadata, id string) (memory.RawEvent, memory.CanonicalMemory, memory.MemoryVersion) {
		t.Helper()
		event, err := repo.WriteRawEvent(ctx, memory.IngestEventInput{Scope: scope, MemoryPath: "pc2/branches", EventType: "conversation.message", Content: "branchmarker", SourceMetadata: labels})
		if err != nil {
			t.Fatal(err)
		}
		candidate := governance.CandidateMemory{ID: uuid.NewString(), SourceRawEventID: event.ID, Scope: scope, MemoryPath: event.MemoryPath, Class: memory.MemoryClassEpisodic, Content: event.Content, Confidence: 0.8, Importance: 0.5, Freshness: 0.5, Sensitivity: governance.SensitivityLow, Mutability: governance.MutabilityImmutable, RetentionClass: policy.RetentionClassSession, Status: governance.CandidateStatusPending, CreatedAt: now, UpdatedAt: now}
		candidate, err = repo.CreateCandidate(ctx, candidate, memory.ProvenanceRecord{ID: uuid.NewString(), Scope: scope, Operation: "extract", CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		if id == "" {
			id = uuid.NewString()
		}
		canonical, version, err := repo.PromoteCandidate(ctx, governance.CanonicalPromotion{MemoryID: id, VersionID: uuid.NewString(), Candidate: candidate, CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		if canonical.MemoryPath != event.MemoryPath || version.MemoryPath != event.MemoryPath {
			t.Fatal("promotion lost non-root path")
		}
		var links int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM provenance_links WHERE memory_id=$1 AND raw_event_id=$2 AND candidate_memory_id=$3`, id, event.ID, candidate.ID).Scan(&links); err != nil || links != 1 {
			t.Fatalf("promotion provenance=%d err=%v", links, err)
		}
		return event, canonical, version
	}
	t.Run("superseding_retains_old_version_and_floor", func(t *testing.T) {
		_, canonical, old := promote(t, high, "")
		_, next, version := promote(t, low, canonical.ID)
		if version.Version != 2 || next.SourceMetadata != high || version.SourceMetadata != high {
			t.Fatal("superseding weakened labels")
		}
		check(t, "memory_versions", old.ID, high)
	})
	t.Run("manual_create_and_update_are_unattributed", func(t *testing.T) {
		id := uuid.NewString()
		versionID := uuid.NewString()
		_, err := repo.CreateMemory(ctx, memory.ManualCreateMemoryRecord{MemoryID: id, VersionID: versionID, Scope: scope, Class: memory.MemoryClassEpisodic, MemoryPath: "pc2/manual", Content: "manualmarker", Actor: "operator", Reason: "fixture", RequestID: uuid.NewString(), CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		check(t, "canonical_memories", id, (memory.SourceMetadata{}).Effective())
		check(t, "memory_versions", versionID, (memory.SourceMetadata{}).Effective())
		_, canonical, old := promote(t, low, "")
		nextID := uuid.NewString()
		_, err = repo.UpdateMemory(ctx, memory.ManualUpdateMemoryRecord{MemoryID: canonical.ID, VersionID: nextID, Scope: scope, Content: "new unattributed manual text", ExpectedVersion: 1, Actor: "operator", Reason: "fixture", RequestID: uuid.NewString(), UpdatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		check(t, "canonical_memories", canonical.ID, (memory.SourceMetadata{}).Effective())
		check(t, "memory_versions", nextID, (memory.SourceMetadata{}).Effective())
		check(t, "memory_versions", old.ID, low)
	})
	t.Run("merge_and_reclassification_preserve_history", func(t *testing.T) {
		_, target, old := promote(t, low, "")
		_, source, _ := promote(t, high, "")
		mergedVersion := uuid.NewString()
		_, err := repo.MergeMemory(ctx, memory.ManualMergeMemoryRecord{TargetMemoryID: target.ID, SourceMemoryID: source.ID, VersionID: mergedVersion, Scope: scope, Content: "manually merged evidence", ExpectedVersion: 1, Actor: "operator", Reason: "fixture", RequestID: uuid.NewString(), AppliedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		check(t, "canonical_memories", target.ID, high)
		check(t, "memory_versions", old.ID, low)
		check(t, "memory_versions", mergedVersion, high)
		_, err = repo.ReclassifyMemory(ctx, memory.ManualReclassifyMemoryRecord{MemoryID: target.ID, VersionID: uuid.NewString(), Scope: scope, TargetClass: memory.MemoryClassProcedural, ExpectedVersion: 2, Actor: "operator", Reason: "fixture", RequestID: uuid.NewString(), AppliedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		check(t, "canonical_memories", target.ID, high)
	})
	t.Run("hidden_summary_source_is_ineligible_before_limit", func(t *testing.T) {
		event, source, _ := promote(t, low, "")
		summary, _, err := repo.CreateSummaryMemory(ctx, governance.SummaryMemoryRecord{MemoryID: uuid.NewString(), VersionID: uuid.NewString(), Scope: scope, Class: memory.MemoryClassSummary, Content: "summaryonlymarker", EvidenceRawEventIDs: []string{event.ID}, CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		hits, err := repo.SearchLexical(ctx, retrieval.SearchInput{Scope: scope, Query: "summaryonlymarker", TopK: 1, DisclosureRequired: true})
		if err != nil || len(hits) != 1 {
			t.Fatalf("visible summary hits=%d err=%v", len(hits), err)
		}
		if _, err = pool.Exec(ctx, `UPDATE canonical_memories SET state='suppressed' WHERE id=$1`, source.ID); err != nil {
			t.Fatal(err)
		}
		hits, err = repo.SearchLexical(ctx, retrieval.SearchInput{Scope: scope, Query: "summaryonlymarker", TopK: 1, DisclosureRequired: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 0 {
			t.Fatalf("summary %s disclosed a hidden source", summary.ID)
		}
	})
}
