package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestProceduralVerificationTaskEvidencePassesScopedTaskEvaluationAPI(t *testing.T) {
	service := &stubTaskEvaluationService{validate: true}
	handler := NewHTTPHandler(HTTPDependencies{
		Readiness:       stubReadinessChecker{},
		APIKeys:         map[string]struct{}{"test-key": {}},
		TaskEvaluations: service,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/task-evaluations", strings.NewReader(`{"objective":"validate maintained procedure","success_criteria":["report evidence"],"verdict":"inconclusive","evidence":[{"kind":"procedural_verification","id":"verification-1"}],"actor":"operator-a","reason":"external observation"}`))
	setAPIScopeHeaders(req)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s, want 201", resp.Code, resp.Body.String())
	}
	if len(service.gotCreateInput.Evidence) != 1 || service.gotCreateInput.Evidence[0].Kind != memory.TaskEvidenceTargetProceduralVerification || service.gotCreateInput.Scope != (memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}) {
		t.Fatalf("task evidence input = %+v, want exact-scope procedural verification reference", service.gotCreateInput)
	}
}
