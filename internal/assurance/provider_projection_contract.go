package assurance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/getkin/kin-openapi/openapi3"
)

const ProjectionContractVersion = "turn-outcome-feedback-projection-v1"

type providerProjectionContract struct {
	schemas map[string]*openapi3.Schema
	digest  string
}

func NewProviderProjectionContract(spec []byte) (*providerProjectionContract, error) {
	doc, err := openapi3.NewLoader().LoadFromData(spec)
	if err != nil || doc == nil {
		return nil, fmt.Errorf("provider projection OpenAPI unavailable")
	}
	if err = doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("provider projection OpenAPI invalid")
	}
	for _, op := range []struct{ path, request, response, code, id string }{
		{"/v1/provider/projections", "ProviderProjectionRequest", "ProviderProjectionResponse", "202", "providerAdmitProjection"},
		{"/v1/provider/projections/history", "ProviderProjectionHistoryRequest", "ProviderProjectionHistoryResponse", "200", "providerProjectionHistory"},
	} {
		path := doc.Paths.Value(op.path)
		if path == nil || path.Post == nil || path.Post.OperationID != op.id || path.Post.RequestBody == nil || path.Post.RequestBody.Value == nil {
			return nil, fmt.Errorf("provider projection route missing")
		}
		request := path.Post.RequestBody.Value.Content.Get("application/json")
		response := path.Post.Responses.Value(op.code)
		if request == nil || request.Schema == nil || request.Schema.Ref != "#/components/schemas/"+op.request || response == nil || response.Value == nil {
			return nil, fmt.Errorf("provider projection schema missing")
		}
		media := response.Value.Content.Get("application/json")
		if media == nil || media.Schema == nil || media.Schema.Ref != "#/components/schemas/"+op.response {
			return nil, fmt.Errorf("provider projection schema missing")
		}
		for _, status := range []string{"400", "403", "409", "410", "413", "503"} {
			response := path.Post.Responses.Value(status)
			if response == nil || response.Value == nil {
				return nil, fmt.Errorf("provider projection error schema missing")
			}
			media := response.Value.Content.Get("application/json")
			if media == nil || media.Schema == nil || media.Schema.Ref != "#/components/schemas/ProviderProjectionErrorResponse" {
				return nil, fmt.Errorf("provider projection error schema missing")
			}
		}
	}
	c := &providerProjectionContract{schemas: map[string]*openapi3.Schema{}}
	for _, name := range []string{"ProviderCapabilityDocument", "ProviderProjectionOperationMetadata", "ProviderProjectionErrorResponse", "ProviderProjectionError", "ProviderProjectionRequest", "ProviderProjectionResponse", "ProviderProjectionHistoryRequest", "ProviderProjectionHistoryResponse", "ProviderProjectionCapabilities", "ProviderProjectionInput", "ProviderProjectionHistoryInput", "ProviderProjectionReferenceSelector", "ProviderProjectionFeedback", "ProviderProjectionLineage", "ProviderProjectionAttribution", "ProviderProjectionRecord"} {
		ref := doc.Components.Schemas[name]
		if ref == nil || ref.Value == nil || ref.Value.AdditionalProperties.Has == nil || *ref.Value.AdditionalProperties.Has {
			return nil, fmt.Errorf("provider projection closed schema missing")
		}
		c.schemas[name] = ref.Value
	}
	sum := sha256.Sum256(spec)
	c.digest = hex.EncodeToString(sum[:])
	return c, nil
}

func projectionContractFailure() error { return fmt.Errorf("provider projection failed-contract") }

// ValidateError checks the serialized PC6 error union and its HTTP disposition.
// Legacy error categories/codes are never a projection compatibility fallback.
func (c *providerProjectionContract) ValidateError(data []byte, status int) (provider.ProviderError, error) {
	var out struct {
		Error provider.ProviderError `json:"error"`
	}
	if c.decode(data, "ProviderProjectionErrorResponse", 4096, &out) != nil || out.Error.Message == "" || len(out.Error.Message) > 256 {
		return provider.ProviderError{}, projectionContractFailure()
	}
	var expectedStatus int
	var expectedCategory provider.ErrorCategory
	switch out.Error.Code {
	case "invalid_request", "invalid_scope", "invalid_projection_scope":
		expectedStatus, expectedCategory = 400, provider.ErrorCategoryValidation
	case "unauthorized":
		expectedStatus, expectedCategory = 403, provider.ErrorCategoryAuthentication
	case "scope_denied", "runtime_binding_denied", "disclosure_denied":
		expectedStatus, expectedCategory = 403, provider.ErrorCategoryScope
	case "unsupported_projection_contract", "unsupported_schema":
		expectedStatus, expectedCategory = 409, provider.ErrorCategoryCompatibility
	case "projection_conflict":
		expectedStatus, expectedCategory = 409, provider.ErrorCategoryConflict
	case "projection_stale":
		expectedStatus, expectedCategory = 409, provider.ErrorCategoryStale
	case "projection_unavailable":
		expectedStatus, expectedCategory = 410, provider.ErrorCategoryLifecycle
	case "projection_limit_exceeded":
		expectedStatus, expectedCategory = 413, provider.ErrorCategoryValidation
	case "projection_dependency_unavailable", "authorization_unavailable", "disclosure_unavailable":
		expectedStatus, expectedCategory = 503, provider.ErrorCategoryDependency
	default:
		return provider.ProviderError{}, projectionContractFailure()
	}
	if status != expectedStatus || out.Error.Category != expectedCategory || out.Error.Retryable != (expectedCategory == provider.ErrorCategoryDependency) {
		return provider.ProviderError{}, projectionContractFailure()
	}
	return out.Error, nil
}

