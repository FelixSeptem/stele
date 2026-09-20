package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func temporalTestScope() memory.Scope {
	return memory.Scope{Tenant: "temporal-tenant", Project: "temporal-project", Namespace: "temporal-namespace"}
}

func temporalLexicalRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "tenant", "project", "namespace", "class", "state",
		"content", "created_at", "updated_at", "lexical_score",
	})
}

// TestTemporalSelectionForDefaultsToCurrentValid pins the default: an absent
// constraint must resolve to current-valid selection rather than "no filter",
// otherwise ordinary retrieval would start returning expired versions.
func TestTemporalSelectionForDefaultsToCurrentValid(t *testing.T) {
	instant := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	selection := temporalSelectionFor(retrieval.SearchInput{}, instant)

	if selection.ConstraintReject {
		t.Fatal("absent constraint must not reject")
	}
	if selection.Mode != string(memory.TemporalSelectionCurrent) {
		t.Fatalf("Mode = %q, want %q", selection.Mode, memory.TemporalSelectionCurrent)
	}
	if selection.EvaluationAt != instant {
		t.Fatalf("EvaluationAt = %v, want the supplied instant %v", selection.EvaluationAt, instant)
	}
	if selection.AsOf != nil || selection.ValidFrom != nil || selection.ValidTo != nil {
		t.Fatalf("default selection must not carry historical selectors: %+v", selection)
	}
}

// TestTemporalSelectionForCarriesHistoricalSelectors verifies as_of and
// valid_during reach SQL with their own arguments and leave the others unset.
func TestTemporalSelectionForCarriesHistoricalSelectors(t *testing.T) {
	instant := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	t.Run("as_of", func(t *testing.T) {
		selection := temporalSelectionFor(retrieval.SearchInput{
			TemporalConstraint: memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf, AsOf: &asOf},
		}, instant)
		if selection.ConstraintReject {
			t.Fatal("valid as_of constraint must not reject")
		}
		if selection.AsOf != asOf {
			t.Fatalf("AsOf = %v, want %v", selection.AsOf, asOf)
		}
		if selection.ValidFrom != nil || selection.ValidTo != nil {
			t.Fatalf("as_of must not carry interval bounds: %+v", selection)
		}
	})

	t.Run("valid_during", func(t *testing.T) {
		selection := temporalSelectionFor(retrieval.SearchInput{
			TemporalConstraint: memory.TemporalConstraint{Mode: memory.TemporalSelectionDuring, ValidFrom: &from, ValidTo: &to},
		}, instant)
		if selection.ConstraintReject {
			t.Fatal("valid valid_during constraint must not reject")
		}
		if selection.ValidFrom != from || selection.ValidTo != to {
			t.Fatalf("interval bounds = %v..%v, want %v..%v", selection.ValidFrom, selection.ValidTo, from, to)
		}
		if selection.AsOf != nil {
			t.Fatalf("valid_during must not carry as_of: %+v", selection)
		}
	})
}

// TestTemporalSelectionForRejectsMalformedConstraint proves a bad selector fails
// closed. Widening an ambiguous request into "return everything" would leak
// expired versions, so rejection is rendered as a false predicate.
func TestTemporalSelectionForRejectsMalformedConstraint(t *testing.T) {
	instant := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	from := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		constraint memory.TemporalConstraint
	}{
		{name: "as_of without instant", constraint: memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf}},
		{name: "ambiguous selectors", constraint: memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf, AsOf: &asOf, ValidFrom: &from, ValidTo: &to}},
		{name: "inverted interval", constraint: memory.TemporalConstraint{Mode: memory.TemporalSelectionDuring, ValidFrom: &from, ValidTo: &to}},
		{name: "unknown mode", constraint: memory.TemporalConstraint{Mode: "sometime"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection := temporalSelectionFor(retrieval.SearchInput{TemporalConstraint: tt.constraint}, instant)
			if !selection.ConstraintReject {
				t.Fatalf("malformed constraint was accepted: %+v", selection)
			}
			if predicate := temporalSQLPredicate("cm", selection, 8, 9, 10, 11, 12); predicate != "AND false" {
				t.Fatalf("malformed constraint predicate = %q, want a fail-closed predicate", predicate)
			}
		})
	}
}

// TestTemporalSQLPredicateCoversEveryMode guards the shared helper so a new mode
// cannot be added without a matching SQL branch.
func TestTemporalSQLPredicateCoversEveryMode(t *testing.T) {
	instant := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	predicate := temporalSQLPredicate("cm", temporalSelectionFor(retrieval.SearchInput{}, instant), 8, 9, 10, 11, 12)

	for _, mode := range []memory.TemporalSelectionMode{
		memory.TemporalSelectionCurrent,
		memory.TemporalSelectionAsOf,
		memory.TemporalSelectionDuring,
	} {
		if !strings.Contains(predicate, "$8 = '"+string(mode)+"'") {
			t.Errorf("predicate missing branch for %q: %s", mode, predicate)
		}
	}
	// Legacy rows must fall back to recorded time, never be filtered out.
	if !strings.Contains(predicate, "COALESCE(cm.valid_from, cm.created_at)") {
		t.Errorf("predicate must fall back to created_at for legacy rows: %s", predicate)
	}
	if !strings.Contains(predicate, "cm.valid_to IS NULL OR cm.valid_to >") {
		t.Errorf("predicate must treat a NULL valid_to as open-ended: %s", predicate)
	}
}

