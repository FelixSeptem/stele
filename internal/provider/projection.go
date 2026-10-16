package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	ProjectionContractVersion  = "turn-outcome-feedback-projection-v1"
	ProjectionEvidenceKind     = "runtime_reported_evidence"
	ProjectionFeedbackPolicy   = "projection-feedback-v1"
	MaxProjectionRequestBytes  = 32768
	MaxProjectionResponseBytes = 65536
	MaxProjectionReferences    = 8
	MaxProjectionHistory       = 8
)

var (
	ErrProjectionConflict    = errors.New("projection_conflict")
	ErrProjectionStale       = errors.New("projection_stale")
	ErrProjectionUnavailable = errors.New("projection_unavailable")
)

type ProjectionReferenceSelector struct {
	EventID string `json:"event_id"`
}
type ProjectionFeedback struct {
	Signal     string `json:"signal"`
	ReasonCode string `json:"reason_code"`
}
type ProjectionInput struct {
	ContractVersion  string                        `json:"contract_version"`
	Kind             string                        `json:"kind"`
	EffectID         string                        `json:"effect_id"`
	SourceRevision   int64                         `json:"source_revision"`
	OccurredAt       string                        `json:"occurred_at"`
	ReportedOutcome  string                        `json:"reported_outcome"`
	SessionRunID     string                        `json:"session_run_id,omitempty"`
	TurnID           string                        `json:"turn_id,omitempty"`
	ContextDigest    string                        `json:"context_digest,omitempty"`
	ContextReference string                        `json:"context_reference,omitempty"`
	References       []ProjectionReferenceSelector `json:"references,omitempty"`
	Feedback         *ProjectionFeedback           `json:"feedback,omitempty"`
	Provenance       []string                      `json:"provenance,omitempty"`
}
type ProjectionRequest struct {
	Metadata OperationMetadata `json:"metadata"`
	Input    ProjectionInput   `json:"input"`
}
type ProjectionHistoryInput struct {
	ContractVersion string `json:"contract_version"`
	Kind            string `json:"kind"`
	EffectID        string `json:"effect_id"`
	Limit           int    `json:"limit"`
}
type ProjectionHistoryRequest struct {
	Metadata OperationMetadata      `json:"metadata"`
	Input    ProjectionHistoryInput `json:"input"`
}
type ProjectionReferenceAttribution struct {
	EventID  string                      `json:"event_id,omitempty"`
	State    string                      `json:"state"`
	Eligible bool                        `json:"eligible"`
	Lineage  *ProjectionReferenceLineage `json:"lineage,omitempty"`
}

// Lineage is the content-free, currently authorized PC5 completion proof.
type ProjectionReferenceLineage struct {
	CandidateID  string `json:"candidate_id"`
	MemoryID     string `json:"memory_id"`
	VersionID    string `json:"version_id"`
	ProvenanceID string `json:"provenance_id"`
	Version      int64  `json:"version"`
}

// ProjectionRecord contains no input payload, business-state receipt or content.
// References are admission snapshots; current_references are reauthorized reads.
type ProjectionRecord struct {
	ProjectionID             string                           `json:"projection_id"`
	LogicalID                string                           `json:"logical_id"`
	SourceRevision           int64                            `json:"source_revision"`
	SupersedesID             string                           `json:"supersedes_id,omitempty"`
	Disposition              string                           `json:"disposition"`
	EvidenceKind             string                           `json:"evidence_kind"`
	References               []ProjectionReferenceAttribution `json:"references"`
	CurrentReferences        []ProjectionReferenceAttribution `json:"current_references"`
	FeedbackState            string                           `json:"feedback_state"`
	ContextDigestAttribution string                           `json:"context_digest_attribution"`
}
type ProjectionResponse struct {
	Metadata        OperationMetadata        `json:"metadata"`
	ContractVersion string                   `json:"contract_version"`
	Scope           CanonicalScopeDescriptor `json:"scope"`
	Record          ProjectionRecord         `json:"record"`
}
type ProjectionHistoryResponse struct {
	Metadata        OperationMetadata        `json:"metadata"`
	ContractVersion string                   `json:"contract_version"`
	Scope           CanonicalScopeDescriptor `json:"scope"`
	Records         []ProjectionRecord       `json:"records"`
	HasMore         bool                     `json:"has_more"`
}
type ProjectionStore interface {
	AdmitProjection(context.Context, RuntimeBinding, ProjectionRequest) (ProjectionRecord, error)
	ProjectionHistory(context.Context, RuntimeBinding, ProjectionHistoryInput) ([]ProjectionRecord, bool, error)
}

