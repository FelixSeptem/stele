package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestAdminProceduralVerificationRoutesRequireAdminScopeAndNeverUsePublicKey(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	admin := &proceduralVerificationHTTPStub{record: memory.ProceduralMemoryVerification{
		ID: "verification-1", Scope: scope, MemoryID: "memory-1", TargetVersion: 2,
		ClaimFingerprint: strings.Repeat("a", 64), ApplicabilityFingerprint: strings.Repeat("b", 64),
		Disposition: memory.ProceduralVerificationDispositionInconclusive,
		Evidence:    []memory.ProceduralVerificationEvidence{{Scope: scope, Kind: memory.ProceduralVerificationEvidenceToolObservation, SourceKind: memory.ProceduralVerificationEvidenceSourceRawEvent, ID: "event-1", SourceVersion: 1}},
		Actor:       "operator-a", Reason: "inspect procedure", PolicyVersion: "policy-v1", SourceWatermark: "watermark-1", CreatedAt: time.Date(2026, 11, 8, 3, 0, 0, 0, time.UTC),
	}}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness: stubReadinessChecker{}, APIKeys: map[string]struct{}{"public-key": {}}, AdminAPIKeys: map[string]struct{}{"admin-key": {}},
		ProceduralVerificationAdmin: admin, ProceduralVerificationPolicyVersion: "policy-v1",
	})

	public := httptest.NewRequest(http.MethodPost, "/v1/admin/procedural-memory-verifications", strings.NewReader(`{"memory_id":"memory-1","target_version":2,"claim_fingerprint":"`+strings.Repeat("a", 64)+`","applicability_fingerprint":"`+strings.Repeat("b", 64)+`","disposition":"inconclusive","evidence":[{"kind":"tool_observation","source_kind":"raw_event","id":"event-1","source_version":1}],"reason":"inspect procedure","source_watermark":"watermark-1"}`))
	public.Header.Set("X-API-Key", "public-key")
	public.Header.Set("X-Stele-Tenant", scope.Tenant)
	public.Header.Set("X-Stele-Project", scope.Project)
	public.Header.Set("X-Stele-Namespace", scope.Namespace)
	publicResp := httptest.NewRecorder()
	handler.ServeHTTP(publicResp, public)
	if publicResp.Code != http.StatusUnauthorized || admin.recordCalls != 0 {
		t.Fatalf("public verification route status=%d recordCalls=%d, want 401 before lookup", publicResp.Code, admin.recordCalls)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/admin/procedural-memory-verifications", strings.NewReader(`{"memory_id":"memory-1","target_version":2,"claim_fingerprint":"`+strings.Repeat("a", 64)+`","applicability_fingerprint":"`+strings.Repeat("b", 64)+`","disposition":"inconclusive","evidence":[{"kind":"tool_observation","source_kind":"raw_event","id":"event-1","source_version":1}],"reason":"inspect procedure","source_watermark":"watermark-1"}`))
	setAdminActionHeaders(request)
	request.Header.Set("X-Request-ID", "request-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("admin verification create status=%d body=%s, want 201", response.Code, response.Body.String())
	}
	if admin.recordCalls != 1 || admin.record.Scope != scope || admin.record.Actor != "operator-a" || admin.record.PolicyVersion != "policy-v1" {
		t.Fatalf("record input=%+v calls=%d, want scoped server-attributed verification", admin.record, admin.recordCalls)
	}
}

