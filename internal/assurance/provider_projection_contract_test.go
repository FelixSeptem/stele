package assurance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/openapi"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestProjectionRequestProducerSchemaConsumerParity(t *testing.T) {
	c, err := NewProviderProjectionContract([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	d, err := openapi3.NewLoader().LoadFromData([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	admit := `{"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1","idempotency_key":"key"},"input":{"contract_version":"turn-outcome-feedback-projection-v1","kind":"outcome","effect_id":"effect","source_revision":1,"occurred_at":"2026-10-16T00:00:00Z","reported_outcome":"unknown"}}`
	history := `{"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1"},"input":{"contract_version":"turn-outcome-feedback-projection-v1","kind":"outcome","effect_id":"effect","limit":8}}`
	for _, operation := range []struct {
		name, schema, base string
		decode             func([]byte) error
		consume            func([]byte) error
	}{
		{"admit", "ProviderProjectionRequest", admit, func(b []byte) error { _, err := provider.DecodeProjectionRequest(b); return err }, func(b []byte) error { _, err := c.ValidateRequest(b, "provider-v1"); return err }},
		{"history", "ProviderProjectionHistoryRequest", history, func(b []byte) error { _, err := provider.DecodeProjectionHistoryRequest(b); return err }, func(b []byte) error { _, err := c.ValidateHistoryRequest(b, "provider-v1"); return err }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			cases := map[string]struct {
				data  string
				valid bool
			}{
				"valid":                      {operation.base, true},
				"maximum request token":      {strings.Replace(operation.base, `"request_id":"r"`, `"request_id":"`+strings.Repeat("r", 256)+`"`, 1), true},
				"empty request token":        {strings.Replace(operation.base, `"request_id":"r"`, `"request_id":""`, 1), false},
				"empty operation token":      {strings.Replace(operation.base, `"operation_id":"o"`, `"operation_id":""`, 1), false},
				"empty schema token":         {strings.Replace(operation.base, `"schema_version":"provider-v1"`, `"schema_version":""`, 1), false},
				"invalid operation token":    {strings.Replace(operation.base, `"operation_id":"o"`, `"operation_id":"bad token"`, 1), false},
				"normalized request token":   {strings.Replace(operation.base, `"request_id":"r"`, `"request_id":" r "`, 1), false},
				"normalized operation token": {strings.Replace(operation.base, `"operation_id":"o"`, `"operation_id":" o "`, 1), false},
				"normalized schema token":    {strings.Replace(operation.base, `"schema_version":"provider-v1"`, `"schema_version":" provider-v1 "`, 1), false},
				"oversize request token":     {strings.Replace(operation.base, `"request_id":"r"`, `"request_id":"`+strings.Repeat("r", 257)+`"`, 1), false},
				"maximum sequence":           {strings.Replace(operation.base, `"request_id":"r"`, `"event_seq":9223372036854775807,"request_id":"r"`, 1), true},
				"sequence overflow":          {strings.Replace(operation.base, `"request_id":"r"`, `"event_seq":18446744073709551616,"request_id":"r"`, 1), false},
			}
			if operation.name == "admit" {
				for _, field := range []string{"session_run_id", "turn_id", "context_digest", "context_reference"} {
					cases["empty "+field] = struct {
						data  string
						valid bool
					}{strings.Replace(operation.base, `"kind":"outcome"`, `"kind":"outcome","`+field+`":""`, 1), false}
				}
				cases["zero time"] = struct {
					data  string
					valid bool
				}{strings.Replace(operation.base, "2026-10-16T00:00:00Z", "0001-01-01T00:00:00Z", 1), false}
				cases["normalized key"] = struct {
					data  string
					valid bool
				}{strings.Replace(operation.base, `"idempotency_key":"key"`, `"idempotency_key":" key "`, 1), false}
			}
			for name, tc := range cases {
				t.Run(name, func(t *testing.T) {
					var value any
					if err := json.Unmarshal([]byte(tc.data), &value); err != nil {
						t.Fatal(err)
					}
					for boundary, err := range map[string]error{"producer": operation.decode([]byte(tc.data)), "schema": d.Components.Schemas[operation.schema].Value.VisitJSON(value), "consumer": operation.consume([]byte(tc.data))} {
						if (err == nil) != tc.valid {
							t.Errorf("%s acceptance=%t, want %t: %v", boundary, err == nil, tc.valid, err)
						}
					}
				})
			}
		})
	}
}

func projectionConsumerFixture() (provider.CanonicalScopeDescriptor, []byte) {
	scope := provider.CanonicalScopeDescriptor{Tenant: "t", Project: "p", Namespace: "n", AgentID: "a", SessionID: "s", ProviderInstanceID: "i"}
	data := []byte(`{"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1"},"contract_version":"turn-outcome-feedback-projection-v1","scope":{"tenant":"t","project":"p","namespace":"n","agent_id":"a","session_id":"s","provider_instance_id":"i"},"record":{"projection_id":"11111111-1111-4111-8111-111111111111","logical_id":"22222222-2222-4222-8222-222222222222","source_revision":1,"disposition":"accepted","evidence_kind":"runtime_reported_evidence","references":[{"event_id":"33333333-3333-4333-8333-333333333333","state":"pending","eligible":false}],"current_references":[{"event_id":"33333333-3333-4333-8333-333333333333","state":"pending","eligible":false}],"feedback_state":"pending","context_digest_attribution":"reported"}}`)
	return scope, data
}

func TestProjectionConsumerClosedUnions(t *testing.T) {
	c, err := NewProviderProjectionContract([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	scope, data := projectionConsumerFixture()
	if _, err = c.Validate(data, scope, "provider-v1"); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(map[string]any){
		"missing evidence":  func(v map[string]any) { delete(v, "evidence_kind") },
		"null collection":   func(v map[string]any) { v["references"] = nil },
		"authority":         func(v map[string]any) { v["task_state"] = "completed" },
		"nil identity":      func(v map[string]any) { v["projection_id"] = "00000000-0000-0000-0000-000000000000" },
		"conflated pending": func(v map[string]any) { v["references"].([]any)[0].(map[string]any)["eligible"] = true },
		"completed without lineage": func(v map[string]any) {
			ref := v["references"].([]any)[0].(map[string]any)
			ref["state"] = "completed"
			ref["eligible"] = true
		},
		"unavailable identity":            func(v map[string]any) { v["references"].([]any)[0].(map[string]any)["state"] = "unavailable" },
		"correction missing supersession": func(v map[string]any) { v["disposition"] = "corrected" },
		"accepted with supersession":      func(v map[string]any) { v["supersedes_id"] = "44444444-4444-4444-8444-444444444444" },
		"cardinality":                     func(v map[string]any) { v["current_references"] = []any{} },
		"activation claim":                func(v map[string]any) { v["feedback_state"] = "active" },
		"current reference substitution": func(v map[string]any) {
			v["current_references"].([]any)[0].(map[string]any)["event_id"] = "44444444-4444-4444-8444-444444444444"
		},
		"unavailable snapshot identity leak": func(v map[string]any) {
			v["current_references"] = []any{map[string]any{"state": "unavailable", "eligible": false}}
		},
		"pending feedback without evidence": func(v map[string]any) {
			v["references"], v["current_references"] = []any{}, []any{}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var v map[string]any
			_ = json.Unmarshal(data, &v)
			mutate(v["record"].(map[string]any))
			bad, _ := json.Marshal(v)
			if _, err := c.Validate(bad, scope, "provider-v1"); err == nil {
				t.Fatal("invalid union accepted")
			}
		})
	}
	for _, bad := range []string{strings.Replace(string(data), `"evidence_kind":"runtime_reported_evidence"`, `"evidence_kind":"runtime_reported_evidence","evidence_kind":"runtime_reported_evidence"`, 1), strings.Replace(string(data), `"reported"`, `"\ud800"`, 1), strings.Replace(string(data), `"source_revision"`, `"Source_Revision"`, 1)} {
		if _, err := c.Validate([]byte(bad), scope, "provider-v1"); err == nil {
			t.Fatal("noncanonical JSON accepted")
		}
	}
	scope.SessionID = "foreign"
	if _, err := c.Validate(data, scope, "provider-v1"); err == nil {
		t.Fatal("foreign session accepted")
	}
}

func TestProjectionConsumerReferenceStatesPreserveEvidenceBoundary(t *testing.T) {
	c, err := NewProviderProjectionContract([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	scope, data := projectionConsumerFixture()
	for _, state := range []string{"accepted", "pending", "failed", "completed", "unavailable"} {
		t.Run(state, func(t *testing.T) {
			var response provider.ProjectionResponse
			if err := json.Unmarshal(data, &response); err != nil {
				t.Fatal(err)
			}
			ref := provider.ProjectionReferenceAttribution{EventID: "33333333-3333-4333-8333-333333333333", State: state}
			if state == "completed" {
				ref.Eligible = true
				ref.Lineage = &provider.ProjectionReferenceLineage{CandidateID: "44444444-4444-4444-8444-444444444444", MemoryID: "55555555-5555-4555-8555-555555555555", VersionID: "66666666-6666-4666-8666-666666666666", ProvenanceID: "77777777-7777-4777-8777-777777777777", Version: 1}
			}
			if state == "unavailable" {
				ref.EventID = ""
				response.Record.References[0] = ref
			}
			response.Record.CurrentReferences[0] = ref
			encoded, _ := json.Marshal(response)
			out, err := c.Validate(encoded, scope, "provider-v1")
			if err != nil || out.Record.CurrentReferences[0].State != state || out.Record.FeedbackState != "pending" || out.Record.EvidenceKind != "runtime_reported_evidence" {
				t.Fatalf("reference attribution was rejected or conflated: %v", err)
			}
		})
	}
}

func TestProjectionConsumerRejectsUnboundedMetadata(t *testing.T) {
	c, err := NewProviderProjectionContract([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	scope, data := projectionConsumerFixture()
	bad := strings.Replace(string(data), `"request_id":"r"`, `"precedence_version":"`+strings.Repeat("a", 4096)+`","request_id":"r"`, 1)
	if _, err := c.Validate([]byte(bad), scope, "provider-v1"); err == nil {
		t.Fatal("metadata larger than the producer's 4096-byte allocation accepted")
	}
}

func TestProjectionConsumerClosedErrorsAndStatusMapping(t *testing.T) {
	c, err := NewProviderProjectionContract([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		status         int
		category, code string
		retry          bool
	}{
		{400, "validation", "invalid_request", false}, {400, "validation", "invalid_scope", false}, {400, "validation", "invalid_projection_scope", false},
		{403, "authentication", "unauthorized", false}, {403, "scope", "scope_denied", false}, {403, "scope", "runtime_binding_denied", false}, {403, "scope", "disclosure_denied", false},
		{409, "compatibility", "unsupported_projection_contract", false}, {409, "compatibility", "unsupported_schema", false}, {409, "conflict", "projection_conflict", false}, {409, "stale", "projection_stale", false},
		{410, "lifecycle", "projection_unavailable", false}, {413, "validation", "projection_limit_exceeded", false},
		{503, "dependency", "projection_dependency_unavailable", true}, {503, "dependency", "disclosure_unavailable", true}, {503, "dependency", "authorization_unavailable", true},
	} {
		t.Run(tc.code, func(t *testing.T) {
			data, _ := json.Marshal(map[string]any{"error": map[string]any{"category": tc.category, "code": tc.code, "message": "bounded failure", "retryable": tc.retry}})
			out, err := c.ValidateError(data, tc.status)
			if err != nil || string(out.Category) != tc.category || out.Code != tc.code {
				t.Fatalf("valid error rejected: %+v %v", out, err)
			}
			if _, err := c.ValidateError(data, 200); err == nil {
				t.Fatal("error accepted with success status")
			}
		})
	}
	valid := `{"error":{"category":"conflict","code":"projection_conflict","message":"bounded failure","retryable":false}}`
	for name, bad := range map[string]string{
		"unknown outer":     strings.Replace(valid, `{"error":`, `{"authority":true,"error":`, 1),
		"unknown nested":    strings.Replace(valid, `"category":`, `"task_state":"completed","category":`, 1),
		"category mismatch": strings.Replace(valid, `"conflict"`, `"scope"`, 1),
		"legacy category":   strings.Replace(valid, `"conflict"`, `"resync_required"`, 1),
		"unknown code":      strings.Replace(valid, `"projection_conflict"`, `"future_conflict"`, 1),
		"missing message":   strings.Replace(valid, `"message":"bounded failure",`, ``, 1),
		"null message":      strings.Replace(valid, `"bounded failure"`, `null`, 1),
		"empty message":     strings.Replace(valid, `"bounded failure"`, `""`, 1),
		"oversize message":  strings.Replace(valid, `bounded failure`, strings.Repeat("界", 86), 1),
		"wrong retry":       strings.Replace(valid, `false`, `true`, 1),
		"case alias":        strings.Replace(valid, `"category"`, `"Category"`, 1),
		"duplicate":         strings.Replace(valid, `"retryable":false`, `"retryable":false,"retryable":false`, 1),
		"invalid Unicode":   strings.Replace(valid, `bounded failure`, `\ud800`, 1),
		"trailing":          valid + `{}`,
		"oversize":          strings.Repeat(" ", 4096) + valid,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := c.ValidateError([]byte(bad), 409); err == nil {
				t.Fatal("malformed error accepted")
			}
		})
	}
	if _, err := c.ValidateError([]byte(valid), 410); err == nil {
		t.Fatal("code accepted with wrong failure status")
	}
}

func TestProjectionConsumerRequiresClosedNestedSchemas(t *testing.T) {
	for _, name := range []string{"ProviderProjectionInput", "ProviderProjectionHistoryInput", "ProviderProjectionReferenceSelector", "ProviderProjectionFeedback", "ProviderProjectionLineage", "ProviderProjectionAttribution", "ProviderProjectionRecord"} {
		t.Run(name, func(t *testing.T) {
			spec := openapi.SpecYAML()
			start := strings.Index(spec, "    "+name+":")
			if start < 0 {
				t.Fatal("missing schema fixture")
			}
			mutated := spec[:start] + strings.Replace(spec[start:], "additionalProperties: false", "additionalProperties: true", 1)
			if _, err := NewProviderProjectionContract([]byte(mutated)); err == nil {
				t.Fatal("open nested schema accepted")
			}
		})
	}
}

func TestProjectionConsumerRequiresExplicitCapabilityAndRoutes(t *testing.T) {
	c, err := NewProviderProjectionContract([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	capability, _ := json.Marshal(map[string]any{"projections": provider.ProjectionCapability(true)})
	if err = c.Negotiate(capability); err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"Projections", "PROJECTIONS"} {
		t.Run("aliased capability envelope "+alias, func(t *testing.T) {
			aliased := strings.Replace(string(capability), `"projections":`, `"`+alias+`":`, 1)
			if err := c.Negotiate([]byte(aliased)); err == nil || err.Error() != "provider projection failed-contract" {
				t.Fatalf("aliased capability envelope accepted: %v", err)
			}
		})
	}
	for _, bad := range []string{`{}`, strings.Replace(string(capability), `"enabled":true`, `"enabled":false`, 1), strings.Replace(string(capability), `"max_history":8`, `"max_history":null`, 1), strings.Replace(string(capability), `"max_history":8`, `"max_history":8,"extra":1`, 1), strings.Replace(string(capability), `projection-feedback-v1`, `projection-feedback-v0`, 1)} {
		if c.Negotiate([]byte(bad)) == nil {
			t.Fatal("capability fallback accepted")
		}
	}
	bad := strings.Replace(openapi.SpecYAML(), "schema: {$ref: '#/components/schemas/ProviderProjectionResponse'}", "schema: {$ref: '#/components/schemas/ProviderOperationResponse'}", 1)
	if _, err := NewProviderProjectionContract([]byte(bad)); err == nil {
		t.Fatal("legacy route fallback accepted")
	}
}

func TestProjectionConsumerNegotiatesCompleteCapabilityDocument(t *testing.T) {
	c, err := NewProviderProjectionContract([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	document := provider.Discover(provider.CapabilityInput{ProviderVersion: "provider-v1", SchemaVersion: "provider-v1", SchemaDigest: c.digest})
	document.Projections = provider.ProjectionCapability(true)
	full, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.NegotiateDocument(full); err != nil {
		t.Fatalf("complete served capability rejected: %v", err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"missing projections":         func(v map[string]any) { delete(v, "projections") },
		"missing full-document field": func(v map[string]any) { delete(v, "limits") },
		"projection alias":            func(v map[string]any) { v["Projections"] = v["projections"]; delete(v, "projections") },
		"document field alias":        func(v map[string]any) { v["Provider_Version"] = v["provider_version"]; delete(v, "provider_version") },
		"extra document field":        func(v map[string]any) { v["authority"] = true },
		"extra projection field":      func(v map[string]any) { v["projections"].(map[string]any)["task_state"] = "complete" },
		"nested projection alias": func(v map[string]any) {
			p := v["projections"].(map[string]any)
			p["Enabled"] = p["enabled"]
			delete(p, "enabled")
		},
		"extra nested document field": func(v map[string]any) { v["limits"].(map[string]any)["extra"] = 1 },
		"null projections":            func(v map[string]any) { v["projections"] = nil },
		"disabled projections":        func(v map[string]any) { v["projections"].(map[string]any)["enabled"] = false },
		"legacy projection version": func(v map[string]any) {
			v["projections"].(map[string]any)["contract_version"] = "turn-outcome-feedback-projection-v0"
		},
		"legacy generic operation fallback": func(v map[string]any) { v["projections"].(map[string]any)["operations"] = []any{"turn", "outcome"} },
	} {
		t.Run(name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(full, &value); err != nil {
				t.Fatal(err)
			}
			mutate(value)
			bad, _ := json.Marshal(value)
			if err := c.NegotiateDocument(bad); err == nil || err.Error() != "provider projection failed-contract" {
				t.Fatalf("malformed complete capability accepted: %v", err)
			}
		})
	}
	extracted, _ := json.Marshal(map[string]any{"projections": document.Projections})
	for name, data := range map[string][]byte{
		"extracted is not full document": extracted,
		"duplicate member":               []byte(strings.Replace(string(full), `"projections":`, `"provider_version":"duplicate","projections":`, 1)),
		"invalid Unicode":                []byte(strings.Replace(string(full), `"provider_version":"provider-v1"`, `"provider_version":"\ud800"`, 1)),
		"trailing JSON":                  append(append([]byte(nil), full...), []byte("{}")...),
	} {
		t.Run(name, func(t *testing.T) {
			if err := c.NegotiateDocument(data); err == nil {
				t.Fatal("noncanonical complete capability accepted")
			}
		})
	}
	if err := c.Negotiate(full); err == nil {
		t.Fatal("extracted entry point accepted full document")
	}
}

func TestProjectionConsumerRequestValidation(t *testing.T) {
	c, err := NewProviderProjectionContract([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	valid := `{"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1","idempotency_key":"key"},"input":{"contract_version":"turn-outcome-feedback-projection-v1","kind":"outcome","effect_id":"effect","source_revision":1,"occurred_at":"2026-10-16T00:00:00Z","reported_outcome":"unknown"}}`
	if _, err := c.ValidateRequest([]byte(valid), "provider-v1"); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"legacy input":                strings.Replace(valid, `"contract_version":"turn-outcome-feedback-projection-v1",`, "", 1),
		"foreign scope":               strings.Replace(valid, `"kind":"outcome"`, `"tenant":"foreign","kind":"outcome"`, 1),
		"missing key":                 strings.Replace(valid, `,"idempotency_key":"key"`, "", 1),
		"noncanonical metadata":       strings.Replace(valid, `"request_id":"r"`, `"request_id":" r "`, 1),
		"unsupported schema":          strings.Replace(valid, "provider-v1", "provider-v0", 1),
		"turn without run":            strings.Replace(valid, `"kind":"outcome"`, `"kind":"outcome","turn_id":"turn"`, 1),
		"feedback without references": strings.Replace(valid, `"kind":"outcome"`, `"kind":"feedback","feedback":{"signal":"useful","reason_code":"reason"}`, 1),
		"missing digest":              strings.Replace(valid, `"kind":"outcome"`, `"kind":"outcome","context_reference":"ctxc_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`, 1),
		"zero time":                   strings.Replace(valid, "2026-10-16T00:00:00Z", "0001-01-01T00:00:00Z", 1),
		"duplicate":                   strings.Replace(valid, `"source_revision":1`, `"source_revision":1,"source_revision":1`, 1),
		"case alias":                  strings.Replace(valid, `"effect_id"`, `"Effect_ID"`, 1),
		"invalid Unicode":             strings.Replace(valid, `"effect"`, `"\ud800"`, 1),
		"oversize":                    strings.Repeat(" ", 32768) + valid,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := c.ValidateRequest([]byte(data), "provider-v1"); err == nil || err.Error() != "provider projection failed-contract" {
				t.Fatalf("wanted bounded contract failure, got %v", err)
			}
		})
	}
	history := `{"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1"},"input":{"contract_version":"turn-outcome-feedback-projection-v1","kind":"outcome","effect_id":"effect","limit":8}}`
	if _, err := c.ValidateHistoryRequest([]byte(history), "provider-v1"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(history, `"limit":8`, `"limit":9`, 1),
		strings.Replace(history, `"limit":8`, `"limit":null`, 1),
		strings.Replace(history, `"effect_id"`, `"Effect_ID"`, 1),
		strings.Replace(history, "provider-v1", "provider-v0", 1),
		strings.Replace(history, `"limit":8`, `"limit":8,"task_state":"complete"`, 1),
		strings.Repeat(" ", 32768) + history,
	} {
		if _, err := c.ValidateHistoryRequest([]byte(bad), "provider-v1"); err == nil {
			t.Fatal("malformed history request accepted")
		}
	}
}

func TestProjectionConsumerHistoryRequiresBoundedCorrectionChain(t *testing.T) {
	c, err := NewProviderProjectionContract([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	scope, data := projectionConsumerFixture()
	var admitted provider.ProjectionResponse
	if err := json.Unmarshal(data, &admitted); err != nil {
		t.Fatal(err)
	}
	older := admitted.Record
	newer := older
	newer.ProjectionID = "44444444-4444-4444-8444-444444444444"
	newer.SourceRevision = 7
	newer.Disposition = "corrected"
	newer.SupersedesID = older.ProjectionID
	full := provider.ProjectionHistoryResponse{Metadata: admitted.Metadata, ContractVersion: admitted.ContractVersion, Scope: scope, Records: []provider.ProjectionRecord{newer, older}}
	for name, page := range map[string]provider.ProjectionHistoryResponse{
		"full":      full,
		"truncated": {Metadata: admitted.Metadata, ContractVersion: admitted.ContractVersion, Scope: scope, Records: []provider.ProjectionRecord{newer}, HasMore: true},
		"empty":     {Metadata: admitted.Metadata, ContractVersion: admitted.ContractVersion, Scope: scope, Records: []provider.ProjectionRecord{}},
	} {
		t.Run(name, func(t *testing.T) {
			encoded, _ := json.Marshal(page)
			limit := 2
			if page.HasMore {
				limit = 1
			}
			if _, err := c.ValidateHistory(encoded, scope, "provider-v1", limit); err != nil {
				t.Fatal(err)
			}
		})
	}
	for name, mutate := range map[string]func(*provider.ProjectionHistoryResponse){
		"missing oldest":   func(p *provider.ProjectionHistoryResponse) { p.Records = p.Records[:1] },
		"false truncation": func(p *provider.ProjectionHistoryResponse) { p.HasMore = true },
		"reverse order":    func(p *provider.ProjectionHistoryResponse) { p.Records[0], p.Records[1] = p.Records[1], p.Records[0] },
		"different logical identity": func(p *provider.ProjectionHistoryResponse) {
			p.Records[1].LogicalID = "55555555-5555-4555-8555-555555555555"
		},
		"broken supersession": func(p *provider.ProjectionHistoryResponse) {
			p.Records[0].SupersedesID = "55555555-5555-4555-8555-555555555555"
		},
		"duplicate projection identity": func(p *provider.ProjectionHistoryResponse) { p.Records[1].ProjectionID = p.Records[0].ProjectionID },
		"null records":                  func(p *provider.ProjectionHistoryResponse) { p.Records = nil },
		"foreign session":               func(p *provider.ProjectionHistoryResponse) { p.Scope.SessionID = "foreign" },
	} {
		t.Run(name, func(t *testing.T) {
			page := full
			page.Records = append([]provider.ProjectionRecord(nil), full.Records...)
			mutate(&page)
			encoded, _ := json.Marshal(page)
			if _, err := c.ValidateHistory(encoded, scope, "provider-v1", 2); err == nil {
				t.Fatal("invalid history accepted")
			}
		})
	}
}
