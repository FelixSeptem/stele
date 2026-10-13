package assurance

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

func TestIndependentContextDigestVectors(t *testing.T) {
	contract, err := DefaultProviderContextContract()
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("../provider/testdata/context-digest-v1/*.response.json")
	if err != nil || len(files) != 4 {
		t.Fatal("four canonical vectors required")
	}
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := contract.Validate(data, scope, "provider-v1"); err != nil {
				t.Fatal("independent consumer disagrees with vector", err)
			}
		})
	}
}

func TestContextConsumerRejectsForgedDigestAndContinuity(t *testing.T) {
	c, err := DefaultProviderContextContract()
	if err != nil {
		t.Fatal(err)
	}
	scope, r := contextContractFixture()
	data, _ := json.Marshal(r)
	for name, mutate := range map[string]func(map[string]any){"digest missing": func(v map[string]any) { delete(v, "context_digest") }, "hash": func(v map[string]any) { v["context_digest"].(map[string]any)["value"] = strings.Repeat("a", 64) }, "version": func(v map[string]any) { v["context_digest"].(map[string]any)["contract_version"] = "context-digest-v0" }, "policy": func(v map[string]any) {
		v["context_digest"].(map[string]any)["selection_policy_fingerprint"] = strings.Repeat("b", 64)
	}, "continuity missing": func(v map[string]any) { delete(v, "continuity") }, "false unchanged": func(v map[string]any) { v["continuity"].(map[string]any)["outcome"] = "unchanged" }, "not requested reference": func(v map[string]any) {
		v["continuity"].(map[string]any)["reference"] = "ctxc_0123456789abcdef0123456789abcdef"
	}} {
		t.Run(name, func(t *testing.T) {
			var doc map[string]any
			_ = json.Unmarshal(data, &doc)
			mutate(doc["result"].(map[string]any))
			bad, _ := json.Marshal(doc)
			if _, err := c.Validate(bad, scope, "provider-v1"); err == nil || err.Error() != "provider context failed-contract" {
				t.Fatal("forged context passed")
			}
		})
	}
	for _, bad := range [][]byte{bytes.Replace(data, []byte(`"request_id":"r"`), []byte(`"request_id":"r","request_id":"r"`), 1), bytes.Replace(data, []byte(`"request_id":"r"`), []byte(`"request_id":"\ud800"`), 1)} {
		if _, err := c.Validate(bad, scope, "provider-v1"); err == nil {
			t.Fatal("ambiguous/lossy JSON passed")
		}
	}
	if _, err := c.ValidateForRequest(data, scope, "provider-v1", provider.ContextInput{Budget: 1, Continuity: &provider.ContextContinuityInput{Mode: "issue"}}, nil); err == nil {
		t.Fatal("missing issued receipt treated as success")
	}
}
