package retrieval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// Section 7.1 pins the temporal evaluation fixtures. The fixture validates
// before it is ever seeded, so an unsafe or ambiguous scenario fails locally
// instead of producing a green report that measured nothing.

const temporalFixturePath = "testdata/retrieval-temporal-evaluation-fixture-v1.json"

func loadTemporalFixture(t *testing.T) EvaluationFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(temporalFixturePath))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	var fixture EvaluationFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return fixture
}

// TestTemporalEvaluationFixtureCoversEveryKindAndValidates proves the shipped
// fixture is accepted and that it exercises every scenario the release gate
// reasons about. A fixture missing a kind would leave that scenario unmeasured.
func TestTemporalEvaluationFixtureCoversEveryKindAndValidates(t *testing.T) {
	fixture := loadTemporalFixture(t)
	if err := fixture.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	kinds := make(map[EvaluationTemporalCaseKind]int)
	for _, item := range fixture.Cases {
		if item.Temporal == nil {
			t.Fatalf("case %q has no temporal expectation", item.ID)
		}
		kinds[item.Temporal.Kind]++
	}
	for _, kind := range evaluationTemporalCaseKinds {
		if kinds[kind] != 1 {
			t.Fatalf("kind %q appears %d times, want exactly once", kind, kinds[kind])
		}
	}
	if len(kinds) != evaluationTemporalFixtureMinimum {
		t.Fatalf("fixture covers %d kinds, want %d", len(kinds), evaluationTemporalFixtureMinimum)
	}
}

// TestTemporalEvaluationFixtureSelectorsMatchTheirKind proves every selector is
// internally consistent and never restates the current mode, which would make a
// historical scenario silently equivalent to an ordinary search.
func TestTemporalEvaluationFixtureSelectorsMatchTheirKind(t *testing.T) {
	fixture := loadTemporalFixture(t)
	for _, item := range fixture.Cases {
		temporal := item.Temporal
		if temporal == nil {
			continue
		}
		if temporal.Kind.requiresSelector() {
			if temporal.Selector == nil {
				t.Fatalf("case %q: kind %q must declare a selector", item.ID, temporal.Kind)
			}
			constraint, err := temporal.Selector.Constraint()
			if err != nil {
				t.Fatalf("case %q: Constraint() error = %v", item.ID, err)
			}
			if constraint.Mode == memory.TemporalSelectionCurrent {
				t.Fatalf("case %q: historical kind restates the current mode", item.ID)
			}
			// A historical fixture must not carry a competing evaluation
			// instant; the selector is the only clock.
			if temporal.EvaluationInstant != nil {
				t.Fatalf("case %q: historical kind declares an evaluation instant", item.ID)
			}
			continue
		}
		if temporal.Selector != nil {
			t.Fatalf("case %q: kind %q must not declare a selector", item.ID, temporal.Kind)
		}
		if temporal.EvaluationInstant == nil {
			t.Fatalf("case %q: kind %q requires an evaluation instant", item.ID, temporal.Kind)
		}
	}
}

// TestTemporalEvaluationFixtureHidesEveryUnsafeVersionWithoutContradiction
// proves each unsafe version is named as hidden and that no hidden alias is also
// declared expected evidence, which would make the case unsatisfiable.
func TestTemporalEvaluationFixtureHidesEveryUnsafeVersionWithoutContradiction(t *testing.T) {
	fixture := loadTemporalFixture(t)
	unsafeKinds := map[EvaluationTemporalCaseKind]struct{}{
		EvaluationTemporalCaseExpired:               {},
		EvaluationTemporalCaseAsOf:                  {},
		EvaluationTemporalCaseInterval:              {},
		EvaluationTemporalCaseRetroactiveCorrection: {},
		EvaluationTemporalCaseStaleSimilarity:       {},
		EvaluationTemporalCaseTemporalIsolation:     {},
	}
	for _, item := range fixture.Cases {
		temporal := item.Temporal
		if temporal == nil {
			continue
		}
		if _, mustHide := unsafeKinds[temporal.Kind]; mustHide && len(temporal.HiddenAliases) == 0 {
			t.Fatalf("case %q: kind %q declares no hidden version", item.ID, temporal.Kind)
		}
		expected := map[string]struct{}{}
		for _, group := range item.ExpectedEvidenceGroups {
			for _, alias := range group {
				expected[alias] = struct{}{}
			}
		}
		declared := map[string]struct{}{}
		for _, source := range item.Sources {
			declared[source.Alias] = struct{}{}
		}
		for _, alias := range temporal.HiddenAliases {
			if _, isExpected := expected[alias]; isExpected {
				t.Fatalf("case %q: hidden alias %q is also expected evidence", item.ID, alias)
			}
			if _, isDeclared := declared[alias]; !isDeclared {
				t.Fatalf("case %q: hidden alias %q is not a declared source", item.ID, alias)
			}
		}
	}
}

