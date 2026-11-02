package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

const BFCLV4SourceSchemaVersion = "bfcl-v4-memory-source-v1"

var (
	bfclV4VersionPattern    = regexp.MustCompile(`^v[0-9]+(?:[._-][0-9A-Za-z]+){0,7}$`)
	bfclV4RevisionPattern   = regexp.MustCompile(`^[a-fA-F0-9]{7,64}$`)
	bfclV4ConversionPattern = regexp.MustCompile(`^bfcl-v4-provider-v[1-9][0-9]*$`)
)

type BFCLV4SourceManifest struct {
	SchemaVersion     string `json:"schema_version"`
	Dataset           string `json:"dataset"`
	Version           string `json:"version"`
	UpstreamRevision  string `json:"upstream_revision"`
	SourceSHA256      string `json:"source_sha256"`
	SourceChecksum    string `json:"source_checksum"`
	ConversionVersion string `json:"conversion_version"`
	Subset            string `json:"subset"`
	SyntheticFixture  bool   `json:"synthetic_fixture,omitempty"`
}

type BFCLV4Prerequisite struct {
	ID             string `json:"id"`
	SnapshotSHA256 string `json:"snapshot_sha256"`
}

type BFCLV4SourceCase struct {
	ID             string             `json:"id"`
	Operation      string             `json:"operation"`
	Arguments      map[string]any     `json:"arguments"`
	Expected       map[string]any     `json:"expected"`
	Scope          memory.Scope       `json:"scope"`
	SessionID      string             `json:"session_id,omitempty"`
	Lifecycle      memory.MemoryState `json:"lifecycle,omitempty"`
	MustRefuse     bool               `json:"must_refuse,omitempty"`
	PrerequisiteID string             `json:"prerequisite_id"`
}

type BFCLV4SourceBundle struct {
	Manifest      BFCLV4SourceManifest `json:"manifest"`
	Prerequisites []BFCLV4Prerequisite `json:"prerequisites"`
	Cases         []BFCLV4SourceCase   `json:"cases"`
}

type BFCLV4Compatibility string

const (
	BFCLV4CompatibilityCompatible  BFCLV4Compatibility = "compatible"
	BFCLV4CompatibilityUnsupported BFCLV4Compatibility = "unsupported"
	BFCLV4CompatibilityIncomplete  BFCLV4Compatibility = "incomplete"
)

type BFCLV4CompatibilityOutcome struct {
	CaseID string              `json:"case_id"`
	Subset string              `json:"subset"`
	State  BFCLV4Compatibility `json:"state"`
	Reason string              `json:"reason,omitempty"`
}

type BFCLV4Coverage struct {
	Total       int `json:"total"`
	Supported   int `json:"supported"`
	Unsupported int `json:"unsupported"`
	Incomplete  int `json:"incomplete"`
}

type BFCLV4SourceProvenance struct {
	Dataset            string `json:"dataset"`
	Version            string `json:"version"`
	Subset             string `json:"subset"`
	UpstreamRevision   string `json:"upstream_revision"`
	SourceSHA256       string `json:"source_sha256"`
	SourceChecksum     string `json:"source_checksum"`
	ConversionVersion  string `json:"conversion_version"`
	PrerequisiteDigest string `json:"prerequisite_digest"`
	SyntheticFixture   bool   `json:"synthetic_fixture"`
}

type BFCLV4ContractSummary struct {
	OperationAccuracy       float64 `json:"operation_accuracy"`
	MalformedRate           float64 `json:"malformed_call_rate"`
	RefusalAccuracy         float64 `json:"refusal_accuracy"`
	ScopeSafetyFailures     int     `json:"scope_safety_failures"`
	LifecycleSafetyFailures int     `json:"lifecycle_safety_failures"`
	Status                  Status  `json:"status"`
}

type BFCLV4ConformanceReport struct {
	SchemaVersion   string                 `json:"schema_version"`
	Family          BenchmarkFamily        `json:"family"`
	Dataset         string                 `json:"dataset"`
	Source          BFCLV4SourceProvenance `json:"source"`
	Coverage        BFCLV4Coverage         `json:"coverage"`
	Contract        BFCLV4ContractSummary  `json:"contract"`
	Status          Status                 `json:"status"`
	Passing         bool                   `json:"passing"`
	CleanupVerified bool                   `json:"cleanup_verified"`
}

