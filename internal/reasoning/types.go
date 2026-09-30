package reasoning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

const SchemaVersionV1 = "reasoning-v1"

type Mode string

const (
	ModeDisabled Mode = "disabled"
	ModeOffline  Mode = "offline"
	ModeShadow   Mode = "shadow"
	ModeLive     Mode = "live"
)

type Limits struct {
	MaxInputBytes      int `json:"max_input_bytes"`
	MaxOutputBytes     int `json:"max_output_bytes"`
	MaxEvidence        int `json:"max_evidence"`
	MaxMetadataBytes   int `json:"max_metadata_bytes"`
	MaxOperationBytes  int `json:"max_operation_bytes"`
	MaxConcurrent      int `json:"max_concurrent"`
	MaxDeadlineSeconds int `json:"max_deadline_seconds"`
}

func DefaultLimits() Limits {
	return Limits{MaxInputBytes: 64 << 10, MaxOutputBytes: 32 << 10, MaxEvidence: 100, MaxMetadataBytes: 16 << 10, MaxOperationBytes: 128, MaxConcurrent: 4, MaxDeadlineSeconds: 60}
}

func (l Limits) Validate() error {
	if l.MaxInputBytes <= 0 || l.MaxInputBytes > 1<<20 || l.MaxOutputBytes <= 0 || l.MaxOutputBytes > 1<<20 || l.MaxEvidence <= 0 || l.MaxEvidence > 1000 || l.MaxMetadataBytes <= 0 || l.MaxMetadataBytes > 1<<20 || l.MaxOperationBytes <= 0 || l.MaxOperationBytes > 1024 || l.MaxConcurrent <= 0 || l.MaxConcurrent > 128 || l.MaxDeadlineSeconds <= 0 || l.MaxDeadlineSeconds > 3600 {
		return fmt.Errorf("reasoning limits are invalid")
	}
	return nil
}

type CapabilityInput struct {
	ProviderVersion string
	Model           string
	ServiceVersion  string
	BuildID         string
	SchemaDigest    string
	Enabled         bool
	Mode            Mode
	Limits          Limits
}

type Capability struct {
	ProviderVersion string   `json:"provider_version"`
	Model           string   `json:"model,omitempty"`
	ServiceVersion  string   `json:"service_version"`
	BuildID         string   `json:"build_id"`
	SchemaVersion   string   `json:"schema_version"`
	SchemaDigest    string   `json:"schema_digest"`
	Enabled         bool     `json:"enabled"`
	Mode            Mode     `json:"mode"`
	Operations      []string `json:"operations"`
	Limits          Limits   `json:"limits"`
}

func Discover(in CapabilityInput) Capability {
	limits := in.Limits
	if limits.Validate() != nil {
		limits = DefaultLimits()
	}
	mode := in.Mode
	if !in.Enabled {
		mode = ModeDisabled
	}
	if mode == "" {
		mode = ModeDisabled
	}
	return Capability{ProviderVersion: fallback(in.ProviderVersion), Model: optionalBounded(in.Model, 256), ServiceVersion: fallback(in.ServiceVersion), BuildID: fallback(in.BuildID), SchemaVersion: SchemaVersionV1, SchemaDigest: fallback(in.SchemaDigest), Enabled: in.Enabled, Mode: mode, Operations: []string{"capability", "derive", "replay", "shadow", "conformance"}, Limits: limits}
}

func (c Capability) Validate() error {
	for _, v := range []string{c.ProviderVersion, c.ServiceVersion, c.BuildID, c.SchemaVersion, c.SchemaDigest} {
		if !bounded(v, 128) {
			return fmt.Errorf("reasoning capability version field is invalid")
		}
	}
	if c.Mode != ModeDisabled && c.Mode != ModeOffline && c.Mode != ModeShadow && c.Mode != ModeLive {
		return fmt.Errorf("reasoning mode is invalid")
	}
	if c.Enabled && c.Mode == ModeDisabled {
		return fmt.Errorf("enabled reasoning capability cannot be disabled")
	}
	if len(c.Operations) == 0 || len(c.Operations) > 16 {
		return fmt.Errorf("reasoning operations are invalid")
	}
	for _, op := range c.Operations {
		if !bounded(op, 128) {
			return fmt.Errorf("reasoning operation is invalid")
		}
	}
	return c.Limits.Validate()
}

