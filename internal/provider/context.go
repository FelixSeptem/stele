package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

// ContextRequest is the public provider-v1 boundary, independent of service inputs.
type ContextRequest struct {
	Metadata OperationMetadata `json:"metadata"`
	Input    ContextInput      `json:"input"`
}
type ContextInput struct {
	Query                      string `json:"query"`
	Budget                     int    `json:"budget"`
	Path                       string `json:"path,omitempty"`
	PathPrefix                 string `json:"path_prefix,omitempty"`
	IncludeRelations           bool   `json:"include_relations,omitempty"`
	IncludeExperienceInsights  bool   `json:"include_experience_insights,omitempty"`
	IncludeGoalContext         bool   `json:"include_goal_context,omitempty"`
	IncludeDiagnostics         bool   `json:"include_diagnostics,omitempty"`
	IncludeFeedbackDiagnostics bool   `json:"include_feedback_diagnostics,omitempty"`
	FeedbackAwareRanking       bool   `json:"feedback_aware_ranking,omitempty"`
	FeedbackRankingPolicy      string `json:"feedback_ranking_policy,omitempty"`
}

// canonicalObject also rejects duplicate keys, null values, and Go's case aliases.
func canonicalObject(data []byte, allowed, required []string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	tok, err := d.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("object required")
	}
	keys := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		keys[key] = true
	}
	out := make(map[string]json.RawMessage)
	for d.More() {
		tok, err = d.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok || !keys[key] || out[key] != nil {
			return nil, fmt.Errorf("noncanonical field")
		}
		var raw json.RawMessage
		if err = d.Decode(&raw); err != nil {
			return nil, err
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, fmt.Errorf("null field")
		}
		out[key] = raw
	}
	if _, err = d.Token(); err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON")
	}
	for _, key := range required {
		if out[key] == nil {
			return nil, fmt.Errorf("required field")
		}
	}
	return out, nil
}

