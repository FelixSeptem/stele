package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
)

type WhoAmIResponse struct {
	PrincipalID          string       `json:"principal_id"`
	Role                 string       `json:"role"`
	AccessMode           string       `json:"access_mode"`
	ActiveScopeAvailable bool         `json:"active_scope_available"`
	ScopeDigest          string       `json:"scope_digest,omitempty"`
	Credential           string       `json:"-"`
	Scope                memory.Scope `json:"-"`
}

func BuildWhoAmI(dispatch DispatchContext, activeScopeAvailable bool) WhoAmIResponse {
	return WhoAmIResponse{
		PrincipalID:          dispatch.PrincipalID,
		Role:                 dispatch.Role,
		AccessMode:           dispatch.AccessMode,
		ActiveScopeAvailable: activeScopeAvailable,
		ScopeDigest:          scopeDigest(dispatch.Scope),
	}
}

func scopeDigest(scope memory.Scope) string {
	if scope.Tenant == "" && scope.Project == "" && scope.Namespace == "" {
		return ""
	}
	value := fmt.Sprintf("%s\x00%s\x00%s", scope.Tenant, scope.Project, scope.Namespace)
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:24]
}