func (c *providerProjectionContract) decode(data []byte, name string, limit int, out any) error {
	if c == nil || c.schemas[name] == nil || len(data) == 0 || len(data) > limit || !provider.ValidJSONUnicode(data) || !uniqueJSON(data) {
		return projectionContractFailure()
	}
	var value any
	if json.Unmarshal(data, &value) != nil || c.schemas[name].VisitJSON(value) != nil || provider.DecodeStrict(data, out) != nil {
		return projectionContractFailure()
	}
	return nil
}

// NegotiateDocument validates the complete served capability document before
// explicitly extracting the dedicated projections capability.
func (c *providerProjectionContract) NegotiateDocument(data []byte) error {
	var document map[string]json.RawMessage
	if c.decode(data, "ProviderCapabilityDocument", 65536, &document) != nil {
		return projectionContractFailure()
	}
	extracted, err := json.Marshal(map[string]json.RawMessage{"projections": document["projections"]})
	if err != nil {
		return projectionContractFailure()
	}
	return c.Negotiate(extracted)
}

// Negotiate accepts only the extracted projections capability, never a legacy fallback.
func (c *providerProjectionContract) Negotiate(data []byte) error {
	var outer map[string]json.RawMessage
	if !provider.ValidJSONUnicode(data) || !uniqueJSON(data) || json.Unmarshal(data, &outer) != nil || len(outer) != 1 || outer["projections"] == nil {
		return projectionContractFailure()
	}
	var x struct {
		Enabled bool `json:"enabled"`
	}
	var value map[string]any
	if c == nil || c.schemas["ProviderProjectionCapabilities"] == nil || json.Unmarshal(outer["projections"], &value) != nil || c.schemas["ProviderProjectionCapabilities"].VisitJSON(value) != nil || json.Unmarshal(outer["projections"], &x) != nil || !x.Enabled {
		return projectionContractFailure()
	}
	return nil
}

func projectionConsumerMetadata(meta provider.OperationMetadata, version string) bool {
	original := meta
	encoded, err := json.Marshal(meta)
	return err == nil && len(encoded) <= 4096 && meta.NormalizeValidate() == nil && meta == original && meta.SchemaVersion == version
}

func (c *providerProjectionContract) ValidateRequest(data []byte, version string) (provider.ProjectionRequest, error) {
	var out provider.ProjectionRequest
	if c.decode(data, "ProviderProjectionRequest", 32768, &out) != nil || !projectionConsumerMetadata(out.Metadata, version) || out.Metadata.IdempotencyKey == "" || !validProjectionConsumerInput(out.Input) {
		return provider.ProjectionRequest{}, projectionContractFailure()
	}
	return out, nil
}

func (c *providerProjectionContract) ValidateHistoryRequest(data []byte, version string) (provider.ProjectionHistoryRequest, error) {
	var out provider.ProjectionHistoryRequest
	if c.decode(data, "ProviderProjectionHistoryRequest", 32768, &out) != nil || !projectionConsumerMetadata(out.Metadata, version) || out.Input.ContractVersion != ProjectionContractVersion || out.Input.Limit < 1 || out.Input.Limit > 8 {
		return provider.ProjectionHistoryRequest{}, projectionContractFailure()
	}
	return out, nil
}

// Validate semantic relationships independently of producer admission helpers.
func validProjectionConsumerInput(input provider.ProjectionInput) bool {
	at, err := time.Parse(time.RFC3339, input.OccurredAt)
	if input.ContractVersion != ProjectionContractVersion || err != nil || at.IsZero() || at.UTC().Format(time.RFC3339) != input.OccurredAt || input.TurnID != "" && input.SessionRunID == "" || input.ContextReference != "" && input.ContextDigest == "" {
		return false
	}
	if input.Kind == "feedback" && input.Feedback == nil || input.Feedback != nil && len(input.References) == 0 {
		return false
	}
	return true
}

