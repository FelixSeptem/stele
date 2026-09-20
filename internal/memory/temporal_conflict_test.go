package memory

import (
	"errors"
	"testing"
	"time"
)

func temporalValidityFor(factID string, from time.Time, to *time.Time) TemporalValidity {
	return TemporalValidity{
		TemporalFactID: factID,
		IngestedAt:     from,
		ValidFrom:      from,
		ValidTo:        to,
		ValiditySource: TemporalValiditySourceExplicit,
	}
}

func TestTemporalConflictDispositionIsAClosedSet(t *testing.T) {
	for _, disposition := range []TemporalConflictDisposition{
		TemporalConflictNone, TemporalConflictRejected, TemporalConflictOpen, TemporalConflictResolved,
	} {
		if !disposition.Valid() {
			t.Fatalf("declared disposition %q must be valid", disposition)
		}
	}
	for _, disposition := range []TemporalConflictDisposition{"", "conflict", "CONFLICT_OPEN", "conflict_open "} {
		if disposition.Valid() {
			t.Fatalf("disposition %q must be rejected", disposition)
		}
	}
}

// TestTemporalConflictOpenSuppressesCurrentRetrieval pins the safety property:
// an unreviewed conflict must be withheld from ordinary current results.
func TestTemporalConflictOpenSuppressesCurrentRetrieval(t *testing.T) {
	if !TemporalConflictOpen.SuppressesCurrentRetrieval() {
		t.Fatal("an open conflict must suppress current retrieval")
	}
	for _, disposition := range []TemporalConflictDisposition{
		TemporalConflictNone, TemporalConflictRejected, TemporalConflictResolved,
	} {
		if disposition.SuppressesCurrentRetrieval() {
			t.Fatalf("disposition %q must not suppress current retrieval", disposition)
		}
	}
}

