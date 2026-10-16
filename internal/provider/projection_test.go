package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func projectionJSON() string {
	return `{"metadata":{"request_id":"r1","operation_id":"project","schema_version":"v1","idempotency_key":"key1"},"input":{"contract_version":"turn-outcome-feedback-projection-v1","kind":"outcome","effect_id":"effect1","source_revision":1,"occurred_at":"2026-10-16T00:00:00Z","reported_outcome":"succeeded","references":[{"event_id":"11111111-1111-4111-8111-111111111111"}],"feedback":{"signal":"useful","reason_code":"helpful"}}}`
}

func TestProjectionClosedContract(t *testing.T) {
	valid := projectionJSON()
	if _, err := DecodeProjectionRequest([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"authority":           strings.Replace(valid, `"kind":"outcome"`, `"task_state":"done","kind":"outcome"`, 1),
		"alias":               strings.Replace(valid, `"effect_id"`, `"Effect_ID"`, 1),
		"duplicate":           strings.Replace(valid, `"source_revision":1`, `"source_revision":1,"source_revision":1`, 1),
		"null":                strings.Replace(valid, `"source_revision":1`, `"source_revision":null`, 1),
		"overflow":            strings.Replace(valid, `"source_revision":1`, `"source_revision":2147483648`, 1),
		"zero_revision":       strings.Replace(valid, `"source_revision":1`, `"source_revision":0`, 1),
		"nil_uuid":            strings.ReplaceAll(valid, "11111111-1111-4111-8111-111111111111", "00000000-0000-0000-0000-000000000000"),
		"nested_unknown":      strings.Replace(valid, `"signal":"useful"`, `"Signal":"useful"`, 1),
		"nested_null":         strings.Replace(valid, `"reason_code":"helpful"`, `"reason_code":null`, 1),
		"nonutc":              strings.Replace(valid, "2026-10-16T00:00:00Z", "2026-10-16T08:00:00+08:00", 1),
		"invalid_time":        strings.Replace(valid, "2026-10-16T00:00:00Z", "yesterday", 1),
		"oversize":            strings.Replace(valid, "effect1", strings.Repeat("a", 129), 1),
		"bad_outcome":         strings.Replace(valid, "succeeded", "committed", 1),
		"bad_reference":       strings.Replace(valid, `"event_id":`, `"tenant":"foreign","event_id":`, 1),
		"duplicate_reference": strings.Replace(valid, `"references":[{`, `"references":[{"event_id":"11111111-1111-4111-8111-111111111111"},{`, 1),
		"surrogate":           strings.Replace(valid, "effect1", `\ud800`, 1),
		"bad_digest":          strings.Replace(valid, `"kind":"outcome"`, `"context_digest":"not-a-hash","kind":"outcome"`, 1),
		"empty_key":           strings.Replace(valid, `"idempotency_key":"key1"`, `"idempotency_key":""`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeProjectionRequest([]byte(data)); err == nil {
				t.Fatal("accepted malformed request")
			}
		})
	}
	if _, err := DecodeProjectionRequest([]byte(strings.Repeat(" ", MaxProjectionRequestBytes) + valid)); err == nil {
		t.Fatal("unbounded request")
	}
}

func TestProjectionFingerprintExcludesDeliveryMetadata(t *testing.T) {
	a, err := DecodeProjectionRequest([]byte(projectionJSON()))
	if err != nil {
		t.Fatal(err)
	}
	b := a
	b.Metadata.RequestID = "redelivery"
	b.Metadata.EventSeq = 99
	if a.Input.Fingerprint() != b.Input.Fingerprint() {
		t.Fatal("delivery metadata changed payload identity")
	}
	b.Input.SourceRevision++
	if a.Input.Fingerprint() == b.Input.Fingerprint() {
		t.Fatal("revision omitted from fingerprint")
	}
}

