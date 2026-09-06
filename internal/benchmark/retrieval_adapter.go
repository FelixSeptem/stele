package benchmark

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

type RetrievalEvidenceMapping struct {
	MemoryID   string
	RawEventID string
	Scope      memory.Scope
	State      memory.MemoryState
}

type RetrievalEvaluationMetadata struct {
	FixtureVersion        string
	RepresentationVersion string
	RankingVersion        string
	EmbeddingRevision     string
	LexicalMatchMode      retrieval.LexicalMatchMode
	PolicyVersion         string
}

type PreparedRetrievalEvaluation struct {
	Fixture  retrieval.EvaluationFixture
	Seed     retrieval.EvaluationFixtureSeed
	Metadata retrieval.EvaluationRankingMetadata
}

func BuildRetrievalEvaluationFixture(corpus NormalizedCorpus, mappings map[string]RetrievalEvidenceMapping, metadata RetrievalEvaluationMetadata) (PreparedRetrievalEvaluation, error) {
	if err := corpus.Validate(); err != nil {
		return PreparedRetrievalEvaluation{}, err
	}
	result := PreparedRetrievalEvaluation{
		Fixture:  retrieval.EvaluationFixture{Version: strings.TrimSpace(metadata.FixtureVersion), Cases: make([]retrieval.EvaluationCase, 0, len(corpus.Queries))},
		Seed:     retrieval.EvaluationFixtureSeed{FixtureVersion: strings.TrimSpace(metadata.FixtureVersion)},
		Metadata: retrieval.EvaluationRankingMetadata{FixtureVersion: strings.TrimSpace(metadata.FixtureVersion), RepresentationVersion: strings.TrimSpace(metadata.RepresentationVersion), RankingVersion: strings.TrimSpace(metadata.RankingVersion), CompatibleEmbeddingRevision: strings.TrimSpace(metadata.EmbeddingRevision), LexicalMatchMode: metadata.LexicalMatchMode, PolicyVersion: strings.TrimSpace(metadata.PolicyVersion)},
	}
	if err := result.Metadata.Validate(); err != nil {
		return PreparedRetrievalEvaluation{}, err
	}
	events := append([]MemoryEventRecord(nil), corpus.Events...)
	sort.Slice(events, func(i, j int) bool { return events[i].ID < events[j].ID })
	eventsByQuery := make(map[string][]MemoryEventRecord)
	allEvents := make([]MemoryEventRecord, 0, len(events))
	for _, event := range events {
		questionID := ""
		if event.Provenance != nil {
			questionID = event.Provenance["question_id"]
		}
		if questionID != "" {
			eventsByQuery[questionID] = append(eventsByQuery[questionID], event)
		} else {
			allEvents = append(allEvents, event)
		}
	}
	queries := append([]BenchmarkQuery(nil), corpus.Queries...)
	sort.Slice(queries, func(i, j int) bool { return queries[i].ID < queries[j].ID })
	for _, query := range queries {
		candidateEvents := events
		if strings.TrimSpace(query.SessionID) != "" {
			candidateEvents = eventsByQuery[query.SessionID]
			if len(candidateEvents) == 0 {
				candidateEvents = allEvents
			}
		}
		caseSources := make([]retrieval.EvaluationSource, 0, len(candidateEvents))
		for _, event := range candidateEvents {
			if !eventBelongsToQuerySession(event, query) {
				continue
			}
			mapping, found := mappings[event.ID]
			if !found || strings.TrimSpace(mapping.MemoryID) == "" {
				return PreparedRetrievalEvaluation{}, fmt.Errorf("missing retrieval mapping for benchmark evidence %s", event.ID)
			}
			if mapping.Scope.Normalized() != query.Scope.Normalized() {
				return PreparedRetrievalEvaluation{}, fmt.Errorf("retrieval mapping scope mismatch for benchmark evidence %s", event.ID)
			}
			state := mapping.State
			if state == "" {
				state = memory.MemoryStateActive
			}
			observedAt, err := parseObservedAt(event.ObservedAt)
			if err != nil {
				return PreparedRetrievalEvaluation{}, err
			}
			caseSources = append(caseSources, retrieval.EvaluationSource{Alias: event.ID, EventType: "benchmark." + string(event.Class), Content: event.Text, Class: event.Class, State: state, FactCluster: event.SessionID, SourceTimestamp: observedAt})
			result.Seed.Aliases = append(result.Seed.Aliases, retrieval.EvaluationSeededAlias{CaseID: query.ID, Alias: event.ID, Scope: mapping.Scope, MemoryID: mapping.MemoryID, RawEventID: mapping.RawEventID, State: state, FactCluster: event.SessionID})
		}
		groups := make([][]string, 0, len(query.EvidenceGroups))
		for _, group := range query.EvidenceGroups {
			if !group.Required {
				continue
			}
			aliases := append([]string(nil), group.EvidenceIDs...)
			sort.Strings(aliases)
			groups = append(groups, aliases)
		}
		if len(groups) == 0 {
			return PreparedRetrievalEvaluation{}, fmt.Errorf("benchmark query %s has no required evidence groups", query.ID)
		}
		result.Fixture.Cases = append(result.Fixture.Cases, retrieval.EvaluationCase{ID: query.ID, Category: query.QueryType, Scope: query.Scope, Query: query.Text, Sources: caseSources, ExpectedEvidenceGroups: groups, ExcludedAliases: append([]string(nil), query.MustNotReturnIDs...)})
	}
	if err := result.Fixture.Validate(); err != nil {
		return PreparedRetrievalEvaluation{}, err
	}
	return result, nil
}

func eventBelongsToQuerySession(event MemoryEventRecord, query BenchmarkQuery) bool {
	if strings.TrimSpace(query.SessionID) == "" {
		return true
	}
	if event.Provenance != nil && event.Provenance["question_id"] == query.SessionID {
		return true
	}
	return event.SessionID == query.SessionID || strings.HasPrefix(event.SessionID, query.SessionID+"/")
}
