package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func identityFixture(t *testing.T, mode string, nonempty bool) (memory.Scope, DisclosedContextResponse) {
	t.Helper()
	scope := memory.Scope{Tenant: "t", Project: "p", Namespace: "n"}
	decision, err := (auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileContent}, DefaultProfile: auth.DisclosureProfileContent}).Resolve("", mode)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewContextAdmission(ContextInput{Query: "query", Budget: 8}, Discover(CapabilityInput{}).Limits, scope, decision, OperationMetadata{RequestID: "r", OperationID: "o", SchemaVersion: "provider-v1"})
	if err != nil {
		t.Fatal(err)
	}
	out := retrieval.AssembledContext{}
	if nonempty {
		hit := retrieval.SearchHit{Memory: memory.CanonicalMemory{ID: "m", Scope: scope, MemoryPath: "tasks/one", Class: memory.MemoryClassEpisodic, State: memory.MemoryStateActive, Content: "中文🙂 <&> \" \\ \n \u2028 \u2029", SourceVersion: 1, SourceMetadata: memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}}}
		for _, cat := range []string{"profile", "recent_session", "recent_episodes"} {
			if !a.AdmitContextItem(cat, hit) {
				t.Fatal("placement not admitted")
			}
		}
		out.Profile = []retrieval.SearchHit{hit}
		out.RecentSession = []retrieval.SearchHit{hit}
		out.RecentEpisodes = []retrieval.SearchHit{hit}
	}
	data, err := a.Finalize(out, OperationMetadata{RequestID: "r", OperationID: "o", SchemaVersion: "provider-v1"})
	if err != nil {
		t.Fatal(err)
	}
	var response DisclosedContextResponse
	if json.Unmarshal(data, &response) != nil {
		t.Fatal("decode")
	}
	return scope, response
}

