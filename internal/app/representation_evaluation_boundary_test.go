package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

// EV3 is intentionally evaluation-only. Its internal shadow evaluator must
// neither register a public route nor perturb ordinary memory search output.
func TestRepresentationEvaluationHasNoPublicRouteOrDefaultSearchEffect(t *testing.T) {
	searcher := &stubMemorySearcher{}
	handler := NewHTTPHandler(HTTPDependencies{Readiness: stubReadinessChecker{}, APIKeys: map[string]struct{}{"test-key": {}}, MemorySearcher: searcher})
	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/memories/search", bytes.NewBufferString(`{"query":"concise","top_k":3}`))
		req.Header.Set("Content-Type", "application/json")
		setAPIScopeHeaders(req)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}

	baseline := request()
	if baseline.Code != http.StatusOK {
		t.Fatalf("baseline search status=%d body=%s", baseline.Code, baseline.Body.String())
	}
	report, err := retrieval.EvaluateRepresentation(retrieval.RepresentationEvaluationInput{
		Baseline: &retrieval.RepresentationComparisonMetrics{CriticalFactRetention: 1, SourceCoverage: 1, CitationCoverage: 1, CandidateCount: 1, CharacterCount: 16, TokenCount: 4, LatencyMS: 1},
		Scope:    memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"},
		Policy:   retrieval.RepresentationEvaluationPolicy{Version: "representation-evaluation-v1", Baseline: "flat-fusion-v1", Candidate: retrieval.RepresentationCandidateExactExcerptWithSearch, Mode: retrieval.RepresentationEvaluationModeShadow, Limits: retrieval.RepresentationEvaluationLimits{MaxCandidates: 4, MaxCharacters: 256, MaxTokens: 64, MaxLatencyMS: 100}},
		Family:   retrieval.RepresentationTaskFamilyExactFacts, FixtureVersion: "representation-fixture-v1", SourceChecksum: strings.Repeat("a", 64), QRELChecksum: strings.Repeat("b", 64), SourceWatermark: "watermark-v1", RendererVersion: "renderer-v1", EvaluatedAt: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
		CriticalFactsExpected: 1, CriticalFactsReturned: 1, ExpectedSources: 1, ReturnedSources: 1, ExpectedCitations: 1, ReturnedCitations: 1, CandidateCount: 1, CharacterCount: 16, TokenCount: 4, LatencyMS: 1, SourceSearchRetained: true, DeterministicReplay: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Comparable {
		t.Fatalf("shadow comparison failed: %+v", report)
	}
	after := request()
	if after.Code != http.StatusOK || after.Body.String() != baseline.Body.String() {
		t.Fatalf("ordinary search changed after shadow evaluation: before=%q after=%q", baseline.Body.String(), after.Body.String())
	}

	noRoute := httptest.NewRecorder()
	handler.ServeHTTP(noRoute, httptest.NewRequest(http.MethodGet, "/v1/admin/representation-evaluations", nil))
	if noRoute.Code != http.StatusNotFound {
		t.Fatalf("representation evaluation must not add a public/admin HTTP route: status=%d body=%s", noRoute.Code, noRoute.Body.String())
	}
}
