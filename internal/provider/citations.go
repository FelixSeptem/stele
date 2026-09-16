package provider

import (
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

const (
	CitationAvailable   = "available"
	CitationUnavailable = "unavailable"
)

// ShapeSearchResult removes ranking diagnostics and canonical content from the
// citation surface while retaining the ordinary, already scope-filtered hits.
func ShapeSearchResult(result retrieval.SearchResult, maxCitations int) (Result, error) {
	if maxCitations <= 0 {
		return Result{}, fmt.Errorf("citation budget must be greater than zero")
	}
	citations := make([]Citation, 0, min(maxCitations, len(result.Hits)))
	for _, hit := range result.Hits {
		if hit.Memory.State != memory.MemoryStateActive {
			continue
		}
		for _, source := range hit.Citations {
			if len(citations) == maxCitations {
				return Result{}, fmt.Errorf("citation budget exceeded")
			}
			reference := strings.TrimSpace(source.MemoryID)
			kind := "memory"
			if strings.TrimSpace(source.RawEventID) != "" {
				reference = strings.TrimSpace(source.RawEventID)
				kind = "raw_event"
			}
			citation := Citation{SourceKind: kind, Reference: reference, Availability: CitationAvailable}
			if err := citation.Validate(); err != nil {
				return Result{}, err
			}
			citations = append(citations, citation)
		}
	}
	// SearchResult's exported JSON shape is lifecycle filtered by retrieval.
	// Diagnostics and scores are intentionally replaced by the bounded view.
	visible := make([]map[string]any, 0, len(result.Hits))
	for _, hit := range result.Hits {
		if hit.Memory.State != memory.MemoryStateActive {
			continue
		}
		visible = append(visible, map[string]any{
			"memory_id": hit.Memory.ID,
			"class":     hit.Memory.Class,
			"content":   hit.Memory.Content,
		})
	}
	return Result{Data: map[string]any{"items": visible}, Citations: citations}, nil
}

func ShapeContext(context retrieval.AssembledContext, maxCitations int) (Result, error) {
	if maxCitations <= 0 {
		return Result{}, fmt.Errorf("citation budget must be greater than zero")
	}
	citations := make([]Citation, 0, min(maxCitations, len(context.Citations)))
	for _, source := range context.Citations {
		if len(citations) == maxCitations {
			return Result{}, fmt.Errorf("citation budget exceeded")
		}
		reference := strings.TrimSpace(source.MemoryID)
		kind := "memory"
		if strings.TrimSpace(source.RawEventID) != "" {
			reference = strings.TrimSpace(source.RawEventID)
			kind = "raw_event"
		}
		citation := Citation{SourceKind: kind, Reference: reference, Availability: CitationAvailable}
		if err := citation.Validate(); err != nil {
			return Result{}, err
		}
		citations = append(citations, citation)
	}
	return Result{Data: map[string]any{
		"profile":            visibleContextHits(context.Profile),
		"recent_session":     visibleContextHits(context.RecentSession),
		"recent_episodes":    visibleContextHits(context.RecentEpisodes),
		"relevant_summaries": visibleContextHits(context.RelevantSummaries),
		"related_entities":   visibleContextHits(context.RelatedEntities),
	}, Citations: citations}, nil
}

func visibleContextHits(hits []retrieval.SearchHit) []map[string]any {
	out := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		if hit.Memory.State == memory.MemoryStateActive {
			out = append(out, map[string]any{"memory_id": hit.Memory.ID, "class": hit.Memory.Class, "content": hit.Memory.Content})
		}
	}
	return out
}
