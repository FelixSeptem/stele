package provider

import (
	"encoding/json"
	"fmt"
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"strings"
	"testing"
)

func budgetFixture(t *testing.T, input ContextInput, mode string) (*ContextAdmission, retrieval.SearchHit, OperationMetadata) {
	t.Helper()
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	meta := OperationMetadata{RequestID: "r", OperationID: "o", SchemaVersion: "provider-v1"}
	decision, err := (auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileContent}, DefaultProfile: auth.DisclosureProfileContent}).Resolve("", mode)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewContextAdmission(input, Discover(CapabilityInput{}).Limits, scope, decision, meta)
	if err != nil {
		t.Fatal(err)
	}
	hit := retrieval.SearchHit{Memory: memory.CanonicalMemory{ID: "memory", Scope: scope, MemoryPath: strings.Repeat("path/", 90) + "leaf", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, SourceVersion: 1, SourceMetadata: memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}, Content: "中文🙂\"\\\n"}}
	return a, hit, meta
}
func TestContextBudgetExactUTF8RepeatedPlacementsAndCitations(t *testing.T) {
	for _, mode := range []string{auth.ResponseModeReferenceOnly, auth.ResponseModeBoundedContent} {
		a, hit, meta := budgetFixture(t, ContextInput{Budget: 2}, mode)
		if !a.AdmitContextItem("profile", hit) || !a.AdmitContextItem("recent_session", hit) {
			t.Fatal("repeated placements did not fit")
		}
		b, err := a.Finalize(retrieval.AssembledContext{Profile: []retrieval.SearchHit{hit}, RecentSession: []retrieval.SearchHit{hit}}, meta)
		if err != nil {
			t.Fatal(err)
		}
		var r DisclosedContextResponse
		if err = json.Unmarshal(b, &r); err != nil {
			t.Fatal(err)
		}
		u := r.Result.BudgetAccounting.Usage
		content := 0
		if mode == auth.ResponseModeBoundedContent {
			content = 2 * len(hit.Memory.Content)
		}
		if u.Items != 2 || u.Citations != 1 || u.ContentBytes != content || u.ResponseBytes != len(b) || b[len(b)-1] == '\n' {
			t.Fatalf("incorrect usage %+v body %d", u, len(b))
		}
	}
}
func TestContextBudgetZeroAndUnsafeReasons(t *testing.T) {
	zero := 0
	for _, mode := range []string{auth.ResponseModeReferenceOnly, auth.ResponseModeBoundedContent} {
		for _, limit := range []ContextBudgetLimits{{MaxContentBytes: &zero}, {MaxCitations: &zero}} {
			a, hit, meta := budgetFixture(t, ContextInput{Budget: 1, BudgetLimits: &limit, IncludeDiagnostics: true, IncludeGoalContext: true}, mode)
			unsafe := hit
			unsafe.Memory.SourceMetadata.Classification = memory.SourceClassificationHigh
			if a.AdmitContextItem("recent_session", unsafe) {
				t.Fatal("high item admitted")
			}
			b, err := a.Finalize(retrieval.AssembledContext{}, meta)
			if err != nil {
				t.Fatal(err)
			}
			var r DisclosedContextResponse
			_ = json.Unmarshal(b, &r)
			if r.Result.BudgetAccounting.Selection.Outcome != "empty" || len(r.Result.BudgetAccounting.Selection.OmissionReasons) != 0 {
				t.Fatal("unsafe existence leaked")
			}
			// Create a separate ledger; finalize is deliberately a single-use boundary.
			a, hit, meta = budgetFixture(t, ContextInput{Budget: 1, BudgetLimits: &limit}, mode)
			admitted := a.AdmitContextItem("recent_session", hit)
			shouldFit := mode == auth.ResponseModeReferenceOnly && limit.MaxCitations == nil
			if admitted != shouldFit {
				t.Fatal("zero allocation changed mode or stripped evidence")
			}
			out := retrieval.AssembledContext{}
			if admitted {
				out.RecentSession = []retrieval.SearchHit{hit}
			}
			b, err = a.Finalize(out, meta)
			if err != nil {
				t.Fatal(err)
			}
			_ = json.Unmarshal(b, &r)
			want := "budget_exhausted"
			if admitted {
				want = "selected"
			}
			if r.Result.BudgetAccounting.Selection.Outcome != want {
				t.Fatal("outcome")
			}
		}
	}
	a, hit, meta := budgetFixture(t, ContextInput{Budget: 1, BudgetLimits: &ContextBudgetLimits{MaxContentBytes: &zero}}, auth.ResponseModeBoundedContent)
	hit.Memory.Content = ""
	if !a.AdmitContextItem("profile", hit) {
		t.Fatal("empty content must fit zero")
	}
	if _, err := a.Finalize(retrieval.AssembledContext{Profile: []retrieval.SearchHit{hit}}, meta); err != nil {
		t.Fatal(err)
	}
}
func TestContextBudgetReserveBoundaryAndMismatch(t *testing.T) {
	a, hit, meta := budgetFixture(t, ContextInput{Budget: 1}, auth.ResponseModeBoundedContent)
	one := retrieval.AssembledContext{Profile: []retrieval.SearchHit{hit}}
	cost := a.reservedSize(a.response(one))
	// Requested/effective ceilings themselves are part of the serialized envelope.
	for n := 0; n < 4; n++ {
		a, _, _ = budgetFixture(t, ContextInput{Budget: 1, BudgetLimits: &ContextBudgetLimits{MaxResponseBytes: &cost}}, auth.ResponseModeBoundedContent)
		cost = a.reservedSize(a.response(one))
	}
	for _, delta := range []int{0, -1} {
		limit := cost + delta
		a, hit, meta = budgetFixture(t, ContextInput{Budget: 1, BudgetLimits: &ContextBudgetLimits{MaxResponseBytes: &limit}}, auth.ResponseModeBoundedContent)
		fit := a.AdmitContextItem("profile", hit)
		if fit != (delta == 0) {
			t.Fatalf("reserve boundary %d", delta)
		}
		out := retrieval.AssembledContext{}
		if fit {
			out.Profile = []retrieval.SearchHit{hit}
		}
		b, err := a.Finalize(out, meta)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) > limit {
			t.Fatal("body exceeds cap")
		}
	}
	a, hit, meta = budgetFixture(t, ContextInput{Budget: 1}, auth.ResponseModeBoundedContent)
	if _, err := a.Finalize(one, meta); err == nil {
		t.Fatal("assembler bypass accepted")
	}
	if !a.AdmitContextItem("profile", hit) {
		t.Fatal("fit")
	}
	meta.RequestID = strings.Repeat("x", 3000)
	a.accounting.Effective.MaxResponseBytes = cost
	if _, err := a.Finalize(one, meta); err == nil {
		t.Fatal("changed envelope overflow accepted")
	}
}

