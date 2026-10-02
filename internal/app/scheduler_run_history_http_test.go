package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/jobs"
	"github.com/FelixSeptem/stele/internal/memory"
)

type schedulerRunHistoryReaderStub struct {
	page jobs.SchedulerRunHistoryPage
}

type schedulerRunActionStub struct {
	action string
}

func (s *schedulerRunActionStub) CancelSchedulerRun(context.Context, memory.Scope, string, time.Time) error {
	s.action = "cancel"
	return nil
}

func (s *schedulerRunActionStub) RecoverSchedulerRun(context.Context, memory.Scope, string, time.Time) error {
	s.action = "recover"
	return nil
}

func (s *schedulerRunHistoryReaderStub) ListSchedulerRunHistory(_ context.Context, input jobs.SchedulerRunHistoryQuery) (jobs.SchedulerRunHistoryPage, error) {
	if input.Scope.Tenant == "" {
		return jobs.SchedulerRunHistoryPage{}, context.Canceled
	}
	return s.page, nil
}

func (s *schedulerRunHistoryReaderStub) ReadSchedulerRunHistory(context.Context, memory.Scope, string) (jobs.SchedulerRunSummary, []jobs.SchedulerRunAttempt, error) {
	return jobs.SchedulerRunSummary{}, nil, context.Canceled
}

func TestSchedulerRunHistoryAdminSurfaceIsScopedAndPaginated(t *testing.T) {
	reader := &schedulerRunHistoryReaderStub{page: jobs.SchedulerRunHistoryPage{Runs: []jobs.SchedulerRunSummary{{RunKey: "maintenance:opaque", State: jobs.SchedulerRunCompleted}}, NextCursor: "opaque-cursor"}}
	handler := NewHTTPHandler(HTTPDependencies{AdminAPIKeys: auth.StaticAPIKeys{"admin-key": {}}, SchedulerRunHistoryRead: reader})
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs/run-history?limit=2&state=completed&cursor=opaque-cursor", nil)
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "opaque-cursor") {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}

	unauthorized := httptest.NewRequest(http.MethodGet, "/v1/admin/jobs/run-history", nil)
	unauthorizedResp := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedResp, unauthorized)
	if unauthorizedResp.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorizedResp.Code)
	}
}

func TestSchedulerRunAdminActionUsesGovernedPath(t *testing.T) {
	actions := &schedulerRunActionStub{}
	handler := NewHTTPHandler(HTTPDependencies{AdminAPIKeys: auth.StaticAPIKeys{"admin-key": {}}, SchedulerRunActions: actions})
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/jobs/run-history/maintenance:run-1?action=cancel", nil)
	req.Header.Set("X-API-Key", "admin-key")
	req.Header.Set("X-Stele-Tenant", "tenant-a")
	req.Header.Set("X-Stele-Project", "project-a")
	req.Header.Set("X-Stele-Namespace", "namespace-a")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || actions.action != "cancel" {
		t.Fatalf("status=%d action=%q body=%s", resp.Code, actions.action, resp.Body.String())
	}
}
