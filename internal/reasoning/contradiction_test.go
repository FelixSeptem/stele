package reasoning

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func contradictionScope() memory.Scope {
	return memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
}

func contradictionValidity(from time.Time, to *time.Time, id string) memory.TemporalValidity {
	return memory.TemporalValidity{
		TemporalFactID: id,
		IngestedAt:     from,
		ValidFrom:      from,
		ValidTo:        to,
		ValiditySource: memory.TemporalValiditySourceExplicit,
	}
}

func TestNormalizeContradictionKeyIsStableAndDoesNotExposeInputs(t *testing.T) {
	left, err := NormalizeContradictionKey("  User-42 ", "timezone", "single_value")
	if err != nil {
		t.Fatalf("NormalizeContradictionKey() error = %v", err)
	}
	right, err := NormalizeContradictionKey("user-42", "timezone", "single_value")
	if err != nil {
		t.Fatalf("NormalizeContradictionKey() error = %v", err)
	}
	if left != right {
		t.Fatalf("normalized key changed with whitespace: %q != %q", left, right)
	}
	if left == "user-42" || left == "timezone" || len(left) < len("contradiction-key:")+16 {
		t.Fatalf("key is not a bounded digest: %q", left)
	}
}

func TestCompareContradictionTemporalValidityDistinguishesOverlapCoexistenceAndUnknown(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := base.Add(24 * time.Hour)
	overlapEnd := base.Add(48 * time.Hour)
	cases := []struct {
		name  string
		left  memory.TemporalValidity
		right memory.TemporalValidity
		want  ContradictionTemporalDisposition
	}{
		{name: "overlap", left: contradictionValidity(base, &end, "a"), right: contradictionValidity(base.Add(12*time.Hour), &overlapEnd, "b"), want: ContradictionTemporalOverlap},
		{name: "coexistence", left: contradictionValidity(base, &end, "a"), right: contradictionValidity(end, nil, "b"), want: ContradictionTemporalCoexistence},
		{name: "unknown", left: memory.TemporalValidity{}, right: contradictionValidity(base, nil, "b"), want: ContradictionTemporalUnresolved},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, _, err := CompareContradictionTemporalValidity(tc.left, tc.right)
			if err != nil {
				t.Fatalf("CompareContradictionTemporalValidity() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("disposition = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDetectContradictionsRequiresExactScopeAndBoundedEvidence(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := base.Add(24 * time.Hour)
	key, err := NormalizeContradictionKey("user-42", "timezone", "single_value")
	if err != nil {
		t.Fatal(err)
	}
	facts := []ContradictionFact{
		{Scope: contradictionScope(), ID: "memory-a", Version: 1, Key: key, ValueDigest: "value-a", Validity: contradictionValidity(base, &end, "fact-a"), Evidence: evidenceRef("memory-a")},
		{Scope: contradictionScope(), ID: "memory-b", Version: 1, Key: key, ValueDigest: "value-b", Validity: contradictionValidity(base.Add(12*time.Hour), nil, "fact-b"), Evidence: evidenceRef("memory-b")},
	}
	result, err := DetectContradictions(facts, ContradictionDetectionPolicy{Scope: contradictionScope(), MaxPairs: 4, SourceWatermark: "watermark-1"})
	if err != nil {
		t.Fatalf("DetectContradictions() error = %v", err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].TemporalDisposition != ContradictionTemporalOverlap {
		t.Fatalf("candidates = %+v", result.Candidates)
	}
	if result.Candidates[0].ReplayID == "" || result.Candidates[0].EvidenceDigest == "" {
		t.Fatalf("candidate lacks deterministic evidence identity: %+v", result.Candidates[0])
	}

	facts[1].Scope.Namespace = "foreign"
	if _, err := DetectContradictions(facts, ContradictionDetectionPolicy{Scope: contradictionScope(), MaxPairs: 4}); err == nil {
		t.Fatal("foreign scope fact was accepted")
	}
}

func TestDetectContradictionsRecordsUnresolvedTemporalForUnsetValidity(t *testing.T) {
	key, err := NormalizeContradictionKey("user-42", "timezone", "single_value")
	if err != nil {
		t.Fatal(err)
	}
	facts := []ContradictionFact{
		{Scope: contradictionScope(), ID: "a", Version: 1, Key: key, ValueDigest: "a", Evidence: evidenceRef("a")},
		{Scope: contradictionScope(), ID: "b", Version: 1, Key: key, ValueDigest: "b", Evidence: evidenceRef("b")},
	}
	result, err := DetectContradictions(facts, ContradictionDetectionPolicy{Scope: contradictionScope(), MaxPairs: 2, SourceWatermark: "w1"})
	if err != nil {
		t.Fatalf("DetectContradictions() error = %v", err)
	}
	if result.Disposition[ContradictionTemporalUnresolved] != 1 || len(result.Candidates) != 0 {
		t.Fatalf("result = %+v, want unresolved without candidate", result)
	}
}

func TestClassifyContradictionFailureUsesStableNonAuthoritativeDispositions(t *testing.T) {
	cases := []struct {
		name string
		want ContradictionFailureDisposition
		args [5]bool
	}{
		{"watermark", ContradictionFailureStaleWatermark, [5]bool{false, true, true, true, true}},
		{"schema", ContradictionFailureSchemaMismatch, [5]bool{true, true, false, true, true}},
		{"provenance", ContradictionFailureIncompleteSource, [5]bool{true, false, true, true, true}},
		{"budget", ContradictionFailureBudgetExhausted, [5]bool{true, true, true, false, true}},
		{"timeout", ContradictionFailureTimeout, [5]bool{true, true, true, true, true}},
		{"provider", ContradictionFailureUnsafeProvider, [5]bool{true, true, true, true, false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.name == "timeout" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got := ClassifyContradictionFailure(ctx, tc.args[0], tc.args[1], tc.args[2], tc.args[3], tc.args[4])
			if got != tc.want {
				t.Fatalf("disposition = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyContradictionRejectsUnsafeProviderOutput(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := base.Add(24 * time.Hour)
	key, err := NormalizeContradictionKey("user-42", "timezone", "single_value")
	if err != nil {
		t.Fatal(err)
	}
	detected, err := DetectContradictions([]ContradictionFact{
		{Scope: contradictionScope(), ID: "a", Version: 1, Key: key, ValueDigest: "a", Validity: contradictionValidity(base, &end, "a"), Evidence: evidenceRef("a")},
		{Scope: contradictionScope(), ID: "b", Version: 1, Key: key, ValueDigest: "b", Validity: contradictionValidity(base, nil, "b"), Evidence: evidenceRef("b")},
	}, ContradictionDetectionPolicy{Scope: contradictionScope(), MaxPairs: 2, SourceWatermark: "w1"})
	if err != nil || len(detected.Candidates) != 1 {
		t.Fatalf("detect = %+v, err = %v", detected, err)
	}
	result := ClassifyContradiction(context.Background(), unsafeContradictionClassifier{}, detected.Candidates[0])
	if result.Disposition != ContradictionClassificationQuarantined {
		t.Fatalf("disposition = %q, want quarantined", result.Disposition)
	}
}

func TestContradictionCandidateConvertsToGovernedEnvelope(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := base.Add(24 * time.Hour)
	key, err := NormalizeContradictionKey("user-42", "timezone", "single_value")
	if err != nil {
		t.Fatal(err)
	}
	detected, err := DetectContradictions([]ContradictionFact{
		{Scope: contradictionScope(), ID: "a", Version: 1, Key: key, ValueDigest: "a", Validity: contradictionValidity(base, &end, "a"), Evidence: evidenceRef("a")},
		{Scope: contradictionScope(), ID: "b", Version: 1, Key: key, ValueDigest: "b", Validity: contradictionValidity(base.Add(12*time.Hour), nil, "b"), Evidence: evidenceRef("b")},
	}, ContradictionDetectionPolicy{Scope: contradictionScope(), MaxPairs: 2, SourceWatermark: "w1"})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := detected.Candidates[0].ToInsightCandidate("provider-v1", "schema-v1", "policy-v1", ModeShadow, "scope-proof")
	if err != nil {
		t.Fatalf("ToInsightCandidate() error = %v", err)
	}
	request := InsightDerivationRequest{Scope: contradictionScope(), InsightType: memory.DerivedInsightTypeContradiction, Mode: ModeShadow, Evidence: candidate.Evidence, SourceWatermark: "w1", ScopeProof: "scope-proof", LifecycleVisibility: "active_only", RedactionPolicy: "references_only", ProviderVersion: "provider-v1", SchemaVersion: "schema-v1", PolicyVersion: "policy-v1", InputDigest: detected.Candidates[0].ReplayID, Limits: DefaultLimits(), Now: candidate.CreatedAt}
	if err := ValidateInsightCandidate(request, candidate); err != nil {
		t.Fatalf("ValidateInsightCandidate() error = %v", err)
	}
}

type unsafeContradictionClassifier struct{}

func (unsafeContradictionClassifier) ClassifyContradiction(context.Context, ContradictionCandidate) (ContradictionClassification, error) {
	return ContradictionClassification{Summary: "unsafe", Uncertainty: 0.2, DirectActivation: true}, nil
}

func evidenceRef(id string) memory.DerivedInsightEvidenceRef {
	return memory.DerivedInsightEvidenceRef{Kind: memory.DerivedInsightEvidenceKindCanonicalMemory, ID: id, Relation: memory.DerivedInsightEvidenceRelationSupports}
}
