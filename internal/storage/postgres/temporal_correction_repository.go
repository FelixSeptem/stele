package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/jackc/pgx/v5"
)

// temporalCorrectionInsert is the ledger append. It is deliberately an INSERT
// with an ON CONFLICT guard: the ledger is append-only, so replaying a
// correction converges on the stored row (RowsAffected == 0) instead of
// erroring, and an existing correction is never rewritten in place.
const temporalCorrectionInsert = `
INSERT INTO temporal_corrections (
	tenant,
	project,
	namespace,
	temporal_fact_id,
	memory_id,
	predecessor_version,
	successor_version,
	actor,
	reason,
	disposition,
	created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (tenant, project, namespace, memory_id, successor_version) DO NOTHING
`

// temporalCorrectionSelect reads one scope's lineage oldest-first. The ordering
// is part of the contract: successor_version ascending makes the chain
// reproducible without a defensive re-sort, and a stable tiebreak on created_at
// keeps two corrections that share a version from alternating between reads.
const temporalCorrectionSelect = `
SELECT
	temporal_fact_id,
	predecessor_version,
	successor_version,
	actor,
	reason,
	disposition,
	created_at
FROM temporal_corrections
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND memory_id = $4
ORDER BY successor_version ASC, created_at ASC
`

// temporalCorrectionLookup reads the durable identity of one already-recorded
// correction so a conflicting replay can be compared against what is stored.
const temporalCorrectionLookup = `
SELECT
	temporal_fact_id,
	predecessor_version,
	successor_version,
	actor,
	reason,
	disposition,
	created_at
FROM temporal_corrections
WHERE tenant = $1
	AND project = $2
	AND namespace = $3
	AND memory_id = $4
	AND successor_version = $5
`

// temporalCorrectionHeadAdvance moves the canonical row's temporal head pointer
// to the newly recorded successor version. It touches only the identity and head
// columns: the canonical content and every memory_versions payload stay exactly
// as they were, which is what makes retroactive correction an append rather than
// a rewrite.
const temporalCorrectionHeadAdvance = `
UPDATE canonical_memories
SET temporal_fact_id = COALESCE(temporal_fact_id, $5),
	temporal_head_version = $6,
	updated_at = now()
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
`

// temporalCorrectionHeadGuard reads the canonical row's current temporal head so
// the append can refuse a non-contiguous successor: a gap would leave a version
// that no correction accounts for.
// The guard also reads the class so a factual correction pays no extra round
// trip: only a relation fact owns a derived projection that has to be rebuilt.
const temporalCorrectionHeadGuard = `
SELECT
	COALESCE(temporal_fact_id, id::text),
	COALESCE(temporal_head_version, 0),
	class
FROM canonical_memories
WHERE id = $1
	AND tenant = $2
	AND project = $3
	AND namespace = $4
`

