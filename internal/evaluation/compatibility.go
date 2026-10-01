package evaluation

import (
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
)

func AuthorizeReportScope(requested memory.Scope, reportScopeHash string) error {
	if err := requested.Validate(); err != nil {
		return fmt.Errorf("scope is required: %w", err)
	}
	if reportScopeHash == "" || reportScopeHash != hashScope(requested) {
		return fmt.Errorf("report scope does not match authorized scope")
	}
	return nil
}

func RequireCompatibleIdentity(expected, actual CompatibilityIdentity) error {
	if err := expected.Validate(); err != nil {
		return fmt.Errorf("expected identity: %w", err)
	}
	if err := actual.Validate(); err != nil {
		return fmt.Errorf("actual identity: %w", err)
	}
	fields := []struct {
		name string
		want string
		have string
	}{
		{"fixture version", expected.FixtureVersion, actual.FixtureVersion},
		{"policy version", expected.PolicyVersion, actual.PolicyVersion},
		{"strategy", expected.Strategy, actual.Strategy},
		{"renderer", expected.Renderer, actual.Renderer},
		{"provider", expected.Provider, actual.Provider},
		{"source watermark", expected.SourceWatermark, actual.SourceWatermark},
	}
	for _, field := range fields {
		if field.want != field.have {
			return fmt.Errorf("%s is incompatible", field.name)
		}
	}
	return nil
}