// TestTemporalEvaluationFixtureRejectsUnsafeOrAmbiguousCases proves validation
// actively bites: each mutation below turns a previously valid fixture into one
// that must be refused before seeding.
func TestTemporalEvaluationFixtureRejectsUnsafeOrAmbiguousCases(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*EvaluationFixture)
	}{
		{
			name: "historical kind without a selector",
			mutate: func(f *EvaluationFixture) {
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil && f.Cases[i].Temporal.Kind == EvaluationTemporalCaseAsOf {
						f.Cases[i].Temporal.Selector = nil
					}
				}
			},
		},
		{
			name: "current kind carrying a selector",
			mutate: func(f *EvaluationFixture) {
				asOf := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil && f.Cases[i].Temporal.Kind == EvaluationTemporalCaseCurrentValid {
						f.Cases[i].Temporal.Selector = &TemporalCaseSelector{Mode: memory.TemporalSelectionAsOf, AsOf: &asOf}
					}
				}
			},
		},
		{
			name: "selector restates the current mode",
			mutate: func(f *EvaluationFixture) {
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil && f.Cases[i].Temporal.Kind == EvaluationTemporalCaseAsOf {
						f.Cases[i].Temporal.Selector = &TemporalCaseSelector{Mode: memory.TemporalSelectionCurrent}
					}
				}
			},
		},
		{
			name: "interval selector is inverted",
			mutate: func(f *EvaluationFixture) {
				from := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
				to := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil && f.Cases[i].Temporal.Kind == EvaluationTemporalCaseInterval {
						f.Cases[i].Temporal.Selector = &TemporalCaseSelector{Mode: memory.TemporalSelectionDuring, ValidFrom: &from, ValidTo: &to}
					}
				}
			},
		},
		{
			name: "as_of selector carries an extra interval bound",
			mutate: func(f *EvaluationFixture) {
				asOf := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
				from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil && f.Cases[i].Temporal.Kind == EvaluationTemporalCaseAsOf {
						f.Cases[i].Temporal.Selector = &TemporalCaseSelector{Mode: memory.TemporalSelectionAsOf, AsOf: &asOf, ValidFrom: &from}
					}
				}
			},
		},
		{
			name: "retroactive correction is not authorized",
			mutate: func(f *EvaluationFixture) {
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil && f.Cases[i].Temporal.Kind == EvaluationTemporalCaseRetroactiveCorrection {
						f.Cases[i].Temporal.AuthorizedCorrection = false
					}
				}
			},
		},
		{
			name: "authorization claimed without a correction scenario",
			mutate: func(f *EvaluationFixture) {
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil && f.Cases[i].Temporal.Kind == EvaluationTemporalCaseCurrentValid {
						f.Cases[i].Temporal.AuthorizedCorrection = true
					}
				}
			},
		},
		{
			name: "unselected evidence tolerated outside an isolation case",
			mutate: func(f *EvaluationFixture) {
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil && f.Cases[i].Temporal.Kind == EvaluationTemporalCaseAsOf {
						f.Cases[i].Temporal.AllowUnselectedExpected = true
					}
				}
			},
		},
		{
			name: "hidden alias is also expected evidence",
			mutate: func(f *EvaluationFixture) {
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil && len(f.Cases[i].Temporal.HiddenAliases) > 0 {
						f.Cases[i].ExpectedEvidenceGroups = append(f.Cases[i].ExpectedEvidenceGroups, []string{f.Cases[i].Temporal.HiddenAliases[0]})
					}
				}
			},
		},
		{
			name: "hidden alias is unknown",
			mutate: func(f *EvaluationFixture) {
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil && len(f.Cases[i].Temporal.HiddenAliases) > 0 {
						f.Cases[i].Temporal.HiddenAliases = []string{"never-declared-alias"}
					}
				}
			},
		},
		{
			name: "selected-version expectation exceeds declared evidence",
			mutate: func(f *EvaluationFixture) {
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil {
						f.Cases[i].Temporal.MinimumSelectedVersions = evaluationFixtureMaxSourcesPerCase + 1
					}
				}
			},
		},
		{
			name: "selected-version expectation is zero",
			mutate: func(f *EvaluationFixture) {
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil {
						f.Cases[i].Temporal.MinimumSelectedVersions = 0
					}
				}
			},
		},
		{
			name: "evaluation instant is zero for a current kind",
			mutate: func(f *EvaluationFixture) {
				zero := time.Time{}
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil && f.Cases[i].Temporal.Kind == EvaluationTemporalCaseCurrentValid {
						f.Cases[i].Temporal.EvaluationInstant = &zero
					}
				}
			},
		},
		{
			name: "temporal kind is unknown",
			mutate: func(f *EvaluationFixture) {
				for i := range f.Cases {
					if f.Cases[i].Temporal != nil && f.Cases[i].Temporal.Kind == EvaluationTemporalCaseExpired {
						f.Cases[i].Temporal.Kind = EvaluationTemporalCaseKind("made_up_kind")
					}
				}
			},
		},
		{
			name: "a required kind is missing",
			mutate: func(f *EvaluationFixture) {
				cases := make([]EvaluationCase, 0, len(f.Cases))
				for _, item := range f.Cases {
					if item.Temporal != nil && item.Temporal.Kind == EvaluationTemporalCaseLegacyCompatible {
						continue
					}
					cases = append(cases, item)
				}
				f.Cases = cases
			},
		},
		{
			name: "a kind is duplicated",
			mutate: func(f *EvaluationFixture) {
				duplicate := f.Cases[0]
				duplicate.ID = "temporal-duplicate-kind"
				f.Cases = append(f.Cases, duplicate)
			},
		},
	}

	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			fixture := loadTemporalFixture(t)
			mutation.mutate(&fixture)
			if err := fixture.Validate(); err == nil {
				t.Fatalf("Validate() error = nil, want the mutated fixture to be refused")
			}
		})
	}
}

