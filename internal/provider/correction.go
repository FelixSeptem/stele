package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/FelixSeptem/stele/internal/memory"
)

const (
	CorrectionContractVersion = "provider-correction-governed-version-v1"
	CorrectionOperationName   = "corrections"
	CorrectionOperation       = "POST /v1/provider/corrections"
	MaxCorrectionRequestBytes = 32768
	MaxCorrectionContentBytes = 16384
	MaxCorrectionReasonBytes  = 512
)

var (
	// ErrCorrectionUnavailable merges foreign, absent, first revision, hidden,
	// and mismatched correction sources into one non-disclosing public outcome.
	ErrCorrectionUnavailable = errors.New("correction_unavailable")
	ErrCorrectionStale       = errors.New("correction_stale")
	ErrCorrectionConflict    = errors.New("correction_conflict")
	ErrCorrectionCapability  = errors.New("provider correction capability unavailable")
)

// RequireCorrectionCapability is for strict consumers that cannot safely
// substitute generic intents when the governed correction operation is absent.
func RequireCorrectionCapability(document CapabilityDocument) error {
	if err := document.Validate(); err != nil {
		return fmt.Errorf("%w: invalid capability document", ErrCorrectionCapability)
	}
	if !document.Corrections.Enabled {
		return ErrCorrectionCapability
	}
	for _, operation := range document.Operations {
		if operation == CorrectionOperationName {
			return nil
		}
	}
	return ErrCorrectionCapability
}

type CorrectionCapabilities struct {
	Enabled          bool     `json:"enabled"`
	ContractVersion  string   `json:"contract_version"`
	Operation        string   `json:"operation"`
	MaxRequestBytes  int      `json:"max_request_bytes"`
	MaxContentBytes  int      `json:"max_content_bytes"`
	MaxReasonBytes   int      `json:"max_reason_bytes"`
	ResultCategories []string `json:"result_categories"`
}

func CorrectionCapability(enabled bool) CorrectionCapabilities {
	return CorrectionCapabilities{
		Enabled: enabled, ContractVersion: CorrectionContractVersion, Operation: CorrectionOperation,
		MaxRequestBytes: MaxCorrectionRequestBytes, MaxContentBytes: MaxCorrectionContentBytes,
		MaxReasonBytes:   MaxCorrectionReasonBytes,
		ResultCategories: []string{"accepted", "unavailable", "stale", "conflict", "dependency"},
	}
}

func (c CorrectionCapabilities) Validate() error {
	expected := CorrectionCapability(c.Enabled)
	if c.ContractVersion != expected.ContractVersion || c.Operation != expected.Operation ||
		c.MaxRequestBytes != expected.MaxRequestBytes || c.MaxContentBytes != expected.MaxContentBytes ||
		c.MaxReasonBytes != expected.MaxReasonBytes || len(c.ResultCategories) != len(expected.ResultCategories) {
		return fmt.Errorf("unsupported correction capabilities")
	}
	for index, category := range expected.ResultCategories {
		if c.ResultCategories[index] != category {
			return fmt.Errorf("unsupported correction result categories")
		}
	}
	return nil
}

type CorrectionInput struct {
	ContractVersion string `json:"contract_version"`
	ProjectionID    string `json:"projection_id"`
	TargetMemoryID  string `json:"target_memory_id"`
	ExpectedVersion int64  `json:"expected_version"`
	Content         string `json:"content"`
	Reason          string `json:"reason"`
}

type CorrectionRequest struct {
	Metadata      OperationMetadata `json:"metadata"`
	Input         CorrectionInput   `json:"input"`
	PolicyVersion string            `json:"-"`
}

// CorrectionResult is the intentionally bounded public acknowledgement.
// Projection and target lineage details remain internal to the ledger.
type CorrectionResult struct {
	IntentID         string                    `json:"intent_id"`
	Status           memory.MemoryIntentStatus `json:"status"`
	AttributionState string                    `json:"attribution_state"`
}

func (r CorrectionResult) Validate() error {
	if _, err := parseUUID(r.IntentID); err != nil || r.IntentID == "" {
		return fmt.Errorf("invalid correction intent result")
	}
	if r.Status != memory.MemoryIntentStatusAccepted && r.Status != memory.MemoryIntentStatusReplayed {
		return fmt.Errorf("invalid correction intent status")
	}
	if r.AttributionState != "verified" {
		return fmt.Errorf("invalid correction attribution state")
	}
	return nil
}

// CorrectionResolver receives the authenticated runtime binding from HTTP
// middleware and owns correction/reference/target verification and attribution.
type CorrectionResolver interface {
	SubmitCorrection(context.Context, RuntimeBinding, CorrectionRequest) (CorrectionResult, error)
}

func (i CorrectionInput) Validate() error {
	if i.ContractVersion != CorrectionContractVersion {
		return fmt.Errorf("unsupported correction contract")
	}
	for _, value := range []string{i.ProjectionID, i.TargetMemoryID} {
		parsed, err := parseUUID(value)
		if err != nil || parsed != value {
			return fmt.Errorf("invalid correction identity")
		}
	}
	if i.ExpectedVersion < 1 {
		return fmt.Errorf("invalid expected target version")
	}
	if !validCorrectionText(i.Content, MaxCorrectionContentBytes) || !validCorrectionText(i.Reason, MaxCorrectionReasonBytes) {
		return fmt.Errorf("invalid correction content or reason")
	}
	return nil
}

func validCorrectionText(value string, maxBytes int) bool {
	return value != "" && value == strings.TrimSpace(value) && utf8.ValidString(value) && len(value) <= maxBytes
}

func DecodeCorrectionRequest(data []byte) (CorrectionRequest, error) {
	var request CorrectionRequest
	if len(data) > MaxCorrectionRequestBytes {
		return request, fmt.Errorf("correction request exceeds limit")
	}
	metadata, err := decodeReferenceEnvelope(data, &request.Input,
		[]string{"contract_version", "projection_id", "target_memory_id", "expected_version", "content", "reason"},
		[]string{"contract_version", "projection_id", "target_memory_id", "expected_version", "content", "reason"}, true)
	request.Metadata = metadata
	if err != nil {
		return request, err
	}
	object, err := CanonicalObject(data, []string{"metadata", "input"}, []string{"metadata", "input"})
	if err != nil {
		return request, err
	}
	if err = validateProjectionWireMetadata(object["metadata"], metadata); err != nil {
		return request, err
	}
	if err = request.Input.Validate(); err != nil {
		return request, err
	}
	return request, nil
}
