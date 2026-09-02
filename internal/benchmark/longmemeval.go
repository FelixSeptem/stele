package benchmark

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

// LongMemEvalAdapter is an intentionally gated normalization spike. The
// upstream dataset remains metadata-only until its full schema and retrieval
// quality gate are validated.
type LongMemEvalAdapter struct {
	Enabled bool
	Subset  string
}

type LongMemEvalDataset struct {
	Samples []LongMemEvalSample `json:"samples"`
}

type LongMemEvalSample struct {
	Subset             string               `json:"subset,omitempty"`
	QuestionID         string               `json:"question_id"`
	QuestionType       string               `json:"question_type,omitempty"`
	Question           string               `json:"question"`
	QuestionDate       string               `json:"question_date,omitempty"`
	AnswerSessionIDs   []string             `json:"answer_session_ids,omitempty"`
	ObsoleteSessionIDs []string             `json:"obsolete_session_ids,omitempty"`
	ConflictSessionIDs []string             `json:"conflict_session_ids,omitempty"`
	Abstention         bool                 `json:"abstention,omitempty"`
	Sessions           []LongMemEvalSession `json:"sessions"`
	HaystackDates      []string             `json:"haystack_dates,omitempty"`
	HaystackSessionIDs []string             `json:"haystack_session_ids,omitempty"`
	HaystackSessions   [][]LongMemEvalTurn  `json:"haystack_sessions,omitempty"`
}

type LongMemEvalSession struct {
	SessionID string            `json:"session_id"`
	Date      string            `json:"date,omitempty"`
	Turns     []LongMemEvalTurn `json:"turns"`
}