// TestSearchLexicalAppliesTemporalValidityPredicate proves the shared predicate
// actually reaches the executed statement and that one instant is bound.
func TestSearchLexicalAppliesTemporalValidityPredicate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	instant := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	repo := NewRepository(mock)
	repo.now = func() time.Time { return instant }

	mock.ExpectQuery(`COALESCE\(valid_from, created_at\) <= \$9::timestamptz`).
		WithArgs(
			"temporal-tenant", "temporal-project", "temporal-namespace",
			"query", nil, nil, 10,
			string(memory.TemporalSelectionCurrent), instant, nil, nil, nil,
		).
		WillReturnRows(temporalLexicalRows())

	if _, err := repo.SearchLexical(context.Background(), retrieval.SearchInput{
		Scope: temporalTestScope(),
		Query: "query",
		TopK:  10,
	}); err != nil {
		t.Fatalf("SearchLexical() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestSearchLexicalBindsAsOfSelectorWithoutReinterpretingRecordedTime proves an
// as_of request binds its own instant and still keeps the recorded-time window
// as recorded time, so the two filters stay independent.
func TestSearchLexicalBindsAsOfSelectorWithoutReinterpretingRecordedTime(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	instant := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	recordedFrom := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recordedTo := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)

	repo := NewRepository(mock)
	repo.now = func() time.Time { return instant }

	mock.ExpectQuery(`\$8 = 'as_of'`).
		WithArgs(
			"temporal-tenant", "temporal-project", "temporal-namespace",
			"query", recordedFrom, recordedTo, 10,
			string(memory.TemporalSelectionAsOf), instant, asOf, nil, nil,
		).
		WillReturnRows(temporalLexicalRows())

	if _, err := repo.SearchLexical(context.Background(), retrieval.SearchInput{
		Scope:              temporalTestScope(),
		Query:              "query",
		TopK:               10,
		TimeFrom:           recordedFrom,
		TimeTo:             recordedTo,
		TemporalConstraint: memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf, AsOf: &asOf},
	}); err != nil {
		t.Fatalf("SearchLexical() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

// TestSearchLexicalRejectsMalformedSelectorAtInputBoundary proves a malformed
// selector is refused by input validation rather than reaching storage with a
// predicate that could widen the query.
func TestSearchLexicalRejectsMalformedSelectorAtInputBoundary(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	repo := NewRepository(mock)
	repo.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

	// No query expectation: a rejected input must never touch the database.
	if _, err := repo.SearchLexical(context.Background(), retrieval.SearchInput{
		Scope:              temporalTestScope(),
		Query:              "query",
		TopK:               10,
		TemporalConstraint: memory.TemporalConstraint{Mode: memory.TemporalSelectionAsOf},
	}); err == nil {
		t.Fatal("SearchLexical() accepted an as_of constraint without an instant")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("a rejected input must not execute a query: %v", err)
	}
}

// TestTemporalSQLPredicateFailsClosedForUnreachableSelector documents the second
// layer of defence: even if a malformed constraint reached the predicate, it is
// rendered as an always-false condition instead of removing the filter.
func TestTemporalSQLPredicateFailsClosedForUnreachableSelector(t *testing.T) {
	selection := temporalSelection{ConstraintReject: true, Mode: "as_of"}
	if got := temporalSQLPredicate("cm", selection, 8, 9, 10, 11, 12); got != "AND false" {
		t.Fatalf("rejected selection predicate = %q, want always-false", got)
	}
}

// TestSearchSemanticAndRelationsApplyTheSameTemporalPredicate keeps the three
// search surfaces aligned; drift here is how expired versions sneak back in
// through a single unpatched path.
func TestSearchSemanticAndRelationsApplyTheSameTemporalPredicate(t *testing.T) {
	instant := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	selection := temporalSelectionFor(retrieval.SearchInput{}, instant)

	t.Run("semantic", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("pgxmock.NewPool() error = %v", err)
		}
		defer mock.Close()

		repo := NewRepository(mock)
		repo.now = func() time.Time { return instant }

		mock.ExpectQuery(`COALESCE\(cm.valid_from, cm.created_at\) <= \$9::timestamptz`).
			WithArgs(
				"temporal-tenant", "temporal-project", "temporal-namespace",
				pgxmock.AnyArg(), nil, nil, 10,
				selection.Mode, instant, nil, nil, nil,
			).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "tenant", "project", "namespace", "class", "state",
				"content", "created_at", "updated_at", "semantic_score",
			}))

		if _, err := repo.SearchSemantic(context.Background(), retrieval.SearchInput{
			Scope:          temporalTestScope(),
			Query:          "query",
			QueryEmbedding: []float32{1, 0, 0},
			TopK:           10,
		}); err != nil {
			t.Fatalf("SearchSemantic() error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("expectations: %v", err)
		}
	})

	t.Run("relations", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("pgxmock.NewPool() error = %v", err)
		}
		defer mock.Close()

		repo := NewRepository(mock)
		repo.now = func() time.Time { return instant }

		mock.ExpectQuery(`COALESCE\(cm.valid_from, cm.created_at\) <= \$13::timestamptz`).
			WithArgs(
				"temporal-tenant", "temporal-project", "temporal-namespace",
				"query", "query", "query", "query", "query", nil, nil, 10,
				selection.Mode, instant, nil, nil, nil,
			).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "tenant", "project", "namespace", "class", "state",
				"content", "created_at", "updated_at", "relation_score",
			}))

		if _, err := repo.SearchRelations(context.Background(), retrieval.SearchInput{
			Scope:            temporalTestScope(),
			Query:            "query",
			TopK:             10,
			IncludeRelations: true,
		}); err != nil {
			t.Fatalf("SearchRelations() error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("expectations: %v", err)
		}
	})
}