type InvocationMetadata struct {
	RequestID       string `json:"request_id"`
	OperationID     string `json:"operation_id"`
	IdempotencyKey  string `json:"idempotency_key"`
	SchemaVersion   string `json:"schema_version"`
	PolicyVersion   string `json:"policy_version"`
	ProviderVersion string `json:"provider_version"`
}

func (m InvocationMetadata) Validate() error {
	for label, value := range map[string]string{"request_id": m.RequestID, "operation_id": m.OperationID, "idempotency_key": m.IdempotencyKey, "schema_version": m.SchemaVersion, "policy_version": m.PolicyVersion, "provider_version": m.ProviderVersion} {
		if !bounded(value, 256) {
			return fmt.Errorf("%s is invalid", label)
		}
	}
	return nil
}

type Evidence struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Version   string `json:"version,omitempty"`
	Watermark string `json:"watermark,omitempty"`
}

func (e Evidence) Validate() error {
	for label, value := range map[string]string{"evidence kind": e.Kind, "evidence reference": e.Reference} {
		if !bounded(value, 256) {
			return fmt.Errorf("%s is invalid", label)
		}
	}
	if e.Version != "" && !bounded(e.Version, 128) || e.Watermark != "" && !bounded(e.Watermark, 256) {
		return fmt.Errorf("evidence version or watermark is invalid")
	}
	return nil
}

type InvocationRequest struct {
	Metadata       InvocationMetadata `json:"metadata"`
	Scope          memory.Scope       `json:"scope"`
	Evidence       []Evidence         `json:"evidence"`
	Input          []byte             `json:"input"`
	MaxOutputBytes int                `json:"max_output_bytes"`
	Deadline       time.Time          `json:"deadline"`
	Now            time.Time          `json:"-"`
}

func (r InvocationRequest) Validate(l Limits, now time.Time) error {
	if err := l.Validate(); err != nil {
		return err
	}
	if err := r.Metadata.Validate(); err != nil {
		return err
	}
	if err := r.Scope.Validate(); err != nil {
		return fmt.Errorf("scope: %w", err)
	}
	if len(r.Input) == 0 || len(r.Input) > l.MaxInputBytes {
		return fmt.Errorf("reasoning input exceeds limit")
	}
	if r.MaxOutputBytes <= 0 || r.MaxOutputBytes > l.MaxOutputBytes {
		return fmt.Errorf("reasoning output budget exceeds limit")
	}
	if len(r.Evidence) == 0 || len(r.Evidence) > l.MaxEvidence {
		return fmt.Errorf("reasoning evidence is invalid")
	}
	for _, e := range r.Evidence {
		if err := e.Validate(); err != nil {
			return err
		}
	}
	if len(r.Input)+evidenceBytes(r.Evidence) > l.MaxInputBytes+l.MaxMetadataBytes {
		return fmt.Errorf("reasoning envelope exceeds limit")
	}
	if r.Deadline.IsZero() || !r.Deadline.After(now) || r.Deadline.After(now.Add(time.Duration(l.MaxDeadlineSeconds)*time.Second)) {
		return fmt.Errorf("reasoning deadline is invalid")
	}
	return nil
}

type Candidate struct {
	ID              string            `json:"id"`
	Scope           memory.Scope      `json:"scope"`
	Kind            string            `json:"kind"`
	InsightType     string            `json:"insight_type"`
	Content         string            `json:"content"`
	Evidence        []Evidence        `json:"evidence"`
	Provenance      map[string]string `json:"provenance"`
	PolicyVersion   string            `json:"policy_version"`
	ProviderVersion string            `json:"provider_version"`
	ReplayID        string            `json:"replay_id"`
	Confidence      float64           `json:"confidence,omitempty"`
	DirectMutation  bool              `json:"direct_mutation,omitempty"`
}

