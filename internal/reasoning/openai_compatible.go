package reasoning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const OpenAICompatibleProviderVersion = "openai-compatible-reasoning-v1"

type OpenAICompatibleConfig struct {
	Endpoint         string
	Model            string
	APIKey           string
	Timeout          time.Duration
	MaxRequestBytes  int
	MaxResponseBytes int
	HTTPClient       *http.Client
	Limits           Limits
}

type OpenAICompatibleProvider struct {
	endpoint         string
	model            string
	apiKey           string
	timeout          time.Duration
	maxRequestBytes  int
	maxResponseBytes int
	limits           Limits
	client           *http.Client
}

type openAICompatibleRequest struct {
	Model          string                         `json:"model"`
	Messages       []openAICompatibleMessage      `json:"messages"`
	Temperature    int                            `json:"temperature"`
	ResponseFormat openAICompatibleResponseFormat `json:"response_format"`
}

type openAICompatibleMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAICompatibleResponseFormat struct {
	Type string `json:"type"`
}

type openAICompatibleResponse struct {
	Choices []openAICompatibleChoice `json:"choices"`
}

type openAICompatibleChoice struct {
	Message openAICompatibleMessage `json:"message"`
	Tool    json.RawMessage         `json:"tool_calls,omitempty"`
	Refusal string                  `json:"refusal,omitempty"`
}

