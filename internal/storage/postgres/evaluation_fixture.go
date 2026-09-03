package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	evaluationFixtureTenant  = "eval"
	evaluationFixtureProject = "retrieval-baseline"
)

// EvaluationFixtureSeeder persists controlled fixture data through the same raw event,
// candidate, canonical-version, provenance, and lifecycle operations used by the
// service. It only accepts the reserved evaluation tenant and project.
type EvaluationFixtureSeeder struct {
	repository *Repository
}

// Cleanup removes only records named by an earlier seed result. It never performs a
// broad tenant/project delete and is therefore safe to defer in an owned harness.
func (s *EvaluationFixtureSeeder) Cleanup(ctx context.Context, seed retrieval.EvaluationFixtureSeed) error {
	if s == nil || s.repository == nil {
		return fmt.Errorf("evaluation fixture repository is not configured")
	}
	rawEventIDs, memoryIDs := evaluationSeedRecordIDs(seed)
	// Large LongMemEval runs may contain hundreds of thousands of records. A
	// single delete with two huge ANY arrays and an OR predicate prevents the
	// planner from using the narrow provenance indexes efficiently. Delete in
	// dependency order and bounded transactions instead. Each query is still
	// limited to the exact IDs returned by the seeder; no scope-wide deletion is
	// used.
	for _, rawBatch := range evaluationIDBatches(rawEventIDs, 2000) {
		if err := s.cleanupBatch(ctx, rawBatch, nil, true); err != nil {
			return err
		}
	}
	for _, memoryBatch := range evaluationIDBatches(memoryIDs, 2000) {
		if err := s.cleanupBatch(ctx, nil, memoryBatch, false); err != nil {
			return err
		}
	}
	for _, rawBatch := range evaluationIDBatches(rawEventIDs, 2000) {
		if err := s.cleanupRawBatch(ctx, rawBatch); err != nil {
			return err
		}
	}
	return nil
}

