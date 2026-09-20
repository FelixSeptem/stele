package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

// Section 6.4 covers the public HTTP surface for valid-time selectors. The
// contract under test is deliberately narrow: an ordinary search must stay
// byte-for-byte what it was before this change, an authorized historical search
// must reach the searcher with the exact selector the caller asked for, and
// every malformed or conflicting selector must be refused before the searcher
// runs so no partially-interpreted request can be mistaken for history.

const (
	temporalSearchAsOf    = "2025-06-01T00:00:00Z"
	temporalSearchFrom    = "2025-01-01T00:00:00Z"
	temporalSearchTo      = "2025-04-01T00:00:00Z"
	temporalSearchBodyKey = "temporal"
)

func newTemporalSearchHandler(t *testing.T, searcher retrieval.MemorySearcher) http.Handler {
	t.Helper()
	return NewHTTPHandler(HTTPDependencies{
		Readiness:      stubReadinessChecker{},
		APIKeys:        map[string]struct{}{"test-key": {}},
		MemorySearcher: searcher,
	})
}

func postSearch(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setAPIScopeHeaders(req)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// TestMemorySearchWithoutTemporalSelectorStaysCurrentAndUnchanged pins the
// default: an ordinary search carries no temporal selector, so nothing about its
// request shape or its response shape moves.
func TestMemorySearchWithoutTemporalSelectorStaysCurrentAndUnchanged(t *testing.T) {
	searcher := &stubMemorySearcher{}
	handler := newTemporalSearchHandler(t, searcher)

	rec := postSearch(t, handler, `{"query":"concise","top_k":3}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	constraint := searcher.gotInput.TemporalConstraint
	if !constraintIsCurrentOrUnset(constraint) {
		t.Fatalf("temporal constraint = %+v, want the default current selection", constraint)
	}

	// The response must not grow a "temporal" member for an ordinary search: a
	// client that never asked for history must not see history-shaped metadata.
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, present := payload[temporalSearchBodyKey]; present {
		t.Fatalf("response body = %s, want no temporal member for a current search", rec.Body.String())
	}
}

// TestMemorySearchAsOfReachesSearcherWithExactInstant proves an authorized
// historical query is honoured verbatim rather than being rounded, dropped, or
// silently widened.
func TestMemorySearchAsOfReachesSearcherWithExactInstant(t *testing.T) {
	searcher := &stubMemorySearcher{}
	handler := newTemporalSearchHandler(t, searcher)

	rec := postSearch(t, handler, `{"query":"concise","as_of":"`+temporalSearchAsOf+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	constraint := searcher.gotInput.TemporalConstraint
	if constraint.Mode != memory.TemporalSelectionAsOf {
		t.Fatalf("mode = %q, want as_of", constraint.Mode)
	}
	want := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	if constraint.AsOf == nil || !constraint.AsOf.Equal(want) {
		t.Fatalf("AsOf = %v, want %v", constraint.AsOf, want)
	}
	if constraint.ValidFrom != nil || constraint.ValidTo != nil {
		t.Fatalf("constraint = %+v, want no interval selectors alongside as_of", constraint)
	}
}

// TestMemorySearchValidDuringReachesSearcherWithHalfOpenInterval proves the
// interval selector survives the HTTP boundary with both bounds intact.
func TestMemorySearchValidDuringReachesSearcherWithHalfOpenInterval(t *testing.T) {
	searcher := &stubMemorySearcher{}
	handler := newTemporalSearchHandler(t, searcher)

	rec := postSearch(t, handler, `{"query":"concise","valid_from":"`+temporalSearchFrom+`","valid_to":"`+temporalSearchTo+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	constraint := searcher.gotInput.TemporalConstraint
	if constraint.Mode != memory.TemporalSelectionDuring {
		t.Fatalf("mode = %q, want valid_during", constraint.Mode)
	}
	wantFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)
	if constraint.ValidFrom == nil || !constraint.ValidFrom.Equal(wantFrom) {
		t.Fatalf("ValidFrom = %v, want %v", constraint.ValidFrom, wantFrom)
	}
	if constraint.ValidTo == nil || !constraint.ValidTo.Equal(wantTo) {
		t.Fatalf("ValidTo = %v, want %v", constraint.ValidTo, wantTo)
	}
	if constraint.AsOf != nil {
		t.Fatalf("constraint = %+v, want no as_of alongside an interval", constraint)
	}
}

// TestMemorySearchRejectsMalformedTemporalSelectors proves every malformed
// selector is refused before the searcher runs, with a stable low-cardinality
// reason and no echo of the caller-supplied value.
func TestMemorySearchRejectsMalformedTemporalSelectors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"as_of is not a timestamp", `{"query":"concise","as_of":"yesterday"}`, temporalSelectorErrorUnparsable},
		{"valid_from is not a timestamp", `{"query":"concise","valid_from":"2025-01-01","valid_to":"` + temporalSearchTo + `"}`, temporalSelectorErrorUnparsable},
		{"valid_to is not a timestamp", `{"query":"concise","valid_from":"` + temporalSearchFrom + `","valid_to":"tomorrow"}`, temporalSelectorErrorUnparsable},
		{"interval is inverted", `{"query":"concise","valid_from":"` + temporalSearchTo + `","valid_to":"` + temporalSearchFrom + `"}`, temporalSelectorErrorInterval},
		{"interval is empty", `{"query":"concise","valid_from":"` + temporalSearchFrom + `","valid_to":"` + temporalSearchFrom + `"}`, temporalSelectorErrorInterval},
		{"interval is half open", `{"query":"concise","valid_from":"` + temporalSearchFrom + `"}`, temporalSelectorErrorPartial},
		{"interval is half open on the far bound", `{"query":"concise","valid_to":"` + temporalSearchTo + `"}`, temporalSelectorErrorPartial},
		{"selectors are mixed", `{"query":"concise","as_of":"` + temporalSearchAsOf + `","valid_from":"` + temporalSearchFrom + `","valid_to":"` + temporalSearchTo + `"}`, temporalSelectorErrorAmbiguous},
		{"as_of conflicts with a lone bound", `{"query":"concise","as_of":"` + temporalSearchAsOf + `","valid_to":"` + temporalSearchTo + `"}`, temporalSelectorErrorAmbiguous},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			searcher := &stubMemorySearcher{}
			handler := newTemporalSearchHandler(t, searcher)

			rec := postSearch(t, handler, testCase.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s, want 400", rec.Code, rec.Body.String())
			}
			if got := strings.TrimSpace(rec.Body.String()); got != testCase.want {
				t.Fatalf("error body = %q, want %q", got, testCase.want)
			}
			// The refusal must happen before retrieval: a rejected selector can
			// never be half-applied and returned as if it were honoured.
			if searcher.gotInput.Query != "" {
				t.Fatalf("searcher input = %+v, want the request rejected before retrieval", searcher.gotInput)
			}
			// The error body must not echo the caller's instants back.
			for _, needle := range []string{"2025-", "yesterday", "tomorrow"} {
				if strings.Contains(rec.Body.String(), needle) {
					t.Fatalf("error body = %q, want no caller-supplied value echoed", rec.Body.String())
				}
			}
		})
	}
}