func (m BFCLV4SourceManifest) validate() error {
	if m.SchemaVersion != BFCLV4SourceSchemaVersion {
		return fmt.Errorf("unsupported BFCL V4 source schema %q", m.SchemaVersion)
	}
	for name, value := range map[string]string{
		"dataset": m.Dataset, "version": m.Version, "upstream_revision": m.UpstreamRevision,
		"source_sha256": m.SourceSHA256, "source_checksum": m.SourceChecksum,
		"conversion_version": m.ConversionVersion, "subset": m.Subset,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("BFCL V4 source manifest %s is required", name)
		}
	}
	if m.Dataset != "bfcl-v4-memory" {
		return fmt.Errorf("unsupported BFCL V4 dataset %q", m.Dataset)
	}
	if !sha256Pattern.MatchString(m.SourceSHA256) || !sha256Pattern.MatchString(m.SourceChecksum) {
		return fmt.Errorf("BFCL V4 source checksums must be 64-character hexadecimal digests")
	}
	if !knownBFCLV4Subset(m.Subset) {
		return fmt.Errorf("unsupported BFCL V4 memory subset %q", m.Subset)
	}
	if !bfclV4VersionPattern.MatchString(m.Version) {
		return fmt.Errorf("BFCL V4 source version must use the bounded vN format")
	}
	if !bfclV4RevisionPattern.MatchString(m.UpstreamRevision) {
		return fmt.Errorf("BFCL V4 upstream revision must be a bounded hexadecimal revision")
	}
	if !bfclV4ConversionPattern.MatchString(m.ConversionVersion) {
		return fmt.Errorf("BFCL V4 conversion version is unsupported")
	}
	return nil
}

func (bundle BFCLV4SourceBundle) validate() error {
	if err := bundle.Manifest.validate(); err != nil {
		return err
	}
	if len(bundle.Cases) > 0 && len(bundle.Prerequisites) == 0 {
		return fmt.Errorf("BFCL V4 source cases require prerequisite snapshots")
	}
	seen := make(map[string]struct{}, len(bundle.Prerequisites))
	for _, prerequisite := range bundle.Prerequisites {
		if strings.TrimSpace(prerequisite.ID) == "" || !sha256Pattern.MatchString(prerequisite.SnapshotSHA256) {
			return fmt.Errorf("BFCL V4 prerequisite snapshot is malformed")
		}
		if _, exists := seen[prerequisite.ID]; exists {
			return fmt.Errorf("BFCL V4 prerequisite snapshot %q is duplicated", prerequisite.ID)
		}
		seen[prerequisite.ID] = struct{}{}
	}
	return nil
}

func knownBFCLV4Subset(subset string) bool {
	switch subset {
	case "memory_kv", "memory_rec_sum", "memory_vector":
		return true
	default:
		return false
	}
}

func BFCLV4SourceChecksum(bundle BFCLV4SourceBundle) (string, error) {
	bundle.Manifest.SourceChecksum = ""
	encoded, err := json.Marshal(bundle)
	if err != nil {
		return "", fmt.Errorf("marshal BFCL V4 source lock: %w", err)
	}
	return checksumCanonicalText(encoded), nil
}

func LoadBFCLV4Source(source []byte) (BFCLV4SourceBundle, error) {
	bundle, err := DecodeBFCLV4Source(source)
	if err != nil {
		return BFCLV4SourceBundle{}, err
	}
	if err := bundle.validate(); err != nil {
		return BFCLV4SourceBundle{}, &StatusError{Status: StatusInvalidManifest, Message: "validate BFCL V4 source", Cause: err}
	}
	checksum, err := BFCLV4SourceChecksum(bundle)
	if err != nil {
		return BFCLV4SourceBundle{}, &StatusError{Status: StatusInvalidManifest, Message: "lock BFCL V4 source", Cause: err}
	}
	if !strings.EqualFold(checksum, bundle.Manifest.SourceChecksum) {
		return BFCLV4SourceBundle{}, &StatusError{Status: StatusChecksumMismatch, Message: "BFCL V4 source checksum does not match its lock"}
	}
	return bundle, nil
}

