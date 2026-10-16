package jobs

import (
	"context"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
)

type ProjectionFeedbackReconciler interface {
	ReconcileProjectionFeedback(context.Context, memory.Scope, int) (int, error)
}

type ProjectionFeedbackScopeLister interface {
	ListProjectionFeedbackScopes(context.Context, int) ([]memory.Scope, error)
}

// ProjectionFeedbackScopeSource adapts ledger-based fair discovery for the
// existing bounded scope dispatcher without changing other maintenance jobs.
type ProjectionFeedbackScopeSource struct {
	Store ProjectionFeedbackScopeLister
}

func (s ProjectionFeedbackScopeSource) ListMaintenanceScopes(ctx context.Context, limit int) ([]memory.Scope, error) {
	if s.Store == nil || limit < 1 {
		return nil, fmt.Errorf("projection feedback scope source unavailable")
	}
	return s.Store.ListProjectionFeedbackScopes(ctx, limit)
}

// Admission never runs this policy. Only the asynchronous scheduler maps
// currently eligible evidence into the existing feedback source log.
type ProjectionFeedbackJob struct {
	Scope memory.Scope
	Store ProjectionFeedbackReconciler
	Limit int
}

func (ProjectionFeedbackJob) Name() string { return "provider_projection_feedback" }
func (j ProjectionFeedbackJob) Run(ctx context.Context) (int, error) {
	if j.Store == nil || j.Scope.Validate() != nil || j.Limit < 1 || j.Limit > 100 {
		return 0, fmt.Errorf("projection feedback dependency or bounds unavailable")
	}
	return j.Store.ReconcileProjectionFeedback(ctx, j.Scope, j.Limit)
}
