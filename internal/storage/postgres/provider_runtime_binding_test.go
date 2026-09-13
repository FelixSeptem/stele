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
	// Keep the fixture valid against the repository's current-time expiry check.
	now := time.Now().UTC()
	binding := provider.RuntimeBinding{BindingID: "rb_opaque", PrincipalID: "principal-1", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, AgentID: "agent-1", SessionID: "session-1", ConversationID: "conversation-1", ProviderInstanceID: "instance-1", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	mock.ExpectExec("INSERT INTO provider_runtime_bindings").WithArgs(binding.BindingID, binding.PrincipalID, "tenant-a", "project-a", "namespace-a", binding.AgentID, binding.SessionID, binding.ConversationID, binding.ProviderInstanceID, binding.CreatedAt, binding.ExpiresAt).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := NewRepository(mock).Create(context.Background(), binding); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	mock.ExpectQuery("FROM provider_runtime_bindings").WithArgs(binding.BindingID).WillReturnRows(pgxmock.NewRows([]string{"binding_id", "principal_id", "tenant", "project", "namespace", "agent_id", "session_id", "conversation_id", "provider_instance_id", "created_at", "expires_at", "revoked_at"}).AddRow(binding.BindingID, binding.PrincipalID, "tenant-a", "project-a", "namespace-a", binding.AgentID, binding.SessionID, binding.ConversationID, binding.ProviderInstanceID, binding.CreatedAt, binding.ExpiresAt, nil))
	got, err := NewRepository(mock).Lookup(context.Background(), binding.BindingID)
	if err != nil || got.BindingID != binding.BindingID || got.Scope != binding.Scope {
		t.Fatalf("Lookup() = %+v, %v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
