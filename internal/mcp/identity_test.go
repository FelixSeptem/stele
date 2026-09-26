package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestBuildWhoAmIRedactsCredentialsAndBoundsScopeSummary(t *testing.T) {
	response := BuildWhoAmI(DispatchContext{PrincipalID: "principal-1", Role: "public", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, AccessMode: "explicit"}, true)
	if response.PrincipalID != "principal-1" || response.Role != "public" || response.AccessMode != "explicit" || !response.ActiveScopeAvailable {
		t.Fatalf("who_am_i = %+v, want bounded identity summary", response)
	}
	if response.ScopeDigest == "" || len(response.ScopeDigest) > 24 {
		t.Fatalf("ScopeDigest = %q, want bounded digest", response.ScopeDigest)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if response.Credential != "" || strings.Contains(string(encoded), "tenant-a") || strings.Contains(string(encoded), "project-a") || strings.Contains(string(encoded), "authorized_scope_count") {
		t.Fatalf("who_am_i leaked credential or raw scope: %s", encoded)
	}
}
