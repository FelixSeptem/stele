package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Materialize the newest identity before reading disclosure or lifecycle. In
// particular, a hidden latest write must never make an older write reappear.
const referenceSelector = `WITH selected AS MATERIALIZED (
 SELECT * FROM raw_events
 WHERE tenant=$1 AND project=$2 AND namespace=$3
 AND metadata->>'reference_contract_version'='durable-reference-v1'
 AND event_type IN ('provider.reference.task','provider.reference.run','provider.reference.handoff','provider.reference.compilation')
 AND `

const referenceStatusQuery = referenceSelector + `id=$4::uuid
 ORDER BY created_at DESC, id DESC LIMIT 1
)` + referenceProjection

const referenceLookupQuery = referenceSelector + `metadata->>'reference_id'=$4 AND memory_path=$5
 ORDER BY created_at DESC, id DESC LIMIT 1
)` + referenceProjection

// Each lateral selection chooses the durable head before applying its gates.
// Nullable joins keep accepted, pending and failed raw events observable without
// claiming canonical completion. All lineage joins remain inside exact scope.
const referenceProjection = `
SELECT e.id::text, COALESCE(e.metadata->>'reference_id',''), e.memory_path, e.event_type,
 COALESCE(
  e.source_metadata->>'version'='source-metadata-v1'
  AND e.source_metadata->>'trust' IN ('untrusted','attested_data')
  AND e.source_metadata->>'classification' IN ('low','medium')
  AND (c.id IS NULL OR (
   c.source_metadata->>'version'='source-metadata-v1'
   AND c.source_metadata->>'trust' IN ('untrusted','attested_data')
   AND c.source_metadata->>'classification' IN ('low','medium')))
  AND (m.id IS NULL OR (
   m.state='active'
   AND m.source_metadata->>'version'='source-metadata-v1'
   AND m.source_metadata->>'trust' IN ('untrusted','attested_data')
   AND m.source_metadata->>'classification' IN ('low','medium')))
  AND (v.id IS NULL OR (
   v.state='active'
   AND v.source_metadata->>'version'='source-metadata-v1'
   AND v.source_metadata->>'trust' IN ('untrusted','attested_data')
   AND v.source_metadata->>'classification' IN ('low','medium'))),false) AS sources_visible,
 e.governance_processed_at IS NOT NULL,
 e.governance_last_failed_at IS NOT NULL,
 e.governance_exhausted_at IS NOT NULL,
 COALESCE(c.status='promoted' AND m.state='active' AND v.state='active'
  AND c.memory_path=e.memory_path AND m.memory_path=e.memory_path AND v.memory_path=e.memory_path
  AND p.id IS NOT NULL AND v.version>=1,false) AS completed,
 COALESCE(c.id::text,''), COALESCE(m.id::text,''), COALESCE(v.id::text,''), COALESCE(p.id::text,''),
 COALESCE(v.version,0), COALESCE(v.content,'')
FROM selected e
LEFT JOIN LATERAL (
 SELECT * FROM candidate_memories c WHERE c.source_raw_event_id=e.id
 AND c.tenant=e.tenant AND c.project=e.project AND c.namespace=e.namespace
 ORDER BY c.created_at DESC,c.id DESC LIMIT 1
) c ON true
LEFT JOIN LATERAL (
 SELECT * FROM provenance_links p WHERE p.raw_event_id=e.id AND p.candidate_memory_id=c.id
 AND p.tenant=e.tenant AND p.project=e.project AND p.namespace=e.namespace
 AND p.operation='promote_candidate' AND p.memory_id IS NOT NULL
 ORDER BY p.created_at DESC,p.id DESC LIMIT 1
) p ON true
LEFT JOIN canonical_memories m ON m.id=p.memory_id
 AND m.tenant=e.tenant AND m.project=e.project AND m.namespace=e.namespace
LEFT JOIN LATERAL (
 SELECT * FROM memory_versions v WHERE v.memory_id=m.id
 ORDER BY version DESC LIMIT 1
) v ON true
`

var _ provider.ReferenceStore = (*Repository)(nil)