// AppendTemporalCorrection records one append-only validity correction and
// advances the canonical row's temporal head pointer. It never rewrites a prior
// version payload: memory_versions rows are insert-only and this path issues no
// statement against them at all.
//
// The append is idempotent on its durable identity (scope + memory + successor
// version). A replay that agrees with the stored record converges silently; a
// replay that disagrees is refused with ErrTemporalCorrectionConflict rather
// than allowed to overwrite the ledger.
func (r *Repository) AppendTemporalCorrection(ctx context.Context, correction memory.TemporalCorrection) error {
	// The domain contract is enforced at the boundary so an unattributed or
	// non-advancing correction never reaches storage.
	if err := correction.Validate(); err != nil {
		return err
	}
	scope := correction.Scope.Normalized()
	if err := scope.Validate(); err != nil {
		return err
	}

	tx, err := r.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin temporal correction transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Scope isolation: confirm the memory belongs to this scope and read its
	// current head so a gap is refused before anything is written.
	var factID string
	var headVersion int64
	var class string
	if err := tx.QueryRow(
		ctx,
		temporalCorrectionHeadGuard,
		correction.MemoryID,
		scope.Tenant,
		scope.Project,
		scope.Namespace,
	).Scan(&factID, &headVersion, &class); err != nil {
		if err == pgx.ErrNoRows {
			return pgx.ErrNoRows
		}
		return fmt.Errorf("read canonical temporal head: %w", err)
	}

	if correction.TemporalFactID != factID {
		return fmt.Errorf("%w: correction names fact %q but the canonical row is %q",
			memory.ErrTemporalCorrectionIdentityRequired, correction.TemporalFactID, factID)
	}
	if expected := headVersion + 1; correction.SuccessorVersion != expected {
		return fmt.Errorf("%w: successor %d does not follow the current head %d",
			memory.ErrTemporalCorrectionLineageInvalid, correction.SuccessorVersion, headVersion)
	}
	if correction.PredecessorVersion != nil && *correction.PredecessorVersion != headVersion {
		return fmt.Errorf("%w: predecessor %d is not the current head %d",
			memory.ErrTemporalCorrectionLineageInvalid, *correction.PredecessorVersion, headVersion)
	}

	tag, err := tx.Exec(
		ctx,
		temporalCorrectionInsert,
		scope.Tenant,
		scope.Project,
		scope.Namespace,
		correction.TemporalFactID,
		correction.MemoryID,
		nullableOptionalVersion(correction.PredecessorVersion),
		correction.SuccessorVersion,
		strings.TrimSpace(correction.Actor),
		strings.TrimSpace(correction.Reason),
		string(correction.Disposition),
		correction.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("append temporal correction: %w", err)
	}

	if tag.RowsAffected() == 0 {
		// A correction for this version already exists. Converge on agreement,
		// refuse disagreement: the ledger is append-only.
		stored, err := readStoredTemporalCorrection(ctx, tx, scope, correction)
		if err != nil {
			return err
		}
		if !sameTemporalCorrection(stored, correction) {
			return fmt.Errorf("%w: successor version %d", memory.ErrTemporalCorrectionConflict, correction.SuccessorVersion)
		}
		return tx.Commit(ctx)
	}

	if _, err := tx.Exec(
		ctx,
		temporalCorrectionHeadAdvance,
		correction.MemoryID,
		scope.Tenant,
		scope.Project,
		scope.Namespace,
		correction.TemporalFactID,
		correction.SuccessorVersion,
	); err != nil {
		return fmt.Errorf("advance canonical temporal head: %w", err)
	}

	// Derived evidence is bound to one source version, so a successor leaves any
	// projection naming the superseded version stale. Rebuild it here against the
	// successor instead of waiting for an unrelated write; the previous lineage
	// stays auditable through append-only versions and the correction ledger.
	//
	// The successor's own snapshot is read from memory_versions rather than the
	// canonical head, because the head's validity columns describe the latest
	// ordinary write and are not the successor's fact-valid interval.
	if memory.MemoryClass(class) == memory.MemoryClassRelation {
		canonical, err := readScopedCanonicalMemory(ctx, tx, scope, correction.MemoryID)
		if err != nil {
			return fmt.Errorf("read corrected canonical memory: %w", err)
		}
		successor, err := readVersionTemporalSnapshot(ctx, tx, correction.MemoryID, correction.SuccessorVersion)
		if err != nil {
			return fmt.Errorf("read corrected successor version: %w", err)
		}
		canonical.TemporalValidity = successor
		if err := upsertRelationProjection(ctx, tx, canonical, correction.SuccessorVersion, correction.CreatedAt); err != nil {
			return fmt.Errorf("rebuild relation projection after correction: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit temporal correction: %w", err)
	}

	return nil
}

// temporalSuccessorSnapshot reads the fact-valid identity recorded on one
// append-only version. It is the authoritative source for a derived artifact's
// snapshot, because the canonical head is only a projection of the selected
// version.
const temporalSuccessorSnapshot = `
SELECT
	temporal_fact_id,
	COALESCE(ingested_at, created_at),
	COALESCE(valid_from, created_at),
	valid_to,
	validity_source
FROM memory_versions
WHERE memory_id = $1
	AND version = $2
`

func readVersionTemporalSnapshot(ctx context.Context, db queryRower, memoryID string, version int64) (memory.TemporalValidity, error) {
	var validity memory.TemporalValidity
	var factID sql.NullString
	var validitySource sql.NullString
	if err := db.QueryRow(ctx, temporalSuccessorSnapshot, memoryID, version).Scan(
		&factID,
		&validity.IngestedAt,
		&validity.ValidFrom,
		&validity.ValidTo,
		&validitySource,
	); err != nil {
		return memory.TemporalValidity{}, fmt.Errorf("read version temporal snapshot: %w", err)
	}
	if factID.Valid {
		validity.TemporalFactID = factID.String
	}
	if validitySource.Valid {
		validity.ValiditySource = memory.TemporalValiditySource(validitySource.String)
	}
	return validity, nil
}

// ReadTemporalLineage returns a fact's correction history in append order. An
// uncorrected fact reads as an empty lineage rather than an error. A stored
// lineage that skips a version fails the domain contract, so corruption is
// surfaced instead of being silently reordered.
func (r *Repository) ReadTemporalLineage(ctx context.Context, scope memory.Scope, memoryID string) (memory.TemporalLineage, error) {
	if err := scope.Validate(); err != nil {
		return memory.TemporalLineage{}, err
	}
	if strings.TrimSpace(memoryID) == "" {
		return memory.TemporalLineage{}, fmt.Errorf("memory id is required")
	}
	normalized := scope.Normalized()

	rows, err := r.db.Query(
		ctx,
		temporalCorrectionSelect,
		normalized.Tenant,
		normalized.Project,
		normalized.Namespace,
		memoryID,
	)
	if err != nil {
		return memory.TemporalLineage{}, fmt.Errorf("read temporal corrections: %w", err)
	}
	defer rows.Close()

	var lineage memory.TemporalLineage
	for rows.Next() {
		correction, err := scanTemporalCorrection(rows, normalized, memoryID)
		if err != nil {
			return memory.TemporalLineage{}, err
		}
		if lineage.TemporalFactID == "" {
			lineage.TemporalFactID = correction.TemporalFactID
		}
		lineage.Corrections = append(lineage.Corrections, correction)
	}
	if err := rows.Err(); err != nil {
		return memory.TemporalLineage{}, fmt.Errorf("iterate temporal corrections: %w", err)
	}

	ordered := memory.OrderTemporalCorrections(lineage.Corrections)
	lineage.Corrections = ordered
	if len(lineage.Corrections) == 0 {
		return lineage, nil
	}
	if err := lineage.Validate(); err != nil {
		return memory.TemporalLineage{}, fmt.Errorf("%w: %v", memory.ErrTemporalCorrectionGap, err)
	}

	return lineage, nil
}

type temporalCorrectionScanner interface {
	Scan(dest ...any) error
}

func scanTemporalCorrection(row temporalCorrectionScanner, scope memory.Scope, memoryID string) (memory.TemporalCorrection, error) {
	var (
		factID      string
		predecessor sql.NullInt64
		successor   int64
		actor       string
		reason      string
		disposition string
		createdAt   time.Time
	)
	if err := row.Scan(
		&factID,
		&predecessor,
		&successor,
		&actor,
		&reason,
		&disposition,
		&createdAt,
	); err != nil {
		return memory.TemporalCorrection{}, fmt.Errorf("scan temporal correction: %w", err)
	}

	return memory.TemporalCorrection{
		Scope:              scope,
		TemporalFactID:     factID,
		MemoryID:           memoryID,
		PredecessorVersion: nullableInt64Ptr(predecessor),
		SuccessorVersion:   successor,
		Actor:              actor,
		Reason:             reason,
		Disposition:        memory.TemporalConflictDisposition(disposition),
		CreatedAt:          createdAt,
	}, nil
}

// nullableInt64Ptr converts a scanned NULL-able version into the domain's
// optional pointer, keeping SQL NULL and "unset" the same thing.
func nullableInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	version := value.Int64
	return &version
}

