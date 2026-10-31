package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var _ assurance.IngestionFreshnessCollector = (*Repository)(nil)
var _ assurance.IngestionFreshnessEvidenceStore = (*Repository)(nil)
var _ interface {
	RecordProviderMemoryRead(context.Context, memory.Scope, []string, time.Time) error
} = (*Repository)(nil)

var defaultIngestionFreshnessPolicy = assurance.IngestionFreshnessPolicy{
	ContractVersion:  assurance.IngestionFreshnessVersion,
	AdmissionWindow:  15 * time.Minute,
	GovernanceWindow: 5 * time.Minute,
	RetrievalWindow:  5 * time.Minute,
}

// CollectIngestionFreshness resolves a raw-event fixture only inside the
// supplied exact scope. The public-read stage comes exclusively from a prior
// successful Provider retrieval observation, never from canonical state.
func (r *Repository) CollectIngestionFreshness(ctx context.Context, scope memory.Scope, fixtureReference string, observedAt time.Time) (assurance.IngestionFreshnessInput, error) {
	input := assurance.IngestionFreshnessInput{
		Scope:      scope.Normalized(),
		ReportKey:  fixtureReference,
		ObservedAt: observedAt.UTC(),
		Policy:     defaultIngestionFreshnessPolicy,
	}
	if err := scope.Validate(); err != nil {
		return assurance.IngestionFreshnessInput{}, err
	}
	if _, err := uuid.Parse(fixtureReference); err != nil {
		return assurance.IngestionFreshnessInput{}, fmt.Errorf("invalid ingestion freshness fixture reference")
	}
	if observedAt.IsZero() {
		return assurance.IngestionFreshnessInput{}, fmt.Errorf("ingestion freshness observation time is required")
	}
	const query = `
WITH fixture AS (
 SELECT id, created_at, governance_processed_at, governance_last_failed_at, governance_exhausted_at,
        source_metadata
 FROM raw_events
 WHERE tenant=$1 AND project=$2 AND namespace=$3 AND id=$4::uuid
), canonical AS (
 SELECT MAX(p.created_at) FILTER (WHERE m.state='active' AND latest.state='active') AS canonical_active_at,
        COALESCE(bool_or(m.id IS NOT NULL AND (m.state <> 'active' OR latest.id IS NULL OR latest.state <> 'active')), false) AS canonical_hidden,
        COALESCE(bool_or(
          m.state='active' AND latest.state='active'
          AND m.source_metadata->>'version'='source-metadata-v1'
          AND m.source_metadata->>'trust' IN ('untrusted','attested_data')
          AND m.source_metadata->>'classification' IN ('low','medium')
          AND latest.source_metadata->>'version'='source-metadata-v1'
          AND latest.source_metadata->>'trust' IN ('untrusted','attested_data')
          AND latest.source_metadata->>'classification' IN ('low','medium')), false) AS canonical_visible
 FROM fixture e
 JOIN candidate_memories c ON c.source_raw_event_id=e.id
   AND c.tenant=$1 AND c.project=$2 AND c.namespace=$3 AND c.status='promoted'
 JOIN provenance_links p ON p.raw_event_id=e.id AND p.candidate_memory_id=c.id
   AND p.tenant=$1 AND p.project=$2 AND p.namespace=$3 AND p.operation='promote_candidate'
 JOIN canonical_memories m ON m.id=p.memory_id AND m.tenant=$1 AND m.project=$2 AND m.namespace=$3
 LEFT JOIN LATERAL (
   SELECT id, state, source_metadata
   FROM memory_versions
   WHERE memory_id=m.id
   ORDER BY version DESC
   LIMIT 1
 ) latest ON true
), public_read AS (
 SELECT MAX(o.observed_at) AS public_read_at
 FROM fixture e
 JOIN provenance_links p ON p.raw_event_id=e.id AND p.tenant=$1 AND p.project=$2 AND p.namespace=$3
   AND p.operation='promote_candidate'
 JOIN provider_memory_read_observations o ON o.memory_id=p.memory_id
   AND o.tenant=$1 AND o.project=$2 AND o.namespace=$3
 CROSS JOIN canonical c
 WHERE c.canonical_active_at IS NOT NULL AND o.observed_at >= c.canonical_active_at
)
SELECT e.created_at, e.governance_processed_at, e.governance_last_failed_at, e.governance_exhausted_at,
       c.canonical_active_at, c.canonical_hidden,
       COALESCE(e.source_metadata->>'version'='source-metadata-v1'
         AND e.source_metadata->>'trust' IN ('untrusted','attested_data')
         AND e.source_metadata->>'classification' IN ('low','medium')
         AND c.canonical_visible, false),
       pr.public_read_at
FROM fixture e CROSS JOIN canonical c CROSS JOIN public_read pr`
	var admittedAt time.Time
	var processedAt, failedAt, exhaustedAt, canonicalActiveAt, publicReadAt *time.Time
	var canonicalHidden, sourceVisible bool
	err := r.db.QueryRow(ctx, query, scope.Tenant, scope.Project, scope.Namespace, fixtureReference).Scan(
		&admittedAt, &processedAt, &failedAt, &exhaustedAt, &canonicalActiveAt, &canonicalHidden, &sourceVisible, &publicReadAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return unavailableIngestionFreshnessInput(input), nil
	}
	if err != nil {
		return assurance.IngestionFreshnessInput{}, fmt.Errorf("collect ingestion freshness: %w", err)
	}
	input.Stages = append(input.Stages, assurance.IngestionFreshnessStage{Kind: assurance.IngestionStageAdmitted, Disposition: assurance.IngestionDispositionComplete, At: admittedAt.UTC()})
	if processedAt == nil {
		disposition, at := assurance.IngestionDispositionPending, time.Time{}
		if exhaustedAt != nil {
			disposition, at = assurance.IngestionDispositionExhausted, exhaustedAt.UTC()
		} else if failedAt != nil {
			disposition, at = assurance.IngestionDispositionRetrying, failedAt.UTC()
		}
		input.Stages = append(input.Stages,
			assurance.IngestionFreshnessStage{Kind: assurance.IngestionStageGovernanceCompleted, Disposition: disposition, At: at},
			assurance.IngestionFreshnessStage{Kind: assurance.IngestionStageCanonicalActive, Disposition: assurance.IngestionDispositionUnavailable},
			assurance.IngestionFreshnessStage{Kind: assurance.IngestionStagePublicRetrievable, Disposition: assurance.IngestionDispositionUnavailable},
		)
		return input, nil
	}
	input.Stages = append(input.Stages, assurance.IngestionFreshnessStage{Kind: assurance.IngestionStageGovernanceCompleted, Disposition: assurance.IngestionDispositionComplete, At: processedAt.UTC()})
	if canonicalActiveAt == nil {
		disposition := assurance.IngestionDispositionUnavailable
		if canonicalHidden {
			disposition = assurance.IngestionDispositionHidden
		}
		input.Stages = append(input.Stages,
			assurance.IngestionFreshnessStage{Kind: assurance.IngestionStageCanonicalActive, Disposition: disposition},
			assurance.IngestionFreshnessStage{Kind: assurance.IngestionStagePublicRetrievable, Disposition: assurance.IngestionDispositionUnavailable},
		)
		return input, nil
	}
	input.Stages = append(input.Stages, assurance.IngestionFreshnessStage{Kind: assurance.IngestionStageCanonicalActive, Disposition: assurance.IngestionDispositionComplete, At: canonicalActiveAt.UTC()})
	if !sourceVisible {
		input.Stages = append(input.Stages, assurance.IngestionFreshnessStage{Kind: assurance.IngestionStagePublicRetrievable, Disposition: assurance.IngestionDispositionIneligible})
		return input, nil
	}
	if publicReadAt == nil {
		input.Stages = append(input.Stages, assurance.IngestionFreshnessStage{Kind: assurance.IngestionStagePublicRetrievable, Disposition: assurance.IngestionDispositionUnavailable})
		return input, nil
	}
	input.Stages = append(input.Stages, assurance.IngestionFreshnessStage{Kind: assurance.IngestionStagePublicRetrievable, Disposition: assurance.IngestionDispositionComplete, At: publicReadAt.UTC()})
	return input, nil
}

