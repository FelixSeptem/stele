package benchmark

import (
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

// ProfilePreferenceFixture is a license-safe, repository-owned representation
// of cross-session profile and preference updates. It deliberately models
// historical facts rather than overwriting them.
type ProfilePreferenceFixture struct {
	Facts   []ProfilePreferenceFact  `json:"facts"`
	Queries []ProfilePreferenceQuery `json:"queries"`
}

type ProfilePreferenceFact struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	Current   bool   `json:"current"`
}

type ProfilePreferenceQuery struct {
	ID                string   `json:"id"`
	Text              string   `json:"text"`
	FactID            string   `json:"fact_id"`
	Historical        bool     `json:"historical"`
	AllowedSessionIDs []string `json:"allowed_session_ids,omitempty"`
}

type TemporalFixture struct {
	Facts   []TemporalFact  `json:"facts"`
	Queries []TemporalQuery `json:"queries"`
}

type TemporalFact struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	ValidFrom string `json:"valid_from"`
	ValidTo   string `json:"valid_to,omitempty"`
	Current   bool   `json:"current"`
}

type TemporalQuery struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	At     string `json:"at"`
	FactID string `json:"fact_id"`
}

type MultiHopFixture struct {
	Facts   []MultiHopFact  `json:"facts"`
	Queries []MultiHopQuery `json:"queries"`
}

type MultiHopFact struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type MultiHopQuery struct {
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	EvidenceIDs []string `json:"evidence_ids"`
}

func (f MultiHopFixture) Normalize(scope memory.Scope) (NormalizedCorpus, error) {
	if err := scope.Validate(); err != nil {
		return NormalizedCorpus{}, fmt.Errorf("multi-hop fixture scope: %w", err)
	}
	corpus := NormalizedCorpus{SchemaVersion: SchemaVersion}
	facts := make(map[string]struct{}, len(f.Facts))
	for _, fact := range f.Facts {
		if strings.TrimSpace(fact.ID) == "" || strings.TrimSpace(fact.Text) == "" {
			return NormalizedCorpus{}, fmt.Errorf("multi-hop fact id and text are required")
		}
		if _, exists := facts[fact.ID]; exists {
			return NormalizedCorpus{}, fmt.Errorf("duplicate multi-hop fact %q", fact.ID)
		}
		facts[fact.ID] = struct{}{}
		corpus.Events = append(corpus.Events, MemoryEventRecord{ID: fact.ID, Scope: scope, Class: memory.MemoryClassRelation, Text: fact.Text, ExpectedState: memory.MemoryStateActive, Provenance: map[string]string{"subfamily": "multi_hop"}})
	}
	for _, query := range f.Queries {
		if strings.TrimSpace(query.ID) == "" || strings.TrimSpace(query.Text) == "" || len(query.EvidenceIDs) < 2 {
			return NormalizedCorpus{}, fmt.Errorf("multi-hop query id, text, and at least two evidence ids are required")
		}
		for _, id := range query.EvidenceIDs {
			if _, found := facts[id]; !found {
				return NormalizedCorpus{}, fmt.Errorf("multi-hop query %s references unknown fact %s", query.ID, id)
			}
		}
		groupID := query.ID + "/complete-evidence"
		corpus.Queries = append(corpus.Queries, BenchmarkQuery{ID: query.ID, Scope: scope, Text: query.Text, QueryType: "multi_hop", EvidenceGroups: []EvidenceGroup{{ID: groupID, EvidenceIDs: append([]string(nil), query.EvidenceIDs...), Required: true}}})
		for _, id := range query.EvidenceIDs {
			corpus.QRELs = append(corpus.QRELs, QREL{QueryID: query.ID, EvidenceID: id, Grade: 2, Role: "supporting_fact", GroupID: groupID, Expectation: "complete_group"})
		}
	}
	if err := corpus.Validate(); err != nil {
		return NormalizedCorpus{}, err
	}
	return corpus, nil
}