func NewOpenAICompatibleProvider(cfg OpenAICompatibleConfig) (*OpenAICompatibleProvider, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("reasoning adapter endpoint is required")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("reasoning adapter endpoint is invalid")
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" || len(model) > 256 || strings.ContainsAny(model, "\r\n\x00") {
		return nil, fmt.Errorf("reasoning adapter model is required and bounded")
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("reasoning adapter api key is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 || timeout > 10*time.Minute {
		return nil, fmt.Errorf("reasoning adapter timeout is invalid")
	}
	maxRequest := cfg.MaxRequestBytes
	if maxRequest <= 0 {
		maxRequest = 128 << 10
	}
	maxResponse := cfg.MaxResponseBytes
	if maxResponse <= 0 {
		maxResponse = 64 << 10
	}
	if maxRequest > 1<<20 || maxResponse > 1<<20 {
		return nil, fmt.Errorf("reasoning adapter transport bounds are invalid")
	}
	limits := cfg.Limits
	if err := limits.Validate(); err != nil {
		limits = DefaultLimits()
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &OpenAICompatibleProvider{endpoint: endpoint, model: model, apiKey: apiKey, timeout: timeout, maxRequestBytes: maxRequest, maxResponseBytes: maxResponse, limits: limits, client: client}, nil
}

func (p *OpenAICompatibleProvider) Derive(ctx context.Context, input InvocationRequest) (Candidate, error) {
	if p == nil {
		return Candidate{}, &ProviderError{Category: ErrorCategoryConfiguration, Code: "adapter_unconfigured", Message: "reasoning adapter is not configured", Retryable: false}
	}
	limits := p.limits
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := input.Validate(limits, now); err != nil {
		return Candidate{}, &ProviderError{Category: ErrorCategoryValidation, Code: "invalid_envelope", Message: "reasoning invocation failed local validation", Retryable: false}
	}
	payload := openAICompatibleRequest{
		Model: p.model,
		Messages: []openAICompatibleMessage{
			{Role: "system", Content: "Return exactly one JSON reasoning candidate. Do not call tools, mutate records, widen scope, or include unrequested fields."},
			{Role: "user", Content: string(normalizeInvocation(input))},
		},
		Temperature:    0,
		ResponseFormat: openAICompatibleResponseFormat{Type: "json_object"},
	}
	body, err := json.Marshal(payload)
	if err != nil || len(body) > p.maxRequestBytes {
		return Candidate{}, &ProviderError{Category: ErrorCategoryBudget, Code: "request_too_large", Message: "reasoning adapter request exceeds transport bound", Retryable: false}
	}
	deadline := input.Deadline
	requestCtx := ctx
	var cancel context.CancelFunc
	if p.timeout > 0 {
		requestCtx, cancel = context.WithTimeout(requestCtx, p.timeout)
		defer cancel()
	}
	if !deadline.IsZero() {
		if requestCtxDeadline, ok := requestCtx.Deadline(); !ok || deadline.Before(requestCtxDeadline) {
			requestCtx, cancel = context.WithDeadline(ctx, deadline)
			defer cancel()
		}
	}
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return Candidate{}, &ProviderError{Category: ErrorCategoryConfiguration, Code: "request_build_failed", Message: "reasoning adapter request could not be built", Retryable: false}
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return Candidate{}, classifyOpenAICompatibleTransportError(requestCtx, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Candidate{}, classifyOpenAICompatibleStatus(resp.StatusCode)
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, int64(p.maxResponseBytes)+1))
	if err != nil {
		return Candidate{}, &ProviderError{Category: ErrorCategoryUnavailable, Code: "response_read_failed", Message: "reasoning adapter response could not be read", Retryable: true}
	}
	if len(responseBody) > p.maxResponseBytes {
		return Candidate{}, &ProviderError{Category: ErrorCategoryMalformedOutput, Code: "response_too_large", Message: "reasoning adapter response exceeds transport bound", Retryable: false}
	}
	var decoded openAICompatibleResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil || len(decoded.Choices) != 1 {
		return Candidate{}, &ProviderError{Category: ErrorCategoryMalformedOutput, Code: "invalid_response", Message: "reasoning adapter response is malformed", Retryable: false}
	}
	choice := decoded.Choices[0]
	if len(choice.Tool) > 0 || choice.Refusal != "" || choice.Message.Role != "" && choice.Message.Role != "assistant" || strings.TrimSpace(choice.Message.Content) == "" {
		return Candidate{}, &ProviderError{Category: ErrorCategoryMalformedOutput, Code: "unsupported_response", Message: "reasoning adapter response is not candidate-only", Retryable: false}
	}
	var candidate Candidate
	decoder := json.NewDecoder(strings.NewReader(choice.Message.Content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&candidate); err != nil {
		return Candidate{}, &ProviderError{Category: ErrorCategoryMalformedOutput, Code: "invalid_candidate", Message: "reasoning adapter candidate is malformed", Retryable: false}
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Candidate{}, &ProviderError{Category: ErrorCategoryMalformedOutput, Code: "trailing_candidate_data", Message: "reasoning adapter candidate contains trailing data", Retryable: false}
	}
	if err := candidate.Validate(limits); err != nil {
		return Candidate{}, &ProviderError{Category: ErrorCategoryMalformedOutput, Code: "candidate_rejected", Message: "reasoning adapter candidate failed validation", Retryable: false}
	}
	if candidate.Scope.Normalized() != input.Scope.Normalized() {
		return Candidate{}, &ProviderError{Category: ErrorCategoryScope, Code: "scope_mismatch", Message: "reasoning adapter candidate scope is not allowed", Retryable: false}
	}
	if candidate.PolicyVersion != input.Metadata.PolicyVersion || candidate.ProviderVersion != input.Metadata.ProviderVersion {
		return Candidate{}, &ProviderError{Category: ErrorCategoryCompatibility, Code: "version_mismatch", Message: "reasoning adapter candidate versions are incompatible", Retryable: false}
	}
	if !evidenceSubset(candidate.Evidence, input.Evidence) {
		return Candidate{}, &ProviderError{Category: ErrorCategoryScope, Code: "evidence_mismatch", Message: "reasoning adapter candidate evidence is not allowed", Retryable: false}
	}
	return candidate, nil
}

func normalizeInvocation(input InvocationRequest) []byte {
	copyInput := input
	copyInput.Now = time.Time{}
	b, _ := json.Marshal(copyInput)
	return b
}

func evidenceSubset(candidate, allowed []Evidence) bool {
	set := make(map[string]struct{}, len(allowed))
	for _, evidence := range allowed {
		set[evidence.Kind+"\x00"+evidence.Reference+"\x00"+evidence.Version+"\x00"+evidence.Watermark] = struct{}{}
	}
	for _, evidence := range candidate {
		if _, ok := set[evidence.Kind+"\x00"+evidence.Reference+"\x00"+evidence.Version+"\x00"+evidence.Watermark]; !ok {
			return false
		}
	}
	return true
}

func classifyOpenAICompatibleStatus(status int) *ProviderError {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &ProviderError{Category: ErrorCategoryConfiguration, Code: "provider_auth_failed", Message: "reasoning adapter authentication failed", Retryable: false}
	case http.StatusTooManyRequests:
		return &ProviderError{Category: ErrorCategoryRateLimit, Code: "provider_rate_limited", Message: "reasoning adapter was rate limited", Retryable: true}
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return &ProviderError{Category: ErrorCategoryTimeout, Code: "provider_timeout", Message: "reasoning adapter timed out", Retryable: true}
	default:
		return &ProviderError{Category: ErrorCategoryUnavailable, Code: "provider_http_error", Message: "reasoning adapter is unavailable", Retryable: status >= 500}
	}
}

func classifyOpenAICompatibleTransportError(ctx context.Context, err error) *ProviderError {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return &ProviderError{Category: ErrorCategoryCancellation, Code: "request_canceled", Message: "reasoning adapter request was canceled", Retryable: true}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return &ProviderError{Category: ErrorCategoryTimeout, Code: "request_timeout", Message: "reasoning adapter request timed out", Retryable: true}
	}
	return &ProviderError{Category: ErrorCategoryUnavailable, Code: "provider_unavailable", Message: "reasoning adapter is unavailable", Retryable: true}
}

var _ Provider = (*OpenAICompatibleProvider)(nil)
