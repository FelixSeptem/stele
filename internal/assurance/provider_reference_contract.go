package assurance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/openapi"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
)

const DurableReferenceContractVersion = "durable-reference-v1"

type ReferenceConsumerResponse struct {
	Metadata        provider.OperationMetadata        `json:"metadata"`
	ContractVersion string                            `json:"contract_version"`
	Scope           provider.CanonicalScopeDescriptor `json:"scope"`
	Status          string                            `json:"status"`
	EventID         string                            `json:"event_id,omitempty"`
	Replayed        bool                              `json:"replayed,omitempty"`
	Reference       *ReferenceConsumerRecord          `json:"reference,omitempty"`
	RetryAfterMS    int                               `json:"retry_after_ms"`
	Retryable       bool                              `json:"retryable"`
	ErrorCategory   string                            `json:"error_category,omitempty"`
}

type ReferenceConsumerRecord struct {
	ReferenceID  string  `json:"reference_id"`
	MemoryPath   string  `json:"memory_path"`
	EventType    string  `json:"event_type"`
	EventID      string  `json:"event_id"`
	CandidateID  string  `json:"candidate_id"`
	MemoryID     string  `json:"memory_id"`
	VersionID    string  `json:"version_id"`
	ProvenanceID string  `json:"provenance_id"`
	Version      int     `json:"version"`
	Value        *string `json:"value,omitempty"`
}

type providerReferenceContract struct {
	responseSchema *openapi3.Schema
	digest         string
}

func NewProviderReferenceContract(spec []byte) (*providerReferenceContract, error) {
	doc, err := openapi3.NewLoader().LoadFromData(spec)
	if err != nil || doc == nil {
		return nil, fmt.Errorf("provider reference OpenAPI unavailable")
	}
	if err = doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("provider reference OpenAPI invalid")
	}
	for _, route := range []string{"/v1/provider/references", "/v1/provider/references/status", "/v1/provider/references/lookup"} {
		path := doc.Paths.Value(route)
		if path == nil || path.Post == nil {
			return nil, fmt.Errorf("provider reference route missing")
		}
		code := "200"
		if route == "/v1/provider/references" {
			code = "202"
		}
		response := path.Post.Responses.Value(code)
		if response == nil || response.Value == nil {
			return nil, fmt.Errorf("provider reference response schema missing")
		}
		media := response.Value.Content.Get("application/json")
		if media == nil || media.Schema == nil || media.Schema.Ref != "#/components/schemas/ProviderReferenceResponse" || media.Schema.Value == nil {
			return nil, fmt.Errorf("provider reference response schema missing")
		}
	}
	resp := doc.Components.Schemas["ProviderReferenceResponse"]
	if resp == nil || resp.Value == nil || resp.Value.AdditionalProperties.Has == nil || *resp.Value.AdditionalProperties.Has {
		return nil, fmt.Errorf("provider reference response schema missing")
	}
	sum := sha256.Sum256(spec)
	return &providerReferenceContract{responseSchema: resp.Value, digest: hex.EncodeToString(sum[:])}, nil
}

func DefaultProviderReferenceContract() (*providerReferenceContract, error) {
	return NewProviderReferenceContract([]byte(openapi.SpecYAML()))
}

