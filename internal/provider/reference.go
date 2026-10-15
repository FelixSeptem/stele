package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/google/uuid"
)

const (
	DurableReferenceContractVersion = memory.DurableReferenceVersion
	MaxReferenceValueBytes          = 4096
	MaxReferenceIDBytes             = 128
	MaxReferenceResponseBytes       = 16384
)

var referenceEventTypes = map[string]bool{"provider.reference.task": true, "provider.reference.run": true, "provider.reference.handoff": true, "provider.reference.compilation": true}

func IsDurableReferenceEventType(v string) bool { return referenceEventTypes[v] }

type ReferenceWriteInput struct {
	ContractVersion string                 `json:"contract_version"`
	ReferenceID     string                 `json:"reference_id"`
	MemoryPath      string                 `json:"memory_path"`
	EventType       string                 `json:"event_type"`
	Value           string                 `json:"value"`
	SourceMetadata  *memory.SourceMetadata `json:"source_metadata,omitempty"`
}
type ReferenceWriteRequest struct {
	Metadata OperationMetadata   `json:"metadata"`
	Input    ReferenceWriteInput `json:"input"`
}
type ReferenceStatusInput struct {
	ContractVersion   string `json:"contract_version"`
	EventID           string `json:"event_id"`
	DisclosureProfile string `json:"disclosure_profile,omitempty"`
	ResponseMode      string `json:"response_mode,omitempty"`
}
type ReferenceStatusRequest struct {
	Metadata OperationMetadata    `json:"metadata"`
	Input    ReferenceStatusInput `json:"input"`
}
type ReferenceLookupInput struct {
	ContractVersion   string `json:"contract_version"`
	ReferenceID       string `json:"reference_id"`
	MemoryPath        string `json:"memory_path"`
	DisclosureProfile string `json:"disclosure_profile,omitempty"`
	ResponseMode      string `json:"response_mode,omitempty"`
}
type ReferenceLookupRequest struct {
	Metadata OperationMetadata    `json:"metadata"`
	Input    ReferenceLookupInput `json:"input"`
}

type ReferenceRecord struct {
	ReferenceID  string `json:"reference_id"`
	MemoryPath   string `json:"memory_path"`
	EventType    string `json:"event_type"`
	EventID      string `json:"event_id"`
	CandidateID  string `json:"candidate_id"`
	MemoryID     string `json:"memory_id"`
	VersionID    string `json:"version_id"`
	ProvenanceID string `json:"provenance_id"`
	Version      int64  `json:"version"`
	Value        string `json:"value,omitempty"`
}
type ReferenceResponse struct {
	Metadata        OperationMetadata        `json:"metadata"`
	ContractVersion string                   `json:"contract_version"`
	Scope           CanonicalScopeDescriptor `json:"scope"`
	Status          string                   `json:"status"`
	EventID         string                   `json:"event_id,omitempty"`
	Replayed        bool                     `json:"replayed,omitempty"`
	Reference       *ReferenceRecord         `json:"reference,omitempty"`
	RetryAfterMS    int                      `json:"retry_after_ms"`
	Retryable       bool                     `json:"retryable"`
	ErrorCategory   string                   `json:"error_category,omitempty"`
}

const (
	ReferenceStatusAccepted    = "accepted"
	ReferenceStatusPending     = "pending"
	ReferenceStatusFailed      = "failed"
	ReferenceStatusCompleted   = "completed"
	ReferenceStatusUnavailable = "unavailable"
)

type DurableReferenceCapabilities struct {
	Enabled             bool     `json:"enabled"`
	ContractVersion     string   `json:"contract_version"`
	EventTypes          []string `json:"event_types"`
	Statuses            []string `json:"statuses"`
	MaxValueBytes       int      `json:"max_value_bytes"`
	MaxReferenceIDBytes int      `json:"max_reference_id_bytes"`
	MaxPathBytes        int      `json:"max_path_bytes"`
	MaxResponseBytes    int      `json:"max_response_bytes"`
	PollAfterMS         int      `json:"poll_after_ms"`
}

func DurableReferenceCapability(enabled bool) DurableReferenceCapabilities {
	return DurableReferenceCapabilities{Enabled: enabled, ContractVersion: DurableReferenceContractVersion, EventTypes: memory.ReferenceEventTypes(), Statuses: []string{ReferenceStatusAccepted, ReferenceStatusPending, ReferenceStatusFailed, ReferenceStatusCompleted, ReferenceStatusUnavailable}, MaxValueBytes: MaxReferenceValueBytes, MaxReferenceIDBytes: MaxReferenceIDBytes, MaxPathBytes: memory.MemoryPathMaxLength, MaxResponseBytes: MaxReferenceResponseBytes, PollAfterMS: 1000}
}

