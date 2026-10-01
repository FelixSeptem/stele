package evaluation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestEvaluateIntegritySeparatesActionSuccessFromHardIntegrityFailure(t *testing.T) {
	report, err := EvaluateIntegrity(IntegrityInput{
		ID:            "integrity-1",
		Scope:         memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Identity:      CompatibilityIdentity{FixtureVersion: "fixture-v1", PolicyVersion: "policy-v1", Strategy: "strategy-v1", Renderer: "renderer-v1", Provider: "provider-v1", SourceWatermark: "watermark-v1"},
		Action:        "consolidation",
		ActionSuccess: true,
		Findings:      map[FindingCategory]int{"missing": 1, "expected-recall": 3},
		CreatedAt:     time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("evaluate integrity: %v", err)
	}
	if !report.ActionSuccess || report.IntegritySuccess || report.Verdict != VerdictRejected {
		t.Fatalf("action and integrity verdicts were not separated: %#v", report)
	}
	if !report.HasHardFailure(FindingMissing) {
		t.Fatalf("missing evidence must be a hard finding: %#v", report)
	}
	if strings.Contains(report.Redacted().ScopeHash, "tenant-a") {
		t.Fatalf("scope hash leaked tenant: %#v", report.Redacted())
	}
	payload, err := json.Marshal(report.Redacted())
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{report.ID, report.Fingerprint} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("redacted report leaked internal value %q: %s", forbidden, payload)
		}
	}
}

func TestEvaluateIntegrityRejectsForeignLifecycleFinding(t *testing.T) {
	for _, finding := range []FindingCategory{FindingForeignScope, FindingHiddenLifecycle, FindingStaleWatermark, FindingNondeterministic, FindingRollbackIncomplete} {
		t.Run(string(finding), func(t *testing.T) {
			report, err := EvaluateIntegrity(IntegrityInput{
				ID:            "integrity-" + string(finding),
				Scope:         memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
				Identity:      CompatibilityIdentity{FixtureVersion: "fixture-v1", PolicyVersion: "policy-v1", Strategy: "strategy-v1", Renderer: "renderer-v1", Provider: "provider-v1", SourceWatermark: "watermark-v1"},
				Action:        "projection",
				ActionSuccess: true,
				Findings:      map[FindingCategory]int{finding: 1},
			})
			if err != nil {
				t.Fatalf("evaluate integrity: %v", err)
			}
			if report.Verdict != VerdictRejected || report.IntegritySuccess {
				t.Fatalf("hard finding must reject report: %#v", report)
			}
		})
	}
}

func TestReplayIntegrityIsDeterministicAndNonAuthoritative(t *testing.T) {
	input := IntegrityInput{
		ID: "integrity-replay", Scope: memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Identity: CompatibilityIdentity{FixtureVersion: "fixture-v1", PolicyVersion: "policy-v1", Strategy: "strategy-v1", Renderer: "renderer-v1", Provider: "provider-v1", SourceWatermark: "watermark-v1"},
		Action:   "reflection", ActionSuccess: true, Findings: map[FindingCategory]int{"expected-recall": 2},
	}
	first, err := EvaluateIntegrity(input)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	second, err := ReplayIntegrity(first, input)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if first.Fingerprint != second.Fingerprint || second.Authoritative {
		t.Fatalf("replay must be deterministic and non-authoritative: first=%#v second=%#v", first, second)
	}
}