func (f TemporalFixture) Normalize(scope memory.Scope) (NormalizedCorpus, error) {
	if err := scope.Validate(); err != nil {
		return NormalizedCorpus{}, fmt.Errorf("temporal fixture scope: %w", err)
	}
	corpus := NormalizedCorpus{SchemaVersion: SchemaVersion}
	facts := make(map[string]TemporalFact, len(f.Facts))
	stale := make([]string, 0, len(f.Facts))
	for _, fact := range f.Facts {
		if strings.TrimSpace(fact.ID) == "" || strings.TrimSpace(fact.Text) == "" || !validFixtureDate(fact.ValidFrom) || (fact.ValidTo != "" && !validFixtureDate(fact.ValidTo)) {
			return NormalizedCorpus{}, fmt.Errorf("temporal fact id, text, and valid dates are required")
		}
		if _, exists := facts[fact.ID]; exists {
			return NormalizedCorpus{}, fmt.Errorf("duplicate temporal fact %q", fact.ID)
		}
		facts[fact.ID] = fact
		state := memory.MemoryStateActive
		if !fact.Current {
			state = memory.MemoryStateSuppressed
			stale = append(stale, fact.ID)
		}
		corpus.Events = append(corpus.Events, MemoryEventRecord{ID: fact.ID, Scope: scope, Class: memory.MemoryClassProfile, Text: fact.Text, ObservedAt: fact.ValidFrom + "T00:00:00Z", ExpectedState: state, Provenance: map[string]string{"subfamily": "temporal", "valid_from": fact.ValidFrom, "valid_to": fact.ValidTo}})
	}
	for _, query := range f.Queries {
		if strings.TrimSpace(query.ID) == "" || strings.TrimSpace(query.Text) == "" || !validFixtureDate(query.At) {
			return NormalizedCorpus{}, fmt.Errorf("temporal query id, text, and date are required")
		}
		if _, found := facts[query.FactID]; !found {
			return NormalizedCorpus{}, fmt.Errorf("temporal query %s references unknown fact %s", query.ID, query.FactID)
		}
		corpus.Queries = append(corpus.Queries, BenchmarkQuery{ID: query.ID, Scope: scope, Text: query.Text, QueryType: "temporal", MustNotReturnIDs: append([]string(nil), stale...), EvidenceGroups: []EvidenceGroup{{ID: query.ID + "/answer", EvidenceIDs: []string{query.FactID}, Required: true}}, Metadata: map[string]string{"at": query.At}})
		corpus.QRELs = append(corpus.QRELs, QREL{QueryID: query.ID, EvidenceID: query.FactID, Grade: 2, Role: "temporal_fact", GroupID: query.ID + "/answer", Expectation: "date_bounded"})
	}
	if err := corpus.Validate(); err != nil {
		return NormalizedCorpus{}, err
	}
	return corpus, nil
}

