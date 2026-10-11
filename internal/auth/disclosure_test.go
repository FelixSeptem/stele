package auth

import (
	"errors"
	"testing"
)

func TestDisclosureEntitlementsRejectInvalidProvisioning(t *testing.T) {
	for _, e := range []DisclosureEntitlements{
		{},
		{AllowedProfiles: []string{DisclosureProfileReference}, DefaultProfile: DisclosureProfileContent},
		{AllowedProfiles: []string{"caller-admin"}, DefaultProfile: "caller-admin"},
		{AllowedProfiles: []string{DisclosureProfileReference, DisclosureProfileReference}, DefaultProfile: DisclosureProfileReference},
	} {
		if e.Validate() == nil {
			t.Fatalf("accepted invalid entitlements: %+v", e)
		}
		if _, err := e.Resolve("", ""); !errors.Is(err, ErrDisclosureDenied) {
			t.Fatalf("missing/invalid entitlements should deny: %v", err)
		}
	}
}

func TestDisclosureIntersectionAndDefaultReference(t *testing.T) {
	e := DisclosureEntitlements{AllowedProfiles: []string{DisclosureProfileReference, DisclosureProfileContent}, DefaultProfile: DisclosureProfileContent}
	got, err := e.Resolve("", "")
	if err != nil || got.ResponseMode != ResponseModeReferenceOnly || got.Profile != DisclosureProfileContent {
		t.Fatalf("unsafe default: %+v %v", got, err)
	}
	if _, err := e.Resolve(DisclosureProfileReference, ResponseModeBoundedContent); !errors.Is(err, ErrDisclosureDenied) {
		t.Fatalf("reference profile granted content: %v", err)
	}
	for _, profile := range []string{"unknown", "unauthorized"} {
		if _, err := e.Resolve(profile, ResponseModeReferenceOnly); !errors.Is(err, ErrDisclosureDenied) {
			t.Fatalf("profile oracle: %v", err)
		}
	}
	e.AllowedProfiles = []string{DisclosureProfileReference}
	e.DefaultProfile = DisclosureProfileReference
	if _, err := e.Resolve(DisclosureProfileContent, ResponseModeBoundedContent); !errors.Is(err, ErrDisclosureDenied) {
		t.Fatalf("downgrade ignored: %v", err)
	}
}
