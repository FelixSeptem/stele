package provider

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/FelixSeptem/stele/openapi"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestContextResponseAllSectionsAreSafeAndConformant(t *testing.T) {
	now := time.Now().UTC()
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	hit := retrieval.SearchHit{Memory: memory.CanonicalMemory{ID: "memory-a", Scope: scope, MemoryPath: "tasks/a", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "selected content", CreatedAt: now, ModifiedAt: now}, Score: retrieval.ScoreBreakdown{Overall: 0.987654, Lexical: 0.123456}, Citations: []retrieval.Citation{{MemoryID: "memory-a", RawEventID: "event-a", Operation: "promote"}}}
	hit.Memory.SourceMetadata = memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}
	hit.Memory.SourceVersion = 1
	insight := retrieval.ExperienceInsightContext{Insight: memory.DerivedInsight{ID: "insight-a", Scope: scope, Type: memory.DerivedInsightType("failure_pattern"), State: memory.DerivedInsightState("active"), Title: "title", Summary: "summary", Payload: map[string]any{"secret": "raw-payload"}, Derivation: memory.DerivedInsightDerivation{Metadata: map[string]any{"secret": "raw-derivation"}}, CreatedAt: now, UpdatedAt: now, LastObservedAt: now}, Citations: []retrieval.InsightCitation{{InsightID: "insight-a", EvidenceID: "event-a", EvidenceKind: "raw_event", Relation: "supports"}}}
	insight.Insight.Confidence = memory.DerivedInsightConfidence{Score: 0.8, Method: "evidence"}
	insight.Insight.Lesson = &memory.DerivedInsightLesson{SourceFailurePatternID: "pattern-a", Guidance: "public guidance", Avoid: []string{"avoid"}, Prefer: []string{"prefer"}}
	out := retrieval.AssembledContext{Profile: []retrieval.SearchHit{hit}, RecentSession: []retrieval.SearchHit{hit}, RecentEpisodes: []retrieval.SearchHit{hit}, RelevantSummaries: []retrieval.SearchHit{hit}, RelatedEntities: []retrieval.SearchHit{hit}, Citations: hit.Citations, KnownFailures: []retrieval.ExperienceInsightContext{insight}, ExperienceLessons: []retrieval.ExperienceInsightContext{insight}, GoalContext: []retrieval.GoalContextItem{{Title: "goal", Summary: "summary", State: "active", ReviewState: "review_approved", PolicyVersion: "v1"}}, Diagnostics: []retrieval.ContextDiagnostic{{Section: "profile", Status: "included", Included: 1, PolicyVersion: "private-policy", CandidateCount: 987, RolloutStage: "private-stage"}}}
	meta := OperationMetadata{RequestID: "request", OperationID: "operation", SchemaVersion: "provider-v1"}
	decision, err := (auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileContent}, DefaultProfile: auth.DisclosureProfileContent}).Resolve("", auth.ResponseModeBoundedContent)
	if err != nil {
		t.Fatal(err)
	}
	result := ShapeDisclosedContextResponse(out, meta, decision, 1, scope)
	admission, err := NewContextAdmission(ContextInput{Budget: 5}, Discover(CapabilityInput{}).Limits, scope, decision, meta)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"profile", "recent_session", "recent_episodes", "relevant_summaries", "related_entities"} {
		if !admission.AdmitContextItem(s, hit) {
			t.Fatal("ample budget refused item")
		}
	}
	data, err := admission.Finalize(out, meta)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"0.987654", "0.123456", "raw-payload", "raw-derivation", "private-policy", "private-stage", "candidate_count", "feedback_summary", "derivation", "payload"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("leaked %s", secret)
		}
	}
	doc, err := openapi3.NewLoader().LoadFromData([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err = json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	if err = doc.Components.Schemas["ProviderContextResponse"].Value.VisitJSON(value); err != nil {
		t.Fatal(err)
	}
	if len(result.Citations) > 1 || len(result.Result.Profile[0].Citations) != 1 || result.Result.Profile[0].Memory.MemoryID != "memory-a" {
		t.Fatal("citation limit altered inner selection")
	}
	if len(result.Result.Citations) != 1 || result.Result.Citations[0].Reference != "memory-a" || result.Result.Citations[0].SourceKind != "canonical_memory" {
		t.Fatal("selected evidence was not preserved and deduplicated")
	}
	for _, marker := range []string{"public guidance", "raw-event-a", `"goal_context"`, `"known_failures"`, `"experience_lessons"`, `"diagnostics"`} {
		if strings.Contains(string(data), marker) {
			t.Fatalf("unverified section leaked: %s", marker)
		}
	}
}

func TestContextRequestStrictObjectsAndScalars(t *testing.T) {
	metadata := `"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1"}`
	for _, body := range []string{`{"Metadata":{},"input":{}}`, `{` + metadata + `,"input":{"query":"q","budget":1,"budget":2}}`, `{` + metadata + `,"input":{"query":"q","budget":1.5}}`, `{` + metadata + `,"input":{"query":"q","budget":1,"include_goal_context":"true"}}`, `{` + metadata + `,"input":{"query":"q","budget":1,"feedback_ranking_policy":null}}`} {
		if _, err := DecodeContextRequest([]byte(body)); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}