// nullableOptionalVersion renders the domain's optional predecessor version as
// a nullable SQL argument, so the first correction of a fact stores NULL rather
// than a zero that would look like a real version.
func nullableOptionalVersion(version *int64) any {
	if version == nil {
		return nil
	}
	return *version
}

func readStoredTemporalCorrection(ctx context.Context, tx pgx.Tx, scope memory.Scope, correction memory.TemporalCorrection) (memory.TemporalCorrection, error) {
	row := tx.QueryRow(
		ctx,
		temporalCorrectionLookup,
		scope.Tenant,
		scope.Project,
		scope.Namespace,
		correction.MemoryID,
		correction.SuccessorVersion,
	)
	stored, err := scanTemporalCorrection(row, scope, correction.MemoryID)
	if err != nil {
		return memory.TemporalCorrection{}, err
	}
	return stored, nil
}

// sameTemporalCorrection compares the fields a replay must agree on. Actor and
// reason are trimmed on write, so they are compared trimmed here too.
func sameTemporalCorrection(stored, incoming memory.TemporalCorrection) bool {
	if stored.TemporalFactID != incoming.TemporalFactID {
		return false
	}
	if stored.SuccessorVersion != incoming.SuccessorVersion {
		return false
	}
	if !sameOptionalVersion(stored.PredecessorVersion, incoming.PredecessorVersion) {
		return false
	}
	if stored.Actor != strings.TrimSpace(incoming.Actor) || stored.Reason != strings.TrimSpace(incoming.Reason) {
		return false
	}
	return stored.Disposition == incoming.Disposition
}

func sameOptionalVersion(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
