package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	MaxProviderVersionBytes = 64
	MaxSchemaVersionBytes   = 64
	MaxOperationNameBytes   = 64
	MaxScopeDimensionBytes  = 32
	MaxScopeDimensions      = 16
	MaxOperations           = 32
	MaxSchemaDigestBytes    = 128
	MaxErrorMessageBytes    = 256
	MaxErrorCodeBytes       = 64
)

type CanonicalScopeDescriptor struct {
	Tenant             string `json:"tenant"`
	Project            string `json:"project"`
	Namespace          string `json:"namespace"`
	AgentID            string `json:"agent_id"`
	SessionID          string `json:"session_id"`
	ConversationID     string `json:"conversation_id,omitempty"`
	ProviderInstanceID string `json:"provider_instance_id"`
}

func (s CanonicalScopeDescriptor) Validate() error {
	for _, v := range []string{s.Tenant, s.Project, s.Namespace, s.AgentID, s.SessionID, s.ConversationID, s.ProviderInstanceID} {
		if v != "" && !boundedToken(v, 128) {
			return fmt.Errorf("scope value is invalid")
		}
	}
	if s.Tenant == "" || s.Project == "" || s.Namespace == "" || s.AgentID == "" || s.SessionID == "" || s.ProviderInstanceID == "" {
		return fmt.Errorf("scope is incomplete")
	}
	return nil
}

type ResultEnvelope struct {
	SchemaVersion string                   `json:"schema_version"`
	Scope         CanonicalScopeDescriptor `json:"scope"`
	Result        json.RawMessage          `json:"result"`
	Citations     []Citation               `json:"citations,omitempty"`
	Error         *ProviderError           `json:"error,omitempty"`
}

