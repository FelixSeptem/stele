package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
)

// TestTemporalBackfillIsGuardedAndNonDestructive asserts the static shape of the
// legacy backfill: it must only touch rows with unset temporal columns, derive
// its anchors from the recorded creation time, and never clear an existing
// explicit value.
func TestTemporalBackfillIsGuardedAndNonDestructive(t *testing.T) {
	sql := readTemporalMigration(t)

	// Every backfill column assignment must be COALESCE-guarded so an existing
	// explicit snapshot is preserved rather than overwritten with legacy data.
	for _, fragment := range []string{
		"temporal_fact_id = COALESCE(temporal_fact_id, memory_id::text)",
		"ingested_at = COALESCE(ingested_at, created_at)",
		"valid_from = COALESCE(valid_from, created_at)",
		"temporal_fact_id = COALESCE(cm.temporal_fact_id, cm.id::text)",
		"ingested_at = COALESCE(cm.ingested_at, cm.created_at)",
		"valid_from = COALESCE(cm.valid_from, cm.created_at)",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("backfill missing non-destructive assignment %q", fragment)
		}
	}

	// valid_to must never be written: legacy rows stay open-ended so they remain
	// current-valid.
	if strings.Contains(sql, "valid_to = ") {
		t.Error("backfill must not assign valid_to; legacy rows stay open-ended")
	}

	// The guard is what makes a rerun a no-op.
	if !strings.Contains(sql, "WHERE temporal_fact_id IS NULL OR ingested_at IS NULL OR valid_from IS NULL") {
		t.Error("memory_versions backfill must be guarded on unset columns")
	}
	if !strings.Contains(sql, "WHERE cm.temporal_fact_id IS NULL OR cm.ingested_at IS NULL OR cm.valid_from IS NULL") {
		t.Error("canonical_memories backfill must be guarded on unset columns")
	}

	// Backfill must not touch scope, lifecycle, or provenance columns at all.
	for _, forbidden := range []string{
		"SET tenant", "SET project", "SET namespace", "SET state",
		"SET class", "SET content", "SET updated_at", "SET modified_by",
		"INSERT INTO provenance_links", "DELETE FROM",
	} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("backfill must not mutate %q", forbidden)
		}
	}
}

// TestTemporalBackfillAnchorsLegacyRowsOnRecordedTime proves the derived values
// are exactly the ones the in-memory legacy fallback expects: fact identity from
// the memory id, and an open interval starting at the recorded creation time.
func TestTemporalBackfillAnchorsLegacyRowsOnRecordedTime(t *testing.T) {
	sql := readTemporalMigration(t)

	if !strings.Contains(sql, "validity_source = COALESCE(NULLIF(validity_source, ''), 'legacy_current_compatible')") {
		t.Error("memory_versions backfill must default validity_source to legacy_current_compatible, reusing a non-empty existing value")
	}
	if !strings.Contains(sql, "validity_source = COALESCE(NULLIF(cm.validity_source, ''), 'legacy_current_compatible')") {
		t.Error("canonical_memories backfill must default validity_source to legacy_current_compatible")
	}
	// The SQL literal must match the Go constant the read path relies on;
	// otherwise backfilled rows would carry a source the reader rejects.
	if !strings.Contains(sql, "'"+string(memory.TemporalValiditySourceLegacyCompatible)+"'") {
		t.Errorf("backfill must use the Go legacy source token %q", memory.TemporalValiditySourceLegacyCompatible)
	}
}

// TestTemporalBackfillLeavesDerivedRowsAlone verifies derived artifacts are only
// extended with temporal identity, never rewritten by the canonical backfill.
func TestTemporalBackfillLeavesDerivedRowsAlone(t *testing.T) {
	sql := readTemporalMigration(t)

	for _, table := range []string{"relation_projections", "memory_chunk_derivations", "context_projection_items"} {
		start := strings.Index(sql, "ALTER TABLE "+table)
		if start < 0 {
			t.Fatalf("migration must extend %s", table)
		}
		block := sql[start:]
		if idx := strings.Index(block, ";"); idx >= 0 {
			block = block[:idx]
		}
		if strings.Contains(block, "NOT NULL") {
			t.Errorf("%s temporal columns must stay nullable so existing derived rows remain valid", table)
		}
	}

	// No UPDATE may target a derived table; existing derived rows are preserved
	// as-is and their temporal identity is supplied on rebuild.
	for _, derived := range []string{"UPDATE relation_projections", "UPDATE memory_chunk_derivations", "UPDATE context_projection_items"} {
		if strings.Contains(sql, derived) {
			t.Errorf("backfill must not rewrite derived table via %q", derived)
		}
	}
}