func unavailableIngestionFreshnessInput(input assurance.IngestionFreshnessInput) assurance.IngestionFreshnessInput {
	input.Stages = []assurance.IngestionFreshnessStage{
		{Kind: assurance.IngestionStageAdmitted, Disposition: assurance.IngestionDispositionUnavailable},
		{Kind: assurance.IngestionStageGovernanceCompleted, Disposition: assurance.IngestionDispositionUnavailable},
		{Kind: assurance.IngestionStageCanonicalActive, Disposition: assurance.IngestionDispositionUnavailable},
		{Kind: assurance.IngestionStagePublicRetrievable, Disposition: assurance.IngestionDispositionUnavailable},
	}
	return input
}

func (r *Repository) RecordProviderMemoryRead(ctx context.Context, scope memory.Scope, memoryIDs []string, observedAt time.Time) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if observedAt.IsZero() || len(memoryIDs) == 0 {
		return nil
	}
	for _, memoryID := range memoryIDs {
		if _, err := uuid.Parse(memoryID); err != nil {
			return fmt.Errorf("invalid provider read memory reference")
		}
	}
	const query = `
INSERT INTO provider_memory_read_observations (tenant,project,namespace,memory_id,observed_at)
SELECT $1,$2,$3,m.id,$5
FROM canonical_memories m
WHERE m.tenant=$1 AND m.project=$2 AND m.namespace=$3 AND m.id=ANY($4::uuid[]) AND m.state='active'
ON CONFLICT DO NOTHING`
	if _, err := r.db.Exec(ctx, query, scope.Tenant, scope.Project, scope.Namespace, memoryIDs, observedAt.UTC()); err != nil {
		return fmt.Errorf("record provider memory read: %w", err)
	}
	return nil
}

