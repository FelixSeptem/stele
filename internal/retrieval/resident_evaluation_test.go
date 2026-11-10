package retrieval

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func residentFixture(t *testing.T) ResidentEvaluationInput {
	t.Helper()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	scope := memory.Scope{Tenant: "private-tenant", Project: "benchmark-resident", Namespace: "private-namespace"}
	metadata := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustAttestedData, Classification: memory.SourceClassificationLow}
	source := ResidentSource{Memory: memory.CanonicalMemory{ID: "private-memory", Scope: scope, Class: memory.MemoryClassProfile, State: memory.MemoryStateActive, Content: "Use 中文 and retain exact value 47.", SourceVersion: 1, SourceMetadata: metadata, CreatedAt: now.Add(-time.Hour), ModifiedAt: now.Add(-time.Hour)}, Confidence: 1}
	source.Version = memory.MemoryVersion{ID: "private-version", MemoryID: source.Memory.ID, Version: 1, Content: source.Memory.Content, State: source.Memory.State, SourceMetadata: metadata, CreatedAt: now.Add(-time.Hour)}
	support := ResidentSupportObservation{MemoryID: source.Memory.ID, SourceVersion: 1, ClaimFingerprint: ResidentClaimFingerprint(source.Memory.Content), Actor: "private-operator", Stable: true, Applicable: true, Stage: 1, ExpiresAt: now.Add(time.Hour)}
	utility := ResidentUtilityObservation{MemoryID: source.Memory.ID, SourceVersion: 1, ClaimFingerprint: support.ClaimFingerprint, Actor: "private-judge", Stage: 2, ExpiresAt: now.Add(time.Hour), Useful: true}
	return ResidentEvaluationInput{Scope: scope, EvaluatedAt: now, Policy: memory.DefaultContextProjectionPolicy("resident-fixture-v1"), Limits: DefaultResidentEvaluationLimits(), Sources: []ResidentSource{source}, CurrentResidentIDs: []string{source.Memory.ID}, Support: []memory.RawEvent{residentEvent(t, scope, metadata, "private-support", "user_correction", support, now.Add(-30*time.Minute))}, Utility: []memory.RawEvent{residentEvent(t, scope, metadata, "private-utility", "external_evaluation", utility, now.Add(-20*time.Minute))}}
}

func residentEvent(t *testing.T, scope memory.Scope, metadata memory.SourceMetadata, id, kind string, body any, at time.Time) memory.RawEvent {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return memory.RawEvent{ID: id, Scope: scope, SourceMetadata: metadata, EventType: kind, Content: string(data), SourceTimestamp: at, CreatedAt: at}
}

func changeResidentSupport(t *testing.T, in *ResidentEvaluationInput, change func(*ResidentSupportObservation)) {
	t.Helper()
	var body ResidentSupportObservation
	if err := json.Unmarshal([]byte(in.Support[0].Content), &body); err != nil {
		t.Fatal(err)
	}
	change(&body)
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	in.Support[0].Content = string(data)
}