func TestContextBudgetPlannerCapIsNotPublicItemLimit(t *testing.T) {
	for _, budget := range []int{1, 10} {
		a, hit, meta := budgetFixture(t, ContextInput{Budget: budget}, auth.ResponseModeBoundedContent)
		if !a.AdmitContextItem("profile", hit) {
			t.Fatal("first item did not fit")
		}
		// The assembler has exhausted its internal plan after one placement.
		a.NoteContextItemLimit(hit)
		data, err := a.Finalize(retrieval.AssembledContext{Profile: []retrieval.SearchHit{hit}}, meta)
		if err != nil {
			t.Fatal(err)
		}
		var response DisclosedContextResponse
		_ = json.Unmarshal(data, &response)
		reasons := response.Result.BudgetAccounting.Selection.OmissionReasons
		if (len(reasons) == 1) != (budget == 1) || len(reasons) == 1 && reasons[0] != "item_limit" {
			t.Fatalf("budget=%d public reasons=%v", budget, reasons)
		}
	}
}

func TestContextBudgetSelfCountingDecimalTransitions(t *testing.T) {
	widths := map[int]bool{}
	for _, size := range []int{0, 9, 10, 99, 100, 999, 1000, 9000, 9999, 10000, 99999, 100000} {
		a, hit, meta := budgetFixture(t, ContextInput{Budget: 1}, auth.ResponseModeBoundedContent)
		hit.Memory.Content = strings.Repeat("x", size)
		if !a.AdmitContextItem("profile", hit) {
			t.Fatalf("fixture size %d did not fit", size)
		}
		data, err := a.Finalize(retrieval.AssembledContext{Profile: []retrieval.SearchHit{hit}}, meta)
		if err != nil {
			t.Fatal(err)
		}
		var r DisclosedContextResponse
		if json.Unmarshal(data, &r) != nil || r.Result.BudgetAccounting.Usage.ResponseBytes != len(data) || r.Result.BudgetAccounting.Usage.ContentBytes != size {
			t.Fatalf("incorrect decimal accounting for content size %d", size)
		}
		widths[len(fmt.Sprint(len(data)))] = true
	}
	if len(widths) < 3 {
		t.Fatal("fixture failed to cross whole-body decimal widths")
	}
}
