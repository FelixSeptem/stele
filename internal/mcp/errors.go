package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
)

type MCPError struct {
	Category ErrorCategory
	Code     string
}

func (e MCPError) Error() string {
	if e.Category == "" {
		return "mcp_error"
	}
	if e.Code == "" {
		return string(e.Category)
	}
	return fmt.Sprintf("%s:%s", e.Category, e.Code)
}

func mcpError(category ErrorCategory, code string) error {
	return MCPError{Category: category, Code: code}
}

func safeMutationError(err error) error {
	switch {
	case errors.Is(err, memory.ErrIdempotencyConflict):
		return mcpError(ErrorValidation, "idempotency_conflict")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return mcpError(ErrorRetryable, "operation_interrupted")
	default:
		return mcpError(ErrorDependency, "mutation_failed")
	}
}
