package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestSynchronizationCapabilitiesValidateAndDiscover(t *testing.T) {
	doc := Discover(CapabilityInput{ProviderVersion: "provider-v1", SchemaVersion: "schema-v1"})
	if err := doc.Validate(); err != nil {
		t.Fatalf("capability validate: %v", err)
	}
	if !doc.Synchronization.Enabled || doc.Synchronization.Transports[0] != SyncTransportOpenAPI {
		t.Fatalf("sync capabilities = %+v", doc.Synchronization)
	}
}

func TestSyncCursorRoundTripAndScopeHash(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	cursor := SyncCursor{ContractVersion: SyncContractVersion, BindingID: "rb_1", ScopeHash: ScopeHash(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}), SnapshotWatermark: "2026-10-04T00:00:00Z", Sequence: 4, IssuedAt: now}
	encoded, err := EncodeSyncCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeSyncCursor(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.BindingID != cursor.BindingID || decoded.Sequence != cursor.Sequence || decoded.ScopeHash != cursor.ScopeHash {
		t.Fatalf("decoded cursor = %+v, want %+v", decoded, cursor)
	}
}

func TestSyncRequestRejectsUnsupportedVersionAndLimit(t *testing.T) {
	caps := Discover(CapabilityInput{}).Synchronization
	if err := (SyncRequest{SchemaVersion: "schema-v1"}).Validate(caps); err == nil {
		t.Fatal("unsupported sync schema accepted")
	}
	if err := (SyncRequest{SchemaVersion: SyncContractVersion, MaxEvents: caps.MaxBatchEvents + 1}).Validate(caps); err == nil {
		t.Fatal("oversized event batch accepted")
	}
}

func TestSyncResponseRejectsUnavailablePayload(t *testing.T) {
	caps := Discover(CapabilityInput{}).Synchronization
	response := SyncResponse{SchemaVersion: caps.ContractVersion, Status: SyncStatusDelta, SyncID: "sync_1", Events: []SyncEvent{{Sequence: 1, ReplayID: "evt_1", Kind: SyncEventRawEvent, SchemaVersion: SyncContractVersion, SourceWatermark: "w1", Available: false, Payload: []byte(`{"content":"secret"}`)}}}
	if err := response.Validate(caps); err == nil {
		t.Fatal("unavailable event payload accepted")
	}
}

func TestSynchronizerInitialAndResume(t *testing.T) {
	binding := RuntimeBinding{BindingID: "rb_1", PrincipalID: "principal", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, AgentID: "agent", SessionID: "session", ProviderInstanceID: "pi", CreatedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Hour)}
	source := &MemorySyncSource{Snapshots: map[string]SyncSnapshot{binding.BindingID: {SnapshotID: "snap_1", Watermark: "w1", Items: []json.RawMessage{json.RawMessage(`{"id":"m1"}`)}}}, EventsByBinding: map[string][]SyncEvent{binding.BindingID: {{Sequence: 1, ReplayID: "evt_1", Kind: SyncEventRawEvent, SchemaVersion: SyncContractVersion, SourceWatermark: "w2", Available: true, Payload: json.RawMessage(`{"id":"evt_1"}`)}}}}
	caps := Discover(CapabilityInput{}).Synchronization
	syncer := Synchronizer{Source: source, Capabilities: caps}
	initial, err := syncer.Synchronize(context.Background(), binding, SyncRequest{SchemaVersion: SyncContractVersion})
	if err != nil {
		t.Fatal(err)
	}
	if !initial.SyncComplete || initial.Snapshot == nil {
		t.Fatalf("initial = %+v", initial)
	}
	next, err := syncer.Synchronize(context.Background(), binding, SyncRequest{SchemaVersion: SyncContractVersion, Cursor: initial.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Events) != 1 || next.Events[0].ReplayID != "evt_1" {
		t.Fatalf("resume = %+v", next)
	}
}

func TestSynchronizerRejectsForeignCursor(t *testing.T) {
	binding := RuntimeBinding{BindingID: "rb_1", PrincipalID: "principal", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, AgentID: "agent", SessionID: "session", ProviderInstanceID: "pi", CreatedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Hour)}
	source := &MemorySyncSource{Snapshots: map[string]SyncSnapshot{binding.BindingID: {SnapshotID: "snap_1", Watermark: "w1"}}}
	syncer := Synchronizer{Source: source, Capabilities: Discover(CapabilityInput{}).Synchronization}
	initial, err := syncer.Synchronize(context.Background(), binding, SyncRequest{SchemaVersion: SyncContractVersion})
	if err != nil {
		t.Fatal(err)
	}
	foreign := binding
	foreign.BindingID = "rb_2"
	if _, err := syncer.Synchronize(context.Background(), foreign, SyncRequest{SchemaVersion: SyncContractVersion, Cursor: initial.NextCursor}); err == nil {
		t.Fatal("foreign cursor accepted")
	}
}

func TestSynchronizerRejectsRetentionGap(t *testing.T) {
	binding := RuntimeBinding{BindingID: "rb_retention", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, AgentID: "agent", SessionID: "session", ProviderInstanceID: "pi", CreatedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Hour)}
	source := &MemorySyncSource{
		Snapshots:       map[string]SyncSnapshot{binding.BindingID: {SnapshotID: "snap", Watermark: "w"}},
		EventsByBinding: map[string][]SyncEvent{binding.BindingID: {{Sequence: 3, ReplayID: "evt-3", Kind: SyncEventRawEvent, SchemaVersion: SyncContractVersion, SourceWatermark: "w3", Available: true}}},
		RetentionFloors: map[string]int64{binding.BindingID: 3},
	}
	syncer := Synchronizer{Source: source, Capabilities: Discover(CapabilityInput{}).Synchronization}
	initial, err := syncer.Synchronize(context.Background(), binding, SyncRequest{SchemaVersion: SyncContractVersion})
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := DecodeSyncCursor(initial.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	cursor.Sequence = 1
	encoded, err := EncodeSyncCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncer.Synchronize(context.Background(), binding, SyncRequest{SchemaVersion: SyncContractVersion, Cursor: encoded}); err == nil || !strings.Contains(err.Error(), "resync required") {
		t.Fatalf("retention gap error = %v", err)
	}
}

func TestSynchronizerPagesSnapshotWithStableCursor(t *testing.T) {
	binding := RuntimeBinding{BindingID: "rb_pages", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, AgentID: "agent", SessionID: "session", ProviderInstanceID: "pi", CreatedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Hour)}
	source := &MemorySyncSource{Snapshots: map[string]SyncSnapshot{binding.BindingID: {SnapshotID: "snap", Watermark: "w", Items: []json.RawMessage{json.RawMessage(`{"id":"one"}`), json.RawMessage(`{"id":"two"}`)}}}}
	caps := Discover(CapabilityInput{}).Synchronization
	caps.MaxSnapshotBytes = 20
	syncer := Synchronizer{Source: source, Capabilities: caps}
	first, err := syncer.Synchronize(context.Background(), binding, SyncRequest{SchemaVersion: SyncContractVersion, MaxSnapshotBytes: 20})
	if err != nil {
		t.Fatal(err)
	}
	if first.SyncComplete || first.Snapshot == nil || first.NextCursor == "" {
		t.Fatalf("expected continuation page, got %+v", first)
	}
	second, err := syncer.Synchronize(context.Background(), binding, SyncRequest{SchemaVersion: SyncContractVersion, Cursor: first.NextCursor, MaxSnapshotBytes: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !second.SyncComplete || second.Snapshot == nil || len(second.Snapshot.Items) != 1 {
		t.Fatalf("expected final page, got %+v", second)
	}
}
