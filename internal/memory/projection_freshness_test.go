package memory

import (
	"testing"
	"time"
)

func TestEvaluateProjectionFreshnessAllowsMatchingFreshEvidence(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	evidence, err := EvaluateProjectionFreshness(scope, "wm-1", "wm-1", "policy-v1", "renderer-v1", time.Minute, time.Second, 5*time.Minute, 2*time.Second, true, scope)
	if err != nil {
		t.Fatalf("EvaluateProjectionFreshness() error = %v", err)
	}
	if evidence.Category != ProjectionFreshnessFresh || !evidence.Eligible || evidence.SLO != ProjectionSLOWithinBudget {
		t.Fatalf("evidence = %+v, want fresh eligible within budget", evidence)
	}
}

func TestEvaluateProjectionFreshnessFailsClosedForStaleForeignAndHiddenEvidence(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	foreign := Scope{Tenant: "other", Project: "p", Namespace: "n"}
	cases := []struct {
		name               string
		source, projection string
		visible            bool
		evidenceScope      Scope
		want               ProjectionFreshnessCategory
	}{
		{"stale", "wm-1", "wm-1", true, scope, ProjectionFreshnessStale},
		{"divergent", "wm-1", "wm-2", true, scope, ProjectionFreshnessDivergent},
		{"hidden", "wm-1", "wm-1", false, scope, ProjectionFreshnessHidden},
		{"foreign", "wm-1", "wm-1", true, foreign, ProjectionFreshnessForeign},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			age := time.Minute
			if tc.name == "stale" {
				age = time.Hour
			}
			evidence, err := EvaluateProjectionFreshness(scope, tc.source, tc.projection, "policy-v1", "renderer-v1", age, time.Second, 5*time.Minute, 2*time.Second, tc.visible, tc.evidenceScope)
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if evidence.Category != tc.want || evidence.Eligible {
				t.Fatalf("evidence = %+v, want category %s and ineligible", evidence, tc.want)
			}
		})
	}
}

func TestProjectionFreshnessEligibilityRejectsOverBudget(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	evidence, err := EvaluateProjectionFreshness(scope, "wm", "wm", "p", "r", time.Second, 5*time.Second, time.Minute, time.Second, true, scope)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if evidence.SLO != ProjectionSLOOverBudget || evidence.Eligible {
		t.Fatalf("evidence = %+v, want over-budget ineligible", evidence)
	}
}

func TestProjectionFreshnessRejectsPolicyOrRendererIdentityMismatch(t *testing.T) {
	scope := Scope{Tenant: "t", Project: "p", Namespace: "n"}
	evidence, err := EvaluateProjectionFreshnessWithIdentity(scope, "wm", "wm", "policy-old", "renderer-old", "policy-new", "renderer-new", time.Second, time.Second, time.Minute, time.Second, true, scope)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if evidence.Category != ProjectionFreshnessDivergent || evidence.Eligible || !evidence.RebuildRequired {
		t.Fatalf("evidence = %+v, want divergent ineligible rebuild-required", evidence)
	}
}
