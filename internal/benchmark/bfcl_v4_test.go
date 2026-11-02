package benchmark

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestLoadBFCLV4SourceRejectsMissingOrDriftedLock(t *testing.T) {
	bundle := validBFCLV4Bundle()
	bundle.Manifest.SourceChecksum = ""
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBFCLV4Source(encoded); StatusOf(err) != StatusInvalidManifest {
		t.Fatalf("missing source lock status = %q, err = %v", StatusOf(err), err)
	}

	bundle = validBFCLV4Bundle()
	bundle.Manifest.SourceChecksum = strings.Repeat("0", 64)
	encoded, err = json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBFCLV4Source(encoded); StatusOf(err) != StatusChecksumMismatch {
		t.Fatalf("drifted source lock status = %q, err = %v", StatusOf(err), err)
	}
}

func TestLoadBFCLV4SourceRejectsTrailingJSONAmbiguousPrerequisitesAndUnsafeManifestLabels(t *testing.T) {
	bundle := validBFCLV4Bundle()
	lockBFCLV4Bundle(t, &bundle)
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBFCLV4Source(append(encoded, []byte("\n{}")...)); StatusOf(err) != StatusInvalidManifest {
		t.Fatalf("trailing JSON status = %q, err = %v", StatusOf(err), err)
	}

	bundle = validBFCLV4Bundle()
	bundle.Prerequisites = append(bundle.Prerequisites, BFCLV4Prerequisite{ID: "snapshot-a", SnapshotSHA256: strings.Repeat("3", 64)})
	lockBFCLV4Bundle(t, &bundle)
	encoded, err = json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBFCLV4Source(encoded); StatusOf(err) != StatusInvalidManifest {
		t.Fatalf("ambiguous prerequisite status = %q, err = %v", StatusOf(err), err)
	}

	bundle = validBFCLV4Bundle()
	bundle.Manifest.UpstreamRevision = "tenant=bench;token=secret"
	lockBFCLV4Bundle(t, &bundle)
	encoded, err = json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBFCLV4Source(encoded); StatusOf(err) != StatusInvalidManifest {
		t.Fatalf("unsafe manifest label status = %q, err = %v", StatusOf(err), err)
	}
}

func TestNormalizeBFCLV4SourcePreservesSupportedOperationOrderAndContractFields(t *testing.T) {
	bundle := validBFCLV4Bundle()
	bundle.Cases = []BFCLV4SourceCase{
		{ID: "case-kv", Operation: "get", Arguments: map[string]any{}, Expected: map[string]any{}, Scope: bfclV4Scope(), SessionID: "s-kv", PrerequisiteID: "snapshot-a"},
		{ID: "case-summary", Operation: "summarize", Arguments: map[string]any{}, Expected: map[string]any{}, Scope: bfclV4Scope(), SessionID: "s-summary", PrerequisiteID: "snapshot-a"},
		{ID: "case-vector", Operation: "vector_search", Arguments: map[string]any{}, Expected: map[string]any{}, Scope: bfclV4Scope(), SessionID: "s-vector", Lifecycle: memory.MemoryStateForgotten, MustRefuse: true, PrerequisiteID: "snapshot-a"},
	}
	lockBFCLV4Bundle(t, &bundle)

	cases, compatibility, scopeMismatches := NormalizeBFCLV4Source(bundle, bfclV4Scope())
	if len(cases) != 3 || len(compatibility) != 3 {
		t.Fatalf("normalized cases/outcomes = %d/%d, want 3/3", len(cases), len(compatibility))
	}
	if scopeMismatches != 0 {
		t.Fatalf("scope mismatches = %d, want 0", scopeMismatches)
	}
	if cases[0].ID != "case-kv" || cases[1].Operation.Name != "summarize" || cases[2].Operation.Name != "vector_search" {
		t.Fatalf("operation order = %#v", cases)
	}
	if cases[2].Operation.Scope != bfclV4Scope() || !cases[2].Operation.MustRefuse || cases[2].Operation.Lifecycle != memory.MemoryStateForgotten {
		t.Fatalf("contract invariants were not preserved: %#v", cases[2])
	}
	for _, outcome := range compatibility {
		if outcome.State != BFCLV4CompatibilityCompatible {
			t.Fatalf("expected compatible source outcome, got %#v", outcome)
		}
	}
}

