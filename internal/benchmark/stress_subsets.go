package benchmark

import (
	"encoding/json"
	"fmt"
)

// GenerateNeedleStressCases creates deterministic, bounded long-context cases
// without downloading a corpus or invoking a model.
func GenerateNeedleStressCases(dataset string, contextLengths []int, needleCount int) []StressCase {
	if needleCount <= 0 {
		needleCount = 1
	}
	result := make([]StressCase, 0, len(contextLengths)*2)
	for _, length := range contextLengths {
		for index, position := range []float64{0.1, 0.9} {
			result = append(result, StressCase{ID: fmt.Sprintf("%s-%d-%d", dataset, length, index+1), ContextTokens: length, NeedleCount: needleCount, Position: position, Mode: "text"})
		}
	}
	return result
}

type MRCRSubset struct {
	Dataset       string       `json:"dataset"`
	Subset        string       `json:"subset"`
	ContextTokens int          `json:"context_tokens"`
	SampleCount   int          `json:"sample_count"`
	NeedleCount   int          `json:"needle_count"`
	Cases         []StressCase `json:"cases,omitempty"`
}

func LoadMRCRSubset(source []byte) (MRCRSubset, error) {
	var subset MRCRSubset
	if err := json.Unmarshal(source, &subset); err != nil {
		return MRCRSubset{}, fmt.Errorf("decode MRCR subset: %w", err)
	}
	if subset.Dataset == "" {
		subset.Dataset = "mrcr"
	}
	if subset.Subset == "" || subset.ContextTokens <= 0 || subset.SampleCount < 0 {
		return MRCRSubset{}, fmt.Errorf("MRCR subset metadata is incomplete")
	}
	return subset, nil
}

type LongBenchSubset struct {
	Dataset       string `json:"dataset"`
	Subset        string `json:"subset"`
	ContextTokens int    `json:"context_tokens"`
	SampleCount   int    `json:"sample_count"`
	License       string `json:"license"`
}

func EvaluateLongBenchCapacity(subset LongBenchSubset, budget StressBudget) StressReport {
	cases := []StressCase{{ID: subset.Subset, ContextTokens: subset.ContextTokens, Mode: "text"}}
	report := EvaluateStress(subset.Dataset, cases, StressBudget{MaxContextTokens: budget.MaxContextTokens, MaxSamples: budget.MaxSamples}, true)
	if budget.MaxSamples > 0 && subset.SampleCount > budget.MaxSamples {
		report.Failures = append(report.Failures, "sample budget exceeded")
		report.Status = StatusCapacityRefused
	}
	if subset.ContextTokens <= 0 || subset.SampleCount <= 0 || subset.License == "" {
		report.Status = StatusPrerequisiteMissing
	}
	return report
}
