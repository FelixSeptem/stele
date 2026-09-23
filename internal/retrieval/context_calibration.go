package retrieval

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/FelixSeptem/stele/internal/memory"
)

type ContextCalibrationDiagnostic struct {
	Status string
	Reason string
}

func ApplyContextCalibration(scope memory.Scope, candidates []SearchHit, summary memory.ContextCalibrationSummary, limit int) ([]SearchHit, []ContextCalibrationDiagnostic) {
	if err := scope.Validate(); err != nil || summary.Validate() != nil || summary.Scope.Normalized() != scope.Normalized() || summary.Freshness != memory.ContextCalibrationSummaryFresh || limit < 0 {
		return append([]SearchHit(nil), candidates...), []ContextCalibrationDiagnostic{{Status: "baseline_fallback", Reason: "missing, stale, malformed, or foreign calibration summary"}}
	}
	eligible := make([]SearchHit, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Memory.Scope.Normalized() != scope.Normalized() || candidate.Memory.State != memory.MemoryStateActive {
			continue
		}
		eligible = append(eligible, candidate)
	}
	if limit > 0 && len(eligible) > limit {
		eligible = eligible[:limit]
	}
	perCandidate := summary.PrioritySum / float64(maxInt(summary.EvidenceCount, 1))
	if perCandidate > .25 {
		perCandidate = .25
	}
	if perCandidate < -.25 {
		perCandidate = -.25
	}
	for index := range eligible {
		hash := sha256.Sum256([]byte(eligible[index].Memory.ID))
		bucket := float64(binary.BigEndian.Uint16(hash[:2]))/65535.0 - .5
		eligible[index].Score.Overall += perCandidate * bucket
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].Score.Overall == eligible[j].Score.Overall {
			return eligible[i].Memory.ID < eligible[j].Memory.ID
		}
		return eligible[i].Score.Overall > eligible[j].Score.Overall
	})
	return eligible, []ContextCalibrationDiagnostic{{Status: "applied", Reason: fmt.Sprintf("bounded exact-scope calibration applied to %d eligible candidates", len(eligible))}}
}