func (r ResultEnvelope) Validate() error {
	if !boundedToken(r.SchemaVersion, MaxSchemaVersionBytes) {
		return fmt.Errorf("schema_version is invalid")
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if len(r.Result) > 1<<20 {
		return fmt.Errorf("result exceeds limit")
	}
	if len(r.Citations) > 1000 {
		return fmt.Errorf("citations exceed limit")
	}
	for _, c := range r.Citations {
		if err := c.Validate(); err != nil {
			return err
		}
	}
	if r.Error != nil {
		return r.Error.Validate()
	}
	return nil
}

type Citation struct {
	SourceKind   string `json:"source_kind"`
	Reference    string `json:"reference"`
	Version      string `json:"version,omitempty"`
	Watermark    string `json:"watermark,omitempty"`
	Availability string `json:"availability"`
}

func (c Citation) Validate() error {
	if !boundedToken(c.SourceKind, 64) || !boundedToken(c.Reference, 128) || !boundedToken(c.Availability, 32) {
		return fmt.Errorf("citation is invalid")
	}
	if c.Version != "" && !boundedToken(c.Version, 64) {
		return fmt.Errorf("citation version is invalid")
	}
	if c.Watermark != "" && !boundedToken(c.Watermark, 128) {
		return fmt.Errorf("citation watermark is invalid")
	}
	return nil
}

type ErrorCategory string

const (
	ErrorCategoryAuthentication ErrorCategory = "authentication"
	ErrorCategoryScope          ErrorCategory = "scope"
	ErrorCategoryCompatibility  ErrorCategory = "compatibility"
	ErrorCategoryValidation     ErrorCategory = "validation"
	ErrorCategoryConflict       ErrorCategory = "conflict"
	ErrorCategoryLifecycle      ErrorCategory = "lifecycle"
	ErrorCategoryStale          ErrorCategory = "stale"
	ErrorCategoryDependency     ErrorCategory = "dependency"
	ErrorCategoryRetryable      ErrorCategory = "retryable"
)

type ProviderError struct {
	Category          ErrorCategory `json:"category"`
	Code              string        `json:"code"`
	Message           string        `json:"message"`
	Retryable         bool          `json:"retryable"`
	SupportedVersions []string      `json:"supported_versions,omitempty"`
}

func (e ProviderError) Validate() error {
	if e.Category == "" || !boundedToken(string(e.Category), 32) || !boundedToken(e.Code, MaxErrorCodeBytes) || e.Message == "" || len(e.Message) > MaxErrorMessageBytes {
		return fmt.Errorf("provider error is invalid")
	}
	if len(e.SupportedVersions) > 16 {
		return fmt.Errorf("supported versions exceed limit")
	}
	for _, v := range e.SupportedVersions {
		if !boundedToken(v, MaxSchemaVersionBytes) {
			return fmt.Errorf("supported version is invalid")
		}
	}
	return nil
}

type ProviderLimits struct {
	MaxEventBytes       int `json:"max_event_bytes"`
	MaxIntentBytes      int `json:"max_intent_bytes"`
	MaxRetrievalResults int `json:"max_retrieval_results"`
	MaxContextBytes     int `json:"max_context_bytes"`
	MaxCitations        int `json:"max_citations"`
	MaxMetadataBytes    int `json:"max_metadata_bytes"`
}

func (l ProviderLimits) Validate() error {
	if l.MaxEventBytes <= 0 || l.MaxEventBytes > 1<<20 || l.MaxIntentBytes <= 0 || l.MaxIntentBytes > 1<<20 || l.MaxRetrievalResults <= 0 || l.MaxRetrievalResults > 1000 || l.MaxContextBytes <= 0 || l.MaxContextBytes > 4<<20 || l.MaxCitations <= 0 || l.MaxCitations > 1000 || l.MaxMetadataBytes <= 0 || l.MaxMetadataBytes > 1<<20 {
		return fmt.Errorf("provider limits are invalid")
	}
	return nil
}

type CapabilityDocument struct {
	ProviderVersion string         `json:"provider_version"`
	SchemaVersion   string         `json:"schema_version"`
	ServiceVersion  string         `json:"service_version,omitempty"`
	BuildID         string         `json:"build_id,omitempty"`
	Operations      []string       `json:"operations"`
	ScopeDimensions []string       `json:"scope_dimensions"`
	Limits          ProviderLimits `json:"limits"`
	SchemaDigest    string         `json:"schema_digest"`
}

func (d CapabilityDocument) Validate() error {
	if !boundedToken(d.ProviderVersion, MaxProviderVersionBytes) || !boundedToken(d.SchemaVersion, MaxSchemaVersionBytes) || !boundedToken(d.SchemaDigest, MaxSchemaDigestBytes) {
		return fmt.Errorf("provider capability version fields are invalid")
	}
	if len(d.Operations) == 0 || len(d.Operations) > MaxOperations {
		return fmt.Errorf("provider operations are invalid")
	}
	for _, op := range d.Operations {
		if !boundedToken(op, MaxOperationNameBytes) {
			return fmt.Errorf("provider operation is invalid")
		}
	}
	if len(d.ScopeDimensions) == 0 || len(d.ScopeDimensions) > MaxScopeDimensions {
		return fmt.Errorf("provider scope dimensions are invalid")
	}
	for _, dim := range d.ScopeDimensions {
		if !boundedToken(dim, MaxScopeDimensionBytes) {
			return fmt.Errorf("provider scope dimension is invalid")
		}
	}
	return d.Limits.Validate()
}

type CapabilityInput struct{ ProviderVersion, SchemaVersion, ServiceVersion, BuildID, SchemaDigest string }

func Discover(in CapabilityInput) CapabilityDocument {
	d := CapabilityDocument{ProviderVersion: in.ProviderVersion, SchemaVersion: in.SchemaVersion, ServiceVersion: in.ServiceVersion, BuildID: in.BuildID, SchemaDigest: in.SchemaDigest,
		Operations: []string{"ingest", "intent", "retrieve", "context", "forget", "status"}, ScopeDimensions: []string{"tenant", "project", "namespace", "agent", "session", "conversation", "provider_instance"},
		Limits: ProviderLimits{MaxEventBytes: 1 << 20, MaxIntentBytes: 1 << 20, MaxRetrievalResults: 100, MaxContextBytes: 1 << 20, MaxCitations: 100, MaxMetadataBytes: 64 << 10}}
	if strings.TrimSpace(d.ProviderVersion) == "" {
		d.ProviderVersion = "unknown"
	}
	if strings.TrimSpace(d.SchemaVersion) == "" {
		d.SchemaVersion = "schema-v1"
	}
	if strings.TrimSpace(d.ServiceVersion) == "" {
		d.ServiceVersion = "unknown"
	}
	if strings.TrimSpace(d.BuildID) == "" {
		d.BuildID = "unknown"
	}
	if strings.TrimSpace(d.SchemaDigest) == "" {
		d.SchemaDigest = "unknown"
	}
	return d
}

func DecodeStrict(data []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON data")
		}
		return err
	}
	return nil
}
func boundedToken(v string, max int) bool {
	if v == "" || len(v) > max {
		return false
	}
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("-_.:", r) {
			continue
		}
		return false
	}
	return true
}
