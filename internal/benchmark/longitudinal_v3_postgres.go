package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/internal/storage/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func taskSeriesPackHits(hits []retrieval.SearchHit, l TaskSeriesLimits) []retrieval.SearchHit {
	out := make([]retrieval.SearchHit, 0, len(hits))
	bytes, tokens, citations := 0, 0, 0
	for _, hit := range hits {
		b, n, c := len(hit.Memory.Content), len(strings.Fields(hit.Memory.Content)), len(hit.Citations)
		if len(out)+1 > l.MaxItems || bytes+b > l.MaxContentBytes || tokens+n > l.MaxTokens || citations+c > l.MaxCitations {
			continue
		}
		out = append(out, hit)
		bytes += b
		tokens += n
		citations += c
	}
	return out
}

func runTaskSeriesV3Postgres(ctx context.Context, dsn string, m TaskSeriesManifest, corpora map[string]NormalizedCorpus, base memory.Scope, dataset DatasetManifest, synthetic bool) (TaskSeriesReport, error) {
	if err := base.Validate(); err != nil {
		return TaskSeriesReport{}, fmt.Errorf("v3 benchmark scope is invalid")
	}
	if err := postgres.NewMigrationRunner().Apply(ctx, dsn); err != nil {
		return TaskSeriesReport{}, err
	}
	pool, err := postgres.OpenPool(ctx, dsn)
	if err != nil {
		return TaskSeriesReport{}, err
	}
	defer pool.Close()
	if err := postgres.BootstrapDatabase(ctx, pool); err != nil {
		return TaskSeriesReport{}, err
	}
	runtime, err := postgresRuntimeIdentity(ctx, pool)
	if err != nil {
		return TaskSeriesReport{}, err
	}
	obs := TaskSeriesObservations{}
	baseline := map[string]string{}
	corpusDigests := []string{}
	runID := uuid.NewString()
	for ci, c := range m.Conditions {
		obs[c.ID] = map[string]TaskSeriesStageObservation{}
		digest, err := corpora[c.ID].Checksum()
		if err != nil {
			return TaskSeriesReport{}, err
		}
		corpusDigests = append(corpusDigests, digest)
		for si, s := range c.Stages {
			if err := ctx.Err(); err != nil {
				return TaskSeriesReport{}, err
			}
			snapshot, err := taskSeriesStageSnapshot(m, c, corpora[c.ID], si)
			if err != nil {
				return TaskSeriesReport{}, err
			}
			run, err := NewRunScope(base, "task-series-v3", fmt.Sprintf("%s-%d-%d", runID, ci, si))
			if err != nil {
				return TaskSeriesReport{}, err
			}
			observation, identity, err := runTaskSeriesV3Stage(ctx, pool, m, c, snapshot, si, run.Scope)
			if err != nil {
				return TaskSeriesReport{}, err
			}
			obs[c.ID][s.ID] = observation
			if ci == 0 {
				baseline[s.ID] = identity
			}
		}
	}
	// Repeat the declared baseline after all candidate conditions. Scope/run IDs
	// cannot enter evidence identity, so equal results must compare exactly.
	c := m.Conditions[0]
	rollback := map[string]bool{}
	for si, s := range c.Stages {
		snapshot, err := taskSeriesStageSnapshot(m, c, corpora[c.ID], si)
		if err != nil {
			return TaskSeriesReport{}, err
		}
		run, err := NewRunScope(base, "task-series-v3", fmt.Sprintf("%s-rollback-%d", runID, si))
		if err != nil {
			return TaskSeriesReport{}, err
		}
		_, identity, err := runTaskSeriesV3Stage(ctx, pool, m, c, snapshot, si, run.Scope)
		if err != nil {
			return TaskSeriesReport{}, err
		}
		rollback[s.ID] = identity == baseline[s.ID]
	}
	for _, c := range m.Conditions {
		for _, s := range c.Stages {
			o := obs[c.ID][s.ID]
			value := rollback[s.ID]
			o.RollbackMatched = &value
			obs[c.ID][s.ID] = o
		}
	}
	report, err := EvaluateTaskSeries(m, obs)
	if err != nil {
		return TaskSeriesReport{}, err
	}
	report.Runtime = &runtime
	report.Strategy = StrategyProfileLexical
	report.SyntheticFixture = synthetic
	report.CleanupVerified = true
	report.NormalizedChecksum = taskSeriesValueDigest(corpusDigests)
	if dataset.Name != "" {
		report.Dataset = dataset.Name
		report.DatasetVersion = dataset.Version
		report.ManifestChecksum = taskSeriesValueDigest(dataset)
	}
	return report, nil
}

