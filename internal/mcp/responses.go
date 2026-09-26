package mcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

type SafeMemoryHit struct {
	ID        string               `json:"id"`
	Class     memory.MemoryClass   `json:"class"`
	State     memory.MemoryState   `json:"state"`
	Content   string               `json:"content"`
	Citations []retrieval.Citation `json:"citations,omitempty"`
}

type SearchResponse struct {
	Hits     []SafeMemoryHit              `json:"hits"`
	Temporal *retrieval.TemporalSelection `json:"temporal,omitempty"`
}

type ContextResponse struct {
	Profile           []SafeMemoryHit      `json:"profile"`
	RecentSession     []SafeMemoryHit      `json:"recent_session"`
	RecentEpisodes    []SafeMemoryHit      `json:"recent_episodes"`
	RelevantSummaries []SafeMemoryHit      `json:"relevant_summaries"`
	RelatedEntities   []SafeMemoryHit      `json:"related_entities"`
	Citations         []retrieval.Citation `json:"citations"`
}

type SafeMemoryResource struct {
	ID         string                  `json:"id"`
	Class      memory.MemoryClass      `json:"class"`
	State      memory.MemoryState      `json:"state"`
	Content    string                  `json:"content"`
	CreatedAt  time.Time               `json:"created_at"`
	ModifiedAt time.Time               `json:"modified_at"`
	Temporal   memory.TemporalMetadata `json:"temporal"`
}

type BrowseResponse struct {
	Items      []SafeMemoryResource `json:"items"`
	NextOffset int                  `json:"next_offset,omitempty"`
}

type MutationResponse struct {
	IntentID string `json:"intent_id,omitempty"`
	Status   string `json:"status,omitempty"`
	Replayed bool   `json:"replayed,omitempty"`
}

type ForgetPreviewResponse struct {
	PreviewID string   `json:"preview_id"`
	MemoryIDs []string `json:"memory_ids"`
}

type ForgetApplyResponse struct {
	PreviewID  string   `json:"preview_id"`
	AppliedIDs []string `json:"applied_ids"`
}

func shapeSearchResponse(result retrieval.SearchResult) SearchResponse {
	response := SearchResponse{Hits: make([]SafeMemoryHit, 0, len(result.Hits)), Temporal: result.Temporal}
	for _, hit := range result.Hits {
		response.Hits = append(response.Hits, safeHit(hit))
	}
	return response
}

func shapeContextResponse(result retrieval.AssembledContext) ContextResponse {
	return ContextResponse{
		Profile: shapeHits(result.Profile), RecentSession: shapeHits(result.RecentSession), RecentEpisodes: shapeHits(result.RecentEpisodes),
		RelevantSummaries: shapeHits(result.RelevantSummaries), RelatedEntities: shapeHits(result.RelatedEntities), Citations: result.Citations,
	}
}

func shapeHits(hits []retrieval.SearchHit) []SafeMemoryHit {
	out := make([]SafeMemoryHit, 0, len(hits))
	for _, hit := range hits {
		out = append(out, safeHit(hit))
	}
	return out
}

func safeHit(hit retrieval.SearchHit) SafeMemoryHit {
	return SafeMemoryHit{ID: hit.Memory.ID, Class: hit.Memory.Class, State: hit.Memory.State, Content: hit.Memory.Content, Citations: hit.Citations}
}

func shapeBrowseResponse(page memory.MemoryPage, offset, limit int) BrowseResponse {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(page.Items) {
		return BrowseResponse{Items: []SafeMemoryResource{}}
	}
	end := len(page.Items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	items := make([]SafeMemoryResource, 0, end-offset)
	for _, item := range page.Items[offset:end] {
		items = append(items, SafeMemoryResource{ID: item.ID, Class: item.Class, State: item.State, Content: item.Content, CreatedAt: item.CreatedAt, ModifiedAt: item.ModifiedAt, Temporal: item.Temporal})
	}
	next := 0
	if end < len(page.Items) {
		next = end
	}
	return BrowseResponse{Items: items, NextOffset: next}
}

func parseTemporal(asOf, validFrom, validTo string) (memory.TemporalConstraint, error) {
	asOf, validFrom, validTo = strings.TrimSpace(asOf), strings.TrimSpace(validFrom), strings.TrimSpace(validTo)
	if asOf != "" && (validFrom != "" || validTo != "") {
		return memory.TemporalConstraint{}, fmt.Errorf("invalid temporal selector")
	}
	if asOf != "" {
		parsed, err := time.Parse(time.RFC3339, asOf)
		if err != nil {
			return memory.TemporalConstraint{}, fmt.Errorf("invalid temporal selector")
		}
		return memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf, AsOf: &parsed}, nil
	}
	if (validFrom == "") != (validTo == "") {
		return memory.TemporalConstraint{}, fmt.Errorf("invalid temporal selector")
	}
	if validFrom == "" {
		return memory.TemporalConstraint{}, nil
	}
	from, err := time.Parse(time.RFC3339, validFrom)
	if err != nil {
		return memory.TemporalConstraint{}, fmt.Errorf("invalid temporal selector")
	}
	to, err := time.Parse(time.RFC3339, validTo)
	if err != nil || !to.After(from) {
		return memory.TemporalConstraint{}, fmt.Errorf("invalid temporal selector")
	}
	return memory.TemporalConstraint{Mode: memory.TemporalSelectionDuring, ValidFrom: &from, ValidTo: &to}, nil
}

func safeValidationError(error) error { return mcpError(ErrorValidation, "invalid_request") }