func TestVerificationCorrectionAdmissionHTTPIsScopedAndRedacted(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	admin := &proceduralVerificationHTTPStub{record: memory.ProceduralMemoryVerification{
		ID: "verification-1", Scope: scope, MemoryID: "memory-1", TargetVersion: 2,
		ClaimFingerprint: strings.Repeat("a", 64), ApplicabilityFingerprint: strings.Repeat("b", 64),
		Disposition: memory.ProceduralVerificationDispositionRefuted,
		Evidence:    []memory.ProceduralVerificationEvidence{{Scope: scope, Kind: memory.ProceduralVerificationEvidenceToolObservation, SourceKind: memory.ProceduralVerificationEvidenceSourceRawEvent, ID: "event-sensitive", SourceVersion: 1}},
		Actor:       "operator-a", Reason: "verified error", PolicyVersion: "policy-v1", SourceWatermark: "watermark-1", CorrectedContent: "do the corrected thing", CreatedAt: time.Date(2026, 11, 8, 3, 0, 0, 0, time.UTC),
	}}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness: stubReadinessChecker{}, AdminAPIKeys: map[string]struct{}{"admin-key": {}},
		ProceduralVerificationAdmin: admin, ProceduralVerificationPolicyVersion: "policy-v1",
	})

	public := httptest.NewRequest(http.MethodGet, "/v1/admin/procedural-memory-verifications/verification-1", nil)
	public.Header.Set("X-API-Key", "public-key")
	publicResp := httptest.NewRecorder()
	handler.ServeHTTP(publicResp, public)
	if publicResp.Code != http.StatusUnauthorized || admin.readCalls != 0 {
		t.Fatalf("public inspection status=%d readCalls=%d, want 401 before lookup", publicResp.Code, admin.readCalls)
	}

	for _, target := range []string{
		"/v1/admin/procedural-memory-verifications/verification-1",
		"/v1/admin/procedural-memory-verifications?memory_id=memory-1&target_version=2",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		setAdminActionHeaders(req)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatalf("admin inspection %s status=%d body=%s, want 200", target, response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "event-sensitive") || strings.Contains(response.Body.String(), "do the corrected thing") {
			t.Fatalf("admin inspection %s disclosed raw evidence or correction: %s", target, response.Body.String())
		}
	}

	revalidate := httptest.NewRequest(http.MethodPost, "/v1/admin/procedural-memory-verifications/verification-1:revalidate", strings.NewReader(`{"reason":"review source drift"}`))
	setAdminActionHeaders(revalidate)
	revalidateResponse := httptest.NewRecorder()
	handler.ServeHTTP(revalidateResponse, revalidate)
	if revalidateResponse.Code != http.StatusAccepted || admin.revalidation.Scope != scope || admin.revalidation.Actor != "operator-a" || admin.revalidation.Trigger != memory.ProceduralRevalidationTriggerAdministrator {
		t.Fatalf("revalidation status=%d request=%+v, want accepted server-attributed request", revalidateResponse.Code, admin.revalidation)
	}

	correction := httptest.NewRequest(http.MethodPost, "/v1/admin/procedural-memory-verifications/verification-1:correct", strings.NewReader(`{"reason":"apply verified correction","operation_id":"operation-1","idempotency_key":"key-1"}`))
	setAdminActionHeaders(correction)
	correction.Header.Set("X-Request-ID", "request-1")
	correctionResponse := httptest.NewRecorder()
	handler.ServeHTTP(correctionResponse, correction)
	if correctionResponse.Code != http.StatusAccepted || admin.correction.Scope != scope || admin.correction.Actor != "operator-a" || admin.correction.VerificationID != "verification-1" || admin.correction.RequestID != "request-1" {
		t.Fatalf("correction status=%d request=%+v, want accepted server-attributed handoff", correctionResponse.Code, admin.correction)
	}
}