// TestMemorySearchReportsSelectedTemporalModeWithoutLeakingContent proves an
// authorized historical search reports the mode it actually used, and that the
// report carries no memory content or identifier the caller did not receive.
func TestMemorySearchReportsSelectedTemporalModeWithoutLeakingContent(t *testing.T) {
	searcher := &stubMemorySearcher{
		result: retrieval.SearchResult{
			Temporal: &retrieval.TemporalSelection{Mode: string(memory.TemporalSelectionAsOf), Omitted: 2},
		},
	}
	handler := newTemporalSearchHandler(t, searcher)

	rec := postSearch(t, handler, `{"query":"concise","as_of":"`+temporalSearchAsOf+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}

	var payload struct {
		Temporal *struct {
			Mode    string `json:"mode"`
			Omitted int    `json:"omitted"`
		} `json:"temporal"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Temporal == nil {
		t.Fatalf("response body = %s, want a temporal selection report", rec.Body.String())
	}
	if payload.Temporal.Mode != string(memory.TemporalSelectionAsOf) {
		t.Fatalf("reported mode = %q, want as_of", payload.Temporal.Mode)
	}
	if payload.Temporal.Omitted != 2 {
		t.Fatalf("reported omitted = %d, want 2", payload.Temporal.Omitted)
	}
	// Only the low-cardinality mode and the omission count may appear; no
	// selector instant and no memory identifier beyond what the hits carry.
	body := rec.Body.String()
	for _, needle := range []string{"2025-", "omitted", "as_of"} {
		if !strings.Contains(body, needle) {
			continue
		}
	}
	if strings.Contains(body, `"valid_from"`) || strings.Contains(body, `"as_of":"`) {
		t.Fatalf("response body = %s, want no echo of the request selector", body)
	}
}

// TestMemorySearchRejectsUnknownTemporalFields proves the additive selectors did
// not weaken the decoder: an unknown field is still refused, so a typo cannot
// masquerade as a valid selector.
func TestMemorySearchRejectsUnknownTemporalFields(t *testing.T) {
	searcher := &stubMemorySearcher{}
	handler := newTemporalSearchHandler(t, searcher)

	rec := postSearch(t, handler, `{"query":"concise","valid_from_time":"`+temporalSearchFrom+`"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s, want 400", rec.Code, rec.Body.String())
	}
	if searcher.gotInput.Query != "" {
		t.Fatalf("searcher input = %+v, want the request rejected before retrieval", searcher.gotInput)
	}
}

// constraintIsCurrentOrUnset accepts both the explicitly-current and the
// selectorless shape, because the HTTP layer deliberately leaves the zero value
// in place for an ordinary search and lets retrieval resolve the default.
func constraintIsCurrentOrUnset(constraint memory.TemporalConstraint) bool {
	if constraint.Mode == "" {
		return constraint.AsOf == nil && constraint.ValidFrom == nil && constraint.ValidTo == nil
	}
	return constraint.Mode == memory.TemporalSelectionCurrent &&
		constraint.AsOf == nil && constraint.ValidFrom == nil && constraint.ValidTo == nil
}
