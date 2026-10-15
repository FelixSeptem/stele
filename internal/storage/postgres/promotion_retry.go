package postgres

import (
	"context"
	"fmt"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

// lockCandidatePromotion serializes retries using the persisted candidate,
// keeping candidate, canonical, version, and provenance writes in one commit.
func lockCandidatePromotion(ctx context.Context, tx pgx.Tx, input governance.CanonicalPromotion) (string, bool, error) {
	const query = `SELECT c.status,
	COALESCE((SELECT p.memory_id::text FROM provenance_links p
	 WHERE p.candidate_memory_id = c.id AND p.raw_event_id = c.source_raw_event_id
	 AND p.tenant = c.tenant AND p.project = c.project AND p.namespace = c.namespace
	 AND p.operation = 'promote_candidate' AND p.memory_id IS NOT NULL
	 ORDER BY p.created_at DESC, p.id DESC LIMIT 1), '')
FROM candidate_memories c
WHERE c.id = $1 AND c.tenant = $2 AND c.project = $3 AND c.namespace = $4
	AND c.source_raw_event_id = $5
FOR UPDATE OF c`
	var status governance.CandidateStatus
	var existing string
	if err := tx.QueryRow(ctx, query, input.Candidate.ID, input.Candidate.Scope.Tenant, input.Candidate.Scope.Project, input.Candidate.Scope.Namespace, input.Candidate.SourceRawEventID).Scan(&status, &existing); err != nil {
		return "", false, fmt.Errorf("lock candidate promotion: %w", err)
	}
	if existing != "" {
		return existing, status == governance.CandidateStatusPending, nil
	}
	if status != governance.CandidateStatusPending {
		return "", false, fmt.Errorf("candidate is not pending promotion")
	}
	return "", false, nil
}

func markCandidatePromoted(ctx context.Context, tx pgx.Tx, input governance.CanonicalPromotion) error {
	const query = `UPDATE candidate_memories SET status='promoted', updated_at=$2
WHERE id=$1 AND tenant=$3 AND project=$4 AND namespace=$5`
	tag, err := tx.Exec(ctx, query, input.Candidate.ID, input.CreatedAt, input.Candidate.Scope.Tenant, input.Candidate.Scope.Project, input.Candidate.Scope.Namespace)
	if err != nil {
		return fmt.Errorf("mark promoted candidate: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("promoted candidate disappeared")
	}
	return nil
}

func readCommittedCandidatePromotion(ctx context.Context, tx pgx.Tx, input governance.CanonicalPromotion, memoryID string) (memory.CanonicalMemory, memory.MemoryVersion, error) {
	const query = `SELECT
	c.id, c.tenant, c.project, c.namespace, c.memory_path, c.class, c.state, c.content, c.created_at, c.updated_at,
	c.temporal_fact_id, c.ingested_at, c.valid_from, c.valid_to, c.validity_source, c.source_metadata,
	v.id, v.memory_id, v.memory_path, v.version, v.state, v.content, v.created_at, v.modified_by,
	v.temporal_fact_id, v.ingested_at, v.valid_from, v.valid_to, v.validity_source, v.source_metadata
FROM canonical_memories c JOIN memory_versions v ON v.memory_id = c.id
WHERE c.id=$1 AND c.tenant=$2 AND c.project=$3 AND c.namespace=$4
	AND v.modified_by=$5
ORDER BY v.version DESC, v.id DESC LIMIT 1`
	var canonical memory.CanonicalMemory
	var version memory.MemoryVersion
	err := tx.QueryRow(ctx, query, memoryID, input.Candidate.Scope.Tenant, input.Candidate.Scope.Project, input.Candidate.Scope.Namespace, input.Candidate.ID).Scan(
		&canonical.ID, &canonical.Scope.Tenant, &canonical.Scope.Project, &canonical.Scope.Namespace, &canonical.MemoryPath,
		&canonical.Class, &canonical.State, &canonical.Content, &canonical.CreatedAt, &canonical.ModifiedAt,
		&canonical.TemporalFactID, &canonical.IngestedAt, &canonical.ValidFrom, &canonical.ValidTo, &canonical.ValiditySource, &canonical.SourceMetadata,
		&version.ID, &version.MemoryID, &version.MemoryPath, &version.Version, &version.State, &version.Content, &version.CreatedAt, &version.ModifiedBy,
		&version.TemporalFactID, &version.IngestedAt, &version.ValidFrom, &version.ValidTo, &version.ValiditySource, &version.SourceMetadata,
	)
	if err != nil {
		return memory.CanonicalMemory{}, memory.MemoryVersion{}, fmt.Errorf("read committed candidate promotion: %w", err)
	}
	canonical.SourceVersion = version.Version
	return canonical, version, nil
}