func TestRunBFCLV4ConformanceSeparatesUnsupportedIncompleteAndSafetyFailures(t *testing.T) {
	bundle := validBFCLV4Bundle()
	bundle.Cases = []BFCLV4SourceCase{
		{ID: "supported", Operation: "get", Arguments: map[string]any{}, Expected: map[string]any{}, Scope: bfclV4Scope(), PrerequisiteID: "snapshot-a"},
		{ID: "unsupported", Operation: "erase_everything", Arguments: map[string]any{}, Expected: map[string]any{}, Scope: bfclV4Scope(), PrerequisiteID: "snapshot-a"},
		{ID: "incomplete", Operation: "put", Arguments: nil, Expected: map[string]any{}, Scope: bfclV4Scope(), PrerequisiteID: "missing-snapshot"},
		{ID: "foreign", Operation: "search", Arguments: map[string]any{}, Expected: map[string]any{}, Scope: memory.Scope{Tenant: "foreign", Project: "bfcl", Namespace: "memory"}, PrerequisiteID: "snapshot-a"},
		{ID: "hidden", Operation: "get", Arguments: map[string]any{}, Expected: map[string]any{}, Scope: bfclV4Scope(), Lifecycle: memory.MemoryStateSuppressed, PrerequisiteID: "snapshot-a"},
	}
	lockBFCLV4Bundle(t, &bundle)

	report, err := RunBFCLV4Conformance(bundle, bfclV4Scope())
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != StatusQualityGateFailed || report.Coverage.Supported != 2 || report.Coverage.Unsupported != 1 || report.Coverage.Incomplete != 2 {
		t.Fatalf("source coverage/status = %#v", report)
	}
	if report.Contract.ScopeSafetyFailures != 1 || report.Contract.LifecycleSafetyFailures != 1 {
		t.Fatalf("contract safety = %#v", report.Contract)
	}
	if report.Passing || report.Source.SyntheticFixture || !report.CleanupVerified {
		t.Fatalf("report passing/synthetic/cleanup = %#v", report)
	}
	encoded, err := MarshalBFCLV4ConformanceReport(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"\"scope\"", "foreign", "memory-id"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("redacted report exposes %q: %s", forbidden, encoded)
		}
	}
}

func TestRunBFCLV4ConformanceRequiresTrustedExactScopeAndAcceptsCorrectHiddenRefusal(t *testing.T) {
	bundle := validBFCLV4Bundle()
	bundle.Cases = []BFCLV4SourceCase{{ID: "wrong-scope", Operation: "get", Arguments: map[string]any{}, Expected: map[string]any{}, Scope: memory.Scope{Tenant: "production", Project: "bfcl", Namespace: "memory"}, PrerequisiteID: "snapshot-a"}}
	lockBFCLV4Bundle(t, &bundle)
	report, err := RunBFCLV4Conformance(bundle, bfclV4Scope())
	if err != nil {
		t.Fatal(err)
	}
	if report.Passing || report.Coverage.Incomplete != 1 || report.Contract.ScopeSafetyFailures != 1 {
		t.Fatalf("scope mismatch report = %#v", report)
	}
	encoded, err := MarshalBFCLV4ConformanceReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "production") || strings.Contains(string(encoded), `"scope"`) {
		t.Fatalf("scope mismatch report leaks source scope: %s", encoded)
	}

	bundle = validBFCLV4Bundle()
	bundle.Cases = []BFCLV4SourceCase{{ID: "hidden-refusal", Operation: "get", Arguments: map[string]any{}, Expected: map[string]any{}, Scope: bfclV4Scope(), Lifecycle: memory.MemoryStateSuppressed, MustRefuse: true, PrerequisiteID: "snapshot-a"}}
	lockBFCLV4Bundle(t, &bundle)
	report, err = RunBFCLV4Conformance(bundle, bfclV4Scope())
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passing || report.Status != StatusSuccess || report.Contract.LifecycleSafetyFailures != 0 || report.Contract.RefusalAccuracy != 1 {
		t.Fatalf("hidden refusal report = %#v", report)
	}
}

func validBFCLV4Bundle() BFCLV4SourceBundle {
	return BFCLV4SourceBundle{
		Manifest: BFCLV4SourceManifest{
			SchemaVersion:     BFCLV4SourceSchemaVersion,
			Dataset:           "bfcl-v4-memory",
			Version:           "v4.0",
			UpstreamRevision:  "abcdef1",
			SourceSHA256:      strings.Repeat("1", 64),
			ConversionVersion: "bfcl-v4-provider-v1",
			Subset:            "memory_kv",
		},
		Prerequisites: []BFCLV4Prerequisite{{ID: "snapshot-a", SnapshotSHA256: strings.Repeat("2", 64)}},
		Cases:         []BFCLV4SourceCase{{ID: "case-a", Operation: "get", Arguments: map[string]any{}, Expected: map[string]any{}, Scope: bfclV4Scope(), PrerequisiteID: "snapshot-a"}},
	}
}

func lockBFCLV4Bundle(t *testing.T, bundle *BFCLV4SourceBundle) {
	t.Helper()
	checksum, err := BFCLV4SourceChecksum(*bundle)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Manifest.SourceChecksum = checksum
}

func bfclV4Scope() memory.Scope {
	return memory.Scope{Tenant: "bench", Project: "bfcl", Namespace: "memory"}
}
