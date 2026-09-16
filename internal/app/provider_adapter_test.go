package app

import (
	"context"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

func TestProviderRuntimeStatusReadIsBounded(t *testing.T) {
	result, err := (providerRuntimeReader{}).Read(context.Background(), memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, provider.ReadKindStatus, provider.ReadRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(result.(map[string]any)["status"].(string)), "postgres") {
		t.Fatalf("status leaked internals: %+v", result)
	}
}
