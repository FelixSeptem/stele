package assurance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

type contextLiveReport struct {
	Result              string   `json:"result"`
	Stage               string   `json:"stage"`
	SchemaVersion       string   `json:"schema_version"`
	OpenAPIDigest       string   `json:"openapi_digest"`
	ImageDigest         string   `json:"image_digest"`
	SourceRevision      string   `json:"source_revision"`
	SourceDigest        string   `json:"source_digest"`
	ScopeHash           string   `json:"scope_hash"`
	IdentityHash        string   `json:"identity_hash"`
	CitationHash        string   `json:"citation_hash"`
	GovernanceCompleted int      `json:"governance_completed"`
	Checks              []string `json:"checks"`
}

func contextLiveHash(v any) string {
	data, _ := json.Marshal(v)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Invoked only by the owned Compose verifier. Missing dependencies skip ordinary
// unit runs; the verifier requires a passed report and never treats a skip as proof.
func TestProviderContextFreshPostgresLive(t *testing.T) {
	base := os.Getenv("STELE_CONTEXT_VERIFY_URL")
	if base == "" {
		t.Skip("owned live fixture not configured")
	}
	stage := os.Getenv("STELE_CONTEXT_VERIFY_STAGE")
	reportPath := os.Getenv("STELE_CONTEXT_VERIFY_REPORT")
	dir := os.Getenv("STELE_CONTEXT_VERIFY_CREDENTIAL_DIR")
	if (stage != "initial" && stage != "restart") || reportPath == "" || dir == "" {
		t.Fatal("live verifier configuration missing")
	}
	scope := memory.Scope{Tenant: os.Getenv("STELE_AUTH_DEFAULT_TENANT"), Project: os.Getenv("STELE_AUTH_DEFAULT_PROJECT"), Namespace: os.Getenv("STELE_AUTH_DEFAULT_NAMESPACE")}
	if err := scope.Validate(); err != nil {
		t.Fatal("live exact scope missing")
	}
	credentials := func(name string) string {
		data, err := os.ReadFile(filepath.Join(dir, name+".credential"))
		if err != nil || len(data) == 0 {
			t.Fatal("live credential unavailable")
		}
		return strings.TrimSpace(string(data))
	}
	runtimeKey, adminKey := credentials("runtime"), credentials("admin")
	client := &http.Client{Timeout: 10 * time.Second}
	call := func(method, path, key string, body any, binding *provider.RuntimeBinding, foreign bool) ([]byte, int) {
		var data []byte
		if body != nil {
			var err error
			data, err = json.Marshal(body)
			if err != nil {
				t.Fatal("fixture encoding failed")
			}
		}
		req, err := http.NewRequest(method, base+path, bytes.NewReader(data))
		if err != nil {
			t.Fatal("fixture transport invalid")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", key)
		tenant := scope.Tenant
		if foreign {
			tenant += "-foreign"
		}
		req.Header.Set("X-Stele-Tenant", tenant)
		req.Header.Set("X-Stele-Project", scope.Project)
		req.Header.Set("X-Stele-Namespace", scope.Namespace)
		req.Header.Set("X-Stele-Actor", "provider-context-verifier")
		if binding != nil {
			req.Header.Set(provider.HeaderRuntimeBinding, binding.BindingID)
			req.Header.Set(provider.HeaderRuntimeSession, binding.SessionID)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal("fixture HTTP unavailable")
		}
		defer res.Body.Close()
		payload, err := io.ReadAll(io.LimitReader(res.Body, 4<<20+1))
		if err != nil || len(payload) > 4<<20 {
			t.Fatal("fixture response unavailable")
		}
		return payload, res.StatusCode
	}
	spec, status := call("GET", "/openapi.yaml", "", nil, nil, false)
	if status != 200 {
		t.Fatal("served OpenAPI unavailable")
	}
	contract, err := NewProviderContextContract(spec)
	if err != nil {
		t.Fatal(err)
	}
	local, err := DefaultProviderContextContract()
	if err != nil || local.Digest != contract.Digest {
		t.Fatal("image and repaired OpenAPI do not match")
	}
	capsData, status := call("GET", "/v1/provider/capabilities", runtimeKey, nil, nil, false)
	var caps provider.CapabilityDocument
	if status != 200 || json.Unmarshal(capsData, &caps) != nil || caps.SchemaDigest != "sha256:"+contract.Digest {
		t.Fatal("capability OpenAPI digest mismatch")
	}
	var binding provider.RuntimeBinding
	bindPath := filepath.Join(dir, "context-binding.json")
	if stage == "initial" {
		initData, initStatus := call("POST", "/v1/provider/runtime", runtimeKey, provider.RuntimeInitialization{Scope: scope, AgentID: "pc1-context-verifier", SessionID: "pc1-context-session"}, nil, false)
		if initStatus != 201 || json.Unmarshal(initData, &binding) != nil {
			t.Fatal("public runtime initialization failed")
		}
		if err := os.WriteFile(bindPath, initData, 0600); err != nil {
			t.Fatal("owned binding checkpoint failed")
		}
	} else {
		initData, err := os.ReadFile(bindPath)
		if err != nil || json.Unmarshal(initData, &binding) != nil || binding.Scope != scope {
			t.Fatal("durable runtime binding checkpoint unavailable")
		}
	}
	metadata := func(id string) provider.OperationMetadata {
		id = strings.ReplaceAll(id, "/", "-")
		return provider.OperationMetadata{RequestID: "pc1-" + stage + "-" + id, OperationID: "pc1-" + id, SchemaVersion: "provider-v1"}
	}
	read := func(path, prefix string) provider.ContextResponse {
		body := provider.ContextRequest{Metadata: metadata("read"), Input: provider.ContextInput{Query: "context fixture", Budget: 20, Path: path, PathPrefix: prefix, IncludeDiagnostics: true}}
		data, status := call("POST", "/v1/provider/context", runtimeKey, body, &binding, false)
		if status != 200 {
			t.Fatal("public context request failed")
		}
		result, err := contract.Validate(data, scope, "provider-v1")
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	flatten := func(r provider.ContextResponse) []provider.ContextItem {
		out := []provider.ContextItem{}
		for _, items := range [][]provider.ContextItem{r.Result.Profile, r.Result.RecentSession, r.Result.RecentEpisodes, r.Result.RelevantSummaries, r.Result.RelatedEntities} {
			out = append(out, items...)
		}
		return out
	}
	visiblePath, hiddenPath := "pc1/context/visible", "pc1/context/hidden"
	report := contextLiveReport{Result: "failed", Stage: stage, SchemaVersion: "provider-v1", OpenAPIDigest: contract.Digest, ImageDigest: os.Getenv("STELE_CONTEXT_VERIFY_IMAGE_DIGEST"), SourceRevision: os.Getenv("STELE_CONTEXT_VERIFY_SOURCE_REVISION"), SourceDigest: os.Getenv("STELE_CONTEXT_VERIFY_SOURCE_DIGEST"), ScopeHash: contextLiveHash(scope)}
	if !strings.HasPrefix(report.ImageDigest, "sha256:") || len(report.SourceRevision) != 40 || !validContextDigest(report.SourceDigest) {
		t.Fatal("pinned build provenance missing")
	}
	var previous contextLiveReport
	if stage == "restart" {
		data, err := os.ReadFile(reportPath)
		if err != nil || json.Unmarshal(data, &previous) != nil || previous.Result != "passed" || previous.Stage != "initial" || previous.OpenAPIDigest != report.OpenAPIDigest || previous.ImageDigest != report.ImageDigest || previous.SourceDigest != report.SourceDigest || previous.ScopeHash != report.ScopeHash {
			t.Fatal("initial live evidence missing or incompatible")
		}
		report.GovernanceCompleted = previous.GovernanceCompleted
	}
	defer func() {
		if t.Failed() {
			report.Result = "failed"
		}
		data, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(reportPath, data, 0600); err != nil {
			t.Error("bounded report write failed")
		}
	}()
	if stage == "initial" {
		for _, path := range []string{visiblePath, hiddenPath} {
			meta := metadata(path)
			meta.IdempotencyKey = "pc1-event-" + strings.ReplaceAll(path, "/", "-")
			body := map[string]any{"metadata": meta, "event_type": "conversation.message", "memory_path": path, "content": "Durable context fixture for provider contract verification.", "metadata_map": map[string]any{"fixture": "pc1"}}
			data, status := call("POST", "/v1/provider/events", runtimeKey, body, &binding, false)
			var event provider.IngestResult
			if status != 201 || json.Unmarshal(data, &event) != nil || event.EventID == "" {
				t.Fatal("public governed event write failed")
			}
			replay, status := call("POST", "/v1/provider/events", runtimeKey, body, &binding, false)
			var replayed provider.IngestResult
			if status != 201 || json.Unmarshal(replay, &replayed) != nil || replayed.EventID != event.EventID || !replayed.Replayed {
				t.Fatal("public event replay failed")
			}
			deadline := time.Now().Add(45 * time.Second)
			completed := false
			for time.Now().Before(deadline) {
				data, status = call("GET", "/v1/admin/governance/raw-events/"+event.EventID, adminKey, nil, nil, false)
				var raw governance.GovernanceRawEvent
				if status == 200 && json.Unmarshal(data, &raw) == nil && raw.State == governance.GovernanceRawEventStateProcessed && !raw.ProcessedAt.IsZero() {
					completed = true
					break
				}
				time.Sleep(250 * time.Millisecond)
			}
			if !completed {
				t.Fatal("governance completion missing within bound")
			}
			report.GovernanceCompleted++
			if len(flatten(read(path, ""))) == 0 {
				t.Fatal("governed canonical exact-path result missing")
			}
		}
		hidden := flatten(read(hiddenPath, ""))
		ids := map[string]bool{}
		for _, item := range hidden {
			ids[item.Memory.ID] = true
		}
		for id := range ids {
			_, status = call("POST", "/v1/admin/memories/"+id+":suppress", adminKey, map[string]string{"reason": "owned context lifecycle fixture"}, nil, false)
			if status != 200 {
				t.Fatal("public lifecycle fixture suppression failed")
			}
		}
	}
	exact := read(visiblePath, "")
	items := flatten(exact)
	if len(items) == 0 {
		t.Fatal("eligible exact-path context empty")
	}
	identities := []string{}
	for _, item := range items {
		if item.Memory.MemoryPath != visiblePath || item.Memory.Content == "" || len(item.Citations) == 0 {
			t.Fatal("selected context lost path, content, or provenance")
		}
		identities = append(identities, item.Memory.ID)
	}
	sort.Strings(identities)
	report.IdentityHash = contextLiveHash(identities)
	citations := []string{}
	for _, citation := range exact.Result.Citations {
		citations = append(citations, citation.MemoryID+"|"+citation.RawEventID+"|"+citation.Operation)
	}
	sort.Strings(citations)
	report.CitationHash = contextLiveHash(citations)
	if len(citations) == 0 || len(exact.Citations) == 0 {
		t.Fatal("context citations missing")
	}
	if stage == "restart" {
		meta := metadata(visiblePath)
		meta.RequestID = "pc1-initial-" + strings.ReplaceAll(visiblePath, "/", "-")
		meta.IdempotencyKey = "pc1-event-" + strings.ReplaceAll(visiblePath, "/", "-")
		body := map[string]any{"metadata": meta, "event_type": "conversation.message", "memory_path": visiblePath, "content": "Durable context fixture for provider contract verification.", "metadata_map": map[string]any{"fixture": "pc1"}}
		data, replayStatus := call("POST", "/v1/provider/events", runtimeKey, body, &binding, false)
		var replayed provider.IngestResult
		if replayStatus != 201 || json.Unmarshal(data, &replayed) != nil || !replayed.Replayed {
			t.Fatal("event replay did not survive restart")
		}
		found := false
		for _, citation := range exact.Result.Citations {
			if citation.RawEventID == replayed.EventID {
				found = true
			}
		}
		if !found {
			t.Fatal("restart replay lost original event provenance")
		}
		data, governanceStatus := call("GET", "/v1/admin/governance/raw-events/"+replayed.EventID, adminKey, nil, nil, false)
		var raw governance.GovernanceRawEvent
		if governanceStatus != 200 || json.Unmarshal(data, &raw) != nil || raw.State != governance.GovernanceRawEventStateProcessed || raw.ProcessedAt.IsZero() {
			t.Fatal("processed event did not survive restart")
		}
	}
	prefix := read("", "pc1/context")
	if len(flatten(prefix)) == 0 {
		t.Fatal("explicit prefix context empty")
	}
	for _, item := range flatten(prefix) {
		if item.Memory.MemoryPath != visiblePath {
			t.Fatal("prefix leaked hidden or unrelated path")
		}
	}
	if len(flatten(read("pc1/context/absent", ""))) != 0 || len(flatten(read(hiddenPath, ""))) != 0 {
		t.Fatal("empty or lifecycle fixture returned memory")
	}
	_, status = call("POST", "/v1/provider/context", runtimeKey, provider.ContextRequest{Metadata: metadata("foreign"), Input: provider.ContextInput{Query: "context fixture", Budget: 2}}, &binding, true)
	if status != 403 {
		t.Fatal("foreign scope was not denied")
	}
	report.Checks = []string{"public_http_schema", "capability_digest", "governance_completed", "event_replay", "nonroot_exact_path", "explicit_prefix", "genuine_empty", "foreign_scope_denied", "lifecycle_excluded", "citations_present"}
	if stage == "restart" {
		if previous.IdentityHash != report.IdentityHash || previous.CitationHash != report.CitationHash {
			t.Fatal("restart changed context identities or provenance")
		}
		report.Checks = append(report.Checks, "api_worker_restart_identity")
	}
	report.Result = "passed"
}
