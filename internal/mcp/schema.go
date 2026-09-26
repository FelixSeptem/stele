package mcp

import (
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
)

const (
	ToolWhoAmI        = "who_am_i"
	ToolSearch        = "memory_search"
	ToolContext       = "memory_context"
	ToolBrowse        = "memory_browse"
	ToolRemember      = "memory_remember"
	ToolForget        = "memory_forget"
	ToolForgetPreview = "memory_forget_preview"
	ToolForgetApply   = "memory_forget_apply"
)

type ErrorCategory string

const (
	ErrorAuth          ErrorCategory = "auth"
	ErrorScope         ErrorCategory = "scope"
	ErrorValidation    ErrorCategory = "validation"
	ErrorLifecycle     ErrorCategory = "lifecycle"
	ErrorCompatibility ErrorCategory = "compatibility"
	ErrorDependency    ErrorCategory = "dependency"
	ErrorRetryable     ErrorCategory = "retryable"
)

type Limits struct {
	MaxQueryBytes   int
	MaxPayloadBytes int
	MaxResults      int
	MaxIDs          int
}

type ToolAnnotation struct {
	ReadOnly    bool
	Destructive bool
	Idempotent  bool
}

type ToolDescriptor struct {
	Name        string
	Description string
	Annotation  ToolAnnotation
}

func ToolDescriptors() []ToolDescriptor {
	return []ToolDescriptor{
		{Name: ToolWhoAmI, Description: "Resolve the authenticated principal and exact access scope.", Annotation: ToolAnnotation{ReadOnly: true, Idempotent: true}},
		{Name: ToolSearch, Description: "Search governed memories relevant to a query.", Annotation: ToolAnnotation{ReadOnly: true, Idempotent: true}},
		{Name: ToolContext, Description: "Assemble bounded context from governed projections and memories.", Annotation: ToolAnnotation{ReadOnly: true, Idempotent: true}},
		{Name: ToolBrowse, Description: "Browse visible memories in one exact scope.", Annotation: ToolAnnotation{ReadOnly: true, Idempotent: true}},
		{Name: ToolRemember, Description: "Submit a governed memory intent.", Annotation: ToolAnnotation{Idempotent: true}},
		{Name: ToolForget, Description: "Request a governed lifecycle action for one memory.", Annotation: ToolAnnotation{Destructive: true, Idempotent: true}},
		{Name: ToolForgetPreview, Description: "Preview bounded semantic forget candidates without mutation.", Annotation: ToolAnnotation{ReadOnly: true, Idempotent: true}},
		{Name: ToolForgetApply, Description: "Apply only reviewed IDs from a forget preview.", Annotation: ToolAnnotation{Destructive: true, Idempotent: true}},
	}
}

type DispatchContext struct {
	PrincipalID string
	Role        string
	Scope       memory.Scope
	AccessMode  string
	ActiveScope bool
	Writable    bool
}