func (c *providerReferenceContract) Validate(data []byte, scope provider.CanonicalScopeDescriptor, schemaVersion, responseMode string) (ReferenceConsumerResponse, error) {
	var out ReferenceConsumerResponse
	fail := func() (ReferenceConsumerResponse, error) {
		return ReferenceConsumerResponse{}, fmt.Errorf("provider reference failed-contract")
	}
	if c == nil || c.responseSchema == nil || scope.Validate() != nil || len(data) == 0 || len(data) > 16384 || strings.TrimSpace(string(data)) != string(data) || !provider.ValidJSONUnicode(data) || !uniqueJSON(data) {
		return fail()
	}
	var value any
	if json.Unmarshal(data, &value) != nil || c.responseSchema.VisitJSON(value) != nil || provider.DecodeStrict(data, &out) != nil {
		return fail()
	}
	if out.Metadata.NormalizeValidate() != nil || out.Metadata.SchemaVersion != schemaVersion || out.ContractVersion != DurableReferenceContractVersion || out.Scope != scope {
		return fail()
	}
	if out.Status != "accepted" && out.Status != "pending" && out.Status != "failed" && out.Status != "completed" && out.Status != "unavailable" {
		return fail()
	}
	if out.RetryAfterMS < 0 || out.RetryAfterMS > 30000 || (out.Retryable && out.Status != "accepted" && out.Status != "pending" && out.Status != "failed") || out.ErrorCategory != "" && out.ErrorCategory != "retryable" && out.ErrorCategory != "dependency" {
		return fail()
	}
	if out.Status == "completed" {
		if out.Reference == nil || (out.EventID != "" && out.EventID != out.Reference.EventID) || out.ErrorCategory != "" || !validReferenceRecord(*out.Reference) {
			return fail()
		}
	} else if out.Reference != nil {
		return fail()
	}
	if responseMode == "reference_only" && out.Reference != nil && out.Reference.Value != nil {
		return fail()
	}
	if out.EventID != "" && !validUUID(out.EventID) {
		return fail()
	}
	if out.Status == "unavailable" && (out.EventID != "" || out.Replayed || out.Retryable || out.RetryAfterMS != 0) {
		return fail()
	}
	if out.Status == "accepted" && !validUUID(out.EventID) {
		return fail()
	}
	if responseMode != "reference_only" && responseMode != "bounded_content" {
		return fail()
	}
	return out, nil
}

func validReferenceRecord(r ReferenceConsumerRecord) bool {
	if r.ReferenceID == "" || len([]byte(r.ReferenceID)) > 128 || strings.TrimSpace(r.ReferenceID) != r.ReferenceID || r.MemoryPath == "" || len([]byte(r.MemoryPath)) > 512 || strings.HasPrefix(r.MemoryPath, "/") || strings.HasSuffix(r.MemoryPath, "/") || strings.Contains(r.MemoryPath, "//") || r.Version < 1 {
		return false
	}
	for _, ch := range r.ReferenceID {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("-_.:", ch)) {
			return false
		}
	}
	if normalized, err := memory.NormalizeMemoryPath(r.MemoryPath); err != nil || normalized != r.MemoryPath {
		return false
	}
	allowed := map[string]bool{"provider.reference.task": true, "provider.reference.run": true, "provider.reference.handoff": true, "provider.reference.compilation": true}
	if !allowed[r.EventType] {
		return false
	}
	for _, id := range []string{r.EventID, r.CandidateID, r.MemoryID, r.VersionID, r.ProvenanceID} {
		if !validUUID(id) {
			return false
		}
	}
	if r.Value != nil && len([]byte(*r.Value)) > 4096 {
		return false
	}
	return true
}
func validUUID(value string) bool {
	u, e := uuid.Parse(value)
	return e == nil && u != uuid.Nil && u.String() == value
}

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

func ValidateProviderReferenceCapability(data []byte) error {
	var c struct {
		DurableReferences DurableReferenceCapabilities `json:"durable_references"`
	}
	if !provider.ValidJSONUnicode(data) || !uniqueJSON(data) || provider.DecodeStrict(data, &c) != nil {
		return fmt.Errorf("provider reference capability failed-contract")
	}
	x := c.DurableReferences
	if !x.Enabled || x.ContractVersion != DurableReferenceContractVersion || x.MaxValueBytes != 4096 || x.MaxReferenceIDBytes != 128 || x.MaxPathBytes != 512 || x.MaxResponseBytes != 16384 || x.PollAfterMS != 1000 {
		return fmt.Errorf("provider reference capability failed-contract")
	}
	if len(x.EventTypes) != 4 || len(x.Statuses) != 5 {
		return fmt.Errorf("provider reference capability failed-contract")
	}
	for _, pair := range []struct{ values, required []string }{{x.EventTypes, []string{"provider.reference.task", "provider.reference.run", "provider.reference.handoff", "provider.reference.compilation"}}, {x.Statuses, []string{"accepted", "pending", "failed", "completed", "unavailable"}}} {
		seen := map[string]bool{}
		for _, v := range pair.values {
			if seen[v] {
				return fmt.Errorf("provider reference capability failed-contract")
			}
			seen[v] = true
		}
		for _, v := range pair.required {
			if !seen[v] {
				return fmt.Errorf("provider reference capability failed-contract")
			}
		}
	}
	return nil
}