func DecodeContextRequest(data []byte) (ContextRequest, error) {
	var out ContextRequest
	obj, err := canonicalObject(data, []string{"metadata", "input"}, []string{"metadata", "input"})
	if err != nil {
		return out, err
	}
	_, err = canonicalObject(obj["metadata"], []string{"request_id", "operation_id", "idempotency_key", "event_seq", "schema_version", "memory_path", "precedence_version", "precedence_stage", "precedence_outcome"}, []string{"request_id", "operation_id", "schema_version"})
	if err != nil {
		return out, err
	}
	_, err = canonicalObject(obj["input"], []string{"query", "budget", "path", "path_prefix", "include_relations", "include_experience_insights", "include_goal_context", "include_diagnostics", "include_feedback_diagnostics", "feedback_aware_ranking", "feedback_ranking_policy"}, []string{"query", "budget"})
	if err != nil {
		return out, err
	}
	if err = DecodeStrict(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func (i ContextInput) RetrievalInput(scope memory.Scope) (retrieval.AssembleContextInput, error) {
	out := retrieval.AssembleContextInput{Scope: scope, Query: i.Query, Budget: i.Budget, IncludeRelations: i.IncludeRelations, IncludeExperienceInsights: i.IncludeExperienceInsights, IncludeGoalContext: i.IncludeGoalContext, IncludeDiagnostics: i.IncludeDiagnostics, IncludeFeedbackDiagnostics: i.IncludeFeedbackDiagnostics, FeedbackAwareRanking: i.FeedbackAwareRanking}
	if i.FeedbackRankingPolicy != "" {
		return out, fmt.Errorf("feedback ranking policy is unsupported")
	}
	selector, err := memory.NewMemoryPathSelector(i.Path, i.PathPrefix)
	if err != nil {
		return out, err
	}
	switch selector.Kind {
	case memory.MemoryPathSelectorExact:
		out.Path = selector.Value
	case memory.MemoryPathSelectorPrefix:
		out.PathPrefix = selector.Value
	}
	return out, out.Validate()
}

type ContextResponse struct {
	Metadata  OperationMetadata `json:"metadata"`
	Result    ContextResult     `json:"result"`
	Citations []Citation        `json:"citations"`
}
type ContextMemory struct {
	ID             string                        `json:"id"`
	Scope          memory.Scope                  `json:"scope"`
	MemoryPath     string                        `json:"memory_path"`
	Class          memory.MemoryClass            `json:"class"`
	State          memory.MemoryState            `json:"state"`
	Content        string                        `json:"content"`
	CreatedAt      time.Time                     `json:"created_at"`
	ModifiedAt     time.Time                     `json:"modified_at"`
	TemporalFactID string                        `json:"temporal_fact_id"`
	IngestedAt     time.Time                     `json:"ingested_at"`
	ValidFrom      time.Time                     `json:"valid_from"`
	ValidTo        *time.Time                    `json:"valid_to,omitempty"`
	ValiditySource memory.TemporalValiditySource `json:"validity_source"`
}
type ContextItem struct {
	Memory    ContextMemory     `json:"memory"`
	Citations []ContextCitation `json:"citations"`
}
type ContextCitation struct {
	MemoryID   string `json:"memory_id"`
	RawEventID string `json:"raw_event_id"`
	Operation  string `json:"operation"`
}
type ContextConfidence struct {
	Score  float64 `json:"score"`
	Method string  `json:"method,omitempty"`
}
type ContextLesson struct {
	SourceFailurePatternID string   `json:"source_failure_pattern_id"`
	Guidance               string   `json:"guidance"`
	Avoid                  []string `json:"avoid,omitempty"`
	Prefer                 []string `json:"prefer,omitempty"`
}
type ContextInsight struct {
	ID             string                     `json:"id"`
	Scope          memory.Scope               `json:"scope"`
	Type           memory.DerivedInsightType  `json:"type"`
	State          memory.DerivedInsightState `json:"state"`
	Title          string                     `json:"title"`
	Summary        string                     `json:"summary"`
	Confidence     ContextConfidence          `json:"confidence"`
	Lesson         *ContextLesson             `json:"lesson,omitempty"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
	LastObservedAt time.Time                  `json:"last_observed_at"`
}
type ContextInsightItem struct {
	Insight   ContextInsight           `json:"insight"`
	Citations []ContextInsightCitation `json:"citations"`
}
type ContextInsightCitation struct {
	InsightID    string `json:"insight_id"`
	EvidenceKind string `json:"evidence_kind"`
	EvidenceID   string `json:"evidence_id"`
	Relation     string `json:"relation"`
}
type ContextGoal struct {
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	State         string `json:"state"`
	ReviewState   string `json:"review_state"`
	PolicyVersion string `json:"policy_version"`
}
type ContextDiagnostic struct {
	Section     string                    `json:"section"`
	InsightType memory.DerivedInsightType `json:"insight_type,omitempty"`
	Status      string                    `json:"status"`
	Reason      string                    `json:"reason,omitempty"`
	Available   int                       `json:"available,omitempty"`
	Included    int                       `json:"included,omitempty"`
	Omitted     int                       `json:"omitted,omitempty"`
	Hidden      int                       `json:"hidden,omitempty"`
}
type ContextResult struct {
	Profile           []ContextItem        `json:"profile"`
	RecentSession     []ContextItem        `json:"recent_session"`
	RecentEpisodes    []ContextItem        `json:"recent_episodes"`
	RelevantSummaries []ContextItem        `json:"relevant_summaries"`
	RelatedEntities   []ContextItem        `json:"related_entities"`
	Citations         []ContextCitation    `json:"citations"`
	KnownFailures     []ContextInsightItem `json:"known_failures,omitempty"`
	ExperienceLessons []ContextInsightItem `json:"experience_lessons,omitempty"`
	GoalContext       []ContextGoal        `json:"goal_context,omitempty"`
	Diagnostics       []ContextDiagnostic  `json:"diagnostics,omitempty"`
}

func contextItems(hits []retrieval.SearchHit) []ContextItem {
	out := make([]ContextItem, 0, len(hits))
	for _, hit := range hits {
		m := hit.Memory
		citations := make([]ContextCitation, 0, len(hit.Citations))
		for _, c := range hit.Citations {
			citations = append(citations, ContextCitation{MemoryID: c.MemoryID, RawEventID: c.RawEventID, Operation: c.Operation})
		}
		out = append(out, ContextItem{Memory: ContextMemory{ID: m.ID, Scope: m.Scope, MemoryPath: m.MemoryPath, Class: m.Class, State: m.State, Content: m.Content, CreatedAt: m.CreatedAt, ModifiedAt: m.ModifiedAt, TemporalFactID: m.TemporalFactID, IngestedAt: m.IngestedAt, ValidFrom: m.ValidFrom, ValidTo: m.ValidTo, ValiditySource: m.ValiditySource}, Citations: citations})
	}
	return out
}
func contextInsights(items []retrieval.ExperienceInsightContext) []ContextInsightItem {
	out := make([]ContextInsightItem, 0, len(items))
	for _, item := range items {
		i := item.Insight
		var lesson *ContextLesson
		if i.Lesson != nil {
			lesson = &ContextLesson{SourceFailurePatternID: i.Lesson.SourceFailurePatternID, Guidance: i.Lesson.Guidance, Avoid: append([]string(nil), i.Lesson.Avoid...), Prefer: append([]string(nil), i.Lesson.Prefer...)}
		}
		citations := make([]ContextInsightCitation, 0, len(item.Citations))
		for _, c := range item.Citations {
			citations = append(citations, ContextInsightCitation{InsightID: c.InsightID, EvidenceKind: c.EvidenceKind, EvidenceID: c.EvidenceID, Relation: c.Relation})
		}
		out = append(out, ContextInsightItem{Insight: ContextInsight{ID: i.ID, Scope: i.Scope, Type: i.Type, State: i.State, Title: i.Title, Summary: i.Summary, Confidence: ContextConfidence{Score: i.Confidence.Score, Method: i.Confidence.Method}, Lesson: lesson, CreatedAt: i.CreatedAt, UpdatedAt: i.UpdatedAt, LastObservedAt: i.LastObservedAt}, Citations: citations})
	}
	return out
}

// ShapeContextResponse preserves selection/order; it never reranks or truncates items.
// Optional sections remain subject to the assembler's existing visibility gates.
func ShapeContextResponse(out retrieval.AssembledContext, meta OperationMetadata, input ContextInput, citationLimit int) ContextResponse {
	result := ContextResult{Profile: contextItems(out.Profile), RecentSession: contextItems(out.RecentSession), RecentEpisodes: contextItems(out.RecentEpisodes), RelevantSummaries: contextItems(out.RelevantSummaries), RelatedEntities: contextItems(out.RelatedEntities), Citations: []ContextCitation{}}
	// The assembler's aggregate includes pre-budget candidates. Public evidence
	// is projected only from the items the assembler selected for this response.
	seen := make(map[ContextCitation]struct{})
	for _, section := range [][]ContextItem{result.Profile, result.RecentSession, result.RecentEpisodes, result.RelevantSummaries, result.RelatedEntities} {
		for _, item := range section {
			for _, c := range item.Citations {
				if _, exists := seen[c]; !exists {
					seen[c] = struct{}{}
					result.Citations = append(result.Citations, c)
				}
			}
		}
	}
	if input.IncludeExperienceInsights {
		result.KnownFailures = contextInsights(out.KnownFailures)
		result.ExperienceLessons = contextInsights(out.ExperienceLessons)
	}
	if input.IncludeGoalContext {
		for _, g := range out.GoalContext {
			result.GoalContext = append(result.GoalContext, ContextGoal{Title: g.Title, Summary: g.Summary, State: g.State, ReviewState: g.ReviewState, PolicyVersion: g.PolicyVersion})
		}
	}
	if input.IncludeDiagnostics || input.IncludeFeedbackDiagnostics {
		for _, d := range out.Diagnostics {
			result.Diagnostics = append(result.Diagnostics, ContextDiagnostic{Section: d.Section, InsightType: d.InsightType, Status: d.Status, Reason: d.Reason, Available: d.Available, Included: d.Included, Omitted: d.Omitted, Hidden: d.Hidden})
		}
	}
	return ContextResponse{Metadata: meta, Result: result, Citations: append([]Citation{}, ShapeContextCitationsWithLimit(out, citationLimit)...)}
}