type ScopeInput struct {
	Tenant    string `json:"tenant,omitempty"`
	Project   string `json:"project,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

func (s ScopeInput) ToScope() memory.Scope {
	return memory.Scope{Tenant: strings.TrimSpace(s.Tenant), Project: strings.TrimSpace(s.Project), Namespace: strings.TrimSpace(s.Namespace)}
}

type SearchRequest struct {
	Query            string `json:"query"`
	Limit            int    `json:"limit,omitempty"`
	AsOf             string `json:"as_of,omitempty"`
	ValidFrom        string `json:"valid_from,omitempty"`
	ValidTo          string `json:"valid_to,omitempty"`
	RuntimeBindingID string `json:"runtime_binding_id,omitempty"`
	ScopeInput
}

type ContextRequest struct {
	Query            string `json:"query"`
	BudgetBytes      int    `json:"budget_bytes,omitempty"`
	RuntimeBindingID string `json:"runtime_binding_id,omitempty"`
	ScopeInput
}

type BrowseRequest struct {
	Limit            int    `json:"limit,omitempty"`
	Offset           int    `json:"offset,omitempty"`
	RuntimeBindingID string `json:"runtime_binding_id,omitempty"`
	ScopeInput
}

type RememberRequest struct {
	Content          string `json:"content"`
	MemoryClass      string `json:"memory_class,omitempty"`
	Reason           string `json:"reason"`
	TargetMemoryID   string `json:"target_memory_id,omitempty"`
	TargetVersion    int64  `json:"target_version,omitempty"`
	IdempotencyKey   string `json:"idempotency_key,omitempty"`
	RuntimeBindingID string `json:"runtime_binding_id,omitempty"`
	ScopeInput
}

// ForgetRequest targets exactly one caller-identified memory. Semantic bulk
// forgetting must use the separate preview/apply contract.
type ForgetRequest struct {
	MemoryID         string `json:"memory_id"`
	Action           string `json:"action,omitempty"`
	Reason           string `json:"reason"`
	IdempotencyKey   string `json:"idempotency_key"`
	RuntimeBindingID string `json:"runtime_binding_id,omitempty"`
	ScopeInput
}

type ForgetResponse struct {
	MemoryID string `json:"memory_id"`
	Action   string `json:"action"`
	Status   string `json:"status"`
}

type ForgetApplyRequest struct {
	PreviewID        string   `json:"preview_id"`
	MemoryIDs        []string `json:"memory_ids"`
	Action           string   `json:"action,omitempty"`
	Reason           string   `json:"reason"`
	IdempotencyKey   string   `json:"idempotency_key,omitempty"`
	RuntimeBindingID string   `json:"runtime_binding_id,omitempty"`
	ScopeInput
}

type ForgetPreviewRequest struct {
	Query            string  `json:"query"`
	Limit            int     `json:"limit,omitempty"`
	Threshold        float64 `json:"threshold,omitempty"`
	RuntimeBindingID string  `json:"runtime_binding_id,omitempty"`
	ScopeInput
}

func ValidateToolRequest(operation string, request any, limits Limits) error {
	switch operation {
	case ToolWhoAmI:
		return nil
	case ToolSearch:
		r, ok := request.(*SearchRequest)
		if !ok || r == nil {
			return fmt.Errorf("%s request is invalid", operation)
		}
		return r.Validate(limits)
	case ToolContext:
		r, ok := request.(*ContextRequest)
		if !ok || r == nil {
			return fmt.Errorf("%s request is invalid", operation)
		}
		return r.Validate(limits)
	case ToolBrowse:
		r, ok := request.(*BrowseRequest)
		if !ok || r == nil {
			return fmt.Errorf("%s request is invalid", operation)
		}
		return r.Validate(limits)
	case ToolRemember:
		r, ok := request.(*RememberRequest)
		if !ok || r == nil {
			return fmt.Errorf("%s request is invalid", operation)
		}
		return r.Validate(limits)
	case ToolForgetPreview:
		r, ok := request.(*ForgetPreviewRequest)
		if !ok || r == nil {
			return fmt.Errorf("%s request is invalid", operation)
		}
		return r.Validate(limits)
	case ToolForgetApply:
		r, ok := request.(*ForgetApplyRequest)
		if !ok || r == nil {
			return fmt.Errorf("%s request is invalid", operation)
		}
		return r.Validate(limits)
	case ToolForget:
		r, ok := request.(*ForgetRequest)
		if !ok || r == nil {
			return fmt.Errorf("%s request is invalid", operation)
		}
		return r.Validate(limits)
	default:
		return fmt.Errorf("unknown MCP operation %q", operation)
	}
}

func (r ForgetRequest) Validate(_ Limits) error {
	if strings.TrimSpace(r.MemoryID) == "" || len(r.MemoryID) > 256 {
		return fmt.Errorf("memory ID is required")
	}
	if strings.TrimSpace(r.Reason) == "" || len(r.Reason) > 2048 {
		return fmt.Errorf("reason is required and must be bounded")
	}
	if err := idempotencyKeyRequired(r.IdempotencyKey); err != nil {
		return err
	}
	if len(r.RuntimeBindingID) > 256 {
		return fmt.Errorf("runtime binding ID exceeds MCP bound")
	}
	if action := policy.ForgettingAction(strings.TrimSpace(r.Action)); action != "" {
		if err := action.Validate(); err != nil {
			return fmt.Errorf("forget action is invalid")
		}
	}
	return nil
}

func (r SearchRequest) Validate(l Limits) error {
	if len([]byte(r.Query)) == 0 || len([]byte(r.Query)) > l.MaxQueryBytes {
		return fmt.Errorf("query exceeds MCP bound")
	}
	return validateLimit(r.Limit, l.MaxResults)
}

func (r ContextRequest) Validate(l Limits) error {
	if len([]byte(r.Query)) == 0 || len([]byte(r.Query)) > l.MaxQueryBytes {
		return fmt.Errorf("query exceeds MCP bound")
	}
	return validateLimit(r.BudgetBytes, l.MaxPayloadBytes)
}

func (r BrowseRequest) Validate(l Limits) error {
	if r.Offset < 0 || (l.MaxResults > 0 && r.Offset > l.MaxResults) {
		return fmt.Errorf("offset must not be negative")
	}
	return validateLimit(r.Limit, l.MaxResults)
}

func (r RememberRequest) Validate(l Limits) error {
	if len([]byte(r.Content)) == 0 || len([]byte(r.Content)) > l.MaxPayloadBytes {
		return fmt.Errorf("content exceeds MCP bound")
	}
	if strings.TrimSpace(r.Reason) == "" {
		return fmt.Errorf("reason and idempotency key are required")
	}
	if err := idempotencyKeyRequired(r.IdempotencyKey); err != nil {
		return err
	}
	if len(r.TargetMemoryID) > 256 {
		return fmt.Errorf("target memory ID exceeds MCP bound")
	}
	return nil
}

func (r ForgetApplyRequest) Validate(l Limits) error {
	if strings.TrimSpace(r.PreviewID) == "" || len(r.MemoryIDs) == 0 || len(r.MemoryIDs) > l.MaxIDs {
		return fmt.Errorf("forget apply IDs exceed MCP bound")
	}
	if strings.TrimSpace(r.Reason) == "" {
		return fmt.Errorf("reason and idempotency key are required")
	}
	if err := idempotencyKeyRequired(r.IdempotencyKey); err != nil {
		return err
	}
	if len(r.PreviewID) > 128 || len(r.RuntimeBindingID) > 256 {
		return fmt.Errorf("forget apply identity exceeds MCP bound")
	}
	if action := policy.ForgettingAction(strings.TrimSpace(r.Action)); action != "" {
		if err := action.Validate(); err != nil {
			return fmt.Errorf("forget action is invalid")
		}
	}
	seen := make(map[string]struct{}, len(r.MemoryIDs))
	for _, id := range r.MemoryIDs {
		id = strings.TrimSpace(id)
		if id == "" || len(id) > 256 {
			return fmt.Errorf("memory ID is required")
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("memory IDs must be unique")
		}
		seen[id] = struct{}{}
	}
	return nil
}

func (r ForgetPreviewRequest) Validate(l Limits) error {
	if len([]byte(r.Query)) == 0 || len([]byte(r.Query)) > l.MaxQueryBytes {
		return fmt.Errorf("query exceeds MCP bound")
	}
	if len(r.RuntimeBindingID) > 256 {
		return fmt.Errorf("runtime binding ID exceeds MCP bound")
	}
	if r.Threshold < 0 || r.Threshold > 1 {
		return fmt.Errorf("threshold must be between zero and one")
	}
	return validateLimit(r.Limit, l.MaxResults)
}

func validateLimit(value, max int) error {
	if value < 0 || (value != 0 && (max <= 0 || value > max)) {
		return fmt.Errorf("limit exceeds MCP bound")
	}
	return nil
}