func TestResidentEvidenceAndDemotion(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*ResidentEvaluationInput)
		reason ResidentOmissionReason
	}{
		{"missing support", func(in *ResidentEvaluationInput) { in.Support = nil }, ResidentOmissionSupport},
		{"model assertion", func(in *ResidentEvaluationInput) { in.Support[0].EventType = "model_assertion" }, ResidentOmissionSupport},
		{"transcript", func(in *ResidentEvaluationInput) { in.Support[0].EventType = "transcript" }, ResidentOmissionSupport},
		{"untrusted support", func(in *ResidentEvaluationInput) { in.Support[0].SourceMetadata.Trust = memory.SourceTrustUntrusted }, ResidentOmissionSupport},
		{"untrusted utility", func(in *ResidentEvaluationInput) { in.Utility[0].SourceMetadata.Trust = memory.SourceTrustUntrusted }, ResidentOmissionUtility},
		{"claim mismatch", func(in *ResidentEvaluationInput) {
			changeResidentSupport(t, in, func(s *ResidentSupportObservation) { s.ClaimFingerprint = strings.Repeat("f", 64) })
		}, ResidentOmissionSupport},
		{"version mismatch", func(in *ResidentEvaluationInput) {
			changeResidentSupport(t, in, func(s *ResidentSupportObservation) { s.SourceVersion++ })
		}, ResidentOmissionSupport},
		{"unattributed", func(in *ResidentEvaluationInput) {
			changeResidentSupport(t, in, func(s *ResidentSupportObservation) { s.Actor = "" })
		}, ResidentOmissionSupport},
		{"unstable", func(in *ResidentEvaluationInput) {
			changeResidentSupport(t, in, func(s *ResidentSupportObservation) { s.Stable = false })
		}, ResidentOmissionSupport},
		{"inapplicable", func(in *ResidentEvaluationInput) {
			changeResidentSupport(t, in, func(s *ResidentSupportObservation) { s.Applicable = false })
		}, ResidentOmissionSupport},
		{"expired support", func(in *ResidentEvaluationInput) {
			changeResidentSupport(t, in, func(s *ResidentSupportObservation) { s.ExpiresAt = in.EvaluatedAt })
		}, ResidentOmissionSupport},
		{"future support", func(in *ResidentEvaluationInput) { in.Support[0].SourceTimestamp = in.EvaluatedAt.Add(time.Second) }, ResidentOmissionSupport},
		{"missing utility", func(in *ResidentEvaluationInput) { in.Utility = nil }, ResidentOmissionUtility},
		{"future utility", func(in *ResidentEvaluationInput) { in.Utility[0].SourceTimestamp = in.EvaluatedAt.Add(time.Second) }, ResidentOmissionUtility},
		{"foreign support", func(in *ResidentEvaluationInput) { in.Support[0].Scope.Tenant += "foreign" }, ResidentOmissionSupport},
		{"restricted support", func(in *ResidentEvaluationInput) {
			in.Support[0].SourceMetadata.Classification = memory.SourceClassificationRestricted
		}, ResidentOmissionSupport},
		{"hidden source", func(in *ResidentEvaluationInput) { in.Sources[0].Memory.State = memory.MemoryStateForgotten }, ResidentOmissionSafety},
		{"hidden immutable version", func(in *ResidentEvaluationInput) { in.Sources[0].Version.State = memory.MemoryStateSuppressed }, ResidentOmissionSafety},
		{"future source validity", func(in *ResidentEvaluationInput) {
			in.Sources[0].Memory.ValidFrom = in.EvaluatedAt.Add(time.Second)
		}, ResidentOmissionSafety},
		{"expired immutable validity", func(in *ResidentEvaluationInput) {
			end := in.EvaluatedAt
			in.Sources[0].Version.ValidTo = &end
		}, ResidentOmissionSafety},
		{"foreign source", func(in *ResidentEvaluationInput) { in.Sources[0].Memory.Scope.Project += "foreign" }, ResidentOmissionSafety},
		{"restricted source", func(in *ResidentEvaluationInput) {
			in.Sources[0].Memory.SourceMetadata.Classification = memory.SourceClassificationHigh
		}, ResidentOmissionSafety},
		{"restricted immutable version", func(in *ResidentEvaluationInput) {
			in.Sources[0].Version.SourceMetadata.Classification = memory.SourceClassificationRestricted
		}, ResidentOmissionSafety},
		{"source drift", func(in *ResidentEvaluationInput) { in.Sources[0].Memory.SourceVersion++ }, ResidentOmissionSafety},
		{"version text drift", func(in *ResidentEvaluationInput) { in.Sources[0].Version.Content += " drift" }, ResidentOmissionSafety},
		{"nonresident class", func(in *ResidentEvaluationInput) { in.Sources[0].Memory.Class = memory.MemoryClassProcedural }, ResidentOmissionPolicy},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := residentFixture(t)
			tt.change(&in)
			result, err := EvaluateResidentContext(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.CandidateProjection().Items) != 0 || result.Report.Omissions[tt.reason] != 1 || result.Report.Demoted != 1 {
				t.Fatalf("unsafe promotion or missing demotion: %+v", result.Report)
			}
		})
	}
}

