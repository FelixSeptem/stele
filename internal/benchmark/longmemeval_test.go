package benchmark

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

const longMemEvalFixture = `{
  "s": [{
    "question_id": "q-update",
    "question": "Where does Ana work now?",
    "question_date": "2024-02-15",
    "question_type": "knowledge-update",
    "answer_session_ids": ["session-new"],
    "evidence_session_ids": ["session-old", "session-new"],
    "obsolete_session_ids": ["session-old"],
    "haystack_sessions": [
      {"session_id":"session-old","session_date":"2024-01-01","turns":[{"turn_id":"old-turn","role":"user","content":"Ana works at the museum."}]},
      {"session_id":"session-new","session_date":"2024-02-01","turns":[{"turn_id":"new-turn","role":"user","content":"Ana works at the library now."}]}
    ]
  }],
  "m": [{"question_id":"q-m","question":"ignored by s","question_type":"fact","answer_session_ids":["m-session"],"haystack_sessions":[{"session_id":"m-session","turns":[{"turn_id":"m-turn","content":"m"}]}]}],
  "oracle": [{"question_id":"q-abstain","question":"What is unknown?","question_date":"2024-03-01","question_type":"abstention","abstention":true,"haystack_sessions":[{"session_id":"o-session","turns":[{"turn_id":"o-turn","content":"unrelated"}]}]}]
}`

func TestLoadLongMemEvalDatasetSelectsLockedSubset(t *testing.T) {
	dataset, err := LoadLongMemEvalDataset(bytes.NewBufferString(longMemEvalFixture), LongMemEvalSubsetSmall)
	if err != nil {
		t.Fatal(err)
	}
	if dataset.Subset != LongMemEvalSubsetSmall || len(dataset.Samples) != 1 || dataset.Samples[0].ID != "q-update" {
		t.Fatalf("unexpected selected dataset: %#v", dataset)
	}
	if _, err := LoadLongMemEvalDataset(bytes.NewBufferString(longMemEvalFixture), "invalid"); err == nil {
		t.Fatal("expected invalid subset to be rejected")
	}
}

func TestLoadLongMemEvalDatasetReadsJSONL(t *testing.T) {
	jsonl := `{"subset":"s","question_id":"q-one","question":"one","answer_session_ids":["session-one"],"haystack_sessions":[{"session_id":"session-one","turns":[{"turn_id":"turn-one","content":"one"}]}]}` + "\n" +
		`{"subset":"m","question_id":"q-two","question":"two","answer_session_ids":["session-two"],"haystack_sessions":[{"session_id":"session-two","turns":[{"turn_id":"turn-two","content":"two"}]}]}`
	dataset, err := LoadLongMemEvalDataset(strings.NewReader(jsonl), LongMemEvalSubsetMedium)
	if err != nil {
		t.Fatal(err)
	}
	if len(dataset.Samples) != 1 || dataset.Samples[0].ID != "q-two" {
		t.Fatalf("unexpected JSONL selection: %#v", dataset)
	}
}

func TestLoadLongMemEvalDatasetAcceptsNumericAnswerPayload(t *testing.T) {
	source := `[{"question_id":"q-number","question":"How many meetings did Ana attend?","answer":17,"answer_session_ids":["session-one"],"haystack_sessions":[{"session_id":"session-one","turns":[{"turn_id":"turn-one","content":"Ana attended 17 meetings."}]}]}]`
	dataset, err := LoadLongMemEvalDataset(bytes.NewBufferString(source), LongMemEvalSubsetSmall)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewLongMemEvalAdapter().Normalize(dataset, memoryScope())
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Queries) != 1 || corpus.Queries[0].ID != "query/longmemeval/q-number" {
		t.Fatalf("unexpected corpus normalized from numeric answer payload: %#v", corpus)
	}
}

func TestLongMemEvalNormalizeSkipsEmptyDistractorSession(t *testing.T) {
	source := `[{"question_id":"q-empty-distractor","question":"Where does Ana work?","answer_session_ids":["session-answer"],"haystack_sessions":[{"session_id":"session-empty","turns":[{"turn_id":"blank-turn","content":""}]},{"session_id":"session-answer","turns":[{"turn_id":"answer-turn","content":"Ana works at the library."}]}]}]`
	dataset, err := LoadLongMemEvalDataset(bytes.NewBufferString(source), LongMemEvalSubsetSmall)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewLongMemEvalAdapter().Normalize(dataset, memoryScope())
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Conversations) != 1 || len(corpus.Events) != 1 || corpus.Events[0].ID != "event/longmemeval/q-empty-distractor/session-answer/answer-turn" {
		t.Fatalf("empty distractor session must not produce retrievable memory: %#v", corpus)
	}
}

