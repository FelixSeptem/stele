package jobs

import (
	"context"
	"errors"
	"testing"
)

type contextReceiptCleanerStub struct {
	limit int
	err   error
}

func (s *contextReceiptCleanerStub) CleanupContextReceipts(_ context.Context, limit int) (int, error) {
	s.limit = limit
	return 3, s.err
}

func TestContextContinuityCleanupJobValidatesDependencies(t *testing.T) {
	if _, err := (ContextContinuityCleanupJob{}).Run(context.Background()); !errors.Is(err, ErrContextContinuityCleanupUnavailable) {
		t.Fatalf("missing store error = %v", err)
	}
	if _, err := (ContextContinuityCleanupJob{Store: &contextReceiptCleanerStub{}}).Run(context.Background()); !errors.Is(err, ErrContextContinuityCleanupUnavailable) {
		t.Fatalf("invalid limit error = %v", err)
	}
}

func TestContextContinuityCleanupJobUsesBoundedStore(t *testing.T) {
	store := &contextReceiptCleanerStub{}
	processed, err := (ContextContinuityCleanupJob{Store: store, Limit: 500}).Run(context.Background())
	if err != nil || processed != 3 || store.limit != 500 {
		t.Fatalf("cleanup result = (%d, %v), limit=%d", processed, err, store.limit)
	}
}
