package app

import (
	"context"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

type providerRuntimeReader struct {
	searcher  retrieval.MemorySearcher
	assembler retrieval.ContextAssembler
}

func (r providerRuntimeReader) Read(ctx context.Context, scope memory.Scope, kind provider.ReadKind, request provider.ReadRequest) (any, error) {
	switch kind {
	case provider.ReadKindRetrieval:
		if r.searcher == nil {
			return nil, fmt.Errorf("retrieval dependency is not configured")
		}
		limit := request.Limit
		if limit == 0 {
			limit = 10
		}
		if limit > 100 {
			limit = 100
		}
		result, err := r.searcher.Search(ctx, retrieval.SearchInput{Scope: scope, Query: request.Query, TopK: limit})
		if err != nil {
			return nil, err
		}
		return provider.ShapeSearchResult(result, limit)
	case provider.ReadKindContext:
		if r.assembler == nil {
			return nil, fmt.Errorf("context dependency is not configured")
		}
		limit := request.Limit
		if limit == 0 {
			limit = 10
		}
		if limit > 100 {
			limit = 100
		}
		result, err := r.assembler.AssembleContext(ctx, retrieval.AssembleContextInput{Scope: scope, Query: request.Query, Budget: limit})
		if err != nil {
			return nil, err
		}
		return provider.ShapeContext(result, limit)
	case provider.ReadKindStatus, provider.ReadKindReport:
		return map[string]any{"status": "available", "scope": "exact"}, nil
	default:
		return nil, fmt.Errorf("unsupported provider read kind")
	}
}
