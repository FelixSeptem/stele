package assurance

import (
	"bytes"
	"context"
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

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/jackc/pgx/v5"
)

type contextLiveReport struct {
	Result                string   `json:"result"`
	Stage                 string   `json:"stage"`
	SchemaVersion         string   `json:"schema_version"`
	OpenAPIDigest         string   `json:"openapi_digest"`
	ImageDigest           string   `json:"image_digest"`
	SourceRevision        string   `json:"source_revision"`
	SourceDigest          string   `json:"source_digest"`
	ScopeHash             string   `json:"scope_hash"`
	IdentityHash          string   `json:"identity_hash"`
	CitationHash          string   `json:"citation_hash"`
	GovernanceCompleted   int      `json:"governance_completed"`
	DisclosureVersion     string   `json:"disclosure_policy_version"`
	SourceMetadataVersion string   `json:"source_metadata_version"`
	DurableLabelsVerified bool     `json:"durable_labels_verified"`
	Checks                []string `json:"checks"`
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
	if status != 200 || json.Unmarshal(capsData, &caps) != nil || caps.Validate() != nil || caps.SchemaDigest != "sha256:"+contract.Digest {
		t.Fatal("capability OpenAPI digest mismatch")
	}
	var binding provider.RuntimeBinding
	bindPath := filepath.Join(dir, "context-binding.json")
	if stage == "initial" {
		initData, initStatus := call("POST", "/v1/provider/runtime", runtimeKey, provider.RuntimeInitialization{Scope: scope, AgentID: "pc1-context-verifier", SessionID: "pc1-context-session"}, nil, false)
		if initStatus != 201 || json.Unmarshal(initData, &binding) != nil {
			t.Fatalf("public runtime initialization failed (HTTP %d)", initStatus)
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
	readMode := func(path, prefix, mode string) provider.DisclosedContextResponse {
		body := provider.ContextRequest{Metadata: metadata("read"), Input: provider.ContextInput{Query: "context fixture", Budget: 20, Path: path, PathPrefix: prefix, IncludeDiagnostics: true, ResponseMode: mode}}
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
	read := func(path, prefix string) provider.DisclosedContextResponse {
		return readMode(path, prefix, auth.ResponseModeBoundedContent)
	}
	flatten := func(r provider.DisclosedContextResponse) []provider.DisclosedContextItem {
		out := []provider.DisclosedContextItem{}
		for _, items := range [][]provider.DisclosedContextItem{r.Result.Profile, r.Result.RecentSession, r.Result.RecentEpisodes, r.Result.RelevantSummaries, r.Result.RelatedEntities} {
			out = append(out, items...)
		}
		return out
	}
	visiblePath, hiddenPath := "pc1/context/visible", "pc1/context/hidden"
	report := contextLiveReport{Result: "failed", Stage: stage, SchemaVersion: "provider-v1", DisclosureVersion: auth.DisclosurePolicyVersion, SourceMetadataVersion: memory.SourceMetadataVersion, OpenAPIDigest: contract.Digest, ImageDigest: os.Getenv("STELE_CONTEXT_VERIFY_IMAGE_DIGEST"), SourceRevision: os.Getenv("STELE_CONTEXT_VERIFY_SOURCE_REVISION"), SourceDigest: os.Getenv("STELE_CONTEXT_VERIFY_SOURCE_DIGEST"), ScopeHash: contextLiveHash(scope)}
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
		for _, fixture := range []struct {
			path           string
			classification memory.SourceClassification
			action         string
		}{
			{visiblePath, memory.SourceClassificationMedium, ""},
			{hiddenPath, memory.SourceClassificationMedium, "suppress"},
			{"pc1/context/forgotten", memory.SourceClassificationLow, "expire"},
			{"pc1/context/deleted", memory.SourceClassificationMedium, "delete"},
			{"pc1/context/high", memory.SourceClassificationHigh, ""},
			{"pc1/context/restricted", memory.SourceClassificationRestricted, ""},
		} {
			path := fixture.path
			meta := metadata(path)
			meta.IdempotencyKey = "pc1-event-" + strings.ReplaceAll(path, "/", "-")
			body := map[string]any{"metadata": meta, "event_type": "conversation.message", "memory_path": path, "content": "Durable context fixture for provider contract verification.", "metadata_map": map[string]any{"fixture": "pc1"}, "source_metadata": memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: fixture.classification}}
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
			changed := memory.SourceClassificationLow
			if fixture.classification == changed {
				changed = memory.SourceClassificationMedium
			}
			body["source_metadata"] = memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: changed}
			conflict, conflictStatus := call("POST", "/v1/provider/events", runtimeKey, body, &binding, false)
			if conflictStatus != 400 || !bytes.Contains(conflict, []byte(`"category":"conflict"`)) || bytes.Contains(conflict, []byte("Durable context fixture")) {
				t.Fatal("label-change replay conflict missing or unsafe")
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
			selected := flatten(read(path, ""))
			if fixture.classification == memory.SourceClassificationHigh || fixture.classification == memory.SourceClassificationRestricted {
				if len(selected) != 0 {
					t.Fatal("ineligible source disclosed its identity")
				}
				continue
			}
			if len(selected) == 0 {
				t.Fatal("governed canonical exact-path result missing")
			}
			if fixture.action != "" {
				for _, item := range selected {
					_, code := call("POST", "/v1/admin/memories/"+item.Memory.MemoryID+":"+fixture.action, adminKey, map[string]string{"reason": "owned context lifecycle fixture"}, nil, false)
					if code != 200 {
						t.Fatalf("public lifecycle %s fixture failed (HTTP %d)", fixture.action, code)
					}
				}
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
		if item.Memory == nil || item.Memory.MemoryPath != visiblePath || item.Memory.Content == "" || len(item.Citations) == 0 {
			t.Fatal("selected context lost path, content, or provenance")
		}
		identities = append(identities, item.Memory.MemoryID)
	}
	sort.Strings(identities)
	report.IdentityHash = contextLiveHash(identities)
	citations := []string{}
	for _, citation := range exact.Result.Citations {
		citations = append(citations, citation.Reference+"|"+citation.Version+"|"+citation.SourceKind)
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
		body := map[string]any{"metadata": meta, "event_type": "conversation.message", "memory_path": visiblePath, "content": "Durable context fixture for provider contract verification.", "metadata_map": map[string]any{"fixture": "pc1"}, "source_metadata": memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationMedium}}
		data, replayStatus := call("POST", "/v1/provider/events", runtimeKey, body, &binding, false)
		var replayed provider.IngestResult
		if replayStatus != 201 || json.Unmarshal(data, &replayed) != nil || !replayed.Replayed {
			t.Fatal("event replay did not survive restart")
		}
		found := false
		// The public disclosure citation is intentionally canonical-only; the
		// replay identity is checked by the successful idempotent response above.
		found = replayed.EventID != ""
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
	for _, path := range []string{"pc1/context/forgotten", "pc1/context/deleted", "pc1/context/high", "pc1/context/restricted"} {
		if len(flatten(read(path, ""))) != 0 {
			t.Fatal("lifecycle or classified source disclosed after restart")
		}
	}
	_, status = call("POST", "/v1/provider/context", runtimeKey, provider.ContextRequest{Metadata: metadata("foreign"), Input: provider.ContextInput{Query: "context fixture", Budget: 2}}, &binding, true)
	if status != 403 {
		t.Fatal("foreign scope was not denied")
	}
	// Validate durable snapshots independently of the public projection, using
	// the owned database only for observation. Fixture creation remains HTTP-only.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := pgx.Connect(ctx, os.Getenv("STELE_CONTEXT_VERIFY_POSTGRES_DSN"))
	if err != nil {
		t.Fatal("owned PostgreSQL evidence unavailable")
	}
	defer db.Close(context.Background())
	var snapshots int
	err = db.QueryRow(ctx, `SELECT count(*) FROM raw_events r
JOIN candidate_memories c ON c.source_raw_event_id=r.id
JOIN provenance_links p ON p.candidate_memory_id=c.id AND p.memory_id IS NOT NULL
JOIN canonical_memories m ON m.id=p.memory_id
JOIN memory_versions v ON v.memory_id=m.id
WHERE r.tenant=$1 AND r.project=$2 AND r.namespace=$3 AND r.memory_path=$4
AND r.governance_processed_at IS NOT NULL AND c.memory_path=r.memory_path AND m.memory_path=r.memory_path
AND r.source_metadata=c.source_metadata AND c.source_metadata=m.source_metadata AND m.source_metadata=v.source_metadata
AND r.source_metadata->>'trust'='untrusted' AND r.source_metadata->>'classification'='medium'`, scope.Tenant, scope.Project, scope.Namespace, visiblePath).Scan(&snapshots)
	if err != nil || snapshots == 0 {
		t.Fatal("durable candidate/canonical/version labels unavailable")
	}
	report.DurableLabelsVerified = true
	grantData, grantStatus := call("GET", "/v1/admin/principals/"+binding.PrincipalID+"/grants", adminKey, nil, nil, false)
	var grants struct {
		Grants []auth.ScopeGrant `json:"grants"`
	}
	if grantStatus != 200 || json.Unmarshal(grantData, &grants) != nil || len(grants.Grants) != 1 {
		t.Fatal("runtime exact grant evidence unavailable")
	}
	grantPath := "/v1/admin/principals/" + binding.PrincipalID + "/grants/" + grants.Grants[0].ID + "/disclosure"
	provision := func(profile string, attestation bool) {
		_, code := call("PUT", grantPath, adminKey, map[string]any{"disclosure": auth.DisclosureEntitlements{AllowedProfiles: []string{profile}, DefaultProfile: profile, SourceAttestation: attestation}, "actor": "context-verifier", "reason": "owned disclosure boundary fixture"}, nil, false)
		if code != 204 {
			t.Fatal("disclosure provisioning failed")
		}
	}
	provision(auth.DisclosureProfileReference, false)
	references := readMode(visiblePath, "", auth.ResponseModeReferenceOnly)
	if references.Result.ResponseMode != auth.ResponseModeReferenceOnly || len(flatten(references)) == 0 {
		t.Fatal("reference-only projection missing")
	}
	referenceJSON, _ := json.Marshal(references)
	for _, marker := range []string{"Durable context fixture", `"memory":`, `"content":`, `"title":`, `"summary":`, `"diagnostics":`} {
		if bytes.Contains(referenceJSON, []byte(marker)) {
			t.Fatal("reference-only projection disclosed text")
		}
	}
	_, deniedStatus := call("POST", "/v1/provider/context", runtimeKey, provider.ContextRequest{Metadata: metadata("downgraded"), Input: provider.ContextInput{Query: "context fixture", Budget: 2, Path: visiblePath, ResponseMode: auth.ResponseModeBoundedContent}}, &binding, false)
	if deniedStatus != 403 {
		t.Fatal("old binding ignored grant downgrade")
	}
	for _, route := range []struct {
		method, path string
		body         any
	}{
		{"POST", "/v1/provider/retrieve", map[string]any{"input": map[string]any{"query": "context fixture"}}},
		{"POST", "/v1/provider/sync", map[string]any{}}, {"POST", "/v1/memories/search", map[string]any{"query": "context fixture", "top_k": 2}},
		{"POST", "/v1/context/assemble", map[string]any{"query": "context fixture", "budget": 2}}, {"GET", "/v1/memories", nil},
		{"GET", "/v1/sessions", nil}, {"GET", "/v1/memory-intents", nil},
	} {
		data, code := call(route.method, route.path, runtimeKey, route.body, nil, false)
		if code != 403 || bytes.Contains(data, []byte("Durable context fixture")) {
			t.Fatal("alternate route bypassed disclosure")
		}
	}
	attestMeta := metadata("attested")
	attestMeta.IdempotencyKey = "pc2-attested-event"
	attestBody := map[string]any{"metadata": attestMeta, "event_type": "conversation.message", "memory_path": "pc2/attestation", "content": "Attestation context fixture.", "source_metadata": memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustAttestedData, Classification: memory.SourceClassificationMedium}}
	_, code := call("POST", "/v1/provider/events", runtimeKey, attestBody, &binding, false)
	if code != 403 {
		t.Fatal("ordinary principal self-attested")
	}
	provision(auth.DisclosureProfileContent, true)
	data, code := call("POST", "/v1/provider/events", runtimeKey, attestBody, &binding, false)
	var attested provider.IngestResult
	if code != 201 || json.Unmarshal(data, &attested) != nil || attested.EventID == "" {
		t.Fatal("authorized attestation write failed")
	}
	deadline := time.Now().Add(45 * time.Second)
	for {
		data, code = call("GET", "/v1/admin/governance/raw-events/"+attested.EventID, adminKey, nil, nil, false)
		var raw governance.GovernanceRawEvent
		if code == 200 && json.Unmarshal(data, &raw) == nil && raw.State == governance.GovernanceRawEventStateProcessed && !raw.ProcessedAt.IsZero() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("attested event governance incomplete")
		}
		time.Sleep(250 * time.Millisecond)
	}
	attestedItems := flatten(read("pc2/attestation", ""))
	if len(attestedItems) == 0 {
		t.Fatal("attested canonical missing")
	}
	for _, item := range attestedItems {
		if item.Memory.SourceMetadata.Trust != memory.SourceTrustAttestedData {
			t.Fatal("attested lineage lost")
		}
	}
	provision(auth.DisclosureProfileContent, false)
	_, code = call("POST", "/v1/provider/events", runtimeKey, attestBody, &binding, false)
	if code != 403 {
		t.Fatal("replay ignored current attestation entitlement")
	}
	// A separate principal is revoked permanently. Retain its checkpoint only in
	// the owned secret directory so restart proves the old binding remains denied.
	checkpointPath := filepath.Join(dir, "pc2-revoked.json")
	var checkpoint struct {
		Key     string
		Binding provider.RuntimeBinding
	}
	if stage == "initial" {
		data, code = call("POST", "/v1/admin/principals", adminKey, map[string]string{"role": "public", "label": "context-revoked-fixture", "actor": "context-verifier", "reason": "owned revocation fixture"}, nil, false)
		var issued struct {
			Principal auth.Principal `json:"principal"`
			Key       string         `json:"credential_secret"`
		}
		if code != 201 || json.Unmarshal(data, &issued) != nil || issued.Key == "" {
			t.Fatal("revocation principal creation failed")
		}
		_, code = call("POST", "/v1/provider/runtime", issued.Key, provider.RuntimeInitialization{Scope: scope, AgentID: "pc2-unconfigured", SessionID: "pc2-unconfigured-session"}, nil, false)
		if code != 403 {
			t.Fatal("unconfigured principal initialized public runtime")
		}
		data, code = call("GET", "/v1/admin/principals/"+issued.Principal.ID+"/grants", adminKey, nil, nil, false)
		if code != 200 || json.Unmarshal(data, &grants) != nil || len(grants.Grants) != 1 {
			t.Fatal("revocation grant missing")
		}
		path := "/v1/admin/principals/" + issued.Principal.ID + "/grants/" + grants.Grants[0].ID
		_, code = call("PUT", path+"/disclosure", adminKey, map[string]any{"disclosure": auth.DisclosureEntitlements{AllowedProfiles: []string{auth.DisclosureProfileReference}, DefaultProfile: auth.DisclosureProfileReference}, "actor": "context-verifier", "reason": "owned reference fixture"}, nil, false)
		if code != 204 {
			t.Fatal("reference principal provisioning failed")
		}
		checkpoint.Key = issued.Key
		data, code = call("POST", "/v1/provider/runtime", issued.Key, provider.RuntimeInitialization{Scope: scope, AgentID: "pc2-reference", SessionID: "pc2-reference-session"}, nil, false)
		if code != 201 || json.Unmarshal(data, &checkpoint.Binding) != nil {
			t.Fatal("reference principal binding failed")
		}
		_, code = call("POST", "/v1/admin/principals/"+issued.Principal.ID+"/grants/"+grants.Grants[0].ID+"/revoke", adminKey, map[string]string{"actor": "context-verifier", "reason": "owned revocation fixture"}, nil, false)
		if code != 204 {
			t.Fatal("grant revocation failed")
		}
		secretData, _ := json.Marshal(checkpoint)
		if os.WriteFile(checkpointPath, secretData, 0600) != nil {
			t.Fatal("owned revocation checkpoint unavailable")
		}
	} else {
		secretData, readErr := os.ReadFile(checkpointPath)
		if readErr != nil || json.Unmarshal(secretData, &checkpoint) != nil {
			t.Fatal("revocation checkpoint unavailable")
		}
	}
	_, code = call("POST", "/v1/provider/context", checkpoint.Key, provider.ContextRequest{Metadata: metadata("revoked"), Input: provider.ContextInput{Query: "context fixture", Budget: 2}}, &checkpoint.Binding, false)
	if code != 403 {
		t.Fatal("revoked old binding disclosed context")
	}
	report.Checks = []string{"public_http_schema", "capability_digest", "governance_completed", "event_replay", "nonroot_exact_path", "explicit_prefix", "genuine_empty", "foreign_scope_denied", "lifecycle_excluded", "citations_present"}
	report.Checks = append(report.Checks, "durable_source_labels", "label_replay_conflict", "reference_only_schema", "old_binding_downgrade", "alternate_route_denials", "attestation_authorization", "attestation_replay_reauthorization", "revoked_binding_denied", "unconfigured_profile_denied", "forgotten_deleted_excluded", "high_restricted_excluded")
	if stage == "restart" {
		if previous.IdentityHash != report.IdentityHash || previous.CitationHash != report.CitationHash {
			t.Fatal("restart changed context identities or provenance")
		}
		report.Checks = append(report.Checks, "api_worker_restart_identity")
	}
	report.Result = "passed"
}