// DecodeBFCLV4Source reads the bounded local envelope without accepting it as
// a valid conformance input. Call LoadBFCLV4Source before replaying a source.
func DecodeBFCLV4Source(source []byte) (BFCLV4SourceBundle, error) {
	if len(bytes.TrimSpace(source)) == 0 {
		return BFCLV4SourceBundle{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "BFCL V4 source is missing"}
	}
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.DisallowUnknownFields()
	var bundle BFCLV4SourceBundle
	if err := decoder.Decode(&bundle); err != nil {
		return BFCLV4SourceBundle{}, &StatusError{Status: StatusInvalidManifest, Message: "decode BFCL V4 source", Cause: err}
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return BFCLV4SourceBundle{}, &StatusError{Status: StatusInvalidManifest, Message: "decode BFCL V4 source", Cause: err}
	}
	return bundle, nil
}

func NormalizeBFCLV4Source(bundle BFCLV4SourceBundle, allowedScope memory.Scope) ([]ContractCase, []BFCLV4CompatibilityOutcome, int) {
	prerequisites := make(map[string]struct{}, len(bundle.Prerequisites))
	for _, prerequisite := range bundle.Prerequisites {
		prerequisites[prerequisite.ID] = struct{}{}
	}
	cases := make([]ContractCase, 0, len(bundle.Cases))
	outcomes := make([]BFCLV4CompatibilityOutcome, 0, len(bundle.Cases))
	scopeMismatches := 0
	for _, sourceCase := range bundle.Cases {
		outcome := BFCLV4CompatibilityOutcome{CaseID: sourceCase.ID, Subset: bundle.Manifest.Subset}
		if strings.TrimSpace(sourceCase.ID) == "" || sourceCase.Arguments == nil || sourceCase.Expected == nil || strings.TrimSpace(sourceCase.PrerequisiteID) == "" {
			outcome.State, outcome.Reason = BFCLV4CompatibilityIncomplete, "missing required source case fields"
			outcomes = append(outcomes, outcome)
			continue
		}
		if sourceCase.Scope != allowedScope {
			outcome.State, outcome.Reason = BFCLV4CompatibilityIncomplete, "source scope does not match the declared conformance scope"
			outcomes = append(outcomes, outcome)
			scopeMismatches++
			continue
		}
		if _, ok := prerequisites[sourceCase.PrerequisiteID]; !ok {
			outcome.State, outcome.Reason = BFCLV4CompatibilityIncomplete, "declared prerequisite snapshot is unavailable"
			outcomes = append(outcomes, outcome)
			continue
		}
		if !knownOperation(sourceCase.Operation) {
			outcome.State, outcome.Reason = BFCLV4CompatibilityUnsupported, "source operation is unsupported"
			outcomes = append(outcomes, outcome)
			continue
		}
		outcome.State = BFCLV4CompatibilityCompatible
		outcomes = append(outcomes, outcome)
		cases = append(cases, ContractCase{
			ID:     sourceCase.ID,
			Subset: bundle.Manifest.Subset,
			Operation: ContractOperation{
				Name:       sourceCase.Operation,
				Arguments:  sourceCase.Arguments,
				Expected:   sourceCase.Expected,
				Scope:      sourceCase.Scope,
				SessionID:  sourceCase.SessionID,
				Lifecycle:  sourceCase.Lifecycle,
				MustRefuse: sourceCase.MustRefuse,
			},
		})
	}
	return cases, outcomes, scopeMismatches
}