// TestTemporalBackfillRerunIsNoOp performs a real upgrade rehearsal when a
// disposable DSN is configured: it seeds a pre-migration row, applies the
// ledger, captures the backfilled values, reapplies the same backfill statement,
// and proves nothing changed.
func TestTemporalBackfillRerunIsNoOp(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_TEMPORAL_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_TEMPORAL_DSN is not configured; skipping real PostgreSQL backfill rehearsal")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatalf("OpenPool() error = %v", err)
	}
	defer pool.Close()
	if err := BootstrapDatabase(ctx, pool); err != nil {
		t.Fatalf("BootstrapDatabase() error = %v", err)
	}

	scope := memory.Scope{Tenant: "temporal-backfill", Project: "p", Namespace: "n"}
	createdAt := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	memoryID := uuid.NewString()
	versionID := uuid.NewString()

	if _, err := pool.Exec(ctx, `INSERT INTO canonical_memories (id, tenant, project, namespace, class, state, content, metadata, created_at, updated_at) VALUES ($1, $2, $3, $4, 'episodic', 'active', 'backfill fixture', '{}'::jsonb, $5, $5)`,
		memoryID, scope.Tenant, scope.Project, scope.Namespace, createdAt); err != nil {
		t.Fatalf("insert pre-migration canonical memory: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO memory_versions (id, memory_id, version, state, content, metadata, created_at, modified_by) VALUES ($1, $2, 1, 'active', 'backfill fixture', '{}'::jsonb, $3, 'fixture')`,
		versionID, memoryID, createdAt); err != nil {
		t.Fatalf("insert pre-migration memory version: %v", err)
	}

	runner := NewMigrationRunner()
	if err := runner.Apply(ctx, dsn); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	type snapshot struct {
		FactID, Source        string
		IngestedAt, ValidFrom time.Time
		ValidTo               *time.Time
		Active, Episodic      bool
		ScopePreserved        bool
	}
	read := func() snapshot {
		var s snapshot
		var tenant, project, namespace, state, class string
		var validTo *time.Time
		if err := pool.QueryRow(ctx, `SELECT temporal_fact_id, validity_source, ingested_at, valid_from, valid_to, state, class, tenant, project, namespace FROM canonical_memories WHERE id = $1`, memoryID).
			Scan(&s.FactID, &s.Source, &s.IngestedAt, &s.ValidFrom, &validTo, &state, &class, &tenant, &project, &namespace); err != nil {
			t.Fatalf("read backfilled row: %v", err)
		}
		s.ValidTo = validTo
		s.Active = state == string(memory.MemoryStateActive)
		s.Episodic = class == string(memory.MemoryClassEpisodic)
		s.ScopePreserved = tenant == scope.Tenant && project == scope.Project && namespace == scope.Namespace
		return s
	}

	first := read()

	// The backfill must anchor the legacy row on its recorded creation time with
	// an open interval and a legacy-compatible source.
	if first.FactID != memoryID {
		t.Fatalf("temporal_fact_id = %q, want the memory id %q", first.FactID, memoryID)
	}
	if first.Source != string(memory.TemporalValiditySourceLegacyCompatible) {
		t.Fatalf("validity_source = %q, want %q", first.Source, memory.TemporalValiditySourceLegacyCompatible)
	}
	if !first.IngestedAt.Equal(createdAt) || !first.ValidFrom.Equal(createdAt) {
		t.Fatalf("backfill anchors = %s / %s, want recorded time %s", first.IngestedAt, first.ValidFrom, createdAt)
	}
	if first.ValidTo != nil {
		t.Fatalf("valid_to = %v, want open-ended current interval", *first.ValidTo)
	}
	// Scope and lifecycle must be preserved untouched.
	if !first.Active || !first.Episodic || !first.ScopePreserved {
		t.Fatalf("backfill mutated scope or lifecycle: %+v", first)
	}

	// Reapply the backfill statement verbatim: because it is guarded on unset
	// columns it must not modify the already-backfilled row.
	if _, err := pool.Exec(ctx, `UPDATE canonical_memories cm
SET temporal_fact_id = COALESCE(cm.temporal_fact_id, cm.id::text),
    ingested_at = COALESCE(cm.ingested_at, cm.created_at),
    valid_from = COALESCE(cm.valid_from, cm.created_at),
    validity_source = COALESCE(NULLIF(cm.validity_source, ''), 'legacy_current_compatible')
WHERE cm.temporal_fact_id IS NULL OR cm.ingested_at IS NULL OR cm.valid_from IS NULL OR cm.validity_source IS NULL OR cm.validity_source = ''`); err != nil {
		t.Fatalf("reapply backfill: %v", err)
	}

	second := read()
	if second.FactID != first.FactID || second.Source != first.Source ||
		!second.IngestedAt.Equal(first.IngestedAt) || !second.ValidFrom.Equal(first.ValidFrom) {
		t.Fatalf("backfill rerun changed the row: first=%+v second=%+v", first, second)
	}
	if (second.ValidTo == nil) != (first.ValidTo == nil) {
		t.Fatalf("backfill rerun changed valid_to: first=%v second=%v", first.ValidTo, second.ValidTo)
	}

	// A row whose explicit snapshot is already set must survive untouched.
	explicitFrom := createdAt.Add(-48 * time.Hour)
	if _, err := pool.Exec(ctx, `UPDATE canonical_memories SET temporal_fact_id = 'explicit-fact', valid_from = $2, validity_source = 'explicit' WHERE id = $1`, memoryID, explicitFrom); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE canonical_memories cm
SET temporal_fact_id = COALESCE(cm.temporal_fact_id, cm.id::text),
    ingested_at = COALESCE(cm.ingested_at, cm.created_at),
    valid_from = COALESCE(cm.valid_from, cm.created_at),
    validity_source = COALESCE(NULLIF(cm.validity_source, ''), 'legacy_current_compatible')
WHERE cm.temporal_fact_id IS NULL OR cm.ingested_at IS NULL OR cm.valid_from IS NULL OR cm.validity_source IS NULL OR cm.validity_source = ''`); err != nil {
		t.Fatal(err)
	}
	var factID, source string
	var validFrom time.Time
	if err := pool.QueryRow(ctx, `SELECT temporal_fact_id, validity_source, valid_from FROM canonical_memories WHERE id = $1`, memoryID).Scan(&factID, &source, &validFrom); err != nil {
		t.Fatal(err)
	}
	if factID != "explicit-fact" || source != "explicit" || !validFrom.Equal(explicitFrom) {
		t.Fatalf("backfill overwrote an explicit snapshot: fact=%q source=%q valid_from=%s", factID, source, validFrom)
	}
}
