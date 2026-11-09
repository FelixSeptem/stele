package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/telemetry"
)

func TestProceduralVerificationServiceRecordsOnlyResolvedCurrentTargetAndEvidence(t *testing.T) {
	record := serviceProceduralVerification()
	targets := &proceduralVerificationTargetResolverStub{}
	evidence := &proceduralVerificationEvidenceResolverStub{}
	store := &proceduralVerificationStoreStub{}
	service := ProceduralVerificationService{Targets: targets, Evidence: evidence, Store: store, Policy: enabledProceduralVerificationPolicy(record.Scope)}

	stored, err := service.Record(context.Background(), record)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != record.ID || targets.calls != 1 || evidence.calls != len(record.Evidence) || store.calls != 1 {
		t.Fatalf("stored=%+v targetCalls=%d evidenceCalls=%d storeCalls=%d", stored, targets.calls, evidence.calls, store.calls)
	}
}

func TestProceduralVerificationServiceRejectsIneligibleTargetBeforeAppend(t *testing.T) {
	record := serviceProceduralVerification()
	targets := &proceduralVerificationTargetResolverStub{err: errors.New("not current")}
	store := &proceduralVerificationStoreStub{}
	service := ProceduralVerificationService{Targets: targets, Evidence: &proceduralVerificationEvidenceResolverStub{}, Store: store, Policy: enabledProceduralVerificationPolicy(record.Scope)}

	_, err := service.Record(context.Background(), record)
	if err == nil || !strings.Contains(err.Error(), "validate procedural verification target") {
		t.Fatalf("Record() error = %v", err)
	}
	if store.calls != 0 {
		t.Fatalf("store calls = %d, want no append", store.calls)
	}
}

func TestProceduralVerificationServiceRejectsDisabledPolicyBeforeTargetOrEvidenceLookup(t *testing.T) {
	record := serviceProceduralVerification()
	targets := &proceduralVerificationTargetResolverStub{}
	evidence := &proceduralVerificationEvidenceResolverStub{}
	store := &proceduralVerificationStoreStub{}
	service := ProceduralVerificationService{
		Targets: targets, Evidence: evidence, Store: store,
		Policy: StaticProceduralVerificationPolicy{Scope: record.Scope, Enabled: false, Version: "policy-v1"},
	}

	_, err := service.Record(context.Background(), record)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("Record() error = %v", err)
	}
	if targets.calls != 0 || evidence.calls != 0 || store.calls != 0 {
		t.Fatalf("disabled policy resolved or appended: targets=%d evidence=%d store=%d", targets.calls, evidence.calls, store.calls)
	}
}

func TestProceduralVerificationServiceRecordsBoundedAdmissionTelemetry(t *testing.T) {
	record := serviceProceduralVerification()
	observer := telemetry.NewMetricsObserver()
	service := ProceduralVerificationService{
		Targets: &proceduralVerificationTargetResolverStub{}, Evidence: &proceduralVerificationEvidenceResolverStub{}, Store: &proceduralVerificationStoreStub{},
		Policy: enabledProceduralVerificationPolicy(record.Scope), Observer: observer,
	}
	if _, err := service.Record(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	metrics := observer.RenderPrometheus()
	for _, want := range []string{`operation="record"`, `result="accepted"`, `disposition="supported"`, `trigger="administrator"`} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("admission telemetry missing %s:\n%s", want, metrics)
		}
	}
}

func TestProceduralVerificationServiceRecordsDisabledPolicyTelemetryWithoutRequestContent(t *testing.T) {
	record := serviceProceduralVerification()
	record.Actor = "admin-secret"
	record.Reason = "a detailed verification reason that must never become a label"
	observer := telemetry.NewMetricsObserver()
	service := ProceduralVerificationService{
		Targets: &proceduralVerificationTargetResolverStub{}, Evidence: &proceduralVerificationEvidenceResolverStub{}, Store: &proceduralVerificationStoreStub{},
		Policy: StaticProceduralVerificationPolicy{Scope: record.Scope, Enabled: false, Version: "policy-v1"}, Observer: observer,
	}
	if _, err := service.Record(context.Background(), record); err == nil {
		t.Fatal("Record() accepted disabled policy")
	}
	metrics := observer.RenderPrometheus()
	for _, want := range []string{`operation="record"`, `result="disabled"`, `failure_category="policy_disabled"`} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("disabled admission telemetry missing %s:\n%s", want, metrics)
		}
	}
	for _, forbidden := range []string{record.Actor, record.Reason} {
		if strings.Contains(metrics, forbidden) {
			t.Fatalf("admission telemetry leaked %q:\n%s", forbidden, metrics)
		}
	}
}

func enabledProceduralVerificationPolicy(scope Scope) StaticProceduralVerificationPolicy {
	return StaticProceduralVerificationPolicy{Scope: scope, Enabled: true, Version: "policy-v1"}
}

func serviceProceduralVerification() ProceduralMemoryVerification {
	return ProceduralMemoryVerification{
		ID: "verification-1", Scope: Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}, MemoryID: "memory-1", TargetVersion: 2,
		ClaimFingerprint: strings.Repeat("a", 64), ApplicabilityFingerprint: strings.Repeat("b", 64), Disposition: ProceduralVerificationDispositionSupported,
		Evidence: []ProceduralVerificationEvidence{{Kind: ProceduralVerificationEvidenceToolObservation, SourceKind: ProceduralVerificationEvidenceSourceRawEvent, ID: "event-1", SourceVersion: 1, Corroborates: true}},
		Actor:    "admin-1", Reason: "checked output", PolicyVersion: "policy-v1", SourceWatermark: "watermark-1", CreatedAt: time.Date(2026, time.November, 8, 0, 0, 0, 0, time.UTC),
	}
}

type proceduralVerificationTargetResolverStub struct {
	calls int
	err   error
}

func (s *proceduralVerificationTargetResolverStub) ValidateProceduralVerificationTarget(context.Context, ProceduralMemoryVerification) error {
	s.calls++
	return s.err
}

type proceduralVerificationEvidenceResolverStub struct {
	calls int
	err   error
}

func (s *proceduralVerificationEvidenceResolverStub) ValidateProceduralVerificationEvidence(context.Context, Scope, ProceduralVerificationEvidence) error {
	s.calls++
	return s.err
}

type proceduralVerificationStoreStub struct {
	calls int
}

func (s *proceduralVerificationStoreStub) AppendProceduralMemoryVerification(_ context.Context, record ProceduralMemoryVerification) (ProceduralMemoryVerification, error) {
	s.calls++
	return record, nil
}