func TestTemporalIntervalOverlapUsesHalfOpenBounds(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	hour := base.Add(time.Hour)
	twoHours := base.Add(2 * time.Hour)
	threeHours := base.Add(3 * time.Hour)

	tests := []struct {
		name  string
		left  TemporalValidity
		right TemporalValidity
		want  bool
	}{
		{
			name:  "touching at a boundary does not overlap",
			left:  temporalValidityFor("fact", base, &hour),
			right: temporalValidityFor("fact", hour, &twoHours),
		},
		{
			name:  "identical intervals overlap",
			left:  temporalValidityFor("fact", base, &hour),
			right: temporalValidityFor("fact", base, &hour),
			want:  true,
		},
		{
			name:  "partial overlap",
			left:  temporalValidityFor("fact", base, &twoHours),
			right: temporalValidityFor("fact", hour, &threeHours),
			want:  true,
		},
		{
			name:  "contained interval overlaps",
			left:  temporalValidityFor("fact", base, &threeHours),
			right: temporalValidityFor("fact", hour, &twoHours),
			want:  true,
		},
		{
			name:  "disjoint intervals do not overlap",
			left:  temporalValidityFor("fact", base, &hour),
			right: temporalValidityFor("fact", twoHours, &threeHours),
		},
		{
			name:  "open ended interval overlaps a later interval",
			left:  temporalValidityFor("fact", base, nil),
			right: temporalValidityFor("fact", twoHours, &threeHours),
			want:  true,
		},
		{
			name:  "open ended interval overlaps an earlier interval",
			left:  temporalValidityFor("fact", twoHours, nil),
			right: temporalValidityFor("fact", base, &hour),
		},
		{
			name:  "two open ended intervals overlap",
			left:  temporalValidityFor("fact", base, nil),
			right: temporalValidityFor("fact", hour, nil),
			want:  true,
		},
		{
			name:  "malformed left interval is never a conflict",
			left:  TemporalValidity{TemporalFactID: "fact", IngestedAt: base},
			right: temporalValidityFor("fact", base, &hour),
		},
		{
			name:  "malformed right interval is never a conflict",
			left:  temporalValidityFor("fact", base, &hour),
			right: TemporalValidity{TemporalFactID: "fact", IngestedAt: base, ValidFrom: base, ValidTo: func() *time.Time { v := base; return &v }(), ValiditySource: TemporalValiditySourceExplicit},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TemporalIntervalOverlap(tt.left, tt.right); got != tt.want {
				t.Fatalf("TemporalIntervalOverlap() = %v, want %v", got, tt.want)
			}
			// Overlap is symmetric.
			if got := TemporalIntervalOverlap(tt.right, tt.left); got != tt.want {
				t.Fatalf("TemporalIntervalOverlap() is not symmetric: reverse = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTemporalOverlapConflictFailsClosedWithoutAuthorization is the core 2.4
// guarantee: an implicit overlapping current interval is refused unless the
// caller explicitly authorizes a conflict disposition.
func TestTemporalOverlapConflictFailsClosedWithoutAuthorization(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	hour := base.Add(time.Hour)
	existing := temporalValidityFor("fact-1", base, nil)
	overlapping := temporalValidityFor("fact-1", hour, nil)

	disposition, err := TemporalOverlapConflict(existing, overlapping, false)
	if !errors.Is(err, ErrTemporalOverlapUnresolved) {
		t.Fatalf("error = %v, want ErrTemporalOverlapUnresolved", err)
	}
	if disposition != TemporalConflictOpen {
		t.Fatalf("disposition = %q, want %q", disposition, TemporalConflictOpen)
	}
	if !disposition.SuppressesCurrentRetrieval() {
		t.Fatal("an unresolved overlap must suppress current retrieval")
	}
}

func TestTemporalOverlapConflictResolvesWhenAuthorized(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	hour := base.Add(time.Hour)
	existing := temporalValidityFor("fact-1", base, nil)
	overlapping := temporalValidityFor("fact-1", hour, nil)

	disposition, err := TemporalOverlapConflict(existing, overlapping, true)
	if err != nil {
		t.Fatalf("authorized overlap returned error = %v", err)
	}
	if disposition != TemporalConflictResolved {
		t.Fatalf("disposition = %q, want %q", disposition, TemporalConflictResolved)
	}
	if disposition.SuppressesCurrentRetrieval() {
		t.Fatal("a resolved conflict must not suppress current retrieval")
	}
}

func TestTemporalOverlapConflictIgnoresDistinctFactsAndDisjointIntervals(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	hour := base.Add(time.Hour)
	twoHours := base.Add(2 * time.Hour)
	threeHours := base.Add(3 * time.Hour)

	tests := []struct {
		name      string
		existing  TemporalValidity
		proposed  TemporalValidity
		authorize bool
	}{
		{name: "distinct fact identity", existing: temporalValidityFor("fact-1", base, nil), proposed: temporalValidityFor("fact-2", base, nil)},
		{name: "disjoint intervals", existing: temporalValidityFor("fact-1", base, &hour), proposed: temporalValidityFor("fact-1", twoHours, &threeHours)},
		{name: "distinct fact and authorized", existing: temporalValidityFor("fact-1", base, nil), proposed: temporalValidityFor("fact-2", base, nil), authorize: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disposition, err := TemporalOverlapConflict(tt.existing, tt.proposed, tt.authorize)
			if err != nil {
				t.Fatalf("error = %v, want nil for a non-conflicting write", err)
			}
			if disposition != TemporalConflictNone {
				t.Fatalf("disposition = %q, want %q", disposition, TemporalConflictNone)
			}
		})
	}
}

// TestTemporalOverlapConflictRejectsMalformedIntervals proves a malformed
// interval is refused outright rather than being recorded as a conflict; a
// conflict ledger must not accumulate noise from invalid input.
func TestTemporalOverlapConflictRejectsMalformedIntervals(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	valid := temporalValidityFor("fact-1", base, nil)

	tests := []struct {
		name      string
		existing  TemporalValidity
		proposed  TemporalValidity
		authorize bool
	}{
		{name: "malformed existing", existing: TemporalValidity{TemporalFactID: "fact-1", IngestedAt: base}, proposed: valid},
		{name: "malformed proposed", existing: valid, proposed: TemporalValidity{IngestedAt: base}},
		{name: "malformed existing authorized", existing: TemporalValidity{}, proposed: valid, authorize: true},
		{name: "closed proposed interval", existing: valid, proposed: temporalValidityFor("fact-1", base, &base)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disposition, err := TemporalOverlapConflict(tt.existing, tt.proposed, tt.authorize)
			if err == nil {
				t.Fatal("malformed interval was accepted")
			}
			if disposition != TemporalConflictRejected {
				t.Fatalf("disposition = %q, want %q", disposition, TemporalConflictRejected)
			}
			if disposition.SuppressesCurrentRetrieval() {
				t.Fatal("a rejected write must not create a suppressing conflict state")
			}
		})
	}
}