func TestAdminProceduralRevalidationRunDetailIsScopedAndBounded(t *testing.T) {
	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	admin := &proceduralVerificationHTTPStub{run: memory.ProceduralRevalidationRun{
		ID: "run-1", Scope: scope, VerificationID: "verification-1", MemoryID: "memory-1", TargetVersion: 2,
		SourceWatermark: "watermark-1", Trigger: memory.ProceduralRevalidationTriggerAdministrator, WorkIdentity: "opaque-work-identity",
		Status: memory.ProceduralRevalidationRunStatusRunning, Attempt: 1, MaxAttempts: 3, LeaseOwner: "sensitive-worker-id", CreatedAt: time.Now().UTC(),
	}}
	handler := NewHTTPHandler(HTTPDependencies{Readiness: stubReadinessChecker{}, AdminAPIKeys: map[string]struct{}{"admin-key": {}}, ProceduralVerificationAdmin: admin})

	public := httptest.NewRequest(http.MethodGet, "/v1/admin/procedural-revalidation-runs/run-1", nil)
	public.Header.Set("X-API-Key", "public-key")
	publicResponse := httptest.NewRecorder()
	handler.ServeHTTP(publicResponse, public)
	if publicResponse.Code != http.StatusUnauthorized || admin.readRunCalls != 0 {
		t.Fatalf("public revalidation run detail status=%d read calls=%d, want 401 before lookup", publicResponse.Code, admin.readRunCalls)
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/admin/procedural-revalidation-runs/run-1", nil)
	setAdminActionHeaders(request)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin revalidation run detail status=%d body=%s, want 200", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "sensitive-worker-id") || strings.Contains(response.Body.String(), "opaque-work-identity") {
		t.Fatalf("revalidation run detail disclosed bounded-internal fields: %s", response.Body.String())
	}
}

type proceduralVerificationHTTPStub struct {
	record       memory.ProceduralMemoryVerification
	recordCalls  int
	readCalls    int
	revalidation memory.ProceduralRevalidationRequest
	correction   memory.VerificationCorrectionRequest
	run          memory.ProceduralRevalidationRun
	readRunCalls int
}

func (s *proceduralVerificationHTTPStub) Record(_ context.Context, record memory.ProceduralMemoryVerification) (memory.ProceduralMemoryVerification, error) {
	s.recordCalls++
	s.record = record
	if s.record.ID == "" {
		s.record.ID = "verification-1"
	}
	return s.record, nil
}

func (s *proceduralVerificationHTTPStub) ReadProceduralMemoryVerification(_ context.Context, _ memory.Scope, _ string) (memory.ProceduralMemoryVerification, error) {
	s.readCalls++
	return s.record, nil
}

func (s *proceduralVerificationHTTPStub) ListProceduralMemoryVerificationHistory(_ context.Context, _ memory.Scope, _ string, _ int64, _ int) ([]memory.ProceduralMemoryVerification, error) {
	return []memory.ProceduralMemoryVerification{s.record}, nil
}

func (s *proceduralVerificationHTTPStub) ReadProceduralVerificationLineage(_ context.Context, scope memory.Scope, memoryID string, targetVersion int64) (memory.ProceduralVerificationLineage, error) {
	return memory.ProceduralVerificationLineage{Scope: scope, MemoryID: memoryID, TargetVersion: targetVersion}, nil
}

func (s *proceduralVerificationHTTPStub) ReadProceduralRevalidationRun(_ context.Context, _ memory.Scope, _ string) (memory.ProceduralRevalidationRun, error) {
	s.readRunCalls++
	return s.run, nil
}

func (s *proceduralVerificationHTTPStub) Request(_ context.Context, request memory.ProceduralRevalidationRequest) (memory.ProceduralRevalidationRun, error) {
	s.revalidation = request
	return memory.ProceduralRevalidationRun{ID: "run-1", Scope: request.Scope, VerificationID: request.VerificationID, MemoryID: s.record.MemoryID, TargetVersion: s.record.TargetVersion, SourceWatermark: s.record.SourceWatermark, Trigger: request.Trigger, WorkIdentity: "work-1", Status: memory.ProceduralRevalidationRunStatusPending, MaxAttempts: 3, CreatedAt: time.Now().UTC()}, nil
}

func (s *proceduralVerificationHTTPStub) Submit(_ context.Context, request memory.VerificationCorrectionRequest) (memory.MemoryIntentRecord, error) {
	s.correction = request
	return memory.MemoryIntentRecord{ID: "intent-1", Scope: request.Scope, Status: memory.MemoryIntentStatusAccepted}, nil
}
