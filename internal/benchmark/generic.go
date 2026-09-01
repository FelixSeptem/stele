package benchmark

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

type GenericRetrievalInput struct {
	Documents []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"documents"`
	Queries []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"queries"`
	QRELs []QREL `json:"qrels"`
}

// NormalizeGenericRetrieval converts a small BEIR/MTEB-compatible JSON input
// into the shared intermediate schema while preserving a generic family tag.
func NormalizeGenericRetrieval(scope memory.Scope, source []byte) (NormalizedCorpus, error) {
	if err := scope.Validate(); err != nil {
		return NormalizedCorpus{}, err
	}
	var input GenericRetrievalInput
	if err := json.Unmarshal(source, &input); err != nil {
		return NormalizedCorpus{}, fmt.Errorf("decode generic retrieval input: %w", err)
	}
	corpus := NormalizedCorpus{SchemaVersion: SchemaVersion}
	for _, doc := range input.Documents {
		id := strings.TrimSpace(doc.ID)
		text := strings.TrimSpace(doc.Text)
		if id == "" || text == "" {
			return NormalizedCorpus{}, fmt.Errorf("generic document id and text are required")
		}
		corpus.Events = append(corpus.Events, MemoryEventRecord{ID: id, Scope: scope, Class: memory.MemoryClassEpisodic, Text: text, ExpectedState: memory.MemoryStateActive, Provenance: map[string]string{"family": string(FamilyGenericRetrieval)}})
	}
	for _, query := range input.Queries {
		id := strings.TrimSpace(query.ID)
		text := strings.TrimSpace(query.Text)
		if id == "" || text == "" {
			return NormalizedCorpus{}, fmt.Errorf("generic query id and text are required")
		}
		corpus.Queries = append(corpus.Queries, BenchmarkQuery{ID: id, Scope: scope, Text: text, QueryType: string(FamilyGenericRetrieval)})
	}
	corpus.QRELs = append(corpus.QRELs, input.QRELs...)
	for i := range corpus.Queries {
		for _, qrel := range corpus.QRELs {
			if qrel.QueryID == corpus.Queries[i].ID {
				groupID := qrel.GroupID
				if groupID == "" {
					groupID = "generic-" + qrel.QueryID
				}
				corpus.Queries[i].EvidenceGroups = append(corpus.Queries[i].EvidenceGroups, EvidenceGroup{ID: groupID, EvidenceIDs: []string{qrel.EvidenceID}, Required: true})
			}
		}
	}
	if err := corpus.Validate(); err != nil {
		return NormalizedCorpus{}, err
	}
	return corpus.Canonical(), nil
}
