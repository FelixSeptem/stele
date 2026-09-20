package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestReadLatestContextProjectionUsesExactScopeAndFiltersHiddenItems(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	repo := NewRepository(mock)
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	projectionID := uuid.New()
	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT id, tenant, project, namespace, kind[\s\S]*freshness_eligible = TRUE`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, string(memory.ContextProjectionKindAlwaysVisible)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "kind", "version", "schema_version", "policy_version", "renderer_version", "source_watermark", "status", "created_at", "updated_at", "superseded_at", "freshness_category", "freshness_slo", "freshness_age_ms", "freshness_duration_ms", "freshness_eligible", "rebuild_checkpoint", "rebuild_required"}).
			AddRow(projectionID.String(), scope.Tenant, scope.Project, scope.Namespace, "always_visible", int64(1), "schema-v1", "policy-v1", "renderer-v1", []byte(`{}`), "active", now, now, nil, "fresh", "within_budget", int64(1000), int64(10), true, "cp-1", false))
	mock.ExpectQuery(`SELECT id, source_kind, source_id`).
		WithArgs(projectionID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(pgxmock.NewRows([]string{"id", "source_kind", "source_id", "source_version", "memory_id", "class", "lifecycle_state", "rendered_text", "sort_key", "citation", "temporal_fact_id", "valid_from", "valid_to"}).
			AddRow(uuid.New().String(), "canonical_version", uuid.New().String(), int64(1), nil, "profile", "active", "visible", "01", []byte(`{}`), "fact-visible", now, nil).
			AddRow(uuid.New().String(), "canonical_version", uuid.New().String(), int64(2), nil, "profile", "suppressed", "secret", "02", []byte(`{}`), "fact-hidden", now, nil))

	projection, err := repo.ReadLatestContextProjection(context.Background(), scope, memory.ContextProjectionKindAlwaysVisible)
	if err != nil {
		t.Fatalf("ReadLatestContextProjection() error = %v", err)
	}
	if projection.Scope != scope || len(projection.Items) != 1 || projection.Items[0].Text != "visible" {
		t.Fatalf("projection = %+v, want exact-scope visible item only", projection)
	}
	// The rendered item must carry the validity identity of the version it was
	// rendered from, so a provenance check can tell which window the evidence
	// described without re-reading canonical memory.
	if projection.Items[0].TemporalValidity.TemporalFactID != "fact-visible" {
		t.Fatalf("item temporal fact id = %q, want fact-visible", projection.Items[0].TemporalValidity.TemporalFactID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestReadLatestContextProjectionExcludesMissingFreshnessEvidence(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	repo := NewRepository(mock)
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	mock.ExpectQuery(`SELECT id, tenant, project, namespace, kind[\s\S]*freshness_eligible = TRUE`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, string(memory.ContextProjectionKindRetrieval)).
		WillReturnError(pgx.ErrNoRows)
	if _, err := repo.ReadLatestContextProjection(context.Background(), scope, memory.ContextProjectionKindRetrieval); err == nil {
		t.Fatal("missing freshness evidence returned an ordinary projection")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReadContextProjectionRejectsInvalidScopeAndID(t *testing.T) {
	repo := &Repository{}
	if _, err := repo.ReadContextProjection(context.Background(), memory.Scope{}, uuid.NewString()); err == nil {
		t.Fatal("missing scope accepted")
	}
	if _, err := repo.ReadContextProjection(context.Background(), memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, "not-a-uuid"); err == nil {
		t.Fatal("invalid id accepted")
	}
}

func TestListContextProjectionCandidatesExcludesHiddenLatestCanonicalVersion(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	repo := NewRepository(mock)
	scope := memory.Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}
	versionID := uuid.New()
	memoryID := uuid.New()
	mock.ExpectQuery(`SELECT\s+latest\.id,\s+canonical\.id`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, 10).
		WillReturnRows(pgxmock.NewRows([]string{"version_id", "memory_id", "version", "class", "state", "content", "created_at"}).
			AddRow(versionID.String(), memoryID.String(), int64(2), "profile", "suppressed", "hidden latest", time.Now().UTC()))

	candidates, err := repo.ListContextProjectionCandidates(context.Background(), scope, memory.ContextProjectionKindAlwaysVisible, 10)
	if err != nil {
		t.Fatalf("ListContextProjectionCandidates() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %+v, want hidden latest version excluded", candidates)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestDeleteContextProjectionEvidenceTargetsOnlySupersededExactScope(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewRepository(mock)
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	cutoff := time.Unix(10, 0).UTC()
	mock.ExpectExec(`DELETE FROM context_projections WHERE id IN .*status='superseded'`).WithArgs("t", "p", "n", cutoff, 5).WillReturnResult(pgxmock.NewResult("DELETE", 2))
	deleted, err := repo.DeleteContextProjectionEvidenceBefore(context.Background(), scope, cutoff, 5)
	if err != nil || deleted != 2 {
		t.Fatalf("deleted=%d err=%v", deleted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