func validFixtureDate(value string) bool {
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

func (f ProfilePreferenceFixture) Normalize(scope memory.Scope) (NormalizedCorpus, error) {
	if err := scope.Validate(); err != nil {
		return NormalizedCorpus{}, fmt.Errorf("profile fixture scope: %w", err)
	}
	corpus := NormalizedCorpus{SchemaVersion: SchemaVersion}
	facts := make(map[string]ProfilePreferenceFact, len(f.Facts))
	obsolete := make([]string, 0, len(f.Facts))
	for _, fact := range f.Facts {
		if strings.TrimSpace(fact.ID) == "" || strings.TrimSpace(fact.SessionID) == "" || strings.TrimSpace(fact.Text) == "" {
			return NormalizedCorpus{}, fmt.Errorf("profile fact id, session_id, and text are required")
		}
		if _, exists := facts[fact.ID]; exists {
			return NormalizedCorpus{}, fmt.Errorf("duplicate profile fact %q", fact.ID)
		}
		facts[fact.ID] = fact
		state := memory.MemoryStateActive
		if !fact.Current {
			state = memory.MemoryStateSuppressed
			obsolete = append(obsolete, fact.ID)
		}
		corpus.Events = append(corpus.Events, MemoryEventRecord{ID: fact.ID, Scope: scope, SessionID: fact.SessionID, Class: memory.MemoryClassProfile, Text: fact.Text, ExpectedState: state, Provenance: map[string]string{"subfamily": "profile_preference", "session_id": fact.SessionID, "current": fmt.Sprint(fact.Current)}})
	}
	for _, query := range f.Queries {
		if strings.TrimSpace(query.ID) == "" || strings.TrimSpace(query.Text) == "" {
			return NormalizedCorpus{}, fmt.Errorf("profile query id and text are required")
		}
		if _, found := facts[query.FactID]; !found {
			return NormalizedCorpus{}, fmt.Errorf("profile query %s references unknown fact %s", query.ID, query.FactID)
		}
		queryType := "profile_current"
		mustNotReturn := append([]string(nil), obsolete...)
		if query.Historical {
			queryType = "profile_history"
			mustNotReturn = nil
		}
		allowedSessions := uniqueSorted(query.AllowedSessionIDs)
		for _, sessionID := range allowedSessions {
			if strings.TrimSpace(sessionID) == "" {
				return NormalizedCorpus{}, fmt.Errorf("profile query %s has empty allowed session", query.ID)
			}
		}
		metadata := map[string]string{"allowed_session_ids": strings.Join(allowedSessions, ",")}
		corpus.Queries = append(corpus.Queries, BenchmarkQuery{ID: query.ID, Scope: scope, Text: query.Text, QueryType: queryType, MustNotReturnIDs: mustNotReturn, EvidenceGroups: []EvidenceGroup{{ID: query.ID + "/fact", EvidenceIDs: []string{query.FactID}, Required: true}}, Metadata: metadata})
		corpus.QRELs = append(corpus.QRELs, QREL{QueryID: query.ID, EvidenceID: query.FactID, Grade: 2, Role: "profile_fact", GroupID: query.ID + "/fact", Expectation: queryType})
	}
	if err := corpus.Validate(); err != nil {
		return NormalizedCorpus{}, err
	}
	return corpus, nil
}

// SpecializedFixture is repository-owned or license-reviewed input for the
// profile, temporal, and multi-hop regression suites.
type SpecializedFixture struct {
	SchemaVersion string              `json:"schema_version"`
	Subfamily     string              `json:"subfamily"`
	Events        []MemoryEventRecord `json:"events"`
	Queries       []BenchmarkQuery    `json:"queries"`
	QRELs         []QREL              `json:"qrels"`
}

func (f SpecializedFixture) Normalize() (NormalizedCorpus, error) {
	if f.SchemaVersion != "" && f.SchemaVersion != SchemaVersion {
		return NormalizedCorpus{}, fmt.Errorf("unsupported specialized fixture schema %q", f.SchemaVersion)
	}
	corpus := NormalizedCorpus{SchemaVersion: SchemaVersion, Events: f.Events, Queries: f.Queries, QRELs: f.QRELs}
	if err := corpus.Validate(); err != nil {
		return NormalizedCorpus{}, err
	}
	return corpus, nil
}

type SpecializedMetrics struct {
	ProfileRecall              float64 `json:"profile_recall"`
	PreferenceConsistency      float64 `json:"preference_consistency"`
	HistoricalPreferenceRecall float64 `json:"historical_preference_recall"`
	TemporalUpdatePrecedence   float64 `json:"temporal_update_precedence"`
	StaleFactSuppression       float64 `json:"stale_fact_suppression"`
	EvidenceGroupCompleteness  float64 `json:"evidence_group_completeness"`
	UnmappedEvidenceCount      int     `json:"unmapped_evidence_count"`
	ScopeSafetyFailures        int     `json:"scope_safety_failures"`
	SessionIsolationViolations int     `json:"session_isolation_violations"`
}

type SpecializedReport struct {
	Family    string             `json:"family"`
	Subfamily string             `json:"subfamily"`
	Metrics   SpecializedMetrics `json:"metrics"`
}

func EvaluateSpecialized(corpus NormalizedCorpus, candidates []RetrievedEvidence, duration time.Duration) SpecializedReport {
	return EvaluateSpecializedFor("", corpus, candidates, duration)
}

func EvaluateSpecializedFor(subfamily string, corpus NormalizedCorpus, candidates []RetrievedEvidence, duration time.Duration) SpecializedReport {
	if len(corpus.Queries) == 0 {
		return SpecializedReport{Family: "specialized_retrieval", Subfamily: subfamily}
	}
	return EvaluateSpecializedQueries(subfamily, corpus, map[string][]RetrievedEvidence{corpus.Queries[0].ID: candidates}, duration)
}

// EvaluateSpecializedQueries evaluates each fixture query against its own
// candidate set so current and historical preference assertions cannot mask
// one another in a single aggregate score.
func EvaluateSpecializedQueries(subfamily string, corpus NormalizedCorpus, candidates map[string][]RetrievedEvidence, duration time.Duration) SpecializedReport {
	report := SpecializedReport{Family: "specialized_retrieval", Subfamily: subfamily}
	if len(corpus.Queries) == 0 {
		return report
	}
	var groupTotal, profileTotal, preferenceTotal, historyTotal, temporalTotal, staleTotal float64
	eventSessions := make(map[string]string, len(corpus.Events))
	for _, event := range corpus.Events {
		eventSessions[event.ID] = event.SessionID
	}
	for _, query := range corpus.Queries {
		evaluation := EvaluateQuery(query, corpus.QRELs, candidates[query.ID], duration)
		report.Metrics.EvidenceGroupCompleteness += evaluation.Metrics.GroupHitRate
		report.Metrics.ScopeSafetyFailures += evaluation.SafetyFailures
		allowedSessions := map[string]struct{}{}
		for _, sessionID := range strings.Split(query.Metadata["allowed_session_ids"], ",") {
			if sessionID != "" {
				allowedSessions[sessionID] = struct{}{}
			}
		}
		if len(allowedSessions) > 0 {
			seen := map[string]struct{}{}
			for _, candidate := range candidates[query.ID] {
				if _, duplicate := seen[candidate.EvidenceID]; duplicate {
					continue
				}
				seen[candidate.EvidenceID] = struct{}{}
				if sessionID, known := eventSessions[candidate.EvidenceID]; known {
					if _, allowed := allowedSessions[sessionID]; !allowed {
						report.Metrics.SessionIsolationViolations++
					}
				}
			}
		}
		groupTotal++
		queryType := query.QueryType
		if queryType == "" {
			queryType = subfamily
		}
		switch queryType {
		case "profile_current":
			profileTotal++
			preferenceTotal++
			report.Metrics.ProfileRecall += evaluation.Metrics.RecallAt1
			if evaluation.Metrics.RecallAt1 == 1 && evaluation.Metrics.MustNotReturnViolations == 0 {
				report.Metrics.PreferenceConsistency++
			}
		case "profile_history":
			historyTotal++
			report.Metrics.HistoricalPreferenceRecall += evaluation.Metrics.RecallAt1
		case "temporal", "knowledge-update":
			temporalTotal++
			report.Metrics.TemporalUpdatePrecedence += evaluation.Metrics.RecallAt1
			if evaluation.Metrics.MustNotReturnViolations == 0 {
				staleTotal++
			}
		}
	}
	if groupTotal > 0 {
		report.Metrics.EvidenceGroupCompleteness /= groupTotal
	}
	if profileTotal > 0 {
		report.Metrics.ProfileRecall /= profileTotal
	}
	if preferenceTotal > 0 {
		report.Metrics.PreferenceConsistency /= preferenceTotal
	}
	if historyTotal > 0 {
		report.Metrics.HistoricalPreferenceRecall /= historyTotal
	}
	if temporalTotal > 0 {
		report.Metrics.TemporalUpdatePrecedence /= temporalTotal
		report.Metrics.StaleFactSuppression = staleTotal / temporalTotal
	}
	return report
}