// TestTemporalEvaluationFixtureKeepsItsBaselineValid guards the control for the
// mutation table above: the unmutated fixture must validate, otherwise every
// "refused" assertion would pass for the wrong reason.
func TestTemporalEvaluationFixtureKeepsItsBaselineValid(t *testing.T) {
	base := loadTemporalFixture(t)
	if err := base.Validate(); err != nil {
		t.Fatalf("baseline Validate() error = %v", err)
	}
	raw, err := os.ReadFile(temporalFixturePath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	// The fixture must not smuggle raw content into a temporal expectation.
	lower := strings.ToLower(string(raw))
	if strings.Contains(lower, "temporal_fact_id") || strings.Contains(lower, "memory_id") {
		t.Fatalf("fixture contains a database identifier field")
	}
}

// TestPreTemporalFixtureStillValidates proves the temporal extension is purely
// additive: a fixture that never mentions temporal expectations keeps its exact
// previous meaning and is not forced to cover the temporal kinds.
func TestPreTemporalFixtureStillValidates(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "retrieval-evaluation-fixture-v1.json"))
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	var fixture EvaluationFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if err := fixture.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want the pre-temporal fixture to stay valid", err)
	}
	for _, item := range fixture.Cases {
		if item.Temporal != nil {
			t.Fatalf("case %q unexpectedly declares a temporal expectation", item.ID)
		}
	}
}
