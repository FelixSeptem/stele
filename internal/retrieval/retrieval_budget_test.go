package retrieval

import (
	"testing"
	"time"
)

func TestRetrievalBudgetLedgerAccountsSharedEnvelope(t *testing.T) {
	startedAt := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	ledger, err := NewRetrievalBudgetLedger(RetrievalBudgetEnvelope{
		TotalCandidates: 100,
		ChannelCandidates: map[FusionChannel]int{
			FusionChannelLexical:  40,
			FusionChannelSemantic: 40,
			FusionChannelRelation: 20,
		},
		RerankerHeadroom: 20,
		MaxPasses:        2,
		LatencyBudget:    time.Second,
	}, startedAt)
	if err != nil {
		t.Fatalf("NewRetrievalBudgetLedger() error = %v", err)
	}

	if err := ledger.ConsumeChannel(1, FusionChannelLexical, 30, startedAt.Add(100*time.Millisecond)); err != nil {
		t.Fatalf("ConsumeChannel(lexical) error = %v", err)
	}
	if err := ledger.ConsumeChannel(1, FusionChannelSemantic, 40, startedAt.Add(200*time.Millisecond)); err != nil {
		t.Fatalf("ConsumeChannel(semantic) error = %v", err)
	}
	if got := ledger.RemainingCandidates(); got != 30 {
		t.Fatalf("RemainingCandidates() = %d, want 30", got)
	}
	if got := ledger.RemainingForReranker(); got != 20 {
		t.Fatalf("RemainingForReranker() = %d, want 20", got)
	}
	if got := ledger.RemainingForRetrieval(); got != 10 {
		t.Fatalf("RemainingForRetrieval() = %d, want 10", got)
	}
	if got := ledger.UnusedChannelAllocation(FusionChannelLexical); got != 10 {
		t.Fatalf("UnusedChannelAllocation(lexical) = %d, want 10", got)
	}
	if err := ledger.ConsumeChannel(2, FusionChannelRelation, 10, startedAt.Add(300*time.Millisecond)); err != nil {
		t.Fatalf("ConsumeChannel(follow-up relation) error = %v", err)
	}
	if got := ledger.RemainingForRetrieval(); got != 0 {
		t.Fatalf("RemainingForRetrieval() = %d, want 0", got)
	}
}

func TestRetrievalBudgetLedgerRejectsOverConsumptionAndExpiredWork(t *testing.T) {
	startedAt := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	newLedger := func(t *testing.T) *RetrievalBudgetLedger {
		t.Helper()
		ledger, err := NewRetrievalBudgetLedger(RetrievalBudgetEnvelope{
			TotalCandidates:   50,
			ChannelCandidates: map[FusionChannel]int{FusionChannelLexical: 25, FusionChannelSemantic: 25},
			RerankerHeadroom:  10,
			MaxPasses:         2,
			LatencyBudget:     250 * time.Millisecond,
		}, startedAt)
		if err != nil {
			t.Fatalf("NewRetrievalBudgetLedger() error = %v", err)
		}
		return ledger
	}

	tests := []struct {
		name string
		run  func(*RetrievalBudgetLedger) error
	}{
		{name: "channel allocation", run: func(ledger *RetrievalBudgetLedger) error {
			return ledger.ConsumeChannel(1, FusionChannelLexical, 26, startedAt)
		}},
		{name: "undeclared channel", run: func(ledger *RetrievalBudgetLedger) error {
			return ledger.ConsumeChannel(1, FusionChannelRelation, 1, startedAt)
		}},
		{name: "invalid pass", run: func(ledger *RetrievalBudgetLedger) error {
			return ledger.ConsumeChannel(3, FusionChannelLexical, 1, startedAt)
		}},
		{name: "deadline", run: func(ledger *RetrievalBudgetLedger) error {
			return ledger.ConsumeChannel(1, FusionChannelLexical, 1, startedAt.Add(time.Second))
		}},
		{name: "reranker reserve", run: func(ledger *RetrievalBudgetLedger) error { return ledger.ConsumeReranker(11, startedAt) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(newLedger(t)); err == nil {
				t.Fatal("operation error = nil, want budget error")
			}
		})
	}
}

