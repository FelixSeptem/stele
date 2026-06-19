package embedding

import (
	"fmt"
	"strings"
)

type ProviderResolver interface {
	ResolveProvider(name string) (Provider, error)
}

type StaticProviderRegistry map[string]Provider

func (r StaticProviderRegistry) ResolveProvider(name string) (Provider, error) {
	provider, ok := r[strings.TrimSpace(name)]
	if !ok || provider == nil {
		return nil, fmt.Errorf("embedding provider %q is not registered", name)
	}

	return provider, nil
}
