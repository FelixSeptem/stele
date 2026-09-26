package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

func fingerprint(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "invalid"
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func idempotencyKeyRequired(key string) error {
	if strings.TrimSpace(key) == "" {
		return mcpError(ErrorValidation, "idempotency_required")
	}
	if len(key) > 256 {
		return fmt.Errorf("%w", mcpError(ErrorValidation, "idempotency_invalid"))
	}
	return nil
}
