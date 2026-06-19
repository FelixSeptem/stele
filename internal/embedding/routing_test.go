package embedding

import (
	"testing"
)

func TestResolveTargetUsesDefaultRouteForCanonicalClasses(t *testing.T) {
	router := Router{
		Default: Target{
			Provider:   "openai",
			Model:      "text-embedding-3-small",
			Dimensions: 1536,
		},
	}

	target, err := router.ResolveTarget("profile")
	if err != nil {
		t.Fatalf("ResolveTarget() error = %v", err)
	}

	if target.Provider != "openai" || target.Model != "text-embedding-3-small" || target.Dimensions != 1536 {
		t.Fatalf("target = %+v, want default route", target)
	}
}

func TestResolveTargetPrefersClassSpecificRoute(t *testing.T) {
	router := Router{
		Default: Target{
			Provider:   "openai",
			Model:      "text-embedding-3-small",
			Dimensions: 1536,
		},
		ByClass: map[string]Target{
			"summary": {
				Provider:   "openai",
				Model:      "text-embedding-3-large",
				Dimensions: 3072,
			},
		},
	}

	target, err := router.ResolveTarget("summary")
	if err != nil {
		t.Fatalf("ResolveTarget() error = %v", err)
	}

	if target.Model != "text-embedding-3-large" || target.Dimensions != 3072 {
		t.Fatalf("target = %+v, want summary-specific route", target)
	}
}

func TestResolveTargetRejectsMissingProviderOrModel(t *testing.T) {
	router := Router{
		Default: Target{},
	}

	_, err := router.ResolveTarget("profile")
	if err == nil {
		t.Fatal("ResolveTarget() error = nil, want invalid target error")
	}
}

func TestDetermineDriftDetectsProviderModelOrDimensionMismatch(t *testing.T) {
	target := Target{
		Provider:   "openai",
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
	}

	if drift := DetermineDrift(target, "openai", "text-embedding-3-small", 1536); drift {
		t.Fatal("DetermineDrift() = true, want false for matching active vector target")
	}

	if drift := DetermineDrift(target, "anthropic", "text-embedding-3-small", 1536); !drift {
		t.Fatal("DetermineDrift() = false, want true for provider mismatch")
	}
	if drift := DetermineDrift(target, "openai", "text-embedding-3-large", 1536); !drift {
		t.Fatal("DetermineDrift() = false, want true for model mismatch")
	}
	if drift := DetermineDrift(target, "openai", "text-embedding-3-small", 3072); !drift {
		t.Fatal("DetermineDrift() = false, want true for dimension mismatch")
	}
}
