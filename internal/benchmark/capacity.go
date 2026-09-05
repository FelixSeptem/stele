package benchmark

import "fmt"

// LongMemEvalCapacity is checked before a large corpus is imported. The
// standard s subset is admitted within its declared local event budget; m and
// oracle-sized runs need an explicit operator allowance.
type LongMemEvalCapacity struct {
	Subset          string `json:"subset"`
	RequestedEvents int    `json:"requested_events"`
	MaxEvents       int    `json:"max_events"`
	AllowLarge      bool   `json:"allow_large"`
	BatchSize       int    `json:"batch_size,omitempty"`
}

type LongMemEvalPlan struct {
	Status             Status `json:"status"`
	EffectiveBatchSize int    `json:"effective_batch_size"`
	BatchCount         int    `json:"batch_count"`
	RequiresCleanup    bool   `json:"requires_cleanup"`
}

func PreflightLongMemEval(capacity LongMemEvalCapacity) Status {
	if (capacity.Subset != "s" && capacity.Subset != "m" && capacity.Subset != "oracle") || capacity.RequestedEvents < 0 || capacity.MaxEvents <= 0 || capacity.BatchSize < 0 {
		return StatusInvalidManifest
	}
	if capacity.RequestedEvents > capacity.MaxEvents {
		return StatusCapacityRefused
	}
	if (capacity.Subset == "m" || capacity.Subset == "oracle") && !capacity.AllowLarge {
		return StatusCapacityRefused
	}
	return StatusSuccess
}

// PlanLongMemEval turns an admitted local capacity request into a bounded
// batch plan. It does not import any data, so a refusal is safe to call before
// databases, embeddings, or disk-heavy cache paths are touched.
func PlanLongMemEval(capacity LongMemEvalCapacity) (LongMemEvalPlan, error) {
	plan := LongMemEvalPlan{Status: PreflightLongMemEval(capacity)}
	if plan.Status != StatusSuccess {
		return plan, nil
	}
	batchSize := capacity.BatchSize
	if batchSize == 0 {
		batchSize = 100
	}
	if batchSize <= 0 {
		return LongMemEvalPlan{Status: StatusInvalidManifest}, fmt.Errorf("LongMemEval batch size must be positive")
	}
	plan.EffectiveBatchSize = batchSize
	plan.BatchCount = (capacity.RequestedEvents + batchSize - 1) / batchSize
	plan.RequiresCleanup = capacity.RequestedEvents > 0
	return plan, nil
}
