package evaluation

import (
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestAuthorizeReportScopeRequiresExactScopeHash(t *testing.T) {
	requested := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	other := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-b"}
	if err := AuthorizeReportScope(requested, hashScope(requested)); err != nil {
		t.Fatalf("expected exact scope to authorize: %v", err)
	}
	if err := AuthorizeReportScope(requested, hashScope(other)); err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("expected foreign scope rejection, got %v", err)
	}
}

func TestRequireCompatibleIdentityRejectsWatermarkDrift(t *testing.T) {
	base := CompatibilityIdentity{FixtureVersion: "fixture-v1", PolicyVersion: "policy-v1", Strategy: "strategy-v1", Renderer: "renderer-v1", Provider: "provider-v1", SourceWatermark: "watermark-v1"}
	changed := base
	changed.SourceWatermark = "watermark-v2"
	if err := RequireCompatibleIdentity(base, changed); err == nil || !strings.Contains(err.Error(), "source watermark") {
		t.Fatalf("expected source watermark incompatibility, got %v", err)
	}
}
