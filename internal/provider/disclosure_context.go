package provider

import (
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"strconv"
)

type ContextReference struct {
	Kind           string                `json:"kind"`
	MemoryID       string                `json:"id"`
	Version        string                `json:"version,omitempty"`
	MemoryPath     string                `json:"memory_path"`
	Class          memory.MemoryClass    `json:"class"`
	Availability   string                `json:"availability"`
	SourceMetadata memory.SourceMetadata `json:"source_metadata"`
}
type DisclosedContextItem struct {
	Reference *ContextReference       `json:"reference,omitempty"`
	Memory    *DisclosedContextMemory `json:"memory,omitempty"`
	Citations []Citation              `json:"citations"`
}
type DisclosedContextMemory struct {
	ContextReference
	Content string `json:"content"`
}
type DisclosedContextResult struct {
	TurnID            string                  `json:"turn_id,omitempty"`
	ContextDigest     ContextDigest           `json:"context_digest"`
	Continuity        ContextContinuityResult `json:"continuity"`
	BudgetAccounting  ContextBudgetAccounting `json:"budget_accounting"`
	ResponseMode      string                  `json:"response_mode"`
	Disclosure        auth.DisclosureDecision `json:"disclosure"`
	Profile           []DisclosedContextItem  `json:"profile"`
	RecentSession     []DisclosedContextItem  `json:"recent_session"`
	RecentEpisodes    []DisclosedContextItem  `json:"recent_episodes"`
	RelevantSummaries []DisclosedContextItem  `json:"relevant_summaries"`
	RelatedEntities   []DisclosedContextItem  `json:"related_entities"`
	Citations         []Citation              `json:"citations"`
}
type DisclosedContextResponse struct {
	Metadata  OperationMetadata      `json:"metadata"`
	Result    DisclosedContextResult `json:"result"`
	Citations []Citation             `json:"citations"`
}

// Selection is owned by retrieval. This projection accepts only its authorized,
// current-version results; unsupported insight/goal text has no disclosure path.
func ShapeDisclosedContextResponse(out retrieval.AssembledContext, meta OperationMetadata, decision auth.DisclosureDecision, citationLimit int, expectedScopes ...memory.Scope) DisclosedContextResponse {
	citations := []Citation{}
	seen := map[Citation]bool{}
	project := func(hits []retrieval.SearchHit) []DisclosedContextItem {
		items := make([]DisclosedContextItem, 0, len(hits))
		for _, hit := range hits {
			m := hit.Memory
			if len(expectedScopes) > 0 && m.Scope != expectedScopes[0] {
				continue
			}
			path, pathErr := memory.NormalizeMemoryPath(m.MemoryPath)
			if m.State != memory.MemoryStateActive || !boundedToken(m.ID, MaxOperationTokenBytes) || !disclosedMemoryClass(m.Class) || m.SourceVersion <= 0 || m.Scope.Validate() != nil || pathErr != nil || path != m.MemoryPath || !m.SourceMetadata.Effective().Disclosable() {
				continue
			}
			ref := ContextReference{Kind: "memory", MemoryID: m.ID, MemoryPath: m.MemoryPath, Class: m.Class, Availability: "available", SourceMetadata: m.SourceMetadata.Effective()}
			if m.SourceVersion > 0 {
				ref.Version = strconv.FormatInt(m.SourceVersion, 10)
			}
			item := DisclosedContextItem{Citations: []Citation{}}
			if decision.ResponseMode == auth.ResponseModeBoundedContent {
				item.Memory = &DisclosedContextMemory{ContextReference: ref, Content: m.Content}
			} else {
				item.Reference = &ref
			}
			// Raw/provenance evidence requires independent checks. Only current selected
			// canonical identity is intrinsically authorized by this item.
			c := Citation{SourceKind: "canonical_memory", Reference: m.ID, Version: ref.Version, Availability: "available"}
			item.Citations = append(item.Citations, c)
			if !seen[c] && len(citations) < citationLimit {
				seen[c] = true
				citations = append(citations, c)
			}
			items = append(items, item)
		}
		return items
	}
	result := DisclosedContextResult{ResponseMode: decision.ResponseMode, Disclosure: decision, Profile: project(out.Profile), RecentSession: project(out.RecentSession), RecentEpisodes: project(out.RecentEpisodes), RelevantSummaries: project(out.RelevantSummaries), RelatedEntities: project(out.RelatedEntities), Citations: citations}
	return DisclosedContextResponse{Metadata: SafeOperationMetadata(meta), Result: result, Citations: citations}
}

func disclosedMemoryClass(class memory.MemoryClass) bool {
	switch class {
	case memory.MemoryClassProfile, memory.MemoryClassEpisodic, memory.MemoryClassProcedural, memory.MemoryClassSummary, memory.MemoryClassRelation:
		return true
	default:
		return false
	}
}

// Attribution tokens are bounded by request validation. Paths and free-form
// policy fields are protected metadata and do not belong in acknowledgments.
func SafeOperationMetadata(meta OperationMetadata) OperationMetadata {
	return OperationMetadata{RequestID: meta.RequestID, OperationID: meta.OperationID, SchemaVersion: meta.SchemaVersion, IdempotencyKey: meta.IdempotencyKey, EventSeq: meta.EventSeq}
}
