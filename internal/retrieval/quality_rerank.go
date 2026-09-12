package retrieval

import (
	"fmt"
	"math"
)

type QualityFeatureVector struct {
	Version           string  `json:"version"`
	EvidenceCoverage  float64 `json:"evidence_coverage"`
	Freshness         float64 `json:"freshness"`
	SourceReliability float64 `json:"source_reliability"`
	Conflict          float64 `json:"conflict"`
	Usefulness        float64 `json:"usefulness"`
	TaskSuccess       float64 `json:"task_success"`
	Verification      float64 `json:"verification"`
}

type QualityAdjustmentBounds struct {
	PerFeature float64
	Total      float64
}

func (f QualityFeatureVector) Validate() error {
	if f.Version == "" {
		return fmt.Errorf("quality feature version is required")
	}
	for name, value := range map[string]float64{
		"evidence_coverage": f.EvidenceCoverage, "freshness": f.Freshness,
		"source_reliability": f.SourceReliability, "conflict": f.Conflict,
		"usefulness": f.Usefulness, "task_success": f.TaskSuccess, "verification": f.Verification,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < -1 || value > 1 {
			return fmt.Errorf("quality feature %s must be between -1 and 1", name)
		}
	}
	return nil
}

func ComputeQualityAdjustment(features QualityFeatureVector, bounds QualityAdjustmentBounds) (float64, error) {
	if err := features.Validate(); err != nil {
		return 0, err
	}
	if bounds.PerFeature <= 0 || bounds.Total <= 0 || math.IsNaN(bounds.PerFeature) || math.IsNaN(bounds.Total) {
		return 0, fmt.Errorf("quality adjustment bounds must be positive")
	}
	values := []float64{features.EvidenceCoverage, features.Freshness, features.SourceReliability, features.Conflict, features.Usefulness, features.TaskSuccess, features.Verification}
	// Conflict is represented as a penalty; all other dimensions contribute directly.
	values[3] = -values[3]
	total := 0.0
	for _, value := range values {
		contribution := value * bounds.PerFeature
		if contribution > bounds.PerFeature {
			contribution = bounds.PerFeature
		} else if contribution < -bounds.PerFeature {
			contribution = -bounds.PerFeature
		}
		total += contribution
	}
	if total > bounds.Total {
		return bounds.Total, nil
	}
	if total < -bounds.Total {
		return -bounds.Total, nil
	}
	return total, nil
}
