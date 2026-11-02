package provider

import (
	"strings"
	"testing"
)

func correctionJSON() string {
	return `{"metadata":{"request_id":"r1","operation_id":"correct","schema_version":"provider-v1","idempotency_key":"key1"},"input":{"contract_version":"provider-correction-governed-version-v1","projection_id":"11111111-1111-4111-8111-111111111111","target_memory_id":"22222222-2222-4222-8222-222222222222","expected_version":2,"content":"Corrected durable fact","reason":"source correction"}}`
}

func TestProviderCorrectionRequestUsesClosedWireContract(t *testing.T) {
	valid := correctionJSON()
	request, err := DecodeCorrectionRequest([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if request.Input.ProjectionID != "11111111-1111-4111-8111-111111111111" || request.Input.ExpectedVersion != 2 {
		t.Fatalf("decoded request = %+v", request.Input)
	}

	malformed := map[string]string{
		"unknown_authority": strings.Replace(valid, `"expected_version":2`, `"binding_id":"foreign","expected_version":2`, 1),
		"alias":             strings.Replace(valid, `"projection_id"`, `"Projection_ID"`, 1),
		"duplicate":         strings.Replace(valid, `"expected_version":2`, `"expected_version":2,"expected_version":2`, 1),
		"null":              strings.Replace(valid, `"expected_version":2`, `"expected_version":null`, 1),
		"trailing":          valid + `{}`,
		"nil_projection":    strings.Replace(valid, "11111111-1111-4111-8111-111111111111", "00000000-0000-0000-0000-000000000000", 1),
		"nil_target":        strings.Replace(valid, "22222222-2222-4222-8222-222222222222", "00000000-0000-0000-0000-000000000000", 1),
		"zero_version":      strings.Replace(valid, `"expected_version":2`, `"expected_version":0`, 1),
		"blank_content":     strings.Replace(valid, "Corrected durable fact", "   ", 1),
		"blank_reason":      strings.Replace(valid, "source correction", "   ", 1),
		"unknown_contract":  strings.Replace(valid, "provider-correction-governed-version-v1", "provider-correction-governed-version-v2", 1),
	}
	for name, body := range malformed {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeCorrectionRequest([]byte(body)); err == nil {
				t.Fatal("accepted malformed correction request")
			}
		})
	}

	if _, err := DecodeCorrectionRequest([]byte(strings.Repeat(" ", MaxCorrectionRequestBytes) + valid)); err == nil {
		t.Fatal("accepted oversized correction request")
	}

	oversizedContent := strings.Replace(valid, "Corrected durable fact", strings.Repeat("c", MaxCorrectionContentBytes+1), 1)
	if _, err := DecodeCorrectionRequest([]byte(oversizedContent)); err == nil {
		t.Fatal("accepted correction content above its field limit")
	}
	oversizedReason := strings.Replace(valid, "source correction", strings.Repeat("r", MaxCorrectionReasonBytes+1), 1)
	if _, err := DecodeCorrectionRequest([]byte(oversizedReason)); err == nil {
		t.Fatal("accepted correction reason above its field limit")
	}
}

func TestCorrectionCapabilityIsOptInAndBounded(t *testing.T) {
	if capability := CorrectionCapability(false); capability.Enabled {
		t.Fatal("correction capability must be disabled when its resolver is unavailable")
	}
	capability := CorrectionCapability(true)
	if !capability.Enabled || capability.ContractVersion != CorrectionContractVersion || capability.Operation != "POST /v1/provider/corrections" {
		t.Fatalf("enabled correction capability = %+v", capability)
	}
	if capability.MaxRequestBytes != MaxCorrectionRequestBytes || capability.MaxContentBytes != MaxCorrectionContentBytes || capability.MaxReasonBytes != MaxCorrectionReasonBytes {
		t.Fatalf("correction limits = %+v", capability)
	}
}

func TestStrictCorrectionConsumerRequiresAdvertisedCapabilityAndOperation(t *testing.T) {
	document := Discover(CapabilityInput{ProviderVersion: "provider-v1", SchemaVersion: "provider-v1"})
	if err := RequireCorrectionCapability(document); err == nil {
		t.Fatal("strict correction consumer accepted a server without the capability")
	}

	document.Corrections = CorrectionCapability(true)
	if err := RequireCorrectionCapability(document); err == nil {
		t.Fatal("strict correction consumer accepted a capability missing its operation")
	}
	document.Operations = append(document.Operations, CorrectionOperationName)
	if err := RequireCorrectionCapability(document); err != nil {
		t.Fatalf("strict correction consumer rejected a complete capability: %v", err)
	}

	// Base Provider consumers remain compatible when they do not require this
	// additive capability; document validation does not make it mandatory.
	if err := documentWithCorrectionsDisabled().Validate(); err != nil {
		t.Fatalf("non-correction consumer lost base compatibility: %v", err)
	}
}

func documentWithCorrectionsDisabled() CapabilityDocument {
	return Discover(CapabilityInput{ProviderVersion: "provider-v1", SchemaVersion: "provider-v1"})
}
