package benchmark

import (
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestLongMemEvalAdapterRequiresExplicitFeatureFlag(t *testing.T) {
	adapter := LongMemEvalAdapter{}
	_, err := adapter.NormalizeLocal(longMemEvalScope(), []byte(`{"samples":[]}`))
	if StatusOf(err) != StatusPrerequisiteMissing || !strings.Contains(err.Error(), "feature") {
		t.Fatalf("disabled LongMemEval spike error = %v, want prerequisite_missing feature diagnostic", err)
	}
}

func TestLongMemEvalAdapterNormalizesSessionsUpdatesAndAbstentionDeterministically(t *testing.T) {
	source := []byte(`{
  "samples": [{
    "question_id": "lm-q1",
    "question_type": "update",
    "question": "What is the current editor?",
    "question_date": "2025-02-01",
    "answer_session_ids": ["session-new"],
    "obsolete_session_ids": ["session-old"],
    "abstention": false,
    "sessions": [
      {"session_id":"session-old","date":"2025-01-01","turns":[{"speaker":"user","text":"I use Vim."}]},
      {"session_id":"session-new","date":"2025-01-20","turns":[{"speaker":"user","text":"I switched to Emacs."}]}
    ]
  }, {
    "question_id": "lm-q2",
    "question_type": "abstention",
    "question": "What is the unknown preference?",
    "question_date": "2025-02-02",
    "answer_session_ids": [],
    "abstention": true,
    "sessions": [{"session_id":"session-other","date":"2025-01-02","turns":[{"speaker":"user","text":"We discussed weather."}]}]
  }]
}`)
	adapter := LongMemEvalAdapter{Enabled: true}
	first, err := adapter.NormalizeLocal(longMemEvalScope(), source)
	if err != nil {
		t.Fatalf("NormalizeLocal() error = %v", err)
	}
	second, err := adapter.NormalizeLocal(longMemEvalScope(), source)
	if err != nil {
		t.Fatalf("second NormalizeLocal() error = %v", err)
	}
	firstChecksum, err := first.Checksum()
	if err != nil {
		t.Fatalf("first checksum: %v", err)
	}
	secondChecksum, err := second.Checksum()
	if err != nil {
		t.Fatalf("second checksum: %v", err)
	}
	if firstChecksum != secondChecksum {
		t.Fatalf("normalization checksum changed: %s != %s", firstChecksum, secondChecksum)
	}
	if len(first.Conversations) != 3 || len(first.Events) != 3 || len(first.Queries) != 2 {
		t.Fatalf("unexpected normalized counts: conversations=%d events=%d queries=%d", len(first.Conversations), len(first.Events), len(first.Queries))
	}
	query := findLongMemEvalQuery(first, "lm-q1")
	if query.QueryType != "update" || query.QuestionDate != "2025-02-01" || len(query.AnswerSessionIDs) != 1 || query.AnswerSessionIDs[0] != "session-new" || query.UpdateType != "update" {
		t.Fatalf("update metadata not preserved: %#v", query)
	}
	if len(query.EvidenceGroups) != 1 || len(query.EvidenceGroups[0].EvidenceIDs) != 1 {
		t.Fatalf("answer session evidence group not normalized: %#v", query.EvidenceGroups)
	}
	oldEvent := findLongMemEvalEvent(first, "session-old")
	if oldEvent.ExpectedState != memory.MemoryStateForgotten || oldEvent.Provenance["session_id"] != "session-old" {
		t.Fatalf("obsolete session state/provenance not preserved: %#v", oldEvent)
	}
	abstention := findLongMemEvalQuery(first, "lm-q2")
	if !abstention.AbstentionExpected || abstention.QueryType != "abstention" || len(abstention.EvidenceGroups) != 0 {
		t.Fatalf("abstention metadata not preserved: %#v", abstention)
	}
}

func TestLongMemEvalAdapterRejectsDuplicateSessionsAndMissingAnswers(t *testing.T) {
	duplicate := []byte(`{"samples":[{"question_id":"q","question":"x","sessions":[{"session_id":"s","turns":[{"text":"a"}]},{"session_id":"s","turns":[{"text":"b"}]}]}]}`)
	_, err := (LongMemEvalAdapter{Enabled: true}).NormalizeLocal(longMemEvalScope(), duplicate)
	if err == nil || !strings.Contains(err.Error(), "duplicate session") {
		t.Fatalf("duplicate session error = %v", err)
	}
	missing := []byte(`{"samples":[{"question_id":"q","question":"x","answer_session_ids":["missing"],"sessions":[]}]}`)
	_, err = (LongMemEvalAdapter{Enabled: true}).NormalizeLocal(longMemEvalScope(), missing)
	if err == nil || !strings.Contains(err.Error(), "answer session") {
		t.Fatalf("missing answer session error = %v", err)
	}
}

func longMemEvalScope() memory.Scope {
	return memory.Scope{Tenant: "bench", Project: "longmemeval", Namespace: "fixture"}
}

func findLongMemEvalQuery(corpus NormalizedCorpus, id string) BenchmarkQuery {
	for _, query := range corpus.Queries {
		if query.ID == id {
			return query
		}
	}
	return BenchmarkQuery{}
}

func findLongMemEvalEvent(corpus NormalizedCorpus, sessionID string) MemoryEventRecord {
	for _, event := range corpus.Events {
		if event.SessionID == sessionID {
			return event
		}
	}
	return MemoryEventRecord{}
}