func TestProjectionFingerprintNormalizesSetsWithoutMutatingInput(t *testing.T) {
	a, _ := DecodeProjectionRequest([]byte(projectionJSON()))
	a.Input.References = append(a.Input.References, ProjectionReferenceSelector{EventID: "22222222-2222-4222-8222-222222222222"})
	a.Input.Provenance = []string{"z", "a"}
	b := a
	b.Input.References = []ProjectionReferenceSelector{a.Input.References[1], a.Input.References[0]}
	b.Input.Provenance = []string{"a", "z"}
	if a.Input.Fingerprint() != b.Input.Fingerprint() {
		t.Fatal("set ordering changed normalized identity")
	}
	if a.Input.Provenance[0] != "z" {
		t.Fatal("fingerprinting mutated caller")
	}
}

func TestProjectionHistoryClosedContract(t *testing.T) {
	var req map[string]any
	json.Unmarshal([]byte(projectionJSON()), &req)
	req["input"] = map[string]any{"contract_version": ProjectionContractVersion, "kind": "outcome", "effect_id": "effect1", "limit": 8}
	data, _ := json.Marshal(req)
	if _, err := DecodeProjectionHistoryRequest(data); err != nil {
		t.Fatal(err)
	}
	for _, v := range []any{nil, 0, 9, 33, "32"} {
		req["input"].(map[string]any)["limit"] = v
		data, _ = json.Marshal(req)
		if _, err := DecodeProjectionHistoryRequest(data); err == nil {
			t.Fatalf("accepted limit %v", v)
		}
	}
}

func TestProjectionMetadataRequiresCanonicalWireValuesWithoutChangingLegacy(t *testing.T) {
	history := `{"metadata":{"request_id":"r1","operation_id":"project","schema_version":"v1"},"input":{"contract_version":"turn-outcome-feedback-projection-v1","kind":"outcome","effect_id":"effect1","limit":8}}`
	for operation, base := range map[string]string{"admit": projectionJSON(), "history": history} {
		for name, replacement := range map[string]string{
			"trimmed request": `"request_id":" r1 "`,
			"trimmed path":    `"request_id":"r1","memory_path":" /notes/project/ "`,
			"slash path":      `"request_id":"r1","memory_path":"/notes/project/"`,
		} {
			t.Run(operation+"/"+name, func(t *testing.T) {
				data := []byte(strings.Replace(base, `"request_id":"r1"`, replacement, 1))
				var err error
				if base == history {
					_, err = DecodeProjectionHistoryRequest(data)
				} else {
					_, err = DecodeProjectionRequest(data)
				}
				if err == nil {
					t.Fatal("PC6 accepted metadata requiring normalization")
				}
			})
		}
	}
	legacy := `{"metadata":{"request_id":" r1 ","operation_id":"project","schema_version":"v1","idempotency_key":"key1","memory_path":" /notes/project/ "},"input":{"contract_version":"durable-reference-v1","reference_id":"note1","memory_path":"notes/project","event_type":"provider.reference.task","value":"content"}}`
	// Exercise the unchanged shared envelope separately from PC6's strict wire check.
	var input ReferenceWriteInput
	meta, err := decodeReferenceEnvelope([]byte(legacy), &input, []string{"contract_version", "reference_id", "memory_path", "event_type", "value"}, []string{"contract_version", "reference_id", "memory_path", "event_type", "value"}, true)
	if err != nil || meta.RequestID != "r1" || meta.MemoryPath != "notes/project" {
		t.Fatalf("legacy normalization changed: %+v %v", meta, err)
	}
}

func TestProjectionRecordStateUnions(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	base := ProjectionRecord{ProjectionID: id, LogicalID: id, SourceRevision: 1, Disposition: "accepted", EvidenceKind: ProjectionEvidenceKind, References: []ProjectionReferenceAttribution{}, CurrentReferences: []ProjectionReferenceAttribution{}, FeedbackState: "none", ContextDigestAttribution: "none"}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ProjectionRecord){
		"nil_id":          func(r *ProjectionRecord) { r.ProjectionID = "00000000-0000-0000-0000-000000000000" },
		"authority":       func(r *ProjectionRecord) { r.EvidenceKind = "task_completed" },
		"bad_disposition": func(r *ProjectionRecord) { r.Disposition = "committed" },
		"pending_eligible": func(r *ProjectionRecord) {
			r.References = []ProjectionReferenceAttribution{{EventID: id, State: "pending", Eligible: true}}
		},
		"unavailable_identity": func(r *ProjectionRecord) {
			r.References = []ProjectionReferenceAttribution{{EventID: id, State: "unavailable"}}
		},
		"completed_missing_id": func(r *ProjectionRecord) {
			r.References = []ProjectionReferenceAttribution{{State: "completed", Eligible: true}}
		},
		"null_refs":      func(r *ProjectionRecord) { r.References = nil },
		"bad_feedback":   func(r *ProjectionRecord) { r.FeedbackState = "ranking_activated" },
		"bad_digest":     func(r *ProjectionRecord) { r.ContextDigestAttribution = "authorized_content" },
		"bad_correction": func(r *ProjectionRecord) { r.Disposition = "corrected" },
	} {
		t.Run(name, func(t *testing.T) {
			r := base
			mutate(&r)
			if r.Validate() == nil {
				t.Fatal("accepted inconsistent state")
			}
		})
	}
}

