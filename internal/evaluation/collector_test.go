package evaluation

import (
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestCollectTrajectoryFailureIsDegradedAndNonAuthoritative(t *testing.T) {
	result := CollectTrajectory(TrajectoryInput{Scope: memory.Scope{}, Identity: CompatibilityIdentity{}})
	if result.Err == nil || result.Status != CollectionDegraded || result.Authoritative {
		t.Fatalf("collection failure must be degraded and non-authoritative: %#v", result)
	}
}
