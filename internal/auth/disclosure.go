package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
)

const (
	DisclosurePolicyVersion    = "context-disclosure-v1"
	DisclosureProfileReference = "memory-reference-v1"
	DisclosureProfileContent   = "memory-content-v1"
	ResponseModeReferenceOnly  = "reference_only"
	ResponseModeBoundedContent = "bounded_content"
)

var ErrDisclosureDenied = errors.New("disclosure denied")

type DisclosureEntitlements struct {
	AllowedProfiles   []string `json:"allowed_profiles"`
	DefaultProfile    string   `json:"default_profile"`
	SourceAttestation bool     `json:"source_attestation"`
}

func (e DisclosureEntitlements) Validate() error {
	if len(e.AllowedProfiles) == 0 || len(e.AllowedProfiles) > 2 {
		return fmt.Errorf("invalid disclosure profiles")
	}
	seen := map[string]bool{}
	for _, p := range e.AllowedProfiles {
		if (p != DisclosureProfileReference && p != DisclosureProfileContent) || seen[p] {
			return fmt.Errorf("invalid disclosure profiles")
		}
		seen[p] = true
	}
	if !seen[e.DefaultProfile] {
		return fmt.Errorf("invalid default disclosure profile")
	}
	return nil
}

type DisclosureDecision struct {
	Profile               string `json:"profile"`
	PolicyVersion         string `json:"policy_version"`
	ResponseMode          string `json:"-"`
	ProjectionIntent      string `json:"projection_intent"`
	SourceMetadataVersion string `json:"source_metadata_version"`
}

func (e DisclosureEntitlements) Resolve(profile, mode string) (DisclosureDecision, error) {
	if e.Validate() != nil {
		return DisclosureDecision{}, ErrDisclosureDenied
	}
	if profile == "" {
		profile = e.DefaultProfile
	}
	if mode == "" {
		mode = ResponseModeReferenceOnly
	}
	allowed := false
	for _, p := range e.AllowedProfiles {
		allowed = allowed || p == profile
	}
	if !allowed || (mode != ResponseModeReferenceOnly && mode != ResponseModeBoundedContent) || (mode == ResponseModeBoundedContent && profile != DisclosureProfileContent) {
		return DisclosureDecision{}, ErrDisclosureDenied
	}
	return DisclosureDecision{Profile: profile, PolicyVersion: DisclosurePolicyVersion, ResponseMode: mode, ProjectionIntent: "conversation", SourceMetadataVersion: memory.SourceMetadataVersion}, nil
}

type ScopeDisclosureAuthorizer interface {
	AuthorizeScopeDisclosure(context.Context, string, memory.Scope) (DisclosureEntitlements, error)
}

// ResolveScopeDisclosure always consults the current store; old bindings carry no grant ceiling.
func (s *PrincipalService) AuthorizeScopeDisclosure(ctx context.Context, principalID string, scope memory.Scope) (DisclosureEntitlements, error) {
	if s == nil || s.store == nil {
		return DisclosureEntitlements{}, fmt.Errorf("disclosure store unavailable")
	}
	store, ok := s.store.(interface {
		ScopeGrantDisclosure(context.Context, string, memory.Scope) (DisclosureEntitlements, error)
	})
	if !ok {
		return DisclosureEntitlements{}, fmt.Errorf("disclosure store unavailable")
	}
	return store.ScopeGrantDisclosure(ctx, principalID, scope.Normalized())
}
