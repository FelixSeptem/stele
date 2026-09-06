package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
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
	mock.ExpectQuery(`SELECT id, tenant, project, namespace, kind`).
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, string(memory.ContextProjectionKindAlwaysVisible)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "tenant", "project", "namespace", "kind", "version", "schema_version", "policy_version", "renderer_version", "source_watermark", "status", "created_at", "updated_at", "superseded_at"}).
			AddRow(projectionID.String(), scope.Tenant, scope.Project, scope.Namespace, "always_visible", int64(1), "schema-v1", "policy-v1", "renderer-v1", []byte(`{}`), "active", now, now, nil))
	mock.ExpectQuery(`SELECT id, source_kind, source_id`).
		WithArgs(projectionID, scope.Tenant, scope.Project, scope.Namespace).
		WillReturnRows(pgxmock.NewRows([]string{"id", "source_kind", "source_id", "source_version", "memory_id", "class", "lifecycle_state", "rendered_text", "sort_key", "citation"}).
			AddRow(uuid.New().String(), "canonical_version", uuid.New().String(), int64(1), nil, "profile", "active", "visible", "01", []byte(`{}`)).
			AddRow(uuid.New().String(), "canonical_version", uuid.New().String(), int64(2), nil, "profile", "suppressed", "secret", "02", []byte(`{}`)))

	projection, err := repo.ReadLatestContextProjection(context.Background(), scope, memory.ContextProjectionKindAlwaysVisible)
	if err != nil {
		t.Fatalf("ReadLatestContextProjection() error = %v", err)
	}
	if projection.Scope != scope || len(projection.Items) != 1 || projection.Items[0].Text != "visible" {
		t.Fatalf("projection = %+v, want exact-scope visible item only", projection)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
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
