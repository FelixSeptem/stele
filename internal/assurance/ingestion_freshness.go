package assurance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

const IngestionFreshnessVersion = "ingestion-freshness-v1"

const maxIngestionFreshnessFixtureKeyLength = 128

var ErrIngestionFreshnessReplayConflict = fmt.Errorf("ingestion freshness replay conflict")

// IngestionFreshnessCollector resolves an opaque fixture reference only after
// exact-scope authorization. It is the sole source of stage observations.
type IngestionFreshnessCollector interface {
	CollectIngestionFreshness(context.Context, memory.Scope, string, time.Time) (IngestionFreshnessInput, error)
}

// IngestionFreshnessEvidenceStore persists only the redacted report identity
// and stage categories. Implementations must atomically return an identical
// replay or ErrIngestionFreshnessReplayConflict for the same fixture identity.
type IngestionFreshnessEvidenceStore interface {
	StoreIngestionFreshnessEvidence(context.Context, IngestionFreshnessEvidence) (IngestionFreshnessEvidence, error)
}

type IngestionFreshnessInspectionInput struct {
	Scope            memory.Scope
	FixtureReference string
	ObservedAt       time.Time
}

func (i IngestionFreshnessInspectionInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return err
	}
	return validateIngestionFreshnessFixtureKey(i.FixtureReference)
}

type IngestionFreshnessEvidence struct {
	Scope        memory.Scope
	FixtureHash  string
	IdentityHash string
	Report       IngestionFreshnessReport
	ObservedAt   time.Time
	CreatedAt    time.Time
}

func (e IngestionFreshnessEvidence) Validate() error {
	if err := e.Scope.Validate(); err != nil {
		return err
	}
	if !isIngestionFreshnessHash(e.FixtureHash) || !isIngestionFreshnessHash(e.IdentityHash) {
		return fmt.Errorf("ingestion freshness evidence hashes are invalid")
	}
	if err := e.Report.Validate(); err != nil {
		return fmt.Errorf("ingestion freshness evidence report is invalid: %w", err)
	}
	if e.Report.ContractVersion != IngestionFreshnessVersion || e.Report.FixtureHash != e.FixtureHash || e.Report.IdentityHash != e.IdentityHash {
		return fmt.Errorf("ingestion freshness evidence report identity is invalid")
	}
	if e.ObservedAt.IsZero() || e.CreatedAt.IsZero() {
		return fmt.Errorf("ingestion freshness evidence times are required")
	}
	return nil
}