func validProjectionKind(v string) bool { return v == "turn" || v == "outcome" || v == "feedback" }
func (i ProjectionInput) Validate() error {
	if i.ContractVersion != ProjectionContractVersion {
		return fmt.Errorf("unsupported projection contract")
	}
	if !validProjectionKind(i.Kind) || !boundedToken(i.EffectID, 128) || i.SourceRevision < 1 || i.SourceRevision > 2147483647 {
		return fmt.Errorf("invalid projection identity")
	}
	at, err := time.Parse(time.RFC3339, i.OccurredAt)
	if err != nil || at.IsZero() || at.Format(time.RFC3339) != i.OccurredAt || !strings.HasSuffix(i.OccurredAt, "Z") {
		return fmt.Errorf("invalid projection occurrence time")
	}
	switch i.ReportedOutcome {
	case "unknown", "succeeded", "failed", "cancelled":
	default:
		return fmt.Errorf("invalid reported outcome")
	}
	for _, id := range []string{i.SessionRunID, i.TurnID} {
		if id != "" && !boundedToken(id, 128) {
			return fmt.Errorf("invalid session identity")
		}
	}
	if i.TurnID != "" && i.SessionRunID == "" {
		return fmt.Errorf("turn requires session run")
	}
	if i.ContextDigest != "" && !ValidContextHash(i.ContextDigest) {
		return fmt.Errorf("invalid context digest")
	}
	if i.ContextReference != "" && (i.ContextDigest == "" || !ValidContextReference(i.ContextReference)) {
		return fmt.Errorf("invalid context reference")
	}
	if len(i.References) > MaxProjectionReferences || len(i.Provenance) > 8 {
		return fmt.Errorf("projection evidence exceeds limit")
	}
	seen := map[string]bool{}
	for _, ref := range i.References {
		p, err := parseUUID(ref.EventID)
		if err != nil || p != ref.EventID || seen[p] {
			return fmt.Errorf("invalid or duplicate reference")
		}
		seen[p] = true
	}
	seen = map[string]bool{}
	for _, p := range i.Provenance {
		if !boundedToken(p, 128) || seen[p] {
			return fmt.Errorf("invalid provenance")
		}
		seen[p] = true
	}
	if i.Feedback != nil {
		if len(i.References) == 0 || !boundedToken(i.Feedback.ReasonCode, 64) {
			return fmt.Errorf("feedback requires references and reason")
		}
		switch i.Feedback.Signal {
		case "useful", "irrelevant", "needs_review":
		default:
			return fmt.Errorf("invalid feedback signal")
		}
	}
	if i.Kind == "feedback" && i.Feedback == nil {
		return fmt.Errorf("feedback projection requires feedback")
	}
	return nil
}
func (i ProjectionInput) Fingerprint() string {
	i.References = append([]ProjectionReferenceSelector(nil), i.References...)
	i.Provenance = append([]string(nil), i.Provenance...)
	sort.Slice(i.References, func(a, b int) bool { return i.References[a].EventID < i.References[b].EventID })
	sort.Strings(i.Provenance)
	data, _ := json.Marshal(i)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func (r ProjectionRecord) Validate() error {
	for _, id := range []string{r.ProjectionID, r.LogicalID} {
		if parsed, err := parseUUID(id); err != nil || parsed != id {
			return fmt.Errorf("invalid projection result identity")
		}
	}
	if r.SourceRevision < 1 || r.SourceRevision > 2147483647 || r.EvidenceKind != ProjectionEvidenceKind {
		return fmt.Errorf("invalid projection result")
	}
	if r.SupersedesID != "" {
		if parsed, err := parseUUID(r.SupersedesID); err != nil || parsed != r.SupersedesID || r.SupersedesID == r.ProjectionID {
			return fmt.Errorf("invalid supersession identity")
		}
	}
	if (r.Disposition != "accepted" && r.Disposition != "corrected") || (r.Disposition == "corrected") != (r.SupersedesID != "") {
		return fmt.Errorf("invalid projection disposition")
	}
	switch r.FeedbackState {
	case "none", "pending":
	default:
		return fmt.Errorf("invalid feedback admission state")
	}
	switch r.ContextDigestAttribution {
	case "none", "reported", "verified_receipt":
	default:
		return fmt.Errorf("invalid digest attribution")
	}
	for _, refs := range [][]ProjectionReferenceAttribution{r.References, r.CurrentReferences} {
		if refs == nil || len(refs) > MaxProjectionReferences {
			return fmt.Errorf("invalid reference collection")
		}
		seen := map[string]bool{}
		for _, ref := range refs {
			if (ref.State == "completed") != (ref.Lineage != nil) {
				return fmt.Errorf("inconsistent reference lineage")
			}
			if ref.Lineage != nil {
				for _, id := range []string{ref.Lineage.CandidateID, ref.Lineage.MemoryID, ref.Lineage.VersionID, ref.Lineage.ProvenanceID} {
					if parsed, err := parseUUID(id); err != nil || parsed != id {
						return fmt.Errorf("invalid reference lineage")
					}
				}
				if ref.Lineage.Version < 1 {
					return fmt.Errorf("invalid reference version")
				}
			}
			switch ref.State {
			case "accepted", "pending", "completed", "failed", "unavailable":
			default:
				return fmt.Errorf("invalid reference state")
			}
			if ref.State == "unavailable" {
				if ref.EventID != "" || ref.Eligible {
					return fmt.Errorf("unavailable reference discloses identity")
				}
				continue
			}
			parsed, err := parseUUID(ref.EventID)
			if err != nil || parsed != ref.EventID || seen[parsed] || ref.Eligible != (ref.State == "completed") {
				return fmt.Errorf("inconsistent reference attribution")
			}
			seen[parsed] = true
		}
	}
	if len(r.References) != len(r.CurrentReferences) {
		return fmt.Errorf("inconsistent reference cardinality")
	}
	return nil
}
func (i ProjectionHistoryInput) Validate() error {
	if i.ContractVersion != ProjectionContractVersion {
		return fmt.Errorf("unsupported projection contract")
	}
	if !validProjectionKind(i.Kind) || !boundedToken(i.EffectID, 128) || i.Limit < 1 || i.Limit > MaxProjectionHistory {
		return fmt.Errorf("invalid projection history selector")
	}
	return nil
}

func DecodeProjectionRequest(data []byte) (ProjectionRequest, error) {
	var r ProjectionRequest
	if len(data) > MaxProjectionRequestBytes {
		return r, fmt.Errorf("projection request exceeds limit")
	}
	meta, err := decodeReferenceEnvelope(data, &r.Input, []string{"contract_version", "kind", "effect_id", "source_revision", "occurred_at", "reported_outcome", "session_run_id", "turn_id", "context_digest", "context_reference", "references", "feedback", "provenance"}, []string{"contract_version", "kind", "effect_id", "source_revision", "occurred_at", "reported_outcome"}, true)
	r.Metadata = meta
	if err != nil {
		return r, err
	}
	obj, _ := CanonicalObject(data, []string{"metadata", "input"}, []string{"metadata", "input"})
	if err = validateProjectionWireMetadata(obj["metadata"], meta); err != nil {
		return r, err
	}
	// decodeReferenceEnvelope checked the outer input; inspect nested objects
	// separately because encoding/json alone accepts aliases and unknown fields.
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(obj["input"], &raw); err != nil {
		return r, err
	}
	for _, field := range []string{"session_run_id", "turn_id", "context_digest", "context_reference"} {
		if value, present := raw[field]; present {
			var text string
			if err = json.Unmarshal(value, &text); err != nil || text == "" {
				return r, fmt.Errorf("invalid projection %s", field)
			}
		}
	}
	if value, ok := raw["references"]; ok {
		var refs []json.RawMessage
		if err = json.Unmarshal(value, &refs); err != nil {
			return r, err
		}
		for _, ref := range refs {
			if _, err = CanonicalObject(ref, []string{"event_id"}, []string{"event_id"}); err != nil {
				return r, err
			}
		}
	}
	if value, ok := raw["feedback"]; ok {
		if _, err = CanonicalObject(value, []string{"signal", "reason_code"}, []string{"signal", "reason_code"}); err != nil {
			return r, err
		}
	}
	return r, r.Input.Validate()
}
func DecodeProjectionHistoryRequest(data []byte) (ProjectionHistoryRequest, error) {
	var r ProjectionHistoryRequest
	if len(data) > MaxProjectionRequestBytes {
		return r, fmt.Errorf("projection request exceeds limit")
	}
	meta, err := decodeReferenceEnvelope(data, &r.Input, []string{"contract_version", "kind", "effect_id", "limit"}, []string{"contract_version", "kind", "effect_id", "limit"}, false)
	r.Metadata = meta
	if err != nil {
		return r, err
	}
	obj, _ := CanonicalObject(data, []string{"metadata", "input"}, []string{"metadata", "input"})
	if err = validateProjectionWireMetadata(obj["metadata"], meta); err != nil {
		return r, err
	}
	return r, r.Input.Validate()
}

// PC6 requires canonical metadata on the wire. The shared legacy envelope
// still normalizes its metadata; compare the original value only for PC6.
func validateProjectionWireMetadata(data []byte, normalized OperationMetadata) error {
	var original OperationMetadata
	if err := json.Unmarshal(data, &original); err != nil {
		return err
	}
	if original != normalized {
		return fmt.Errorf("projection metadata must be canonical")
	}
	return nil
}

func ProjectionScope(b RuntimeBinding) CanonicalScopeDescriptor {
	return CanonicalScopeDescriptor{Tenant: b.Scope.Tenant, Project: b.Scope.Project, Namespace: b.Scope.Namespace, AgentID: b.AgentID, SessionID: b.SessionID, ConversationID: b.ConversationID, ProviderInstanceID: b.ProviderInstanceID}
}
