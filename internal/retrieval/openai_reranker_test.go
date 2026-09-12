package retrieval

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAIRerankerAdapterValidatesAndNormalizesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/rerank" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"index":1,"relevance_score":0.9},{"index":0,"relevance_score":0.1}]}`))
	}))
	defer server.Close()
	adapter := OpenAIReranker{Endpoint: server.URL + "/v1/rerank", Model: "test", Timeout: time.Second, MaxCandidates: 4, MaxTextBytes: 100}
	got, err := adapter.Rerank(context.Background(), RerankRequest{Query: "q", Candidates: []RerankCandidate{{ID: "m1", Text: "one"}, {ID: "m2", Text: "two"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "m2" || got[0].Score != 0.9 {
		t.Fatalf("result=%+v", got)
	}
}

func TestOpenAIRerankerAdapterFailsClosedOnMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"index":99,"relevance_score":0.9}]}`))
	}))
	defer server.Close()
	adapter := OpenAIReranker{Endpoint: server.URL, Model: "test", Timeout: time.Second, MaxCandidates: 4, MaxTextBytes: 100}
	if _, err := adapter.Rerank(context.Background(), RerankRequest{Query: "q", Candidates: []RerankCandidate{{ID: "m1", Text: "one"}}}); err == nil {
		t.Fatal("expected malformed response error")
	}
}