func decodeReferenceEnvelope(data []byte, input any, allowed, required []string, write bool) (OperationMetadata, error) {
	if !ValidJSONUnicode(data) {
		return OperationMetadata{}, fmt.Errorf("invalid Unicode")
	}
	obj, err := CanonicalObject(data, []string{"metadata", "input"}, []string{"metadata", "input"})
	if err != nil {
		return OperationMetadata{}, err
	}
	metadataRequired := []string{"request_id", "operation_id", "schema_version"}
	if write {
		metadataRequired = append(metadataRequired, "idempotency_key")
	}
	if _, err = CanonicalObject(obj["metadata"], []string{"request_id", "operation_id", "idempotency_key", "event_seq", "schema_version", "memory_path", "precedence_version", "precedence_stage", "precedence_outcome"}, metadataRequired); err != nil {
		return OperationMetadata{}, err
	}
	if _, err = CanonicalObject(obj["input"], allowed, required); err != nil {
		return OperationMetadata{}, err
	}
	var meta OperationMetadata
	if err := json.Unmarshal(obj["metadata"], &meta); err != nil {
		return meta, err
	}
	if err := validateReferenceMetadata(&meta); err != nil {
		return meta, err
	}
	if write && meta.IdempotencyKey == "" {
		return meta, fmt.Errorf("idempotency key required")
	}
	if err := json.Unmarshal(obj["input"], input); err != nil {
		return meta, err
	}
	return meta, nil
}
func validateReferenceCommon(version, id, path string) (string, error) {
	if version != DurableReferenceContractVersion {
		return "", fmt.Errorf("unsupported reference contract")
	}
	if id == "" || len(id) > MaxReferenceIDBytes || !boundedToken(id, MaxReferenceIDBytes) {
		return "", fmt.Errorf("invalid reference id")
	}
	p, err := memory.NormalizeMemoryPath(path)
	if err != nil || p == memory.MemoryPathRoot {
		return "", fmt.Errorf("non-root memory path required")
	}
	return p, nil
}
func validateReferenceDisclosure(profile, mode string) error {
	if profile != "" && !boundedToken(profile, 64) {
		return fmt.Errorf("invalid disclosure profile")
	}
	if mode != "" && mode != "reference_only" && mode != "bounded_content" {
		return fmt.Errorf("invalid response mode")
	}
	return nil
}

func validateReferenceMetadata(meta *OperationMetadata) error {
	if err := meta.NormalizeValidate(); err != nil {
		return err
	}
	encoded, err := json.Marshal(meta)
	if err != nil || len(encoded) > MaxOperationMetadataBytes {
		return fmt.Errorf("reference operation metadata exceeds limit")
	}
	return nil
}

// Reserve the largest permitted read metadata and canonical scope, all lineage
// identities, and the largest version number. Measuring the actual JSON value
// and normalized path accounts for escaping before durable admission, so an
// accepted value can be read within the response allocation after restart.
func referenceValueFitsResponse(i ReferenceWriteInput) bool {
	path, err := memory.NormalizeMemoryPath(i.MemoryPath)
	if err != nil {
		return false
	}
	token := strings.Repeat("a", 128)
	id := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	metadata := OperationMetadata{}
	response := ReferenceResponse{
		Metadata: metadata, ContractVersion: DurableReferenceContractVersion,
		Scope:     CanonicalScopeDescriptor{Tenant: token, Project: token, Namespace: token, AgentID: token, SessionID: token, ConversationID: token, ProviderInstanceID: token},
		Status:    ReferenceStatusCompleted,
		Reference: &ReferenceRecord{ReferenceID: i.ReferenceID, MemoryPath: path, EventType: i.EventType, EventID: id, CandidateID: id, MemoryID: id, VersionID: id, ProvenanceID: id, Version: 1<<63 - 1, Value: i.Value},
	}
	encoded, err := json.Marshal(response)
	emptyMetadata, _ := json.Marshal(metadata)
	return err == nil && len(encoded)-len(emptyMetadata)+MaxOperationMetadataBytes <= MaxReferenceResponseBytes
}

