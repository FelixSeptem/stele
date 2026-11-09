package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/telemetry"
)

func TestProceduralRevalidationServiceCreatesOneBoundedRunAndDispatchesReference(t *testing.T) {
	verification := serviceProceduralVerification()
	store := &proceduralRevalidationRequestStoreStub{verification: verification}
	dispatcher := &proceduralRevalidationRequestDispatcherStub{}
	service := ProceduralRevalidationService{
		Store: store, Policy: StaticProceduralVerificationPolicy{Scope: verification.Scope, Enabled: true, Version: "policy-v1"}, Dispatcher: dispatcher,
		Now: func() time.Time { return time.Date(2026, time.November, 8, 3, 0, 0, 0, time.UTC) }, NewID: func() string { return "run-1" },
		MaxAttempts: 3,
	}
	run, err := service.Request(context.Background(), ProceduralRevalidationRequest{Scope: verification.Scope, VerificationID: verification.ID, Trigger: ProceduralRevalidationTriggerAdministrator, Actor: "admin-1", Reason: "review evidence"})
	if err != nil {
		t.Fatal(err)
	}
	if run.ID != "run-1" || len(store.created) != 1 || len(dispatcher.dispatched) != 1 || dispatcher.dispatched[0].RunID != run.ID {
		t.Fatalf("run=%+v created=%+v dispatched=%+v", run, store.created, dispatcher.dispatched)
	}
	if run.Status != ProceduralRevalidationRunStatusPending || run.MaxAttempts != 3 || run.SourceWatermark != verification.SourceWatermark || run.WorkIdentity == "" {
		t.Fatalf("created run is not a bounded copy of verification state: %+v", run)
	}
	if got := dispatcher.dispatched[0]; got.Scope != verification.Scope || got.Watermark != verification.SourceWatermark || got.Identity != run.WorkIdentity {
		t.Fatalf("dispatch leaked or omitted durable identity: %+v", got)
	}
}

func TestProceduralRevalidationServiceRejectsDisabledPolicyBeforeRunCreation(t *testing.T) {
	verification := serviceProceduralVerification()
	store := &proceduralRevalidationRequestStoreStub{verification: verification}
	service := ProceduralRevalidationService{Store: store, Policy: StaticProceduralVerificationPolicy{Scope: verification.Scope, Enabled: false, Version: "policy-v1"}}
	_, err := service.Request(context.Background(), ProceduralRevalidationRequest{Scope: verification.Scope, VerificationID: verification.ID, Trigger: ProceduralRevalidationTriggerAdministrator, Actor: "admin-1", Reason: "review evidence"})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("Request() error = %v", err)
	}
	if len(store.created) != 0 {
		t.Fatalf("disabled policy created runs: %+v", store.created)
	}
}

func TestProceduralRevalidationServiceUsesSameIdentityForEquivalentRequest(t *testing.T) {
	verification := serviceProceduralVerification()
	store := &proceduralRevalidationRequestStoreStub{verification: verification}
	dispatcher := &proceduralRevalidationRequestDispatcherStub{}
	service := ProceduralRevalidationService{
		Store: store, Policy: StaticProceduralVerificationPolicy{Scope: verification.Scope, Enabled: true, Version: "policy-v1"}, Dispatcher: dispatcher,
		Now: func() time.Time { return time.Date(2026, time.November, 8, 3, 0, 0, 0, time.UTC) }, NewID: func() string { return "run-1" },
	}
	request := ProceduralRevalidationRequest{Scope: verification.Scope, VerificationID: verification.ID, Trigger: ProceduralRevalidationTriggerAdministrator, Actor: "admin-1", Reason: "review evidence"}
	first, err := service.Request(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Request(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.WorkIdentity != second.WorkIdentity {
		t.Fatalf("equivalent request work identities differ: %q != %q", first.WorkIdentity, second.WorkIdentity)
	}
}

func TestProceduralRevalidationServiceRetainsDurableRunWhenDispatchFails(t *testing.T) {
	verification := serviceProceduralVerification()
	store := &proceduralRevalidationRequestStoreStub{verification: verification}
	observer := telemetry.NewMetricsObserver()
	service := ProceduralRevalidationService{
		Store: store, Policy: enabledProceduralVerificationPolicy(verification.Scope),
		Dispatcher: &proceduralRevalidationRequestDispatcherStub{err: errors.New("queue unavailable")},
		Now:        func() time.Time { return time.Date(2026, time.November, 8, 3, 0, 0, 0, time.UTC) }, NewID: func() string { return "run-dispatch-failed" }, Observer: observer,
	}
	_, err := service.Request(context.Background(), ProceduralRevalidationRequest{Scope: verification.Scope, VerificationID: verification.ID, Trigger: ProceduralRevalidationTriggerAdministrator, Actor: "admin-1", Reason: "retry after queue outage"})
	if err == nil || !strings.Contains(err.Error(), "dispatch") {
		t.Fatalf("Request() error = %v, want dispatch failure", err)
	}
	if len(store.created) != 1 || store.created[0].Status != ProceduralRevalidationRunStatusPending {
		t.Fatalf("durable pending run = %+v, want retained recovery target", store.created)
	}
	metrics := observer.RenderPrometheus()
	for _, want := range []string{`operation="request"`, `result="failed"`, `queue="durable"`, `failure_category="dependency"`} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("dispatch failure telemetry missing %s:\n%s", want, metrics)
		}
	}
}

