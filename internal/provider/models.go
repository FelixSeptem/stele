package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	SchemaVersionV1           = "provider-v1"
	MaxMetadataFieldBytes     = 128
	MaxCitationReferenceBytes = 256
)

type RuntimeMetadata struct {
	ServiceVersion, BuildID, BuildTimestamp string
	SchemaVersion                           int64
}

type Limits struct {
	EventBytes     int `json:"event_bytes"`
	IntentBytes    int `json:"intent_bytes"`
	RetrievalItems int `json:"retrieval_items"`
	ContextItems   int `json:"context_items"`
	CitationItems  int `json:"citation_items"`
	MetadataBytes  int `json:"metadata_bytes"`
}

type CanonicalScope struct {
	Dimensions []string `json:"dimensions"`
	Exact      bool     `json:"exact"`
}

type OperationMetadata struct {
	RequestID      string `json:"request_id,omitempty"`
	OperationID    string `json:"operation_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	EventSeq       int64  `json:"event_seq,omitempty"`
	SchemaVersion  string `json:"schema_version"`
}

func (m OperationMetadata) Validate() error {
	for name, v := range map[string]string{"request_id": m.RequestID, "operation_id": m.OperationID, "idempotency_key": m.IdempotencyKey, "schema_version": m.SchemaVersion} {
		if len(v) > MaxMetadataFieldBytes {
			return fmt.Errorf("%s exceeds %d bytes", name, MaxMetadataFieldBytes)
		}
		if strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("%s contains invalid characters", name)
		}
	}
	if m.EventSeq < 0 {
		return fmt.Errorf("event_seq must be non-negative")
	}
	if m.SchemaVersion == "" {
		return fmt.Errorf("schema_version is required")
	}
	if m.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("unsupported schema_version %q", m.SchemaVersion)
	}
	return nil
}

type Citation struct {
	SourceKind   string `json:"source_kind"`
	Reference    string `json:"reference"`
	Version      int64  `json:"version,omitempty"`
	Watermark    string `json:"watermark,omitempty"`
	Availability string `json:"availability"`
}

func (c Citation) Validate() error {
	if len(c.Reference) > MaxCitationReferenceBytes {
		return fmt.Errorf("citation reference exceeds limit")
	}
	if c.SourceKind == "" || c.Availability == "" {
		return fmt.Errorf("citation source_kind and availability are required")
	}
	return nil
}

type Result struct {
	SchemaVersion string     `json:"schema_version"`
	Disposition   string     `json:"disposition"`
	Data          any        `json:"data,omitempty"`
	Citations     []Citation `json:"citations,omitempty"`
}
type BoundedError struct {
	Category  string `json:"category"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable,omitempty"`
}

func (r Result) Validate() error {
	if r.SchemaVersion != SchemaVersionV1 || strings.TrimSpace(r.Disposition) == "" {
		return fmt.Errorf("result schema_version and disposition are required")
	}
	for _, citation := range r.Citations {
		if err := citation.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (e BoundedError) Validate() error {
	if strings.TrimSpace(e.Category) == "" || strings.TrimSpace(e.Code) == "" || strings.TrimSpace(e.Message) == "" {
		return fmt.Errorf("error category, code, and message are required")
	}
	if len(e.Category) > 64 || len(e.Code) > 64 || len(e.Message) > 512 {
		return fmt.Errorf("error fields exceed limits")
	}
	return nil
}

type CapabilityDocument struct {
	ProviderVersion      string         `json:"provider_version"`
	SchemaVersion        string         `json:"schema_version"`
	RuntimeSchemaVersion int64          `json:"runtime_schema_version"`
	ServiceVersion       string         `json:"service_version"`
	BuildID              string         `json:"build_id"`
	BuildTimestamp       string         `json:"build_timestamp"`
	SchemaDigest         string         `json:"schema_digest"`
	Operations           []string       `json:"operations"`
	Scope                CanonicalScope `json:"scope"`
	Limits               Limits         `json:"limits"`
}
type CapabilityConfig struct{ Limits Limits }

func DiscoverCapabilities(meta RuntimeMetadata, cfg CapabilityConfig) CapabilityDocument {
	sv, bid, bt := safeRuntimeMetadata(meta.ServiceVersion), safeRuntimeMetadata(meta.BuildID), safeRuntimeMetadata(meta.BuildTimestamp)
	if cfg.Limits.EventBytes == 0 {
		cfg.Limits = Limits{EventBytes: 1 << 20, IntentBytes: 64 << 10, RetrievalItems: 100, ContextItems: 100, CitationItems: 100, MetadataBytes: MaxMetadataFieldBytes}
	}
	digest := sha256.Sum256([]byte(SchemaVersionV1))
	return CapabilityDocument{ProviderVersion: "stele-provider-v1", SchemaVersion: SchemaVersionV1, RuntimeSchemaVersion: meta.SchemaVersion, ServiceVersion: sv, BuildID: bid, BuildTimestamp: bt, SchemaDigest: fmt.Sprintf("sha256:%x", digest), Operations: []string{"event.ingest", "memory.intent", "memory.retrieve", "context.assemble", "memory.forget", "status.read"}, Scope: CanonicalScope{Dimensions: []string{"tenant", "project", "namespace"}, Exact: true}, Limits: cfg.Limits}
}

func safeRuntimeMetadata(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 || strings.ContainsAny(value, "\r\n") || strings.Contains(value, "://") {
		return "unknown"
	}
	return value
}

func DecodeJSON(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return err
	}
	switch m := dst.(type) {
	case *OperationMetadata:
		return m.Validate()
	case *Citation:
		return m.Validate()
	case *Result:
		return m.Validate()
	case *BoundedError:
		return m.Validate()
	}
	return nil
}