func runTaskSeriesV3Stage(ctx context.Context, pool *pgxpool.Pool, m TaskSeriesManifest, c TaskSeriesCondition, snapshot NormalizedCorpus, stageIndex int, scope memory.Scope) (observation TaskSeriesStageObservation, identity string, stageErr error) {
	repo := postgres.NewRepository(pool)
	seeder := postgres.NewEvaluationFixtureSeeder(repo)
	seed := retrieval.EvaluationFixtureSeed{}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := seeder.Cleanup(cleanupCtx, seed); err != nil {
			stageErr = fmt.Errorf("v3 owned stage cleanup failed: %w", err)
			return
		}
		for _, table := range []string{"raw_events", "canonical_memories", "candidate_memories", "provenance_links"} {
			var count int
			if err := pool.QueryRow(cleanupCtx, "SELECT count(*) FROM "+table+" WHERE tenant=$1 AND project=$2 AND namespace=$3", scope.Tenant, scope.Project, scope.Namespace).Scan(&count); err != nil || count != 0 {
				stageErr = fmt.Errorf("v3 owned cleanup could not be verified")
				return
			}
		}
	}()
	query := snapshot.Queries[0]
	windows := map[string]TaskSeriesSourceWindow{}
	events := map[string]MemoryEventRecord{}
	for _, w := range c.Sources {
		windows[w.Reference] = w
	}
	for _, e := range snapshot.Events {
		events[e.ID] = e
	}
	if len(snapshot.Events) > 0 {
		item := retrieval.EvaluationCase{ID: query.ID, Category: "longitudinal-v3", Scope: scope, Query: query.Text}
		for _, e := range snapshot.Events {
			w := windows[e.ID]
			metadata := w.Metadata
			// The seeder binds the declared immutable snapshot and version at
			// INSERT. PostgreSQL derives candidate/version snapshots from these
			// rows and rejects any later source_metadata rewrite.
			item.Sources = append(item.Sources, retrieval.EvaluationSource{Alias: e.ID, EventType: "benchmark.v3_snapshot", Class: e.Class, State: e.ExpectedState, Content: e.Text, SourceMetadata: &metadata, SourceVersion: w.SourceVersion})
		}
		// Seeder-only structural expectation, never used as a task verdict or qrel.
		item.ExpectedEvidenceGroups = [][]string{{snapshot.Events[0].ID}}
		var err error
		seed, err = seeder.SeedBatch(ctx, retrieval.EvaluationFixture{Version: "task-series-v3-owned", Cases: []retrieval.EvaluationCase{item}}, 64)
		if err != nil {
			return observation, "", err
		}
	}
	byID := map[string]retrieval.EvaluationSeededAlias{}
	for _, a := range seed.Aliases {
		byID[a.MemoryID] = a
	}
	// Physical stage isolation is checked from actual rows, not just the input
	// count: every row must map to exactly one currently visible snapshot.
	rows, err := pool.Query(ctx, `SELECT m.id::text,m.content,v.version,m.source_metadata,v.source_metadata,e.source_metadata FROM canonical_memories m JOIN memory_versions v ON v.memory_id=m.id JOIN provenance_links p ON p.memory_id=m.id AND p.operation='promote_candidate' AND p.tenant=m.tenant AND p.project=m.project AND p.namespace=m.namespace JOIN raw_events e ON e.id=p.raw_event_id AND e.tenant=m.tenant AND e.project=m.project AND e.namespace=m.namespace WHERE m.tenant=$1 AND m.project=$2 AND m.namespace=$3`, scope.Tenant, scope.Project, scope.Namespace)
	if err != nil {
		return observation, "", err
	}
	visible := true
	seen := map[string]bool{}
	for rows.Next() {
		var id, text string
		var version int64
		var canonical, immutable, raw []byte
		if err := rows.Scan(&id, &text, &version, &canonical, &immutable, &raw); err != nil {
			rows.Close()
			return observation, "", err
		}
		a, ok := byID[id]
		w := windows[a.Alias]
		var cm, vm, rm memory.SourceMetadata
		if !ok || seen[id] || !w.visible(stageIndex+1) || version != w.SourceVersion || checksumBytes([]byte(text)) != w.ContentDigest || json.Unmarshal(canonical, &cm) != nil || json.Unmarshal(immutable, &vm) != nil || json.Unmarshal(raw, &rm) != nil || cm != w.Metadata || vm != w.Metadata || rm != w.Metadata {
			visible = false
		}
		seen[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return observation, "", err
	}
	visible = visible && len(seen) == len(snapshot.Events)
	if !visible {
		return observation, "", fmt.Errorf("v3 physical stage snapshot differs from lock")
	}
	searcher := retrieval.NewService(retrieval.ServiceDependencies{Lexical: repo, Semantic: repo, Relations: repo, Citations: repo})
	start := time.Now()
	result, err := searcher.Search(ctx, retrieval.SearchInput{Scope: scope, Query: query.Text, TopK: 64, DisclosureRequired: true})
	elapsed := time.Since(start).Microseconds()
	observation.LatencyMicros = &elapsed
	if err != nil {
		return observation, "", err
	}
	disclosure := true
	// Check every returned hit before packing; budgets must not mask a leak.
	for _, hit := range result.Hits {
		a, ok := byID[hit.Memory.ID]
		if !ok || hit.Memory.Scope != scope || hit.Memory.State != memory.MemoryStateActive {
			observation.SafetyFailures++
			continue
		}
		if !hit.Memory.SourceMetadata.Disclosable() || hit.Memory.SourceMetadata != windows[a.Alias].Metadata {
			disclosure = false
		}
	}
	identities := []string{}
	for _, hit := range taskSeriesPackHits(result.Hits, m.Experiment.Limits) {
		a, ok := byID[hit.Memory.ID]
		if !ok {
			continue
		}
		w := windows[a.Alias]
		provenance := false
		for _, citation := range hit.Citations {
			if citation.MemoryID == a.MemoryID && citation.RawEventID == a.RawEventID {
				provenance = true
			}
		}
		observation.Evidence = append(observation.Evidence, TaskSeriesObservedEvidence{Reference: a.Alias, ScopeEligible: hit.Memory.Scope == scope, LifecycleVisible: hit.Memory.State == memory.MemoryStateActive, ProvenanceVerified: provenance})
		identities = append(identities, taskSeriesValueDigest(struct {
			Alias   string
			Version int64
			Content string
		}{a.Alias, w.SourceVersion, checksumBytes([]byte(hit.Memory.Content))}))
	}
	sort.Strings(identities)
	observation.DisclosureSafe = &disclosure
	observation.StageVisible = &visible
	return observation, taskSeriesValueDigest(identities), nil
}