func TestProceduralRevalidationServicePreservesEligibleManualAndFeedbackTriggers(t *testing.T) {
	verification := serviceProceduralVerification()
	for _, trigger := range []ProceduralRevalidationTrigger{
		ProceduralRevalidationTriggerAdministrator,
		ProceduralRevalidationTriggerCorrectionSignal,
		ProceduralRevalidationTriggerStaleFeedback,
		ProceduralRevalidationTriggerNeedsReviewFeedback,
	} {
		t.Run(string(trigger), func(t *testing.T) {
			store := &proceduralRevalidationRequestStoreStub{verification: verification}
			service := ProceduralRevalidationService{
				Store: store, Policy: enabledProceduralVerificationPolicy(verification.Scope), Dispatcher: &proceduralRevalidationRequestDispatcherStub{},
				Now: func() time.Time { return time.Date(2026, time.November, 8, 3, 0, 0, 0, time.UTC) }, NewID: func() string { return "run-trigger-" + string(trigger) },
			}
			run, err := service.Request(context.Background(), ProceduralRevalidationRequest{Scope: verification.Scope, VerificationID: verification.ID, Trigger: trigger, Actor: "admin-1", Reason: "eligible maintenance signal"})
			if err != nil || run.Trigger != trigger || len(store.created) != 1 || store.created[0].Trigger != trigger {
				t.Fatalf("trigger=%q run=%+v created=%+v err=%v", trigger, run, store.created, err)
			}
		})
	}
}

func TestProceduralRevalidationServiceRefusesForeignVerificationBeforeRunCreation(t *testing.T) {
	verification := serviceProceduralVerification()
	requestedScope := verification.Scope
	verification.Scope.Tenant = "tenant-b"
	store := &proceduralRevalidationRequestStoreStub{verification: verification}
	service := ProceduralRevalidationService{
		Store: store, Policy: enabledProceduralVerificationPolicy(requestedScope), Dispatcher: &proceduralRevalidationRequestDispatcherStub{},
		Now: func() time.Time { return time.Date(2026, time.November, 8, 3, 0, 0, 0, time.UTC) }, NewID: func() string { return "run-foreign" },
	}
	_, err := service.Request(context.Background(), ProceduralRevalidationRequest{Scope: requestedScope, VerificationID: verification.ID, Trigger: ProceduralRevalidationTriggerAdministrator, Actor: "admin-1", Reason: "foreign probe"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Request() error = %v, want lifecycle-safe not found", err)
	}
	if len(store.created) != 0 {
		t.Fatalf("foreign verification created durable run: %+v", store.created)
	}
}

func TestProceduralRevalidationServiceRecordsBoundedRequestTelemetry(t *testing.T) {
	verification := serviceProceduralVerification()
	observer := telemetry.NewMetricsObserver()
	service := ProceduralRevalidationService{
		Store: &proceduralRevalidationRequestStoreStub{verification: verification}, Policy: enabledProceduralVerificationPolicy(verification.Scope), Dispatcher: &proceduralRevalidationRequestDispatcherStub{},
		Now: func() time.Time { return time.Date(2026, time.November, 8, 3, 0, 0, 0, time.UTC) }, NewID: func() string { return "run-telemetry" }, Observer: observer,
	}
	if _, err := service.Request(context.Background(), ProceduralRevalidationRequest{Scope: verification.Scope, VerificationID: verification.ID, Trigger: ProceduralRevalidationTriggerStaleFeedback, Actor: "admin-1", Reason: "eligible feedback"}); err != nil {
		t.Fatal(err)
	}
	metrics := observer.RenderPrometheus()
	for _, want := range []string{`operation="request"`, `result="accepted"`, `trigger="stale_feedback"`, `queue="durable"`} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("request telemetry missing %s:\n%s", want, metrics)
		}
	}
}

type proceduralRevalidationRequestStoreStub struct {
	verification ProceduralMemoryVerification
	created      []ProceduralRevalidationRun
}

func (s *proceduralRevalidationRequestStoreStub) ReadProceduralMemoryVerification(context.Context, Scope, string) (ProceduralMemoryVerification, error) {
	return s.verification, nil
}
func (s *proceduralRevalidationRequestStoreStub) CreateProceduralRevalidationRun(_ context.Context, run ProceduralRevalidationRun) (ProceduralRevalidationRun, error) {
	s.created = append(s.created, run)
	return run, nil
}

type proceduralRevalidationRequestDispatcherStub struct {
	dispatched []ProceduralRevalidationDispatch
	err        error
}

func (s *proceduralRevalidationRequestDispatcherStub) DispatchProceduralRevalidation(_ context.Context, request ProceduralRevalidationDispatch) error {
	s.dispatched = append(s.dispatched, request)
	return s.err
}