func TestResidentEligibleIsIndependentOfPopularityAndDeterministic(t *testing.T) {
	in := residentFixture(t)
	before, _ := json.Marshal(in)
	first, err := EvaluateResidentContext(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Report.Retained != 1 || first.Report.Candidate.Items != 1 || first.Report.Candidate.Citations != 1 {
		t.Fatalf("eligible source omitted: %+v", first.Report)
	}
	in.Sources[0].AccessCount = 1_000_000
	second, err := EvaluateResidentContext(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("popularity or wall clock affected result")
	}
	in.Sources[0].AccessCount = 0
	after, _ := json.Marshal(in)
	if string(before) != string(after) {
		t.Fatal("evaluator mutated input")
	}
	if !reflect.DeepEqual(first.SelectedProjection(true), first.BaselineProjection()) {
		t.Fatal("disabled evaluator did not select baseline")
	}
}

func TestResidentIndependentBudgetsAndUnicode(t *testing.T) {
	for _, kind := range []string{"items", "bytes", "tokens", "citations"} {
		t.Run(kind, func(t *testing.T) {
			in := residentFixture(t)
			// Each condition uses exactly the same independent limits.
			switch kind {
			case "bytes":
				in.Limits.MaxContentBytes = len(in.Sources[0].Memory.Content) - 1
			case "tokens":
				in.Limits.MaxTokens = 1
			case "items":
				in.Limits.MaxItems = 1
			case "citations":
				in.Limits.MaxCitations = 1
			}
			if kind == "items" || kind == "citations" {
				s := in.Sources[0]
				s.Memory.ID += "2"
				s.Version.ID += "2"
				s.Version.MemoryID = s.Memory.ID
				in.Sources = append(in.Sources, s)
				for _, events := range []*[]memory.RawEvent{&in.Support, &in.Utility} {
					e := (*events)[0]
					e.ID += "2"
					e.Content = strings.ReplaceAll(e.Content, "private-memory", "private-memory2")
					*events = append(*events, e)
				}
			}
			r, err := EvaluateResidentContext(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if kind == "items" || kind == "citations" {
				want = 1
			}
			if r.Report.Baseline.Items != want || r.Report.Candidate.Items != want || r.Report.Omissions[ResidentOmissionBudget] != 1 {
				t.Fatalf("budget %s ignored: %+v", kind, r.Report)
			}
		})
	}
}

func TestResidentUtilityRequiresLaterIndependentCurrentObservation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*ResidentUtilityObservation)
	}{
		{"same stage", func(u *ResidentUtilityObservation) { u.Stage = 1 }},
		{"wrong version", func(u *ResidentUtilityObservation) { u.SourceVersion++ }},
		{"not useful", func(u *ResidentUtilityObservation) { u.Useful = false }},
		{"unattributed", func(u *ResidentUtilityObservation) { u.Actor = "" }},
		{"expired", func(u *ResidentUtilityObservation) { u.ExpiresAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := residentFixture(t)
			var u ResidentUtilityObservation
			json.Unmarshal([]byte(in.Utility[0].Content), &u)
			tt.change(&u)
			data, _ := json.Marshal(u)
			in.Utility[0].Content = string(data)
			r, err := EvaluateResidentContext(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			if r.Report.Candidate.Items != 0 || r.Report.Omissions[ResidentOmissionUtility] != 1 {
				t.Fatal("invalid utility promoted source")
			}
		})
	}
}

func TestResidentReportActualDigestsRedactionAndTampering(t *testing.T) {
	in := residentFixture(t)
	r, err := EvaluateResidentContext(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Report.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-", in.Sources[0].Memory.Content, in.Support[0].Content, in.Policy.Version} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("report disclosed private input: %s", secret)
		}
	}
	changed := residentFixture(t)
	changed.Sources[0].Memory.SourceVersion++
	changed.Sources[0].Version.Version++
	r2, err := EvaluateResidentContext(context.Background(), changed)
	if err != nil {
		t.Fatal(err)
	}
	if r.Report.SourceDigest == r2.Report.SourceDigest || r.Report.OutputDigest == r2.Report.OutputDigest {
		t.Fatal("actual version/output drift was not bound")
	}
	r.Report.Candidate.ContentBytes++
	if r.Report.Validate() == nil {
		t.Fatal("aggregate tampering accepted")
	}
}