func (r *Repository) StoreIngestionFreshnessEvidence(ctx context.Context, evidence assurance.IngestionFreshnessEvidence) (assurance.IngestionFreshnessEvidence, error) {
	if err := evidence.Validate(); err != nil {
		return assurance.IngestionFreshnessEvidence{}, err
	}
	payload, err := json.Marshal(evidence.Report)
	if err != nil {
		return assurance.IngestionFreshnessEvidence{}, fmt.Errorf("marshal ingestion freshness report: %w", err)
	}
	const insert = `
INSERT INTO assurance_ingestion_freshness_evidence (
 tenant, project, namespace, fixture_hash, identity_hash, report, observed_at, created_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (tenant,project,namespace,fixture_hash) DO NOTHING
RETURNING tenant,project,namespace,fixture_hash,identity_hash,report,observed_at,created_at`
	stored, err := scanIngestionFreshnessEvidence(r.db.QueryRow(ctx, insert,
		evidence.Scope.Tenant, evidence.Scope.Project, evidence.Scope.Namespace,
		evidence.FixtureHash, evidence.IdentityHash, payload, evidence.ObservedAt, evidence.CreatedAt,
	))
	if err == nil {
		return stored, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return assurance.IngestionFreshnessEvidence{}, fmt.Errorf("store ingestion freshness evidence: %w", err)
	}
	const existing = `
SELECT tenant,project,namespace,fixture_hash,identity_hash,report,observed_at,created_at
FROM assurance_ingestion_freshness_evidence
WHERE tenant=$1 AND project=$2 AND namespace=$3 AND fixture_hash=$4`
	stored, err = scanIngestionFreshnessEvidence(r.db.QueryRow(ctx, existing, evidence.Scope.Tenant, evidence.Scope.Project, evidence.Scope.Namespace, evidence.FixtureHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return assurance.IngestionFreshnessEvidence{}, fmt.Errorf("read ingestion freshness replay: %w", err)
	}
	if err != nil {
		return assurance.IngestionFreshnessEvidence{}, err
	}
	if stored.IdentityHash != evidence.IdentityHash {
		return assurance.IngestionFreshnessEvidence{}, assurance.ErrIngestionFreshnessReplayConflict
	}
	return stored, nil
}

type ingestionFreshnessEvidenceScanner interface{ Scan(...any) error }

func scanIngestionFreshnessEvidence(scanner ingestionFreshnessEvidenceScanner) (assurance.IngestionFreshnessEvidence, error) {
	var evidence assurance.IngestionFreshnessEvidence
	var payload []byte
	if err := scanner.Scan(&evidence.Scope.Tenant, &evidence.Scope.Project, &evidence.Scope.Namespace, &evidence.FixtureHash, &evidence.IdentityHash, &payload, &evidence.ObservedAt, &evidence.CreatedAt); err != nil {
		return assurance.IngestionFreshnessEvidence{}, err
	}
	if err := json.Unmarshal(payload, &evidence.Report); err != nil {
		return assurance.IngestionFreshnessEvidence{}, fmt.Errorf("unmarshal ingestion freshness report: %w", err)
	}
	if err := evidence.Validate(); err != nil {
		return assurance.IngestionFreshnessEvidence{}, fmt.Errorf("validate ingestion freshness evidence: %w", err)
	}
	return evidence, nil
}