func TestLongMemEvalNormalizePreservesLifecycleEvidenceAndAbstention(t *testing.T) {
	dataset, err := LoadLongMemEvalDataset(bytes.NewBufferString(longMemEvalFixture), LongMemEvalSubsetSmall)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewLongMemEvalAdapter().Normalize(dataset, memoryScope())
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Conversations) != 2 || len(corpus.Events) != 2 || len(corpus.Queries) != 1 || len(corpus.QRELs) != 2 {
		t.Fatalf("unexpected normalized corpus: %#v", corpus)
	}
	query := corpus.Queries[0]
	if query.QueryType != "knowledge-update" || query.Metadata["question_date"] != "2024-02-15" || len(query.MustNotReturnIDs) != 1 {
		t.Fatalf("missing update metadata: %#v", query)
	}
	qrelGrades := map[string]int{}
	for _, qrel := range corpus.QRELs {
		qrelGrades[qrel.EvidenceID] = qrel.Grade
	}
	if corpus.Events[0].ExpectedState != memory.MemoryStateSuppressed || qrelGrades[corpus.Events[0].ID] != 0 || qrelGrades[corpus.Events[1].ID] != 2 {
		t.Fatalf("expected graded update qrels and obsolete lifecycle: %#v %#v", corpus.Events, corpus.QRELs)
	}

	oracle, err := LoadLongMemEvalDataset(bytes.NewBufferString(longMemEvalFixture), LongMemEvalSubsetOracle)
	if err != nil {
		t.Fatal(err)
	}
	abstention, err := NewLongMemEvalAdapter().Normalize(oracle, memoryScope())
	if err != nil {
		t.Fatal(err)
	}
	if len(abstention.Queries) != 1 || abstention.Queries[0].Metadata["abstention"] != "true" || len(abstention.QRELs) != 0 {
		t.Fatalf("abstention must not invent qrels: %#v", abstention)
	}
}

func TestLongMemEvalNormalizeIsDeterministicAndRejectsBadReferences(t *testing.T) {
	dataset, err := LoadLongMemEvalDataset(bytes.NewBufferString(longMemEvalFixture), LongMemEvalSubsetSmall)
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewLongMemEvalAdapter().Normalize(dataset, memoryScope())
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewLongMemEvalAdapter().Normalize(dataset, memoryScope())
	if err != nil {
		t.Fatal(err)
	}
	firstChecksum, _ := first.Checksum()
	secondChecksum, _ := second.Checksum()
	const goldenChecksum = "cf66150d8bf316ce4bf8622c1de5853ba80b4062202db98624c3111e792fb103"
	if firstChecksum != goldenChecksum || secondChecksum != goldenChecksum {
		t.Fatalf("conversion drift: first=%s second=%s golden=%s", firstChecksum, secondChecksum, goldenChecksum)
	}
	bad := dataset
	bad.Samples = append([]LongMemEvalSample(nil), dataset.Samples...)
	bad.Samples[0].AnswerSessionIDs = []string{"missing"}
	if _, err := NewLongMemEvalAdapter().Normalize(bad, memoryScope()); err == nil {
		t.Fatal("expected missing answer session to be rejected")
	}
	bad = dataset
	bad.Samples = append([]LongMemEvalSample(nil), dataset.Samples...)
	bad.Samples[0].EvidenceSessionIDs = []string{"missing"}
	if _, err := NewLongMemEvalAdapter().Normalize(bad, memoryScope()); err == nil {
		t.Fatal("expected unmapped evidence session to be rejected")
	}
	bad = dataset
	bad.Samples = append([]LongMemEvalSample(nil), dataset.Samples...)
	bad.Samples = append(bad.Samples, bad.Samples[0])
	if _, err := NewLongMemEvalAdapter().Normalize(bad, memoryScope()); err == nil {
		t.Fatal("expected duplicate question ID to be rejected")
	}
}

type longMemEvalTestRetriever struct {
	candidates map[string][]RetrievedEvidence
}

func (r longMemEvalTestRetriever) Retrieve(_ context.Context, query BenchmarkQuery) ([]RetrievedEvidence, error) {
	return r.candidates[query.ID], nil
}

func TestLongMemEvalRetrievalOnlyRunnerSeparatesComparisonModes(t *testing.T) {
	dataset, err := LoadLongMemEvalDataset(bytes.NewBufferString(longMemEvalFixture), LongMemEvalSubsetSmall)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewLongMemEvalAdapter().Normalize(dataset, memoryScope())
	if err != nil {
		t.Fatal(err)
	}
	queryID := corpus.Queries[0].ID
	report, err := (LongMemEvalRunner{}).Run(context.Background(), corpus, LongMemEvalComparisonOracle)
	if err != nil || report.Mode != LongMemEvalComparisonOracle || report.Report.Metrics.RecallAt1 != 1 {
		t.Fatalf("unexpected oracle report: %#v, %v", report, err)
	}
	runner := LongMemEvalRunner{Retriever: longMemEvalTestRetriever{candidates: map[string][]RetrievedEvidence{queryID: {{EvidenceID: corpus.QRELs[1].EvidenceID, Rank: 1}}}}}
	logReport, err := runner.Run(context.Background(), corpus, LongMemEvalComparisonRetrievalLog)
	if err != nil || logReport.Mode != LongMemEvalComparisonRetrievalLog {
		t.Fatalf("retrieval-log must remain a distinct mode: %#v, %v", logReport, err)
	}
}
