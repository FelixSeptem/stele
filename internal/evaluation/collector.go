package evaluation

type CollectionStatus string

const (
	CollectionCompleted CollectionStatus = "completed"
	CollectionDegraded  CollectionStatus = "degraded"
)

type CollectionResult struct {
	Status        CollectionStatus
	Aggregate     TrajectoryAggregate
	Err           error
	Authoritative bool
}

// CollectTrajectory is deliberately evaluation-side only. A collection error
// is represented as degraded evidence and never changes ordinary retrieval.
func CollectTrajectory(input TrajectoryInput) CollectionResult {
	aggregate, err := AggregateTrajectory(input)
	if err != nil {
		return CollectionResult{Status: CollectionDegraded, Err: err, Authoritative: false}
	}
	return CollectionResult{Status: CollectionCompleted, Aggregate: aggregate, Authoritative: false}
}
