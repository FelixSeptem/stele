package embedding

import (
	"context"
	"fmt"
	"strings"
)

type Target struct {
	Provider   string
	Model      string
	Dimensions int
}

func (t Target) Validate() error {
	switch {
	case strings.TrimSpace(t.Provider) == "":
		return fmt.Errorf("embedding provider is required")
	case strings.TrimSpace(t.Model) == "":
		return fmt.Errorf("embedding model is required")
	case t.Dimensions <= 0:
		return fmt.Errorf("embedding dimensions must be greater than zero")
	default:
		return nil
	}
}

type ProviderRequest struct {
	Text   string
	Target Target
}

type ProviderResult struct {
	Provider   string
	Model      string
	Dimensions int
	Embedding  []float32
}

type Provider interface {
	GenerateEmbedding(ctx context.Context, input ProviderRequest) (ProviderResult, error)
}

type Router struct {
	Default Target
	ByClass map[string]Target
}

func (r Router) ResolveTarget(memoryClass string) (Target, error) {
	if target, ok := r.ByClass[strings.TrimSpace(memoryClass)]; ok {
		if err := target.Validate(); err != nil {
			return Target{}, err
		}
		return target, nil
	}

	if err := r.Default.Validate(); err != nil {
		return Target{}, err
	}
	return r.Default, nil
}

func DetermineDrift(target Target, activeProvider, activeModel string, activeDimensions int) bool {
	return strings.TrimSpace(target.Provider) != strings.TrimSpace(activeProvider) ||
		strings.TrimSpace(target.Model) != strings.TrimSpace(activeModel) ||
		target.Dimensions != activeDimensions
}
