package jobs

import (
	"context"
	"errors"
)

var ErrContextContinuityCleanupUnavailable = errors.New("context continuity cleanup unavailable")

type ContextReceiptCleaner interface {
	CleanupContextReceipts(context.Context, int) (int, error)
}
type ContextContinuityCleanupJob struct {
	Store ContextReceiptCleaner
	Limit int
}

func (ContextContinuityCleanupJob) Name() string { return "provider_context_continuity_cleanup" }
func (j ContextContinuityCleanupJob) Run(ctx context.Context) (int, error) {
	if j.Store == nil || j.Limit < 1 {
		return 0, ErrContextContinuityCleanupUnavailable
	}
	return j.Store.CleanupContextReceipts(ctx, j.Limit)
}
