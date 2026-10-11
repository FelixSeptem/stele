package provider

import (
	"encoding/json"
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"strings"
	"testing"
)

func TestContextDisclosureDecoderAndReferenceProjection(t *testing.T) {
	prefix := `{"metadata":{"request_id":"req","operation_id":"op","schema_version":"provider-v1"},"input":`
	valid := `{"query":"q","budget":2,"projection_intent":"compilation","disclosure_profile":"memory-reference-v1","response_mode":"reference_only"}`
	req, err := DecodeContextRequest([]byte(prefix + valid + `}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = req.Input.RetrievalInput(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"projection_intent":"admin"`, `"response_mode":"text"`, `"disclosure_profile":"has space"`, `"response_mode":null`, `"ResponseMode":"reference_only"`} {
		in := `{"query":"q","budget":2,` + field + `}`
		r, err := DecodeContextRequest([]byte(prefix + in + `}`))
		if err == nil {
			_, err = r.Input.RetrievalInput(memory.Scope{Tenant: "t", Project: "p", Namespace: "n"})
		}
		if err == nil {
			t.Fatalf("accepted %s", field)
		}
	}
	source := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}
	hit := retrieval.SearchHit{Memory: memory.CanonicalMemory{ID: "m", Scope: memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}, Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "SECRET-TEXT", MemoryPath: "tasks/a", SourceMetadata: source, SourceVersion: 3}}
	decision, _ := auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileReference}, DefaultProfile: auth.DisclosureProfileReference}.Resolve("", "")
	response := ShapeDisclosedContextResponse(retrieval.AssembledContext{RecentEpisodes: []retrieval.SearchHit{hit}}, OperationMetadata{}, decision, 10)
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"SECRET-TEXT", `"content"`, `"memory":`, `"title"`, `"summary"`, `"goal_context"`, `"diagnostics"`} {
		if strings.Contains(string(raw), text) {
			t.Fatalf("reference leaked %s", text)
		}
	}
	if !strings.Contains(string(raw), `"version":"3"`) || !strings.Contains(string(raw), `"response_mode":"reference_only"`) {
		t.Fatalf("missing identity/mode: %s", raw)
	}
}
