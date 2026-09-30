package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/config"
	"github.com/FelixSeptem/stele/internal/reasoning"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

type registeredReasoningProvider struct{}

func (registeredReasoningProvider) Derive(context.Context, reasoning.InvocationRequest) (reasoning.Candidate, error) {
	return reasoning.Candidate{}, nil
}

func TestReasoningCapabilityIsDisabledByDefaultAcrossRuntimeModes(t *testing.T) {
	cfg := config.Config{Reasoning: config.ReasoningConfig{Limits: reasoning.DefaultLimits()}}
	capabilities := []reasoning.Capability{
		buildReasoningCapability(cfg.Reasoning, nil),
		buildReasoningCapability(cfg.Reasoning, nil),
		buildReasoningCapability(cfg.Reasoning, nil),
	}
	for _, capability := range capabilities {
		if err := capability.Validate(); err != nil {
			t.Fatalf("default capability invalid: %v", err)
		}
		if capability.Enabled || capability.Mode != reasoning.ModeDisabled {
			t.Fatalf("default capability = %+v, want disabled", capability)
		}
	}
	if !reflect.DeepEqual(capabilities[0], capabilities[1]) || !reflect.DeepEqual(capabilities[1], capabilities[2]) {
		t.Fatalf("runtime capabilities differ: %+v", capabilities)
	}
}

func TestReasoningCapabilityOfflineModeDoesNotRequireProvider(t *testing.T) {
	cfg := config.ReasoningConfig{Enabled: true, Mode: reasoning.ModeOffline, Limits: reasoning.DefaultLimits()}
	capability := buildReasoningCapability(cfg, nil)
	if err := capability.Validate(); err != nil {
		t.Fatalf("offline capability invalid: %v", err)
	}
	if !capability.Enabled || capability.Mode != reasoning.ModeOffline {
		t.Fatalf("offline capability = %+v, want enabled offline mode", capability)
	}
}

func TestReasoningLiveCapabilityStaysDisabledUntilProviderRegistered(t *testing.T) {
	cfg := config.ReasoningConfig{Enabled: true, Mode: reasoning.ModeLive, Limits: reasoning.DefaultLimits()}
	capability := buildReasoningCapability(cfg, nil)
	if capability.Enabled || capability.Mode != reasoning.ModeDisabled {
		t.Fatalf("unregistered live capability = %+v, want disabled", capability)
	}
}

func TestReasoningLiveCapabilityIsDiscoveredAfterProviderRegistration(t *testing.T) {
	cfg := config.ReasoningConfig{Enabled: true, Mode: reasoning.ModeLive, Limits: reasoning.DefaultLimits()}
	capability := buildReasoningCapability(cfg, registeredReasoningProvider{})
	if err := capability.Validate(); err != nil {
		t.Fatalf("registered live capability invalid: %v", err)
	}
	if !capability.Enabled || capability.Mode != reasoning.ModeLive {
		t.Fatalf("registered live capability = %+v, want enabled live mode", capability)
	}
}

func TestRuntimeBuildersWireTheSameDisabledReasoningCapability(t *testing.T) {
	apiDB, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer apiDB.Close()
	workerDB, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer workerDB.Close()
	schedulerDB, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer schedulerDB.Close()

	base := config.Config{
		PostgresDSN: "postgres://runtime",
		Reasoning:   config.ReasoningConfig{Limits: reasoning.DefaultLimits()},
	}
	var apiDeps HTTPDependencies
	apiRuntime, err := buildAPIRuntime(context.Background(), base, apiRuntimeDependencies{
		openPool:          func(context.Context, string) (postgresRuntimeStore, error) { return apiDB, nil },
		bootstrapDatabase: func(context.Context, postgresRuntimeStore) error { return nil },
		newServer: func(_ string, deps HTTPDependencies) httpServer {
			apiDeps = deps
			return &stubAPIServer{err: http.ErrServerClosed}
		},
	})
	if err != nil {
		t.Fatalf("buildAPIRuntime() error = %v", err)
	}
	if apiDeps.ReasoningCapabilities.Mode != reasoning.ModeDisabled {
		t.Fatalf("api HTTP capability = %+v, want disabled", apiDeps.ReasoningCapabilities)
	}
	workerRuntime, err := buildWorkerRuntime(context.Background(), base, workerRuntimeDependencies{
		openPool:          func(context.Context, string) (postgresRuntimeStore, error) { return workerDB, nil },
		bootstrapDatabase: func(context.Context, postgresRuntimeStore) error { return nil },
	})
	if err != nil {
		t.Fatalf("buildWorkerRuntime() error = %v", err)
	}

	schedulerConfig := base
	schedulerConfig.Auth.DefaultTenant = "tenant-a"
	schedulerConfig.Auth.DefaultProject = "project-a"
	schedulerConfig.Auth.DefaultNamespace = "namespace-a"
	schedulerRuntime, err := buildSchedulerRuntime(context.Background(), schedulerConfig, schedulerRuntimeDependencies{
		openPool:          func(context.Context, string) (postgresRuntimeStore, error) { return schedulerDB, nil },
		bootstrapDatabase: func(context.Context, postgresRuntimeStore) error { return nil },
	})
	if err != nil {
		t.Fatalf("buildSchedulerRuntime() error = %v", err)
	}

	capabilities := []reasoning.Capability{apiRuntime.reasoning, workerRuntime.reasoning, schedulerRuntime.reasoning}
	for _, capability := range capabilities {
		if capability.Enabled || capability.Mode != reasoning.ModeDisabled {
			t.Fatalf("runtime capability = %+v, want disabled without provider", capability)
		}
		if err := capability.Validate(); err != nil {
			t.Fatalf("runtime capability invalid: %v", err)
		}
	}
	if !reflect.DeepEqual(capabilities[0], capabilities[1]) || !reflect.DeepEqual(capabilities[1], capabilities[2]) {
		t.Fatalf("runtime capabilities differ: %+v", capabilities)
	}
}

func TestReasoningCapabilityDiscoveryReturnsBoundedDisabledDocument(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/reasoning/capabilities", nil)
	recorder := httptest.NewRecorder()
	NewHTTPHandler(HTTPDependencies{
		PrincipalAuthorizer: stubPrincipalAuthorizer{
			principal: auth.Principal{ID: "principal-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive},
			granted:   true,
		},
	}).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status without scope = %d, want 400", recorder.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/reasoning/capabilities", nil)
	req.Header.Set(auth.HeaderAPIKey, "key")
	req.Header.Set(auth.HeaderTenant, "tenant-a")
	req.Header.Set(auth.HeaderProject, "project-a")
	req.Header.Set(auth.HeaderNamespace, "namespace-a")
	recorder = httptest.NewRecorder()
	NewHTTPHandler(HTTPDependencies{
		PrincipalAuthorizer: stubPrincipalAuthorizer{
			principal: auth.Principal{ID: "principal-1", Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive},
			granted:   true,
		},
	}).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var capability reasoning.Capability
	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &capability); err != nil {
		t.Fatalf("decode capability: %v", err)
	}
	if err := capability.Validate(); err != nil {
		t.Fatalf("capability invalid: %v", err)
	}
	if capability.Enabled || capability.Mode != reasoning.ModeDisabled {
		t.Fatalf("capability = %+v, want disabled", capability)
	}
}