func TestProjectionCompletedAttributionRequiresClosedLineage(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	ref := ProjectionReferenceAttribution{EventID: id, State: "completed", Eligible: true}
	record := ProjectionRecord{ProjectionID: id, LogicalID: id, SourceRevision: 1, Disposition: "accepted", EvidenceKind: ProjectionEvidenceKind, References: []ProjectionReferenceAttribution{ref}, CurrentReferences: []ProjectionReferenceAttribution{ref}, FeedbackState: "none", ContextDigestAttribution: "none"}
	if record.Validate() == nil {
		t.Fatal("completed evidence without PC5 lineage accepted")
	}
}

func TestProjectionLineageUnionAndMaximumHistoryAllocation(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	lineage := ProjectionReferenceLineage{CandidateID: id, MemoryID: id, VersionID: id, ProvenanceID: id, Version: 9223372036854775807}
	ref := ProjectionReferenceAttribution{EventID: id, State: "completed", Eligible: true, Lineage: &lineage}
	r := ProjectionRecord{ProjectionID: id, LogicalID: id, SourceRevision: 2147483647, SupersedesID: "22222222-2222-4222-8222-222222222222", Disposition: "corrected", EvidenceKind: ProjectionEvidenceKind, References: []ProjectionReferenceAttribution{ref}, CurrentReferences: []ProjectionReferenceAttribution{ref}, FeedbackState: "pending", ContextDigestAttribution: "verified_receipt"}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"accepted", "pending", "failed", "unavailable"} {
		bad := r
		bad.References = []ProjectionReferenceAttribution{{EventID: id, State: state, Lineage: &lineage}}
		if bad.Validate() == nil {
			t.Fatalf("%s accepted completion lineage", state)
		}
	}
	for _, mutate := range []func(*ProjectionReferenceLineage){func(l *ProjectionReferenceLineage) { l.Version = 0 }, func(l *ProjectionReferenceLineage) { l.ProvenanceID = "00000000-0000-0000-0000-000000000000" }} {
		bad := lineage
		mutate(&bad)
		copy := r
		copy.References = []ProjectionReferenceAttribution{{EventID: id, State: "completed", Eligible: true, Lineage: &bad}}
		if copy.Validate() == nil {
			t.Fatal("invalid completion lineage accepted")
		}
	}
	r.References = nil
	r.CurrentReferences = nil
	for n := 0; n < MaxProjectionReferences; n++ {
		r.References = append(r.References, ref)
		r.CurrentReferences = append(r.CurrentReferences, ref)
	}
	token := strings.Repeat("x", 128)
	out := ProjectionHistoryResponse{Metadata: OperationMetadata{RequestID: token, OperationID: token, SchemaVersion: token}, ContractVersion: ProjectionContractVersion, Scope: CanonicalScopeDescriptor{Tenant: token, Project: token, Namespace: token, AgentID: token, SessionID: token, ConversationID: token, ProviderInstanceID: token}, Records: []ProjectionRecord{}}
	for n := 0; n < MaxProjectionHistory; n++ {
		out.Records = append(out.Records, r)
	}
	data, err := json.Marshal(out)
	if err != nil || len(data)+4096 > MaxProjectionResponseBytes {
		t.Fatalf("maximum history exceeds response allocation: %d %v", len(data), err)
	}
}
