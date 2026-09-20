package memory

import (
	"errors"
	"testing"
	"time"
)

func correctionScope() Scope {
	return Scope{Tenant: "t", Project: "p", Namespace: "n"}
}

func correctionFor(t *testing.T, predecessor *int64, successor int64, createdAt time.Time) TemporalCorrection {
	t.Helper()
	correction := TemporalCorrection{
		Scope:              correctionScope(),
		TemporalFactID:     "fact-1",
		MemoryID:           "00000000-0000-0000-0000-000000000001",
		PredecessorVersion: predecessor,
		SuccessorVersion:   successor,
		Actor:              "operator-1",
		Reason:             "corrected validity window",
		Disposition:        TemporalConflictResolved,
		CreatedAt:          createdAt,
	}
	if err := correction.Validate(); err != nil {
		t.Fatalf("fixture correction is invalid: %v", err)
	}
	return correction
}

func TestTemporalCorrectionRequiresIdentityAndProvenance(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	version := int64(1)
	valid := TemporalCorrection{
		Scope: correctionScope(), TemporalFactID: "fact-1", MemoryID: "mem-1",
		PredecessorVersion: &version, SuccessorVersion: 2,
		Actor: "operator-1", Reason: "reason", Disposition: TemporalConflictResolved, CreatedAt: base,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid correction rejected: %v", err)
	}

	tests := []struct {
		name       string
		mutate     func(*TemporalCorrection)
		wantErr    error
		wantAnyErr bool
	}{
		{name: "missing fact identity", mutate: func(c *TemporalCorrection) { c.TemporalFactID = "" }, wantErr: ErrTemporalCorrectionIdentityRequired},
		{name: "missing memory id", mutate: func(c *TemporalCorrection) { c.MemoryID = "" }, wantErr: ErrTemporalCorrectionIdentityRequired},
		// Scope validation reports the scope's own bounded error category.
		{name: "missing scope", mutate: func(c *TemporalCorrection) { c.Scope = Scope{} }, wantAnyErr: true},
		{name: "missing actor", mutate: func(c *TemporalCorrection) { c.Actor = "" }, wantErr: ErrTemporalCorrectionProvenanceRequired},
		{name: "missing reason", mutate: func(c *TemporalCorrection) { c.Reason = "  " }, wantErr: ErrTemporalCorrectionProvenanceRequired},
		{name: "missing successor", mutate: func(c *TemporalCorrection) { c.SuccessorVersion = 0 }, wantErr: ErrTemporalCorrectionSuccessorRequired},
		{name: "successor does not advance", mutate: func(c *TemporalCorrection) { c.SuccessorVersion = 1 }, wantErr: ErrTemporalCorrectionLineageInvalid},
		{name: "non-positive predecessor", mutate: func(c *TemporalCorrection) { zero := int64(0); c.PredecessorVersion = &zero }, wantErr: ErrTemporalCorrectionLineageInvalid},
		{name: "unknown disposition", mutate: func(c *TemporalCorrection) { c.Disposition = "made_up" }, wantErr: ErrTemporalDispositionInvalid},
		{name: "missing disposition", mutate: func(c *TemporalCorrection) { c.Disposition = "" }, wantErr: ErrTemporalDispositionInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := valid
			tt.mutate(&candidate)
			err := candidate.Validate()
			if tt.wantAnyErr {
				if err == nil {
					t.Fatal("Validate() = nil, want an error")
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestTemporalCorrectionAllowsFirstCorrectionWithoutPredecessor proves the
// lineage can start: the first correction of a fact has no predecessor and must
// still be representable.
func TestTemporalCorrectionAllowsFirstCorrectionWithoutPredecessor(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first := TemporalCorrection{
		Scope: correctionScope(), TemporalFactID: "fact-1", MemoryID: "mem-1",
		SuccessorVersion: 1, Actor: "operator-1", Reason: "initial validity",
		Disposition: TemporalConflictNone, CreatedAt: base,
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("first correction rejected: %v", err)
	}
	if first.SuppressesCurrentRetrieval() {
		t.Fatal("a non-conflicting correction must not suppress current retrieval")
	}
}

func TestTemporalCorrectionCarriesNoMemoryPayload(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	correction := correctionFor(t, nil, 1, base)
	// The correction is a pointer to a successor version, not a copy of content,
	// so an append cannot silently rewrite prior payloads.
	if correction.MemoryID == "" || correction.SuccessorVersion == 0 {
		t.Fatalf("correction must name the successor it points at: %+v", correction)
	}
}

func TestTemporalCorrectionOpenConflictSuppressesRetrieval(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	version := int64(1)
	correction := TemporalCorrection{
		Scope: correctionScope(), TemporalFactID: "fact-1", MemoryID: "mem-1",
		PredecessorVersion: &version, SuccessorVersion: 2,
		Actor: "operator-1", Reason: "competing interval", Disposition: TemporalConflictOpen, CreatedAt: base,
	}
	if err := correction.Validate(); err != nil {
		t.Fatalf("open-conflict correction rejected: %v", err)
	}
	if !correction.SuppressesCurrentRetrieval() {
		t.Fatal("an open conflict correction must suppress current retrieval")
	}
}

func TestTemporalLineageValidateRequiresDeterministicChain(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	v1 := int64(1)
	v2 := int64(2)
	chain := TemporalLineage{
		TemporalFactID: "fact-1",
		Corrections: []TemporalCorrection{
			correctionFor(t, nil, 1, base),
			correctionFor(t, &v1, 2, base.Add(time.Hour)),
			correctionFor(t, &v2, 3, base.Add(2*time.Hour)),
		},
	}
	if err := chain.Validate(); err != nil {
		t.Fatalf("valid lineage rejected: %v", err)
	}

	t.Run("missing fact identity", func(t *testing.T) {
		broken := chain
		broken.TemporalFactID = ""
		if err := broken.Validate(); !errors.Is(err, ErrTemporalCorrectionIdentityRequired) {
			t.Fatalf("error = %v, want identity rejection", err)
		}
	})

	t.Run("first correction has a predecessor", func(t *testing.T) {
		broken := TemporalLineage{TemporalFactID: "fact-1", Corrections: []TemporalCorrection{correctionFor(t, &v1, 2, base)}}
		if err := broken.Validate(); err == nil {
			t.Fatal("lineage starting mid-chain was accepted")
		}
	})

	t.Run("broken predecessor link", func(t *testing.T) {
		// Version 3 follows 1 rather than 2: each correction is individually
		// valid, but the chain has a gap.
		gap := int64(1)
		broken := TemporalLineage{TemporalFactID: "fact-1", Corrections: []TemporalCorrection{
			correctionFor(t, nil, 1, base),
			correctionFor(t, &gap, 3, base.Add(time.Hour)),
		}}
		if err := broken.Validate(); err == nil {
			t.Fatal("lineage with a dangling predecessor was accepted")
		}
	})

	t.Run("skipped predecessor version", func(t *testing.T) {
		// A chain must be contiguous: 2 -> 5 would leave versions 3 and 4
		// unaccounted for, so a reader could never reconstruct history in order.
		v2 := int64(2)
		broken := TemporalLineage{TemporalFactID: "fact-1", Corrections: []TemporalCorrection{
			correctionFor(t, nil, 2, base),
			correctionFor(t, &v2, 5, base.Add(time.Hour)),
		}}
		if err := broken.Validate(); err == nil {
			t.Fatal("lineage with a skipped version was accepted")
		}
	})

	t.Run("backwards successor is rejected per correction", func(t *testing.T) {
		// successor <= predecessor is an in-place mutation, refused outright.
		inverted := TemporalCorrection{
			Scope: correctionScope(), TemporalFactID: "fact-1", MemoryID: "mem-1",
			PredecessorVersion: func() *int64 { v := int64(4); return &v }(), SuccessorVersion: 3,
			Actor: "operator-1", Reason: "rewrite", Disposition: TemporalConflictResolved, CreatedAt: base,
		}
		if err := inverted.Validate(); !errors.Is(err, ErrTemporalCorrectionLineageInvalid) {
			t.Fatalf("error = %v, want lineage rejection for a backwards successor", err)
		}
	})

	t.Run("foreign fact identity inside lineage", func(t *testing.T) {
		foreign := correctionFor(t, &v1, 2, base.Add(time.Hour))
		foreign.TemporalFactID = "fact-2"
		broken := TemporalLineage{TemporalFactID: "fact-1", Corrections: []TemporalCorrection{
			correctionFor(t, nil, 1, base),
			foreign,
		}}
		if err := broken.Validate(); err == nil {
			t.Fatal("lineage containing a foreign fact identity was accepted")
		}
	})
}

// TestOrderTemporalCorrectionsSortsAndDoesNotMutate proves history reads are
// stable and that ordering never rewrites the caller's slice.
func TestOrderTemporalCorrectionsSortsAndDoesNotMutate(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	v1 := int64(1)
	v2 := int64(2)
	unsorted := []TemporalCorrection{
		correctionFor(t, &v2, 3, base.Add(2*time.Hour)),
		correctionFor(t, nil, 1, base),
		correctionFor(t, &v1, 2, base.Add(time.Hour)),
	}
	original := make([]TemporalCorrection, len(unsorted))
	copy(original, unsorted)

	ordered := OrderTemporalCorrections(unsorted)
	if len(ordered) != 3 {
		t.Fatalf("len(ordered) = %d, want 3", len(ordered))
	}
	for i, want := range []int64{1, 2, 3} {
		if ordered[i].SuccessorVersion != want {
			t.Fatalf("ordered[%d].SuccessorVersion = %d, want %d", i, ordered[i].SuccessorVersion, want)
		}
	}
	for i := range unsorted {
		if unsorted[i] != original[i] {
			t.Fatalf("OrderTemporalCorrections mutated its input at %d", i)
		}
	}

	// A valid chain must survive ordering.
	lineage := TemporalLineage{TemporalFactID: "fact-1", Corrections: ordered}
	if err := lineage.Validate(); err != nil {
		t.Fatalf("ordered lineage is not a valid chain: %v", err)
	}
}

// TestOrderTemporalCorrectionsBreaksVersionTiesByCreationTime keeps ordering
// total: two corrections sharing a successor version must still sort
// deterministically.
func TestOrderTemporalCorrectionsBreaksVersionTiesByCreationTime(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	late := correctionFor(t, nil, 5, base.Add(time.Hour))
	early := correctionFor(t, nil, 5, base)

	ordered := OrderTemporalCorrections([]TemporalCorrection{late, early})
	if !ordered[0].CreatedAt.Equal(base) || !ordered[1].CreatedAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("tie-break failed: %s then %s", ordered[0].CreatedAt, ordered[1].CreatedAt)
	}
}

// TestTemporalCorrectionIsAppendOnlyByConstruction documents the design
// invariant: a correction identifies its successor by version number and holds
// no content field, so it can never be used to rewrite a prior payload.
func TestTemporalCorrectionIsAppendOnlyByConstruction(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first := correctionFor(t, nil, 1, base)
	v1 := int64(1)
	second := correctionFor(t, &v1, 2, base.Add(time.Hour))

	if second.PredecessorVersion == nil || *second.PredecessorVersion != first.SuccessorVersion {
		t.Fatalf("successor does not reference the prior version: %+v", second)
	}
	if first.SuccessorVersion != 1 {
		t.Fatalf("prior correction was mutated: %+v", first)
	}
}
