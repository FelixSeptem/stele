package postgres

import (
	"context"
	"fmt"
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
	tx, err := s.repository.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin evaluation fixture cleanup: %w", err)
	}
	defer tx.Rollback(ctx)
	for _, record := range seed.Aliases {
		if _, err := tx.Exec(ctx, `DELETE FROM provenance_links WHERE memory_id = $1 OR raw_event_id = $2 OR candidate_memory_id = $3`, record.MemoryID, record.RawEventID, evaluationFixtureID(seed.FixtureVersion+":"+record.CaseID+":"+record.Alias+":candidate")); err != nil {
			return fmt.Errorf("cleanup evaluation provenance: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM relation_projections WHERE memory_id = $1`, record.MemoryID); err != nil {
			return fmt.Errorf("cleanup evaluation relation: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM memory_versions WHERE memory_id = $1`, record.MemoryID); err != nil {
			return fmt.Errorf("cleanup evaluation versions: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM canonical_memories WHERE id = $1`, record.MemoryID); err != nil {
			return fmt.Errorf("cleanup evaluation canonical memory: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM candidate_memories WHERE id = $1`, evaluationFixtureID(seed.FixtureVersion+":"+record.CaseID+":"+record.Alias+":candidate")); err != nil {
			return fmt.Errorf("cleanup evaluation candidate: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM raw_events WHERE id = $1`, record.RawEventID); err != nil {
			return fmt.Errorf("cleanup evaluation raw event: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit evaluation fixture cleanup: %w", err)
	}
	return nil
}

func NewEvaluationFixtureSeeder(repository *Repository) *EvaluationFixtureSeeder {
	return &EvaluationFixtureSeeder{repository: repository}
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
	globalAliases := make(map[string]struct{})
	for caseIndex, item := range fixture.Cases {
		for sourceIndex, source := range item.Sources {
			if _, exists := globalAliases[source.Alias]; exists {
				return retrieval.EvaluationFixtureSeed{}, fmt.Errorf("duplicate fixture alias across cases")
			}
			globalAliases[source.Alias] = struct{}{}

			record, err := s.seedSource(ctx, fixture.Version, item, source, caseIndex, sourceIndex)
			if err != nil {
				return retrieval.EvaluationFixtureSeed{}, err
			}
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
	return normalized.Tenant == evaluationFixtureTenant && normalized.Project == evaluationFixtureProject && strings.TrimSpace(normalized.Namespace) != ""
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
