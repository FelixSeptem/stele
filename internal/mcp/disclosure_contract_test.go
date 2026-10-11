package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/auth"
	protocolmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAllMCPDataToolsDenyPublicBeforeScopeOrDependencyLookup(t *testing.T) {
	adapter := NewAdapter(AdapterOptions{Enabled: true, Limits: Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 10, MaxIDs: 10}})
	session := connectMCPContractSession(t, adapter, auth.Principal{ID: "private-principal-marker", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}, "private-binding-marker")
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{ToolWhoAmI, map[string]any{}},
		{ToolSearch, map[string]any{"query": "private-query-marker"}},
		{ToolContext, map[string]any{"query": "private-query-marker"}},
		{ToolBrowse, map[string]any{}},
		{ToolRemember, map[string]any{"content": "private-content-marker", "reason": "fixture", "idempotency_key": "key"}},
		{ToolForget, map[string]any{"memory_id": "private-memory-marker", "reason": "fixture", "idempotency_key": "key"}},
		{ToolForgetPreview, map[string]any{"query": "private-query-marker"}},
		{ToolForgetApply, map[string]any{"preview_id": "private-preview-marker", "memory_ids": []string{"private-memory-marker"}, "reason": "fixture", "idempotency_key": "key"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &protocolmcp.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError || (!strings.Contains(string(data), "disclosure_denied") && !strings.Contains(string(data), "lifecycle_forbidden")) || strings.Contains(string(data), "marker") {
				t.Fatalf("public tool reached scope/dependency lookup or disclosed identity: %s", data)
			}
		})
	}
}