func (c Candidate) Validate(l Limits) error {
	if err := l.Validate(); err != nil {
		return err
	}
	if !bounded(c.ID, 256) || !bounded(c.Kind, 128) || !bounded(c.InsightType, 128) || !bounded(c.PolicyVersion, 256) || !bounded(c.ProviderVersion, 256) || !bounded(c.ReplayID, 256) || c.Content == "" || len(c.Content) > l.MaxOutputBytes {
		return fmt.Errorf("reasoning candidate fields are invalid")
	}
	if err := c.Scope.Validate(); err != nil {
		return fmt.Errorf("candidate scope: %w", err)
	}
	if c.DirectMutation {
		return fmt.Errorf("direct canonical mutation is not allowed")
	}
	if c.InsightType == "hypothesis" || c.InsightType == "goal" || c.InsightType == "contradiction" || c.InsightType == "causal_link" {
		return fmt.Errorf("reserved insight type is not enabled")
	}
	if len(c.Evidence) == 0 || len(c.Evidence) > l.MaxEvidence {
		return fmt.Errorf("candidate evidence is invalid")
	}
	for _, e := range c.Evidence {
		if err := e.Validate(); err != nil {
			return err
		}
	}
	if c.Confidence < 0 || c.Confidence > 1 {
		return fmt.Errorf("candidate confidence is invalid")
	}
	return nil
}

type ErrorCategory string

const (
	ErrorCategoryConfiguration   ErrorCategory = "configuration"
	ErrorCategoryCompatibility   ErrorCategory = "compatibility"
	ErrorCategoryScope           ErrorCategory = "scope"
	ErrorCategoryValidation      ErrorCategory = "validation"
	ErrorCategoryBudget          ErrorCategory = "budget"
	ErrorCategoryTimeout         ErrorCategory = "timeout"
	ErrorCategoryCancellation    ErrorCategory = "cancellation"
	ErrorCategoryRateLimit       ErrorCategory = "rate_limit"
	ErrorCategoryUnavailable     ErrorCategory = "provider_unavailable"
	ErrorCategoryMalformedOutput ErrorCategory = "malformed_output"
	ErrorCategoryStaleDependency ErrorCategory = "stale_dependency"
	ErrorCategoryRetryable       ErrorCategory = "retryable"
)

type ProviderError struct {
	Category  ErrorCategory `json:"category"`
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	Retryable bool          `json:"retryable"`
}

func (e ProviderError) Validate() error {
	valid := map[ErrorCategory]bool{ErrorCategoryConfiguration: true, ErrorCategoryCompatibility: true, ErrorCategoryScope: true, ErrorCategoryValidation: true, ErrorCategoryBudget: true, ErrorCategoryTimeout: true, ErrorCategoryCancellation: true, ErrorCategoryRateLimit: true, ErrorCategoryUnavailable: true, ErrorCategoryMalformedOutput: true, ErrorCategoryStaleDependency: true, ErrorCategoryRetryable: true}
	if !valid[e.Category] || !bounded(e.Code, 128) || e.Message == "" || len(e.Message) > 256 {
		return fmt.Errorf("reasoning provider error is invalid")
	}
	return nil
}

func Fingerprint(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func evidenceBytes(v []Evidence) int { b, _ := json.Marshal(v); return len(b) }
func fallback(v string) string {
	if strings.TrimSpace(v) == "" {
		return "unknown"
	}
	return strings.TrimSpace(v)
}
func optionalBounded(v string, max int) string {
	v = strings.TrimSpace(v)
	if v == "" || len(v) > max || strings.ContainsAny(v, "\r\n\x00") {
		return ""
	}
	return v
}
func bounded(v string, max int) bool {
	v = strings.TrimSpace(v)
	return v != "" && len(v) <= max && !strings.ContainsAny(v, "\r\n\x00")
}
