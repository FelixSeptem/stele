package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// LongMemEvalSubset is deliberately explicit: callers must select the locked
// local artifact population rather than silently accepting a larger split.
type LongMemEvalSubset string

const (
	LongMemEvalSubsetSmall  LongMemEvalSubset = "s"
	LongMemEvalSubsetMedium LongMemEvalSubset = "m"
	LongMemEvalSubsetOracle LongMemEvalSubset = "oracle"
)

func (s LongMemEvalSubset) Validate() error {
	switch s {
	case LongMemEvalSubsetSmall, LongMemEvalSubsetMedium, LongMemEvalSubsetOracle:
		return nil
	default:
		return fmt.Errorf("unsupported LongMemEval subset %q", s)
	}
}

type LongMemEvalDataset struct {
	Subset  LongMemEvalSubset
	Samples []LongMemEvalSample
}

type LongMemEvalSample struct {
	ID                    string
	Question              string
	QuestionDate          string
	QuestionType          string
	AnswerSessionIDs      []string
	EvidenceSessionIDs    []string
	ObsoleteSessionIDs    []string
	MustNotReturnSessions []string
	Abstention            bool
	Sessions              []LongMemEvalSession
}

type LongMemEvalSession struct {
	ID    string
	Date  string
	Turns []LongMemEvalTurn
}

type LongMemEvalTurn struct {
	ID        string
	Speaker   string
	Text      string
	Timestamp string
}

// LoadLongMemEvalDataset loads only caller-provided, already locked files. It
// accepts an upstream JSON object keyed by s/m/oracle, a JSON array containing
// one local subset, and JSONL whose records contain an optional subset field.
func LoadLongMemEvalDataset(source io.Reader, subset LongMemEvalSubset) (LongMemEvalDataset, error) {
	if source == nil {
		return LongMemEvalDataset{}, fmt.Errorf("LongMemEval source is required")
	}
	if err := subset.Validate(); err != nil {
		return LongMemEvalDataset{}, err
	}
	payload, err := io.ReadAll(source)
	if err != nil {
		return LongMemEvalDataset{}, fmt.Errorf("read LongMemEval source: %w", err)
	}
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return LongMemEvalDataset{}, fmt.Errorf("LongMemEval source is empty")
	}
	var rawSamples []json.RawMessage
	if payload[0] == '{' {
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal(payload, &wrapper); err != nil {
			rawSamples, err = longMemEvalJSONLSamples(payload, subset)
			if err != nil {
				return LongMemEvalDataset{}, err
			}
		} else {
			if selected, ok := wrapper[string(subset)]; ok {
				if err := json.Unmarshal(selected, &rawSamples); err != nil {
					return LongMemEvalDataset{}, fmt.Errorf("decode LongMemEval %s subset: %w", subset, err)
				}
			} else if samples, ok := wrapper["samples"]; ok {
				if err := json.Unmarshal(samples, &rawSamples); err != nil {
					return LongMemEvalDataset{}, fmt.Errorf("decode LongMemEval samples: %w", err)
				}
			} else if _, hasQuestion := wrapper["question"]; hasQuestion {
				rawSamples = []json.RawMessage{payload}
			} else {
				return LongMemEvalDataset{}, fmt.Errorf("LongMemEval source does not contain %s subset", subset)
			}
		}
	} else if payload[0] == '[' {
		if err := json.Unmarshal(payload, &rawSamples); err != nil {
			return LongMemEvalDataset{}, fmt.Errorf("decode LongMemEval JSON array: %w", err)
		}
	} else {
		rawSamples, err = longMemEvalJSONLSamples(payload, subset)
		if err != nil {
			return LongMemEvalDataset{}, err
		}
	}
	if len(rawSamples) == 0 {
		return LongMemEvalDataset{}, fmt.Errorf("LongMemEval %s subset has no samples", subset)
	}
	dataset := LongMemEvalDataset{Subset: subset, Samples: make([]LongMemEvalSample, 0, len(rawSamples))}
	for _, raw := range rawSamples {
		sample, err := decodeLongMemEvalSample(raw)
		if err != nil {
			return LongMemEvalDataset{}, err
		}
		dataset.Samples = append(dataset.Samples, sample)
	}
	return dataset, nil
}

func longMemEvalJSONLSamples(payload []byte, subset LongMemEvalSubset) ([]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	var samples []json.RawMessage
	for {
		var record json.RawMessage
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode LongMemEval JSONL: %w", err)
		}
		var envelope struct {
			Subset string `json:"subset"`
		}
		if err := json.Unmarshal(record, &envelope); err != nil {
			return nil, fmt.Errorf("decode LongMemEval JSONL record: %w", err)
		}
		if envelope.Subset == "" || envelope.Subset == string(subset) {
			samples = append(samples, record)
		}
	}
	return samples, nil
}