func (i ReferenceWriteInput) Validate() error {
	_, err := validateReferenceCommon(i.ContractVersion, i.ReferenceID, i.MemoryPath)
	if err != nil {
		return err
	}
	if !IsDurableReferenceEventType(i.EventType) {
		return fmt.Errorf("unsupported reference event type")
	}
	if strings.TrimSpace(i.Value) == "" || len(i.Value) > MaxReferenceValueBytes || !utf8.ValidString(i.Value) || strings.TrimSpace(i.Value) != i.Value {
		return fmt.Errorf("invalid reference value")
	}
	if !referenceValueFitsResponse(i) {
		return fmt.Errorf("reference value exceeds serialized response allocation")
	}
	if i.SourceMetadata != nil && i.SourceMetadata.Validate() != nil {
		return fmt.Errorf("invalid source metadata")
	}
	return nil
}
func (i ReferenceStatusInput) Validate() error {
	if i.ContractVersion != DurableReferenceContractVersion {
		return fmt.Errorf("unsupported reference contract")
	}
	if _, err := parseUUID(i.EventID); err != nil {
		return fmt.Errorf("invalid event id")
	}
	return validateReferenceDisclosure(i.DisclosureProfile, i.ResponseMode)
}
func (i ReferenceLookupInput) Validate() error {
	if _, err := validateReferenceCommon(i.ContractVersion, i.ReferenceID, i.MemoryPath); err != nil {
		return err
	}
	return validateReferenceDisclosure(i.DisclosureProfile, i.ResponseMode)
}
func DecodeReferenceWriteRequest(data []byte) (ReferenceWriteRequest, error) {
	var i ReferenceWriteInput
	meta, err := decodeReferenceEnvelope(data, &i, []string{"contract_version", "reference_id", "memory_path", "event_type", "value", "source_metadata"}, []string{"contract_version", "reference_id", "memory_path", "event_type", "value"}, true)
	if err != nil {
		return ReferenceWriteRequest{}, err
	}
	r := ReferenceWriteRequest{Metadata: meta, Input: i}
	if err := r.Input.Validate(); err != nil {
		return r, err
	}
	p, _ := memory.NormalizeMemoryPath(i.MemoryPath)
	r.Input.MemoryPath = p
	return r, nil
}
func DecodeReferenceStatusRequest(data []byte) (ReferenceStatusRequest, error) {
	var i ReferenceStatusInput
	meta, err := decodeReferenceEnvelope(data, &i, []string{"contract_version", "event_id", "disclosure_profile", "response_mode"}, []string{"contract_version", "event_id"}, false)
	if err != nil {
		return ReferenceStatusRequest{}, err
	}
	r := ReferenceStatusRequest{Metadata: meta, Input: i}
	return r, r.Input.Validate()
}
func DecodeReferenceLookupRequest(data []byte) (ReferenceLookupRequest, error) {
	var i ReferenceLookupInput
	meta, err := decodeReferenceEnvelope(data, &i, []string{"contract_version", "reference_id", "memory_path", "disclosure_profile", "response_mode"}, []string{"contract_version", "reference_id", "memory_path"}, false)
	if err != nil {
		return ReferenceLookupRequest{}, err
	}
	r := ReferenceLookupRequest{Metadata: meta, Input: i}
	if err := r.Input.Validate(); err != nil {
		return r, err
	}
	p, _ := memory.NormalizeMemoryPath(i.MemoryPath)
	r.Input.MemoryPath = p
	return r, nil
}

func parseUUID(v string) (string, error) {
	parsed, err := uuid.Parse(v)
	if err != nil || len(v) != 36 || parsed == uuid.Nil {
		return "", fmt.Errorf("uuid required")
	}
	return parsed.String(), nil
}
func referenceValueFingerprint(v string) string {
	h := sha256.Sum256([]byte(v))
	return hex.EncodeToString(h[:])
}

type ReferenceStore interface {
	ReferenceStatus(context.Context, memory.Scope, string, string, string, string) (ReferenceResponse, error)
	ReferenceLookup(context.Context, memory.Scope, string, string, string, string, string) (ReferenceResponse, error)
}

