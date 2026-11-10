package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/benchmark"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The harness-owned DSN is the only database accepted. Reports and source
// history survive until the second phase after a real container restart.
func TestRepresentationEvaluationPostgresOwnedReplayAndScope(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("STELE_TEST_REPRESENTATION_EVALUATION_DSN"))
	if dsn == "" {
		t.Skip("owned representation DSN is not configured")
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
	payload, err := os.ReadFile(filepath.Join("..", "..", "benchmark", "testdata", "task-appropriate-representation-fixture-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := benchmark.LoadRepresentationFixture(payload)
	if err != nil {
		t.Fatal(err)
	}
	scope, now := fixture.Cases[0].Scope, time.Now().UTC()
	if os.Getenv("STELE_TEST_REPRESENTATION_PHASE") == "post_restart" {
		reports, err := repo.ListRepresentationEvaluationReports(ctx, scope, 10)
		if err != nil || len(reports) != 4 {
			t.Fatalf("reports after restart=%d err=%v", len(reports), err)
		}
		if err := retrieval.ValidateRepresentationReleaseEvidence(scope, reports, now, now.Add(5*time.Minute)); err != nil {
			t.Fatal(err)
		}
		var histories int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM memory_versions v JOIN canonical_memories m ON m.id=v.memory_id WHERE m.tenant=$1 AND m.project=$2 AND m.namespace=$3`, scope.Tenant, scope.Project, scope.Namespace).Scan(&histories); err != nil || histories < 11 {
			t.Fatalf("history after restart=%d err=%v", histories, err)
		}
		deleted, err := repo.CleanupRepresentationEvaluationReports(ctx, scope, now.Add(2*time.Hour), 10)
		if err != nil || deleted != 4 {
			t.Fatalf("cleanup=%d err=%v", deleted, err)
		}
		remaining, err := repo.ListRepresentationEvaluationReports(ctx, scope, 10)
		if err != nil || len(remaining) != 0 {
			t.Fatalf("remaining reports=%d err=%v", len(remaining), err)
		}
		return
	}
	corpus := retrieval.EvaluationFixture{Version: fixture.SchemaVersion}
	for _, item := range fixture.Cases {
		corpus.Cases = append(corpus.Cases, retrieval.EvaluationCase{ID: item.ID, Scope: item.Scope, Query: item.Query, Sources: item.Sources, ExpectedEvidenceGroups: [][]string{item.RequiredSourceAliases}, ExcludedAliases: item.ExcludedSourceAliases})
	}
	seed, err := postgres.NewEvaluationFixtureSeeder(repo).SeedBatch(ctx, corpus, 16)
	if err != nil {
		t.Fatal(err)
	}
	// Matching text across each of the three isolation boundaries must be absent.
	for dimension := 0; dimension < 3; dimension++ {
		foreign := scope
		switch dimension {
		case 0:
			foreign.Tenant += "-foreign"
		case 1:
			foreign.Project += "-foreign"
		case 2:
			foreign.Namespace += "-foreign"
		}
		alias := fmt.Sprintf("foreign-%d", dimension)
		foreignFixture := retrieval.EvaluationFixture{Version: fixture.SchemaVersion, Cases: []retrieval.EvaluationCase{{ID: alias, Scope: foreign, Query: "exactfact", Sources: []retrieval.EvaluationSource{{Alias: alias, EventType: "fixture", State: memory.MemoryStateActive, Content: "exactfact crosslink profilehint procedurehint wrong=999"}}, ExpectedEvidenceGroups: [][]string{{alias}}}}}
		if _, err := postgres.NewEvaluationFixtureSeeder(repo).SeedBatch(ctx, foreignFixture, 10); err != nil {
			t.Fatal(err)
		}
	}
	o := &ownedRepresentationObserver{repo: repo, pool: pool, fixture: fixture, seed: seed, now: now, projections: map[string]memory.ContextProjection{}}
	for caseIndex, item := range fixture.Cases {
		var candidates []memory.ContextProjectionCandidate
		for _, alias := range seed.Aliases {
			if alias.CaseID != item.ID {
				continue
			}
			history, err := repo.ReadMemoryHistory(ctx, alias.Scope, alias.MemoryID, true)
			if err != nil {
				t.Fatal(err)
			}
			version := history.Versions[0]
			candidates = append(candidates, memory.ContextProjectionCandidate{Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: version.ID, Version: version.Version, MemoryID: alias.MemoryID, Scope: alias.Scope, LifecycleState: history.Memory.State}, Class: history.Memory.Class, State: history.Memory.State, Content: history.Memory.Content, Confidence: 1, ObservedAt: history.Memory.ModifiedAt})
			if alias.State != memory.MemoryStateActive {
				continue
			}
			hash := sha256.Sum256([]byte(history.Memory.Content))
			generated := time.Now().UTC()
			if err := repo.RecordEmbeddingRebuildRequired(ctx, memory.EmbeddingRebuildRecord{MemoryID: alias.MemoryID, Scope: alias.Scope, Class: history.Memory.Class, Content: history.Memory.Content, SourceVersion: version.Version, ContentHash: "sha256:" + hex.EncodeToString(hash[:]), RequestedProvider: "stele-evaluation", RequestedModel: "representation-vector-v1", RequestedDimensions: 4, Status: memory.EmbeddingRebuildStatusPending, RequestedAt: generated}); err != nil {
				t.Fatal(err)
			}
			vector := make([]float32, 4)
			vector[caseIndex] = 1
			revision := memory.VectorRevision{ID: uuid.NewString(), MemoryID: alias.MemoryID, Scope: alias.Scope, SourceVersion: version.Version, ContentHash: "sha256:" + hex.EncodeToString(hash[:]), Provider: "stele-evaluation", Model: "representation-vector-v1", Dimensions: 4, Embedding: vector, Status: memory.VectorRevisionStatusGenerated, GeneratedAt: generated, ActivatedAt: generated.Add(time.Microsecond), LastRebuildRequest: generated}
			if err := repo.AppendVectorRevision(ctx, revision); err != nil {
				t.Fatal(err)
			}
			if err := repo.PromoteVectorRevision(ctx, revision); err != nil {
				t.Fatal(err)
			}
		}
		if item.Candidate == retrieval.RepresentationCandidateExactExcerptWithSearch {
			continue
		}
		kind := memory.ContextProjectionKindRetrieval
		if item.Family == retrieval.RepresentationTaskFamilyProfilePreference {
			kind = memory.ContextProjectionKindAlwaysVisible
		}
		if item.Family == retrieval.RepresentationTaskFamilyProceduralLesson {
			kind = memory.ContextProjectionKindArchivalHistory
		}
		projection, err := memory.MaterializeContextProjection(ctx, memory.MaterializeContextProjectionInput{Scope: scope, Kind: kind, Version: int64(len(o.projections) + 1), SchemaVersion: "projection-v1", Policy: memory.DefaultContextProjectionPolicy("representation-evaluation-v1"), RendererVersion: "renderer-v1", Sources: candidates})
		if err != nil {
			t.Fatal(err)
		}
		projection, err = repo.CreateContextProjection(ctx, projection)
		if err != nil {
			t.Fatal(err)
		}
		o.projections[item.ID] = projection
	}
	now = time.Now().UTC()
	o.now = now
	before := o.snapshot(t, ctx)
	reports, err := benchmark.ReplayRepresentationFixture(ctx, fixture, "renderer-v1", now, o)
	if err != nil {
		t.Fatal(err)
	}
	if o.baselineCalls != 8 || o.candidateCalls != 8 || o.semanticCases != 16 {
		t.Fatalf("conditions baseline=%d candidate=%d semantic=%d", o.baselineCalls, o.candidateCalls, o.semanticCases)
	}
	expires := now.Add(time.Hour)
	for _, report := range reports {
		if !report.Comparable || !report.RealStack || !report.RollbackTested || report.BaselineMetrics == nil || report.ComparisonDelta == nil {
			t.Fatalf("real comparison failed: %+v", report)
		}
		if err := repo.CreateRepresentationEvaluationReport(ctx, scope, report, &expires); err != nil {
			t.Fatal(err)
		}
		if err := repo.CreateRepresentationEvaluationReport(ctx, scope, report, &expires); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE representation_evaluation_reports SET comparable=false WHERE run_identity=$1`, report.RunIdentity); err == nil {
			t.Fatal("append-only guard accepted UPDATE")
		}
	}
	stored, err := repo.ListRepresentationEvaluationReports(ctx, scope, 10)
	if err != nil || len(stored) != 4 {
		t.Fatalf("stored=%d err=%v", len(stored), err)
	}
	if err := retrieval.ValidateRepresentationReleaseEvidence(scope, stored, now, now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	for dimension := 0; dimension < 3; dimension++ {
		foreign := scope
		switch dimension {
		case 0:
			foreign.Tenant += "-foreign"
		case 1:
			foreign.Project += "-foreign"
		case 2:
			foreign.Namespace += "-foreign"
		}
		rows, err := repo.ListRepresentationEvaluationReports(ctx, foreign, 10)
		if err != nil || len(rows) != 0 {
			t.Fatalf("foreign reports=%d err=%v", len(rows), err)
		}
	}
	if after := o.snapshot(t, ctx); before != after {
		t.Fatal("offline comparison changed canonical history")
	}
	// A new actual source version invalidates the persisted derived snapshot.
	// Confine this fault injection to a rolled-back transaction so retained
	// passing reports still refer to the durable source watermark after restart.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	o.repo = postgres.NewRepository(representationTransaction{tx})
	changedID := ""
	for _, alias := range seed.Aliases {
		if alias.CaseID == "cross" && alias.State == memory.MemoryStateActive {
			changedID = alias.MemoryID
			break
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO memory_versions (id,memory_id,version,state,content,created_at,modified_by,temporal_fact_id,ingested_at,valid_from,valid_to,validity_source,source_metadata) SELECT $1,memory_id,version+1,state,content,CURRENT_TIMESTAMP,modified_by,temporal_fact_id,ingested_at,valid_from,valid_to,validity_source,source_metadata FROM memory_versions WHERE memory_id=$2 ORDER BY version DESC LIMIT 1`, uuid.NewString(), changedID)
	if err != nil {
		t.Fatal(err)
	}
	unsafe, err := benchmark.ReplayRepresentationFixture(ctx, fixture, "renderer-v1", now, o)
	if err != nil {
		t.Fatal(err)
	}
	if unsafe[1].Comparable {
		t.Fatal("changed source version passed real replay")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if after := o.snapshot(t, ctx); before != after {
		t.Fatal("source drift fault injection escaped transaction")
	}
}

type representationTransaction struct{ pgx.Tx }

func (t representationTransaction) BeginTx(ctx context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	return t.Begin(ctx)
}

type ownedRepresentationObserver struct {
	repo                                         *postgres.Repository
	pool                                         *pgxpool.Pool
	fixture                                      benchmark.RepresentationFixture
	seed                                         retrieval.EvaluationFixtureSeed
	now                                          time.Time
	projections                                  map[string]memory.ContextProjection
	baselineCalls, candidateCalls, semanticCases int
}

func (o *ownedRepresentationObserver) snapshot(t *testing.T, ctx context.Context) string {
	t.Helper()
	var checksum string
	s := o.fixture.Cases[0].Scope
	if err := o.pool.QueryRow(ctx, `SELECT md5(string_agg(v.id::text||v.version::text||v.content,',' ORDER BY v.id)) FROM memory_versions v JOIN canonical_memories m ON m.id=v.memory_id WHERE m.tenant=$1 AND m.project=$2 AND m.namespace=$3`, s.Tenant, s.Project, s.Namespace).Scan(&checksum); err != nil {
		t.Fatal(err)
	}
	return checksum
}
func (o *ownedRepresentationObserver) ObserveRepresentation(ctx context.Context, item benchmark.RepresentationFixtureCase, strategy string) (benchmark.RepresentationObservation, error) {
	started := time.Now()
	result := benchmark.RepresentationObservation{RealStack: true, SourceSearchRetained: true}
	byID := map[string]retrieval.ProgressiveContextEvidence{}
	var corpus []string
	for _, alias := range o.seed.Aliases {
		history, err := o.repo.ReadMemoryHistory(ctx, alias.Scope, alias.MemoryID, true)
		if err != nil {
			return result, err
		}
		history.Memory.SourceVersion = history.Versions[0].Version
		corpus = append(corpus, fmt.Sprintf("%s:%d:%s:%s", history.Memory.ID, history.Memory.SourceVersion, history.Memory.State, history.Memory.Content))
		if alias.CaseID != item.ID {
			continue
		}
		evidence := retrieval.ProgressiveContextEvidence{Alias: alias.Alias, Memory: history.Memory}
		for _, p := range history.Provenance {
			if p.MemoryID == alias.MemoryID && p.RawEventID != "" {
				evidence.Citations = append(evidence.Citations, retrieval.Citation{MemoryID: alias.MemoryID, RawEventID: p.RawEventID, Operation: p.Operation})
			}
		}
		result.SourceEvidence = append(result.SourceEvidence, evidence)
		byID[alias.MemoryID] = evidence
	}
	sort.Strings(corpus)
	encoded, _ := json.Marshal(corpus)
	hash := sha256.Sum256(encoded)
	result.SourceWatermark = hex.EncodeToString(hash[:])
	vector := make([]float32, 4)
	for index, c := range o.fixture.Cases {
		if c.ID == item.ID {
			vector[index] = 1
		}
	}
	input := retrieval.SearchInput{Scope: item.Scope, Query: item.Query, QueryEmbedding: vector, TopK: len(item.RequiredSourceAliases)}
	semantic, err := o.repo.SearchSemantic(ctx, input)
	if err != nil {
		return result, err
	}
	if len(semantic) == 0 {
		return result, fmt.Errorf("no pgvector evidence")
	}
	o.semanticCases++
	service := retrieval.NewService(retrieval.ServiceDependencies{Lexical: o.repo, Semantic: o.repo, Relations: o.repo, Citations: o.repo})
	search, err := service.Search(ctx, input)
	if err != nil {
		return result, err
	}
	for _, hit := range search.Hits {
		entry := retrieval.ProgressiveContextEvidence{Alias: byID[hit.Memory.ID].Alias, Memory: hit.Memory, Citations: hit.Citations}
		entry.Memory.SourceVersion = byID[hit.Memory.ID].Memory.SourceVersion
		result.Evidence = append(result.Evidence, entry)
	}
	if strategy == retrieval.ParentFirstFlatStrategy {
		o.baselineCalls++
		result.LatencyMS = int(time.Since(started).Milliseconds())
		return result, nil
	}
	o.candidateCalls++
	result.SearchEvidence = append([]retrieval.ProgressiveContextEvidence(nil), result.Evidence...)
	adapter := retrieval.RepresentationPathAdapterInput{Scope: item.Scope, Family: item.Family, Candidate: item.Candidate, Limits: item.CandidateLimits, EvaluatedAt: o.now, ExpectedEvidence: len(item.RequiredSourceAliases), CriticalFacts: item.CriticalFacts, SourceEvidence: result.SourceEvidence, ExpectedPolicyVersion: "representation-evaluation-v1", ExpectedRendererVersion: "renderer-v1", SourceSearch: retrieval.RepresentationSourceSearchObservation{Scope: item.Scope, Active: true, ReturnedSources: len(search.Hits), ReturnedCitations: len(search.Hits), CandidateCount: len(search.Hits)}}
	adapter.SourceEvidence = nil
	for _, reference := range result.SourceEvidence {
		if reference.Memory.State == memory.MemoryStateActive {
			adapter.SourceEvidence = append(adapter.SourceEvidence, reference)
		}
	}
	if item.Candidate == retrieval.RepresentationCandidateExactExcerptWithSearch {
		adapter.ExactEvidence = result.Evidence
	} else {
		projection, err := o.repo.ReadContextProjection(ctx, item.Scope, o.projections[item.ID].ID)
		if err != nil {
			return result, err
		}
		adapter.TopicProjection = &projection
		adapter.ExpectedWatermarkHash = projection.SourceWatermarkHash()
		result.Evidence = nil
		for _, entry := range projection.Items {
			reference := byID[entry.Source.MemoryID]
			evidence := reference
			evidence.Memory.Content = entry.Text
			evidence.Memory.SourceVersion = entry.Source.Version
			if entry.Citation.MemoryID != reference.Memory.ID || entry.Citation.SourceVersion != reference.Memory.SourceVersion {
				evidence.Citations = nil
			}
			result.Evidence = append(result.Evidence, evidence)
		}
		if item.Candidate == retrieval.RepresentationCandidateHierarchicalWithSearch {
			parent := retrieval.ParentFirstEvaluationInput{Scope: item.Scope, StrategyIdentity: string(item.Candidate), Mode: retrieval.ParentFirstModeShadow, Parents: []retrieval.ParentFirstCandidate{{ID: projection.ID, Scope: item.Scope, LifecycleState: memory.MemoryStateActive, ProjectionStatus: projection.Status, UpdatedAt: projection.UpdatedAt}}, Limits: retrieval.ParentFirstExpansionLimits{MaxParents: 1, MaxChildrenPerParent: 4, MaxCandidates: 4}}
			for _, e := range result.Evidence {
				parent.Children = append(parent.Children, retrieval.ParentFirstChild{ParentID: projection.ID, Evidence: e})
			}
			adapter.ParentFirst = parent
			expanded, err := retrieval.EvaluateParentFirst(parent)
			if err != nil {
				return result, err
			}
			result.Evidence = expanded.Expanded
		}
	}
	observed, err := retrieval.ObserveTaskAppropriateRepresentation(adapter)
	if err != nil {
		return result, err
	}
	result.SafetyFailures = observed.SafetyFailures
	result.BrokenReferences = observed.BrokenReferences
	result.StaleOrNoisyEvidence = observed.StaleOrNoisyEvidence
	result.LatencyMS = int(time.Since(started).Milliseconds())
	return result, nil
}