func TestRetrievalBudgetLedgerCanRedistributeOnlyUnusedRetrievalHeadroom(t *testing.T) {
	startedAt := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	ledger, err := NewRetrievalBudgetLedger(RetrievalBudgetEnvelope{
		TotalCandidates:   60,
		ChannelCandidates: map[FusionChannel]int{FusionChannelLexical: 30, FusionChannelSemantic: 30},
		RerankerHeadroom:  10,
		MaxPasses:         2,
		LatencyBudget:     time.Second,
	}, startedAt)
	if err != nil {
		t.Fatalf("NewRetrievalBudgetLedger() error = %v", err)
	}
	if err := ledger.ConsumeChannel(1, FusionChannelLexical, 10, startedAt); err != nil {
		t.Fatalf("ConsumeChannel() error = %v", err)
	}
	if err := ledger.Redistribute(FusionChannelLexical, FusionChannelSemantic, 15); err != nil {
		t.Fatalf("Redistribute() error = %v", err)
	}
	if got := ledger.ChannelLimit(FusionChannelLexical); got != 15 {
		t.Fatalf("lexical ChannelLimit() = %d, want 15", got)
	}
	if got := ledger.ChannelLimit(FusionChannelSemantic); got != 45 {
		t.Fatalf("semantic ChannelLimit() = %d, want 45", got)
	}
	if err := ledger.Redistribute(FusionChannelLexical, FusionChannelSemantic, 6); err == nil {
		t.Fatal("Redistribute() error = nil, want consumed-allocation error")
	}
}

func TestRetrievalBudgetLedgerTracksRequestedAndAcceptedCandidatesSeparately(t *testing.T) {
	startedAt := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	ledger, err := NewRetrievalBudgetLedger(RetrievalBudgetEnvelope{
		TotalCandidates:   4,
		ChannelCandidates: map[FusionChannel]int{FusionChannelLexical: 4},
		MaxPasses:         2,
		LatencyBudget:     time.Second,
	}, startedAt)
	if err != nil {
		t.Fatalf("NewRetrievalBudgetLedger() error = %v", err)
	}

	if err := ledger.ReserveChannelRequest(1, FusionChannelLexical, 3, startedAt); err != nil {
		t.Fatalf("ReserveChannelRequest(pass one) error = %v", err)
	}
	if err := ledger.RecordChannelAccepted(1, FusionChannelLexical, 1, startedAt); err != nil {
		t.Fatalf("RecordChannelAccepted(pass one) error = %v", err)
	}
	if got := ledger.RequestedForChannel(FusionChannelLexical); got != 3 {
		t.Fatalf("RequestedForChannel() = %d, want 3", got)
	}
	if got := ledger.AcceptedForChannel(FusionChannelLexical); got != 1 {
		t.Fatalf("AcceptedForChannel() = %d, want 1", got)
	}
	if got := ledger.RequestedForPass(1); got != 3 {
		t.Fatalf("RequestedForPass(1) = %d, want 3", got)
	}
	if got := ledger.AcceptedForPass(1); got != 1 {
		t.Fatalf("AcceptedForPass(1) = %d, want 1", got)
	}
	if got := ledger.UnacceptedChannelRequestsForPass(1, FusionChannelLexical); got != 2 {
		t.Fatalf("UnacceptedChannelRequestsForPass(1) = %d, want 2", got)
	}
	if got := ledger.RemainingForRetrieval(); got != 1 {
		t.Fatalf("RemainingForRetrieval() = %d, want 1 requested slot", got)
	}
	if got := ledger.UnusedChannelAllocation(FusionChannelLexical); got != 1 {
		t.Fatalf("UnusedChannelAllocation() = %d, want 1 requested slot", got)
	}
	if err := ledger.ReserveChannelRequest(2, FusionChannelLexical, 2, startedAt); err == nil {
		t.Fatal("ReserveChannelRequest(over envelope) error = nil")
	}
	if err := ledger.ReserveChannelRequest(2, FusionChannelLexical, 1, startedAt); err != nil {
		t.Fatalf("ReserveChannelRequest(pass two) error = %v", err)
	}
	if err := ledger.RecordChannelAccepted(2, FusionChannelLexical, 2, startedAt); err == nil {
		t.Fatal("RecordChannelAccepted(over requested work) error = nil")
	}
	if got := ledger.RemainingForRetrieval(); got != 0 {
		t.Fatalf("RemainingForRetrieval() = %d, want 0", got)
	}
}

