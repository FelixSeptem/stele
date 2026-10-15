package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/assurance"
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

type referenceHTTPStore struct {
	calls       int
	status      string
	err         error
	mode        string
	memoryState string
	path        string
	referenceID string
	principalID string
	scope       memory.Scope
}

func referenceHTTPRecord() provider.ReferenceRecord {
	return provider.ReferenceRecord{ReferenceID: "task-one", MemoryPath: "tasks/one", EventType: "provider.reference.task", EventID: "0192a1a2-01f1-4000-8000-000000000001", CandidateID: "22222222-2222-4222-8222-222222222222", MemoryID: "33333333-3333-4333-8333-333333333333", VersionID: "44444444-4444-4444-8444-444444444444", ProvenanceID: "55555555-5555-4555-8555-555555555555", Version: 2, Value: "durable evidence 中文🙂"}
}

func (s *referenceHTTPStore) ReferenceStatus(_ context.Context, scope memory.Scope, principal, _, _ string, mode string) (provider.ReferenceResponse, error) {
	s.calls++
	s.mode = mode
	s.scope, s.principalID = scope, principal
	if s.memoryState == "suppressed" || s.memoryState == "forgotten" || s.memoryState == "deleted" {
		return provider.ReferenceResponse{Status: "unavailable"}, s.err
	}
	out := provider.ReferenceResponse{Status: s.status}
	switch s.status {
	case "completed":
		record := referenceHTTPRecord()
		// Deliberately include content so the public boundary must remove it
		// when the negotiated response mode is reference_only.
		out.Reference = &record
	case "pending":
		out.EventID, out.Retryable, out.RetryAfterMS = referenceHTTPRecord().EventID, true, 1000
	case "failed":
		out.EventID, out.ErrorCategory = referenceHTTPRecord().EventID, "retryable"
	}
	return out, s.err
}
func (s *referenceHTTPStore) ReferenceLookup(c context.Context, scope memory.Scope, p, id, path, profile, mode string) (provider.ReferenceResponse, error) {
	s.path, s.referenceID = path, id
	out, err := s.ReferenceStatus(c, scope, p, id, profile, mode)
	if id != "task-one" || path != "tasks/one" {
		return provider.ReferenceResponse{Status: "unavailable"}, err
	}
	return out, err
}

type referenceHTTPIngest struct {
	calls  int
	err    error
	replay bool
}

func (s *referenceHTTPIngest) IngestIdempotent(_ context.Context, _ memory.IngestEventInput, _, _ string) (memory.IdempotentEventIngestResult, error) {
	s.calls++
	return memory.IdempotentEventIngestResult{Event: memory.RawEvent{ID: "0192a1a2-01f1-4000-8000-000000000001"}, Replayed: s.replay}, s.err
}

type referenceHTTPAuthorizer struct{ providerLifecycleAuthorizer }

func (referenceHTTPAuthorizer) AuthorizeScopeDisclosure(context.Context, string, memory.Scope) (auth.DisclosureEntitlements, error) {
	return auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileReference, auth.DisclosureProfileContent}, DefaultProfile: auth.DisclosureProfileReference}, nil
}