func (c *providerProjectionContract) Validate(data []byte, scope provider.CanonicalScopeDescriptor, version string) (provider.ProjectionResponse, error) {
	var out provider.ProjectionResponse
	if c.decode(data, "ProviderProjectionResponse", 65536, &out) != nil || scope.Validate() != nil || out.Scope != scope || out.ContractVersion != ProjectionContractVersion || !projectionConsumerMetadata(out.Metadata, version) || !validProjectionConsumerRecord(out.Record) {
		return provider.ProjectionResponse{}, projectionContractFailure()
	}
	return out, nil
}

func (c *providerProjectionContract) ValidateHistory(data []byte, scope provider.CanonicalScopeDescriptor, version string, limit int) (provider.ProjectionHistoryResponse, error) {
	var out provider.ProjectionHistoryResponse
	if c.decode(data, "ProviderProjectionHistoryResponse", 65536, &out) != nil || scope.Validate() != nil || out.Scope != scope || out.ContractVersion != ProjectionContractVersion || !projectionConsumerMetadata(out.Metadata, version) || limit < 1 || limit > 8 || len(out.Records) > limit || (out.HasMore && len(out.Records) != limit) {
		return provider.ProjectionHistoryResponse{}, projectionContractFailure()
	}
	seen := map[string]bool{}
	for n, r := range out.Records {
		if !validProjectionConsumerRecord(r) || seen[r.ProjectionID] || n > 0 && (r.LogicalID != out.Records[0].LogicalID || r.SourceRevision >= out.Records[n-1].SourceRevision || out.Records[n-1].SupersedesID != r.ProjectionID) {
			return provider.ProjectionHistoryResponse{}, projectionContractFailure()
		}
		seen[r.ProjectionID] = true
	}
	if len(out.Records) > 0 && (out.HasMore && out.Records[len(out.Records)-1].Disposition != "corrected" || !out.HasMore && out.Records[len(out.Records)-1].Disposition != "accepted") {
		return provider.ProjectionHistoryResponse{}, projectionContractFailure()
	}
	return out, nil
}

// These semantic gates deliberately do not call the producer's record validator.
func validProjectionConsumerRecord(r provider.ProjectionRecord) bool {
	if !validUUID(r.ProjectionID) || !validUUID(r.LogicalID) || r.SourceRevision < 1 || r.SourceRevision > 2147483647 || r.EvidenceKind != "runtime_reported_evidence" {
		return false
	}
	if r.Disposition == "accepted" {
		if r.SupersedesID != "" {
			return false
		}
	} else if r.Disposition != "corrected" || !validUUID(r.SupersedesID) || r.SupersedesID == r.ProjectionID {
		return false
	}
	if r.FeedbackState != "none" && r.FeedbackState != "pending" {
		return false
	}
	if r.ContextDigestAttribution != "none" && r.ContextDigestAttribution != "reported" && r.ContextDigestAttribution != "verified_receipt" {
		return false
	}
	if len(r.References) != len(r.CurrentReferences) {
		return false
	}
	if r.FeedbackState == "pending" && len(r.References) == 0 {
		return false
	}
	for n, ref := range r.CurrentReferences {
		snapshot := r.References[n]
		if ref.State == "unavailable" && snapshot.State != "unavailable" || ref.EventID != "" && snapshot.EventID != "" && ref.EventID != snapshot.EventID {
			return false
		}
	}
	for _, refs := range [][]provider.ProjectionReferenceAttribution{r.References, r.CurrentReferences} {
		if refs == nil || len(refs) > 8 {
			return false
		}
		seen := map[string]bool{}
		for _, ref := range refs {
			if ref.State == "unavailable" {
				if ref.EventID != "" || ref.Eligible || ref.Lineage != nil {
					return false
				}
				continue
			}
			if !validUUID(ref.EventID) || seen[ref.EventID] {
				return false
			}
			seen[ref.EventID] = true
			switch ref.State {
			case "accepted", "pending", "failed":
				if ref.Eligible || ref.Lineage != nil {
					return false
				}
			case "completed":
				if !ref.Eligible || ref.Lineage == nil || ref.Lineage.Version < 1 {
					return false
				}
				for _, id := range []string{ref.Lineage.CandidateID, ref.Lineage.MemoryID, ref.Lineage.VersionID, ref.Lineage.ProvenanceID} {
					if !validUUID(id) {
						return false
					}
				}
			default:
				return false
			}
		}
	}
	return true
}