func isIngestionFreshnessHash(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

type IngestionStageKind string

const (
	IngestionStageAdmitted            IngestionStageKind = "admitted"
	IngestionStageGovernanceCompleted IngestionStageKind = "governance_completed"
	IngestionStageCanonicalActive     IngestionStageKind = "canonical_active"
	IngestionStagePublicRetrievable   IngestionStageKind = "public_retrievable"
)

func (k IngestionStageKind) valid() bool {
	switch k {
	case IngestionStageAdmitted, IngestionStageGovernanceCompleted, IngestionStageCanonicalActive, IngestionStagePublicRetrievable:
		return true
	default:
		return false
	}
}

type IngestionStageDisposition string

const (
	IngestionDispositionComplete    IngestionStageDisposition = "complete"
	IngestionDispositionPending     IngestionStageDisposition = "pending"
	IngestionDispositionRetrying    IngestionStageDisposition = "retrying"
	IngestionDispositionExhausted   IngestionStageDisposition = "exhausted"
	IngestionDispositionIneligible  IngestionStageDisposition = "ineligible"
	IngestionDispositionHidden      IngestionStageDisposition = "hidden"
	IngestionDispositionUnavailable IngestionStageDisposition = "unavailable"
	IngestionDispositionInvalid     IngestionStageDisposition = "invalid"
)

func (d IngestionStageDisposition) valid() bool {
	switch d {
	case IngestionDispositionComplete, IngestionDispositionPending, IngestionDispositionRetrying,
		IngestionDispositionExhausted, IngestionDispositionIneligible, IngestionDispositionHidden,
		IngestionDispositionUnavailable, IngestionDispositionInvalid:
		return true
	default:
		return false
	}
}

type IngestionFreshnessResult string

const (
	IngestionFreshnessResultFresh    IngestionFreshnessResult = "fresh"
	IngestionFreshnessResultDegraded IngestionFreshnessResult = "degraded"
	IngestionFreshnessResultInvalid  IngestionFreshnessResult = "invalid"
)

func (r IngestionFreshnessResult) valid() bool {
	return r == IngestionFreshnessResultFresh || r == IngestionFreshnessResultDegraded || r == IngestionFreshnessResultInvalid
}

// IngestionFreshnessBucket is a closed, redacted stage condition. It carries
// no raw timestamp, identifier, path, query, or lifecycle detail.
type IngestionFreshnessBucket string

const (
	IngestionFreshnessBucketFresh       IngestionFreshnessBucket = "fresh"
	IngestionFreshnessBucketMissing     IngestionFreshnessBucket = "missing"
	IngestionFreshnessBucketPending     IngestionFreshnessBucket = "pending"
	IngestionFreshnessBucketRetrying    IngestionFreshnessBucket = "retrying"
	IngestionFreshnessBucketExhausted   IngestionFreshnessBucket = "exhausted"
	IngestionFreshnessBucketIneligible  IngestionFreshnessBucket = "ineligible"
	IngestionFreshnessBucketHidden      IngestionFreshnessBucket = "hidden"
	IngestionFreshnessBucketUnavailable IngestionFreshnessBucket = "unavailable"
	IngestionFreshnessBucketInvalid     IngestionFreshnessBucket = "invalid"
)

func (b IngestionFreshnessBucket) valid() bool {
	switch b {
	case IngestionFreshnessBucketFresh, IngestionFreshnessBucketMissing,
		IngestionFreshnessBucketPending, IngestionFreshnessBucketRetrying,
		IngestionFreshnessBucketExhausted, IngestionFreshnessBucketIneligible,
		IngestionFreshnessBucketHidden, IngestionFreshnessBucketUnavailable,
		IngestionFreshnessBucketInvalid:
		return true
	default:
		return false
	}
}

type IngestionSLOBucket string

const (
	IngestionSLOWithinWindow  IngestionSLOBucket = "within_window"
	IngestionSLOOutsideWindow IngestionSLOBucket = "outside_window"
	IngestionSLONotEvaluated  IngestionSLOBucket = "not_evaluated"
)

func (b IngestionSLOBucket) valid() bool {
	return b == IngestionSLOWithinWindow || b == IngestionSLOOutsideWindow || b == IngestionSLONotEvaluated
}

type IngestionLatencyBucket string

const (
	IngestionLatencyZero    IngestionLatencyBucket = "zero"
	IngestionLatencyUnder1S IngestionLatencyBucket = "under_1s"
	IngestionLatency1STo10S IngestionLatencyBucket = "1s_10s"
	IngestionLatency10STo1M IngestionLatencyBucket = "10s_1m"
	IngestionLatency1MTo5M  IngestionLatencyBucket = "1m_5m"
	IngestionLatency5MPlus  IngestionLatencyBucket = "5m_plus"
)

func (b IngestionLatencyBucket) valid() bool {
	switch b {
	case IngestionLatencyZero, IngestionLatencyUnder1S, IngestionLatency1STo10S, IngestionLatency10STo1M, IngestionLatency1MTo5M, IngestionLatency5MPlus:
		return true
	default:
		return false
	}
}

type IngestionFreshnessPolicy struct {
	ContractVersion  string
	AdmissionWindow  time.Duration
	GovernanceWindow time.Duration
	RetrievalWindow  time.Duration
}

type IngestionFreshnessStage struct {
	Kind        IngestionStageKind
	Disposition IngestionStageDisposition
	At          time.Time
}

type IngestionFreshnessInput struct {
	Scope      memory.Scope
	ReportKey  string
	ObservedAt time.Time
	Policy     IngestionFreshnessPolicy
	Stages     []IngestionFreshnessStage
}

type IngestionFreshnessStageReport struct {
	Kind          IngestionStageKind        `json:"kind"`
	Disposition   IngestionStageDisposition `json:"disposition"`
	LatencyBucket IngestionLatencyBucket    `json:"latency_bucket,omitempty"`
	Freshness     IngestionFreshnessBucket  `json:"freshness"`
	SLO           IngestionSLOBucket        `json:"slo"`
}

type IngestionFreshnessReport struct {
	ContractVersion string                          `json:"contract_version"`
	FixtureHash     string                          `json:"fixture_hash"`
	IdentityHash    string                          `json:"identity_hash"`
	Result          IngestionFreshnessResult        `json:"result"`
	SLO             IngestionSLOBucket              `json:"slo"`
	Stages          []IngestionFreshnessStageReport `json:"stages"`
}

// Validate ensures persisted or replayed reports remain in the closed public
// contract. It accepts no ad-hoc stage or bucket values.
func (r IngestionFreshnessReport) Validate() error {
	if r.ContractVersion != IngestionFreshnessVersion || !isIngestionFreshnessHash(r.FixtureHash) || !isIngestionFreshnessHash(r.IdentityHash) {
		return fmt.Errorf("invalid ingestion freshness report identity")
	}
	if !r.Result.valid() || !r.SLO.valid() || len(r.Stages) != len(ingestionStageOrder) {
		return fmt.Errorf("invalid ingestion freshness report shape")
	}
	for index, stage := range r.Stages {
		if stage.Kind != ingestionStageOrder[index] || !stage.Disposition.valid() || !stage.Freshness.valid() || !stage.SLO.valid() {
			return fmt.Errorf("invalid ingestion freshness report stage")
		}
		if stage.LatencyBucket != "" && !stage.LatencyBucket.valid() {
			return fmt.Errorf("invalid ingestion freshness latency bucket")
		}
	}
	return nil
}

var ingestionStageOrder = []IngestionStageKind{
	IngestionStageAdmitted,
	IngestionStageGovernanceCompleted,
	IngestionStageCanonicalActive,
	IngestionStagePublicRetrievable,
}

func EvaluateIngestionFreshness(input IngestionFreshnessInput) (IngestionFreshnessReport, error) {
	if err := input.Scope.Validate(); err != nil {
		return IngestionFreshnessReport{}, err
	}
	if err := validateIngestionFreshnessFixtureKey(input.ReportKey); err != nil {
		return IngestionFreshnessReport{}, err
	}
	if input.Policy.ContractVersion != IngestionFreshnessVersion {
		return IngestionFreshnessReport{}, fmt.Errorf("unsupported ingestion freshness contract version %q", input.Policy.ContractVersion)
	}
	if input.ObservedAt.IsZero() || input.Policy.AdmissionWindow <= 0 || input.Policy.GovernanceWindow <= 0 || input.Policy.RetrievalWindow <= 0 {
		return IngestionFreshnessReport{}, fmt.Errorf("observed time and positive freshness windows are required")
	}
	if len(input.Stages) > len(ingestionStageOrder) {
		return IngestionFreshnessReport{}, fmt.Errorf("too many ingestion freshness stages")
	}
	provided := make(map[IngestionStageKind]IngestionFreshnessStage, len(input.Stages))
	for _, stage := range input.Stages {
		if !stage.Kind.valid() || !stage.Disposition.valid() || stage.At.After(input.ObservedAt) {
			return IngestionFreshnessReport{}, fmt.Errorf("invalid ingestion freshness stage")
		}
		if stage.Disposition == IngestionDispositionComplete && stage.At.IsZero() {
			return IngestionFreshnessReport{}, fmt.Errorf("completed ingestion freshness stage requires a timestamp")
		}
		if _, exists := provided[stage.Kind]; exists {
			return IngestionFreshnessReport{}, fmt.Errorf("duplicate ingestion freshness stage %q", stage.Kind)
		}
		provided[stage.Kind] = stage
	}

	fixtureInput := struct {
		Version string
		Scope   memory.Scope
		Key     string
	}{input.Policy.ContractVersion, input.Scope.Normalized(), strings.TrimSpace(input.ReportKey)}
	fixtureBytes, err := json.Marshal(fixtureInput)
	if err != nil {
		return IngestionFreshnessReport{}, fmt.Errorf("encode freshness fixture identity: %w", err)
	}
	fixtureDigest := sha256.Sum256(fixtureBytes)
	identityInput := struct {
		Version string
		Scope   memory.Scope
		Key     string
		Policy  IngestionFreshnessPolicy
		Stages  []IngestionFreshnessStage
	}{input.Policy.ContractVersion, input.Scope.Normalized(), strings.TrimSpace(input.ReportKey), input.Policy, append([]IngestionFreshnessStage(nil), input.Stages...)}
	sort.Slice(identityInput.Stages, func(i, j int) bool { return identityInput.Stages[i].Kind < identityInput.Stages[j].Kind })
	identityBytes, err := json.Marshal(identityInput)
	if err != nil {
		return IngestionFreshnessReport{}, fmt.Errorf("encode freshness identity: %w", err)
	}
	digest := sha256.Sum256(identityBytes)
	report := IngestionFreshnessReport{ContractVersion: input.Policy.ContractVersion, FixtureHash: hex.EncodeToString(fixtureDigest[:]), IdentityHash: hex.EncodeToString(digest[:]), Result: IngestionFreshnessResultFresh, SLO: IngestionSLOWithinWindow, Stages: make([]IngestionFreshnessStageReport, 0, len(ingestionStageOrder))}
	var previous time.Time
	for index, kind := range ingestionStageOrder {
		stage, ok := provided[kind]
		if !ok {
			report.Result = IngestionFreshnessResultDegraded
			report.SLO = IngestionSLONotEvaluated
			report.Stages = append(report.Stages, IngestionFreshnessStageReport{Kind: kind, Disposition: IngestionDispositionUnavailable, Freshness: IngestionFreshnessBucketMissing, SLO: IngestionSLONotEvaluated})
			continue
		}
		stageReport := IngestionFreshnessStageReport{Kind: kind, Disposition: stage.Disposition, SLO: IngestionSLONotEvaluated}
		if stage.Disposition == IngestionDispositionComplete {
			if !previous.IsZero() && stage.At.Before(previous) {
				return IngestionFreshnessReport{}, fmt.Errorf("ingestion freshness stage %q regresses prior stage", kind)
			}
			if index == 0 {
				stageReport.LatencyBucket = IngestionLatencyZero
			} else {
				stageReport.LatencyBucket = ingestionLatencyBucket(stage.At.Sub(previous))
			}
			stageReport.Freshness = IngestionFreshnessBucketFresh
			stageReport.SLO = IngestionSLOWithinWindow
			previous = stage.At
		} else {
			report.Result = IngestionFreshnessResultDegraded
			stageReport.Freshness = IngestionFreshnessBucket(stage.Disposition)
			stageReport.SLO = IngestionSLONotEvaluated
		}
		report.Stages = append(report.Stages, stageReport)
	}
	if admission, ok := provided[IngestionStageAdmitted]; ok && admission.Disposition == IngestionDispositionComplete {
		age := input.ObservedAt.Sub(admission.At)
		admissionReport := &report.Stages[0]
		admissionReport.LatencyBucket = ingestionLatencyBucket(age)
		if age > input.Policy.AdmissionWindow {
			report.Result = IngestionFreshnessResultDegraded
			report.SLO = IngestionSLOOutsideWindow
			admissionReport.SLO = IngestionSLOOutsideWindow
		}
	}
	if report.Result == IngestionFreshnessResultFresh {
		if err := validateIngestionFreshnessWindows(provided, input.Policy); err != nil {
			report.Result = IngestionFreshnessResultDegraded
			report.SLO = IngestionSLOOutsideWindow
			return report, nil
		}
	}
	return report, nil
}

func validateIngestionFreshnessFixtureKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("ingestion freshness fixture key is required")
	}
	if len(key) > maxIngestionFreshnessFixtureKeyLength {
		return fmt.Errorf("ingestion freshness fixture key must be at most %d bytes", maxIngestionFreshnessFixtureKeyLength)
	}
	return nil
}

func ingestionLatencyBucket(d time.Duration) IngestionLatencyBucket {
	switch {
	case d < time.Second:
		return IngestionLatencyUnder1S
	case d < 10*time.Second:
		return IngestionLatency1STo10S
	case d < time.Minute:
		return IngestionLatency10STo1M
	case d < 5*time.Minute:
		return IngestionLatency1MTo5M
	default:
		return IngestionLatency5MPlus
	}
}

func validateIngestionFreshnessWindows(stages map[IngestionStageKind]IngestionFreshnessStage, policy IngestionFreshnessPolicy) error {
	for _, pair := range [][2]IngestionStageKind{{IngestionStageAdmitted, IngestionStageGovernanceCompleted}, {IngestionStageGovernanceCompleted, IngestionStageCanonicalActive}, {IngestionStageCanonicalActive, IngestionStagePublicRetrievable}} {
		left, right := stages[pair[0]], stages[pair[1]]
		window := policy.GovernanceWindow
		if pair[1] == IngestionStagePublicRetrievable {
			window = policy.RetrievalWindow
		}
		if right.At.Sub(left.At) > window {
			return fmt.Errorf("ingestion freshness window exceeded between %q and %q", pair[0], pair[1])
		}
	}
	return nil
}