func referenceHTTPHandler(t *testing.T, s *referenceHTTPStore, i *referenceHTTPIngest, mutate ...func(*provider.RuntimeBinding)) func(string, string, http.Header) *httptest.ResponseRecorder {
	t.Helper()
	now := time.Now()
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	p := auth.Principal{ID: "principal-context", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	b := provider.RuntimeBinding{BindingID: "binding-context", PrincipalID: p.ID, Scope: scope, AgentID: "agent-a", SessionID: "session-a", ProviderInstanceID: "pi-a", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	for _, apply := range mutate {
		apply(&b)
	}
	scope = b.Scope
	store := provider.NewMemoryBindingStore()
	if err := store.Create(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	h := NewHTTPHandler(HTTPDependencies{ProviderEnabled: true, ProviderReferences: s, ProviderAdapter: provider.NewAdapter(provider.AdapterDependencies{ReferenceStore: s, IdempotentIngestor: i}), ProviderBindings: store, ProviderSchemaVersions: []string{"provider-v1"}, PrincipalAuthorizer: referenceHTTPAuthorizer{providerLifecycleAuthorizer{principals: map[string]auth.Principal{"context-key": p}}}})
	return func(path, body string, headers http.Header) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set("X-API-Key", "context-key")
		r.Header.Set(provider.HeaderRuntimeBinding, b.BindingID)
		r.Header.Set(provider.HeaderRuntimeSession, b.SessionID)
		r.Header.Set(auth.HeaderTenant, scope.Tenant)
		r.Header.Set(auth.HeaderProject, scope.Project)
		r.Header.Set(auth.HeaderNamespace, scope.Namespace)
		for k, v := range headers {
			r.Header[k] = v
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
}

func TestProviderReferenceRejectsUnsupportedBoundScopeBeforeDispatch(t *testing.T) {
	write := `{"metadata":{"request_id":"r","operation_id":"o","idempotency_key":"k","schema_version":"provider-v1"},"input":{"contract_version":"durable-reference-v1","reference_id":"ref","memory_path":"tasks/a","event_type":"provider.reference.task","value":"artifact"}}`
	status := `{` + referenceMetadata + `,"input":{"contract_version":"durable-reference-v1","event_id":"0192a1a2-01f1-4000-8000-000000000001"}}`
	lookup := `{` + referenceMetadata + `,"input":{"contract_version":"durable-reference-v1","reference_id":"task-one","memory_path":"tasks/one"}}`
	for _, tc := range []struct {
		name   string
		mutate func(*provider.RuntimeBinding)
	}{
		{"long tenant", func(b *provider.RuntimeBinding) { b.Scope.Tenant = strings.Repeat("a", 129) }},
		{"escaped namespace", func(b *provider.RuntimeBinding) { b.Scope.Namespace = "scope<name" }},
		{"long agent", func(b *provider.RuntimeBinding) { b.AgentID = strings.Repeat("a", 129) }},
		{"long session", func(b *provider.RuntimeBinding) { b.SessionID = strings.Repeat("a", 129) }},
		{"long conversation", func(b *provider.RuntimeBinding) { b.ConversationID = strings.Repeat("a", 129) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, ingest := &referenceHTTPStore{status: "pending"}, &referenceHTTPIngest{}
			call := referenceHTTPHandler(t, store, ingest, tc.mutate)
			for _, req := range []struct{ path, body string }{{"/v1/provider/references", write}, {"/v1/provider/references/status", status}, {"/v1/provider/references/lookup", lookup}} {
				w := call(req.path, req.body, nil)
				if w.Code != 400 || !strings.Contains(w.Body.String(), `"code":"invalid_reference_scope"`) || ingest.calls != 0 || store.calls != 0 {
					t.Fatalf("unsupported scope dispatched %s: status=%d ingest=%d reads=%d body=%s", req.path, w.Code, ingest.calls, store.calls, w.Body.String())
				}
			}
		})
	}
}

const referenceMetadata = `"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1"}`

func validateReferenceHTTPResponse(t *testing.T, w *httptest.ResponseRecorder, mode string) assurance.ReferenceConsumerResponse {
	t.Helper()
	contract, err := assurance.DefaultProviderReferenceContract()
	if err != nil {
		t.Fatal(err)
	}
	scope := provider.CanonicalScopeDescriptor{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a", AgentID: "agent-a", SessionID: "session-a", ProviderInstanceID: "pi-a"}
	out, err := contract.Validate(w.Body.Bytes(), scope, "provider-v1", mode)
	if err != nil {
		t.Fatalf("serialized reference response violates consumer contract: %v; %s", err, w.Body.String())
	}
	return out
}

func TestProviderReferencePublicStatesAndStrictBoundary(t *testing.T) {
	s := &referenceHTTPStore{status: "pending"}
	i := &referenceHTTPIngest{}
	call := referenceHTTPHandler(t, s, i)
	body := `{` + referenceMetadata + `,"input":{"contract_version":"durable-reference-v1","event_id":"0192a1a2-01f1-4000-8000-000000000001"}}`
	for _, status := range []string{"pending", "failed", "completed", "unavailable"} {
		s.status = status
		w := call("/v1/provider/references/status", body, nil)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", status, w.Code, w.Body.String())
		}
		out := validateReferenceHTTPResponse(t, w, "reference_only")
		if out.Status != status || out.Scope.AgentID != "agent-a" {
			t.Fatalf("bad response %s", w.Body.String())
		}
	}
	for _, bad := range []string{strings.Replace(body, `"event_id"`, `"Event_ID"`, 1), strings.Replace(body, `"event_id":"0192a1a2-01f1-4000-8000-000000000001"`, `"event_id":null`, 1), strings.Replace(body, `"event_id":"0192a1a2-01f1-4000-8000-000000000001"`, `"event_id":"0192a1a2-01f1-4000-8000-000000000001","extra":1`, 1)} {
		before := s.calls
		w := call("/v1/provider/references/status", bad, nil)
		if w.Code != 400 || s.calls != before {
			t.Fatalf("unsafe invalid dispatch %d %s", w.Code, w.Body.String())
		}
	}
	w := call("/v1/provider/references/status", strings.Replace(body, "durable-reference-v1", "future-v1", 1), nil)
	if w.Code != 409 {
		t.Fatalf("version: %d", w.Code)
	}
	before := s.calls
	w = call("/v1/provider/references/status", body, http.Header{auth.HeaderTenant: []string{"foreign"}})
	if w.Code != 403 || s.calls != before {
		t.Fatal("foreign dispatched")
	}
	s.err = errors.New("SQL private secret")
	w = call("/v1/provider/references/status", body, nil)
	if w.Code != 503 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("dependency leaked")
	}
}
func TestProviderReferenceWriteAcceptanceConflictAndLimits(t *testing.T) {
	s := &referenceHTTPStore{}
	i := &referenceHTTPIngest{}
	call := referenceHTTPHandler(t, s, i)
	body := `{"metadata":{"request_id":"r","operation_id":"o","idempotency_key":"k","schema_version":"provider-v1"},"input":{"contract_version":"durable-reference-v1","reference_id":"ref","memory_path":"tasks/a","event_type":"provider.reference.task","value":"artifact"}}`
	w := call("/v1/provider/references", body, nil)
	if w.Code != 202 {
		t.Fatalf("write %d %s", w.Code, w.Body.String())
	}
	accepted := validateReferenceHTTPResponse(t, w, "reference_only")
	if accepted.Status != "accepted" || accepted.EventID != referenceHTTPRecord().EventID || !accepted.Retryable {
		t.Fatalf("invalid accepted response: %+v", accepted)
	}
	i.replay = true
	w = call("/v1/provider/references", body, nil)
	if w.Code != 202 {
		t.Fatalf("replay %d %s", w.Code, w.Body.String())
	}
	replay := validateReferenceHTTPResponse(t, w, "reference_only")
	if !replay.Replayed || replay.EventID != accepted.EventID {
		t.Fatalf("replay changed durable identity: %+v", replay)
	}
	i.err = memory.ErrIdempotencyConflict
	w = call("/v1/provider/references", body, nil)
	if w.Code != 409 {
		t.Fatalf("conflict %d %s", w.Code, w.Body.String())
	}
	before := i.calls
	w = call("/v1/provider/references", strings.Replace(body, "provider.reference.task", "task.completed", 1), nil)
	if w.Code != 400 || i.calls != before {
		t.Fatal("unsupported dispatched")
	}
	w = call("/v1/provider/references", strings.Repeat(" ", 32769), nil)
	if w.Code != 413 {
		t.Fatalf("body limit %d", w.Code)
	}
}

func TestProviderReferenceExactLookupContentAndLifecycleBoundary(t *testing.T) {
	s := &referenceHTTPStore{status: "completed"}
	call := referenceHTTPHandler(t, s, &referenceHTTPIngest{})
	body := `{` + referenceMetadata + `,"input":{"contract_version":"durable-reference-v1","reference_id":"task-one","memory_path":"/tasks/one/"}}`
	w := call("/v1/provider/references/lookup", body, nil)
	if w.Code != 200 {
		t.Fatalf("lookup %d %s", w.Code, w.Body.String())
	}
	out := validateReferenceHTTPResponse(t, w, "reference_only")
	if out.Status != "completed" || out.Reference == nil || out.Reference.Value != nil || out.Reference.MemoryPath != "tasks/one" || out.Reference.Version != 2 || s.path != "tasks/one" || s.referenceID != "task-one" || s.mode != "reference_only" || s.principalID != "principal-context" || s.scope != (memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}) {
		t.Fatalf("exact reference-only lookup lost bound selector or leaked value: %+v", out)
	}
	content := strings.Replace(body, `"memory_path":"/tasks/one/"`, `"memory_path":"tasks/one","disclosure_profile":"memory-content-v1","response_mode":"bounded_content"`, 1)
	w = call("/v1/provider/references/lookup", content, nil)
	if w.Code != 200 {
		t.Fatalf("content lookup %d %s", w.Code, w.Body.String())
	}
	out = validateReferenceHTTPResponse(t, w, "bounded_content")
	if out.Reference == nil || out.Reference.Value == nil || *out.Reference.Value != referenceHTTPRecord().Value {
		t.Fatalf("content lookup lost current value: %+v", out)
	}
	for _, changed := range []string{strings.Replace(body, "/tasks/one/", "tasks/one/descendant", 1), strings.Replace(body, "task-one", "task-other", 1)} {
		w = call("/v1/provider/references/lookup", changed, nil)
		if w.Code != 200 || validateReferenceHTTPResponse(t, w, "reference_only").Status != "unavailable" {
			t.Fatalf("lookup broadened selector: %d %s", w.Code, w.Body.String())
		}
	}
	status := `{` + referenceMetadata + `,"input":{"contract_version":"durable-reference-v1","event_id":"` + referenceHTTPRecord().EventID + `"}}`
	for _, state := range []string{"suppressed", "forgotten", "deleted"} {
		s.memoryState = state
		for _, request := range []struct{ path, body string }{{"/v1/provider/references/status", status}, {"/v1/provider/references/lookup", body}} {
			w = call(request.path, request.body, nil)
			if w.Code != 200 {
				t.Fatalf("%s %s: %d %s", state, request.path, w.Code, w.Body.String())
			}
			out = validateReferenceHTTPResponse(t, w, "reference_only")
			if out.Status != "unavailable" || out.Reference != nil || out.EventID != "" || out.ErrorCategory != "" {
				t.Fatalf("%s existence or lineage leaked: %+v", state, out)
			}
		}
	}
	for _, bad := range []string{strings.Replace(body, `"reference_id"`, `"referenceId"`, 1), strings.Replace(body, `"task-one"`, `null`, 1), strings.Replace(body, `"memory_path":"/tasks/one/"`, `"memory_path":"/tasks/one/","include_history":true`, 1)} {
		before := s.calls
		w = call("/v1/provider/references/lookup", bad, nil)
		if w.Code != 400 || s.calls != before {
			t.Fatalf("invalid lookup dispatched: %d %s", w.Code, w.Body.String())
		}
	}
}
