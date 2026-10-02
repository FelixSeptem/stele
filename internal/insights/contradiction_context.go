package insights

import (
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// ContradictionContextRequest is an explicit authorization boundary. Ordinary
// retrieval must leave this request nil or Unauthorized; only a reviewed,
// fresh, active contradiction can cross it.
type ContradictionContextRequest struct {
	Scope           memory.Scope
	Authorized      bool
	Now             time.Time
	SourceWatermark string
	MaxItems        int
}

func AuthorizedContradictionContext(request ContradictionContextRequest, candidates []memory.DerivedInsight) []memory.DerivedInsight {
	if !request.Authorized || request.Scope.Validate() != nil || request.MaxItems <= 0 {
		return nil
	}
	result := make([]memory.DerivedInsight, 0, minInt(request.MaxItems, len(candidates)))
	for _, candidate := range candidates {
		if candidate.Type != memory.DerivedInsightTypeContradiction || candidate.State != memory.DerivedInsightStateActive || candidate.Scope.Normalized() != request.Scope.Normalized() {
			continue
		}
		metadata := candidate.Derivation.Metadata
		if strings.TrimSpace(metadataString(metadata, "contradiction_review_state")) != string(ContradictionReviewConfirmed) || strings.TrimSpace(metadataString(metadata, "contradiction_temporal_disposition")) != "contradiction" {
			continue
		}
		if request.SourceWatermark == "" || metadataString(metadata, "source_watermark") != request.SourceWatermark {
			continue
		}
		if request.Now.IsZero() || candidate.UpdatedAt.IsZero() || candidate.UpdatedAt.After(request.Now) {
			continue
		}
		result = append(result, candidate)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if !result[i].UpdatedAt.Equal(result[j].UpdatedAt) {
			return result[i].UpdatedAt.After(result[j].UpdatedAt)
		}
		return result[i].ID < result[j].ID
	})
	if len(result) > request.MaxItems {
		result = result[:request.MaxItems]
	}
	return result
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