func LoadLongMemEvalDatasetFromBytes(source []byte, subset LongMemEvalSubset) (LongMemEvalDataset, error) {
	return LoadLongMemEvalDataset(bytes.NewReader(source), subset)
}

func decodeLongMemEvalSample(raw json.RawMessage) (LongMemEvalSample, error) {
	var value struct {
		ID                    string          `json:"id"`
		QuestionID            string          `json:"question_id"`
		Question              string          `json:"question"`
		QuestionDate          string          `json:"question_date"`
		QuestionType          string          `json:"question_type"`
		AnswerSessionIDs      json.RawMessage `json:"answer_session_ids"`
		EvidenceSessionIDs    json.RawMessage `json:"evidence_session_ids"`
		ObsoleteSessionIDs    json.RawMessage `json:"obsolete_session_ids"`
		MustNotReturnSessions json.RawMessage `json:"must_not_return_session_ids"`
		Abstention            bool            `json:"abstention"`
		// Answer is preserved as an opaque upstream payload. LongMemEval uses
		// both string and numeric gold answers, while retrieval-only evaluation
		// derives relevance from answer-session evidence rather than this value.
		Answer             json.RawMessage   `json:"answer"`
		HaystackSessionIDs json.RawMessage   `json:"haystack_session_ids"`
		HaystackSessions   []json.RawMessage `json:"haystack_sessions"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return LongMemEvalSample{}, fmt.Errorf("decode LongMemEval sample: %w", err)
	}
	id := firstNonEmpty(value.QuestionID, value.ID)
	if strings.TrimSpace(id) == "" || strings.TrimSpace(value.Question) == "" {
		return LongMemEvalSample{}, fmt.Errorf("LongMemEval sample has missing question id or question")
	}
	answerIDs, err := longMemEvalIDs(value.AnswerSessionIDs)
	if err != nil {
		return LongMemEvalSample{}, fmt.Errorf("LongMemEval question %s answer session ids: %w", id, err)
	}
	evidenceIDs, err := longMemEvalIDs(value.EvidenceSessionIDs)
	if err != nil {
		return LongMemEvalSample{}, fmt.Errorf("LongMemEval question %s evidence session ids: %w", id, err)
	}
	obsoleteIDs, err := longMemEvalIDs(value.ObsoleteSessionIDs)
	if err != nil {
		return LongMemEvalSample{}, fmt.Errorf("LongMemEval question %s obsolete session ids: %w", id, err)
	}
	mustNotIDs, err := longMemEvalIDs(value.MustNotReturnSessions)
	if err != nil {
		return LongMemEvalSample{}, fmt.Errorf("LongMemEval question %s must-not-return session ids: %w", id, err)
	}
	haystackIDs, err := longMemEvalIDs(value.HaystackSessionIDs)
	if err != nil {
		return LongMemEvalSample{}, fmt.Errorf("LongMemEval question %s haystack session ids: %w", id, err)
	}
	sample := LongMemEvalSample{ID: id, Question: value.Question, QuestionDate: value.QuestionDate, QuestionType: value.QuestionType, AnswerSessionIDs: answerIDs, EvidenceSessionIDs: evidenceIDs, ObsoleteSessionIDs: obsoleteIDs, MustNotReturnSessions: mustNotIDs, Abstention: value.Abstention || strings.EqualFold(strings.TrimSpace(value.QuestionType), "abstention")}
	for index, rawSession := range value.HaystackSessions {
		session, err := decodeLongMemEvalSession(rawSession, at(haystackIDs, index), index)
		if err != nil {
			return LongMemEvalSample{}, fmt.Errorf("LongMemEval question %s: %w", id, err)
		}
		if len(session.Turns) == 0 {
			// An upstream empty distractor session has no retrievable memory. Do
			// not manufacture an event; later answer/evidence mapping still
			// rejects it if the question declares that session as required.
			continue
		}
		sample.Sessions = append(sample.Sessions, session)
	}
	if len(sample.Sessions) == 0 {
		return LongMemEvalSample{}, fmt.Errorf("LongMemEval question %s has no haystack sessions", id)
	}
	return sample, nil
}

func decodeLongMemEvalSession(raw json.RawMessage, fallbackID string, index int) (LongMemEvalSession, error) {
	var object struct {
		ID          string            `json:"id"`
		SessionID   string            `json:"session_id"`
		Date        string            `json:"date"`
		SessionDate string            `json:"session_date"`
		Timestamp   string            `json:"timestamp"`
		Turns       []json.RawMessage `json:"turns"`
	}
	var turns []json.RawMessage
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) { // The upstream format also represents a session as a turn array.
		if err := json.Unmarshal(raw, &turns); err != nil {
			return LongMemEvalSession{}, fmt.Errorf("session %d has no turns", index)
		}
	} else {
		if err := json.Unmarshal(raw, &object); err != nil {
			return LongMemEvalSession{}, err
		}
		turns = object.Turns
	}
	id := firstNonEmpty(object.SessionID, object.ID, fallbackID)
	if strings.TrimSpace(id) == "" {
		id = fmt.Sprintf("session-%03d", index+1)
	}
	session := LongMemEvalSession{ID: id, Date: firstNonEmpty(object.SessionDate, object.Date, object.Timestamp)}
	for turnIndex, rawTurn := range turns {
		var turn struct {
			ID        string `json:"id"`
			TurnID    string `json:"turn_id"`
			Speaker   string `json:"speaker"`
			Role      string `json:"role"`
			Text      string `json:"text"`
			Content   string `json:"content"`
			Timestamp string `json:"timestamp"`
			Date      string `json:"date"`
		}
		if err := json.Unmarshal(rawTurn, &turn); err != nil {
			return LongMemEvalSession{}, fmt.Errorf("decode turn: %w", err)
		}
		text := firstNonEmpty(turn.Content, turn.Text)
		if strings.TrimSpace(text) == "" {
			continue
		}
		turnID := firstNonEmpty(turn.TurnID, turn.ID)
		if turnID == "" {
			turnID = fmt.Sprintf("turn-%03d", turnIndex+1)
		}
		session.Turns = append(session.Turns, LongMemEvalTurn{ID: turnID, Speaker: firstNonEmpty(turn.Speaker, turn.Role), Text: text, Timestamp: firstNonEmpty(turn.Timestamp, turn.Date, session.Date)})
	}
	return session, nil
}

func longMemEvalIDs(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		id := strings.TrimSpace(fmt.Sprint(value))
		if id == "" || id == "<nil>" {
			return nil, fmt.Errorf("contains an empty id")
		}
		result = append(result, id)
	}
	return uniqueSorted(result), nil
}

type LongMemEvalAdapter struct{}

func NewLongMemEvalAdapter() LongMemEvalAdapter { return LongMemEvalAdapter{} }

func (LongMemEvalAdapter) Normalize(dataset LongMemEvalDataset, scope memory.Scope) (NormalizedCorpus, error) {
	if err := dataset.Subset.Validate(); err != nil {
		return NormalizedCorpus{}, err
	}
	if err := scope.Validate(); err != nil {
		return NormalizedCorpus{}, fmt.Errorf("validate benchmark scope: %w", err)
	}
	corpus := NormalizedCorpus{SchemaVersion: SchemaVersion}
	questionIDs := make(map[string]struct{}, len(dataset.Samples))
	for _, sample := range dataset.Samples {
		if strings.TrimSpace(sample.ID) == "" || strings.TrimSpace(sample.Question) == "" {
			return NormalizedCorpus{}, fmt.Errorf("LongMemEval sample has missing question id or question")
		}
		if _, exists := questionIDs[sample.ID]; exists {
			return NormalizedCorpus{}, fmt.Errorf("duplicate LongMemEval question id %q", sample.ID)
		}
		questionIDs[sample.ID] = struct{}{}
		sessionEvents := map[string][]string{}
		for _, session := range sample.Sessions {
			if strings.TrimSpace(session.ID) == "" || len(session.Turns) == 0 {
				return NormalizedCorpus{}, fmt.Errorf("LongMemEval question %s has malformed session", sample.ID)
			}
			conversationID := "longmemeval/" + sample.ID + "/" + session.ID
			conversation := ConversationRecord{ID: conversationID, Scope: scope, Provenance: map[string]string{"dataset": "longmemeval", "subset": string(dataset.Subset), "question_id": sample.ID, "session_id": session.ID}}
			seenTurns := map[string]struct{}{}
			for _, turn := range session.Turns {
				if strings.TrimSpace(turn.ID) == "" || strings.TrimSpace(turn.Text) == "" {
					return NormalizedCorpus{}, fmt.Errorf("LongMemEval question %s session %s has malformed turn", sample.ID, session.ID)
				}
				if _, exists := seenTurns[turn.ID]; exists {
					return NormalizedCorpus{}, fmt.Errorf("LongMemEval question %s session %s has duplicate turn id %q", sample.ID, session.ID, turn.ID)
				}
				seenTurns[turn.ID] = struct{}{}
				source := sample.ID + "/" + session.ID + "/" + turn.ID
				eventID := "event/longmemeval/" + source
				conversation.Turns = append(conversation.Turns, ConversationTurn{ID: turn.ID, Speaker: turn.Speaker, Text: turn.Text, Timestamp: turn.Timestamp, Source: source})
				corpus.Events = append(corpus.Events, MemoryEventRecord{ID: eventID, Scope: scope, SessionID: conversationID, SourceTurnID: turn.ID, Class: longMemEvalMemoryClass(sample.QuestionType), Text: turn.Text, ObservedAt: firstNonEmpty(turn.Timestamp, session.Date), ExpectedState: memory.MemoryStateActive, Provenance: map[string]string{"dataset": "longmemeval", "subset": string(dataset.Subset), "question_id": sample.ID, "session_id": session.ID, "source": source}})
				sessionEvents[session.ID] = append(sessionEvents[session.ID], eventID)
			}
			corpus.Conversations = append(corpus.Conversations, conversation)
		}
		if sample.Abstention && len(sample.AnswerSessionIDs) > 0 {
			return NormalizedCorpus{}, fmt.Errorf("LongMemEval abstention question %s has answer sessions", sample.ID)
		}
		answerEvents, err := longMemEvalSessionEvents(sample.ID, "answer", sample.AnswerSessionIDs, sessionEvents)
		if err != nil {
			return NormalizedCorpus{}, err
		}
		evidenceIDs := append([]string(nil), sample.EvidenceSessionIDs...)
		if len(evidenceIDs) == 0 {
			evidenceIDs = append(evidenceIDs, sample.AnswerSessionIDs...)
		}
		evidenceEvents, err := longMemEvalSessionEvents(sample.ID, "evidence", evidenceIDs, sessionEvents)
		if err != nil {
			return NormalizedCorpus{}, err
		}
		obsoleteEvents, err := longMemEvalSessionEvents(sample.ID, "obsolete", append(append([]string(nil), sample.ObsoleteSessionIDs...), sample.MustNotReturnSessions...), sessionEvents)
		if err != nil {
			return NormalizedCorpus{}, err
		}
		obsoleteSet := make(map[string]struct{}, len(obsoleteEvents))
		for _, eventID := range obsoleteEvents {
			obsoleteSet[eventID] = struct{}{}
		}
		for index := range corpus.Events {
			if _, obsolete := obsoleteSet[corpus.Events[index].ID]; obsolete {
				corpus.Events[index].ExpectedState = memory.MemoryStateSuppressed
			}
		}
		queryID := "query/longmemeval/" + sample.ID
		metadata := map[string]string{"dataset": "longmemeval", "subset": string(dataset.Subset)}
		if sample.QuestionDate != "" {
			metadata["question_date"] = sample.QuestionDate
		}
		if sample.Abstention {
			metadata["abstention"] = "true"
		}
		query := BenchmarkQuery{ID: queryID, Scope: scope, Text: sample.Question, QueryType: normalizedQueryType(sample.QuestionType), MustNotReturnIDs: uniqueSorted(obsoleteEvents), Metadata: metadata}
		if !sample.Abstention && len(answerEvents) == 0 {
			return NormalizedCorpus{}, fmt.Errorf("LongMemEval question %s has missing answer session", sample.ID)
		}
		if len(answerEvents) > 0 {
			groupID := queryID + "/answer-sessions"
			query.EvidenceGroups = []EvidenceGroup{{ID: groupID, EvidenceIDs: uniqueSorted(answerEvents), Required: true}}
		}
		corpus.Queries = append(corpus.Queries, query)
		answerSet := map[string]struct{}{}
		for _, eventID := range answerEvents {
			answerSet[eventID] = struct{}{}
		}
		for _, eventID := range uniqueSorted(evidenceEvents) {
			grade, role, group := 1, "supporting", ""
			if _, answer := answerSet[eventID]; answer {
				grade, role, group = 2, "answer-session", queryID+"/answer-sessions"
			}
			if _, obsolete := obsoleteSet[eventID]; obsolete {
				grade, role, group = 0, "obsolete", ""
			}
			corpus.QRELs = append(corpus.QRELs, QREL{QueryID: queryID, EvidenceID: eventID, Grade: grade, Role: role, GroupID: group, Expectation: longMemEvalExpectation(eventID, obsoleteSet)})
		}
	}
	if err := corpus.Validate(); err != nil {
		return NormalizedCorpus{}, err
	}
	return corpus, nil
}

func longMemEvalSessionEvents(questionID, kind string, sessionIDs []string, sessions map[string][]string) ([]string, error) {
	var events []string
	for _, sessionID := range uniqueSorted(sessionIDs) {
		mapped, found := sessions[sessionID]
		if !found {
			return nil, fmt.Errorf("LongMemEval question %s references unmapped %s session %s", questionID, kind, sessionID)
		}
		events = append(events, mapped...)
	}
	return uniqueSorted(events), nil
}

func longMemEvalExpectation(eventID string, obsolete map[string]struct{}) string {
	if _, found := obsolete[eventID]; found {
		return "suppressed"
	}
	return "active"
}

func longMemEvalMemoryClass(questionType string) memory.MemoryClass {
	if strings.Contains(strings.ToLower(questionType), "preference") {
		return memory.MemoryClassProfile
	}
	return memory.MemoryClassEpisodic
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = struct{}{}
		}
	}
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
func at(values []string, index int) string {
	if index >= 0 && index < len(values) {
		return values[index]
	}
	return ""
}

// LongMemEvalRetriever is deliberately retrieval-only: answer strings are not
// sent to an LLM judge and no model-answer score is produced.
type LongMemEvalRetriever interface {
	Retrieve(context.Context, BenchmarkQuery) ([]RetrievedEvidence, error)
}

type LongMemEvalComparisonMode string

const (
	LongMemEvalComparisonSteleRetrieval LongMemEvalComparisonMode = "stele-retrieval"
	LongMemEvalComparisonOracle         LongMemEvalComparisonMode = "oracle"
	LongMemEvalComparisonRetrievalLog   LongMemEvalComparisonMode = "retrieval-log"
)

type LongMemEvalRunReport struct {
	Mode   LongMemEvalComparisonMode `json:"comparison_mode"`
	Report EvaluationReport          `json:"report"`
}
type LongMemEvalRunner struct{ Retriever LongMemEvalRetriever }

func (r LongMemEvalRunner) Run(ctx context.Context, corpus NormalizedCorpus, mode LongMemEvalComparisonMode) (LongMemEvalRunReport, error) {
	switch mode {
	case LongMemEvalComparisonSteleRetrieval, LongMemEvalComparisonOracle, LongMemEvalComparisonRetrievalLog:
	default:
		return LongMemEvalRunReport{}, fmt.Errorf("unsupported LongMemEval comparison mode %q", mode)
	}
	if err := corpus.Validate(); err != nil {
		return LongMemEvalRunReport{}, fmt.Errorf("validate LongMemEval corpus: %w", err)
	}
	if mode != LongMemEvalComparisonOracle && r.Retriever == nil {
		return LongMemEvalRunReport{}, fmt.Errorf("LongMemEval retriever is required")
	}
	canonical := corpus.Canonical()
	evaluations := make([]QueryEvaluation, 0, len(canonical.Queries))
	for _, query := range canonical.Queries {
		started := time.Now()
		var candidates []RetrievedEvidence
		if mode == LongMemEvalComparisonOracle {
			candidates = longMemEvalOracleCandidates(query.ID, canonical.QRELs)
		} else {
			var err error
			candidates, err = r.Retriever.Retrieve(ctx, query)
			if err != nil {
				return LongMemEvalRunReport{}, fmt.Errorf("retrieve LongMemEval query %s: %w", query.ID, err)
			}
		}
		evaluations = append(evaluations, EvaluateQuery(query, canonical.QRELs, candidates, time.Since(started)))
	}
	return LongMemEvalRunReport{Mode: mode, Report: AggregateEvaluation(evaluations)}, nil
}

func longMemEvalOracleCandidates(queryID string, qrels []QREL) []RetrievedEvidence {
	items := make([]QREL, 0)
	for _, qrel := range qrels {
		if qrel.QueryID == queryID && qrel.Grade > 0 {
			items = append(items, qrel)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Grade == items[j].Grade {
			return items[i].EvidenceID < items[j].EvidenceID
		}
		return items[i].Grade > items[j].Grade
	})
	candidates := make([]RetrievedEvidence, 0, len(items))
	for index, qrel := range items {
		candidates = append(candidates, RetrievedEvidence{EvidenceID: qrel.EvidenceID, Rank: index + 1})
	}
	return candidates
}