func (r *Repository) ReferenceStatus(ctx context.Context, scope memory.Scope, principalID, eventID, profile, mode string) (provider.ReferenceResponse, error) {
	if _, err := uuid.Parse(eventID); err != nil {
		return provider.ReferenceResponse{}, fmt.Errorf("invalid reference event identity")
	}
	return r.referenceRead(ctx, scope, profile, mode, referenceStatusQuery, eventID)
}
func (r *Repository) ReferenceLookup(ctx context.Context, scope memory.Scope, principalID, referenceID, path, profile, mode string) (provider.ReferenceResponse, error) {
	input := provider.ReferenceLookupInput{ContractVersion: provider.DurableReferenceContractVersion, ReferenceID: referenceID, MemoryPath: path, DisclosureProfile: profile, ResponseMode: mode}
	if err := input.Validate(); err != nil {
		return provider.ReferenceResponse{}, err
	}
	normalizedPath, err := memory.NormalizeMemoryPath(input.MemoryPath)
	if err != nil {
		return provider.ReferenceResponse{}, err
	}
	return r.referenceRead(ctx, scope, profile, mode, referenceLookupQuery, referenceID, normalizedPath)
}

func (r *Repository) referenceRead(ctx context.Context, scope memory.Scope, profile, mode, query string, selector ...any) (provider.ReferenceResponse, error) {
	out := provider.ReferenceResponse{ContractVersion: provider.DurableReferenceContractVersion, Status: provider.ReferenceStatusUnavailable}
	if err := scope.Validate(); err != nil {
		return out, err
	}
	// The HTTP boundary supplies the currently authorized profile and mode. Empty
	// or unknown decisions cannot become ordinary visibility at the repository.
	if (profile != "memory-reference-v1" && profile != "memory-content-v1") || (mode != "reference_only" && mode != "bounded_content") || (mode == "bounded_content" && profile != "memory-content-v1") {
		return out, fmt.Errorf("reference disclosure decision is invalid")
	}
	args := []any{scope.Tenant, scope.Project, scope.Namespace}
	args = append(args, selector...)
	var rec provider.ReferenceRecord
	var visible, processed, failed, exhausted, complete bool
	err := r.db.QueryRow(ctx, query, args...).Scan(&rec.EventID, &rec.ReferenceID, &rec.MemoryPath, &rec.EventType, &visible, &processed, &failed, &exhausted, &complete, &rec.CandidateID, &rec.MemoryID, &rec.VersionID, &rec.ProvenanceID, &rec.Version, &rec.Value)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("read durable reference: %w", err)
	}
	if !visible {
		return out, nil
	}
	if !processed {
		out.EventID = rec.EventID
		out.Status = provider.ReferenceStatusPending
		out.Retryable = true
		out.RetryAfterMS = 1000
		if failed || exhausted {
			out.Status = provider.ReferenceStatusFailed
			out.Retryable = !exhausted
			out.RetryAfterMS = 0
			if out.Retryable {
				out.ErrorCategory = "retryable"
				out.RetryAfterMS = 1000
			}
		}
		return out, nil
	}
	if !complete || !validReferenceRecord(rec) {
		return out, nil
	}
	out.Status = provider.ReferenceStatusCompleted
	if mode != "bounded_content" {
		rec.Value = ""
	}
	out.Reference = &rec
	return out, nil
}

func validReferenceRecord(rec provider.ReferenceRecord) bool {
	for _, id := range []string{rec.EventID, rec.CandidateID, rec.MemoryID, rec.VersionID, rec.ProvenanceID} {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed == uuid.Nil || parsed.String() != strings.ToLower(id) {
			return false
		}
	}
	if rec.Version < 1 || len(rec.Value) > provider.MaxReferenceValueBytes || !utf8.ValidString(rec.Value) {
		return false
	}
	if rec.ReferenceID == "" || len(rec.ReferenceID) > provider.MaxReferenceIDBytes || !isReferenceToken(rec.ReferenceID) {
		return false
	}
	normalized, err := memory.NormalizeMemoryPath(rec.MemoryPath)
	return err == nil && normalized == rec.MemoryPath && normalized != memory.MemoryPathRoot && memory.IsReferenceEventType(rec.EventType)
}

func isReferenceToken(value string) bool {
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("-_.:", r) {
			continue
		}
		return false
	}
	return value != ""
}