func (s *EvaluationFixtureSeeder) cleanupBatch(ctx context.Context, rawEventIDs, memoryIDs []string, includeCandidateProvenance bool) error {
	tx, err := s.repository.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin evaluation fixture cleanup: %w", err)
	}
	defer tx.Rollback(ctx)
	if len(rawEventIDs) > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM provenance_links WHERE raw_event_id = ANY($1)`, rawEventIDs); err != nil {
			return fmt.Errorf("cleanup evaluation raw-event provenance: %w", err)
		}
		if includeCandidateProvenance {
			if _, err := tx.Exec(ctx, `DELETE FROM provenance_links p USING candidate_memories c WHERE p.candidate_memory_id = c.id AND c.source_raw_event_id = ANY($1)`, rawEventIDs); err != nil {
				return fmt.Errorf("cleanup evaluation candidate provenance: %w", err)
			}
		}
	}
	if len(memoryIDs) > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM provenance_links WHERE memory_id = ANY($1)`, memoryIDs); err != nil {
			return fmt.Errorf("cleanup evaluation memory provenance: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM relation_projections WHERE memory_id = ANY($1)`, memoryIDs); err != nil {
			return fmt.Errorf("cleanup evaluation relation: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM deletion_markers WHERE memory_id = ANY($1)`, memoryIDs); err != nil {
			return fmt.Errorf("cleanup evaluation deletion marker: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM memory_versions WHERE memory_id = ANY($1)`, memoryIDs); err != nil {
			return fmt.Errorf("cleanup evaluation versions: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM canonical_memories WHERE id = ANY($1)`, memoryIDs); err != nil {
			return fmt.Errorf("cleanup evaluation canonical memory: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit evaluation fixture cleanup: %w", err)
	}
	return nil
}

func (s *EvaluationFixtureSeeder) cleanupRawBatch(ctx context.Context, rawEventIDs []string) error {
	if len(rawEventIDs) == 0 {
		return nil
	}
	tx, err := s.repository.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin evaluation raw cleanup: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM candidate_memories WHERE source_raw_event_id = ANY($1)`, rawEventIDs); err != nil {
		return fmt.Errorf("cleanup evaluation candidate: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM raw_events WHERE id = ANY($1)`, rawEventIDs); err != nil {
		return fmt.Errorf("cleanup evaluation raw event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit evaluation raw cleanup: %w", err)
	}
	return nil
}

func evaluationIDBatches(ids []string, batchSize int) [][]string {
	if batchSize <= 0 {
		batchSize = 1
	}
	result := make([][]string, 0, (len(ids)+batchSize-1)/batchSize)
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		result = append(result, ids[start:end])
	}
	return result
}

func evaluationSeedRecordIDs(seed retrieval.EvaluationFixtureSeed) ([]string, []string) {
	rawEvents := make(map[string]struct{}, len(seed.Aliases))
	memories := make(map[string]struct{}, len(seed.Aliases))
	for _, record := range seed.Aliases {
		if record.RawEventID != "" {
			rawEvents[record.RawEventID] = struct{}{}
		}
		if record.MemoryID != "" {
			memories[record.MemoryID] = struct{}{}
		}
	}
	rawEventIDs := make([]string, 0, len(rawEvents))
	memoryIDs := make([]string, 0, len(memories))
	for id := range rawEvents {
		rawEventIDs = append(rawEventIDs, id)
	}
	for id := range memories {
		memoryIDs = append(memoryIDs, id)
	}
	sort.Strings(rawEventIDs)
	sort.Strings(memoryIDs)
	return rawEventIDs, memoryIDs
}

func NewEvaluationFixtureSeeder(repository *Repository) *EvaluationFixtureSeeder {
	return &EvaluationFixtureSeeder{repository: repository}
}

// SeedBatch imports a fixture through one transaction with deterministic IDs.
// It is intended for large benchmark corpora where opening a transaction per
// raw event/candidate/promotion would dominate runtime. Repeated imports are
// idempotent because every row uses ON CONFLICT DO NOTHING.
func (s *EvaluationFixtureSeeder) SeedBatch(ctx context.Context, fixture retrieval.EvaluationFixture, batchSize int) (retrieval.EvaluationFixtureSeed, error) {
	if err := fixture.Validate(); err != nil {
		return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("validate evaluation fixture: %w", err)
	}
	if s == nil || s.repository == nil {
		return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("evaluation fixture repository is not configured")
	}
	if batchSize <= 0 {
		batchSize = 1000
	}
	type item struct {
		alias       retrieval.EvaluationSeededAlias
		source      retrieval.EvaluationSource
		scope       memory.Scope
		createdAt   time.Time
		key         string
		candidateID string
	}
	items := make([]item, 0)
	seed := retrieval.EvaluationFixtureSeed{FixtureVersion: fixture.Version, Aliases: make([]retrieval.EvaluationSeededAlias, 0)}
	seen := map[string]retrieval.EvaluationSeededAlias{}
	for caseIndex, currentCase := range fixture.Cases {
		if !isOwnedEvaluationFixtureScope(currentCase.Scope) {
			return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("evaluation fixture scope is not owned")
		}
		for sourceIndex, source := range currentCase.Sources {
			if existing, ok := seen[source.Alias]; ok {
				if existing.Scope.Normalized() != currentCase.Scope.Normalized() {
					return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("duplicate fixture alias scope mismatch across cases")
				}
				seed.Aliases = append(seed.Aliases, retrieval.EvaluationSeededAlias{CaseID: currentCase.ID, Alias: source.Alias, Scope: existing.Scope, MemoryID: existing.MemoryID, RawEventID: existing.RawEventID, State: existing.State, FactCluster: existing.FactCluster})
				continue
			}
			class := source.Class
			if class == "" {
				class = memory.MemoryClassEpisodic
			}
			state := source.State
			if state == "" {
				state = memory.MemoryStateActive
			}
			createdAt := evaluationFixtureTimestamp(source.SourceTimestamp, caseIndex, sourceIndex)
			key := fixture.Version + ":" + currentCase.ID + ":" + source.Alias
			alias := retrieval.EvaluationSeededAlias{CaseID: currentCase.ID, Alias: source.Alias, Scope: currentCase.Scope, MemoryID: evaluationFixtureID(key + ":memory"), RawEventID: evaluationFixtureID(key + ":raw-event"), State: state, FactCluster: source.FactCluster}
			seen[source.Alias] = alias
			seed.Aliases = append(seed.Aliases, alias)
			items = append(items, item{alias: alias, source: source, scope: currentCase.Scope, createdAt: createdAt, key: key, candidateID: evaluationFixtureID(key + ":candidate")})
		}
	}
	tx, err := s.repository.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("begin evaluation fixture batch transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	for start := 0; start < len(items); start += batchSize {
		end := start + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := &pgx.Batch{}
		for _, current := range items[start:end] {
			metadata, marshalErr := json.Marshal(map[string]any{"fixture_owner": fixture.Version, "fixture_alias": current.source.Alias})
			if marshalErr != nil {
				return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("marshal evaluation metadata: %w", marshalErr)
			}
			batch.Queue(`INSERT INTO raw_events (id,tenant,project,namespace,event_type,content,metadata,source_timestamp,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (id) DO NOTHING`, current.alias.RawEventID, current.scope.Tenant, current.scope.Project, current.scope.Namespace, current.source.EventType, current.source.Content, metadata, nullableTime(current.source.SourceTimestamp), current.createdAt)
		}
		for _, current := range items[start:end] {
			class := current.source.Class
			if class == "" {
				class = memory.MemoryClassEpisodic
			}
			batch.Queue(`INSERT INTO candidate_memories (id,source_raw_event_id,tenant,project,namespace,class,content,confidence,importance,freshness,sensitivity,mutability,retention_class,status,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,1,1,1,$8,$9,$10,'promoted',$11,$11) ON CONFLICT (id) DO NOTHING`, current.candidateID, current.alias.RawEventID, current.scope.Tenant, current.scope.Project, current.scope.Namespace, class, current.source.Content, governance.SensitivityLow, governance.MutabilityImmutable, policy.RetentionClassPermanent, current.createdAt)
		}
		for _, current := range items[start:end] {
			class := current.source.Class
			if class == "" {
				class = memory.MemoryClassEpisodic
			}
			batch.Queue(`INSERT INTO canonical_memories (id,tenant,project,namespace,class,state,retention_class,content,search_text,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,'active',$6,$7,to_tsvector('simple',$7),$8,$8) ON CONFLICT (id) DO NOTHING`, current.alias.MemoryID, current.scope.Tenant, current.scope.Project, current.scope.Namespace, class, policy.RetentionClassPermanent, current.source.Content, current.createdAt)
		}
		for _, current := range items[start:end] {
			batch.Queue(`INSERT INTO memory_versions (id,memory_id,version,state,content,created_at,modified_by) VALUES ($1,$2,1,'active',$3,$4,$5) ON CONFLICT (id) DO NOTHING`, evaluationFixtureID(current.key+":version"), current.alias.MemoryID, current.source.Content, current.createdAt, current.candidateID)
		}
		for _, current := range items[start:end] {
			batch.Queue(`INSERT INTO provenance_links (id,raw_event_id,tenant,project,namespace,operation,actor,source_context,created_at) VALUES ($1,$2,$3,$4,$5,'evaluation_fixture_ingest','retrieval-evaluation-fixture','{}'::jsonb,$6) ON CONFLICT (id) DO NOTHING`, evaluationFixtureID(current.key+":event-provenance"), current.alias.RawEventID, current.scope.Tenant, current.scope.Project, current.scope.Namespace, current.createdAt)
			batch.Queue(`INSERT INTO provenance_links (id,raw_event_id,candidate_memory_id,tenant,project,namespace,operation,actor,source_context,created_at) VALUES ($1,$2,$3,$4,$5,$6,'evaluation_fixture_candidate','retrieval-evaluation-fixture','{}'::jsonb,$7) ON CONFLICT (id) DO NOTHING`, evaluationFixtureID(current.key+":candidate-provenance"), current.alias.RawEventID, current.candidateID, current.scope.Tenant, current.scope.Project, current.scope.Namespace, current.createdAt)
			batch.Queue(`INSERT INTO provenance_links (id,raw_event_id,candidate_memory_id,memory_id,tenant,project,namespace,operation,actor,source_context,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,'evaluation_fixture_promote','retrieval-evaluation-fixture','{}'::jsonb,$8) ON CONFLICT (id) DO NOTHING`, evaluationFixtureID(current.key+":promotion-provenance"), current.alias.RawEventID, current.candidateID, current.alias.MemoryID, current.scope.Tenant, current.scope.Project, current.scope.Namespace, current.createdAt)
			batch.Queue(`INSERT INTO provenance_links (id,raw_event_id,candidate_memory_id,memory_id,tenant,project,namespace,operation,source_context,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,'promote_candidate','{}'::jsonb,$8) ON CONFLICT (id) DO NOTHING`, promotionProvenanceID(evaluationFixtureID(current.key+":version")), current.alias.RawEventID, current.candidateID, current.alias.MemoryID, current.scope.Tenant, current.scope.Project, current.scope.Namespace, current.createdAt)
		}
		results := tx.SendBatch(ctx, batch)
		for commandIndex := 0; commandIndex < batch.Len(); commandIndex++ {
			if _, execErr := results.Exec(); execErr != nil {
				results.Close()
				return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("execute evaluation fixture batch: %w", execErr)
			}
		}
		if closeErr := results.Close(); closeErr != nil {
			return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("close evaluation fixture batch: %w", closeErr)
		}
		for _, current := range items[start:end] {
			if current.alias.State == memory.MemoryStateActive {
				continue
			}
			content := current.source.Content
			search := "search_text"
			embedding := "embedding"
			if current.alias.State == memory.MemoryStateDeleted {
				content = ""
				search = "NULL"
				embedding = "NULL"
			}
			if _, execErr := tx.Exec(ctx, `UPDATE canonical_memories SET state=$2,content=$3,search_text=`+search+`,embedding=`+embedding+`,updated_at=$4 WHERE id=$1 AND tenant=$5 AND project=$6 AND namespace=$7`, current.alias.MemoryID, current.alias.State, content, current.createdAt.Add(time.Second), current.scope.Tenant, current.scope.Project, current.scope.Namespace); execErr != nil {
				return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("apply evaluation lifecycle state: %w", execErr)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("commit evaluation fixture batch transaction: %w", err)
	}
	return seed, nil
}

func (s *EvaluationFixtureSeeder) Seed(ctx context.Context, fixture retrieval.EvaluationFixture) (retrieval.EvaluationFixtureSeed, error) {
	if err := fixture.Validate(); err != nil {
		return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("validate evaluation fixture: %w", err)
	}
	for _, item := range fixture.Cases {
		if !isOwnedEvaluationFixtureScope(item.Scope) {
			return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("evaluation fixture scope is not owned")
		}
	}
	if s == nil || s.repository == nil {
		return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("evaluation fixture repository is not configured")
	}

	seed := retrieval.EvaluationFixtureSeed{
		FixtureVersion: fixture.Version,
		Aliases:        make([]retrieval.EvaluationSeededAlias, 0),
	}
	globalAliases := make(map[string]retrieval.EvaluationSeededAlias)
	for caseIndex, item := range fixture.Cases {
		for sourceIndex, source := range item.Sources {
			if existing, exists := globalAliases[source.Alias]; exists {
				if existing.Scope.Normalized() != item.Scope.Normalized() {
					return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("duplicate fixture alias scope mismatch across cases")
				}
				seed.Aliases = append(seed.Aliases, retrieval.EvaluationSeededAlias{
					CaseID:      item.ID,
					Alias:       source.Alias,
					Scope:       existing.Scope,
					MemoryID:    existing.MemoryID,
					RawEventID:  existing.RawEventID,
					State:       existing.State,
					FactCluster: existing.FactCluster,
				})
				continue
			}

			record, err := s.seedSource(ctx, fixture.Version, item, source, caseIndex, sourceIndex)
			if err != nil {
				return retrieval.EvaluationFixtureSeed{}, err
			}
			globalAliases[source.Alias] = record
			seed.Aliases = append(seed.Aliases, record)
		}
	}
	return seed, nil
}

func (s *EvaluationFixtureSeeder) seedSource(ctx context.Context, fixtureVersion string, item retrieval.EvaluationCase, source retrieval.EvaluationSource, caseIndex, sourceIndex int) (retrieval.EvaluationSeededAlias, error) {
	createdAt := evaluationFixtureTimestamp(source.SourceTimestamp, caseIndex, sourceIndex)
	key := fixtureVersion + ":" + item.ID + ":" + source.Alias
	event, err := s.repository.IngestEvent(ctx, memory.IngestEventInput{
		Scope:           item.Scope,
		EventType:       source.EventType,
		Content:         source.Content,
		Metadata:        map[string]any{"fixture_owner": fixtureVersion, "fixture_alias": source.Alias},
		SourceTimestamp: source.SourceTimestamp,
	}, memory.ProvenanceRecord{
		ID:        evaluationFixtureID(key + ":event-provenance"),
		Scope:     item.Scope,
		Operation: "evaluation_fixture_ingest",
		Actor:     "retrieval-evaluation-fixture",
		CreatedAt: createdAt,
	})
	if err != nil {
		return retrieval.EvaluationSeededAlias{}, fmt.Errorf("seed evaluation raw event: %w", err)
	}

	class := source.Class
	if class == "" {
		class = memory.MemoryClassEpisodic
	}
	candidate := governance.CandidateMemory{
		ID:               evaluationFixtureID(key + ":candidate"),
		SourceRawEventID: event.ID,
		Scope:            item.Scope,
		Class:            class,
		Content:          source.Content,
		Confidence:       1,
		Importance:       1,
		Freshness:        1,
		Sensitivity:      governance.SensitivityLow,
		Mutability:       governance.MutabilityImmutable,
		RetentionClass:   policy.RetentionClassPermanent,
		Status:           governance.CandidateStatusPending,
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
	}
	if _, err := s.repository.CreateCandidate(ctx, candidate, memory.ProvenanceRecord{
		ID:                evaluationFixtureID(key + ":candidate-provenance"),
		Scope:             item.Scope,
		RawEventID:        event.ID,
		CandidateMemoryID: candidate.ID,
		Operation:         "evaluation_fixture_candidate",
		Actor:             "retrieval-evaluation-fixture",
		CreatedAt:         createdAt,
	}); err != nil {
		return retrieval.EvaluationSeededAlias{}, fmt.Errorf("seed evaluation candidate: %w", err)
	}

	canonical, _, err := s.repository.PromoteCandidate(ctx, governance.CanonicalPromotion{
		Candidate: candidate,
		MemoryID:  evaluationFixtureID(key + ":memory"),
		VersionID: evaluationFixtureID(key + ":version"),
		Version:   1,
		CreatedAt: createdAt,
	})
	if err != nil {
		return retrieval.EvaluationSeededAlias{}, fmt.Errorf("promote evaluation candidate: %w", err)
	}
	if _, err := s.repository.TransitionCandidateStatus(ctx, governance.CandidateStatusTransition{
		CandidateID: candidate.ID,
		ToStatus:    governance.CandidateStatusPromoted,
		UpdatedAt:   createdAt,
	}, memory.ProvenanceRecord{
		ID:                evaluationFixtureID(key + ":promotion-provenance"),
		Scope:             item.Scope,
		RawEventID:        event.ID,
		CandidateMemoryID: candidate.ID,
		MemoryID:          canonical.ID,
		Operation:         "evaluation_fixture_promote",
		Actor:             "retrieval-evaluation-fixture",
		CreatedAt:         createdAt,
	}); err != nil {
		return retrieval.EvaluationSeededAlias{}, fmt.Errorf("mark evaluation candidate promoted: %w", err)
	}

	state := source.State
	if state == "" {
		state = memory.MemoryStateActive
	}
	if err := s.applySourceState(ctx, canonical, state, createdAt, key); err != nil {
		return retrieval.EvaluationSeededAlias{}, err
	}
	return retrieval.EvaluationSeededAlias{
		CaseID:      item.ID,
		Alias:       source.Alias,
		Scope:       item.Scope,
		MemoryID:    canonical.ID,
		RawEventID:  event.ID,
		State:       state,
		FactCluster: source.FactCluster,
	}, nil
}

func (s *EvaluationFixtureSeeder) applySourceState(ctx context.Context, canonical memory.CanonicalMemory, state memory.MemoryState, appliedAt time.Time, key string) error {
	var action policy.ForgettingAction
	switch state {
	case memory.MemoryStateActive:
		return nil
	case memory.MemoryStateSuppressed:
		action = policy.ForgettingActionSuppress
	case memory.MemoryStateForgotten:
		action = policy.ForgettingActionExpire
	case memory.MemoryStateDeleted:
		action = policy.ForgettingActionDelete
	default:
		return fmt.Errorf("unsupported evaluation source state")
	}
	if _, err := s.repository.ApplyLifecycleAction(ctx, governance.LifecycleAction{
		MemoryID:  canonical.ID,
		Scope:     canonical.Scope,
		Action:    action,
		Reason:    "evaluation fixture lifecycle coverage",
		Actor:     "retrieval-evaluation-fixture",
		RequestID: evaluationFixtureID(key + ":lifecycle-request"),
		AppliedAt: appliedAt.Add(time.Second),
	}); err != nil {
		return fmt.Errorf("apply evaluation source lifecycle: %w", err)
	}
	return nil
}

func isOwnedEvaluationFixtureScope(scope memory.Scope) bool {
	normalized := scope.Normalized()
	if strings.TrimSpace(normalized.Tenant) == "" || strings.TrimSpace(normalized.Namespace) == "" {
		return false
	}
	if normalized.Tenant == evaluationFixtureTenant && normalized.Project == evaluationFixtureProject {
		return true
	}
	return strings.HasPrefix(normalized.Project, "benchmark-")
}

func evaluationFixtureTimestamp(sourceTimestamp time.Time, caseIndex, sourceIndex int) time.Time {
	if !sourceTimestamp.IsZero() {
		return sourceTimestamp.UTC()
	}
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(caseIndex*100+sourceIndex) * time.Minute)
}

func evaluationFixtureID(value string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("stele:retrieval-evaluation:"+value)).String()
}