type LongMemEvalTurn struct {
	ID        string `json:"id,omitempty"`
	Speaker   string `json:"speaker,omitempty"`
	Text      string `json:"text"`
	Content   string `json:"content,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

func LoadLongMemEvalDatasetFromBytes(source []byte) (LongMemEvalDataset, error) {
	var dataset LongMemEvalDataset
	trimmed := bytes.TrimSpace(source)
	if len(trimmed) == 0 {
		return LongMemEvalDataset{}, fmt.Errorf("decode longmemeval dataset: empty source")
	}
	if err := json.Unmarshal(trimmed, &dataset); err != nil {
		var samples []LongMemEvalSample
		if arrayErr := json.Unmarshal(trimmed, &samples); arrayErr != nil {
			return LongMemEvalDataset{}, fmt.Errorf("decode longmemeval dataset: %w", err)
		}
		dataset.Samples = samples
	}
	if len(dataset.Samples) == 0 {
		return LongMemEvalDataset{}, fmt.Errorf("longmemeval dataset must include at least one sample")
	}
	return dataset, nil
}

// LoadLongMemEvalSubset accepts the object, array, and JSONL forms used by
// local dataset mirrors and applies an explicit s/m/oracle subset filter.
func LoadLongMemEvalSubset(source []byte, subset string) (LongMemEvalDataset, error) {
	var dataset LongMemEvalDataset
	trimmed := bytes.TrimSpace(source)
	if bytes.HasPrefix(trimmed, []byte("{")) || bytes.HasPrefix(trimmed, []byte("[")) {
		loaded, err := LoadLongMemEvalDatasetFromBytes(trimmed)
		if err == nil {
			dataset = loaded
		} else if !bytes.Contains(trimmed, []byte("\n")) {
			return dataset, err
		}
	}
	if len(dataset.Samples) == 0 {
		scanner := bufio.NewScanner(bytes.NewReader(source))
		scanner.Buffer(make([]byte, 1024), 16*1024*1024)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			var sample LongMemEvalSample
			if err := json.Unmarshal(line, &sample); err != nil {
				return dataset, fmt.Errorf("decode longmemeval jsonl: %w", err)
			}
			dataset.Samples = append(dataset.Samples, sample)
		}
		if err := scanner.Err(); err != nil {
			return dataset, err
		}
		if len(dataset.Samples) == 0 {
			return dataset, fmt.Errorf("longmemeval dataset must include at least one sample")
		}
	}
	subset = strings.ToLower(strings.TrimSpace(subset))
	if subset == "" {
		return dataset, nil
	}
	if subset != "s" && subset != "m" && subset != "oracle" {
		return LongMemEvalDataset{}, &StatusError{Status: StatusInvalidManifest, Message: "longmemeval subset must be s, m, or oracle"}
	}
	filtered := dataset.Samples[:0]
	for _, sample := range dataset.Samples {
		value := strings.ToLower(strings.TrimSpace(sample.Subset))
		if value == "" || value == subset || (subset == "oracle" && (value == "s" || value == "m")) {
			filtered = append(filtered, sample)
		}
	}
	if len(filtered) == 0 {
		return LongMemEvalDataset{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "requested longmemeval subset is empty"}
	}
	dataset.Samples = filtered
	return dataset, nil
}

func (a LongMemEvalAdapter) NormalizeLocal(scope memory.Scope, source []byte) (NormalizedCorpus, error) {
	if !a.Enabled {
		return NormalizedCorpus{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "longmemeval adapter feature flag is disabled"}
	}
	if err := scope.Validate(); err != nil {
		return NormalizedCorpus{}, fmt.Errorf("longmemeval scope: %w", err)
	}
	dataset, err := LoadLongMemEvalSubset(source, a.Subset)
	if err != nil {
		return NormalizedCorpus{}, err
	}
	return normalizeLongMemEval(dataset, scope)
}

func normalizeLongMemEval(dataset LongMemEvalDataset, scope memory.Scope) (NormalizedCorpus, error) {
	samples := append([]LongMemEvalSample(nil), dataset.Samples...)
	sort.SliceStable(samples, func(i, j int) bool { return samples[i].QuestionID < samples[j].QuestionID })
	corpus := NormalizedCorpus{SchemaVersion: SchemaVersion}
	for _, sample := range samples {
		questionID := strings.TrimSpace(sample.QuestionID)
		if questionID == "" || strings.TrimSpace(sample.Question) == "" {
			return NormalizedCorpus{}, fmt.Errorf("longmemeval question id and text are required")
		}
		obsolete := stringSet(sample.ObsoleteSessionIDs)
		conflicts := stringSet(sample.ConflictSessionIDs)
		answers := stringSet(sample.AnswerSessionIDs)
		sessions, err := longMemEvalSessions(sample)
		if err != nil && len(sample.AnswerSessionIDs) == 0 {
			return NormalizedCorpus{}, fmt.Errorf("longmemeval question %s: %w", questionID, err)
		}
		if err != nil {
			sessions = nil
		}
		haystackSource := len(sample.HaystackSessions) > 0
		sort.SliceStable(sessions, func(i, j int) bool { return sessions[i].SessionID < sessions[j].SessionID })
		sessionEvents := make(map[string][]string, len(sessions))
		seenSessions := make(map[string]struct{}, len(sessions))
		sessionOccurrences := make(map[string]int, len(sessions))
		for _, session := range sessions {
			sourceSessionID := strings.TrimSpace(session.SessionID)
			sessionID := sourceSessionID
			if sessionID == "" {
				return NormalizedCorpus{}, fmt.Errorf("longmemeval question %s session id is required", questionID)
			}
			sessionOccurrences[sourceSessionID]++
			if _, exists := seenSessions[sessionID]; exists {
				if len(sample.HaystackSessions) > 0 {
					sessionID = fmt.Sprintf("%s#%02d", sourceSessionID, sessionOccurrences[sourceSessionID])
				} else {
					return NormalizedCorpus{}, fmt.Errorf("longmemeval question %s has duplicate session %s", questionID, sessionID)
				}
			}
			seenSessions[sessionID] = struct{}{}
			conversationID := longMemEvalID(questionID, sessionID)
			conversation := ConversationRecord{ID: conversationID, Scope: scope, Provenance: map[string]string{"dataset": "longmemeval", "question_id": questionID, "session_id": sessionID, "session_date": session.Date}}
			turns := append([]LongMemEvalTurn(nil), session.Turns...)
			for index, turn := range turns {
				text := strings.TrimSpace(turn.Text)
				if text == "" {
					text = strings.TrimSpace(turn.Content)
				}
				if text == "" {
					if haystackSource {
						continue
					}
					return NormalizedCorpus{}, fmt.Errorf("longmemeval question %s session %s has malformed turn", questionID, sessionID)
				}
				turnID := strings.TrimSpace(turn.ID)
				if turnID == "" {
					turnID = fmt.Sprintf("turn-%04d", index)
				}
				eventID := fmt.Sprintf("%s:%s", conversationID, turnID)
				state := memory.MemoryStateActive
				expectation := "active"
				if _, isObsolete := obsolete[sessionID]; isObsolete {
					state = memory.MemoryStateForgotten
					expectation = "obsolete"
				} else if _, isConflict := conflicts[sessionID]; isConflict {
					state = memory.MemoryStateCandidate
					expectation = "conflict"
				}
				conversation.Turns = append(conversation.Turns, ConversationTurn{ID: turnID, Speaker: turn.Speaker, Text: text, Timestamp: turn.Timestamp, Source: eventID})
				class := memory.MemoryClassEpisodic
				if isProfileQuestion(sample.QuestionType) {
					class = memory.MemoryClassProfile
				}
				corpus.Events = append(corpus.Events, MemoryEventRecord{ID: eventID, Scope: scope, SessionID: sessionID, SourceTurnID: turnID, Class: class, Text: text, ObservedAt: turn.Timestamp, ExpectedState: state, Provenance: map[string]string{"dataset": "longmemeval", "question_id": questionID, "session_id": sessionID, "session_date": session.Date, "question_date": sample.QuestionDate, "expectation": expectation}})
				sessionEvents[sourceSessionID] = append(sessionEvents[sourceSessionID], eventID)
			}
			corpus.Conversations = append(corpus.Conversations, conversation)
		}
		for answerSessionID := range answers {
			if _, found := sessionEvents[answerSessionID]; !found {
				return NormalizedCorpus{}, fmt.Errorf("longmemeval question %s references missing answer session %s", questionID, answerSessionID)
			}
		}
		query := BenchmarkQuery{ID: questionID, Scope: scope, SessionID: questionID, Text: strings.TrimSpace(sample.Question), QueryType: normalizedLongMemEvalQuestionType(sample.QuestionType), QuestionDate: sample.QuestionDate, AnswerSessionIDs: append([]string(nil), sample.AnswerSessionIDs...), AbstentionExpected: sample.Abstention, UpdateType: normalizedLongMemEvalUpdateType(sample.QuestionType)}
		answerIDs := append([]string(nil), sample.AnswerSessionIDs...)
		sort.Strings(answerIDs)
		for index, sessionID := range answerIDs {
			ids := append([]string(nil), sessionEvents[sessionID]...)
			sort.Strings(ids)
			groupID := fmt.Sprintf("%s-answer-%02d", questionID, index)
			query.EvidenceGroups = append(query.EvidenceGroups, EvidenceGroup{ID: groupID, EvidenceIDs: ids, Required: true})
			for _, evidenceID := range ids {
				corpus.QRELs = append(corpus.QRELs, QREL{QueryID: questionID, EvidenceID: evidenceID, Grade: 2, Role: "answer-session", GroupID: groupID, Expectation: "active"})
			}
		}
		for sessionID := range obsolete {
			for _, evidenceID := range sessionEvents[sessionID] {
				corpus.QRELs = append(corpus.QRELs, QREL{QueryID: questionID, EvidenceID: evidenceID, Grade: 0, Role: "obsolete", Expectation: "forgotten"})
				query.MustNotReturnIDs = append(query.MustNotReturnIDs, evidenceID)
			}
		}
		for sessionID := range conflicts {
			for _, evidenceID := range sessionEvents[sessionID] {
				query.MustNotReturnIDs = append(query.MustNotReturnIDs, evidenceID)
			}
		}
		sort.Strings(query.MustNotReturnIDs)
		corpus.Queries = append(corpus.Queries, query)
	}
	if err := corpus.Validate(); err != nil {
		return NormalizedCorpus{}, err
	}
	return corpus.Canonical(), nil
}

func longMemEvalID(questionID, sessionID string) string {
	return "longmemeval:" + questionID + ":" + sessionID
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}

func isProfileQuestion(questionType string) bool {
	value := strings.ToLower(questionType)
	return strings.Contains(value, "profile") || strings.Contains(value, "preference") || strings.Contains(value, "persona")
}

func normalizedLongMemEvalQuestionType(questionType string) string {
	if value := strings.TrimSpace(strings.ToLower(questionType)); value != "" {
		return value
	}
	return "retrieval"
}

func normalizedLongMemEvalUpdateType(questionType string) string {
	value := strings.ToLower(questionType)
	if strings.Contains(value, "update") || strings.Contains(value, "conflict") {
		return "update"
	}
	return ""
}

func longMemEvalSessions(sample LongMemEvalSample) ([]LongMemEvalSession, error) {
	if len(sample.Sessions) > 0 {
		return append([]LongMemEvalSession(nil), sample.Sessions...), nil
	}
	if len(sample.HaystackSessions) == 0 {
		return nil, fmt.Errorf("sessions are required")
	}
	if len(sample.HaystackSessionIDs) != len(sample.HaystackSessions) {
		return nil, fmt.Errorf("haystack session ids do not match sessions")
	}
	if len(sample.HaystackDates) != 0 && len(sample.HaystackDates) != len(sample.HaystackSessions) {
		return nil, fmt.Errorf("haystack dates do not match sessions")
	}
	result := make([]LongMemEvalSession, 0, len(sample.HaystackSessions))
	for index, turns := range sample.HaystackSessions {
		date := ""
		if len(sample.HaystackDates) > 0 {
			date = sample.HaystackDates[index]
		}
		result = append(result, LongMemEvalSession{SessionID: sample.HaystackSessionIDs[index], Date: date, Turns: turns})
	}
	return result, nil
}
