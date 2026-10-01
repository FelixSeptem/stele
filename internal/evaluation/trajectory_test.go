package evaluation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestTrajectoryAggregateRedactsSensitiveFieldsAndUsesStableBuckets(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	identity := CompatibilityIdentity{
		FixtureVersion:  "fixture-v1",
		PolicyVersion:   "policy-v1",
		Strategy:        "flat-fusion-v1",
		Renderer:        "trajectory-v1",
		Provider:        "provider-profile-v1",
		SourceWatermark: "watermark-42",
	}
	input := TrajectoryInput{
		Scope:          scope,
		Identity:       identity,
		Channel:        "semantic",
		CandidateCount: 12,
		ExpansionCount: 2,
		Disposition:    "fallback",
		Fallback:       "provider_unavailable",
		Freshness:      "fresh",
		Budget:         3200,
		Latency:        1500 * time.Millisecond,
	}

	first, err := AggregateTrajectory(input)
	if err != nil {
		t.Fatalf("aggregate trajectory: %v", err)
	}
	second, err := AggregateTrajectory(input)
	if err != nil {
		t.Fatalf("aggregate trajectory repeat: %v", err)
	}
	if first != second {
		t.Fatalf("trajectory aggregation is not deterministic:\nfirst=%#v\nsecond=%#v", first, second)
	}
	payload, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal trajectory: %v", err)
	}
	encoded := string(payload)
	for _, forbidden := range []string{"tenant-a", "project-a", "namespace-a", "provider_unavailable", "query", "memory_id", "raw_score"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("trajectory payload leaked %q: %s", forbidden, encoded)
		}
	}
	if first.ScopeHash == "" || first.CandidateBucket != "11_50" || first.ExpansionBucket != "1_5" || first.LatencyBucket != "1_5s" {
		t.Fatalf("unexpected redacted trajectory buckets: %#v", first)
	}
}

func TestCompatibilityIdentityRejectsUnsafeLogicalIdentity(t *testing.T) {
	identity := CompatibilityIdentity{FixtureVersion: "fixture-v1", PolicyVersion: "policy-v1", Strategy: "strategy with spaces", Renderer: "renderer-v1", Provider: "provider-v1", SourceWatermark: "watermark-v1"}
	if err := identity.Validate(); err == nil {
		t.Fatal("expected unsafe identity to be rejected")
	}
}

func TestTrajectoryRequiresExactScope(t *testing.T) {
	_, err := AggregateTrajectory(TrajectoryInput{Identity: CompatibilityIdentity{FixtureVersion: "fixture-v1", PolicyVersion: "policy-v1", Strategy: "strategy-v1", Renderer: "renderer-v1", Provider: "provider-v1", SourceWatermark: "watermark-v1"}})
	if err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("expected exact scope validation error, got %v", err)
	}
}