func (a *Adapter) WriteReference(ctx context.Context, binding RuntimeBinding, req ReferenceWriteRequest) (ReferenceResponse, error) {
	// The bounded response reserves ASCII scope tokens of at most 128 bytes.
	// Internal callers may omit runtime dimensions; every supplied value must
	// still fit the public reference projection before durable admission.
	for _, value := range []string{binding.Scope.Tenant, binding.Scope.Project, binding.Scope.Namespace, binding.AgentID, binding.SessionID, binding.ConversationID, binding.ProviderInstanceID} {
		if value != "" && !boundedToken(value, 128) {
			return ReferenceResponse{}, fmt.Errorf("invalid reference scope")
		}
	}
	if err := validateReferenceMetadata(&req.Metadata); err != nil {
		return ReferenceResponse{}, err
	}
	if err := req.Input.Validate(); err != nil {
		return ReferenceResponse{}, err
	}
	if req.Metadata.IdempotencyKey == "" {
		return ReferenceResponse{}, fmt.Errorf("idempotency key required")
	}
	normalizedPath, err := memory.NormalizeMemoryPath(req.Input.MemoryPath)
	if err != nil {
		return ReferenceResponse{}, err
	}
	req.Input.MemoryPath = normalizedPath
	input := memory.IngestEventInput{Scope: binding.Scope, MemoryPath: req.Input.MemoryPath, EventType: req.Input.EventType, Content: req.Input.Value, Metadata: map[string]any{"reference_contract_version": DurableReferenceContractVersion, "reference_id": req.Input.ReferenceID, "reference_value_fingerprint": referenceValueFingerprint(req.Input.Value), "reference_binding_id": binding.BindingID, "reference_principal_id": binding.PrincipalID, "reference_operation_id": req.Metadata.OperationID}}
	if req.Input.SourceMetadata != nil {
		input.SourceMetadata = *req.Input.SourceMetadata
	} else {
		input.SourceMetadata = (memory.SourceMetadata{}).Effective()
	}
	// Reference replay is fenced by the durable idempotency key. event_seq is
	// compatibility attribution, not a process-local ordering or replay gate.
	ingestMetadata := req.Metadata
	ingestMetadata.EventSeq = 0
	out, err := a.Ingest(ctx, binding, ingestMetadata, input)
	out.Metadata.EventSeq = req.Metadata.EventSeq
	if err != nil {
		return ReferenceResponse{Metadata: out.Metadata, ContractVersion: DurableReferenceContractVersion, Scope: scopeDescriptor(binding.Scope), Status: ReferenceStatusFailed, Retryable: false}, err
	}
	return ReferenceResponse{Metadata: out.Metadata, ContractVersion: DurableReferenceContractVersion, Scope: scopeDescriptor(binding.Scope), Status: ReferenceStatusAccepted, EventID: out.EventID, Replayed: out.Replayed, RetryAfterMS: 1000, Retryable: true}, nil
}
func (a *Adapter) ReferenceStatus(ctx context.Context, binding RuntimeBinding, req ReferenceStatusRequest) (ReferenceResponse, error) {
	if err := a.validate(binding, &req.Metadata); err != nil {
		return ReferenceResponse{}, err
	}
	if err := validateReferenceMetadata(&req.Metadata); err != nil {
		return ReferenceResponse{}, err
	}
	if err := req.Input.Validate(); err != nil {
		return ReferenceResponse{}, err
	}
	if a.deps.ReferenceStore == nil {
		return ReferenceResponse{}, fmt.Errorf("reference store is not configured")
	}
	return a.deps.ReferenceStore.ReferenceStatus(ctx, binding.Scope, binding.PrincipalID, req.Input.EventID, req.Input.DisclosureProfile, req.Input.ResponseMode)
}
func (a *Adapter) ReferenceLookup(ctx context.Context, binding RuntimeBinding, req ReferenceLookupRequest) (ReferenceResponse, error) {
	if err := a.validate(binding, &req.Metadata); err != nil {
		return ReferenceResponse{}, err
	}
	if err := validateReferenceMetadata(&req.Metadata); err != nil {
		return ReferenceResponse{}, err
	}
	if err := req.Input.Validate(); err != nil {
		return ReferenceResponse{}, err
	}
	if a.deps.ReferenceStore == nil {
		return ReferenceResponse{}, fmt.Errorf("reference store is not configured")
	}
	return a.deps.ReferenceStore.ReferenceLookup(ctx, binding.Scope, binding.PrincipalID, req.Input.ReferenceID, req.Input.MemoryPath, req.Input.DisclosureProfile, req.Input.ResponseMode)
}
func scopeDescriptor(s memory.Scope) CanonicalScopeDescriptor {
	return CanonicalScopeDescriptor{Tenant: s.Tenant, Project: s.Project, Namespace: s.Namespace}
}