func TestContextCanonicalGoldenVectors(t *testing.T) {
	for _, mode := range []string{auth.ResponseModeReferenceOnly, auth.ResponseModeBoundedContent} {
		for _, full := range []bool{false, true} {
			name := mode + "-empty"
			if full {
				name = mode + "-nonempty"
			}
			t.Run(name, func(t *testing.T) {
				scope, r := identityFixture(t, mode, full)
				canonical, err := CanonicalContextBytes(scope, r, r.Result.ContextDigest.SelectionPolicyFingerprint)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join("testdata", "context-digest-v1", name+".json")
				responsePath := filepath.Join("testdata", "context-digest-v1", name+".response.json")
				if os.Getenv("STELE_UPDATE_CONTEXT_VECTORS") == "1" {
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, canonical, 0644); err != nil {
						t.Fatal(err)
					}
					data, _ := json.Marshal(r)
					if err := os.WriteFile(responsePath, data, 0644); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path+".sha256", []byte(r.Result.ContextDigest.Value+"\n"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				golden, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(canonical, golden) {
					t.Fatal("canonical contract byte vector changed")
				}
				hash, err := os.ReadFile(path + ".sha256")
				if err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256(append([]byte("stele-context-digest-v1\n"), golden...))
				if strings.TrimSpace(string(hash)) != hex.EncodeToString(sum[:]) || r.Result.ContextDigest.Value != strings.TrimSpace(string(hash)) {
					t.Fatal("digest vector changed")
				}
			})
		}
	}
}

func TestContextDigestProjectionInvariantsAndChanges(t *testing.T) {
	scope, r := identityFixture(t, auth.ResponseModeBoundedContent, true)
	fp := r.Result.ContextDigest.SelectionPolicyFingerprint
	base := r.Result.ContextDigest.Value
	clone := func() DisclosedContextResponse {
		data, _ := json.Marshal(r)
		var x DisclosedContextResponse
		_ = json.Unmarshal(data, &x)
		return x
	}
	for _, mutate := range []func(*DisclosedContextResponse){func(x *DisclosedContextResponse) { x.Metadata.RequestID = "other"; x.Metadata.OperationID = "other" }, func(x *DisclosedContextResponse) {
		x.Result.BudgetAccounting.Usage.ResponseBytes = 999
		x.Result.BudgetAccounting.Requested.MaxItems = 999
	}, func(x *DisclosedContextResponse) {
		x.Result.Continuity.Reference = "ctxc_0123456789abcdef0123456789abcdef"
		x.Result.Continuity.Outcome = "issued"
	}} {
		x := clone()
		mutate(&x)
		d, err := ComputeContextDigest(scope, x, fp)
		if err != nil || d.Value != base {
			t.Fatal("volatile metadata changed projection")
		}
	}
	for name, mutate := range map[string]func(*DisclosedContextResponse){"version": func(x *DisclosedContextResponse) { x.Result.Profile[0].Memory.Version = "2" }, "path": func(x *DisclosedContextResponse) { x.Result.Profile[0].Memory.MemoryPath = "tasks/two" }, "content": func(x *DisclosedContextResponse) { x.Result.Profile[0].Memory.Content += "changed" }, "trust": func(x *DisclosedContextResponse) {
		x.Result.Profile[0].Memory.SourceMetadata.Trust = memory.SourceTrustAttestedData
	}, "placement": func(x *DisclosedContextResponse) { x.Result.Profile = []DisclosedContextItem{} }, "intent": func(x *DisclosedContextResponse) { x.Result.Disclosure.ProjectionIntent = "compiler" }, "budget": func(x *DisclosedContextResponse) { x.Result.BudgetAccounting.Effective.MaxItems-- }, "citation": func(x *DisclosedContextResponse) { x.Result.Profile[0].Citations[0].Version = "2" }} {
		t.Run(name, func(t *testing.T) {
			x := clone()
			mutate(&x)
			d, err := ComputeContextDigest(scope, x, fp)
			if err != nil || d.Value == base {
				t.Fatal("visible semantic change did not change identity")
			}
		})
	}
	other := scope
	other.Namespace = "other"
	d, _ := ComputeContextDigest(other, r, fp)
	if d.Value == base {
		t.Fatal("scope not bound")
	}
	d, _ = ComputeContextDigest(scope, r, strings.Repeat("a", 64))
	if d.Value == base {
		t.Fatal("policy not bound")
	}
	r.Citations = append(r.Citations, Citation{Reference: "foreign"})
	if _, err := ComputeContextDigest(scope, r, fp); err == nil {
		t.Fatal("citation disagreement hashed")
	}
}

func TestContextReadFingerprintBindsEffectiveReadAndIdentity(t *testing.T) {
	scope, r := identityFixture(t, auth.ResponseModeReferenceOnly, false)
	b := RuntimeBinding{BindingID: "b", PrincipalID: "id_public", Scope: scope, AgentID: "a", SessionID: "s", ConversationID: "c", ProviderInstanceID: "pi"}
	input := ContextInput{Query: "中文", Budget: 8}
	base, err := ContextReadFingerprint(b, input, r)
	if err != nil {
		t.Fatal(err)
	}
	input.PathPrefix = "/"
	same, _ := ContextReadFingerprint(b, input, r)
	if same != base {
		t.Fatal("default root prefix differs")
	}
	input.Continuity = &ContextContinuityInput{Mode: "issue"}
	same, _ = ContextReadFingerprint(b, input, r)
	if same != base {
		t.Fatal("continuity changed originating read")
	}
	b.CreatedAt = time.Now()
	same, _ = ContextReadFingerprint(b, input, r)
	if same != base {
		t.Fatal("timestamps entered fingerprint")
	}
	for _, change := range []func(*RuntimeBinding, *ContextInput){func(b *RuntimeBinding, i *ContextInput) { b.PrincipalID = "other" }, func(b *RuntimeBinding, i *ContextInput) { b.SessionID = "other" }, func(b *RuntimeBinding, i *ContextInput) { b.BindingID = "other" }, func(b *RuntimeBinding, i *ContextInput) { b.Scope.Tenant = "other" }, func(b *RuntimeBinding, i *ContextInput) { b.AgentID = "other" }, func(b *RuntimeBinding, i *ContextInput) { b.ConversationID = "other" }, func(b *RuntimeBinding, i *ContextInput) { b.ProviderInstanceID = "other" }, func(b *RuntimeBinding, i *ContextInput) { i.Query += " " }, func(b *RuntimeBinding, i *ContextInput) { i.PathPrefix = ""; i.Path = "/" }, func(b *RuntimeBinding, i *ContextInput) { i.IncludeRelations = true }} {
		x, y := b, input
		change(&x, &y)
		v, _ := ContextReadFingerprint(x, y, r)
		if v == base {
			t.Fatal("changed originating read accepted")
		}
	}
}

func TestContextUnicodeExactValidation(t *testing.T) {
	for _, data := range []string{`{"q":"\ud83d\ude42"}`, `{"q":"中文🙂"}`, `{"q":"\\ud800"}`, `{"q":"quote\"ok"}`} {
		if !ValidJSONUnicode([]byte(data)) {
			t.Fatal("valid Unicode rejected")
		}
	}
	for _, data := range []string{`{"q":"\ud800"}`, `{"q":"\udc00"}`, `{"q":"\ud800\u0041"}`, "{\"q\":\"" + string([]byte{0xff}) + "\"}"} {
		if ValidJSONUnicode([]byte(data)) {
			t.Fatal("invalid Unicode accepted")
		}
	}
}