func TestRetrievalBudgetLedgerReservesExplicitFallbackInsideSharedEnvelope(t *testing.T) {
	startedAt := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	ledger, err := NewRetrievalBudgetLedger(RetrievalBudgetEnvelope{
		TotalCandidates:           6,
		ChannelCandidates:         map[FusionChannel]int{FusionChannelSemantic: 6},
		FallbackChannelCandidates: map[FusionChannel]int{FusionChannelLexical: 2},
		RerankerHeadroom:          1,
		MaxPasses:                 1,
		LatencyBudget:             time.Second,
	}, startedAt)
	if err != nil {
		t.Fatalf("NewRetrievalBudgetLedger() error = %v", err)
	}
	if got := ledger.RemainingForRetrieval(); got != 3 {
		t.Fatalf("RemainingForRetrieval() = %d, want 3", got)
	}
	if err := ledger.ReserveChannelRequest(1, FusionChannelSemantic, 3, startedAt); err != nil {
		t.Fatalf("ReserveChannelRequest() error = %v", err)
	}
	if err := ledger.ReserveBaseline(FusionChannelLexical, 2, startedAt); err != nil {
		t.Fatalf("ReserveBaseline() error = %v", err)
	}
	if err := ledger.ConsumeReranker(1, startedAt); err != nil {
		t.Fatalf("ConsumeReranker() error = %v", err)
	}
	if got := ledger.RemainingCandidates(); got != 0 {
		t.Fatalf("RemainingCandidates() = %d, want 0", got)
	}
	if err := ledger.ReserveBaseline(FusionChannelLexical, 1, startedAt); err == nil {
		t.Fatal("ReserveBaseline(over channel allocation) error = nil")
	}
}

func TestRetrievalBudgetLedgerRejectsInvalidOrExpiredFallbackWork(t *testing.T) {
	startedAt := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	newLedger := func(t *testing.T) *RetrievalBudgetLedger {
		t.Helper()
		ledger, err := NewRetrievalBudgetLedger(RetrievalBudgetEnvelope{
			TotalCandidates:           4,
			ChannelCandidates:         map[FusionChannel]int{FusionChannelSemantic: 4},
			FallbackChannelCandidates: map[FusionChannel]int{FusionChannelLexical: 1},
			MaxPasses:                 1,
			LatencyBudget:             time.Millisecond,
		}, startedAt)
		if err != nil {
			t.Fatal(err)
		}
		return ledger
	}
	if err := newLedger(t).ReserveBaseline(FusionChannelRelation, 1, startedAt); err == nil {
		t.Fatal("ReserveBaseline(undeclared channel) error = nil")
	}
	if err := newLedger(t).ReserveBaseline(FusionChannelLexical, 2, startedAt); err == nil {
		t.Fatal("ReserveBaseline(over allocation) error = nil")
	}
	if err := newLedger(t).ReserveBaseline(FusionChannelLexical, 1, startedAt.Add(time.Second)); err == nil {
		t.Fatal("ReserveBaseline(expired) error = nil")
	}
}