func TestResidentExternalComparisonDoesNotInferMissingOutcomes(t *testing.T) {
	in := residentFixture(t)
	r, err := EvaluateResidentContext(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if r.Report.OutcomeStatus != "inconclusive" || r.Report.BaselineOutcome.TaskSucceeded != nil || r.Report.Delta.TaskSucceeded != nil {
		t.Fatal("missing task inferred")
	}
	at := in.EvaluatedAt.Add(time.Minute)
	yes, no := true, false
	zero, one := 0, 1
	cost := int64(17)
	observation := ResidentConditionObservation{Condition: "baseline", SourceDigest: r.Report.SourceDigest, OutputDigest: r.Report.OutputDigest, Actor: "private-judge", Stage: 3, TaskSucceeded: &no, RepeatedErrors: &one, StaleEvidenceHarm: &one, MaintenanceCost: &cost}
	a := residentEvent(t, in.Scope, in.Sources[0].Memory.SourceMetadata, "private-baseline", "external_evaluation", observation, at)
	observation.Condition = "candidate"
	observation.TaskSucceeded = &yes
	observation.RepeatedErrors = &zero
	observation.StaleEvidenceHarm = &zero
	b := residentEvent(t, in.Scope, in.Sources[0].Memory.SourceMetadata, "private-candidate", "external_evaluation", observation, at)
	report, err := CompareResidentOutcomes(r, []memory.RawEvent{a, b}, at)
	if err != nil {
		t.Fatal(err)
	}
	if report.OutcomeStatus != "observational" || report.Delta.TaskSucceeded == nil || *report.Delta.TaskSucceeded != 1 || *report.Delta.RepeatedErrors != -1 || *report.Delta.StaleEvidenceHarm != -1 || *report.Delta.MaintenanceCost != 0 {
		t.Fatalf("observed deltas lost: %+v", report)
	}
	if err := report.Validate(); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(report)
	if strings.Contains(string(data), "private-") {
		t.Fatal("outcome report leaked source attribution")
	}
	partial, err := CompareResidentOutcomes(r, []memory.RawEvent{b}, at)
	if err != nil {
		t.Fatal(err)
	}
	if partial.Delta.TaskSucceeded != nil || partial.OutcomeStatus != "inconclusive" {
		t.Fatal("missing baseline inferred")
	}
	for _, alter := range []func(*memory.RawEvent){
		func(e *memory.RawEvent) { e.SourceMetadata.Trust = memory.SourceTrustUntrusted },
		func(e *memory.RawEvent) { e.Content = strings.TrimSuffix(e.Content, "}") + `,"Condition":"candidate"}` },
		func(e *memory.RawEvent) { e.Scope.Namespace += "foreign" },
		func(e *memory.RawEvent) { e.EventType = "model_assertion" },
		func(e *memory.RawEvent) { e.SourceTimestamp = in.EvaluatedAt.Add(-time.Second) },
		func(e *memory.RawEvent) { e.Content = strings.Replace(e.Content, `"stage":3`, `"stage":2`, 1) },
		func(e *memory.RawEvent) {
			e.Content = strings.Replace(e.Content, r.Report.OutputDigest, strings.Repeat("0", 64), 1)
		},
	} {
		changed := b
		alter(&changed)
		if _, err := CompareResidentOutcomes(r, []memory.RawEvent{changed}, at); err == nil {
			t.Fatal("invalid external comparison accepted")
		}
	}
}

func TestResidentMalformedAndConflictingEvidenceFailsClosed(t *testing.T) {
	for _, change := range []func(*ResidentEvaluationInput){
		func(in *ResidentEvaluationInput) {
			in.Support[0].Content = strings.Replace(in.Support[0].Content, `"stable":true`, `"stable":false,"Stable":true`, 1)
		},
		func(in *ResidentEvaluationInput) {
			in.Utility[0].Content = strings.Replace(in.Utility[0].Content, `"useful":true`, `"useful":false,"Useful":true`, 1)
		},
		func(in *ResidentEvaluationInput) {
			in.Support[0].Content = strings.Replace(in.Support[0].Content, `"actor":`, `"Actor":`, 1)
		},
		func(in *ResidentEvaluationInput) {
			in.Support[0].Content = strings.TrimSuffix(in.Support[0].Content, "}") + `,"stable":true}`
		},
		func(in *ResidentEvaluationInput) {
			e := in.Support[0]
			e.ID += "duplicate"
			in.Support = append(in.Support, e)
		},
		func(in *ResidentEvaluationInput) { in.Utility[0].SourceTimestamp = in.Support[0].SourceTimestamp },
	} {
		in := residentFixture(t)
		change(&in)
		r, err := EvaluateResidentContext(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		if r.Report.Candidate.Items != 0 {
			t.Fatal("ambiguous evidence promoted source")
		}
	}
	for _, change := range []func(*ResidentEvaluationInput){
		func(in *ResidentEvaluationInput) { in.Limits.MaxItems = 0 },
		func(in *ResidentEvaluationInput) { in.Sources = append(in.Sources, in.Sources[0]) },
		func(in *ResidentEvaluationInput) { in.Support[0].Content = strings.Repeat("x", 16385) },
	} {
		in := residentFixture(t)
		change(&in)
		if _, err := EvaluateResidentContext(context.Background(), in); err == nil {
			t.Fatal("unbounded/ambiguous input accepted")
		}
	}
}

func TestResidentPlansPreserveIndependentTemporalSnapshots(t *testing.T) {
	in := residentFixture(t)
	end := in.EvaluatedAt.Add(time.Hour)
	validity := memory.TemporalValidity{TemporalFactID: "private-fact", IngestedAt: in.Sources[0].Memory.ModifiedAt, ValidFrom: in.Sources[0].Memory.ModifiedAt, ValidTo: &end, ValiditySource: memory.TemporalValiditySourceExplicit}
	in.Sources[0].Memory.TemporalValidity = validity
	in.Sources[0].Version.TemporalValidity = validity
	r, err := EvaluateResidentContext(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range []memory.ContextProjection{r.BaselineProjection(), r.CandidateProjection()} {
		if len(plan.Items) != 1 || !reflect.DeepEqual(plan.Items[0].TemporalValidity, validity) {
			t.Fatal("plan lost immutable fact validity")
		}
		*plan.Items[0].ValidTo = end.Add(time.Hour)
	}
	if !r.BaselineProjection().Items[0].ValidTo.Equal(end) || !r.CandidateProjection().Items[0].ValidTo.Equal(end) || !in.Sources[0].Version.ValidTo.Equal(end) {
		t.Fatal("returned plan or input aliases retained temporal pointers")
	}
}

func TestResidentRejectedObservationsDoNotAdvanceComparisonStage(t *testing.T) {
	for _, kind := range []string{"support", "utility"} {
		for _, rejected := range []string{"unrelated", "foreign", "expired", "oversized", "untrusted"} {
			t.Run(kind+"/"+rejected, func(t *testing.T) {
				in := residentFixture(t)
				e := in.Support[0]
				if kind == "utility" {
					e = in.Utility[0]
				}
				// Distinct target avoids conflicting evidence for a valid source.
				e.ID += "-rejected"
				e.Content = strings.Replace(e.Content, `"private-memory"`, `"unrelated-memory"`, 1)
				e.Content = strings.Replace(e.Content, `"stage":1`, `"stage":100`, 1)
				e.Content = strings.Replace(e.Content, `"stage":2`, `"stage":100`, 1)
				if rejected != "unrelated" {
					s := in.Sources[0]
					s.Memory.ID = "unrelated-memory"
					s.Version.MemoryID = s.Memory.ID
					s.Version.ID += "-second"
					in.Sources = append(in.Sources, s)
					if kind == "utility" {
						prior := in.Support[0]
						prior.ID += "-second"
						prior.Content = strings.Replace(prior.Content, `"private-memory"`, `"unrelated-memory"`, 1)
						in.Support = append(in.Support, prior)
					}
				}
				switch rejected {
				case "foreign":
					e.Scope.Tenant += "foreign"
				case "expired":
					e.SourceTimestamp = in.EvaluatedAt.Add(-31 * 24 * time.Hour)
				case "oversized":
					e.Content = strings.Replace(e.Content, `"stage":100`, `"stage":1000001`, 1)
				case "untrusted":
					e.SourceMetadata.Trust = memory.SourceTrustUntrusted
				}
				if kind == "support" {
					in.Support = append(in.Support, e)
				} else {
					in.Utility = append(in.Utility, e)
				}
				r, err := EvaluateResidentContext(context.Background(), in)
				if err != nil {
					t.Fatal(err)
				}
				at := in.EvaluatedAt.Add(time.Minute)
				outcome := ResidentConditionObservation{Condition: "candidate", SourceDigest: r.Report.SourceDigest, OutputDigest: r.Report.OutputDigest, Actor: "private-judge", Stage: 3}
				observation := residentEvent(t, in.Scope, in.Sources[0].Memory.SourceMetadata, "later-outcome", "external_evaluation", outcome, at)
				if _, err := CompareResidentOutcomes(r, []memory.RawEvent{observation}, at); err != nil {
					t.Fatalf("rejected observation poisoned later comparison: %v", err)
				}
			})
		}
	}
}
