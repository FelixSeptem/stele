package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryCreatesAndLooksUpProviderRuntimeBinding(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	binding := provider.RuntimeBinding{BindingID: "rb_opaque", PrincipalID: "principal-1", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, AgentID: "agent", SessionID: "session", ConversationID: "conversation", ProviderInstanceID: "instance", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	mock.ExpectExec("INSERT INTO provider_runtime_bindings").WithArgs(binding.BindingID, binding.PrincipalID, "tenant-a", "project-a", "namespace-a", "agent", "session", "conversation", "instance", now, now.Add(time.Hour)).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	repository := NewRepository(mock)
	if err := repository.Create(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("FROM provider_runtime_bindings").WithArgs(binding.BindingID).WillReturnRows(pgxmock.NewRows([]string{"binding_id", "principal_id", "tenant", "project", "namespace", "agent_id", "session_id", "conversation_id", "provider_instance_id", "created_at", "expires_at", "revoked_at"}).AddRow(binding.BindingID, binding.PrincipalID, "tenant-a", "project-a", "namespace-a", "agent", "session", "conversation", "instance", now, now.Add(time.Hour), nil))
	got, err := repository.Lookup(context.Background(), binding.BindingID)
	if err != nil || got != binding {
		t.Fatalf("Lookup() = %+v, %v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
