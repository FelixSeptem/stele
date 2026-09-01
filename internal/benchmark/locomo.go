package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

type LoCoMoDataset struct {
	Samples []LoCoMoSample `json:"samples"`
}

type LoCoMoSample struct {
	ID        string           `json:"id"`
	Sessions  []LoCoMoSession  `json:"sessions"`
	Questions []LoCoMoQuestion `json:"questions"`
}

type LoCoMoSession struct {
	ID    string       `json:"id"`
	Turns []LoCoMoTurn `json:"turns"`
}

type LoCoMoTurn struct {
	ID        string `json:"id"`
	Speaker   string `json:"speaker,omitempty"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp,omitempty"`
}

type LoCoMoQuestion struct {
	ID               string   `json:"id"`
	Text             string   `json:"text"`
	Category         string   `json:"category,omitempty"`
	EvidenceTurnIDs  []string `json:"evidence_turn_ids"`
	MustNotReturnIDs []string `json:"must_not_return_ids,omitempty"`
}

type LoCoMoAdapter struct{}

func NewLoCoMoAdapter() LoCoMoAdapter { return LoCoMoAdapter{} }

func LoadLoCoMoDataset(source io.Reader) (LoCoMoDataset, error) {
	if source == nil {
		return LoCoMoDataset{}, fmt.Errorf("locomo source is required")
	}
	decoder := json.NewDecoder(source)
	decoder.DisallowUnknownFields()
	var dataset LoCoMoDataset
	if err := decoder.Decode(&dataset); err != nil {
		return LoCoMoDataset{}, fmt.Errorf("decode locomo dataset: %w", err)
	}
	if len(dataset.Samples) == 0 {
		return LoCoMoDataset{}, fmt.Errorf("locomo dataset has no samples")
	}
	return dataset, nil
}

func LoadLoCoMoDatasetFromBytes(source []byte) (LoCoMoDataset, error) {
	return LoadLoCoMoDataset(bytes.NewReader(source))
}

func (LoCoMoAdapter) Normalize(dataset LoCoMoDataset, scope memory.Scope) (NormalizedCorpus, error) {
	if err := scope.Validate(); err != nil {
		return NormalizedCorpus{}, fmt.Errorf("validate benchmark scope: %w", err)
	}
	corpus := NormalizedCorpus{SchemaVersion: SchemaVersion}
	for _, sample := range dataset.Samples {
		if strings.TrimSpace(sample.ID) == "" {
			return NormalizedCorpus{}, fmt.Errorf("locomo sample id is required")
		}
		turnEvents := make(map[string]string)
		for _, session := range sample.Sessions {
			if strings.TrimSpace(session.ID) == "" {
				return NormalizedCorpus{}, fmt.Errorf("locomo sample %s has session without id", sample.ID)
			}
			conversationID := sample.ID + "/" + session.ID
			conversation := ConversationRecord{ID: conversationID, Scope: scope, Turns: make([]ConversationTurn, 0, len(session.Turns)), Provenance: map[string]string{"dataset": "locomo", "sample_id": sample.ID, "session_id": session.ID}}
			for _, turn := range session.Turns {
				if strings.TrimSpace(turn.ID) == "" || strings.TrimSpace(turn.Text) == "" {
					return NormalizedCorpus{}, fmt.Errorf("locomo session %s has malformed turn", conversationID)
				}
				if _, exists := turnEvents[turn.ID]; exists {
					return NormalizedCorpus{}, fmt.Errorf("locomo sample %s turn id %q is not unique", sample.ID, turn.ID)
				}
				source := sample.ID + "/" + session.ID + "/" + turn.ID
				conversation.Turns = append(conversation.Turns, ConversationTurn{ID: turn.ID, Speaker: turn.Speaker, Text: turn.Text, Timestamp: turn.Timestamp, Source: source})
				eventID := "event/" + source
				turnEvents[turn.ID] = eventID
				corpus.Events = append(corpus.Events, MemoryEventRecord{ID: eventID, Scope: scope, SessionID: conversationID, SourceTurnID: turn.ID, Class: loCoMoMemoryClass(turn), Text: turn.Text, ObservedAt: turn.Timestamp, ExpectedState: memory.MemoryStateActive, Provenance: map[string]string{"dataset": "locomo", "source": source}})
			}
			corpus.Conversations = append(corpus.Conversations, conversation)
		}
		for _, question := range sample.Questions {
			if strings.TrimSpace(question.ID) == "" || strings.TrimSpace(question.Text) == "" {
				return NormalizedCorpus{}, fmt.Errorf("locomo sample %s has malformed question", sample.ID)
			}
			if len(question.EvidenceTurnIDs) == 0 {
				return NormalizedCorpus{}, fmt.Errorf("locomo question %s has no evidence", question.ID)
			}
			evidenceIDs := make([]string, 0, len(question.EvidenceTurnIDs))
			for _, turnID := range question.EvidenceTurnIDs {
				eventID, ok := turnEvents[turnID]
				if !ok {
					return NormalizedCorpus{}, fmt.Errorf("locomo question %s references unmapped evidence turn %s", question.ID, turnID)
				}
				evidenceIDs = append(evidenceIDs, eventID)
			}
			sort.Strings(evidenceIDs)
			queryID := "query/" + sample.ID + "/" + question.ID
			groupID := queryID + "/evidence"
			query := BenchmarkQuery{ID: queryID, Scope: scope, SessionID: sample.ID, Text: question.Text, QueryType: normalizedQueryType(question.Category), EvidenceGroups: []EvidenceGroup{{ID: groupID, EvidenceIDs: evidenceIDs, Required: true}}, MustNotReturnIDs: append([]string(nil), question.MustNotReturnIDs...)}
			corpus.Queries = append(corpus.Queries, query)
			for _, evidenceID := range evidenceIDs {
				corpus.QRELs = append(corpus.QRELs, QREL{QueryID: queryID, EvidenceID: evidenceID, Grade: 1, Role: "supporting", GroupID: groupID, Expectation: "active"})
			}
		}
	}
	if err := corpus.Validate(); err != nil {
		return NormalizedCorpus{}, err
	}
	return corpus, nil
}

func loCoMoMemoryClass(turn LoCoMoTurn) memory.MemoryClass {
	text := strings.ToLower(turn.Text)
	if strings.Contains(text, "my favorite") || strings.Contains(text, "i prefer") || strings.Contains(text, "i work") {
		return memory.MemoryClassProfile
	}
	return memory.MemoryClassEpisodic
}

func normalizedQueryType(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		return "fact"
	}
	return category
}