func RunBFCLV4Conformance(bundle BFCLV4SourceBundle, allowedScope memory.Scope) (BFCLV4ConformanceReport, error) {
	if err := allowedScope.Validate(); err != nil {
		return BFCLV4ConformanceReport{}, &StatusError{Status: StatusInvalidManifest, Message: "validate BFCL V4 conformance scope", Cause: err}
	}
	if err := bundle.validate(); err != nil {
		return BFCLV4ConformanceReport{}, &StatusError{Status: StatusInvalidManifest, Message: "validate BFCL V4 source", Cause: err}
	}
	checksum, err := BFCLV4SourceChecksum(bundle)
	if err != nil {
		return BFCLV4ConformanceReport{}, &StatusError{Status: StatusInvalidManifest, Message: "lock BFCL V4 source", Cause: err}
	}
	if !strings.EqualFold(checksum, bundle.Manifest.SourceChecksum) {
		return BFCLV4ConformanceReport{}, &StatusError{Status: StatusChecksumMismatch, Message: "BFCL V4 source checksum does not match its lock"}
	}
	cases, compatibility, scopeMismatches := NormalizeBFCLV4Source(bundle, allowedScope)
	coverage := BFCLV4Coverage{Total: len(compatibility)}
	for _, outcome := range compatibility {
		switch outcome.State {
		case BFCLV4CompatibilityCompatible:
			coverage.Supported++
		case BFCLV4CompatibilityUnsupported:
			coverage.Unsupported++
		case BFCLV4CompatibilityIncomplete:
			coverage.Incomplete++
		}
	}
	contract := ReplayContract(cases)
	if coverage.Unsupported > 0 || coverage.Incomplete > 0 || scopeMismatches > 0 || contract.Status != StatusSuccess {
		contract.Status = StatusQualityGateFailed
	}
	prerequisiteBytes, err := json.Marshal(bundle.Prerequisites)
	if err != nil {
		return BFCLV4ConformanceReport{}, fmt.Errorf("marshal BFCL V4 prerequisite digest: %w", err)
	}
	report := BFCLV4ConformanceReport{
		SchemaVersion: SchemaVersion,
		Family:        FamilyProviderContract,
		Dataset:       bundle.Manifest.Dataset,
		Source: BFCLV4SourceProvenance{
			Dataset:            bundle.Manifest.Dataset,
			Version:            bundle.Manifest.Version,
			Subset:             bundle.Manifest.Subset,
			UpstreamRevision:   bundle.Manifest.UpstreamRevision,
			SourceSHA256:       strings.ToLower(bundle.Manifest.SourceSHA256),
			SourceChecksum:     strings.ToLower(bundle.Manifest.SourceChecksum),
			ConversionVersion:  bundle.Manifest.ConversionVersion,
			PrerequisiteDigest: checksumBytes(prerequisiteBytes),
			SyntheticFixture:   bundle.Manifest.SyntheticFixture,
		},
		Coverage: coverage,
		Contract: BFCLV4ContractSummary{
			OperationAccuracy:       contract.OperationAccuracy,
			MalformedRate:           contract.MalformedRate,
			RefusalAccuracy:         contract.RefusalAccuracy,
			ScopeSafetyFailures:     contract.ScopeSafetyFailures + scopeMismatches,
			LifecycleSafetyFailures: contract.LifecycleSafetyFailures,
			Status:                  contract.Status,
		},
		Status:          contract.Status,
		CleanupVerified: true,
	}
	report.Passing = !report.Source.SyntheticFixture && report.Status == StatusSuccess && report.Coverage.Unsupported == 0 && report.Coverage.Incomplete == 0
	return report, nil
}

func MarshalBFCLV4ConformanceReport(report BFCLV4ConformanceReport) ([]byte, error) {
	report.SchemaVersion = SchemaVersion
	if report.Family != FamilyProviderContract || report.Dataset != "bfcl-v4-memory" {
		return nil, &StatusError{Status: StatusInvalidManifest, Message: "invalid BFCL V4 conformance report identity"}
	}
	if err := (BFCLV4SourceManifest{
		SchemaVersion:     BFCLV4SourceSchemaVersion,
		Dataset:           report.Source.Dataset,
		Version:           report.Source.Version,
		UpstreamRevision:  report.Source.UpstreamRevision,
		SourceSHA256:      report.Source.SourceSHA256,
		SourceChecksum:    report.Source.SourceChecksum,
		ConversionVersion: report.Source.ConversionVersion,
		Subset:            report.Source.Subset,
	}).validate(); err != nil {
		return nil, &StatusError{Status: StatusInvalidManifest, Message: "invalid redacted BFCL V4 source provenance", Cause: err}
	}
	return json.Marshal(report)
}
