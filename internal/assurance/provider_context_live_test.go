package assurance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	Result                   string           `json:"result"`
	Stage                    string           `json:"stage"`
	SchemaVersion            string           `json:"schema_version"`
	OpenAPIDigest            string           `json:"openapi_digest"`
	ImageDigest              string           `json:"image_digest"`
	SourceRevision           string           `json:"source_revision"`
	SourceDigest             string           `json:"source_digest"`
	ScopeHash                string           `json:"scope_hash"`
	IdentityHash             string           `json:"identity_hash"`
	CitationHash             string           `json:"citation_hash"`
	GovernanceCompleted      int              `json:"governance_completed"`
	DisclosureVersion        string           `json:"disclosure_policy_version"`
	SourceMetadataVersion    string           `json:"source_metadata_version"`
	DurableLabelsVerified    bool             `json:"durable_labels_verified"`
	Checks                   []string         `json:"checks"`
	BudgetContractVersion    string           `json:"budget_contract_version,omitempty"`
	BudgetPackingVersion     string           `json:"budget_packing_version,omitempty"`
	BudgetOutcomes           []string         `json:"budget_outcomes,omitempty"`
	BudgetUsage              []map[string]int `json:"budget_usage,omitempty"`
	BudgetIdentityHash       string           `json:"budget_identity_hash,omitempty"`
	BudgetCitationHash       string           `json:"budget_citation_hash,omitempty"`
	ContextDigestVersion     string           `json:"context_digest_version,omitempty"`
	ContextContinuityVersion string           `json:"context_continuity_version,omitempty"`
	ContextDigestHash        string           `json:"context_digest_hash,omitempty"`
	ContinuityOutcomes       []string         `json:"continuity_outcomes,omitempty"`
	ContinuityChecked        bool             `json:"continuity_checked,omitempty"`
}

type liveBudgetUsage struct {
	Items         int
	ContentBytes  int
	ResponseBytes int
	Citations     int
}

func budgetLiveFixturePath() string {
	return "pc3/budget/multilingual/" + strings.Repeat("provenance/", 12) + "fixture"
}

func TestProviderContextBudgetLiveFixturePathIsCanonical(t *testing.T) {
	path := budgetLiveFixturePath()
	if normalized, err := memory.NormalizeMemoryPath(path); err != nil || normalized != path {
		t.Fatalf("live fixture path violates public ingestion contract: %v", err)
	}
}

func TestProviderContextBudgetUsageMeasuresUTF8AndUniqueCitations(t *testing.T) {
	body := []byte(`{"metadata":{},"result":{"profile":[{"memory":{"content":"中文🙂\"\\\n","id":"m1"},"citations":[{"source_kind":"canonical_memory","reference":"m1","version":"1","availability":"available"}]}],"citations":[{"source_kind":"canonical_memory","reference":"m1","version":"1","availability":"available"}],"budget_accounting":{"usage":{"items":1,"content_bytes":13,"response_bytes":353,"citations":1}}},"citations":[]}`)
	usage, err := measureLiveBudgetUsage(body)
	if err != nil {
		t.Fatal(err)
	}
	if usage.Items != 1 || usage.ContentBytes != len([]byte("中文🙂\"\\\n")) || usage.Citations != 1 || usage.ResponseBytes != len(body) {
		t.Fatalf("unexpected budget usage: %+v", usage)
	}
}

func TestProviderContextLiveAccountingRejectsFalseReportedUsage(t *testing.T) {
	fixture := map[string]any{"result": map[string]any{
		"profile":           []any{map[string]any{"memory": map[string]any{"content": "中文🙂"}, "citations": []any{map[string]any{"source_kind": "canonical_memory", "reference": "m1", "version": "1"}}}},
		"budget_accounting": map[string]any{"contract_version": "context-budget-v1", "packing_policy_version": "context-packing-v1", "usage": map[string]any{"items": 1, "content_bytes": 10, "response_bytes": 0, "citations": 1}},
	}}
	result := fixture["result"].(map[string]any)
	accounting := result["budget_accounting"].(map[string]any)
	usage := accounting["usage"].(map[string]any)
	marshal := func() []byte {
		for i := 0; i < 8; i++ {
			data, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			if usage["response_bytes"] == len(data) {
				return data
			}
			usage["response_bytes"] = len(data)
		}
		t.Fatal("fixture accounting did not converge")
		return nil
	}
	valid := marshal()
	if _, _, _, err := validateLiveBudgetAccounting(valid, map[string]int{"items": 1, "content_bytes": 10, "response_bytes": len(valid), "citations": 1}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func()
	}{
		{"characters-as-bytes", func() { usage["content_bytes"] = 3 }},
		{"citation-appearances-as-identities", func() { usage["citations"] = 2 }},
		{"wrong-items", func() { usage["items"] = 0 }},
		{"unsupported-version", func() { accounting["contract_version"] = "context-budget-v0" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.mutate()
			if _, _, _, err := validateLiveBudgetAccounting(marshal(), nil); err == nil {
				t.Fatal("incorrect accounting accepted")
			}
			usage["content_bytes"], usage["citations"], usage["items"] = 10, 1, 1
			accounting["contract_version"] = "context-budget-v1"
		})
	}
	for _, data := range [][]byte{append(append([]byte{}, valid...), '\n'), bytes.Replace(valid, []byte(fmt.Sprintf(`"response_bytes":%d`, len(valid))), []byte(`"response_bytes":0`), 1), []byte(`{"result":{}}`)} {
		if _, _, _, err := validateLiveBudgetAccounting(data, nil); err == nil {
			t.Fatal("missing or false body accounting accepted")
		}
	}
	if _, _, _, err := validateLiveBudgetAccounting(valid, map[string]int{"content_bytes": 9}); err == nil {
		t.Fatal("over-limit content accounting accepted")
	}
}

func measureLiveBudgetUsage(data []byte) (liveBudgetUsage, error) {
	var doc struct {
		Result struct {
			Profile           []provider.DisclosedContextItem `json:"profile"`
			RecentSession     []provider.DisclosedContextItem `json:"recent_session"`
			RecentEpisodes    []provider.DisclosedContextItem `json:"recent_episodes"`
			RelevantSummaries []provider.DisclosedContextItem `json:"relevant_summaries"`
			RelatedEntities   []provider.DisclosedContextItem `json:"related_entities"`
			Citations         []provider.Citation             `json:"citations"`
		} `json:"result"`
		Citations []provider.Citation `json:"citations"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return liveBudgetUsage{}, fmt.Errorf("budget response invalid")
	}
	usage := liveBudgetUsage{ResponseBytes: len(data)}
	type identity struct{ kind, id, version string }
	seen := map[identity]bool{}
	add := func(citations []provider.Citation) {
		for _, citation := range citations {
			seen[identity{citation.SourceKind, citation.Reference, citation.Version}] = true
		}
	}
	for _, items := range [][]provider.DisclosedContextItem{doc.Result.Profile, doc.Result.RecentSession, doc.Result.RecentEpisodes, doc.Result.RelevantSummaries, doc.Result.RelatedEntities} {
		for _, item := range items {
			usage.Items++
			if item.Memory != nil {
				usage.ContentBytes += len(item.Memory.Content)
			}
			add(item.Citations)
		}
	}
	add(doc.Result.Citations)
	add(doc.Citations)
	usage.Citations = len(seen)
	return usage, nil
}

func budgetNumber(value any) (int, bool) {
	n, ok := value.(float64)
	if !ok || n < 0 || n > float64(int(^uint(0)>>1)) || n != float64(int(n)) {
		return 0, false
	}
	return int(n), true
}

func validateLiveBudgetAccounting(data []byte, expected map[string]int) (liveBudgetUsage, string, string, error) {
	if !bytes.Equal(bytes.TrimSpace(data), data) {
		return liveBudgetUsage{}, "", "", fmt.Errorf("budget response has unmeasured formatting")
	}
	usage, err := measureLiveBudgetUsage(data)
	if err != nil {
		return liveBudgetUsage{}, "", "", err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return liveBudgetUsage{}, "", "", err
	}
	result, ok := doc["result"].(map[string]any)
	if !ok {
		return liveBudgetUsage{}, "", "", fmt.Errorf("result missing")
	}
	accounting, ok := result["budget_accounting"].(map[string]any)
	if !ok {
		return liveBudgetUsage{}, "", "", fmt.Errorf("budget_accounting missing")
	}
	contract, _ := accounting["contract_version"].(string)
	packing, _ := accounting["packing_policy_version"].(string)
	if contract != "context-budget-v1" || packing != "context-packing-v1" {
		return liveBudgetUsage{}, contract, packing, fmt.Errorf("unsupported budget versions")
	}
	reported, ok := accounting["usage"].(map[string]any)
	if !ok {
		return liveBudgetUsage{}, contract, packing, fmt.Errorf("budget usage missing")
	}
	for key, actual := range map[string]int{"items": usage.Items, "content_bytes": usage.ContentBytes, "response_bytes": usage.ResponseBytes, "citations": usage.Citations} {
		n, valid := budgetNumber(reported[key])
		if !valid || n != actual {
			return liveBudgetUsage{}, contract, packing, fmt.Errorf("budget usage mismatch: %s", key)
		}
		if limit, exists := expected[key]; exists && actual > limit {
			return liveBudgetUsage{}, contract, packing, fmt.Errorf("budget usage exceeds %s", key)
		}
	}
	return usage, contract, packing, nil
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
	var budgetCaps struct {
		ContractVersion string `json:"context_budget_version"`
		PackingVersion  string `json:"context_packing_policy_version"`
		TokenSupported  *bool  `json:"token_budget_supported"`
		Limits          struct {
			Items         int `json:"max_context_items"`
			ContentBytes  int `json:"max_context_content_bytes"`
			ResponseBytes int `json:"max_context_bytes"`
			Citations     int `json:"max_citations"`
		} `json:"limits"`
	}
	if json.Unmarshal(capsData, &budgetCaps) != nil || budgetCaps.ContractVersion != "context-budget-v1" || budgetCaps.PackingVersion != "context-packing-v1" || budgetCaps.TokenSupported == nil || *budgetCaps.TokenSupported || budgetCaps.Limits.Items < 1 || budgetCaps.Limits.ContentBytes < 0 || budgetCaps.Limits.ContentBytes > budgetCaps.Limits.ResponseBytes || budgetCaps.Limits.ResponseBytes < 1 || budgetCaps.Limits.Citations < 1 {
		t.Fatal("context budget discovery unavailable or invalid")
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
		result, err := contract.ValidateForRequest(data, scope, "provider-v1", body.Input, &caps.Limits)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := validateLiveBudgetAccounting(data, map[string]int{"items": min(20, budgetCaps.Limits.Items), "content_bytes": budgetCaps.Limits.ContentBytes, "response_bytes": budgetCaps.Limits.ResponseBytes, "citations": budgetCaps.Limits.Citations}); err != nil {
			t.Fatal(err)
		}
		return result
	}
	read := func(path, prefix string) provider.DisclosedContextResponse {
		return readMode(path, prefix, auth.ResponseModeBoundedContent)
	}
	var report contextLiveReport
	continuityCheckpoint := filepath.Join(dir, "pc4-continuity.json")
	readContinuity := func() provider.DisclosedContextResponse {
		body := provider.ContextRequest{Metadata: metadata("pc4-continuity"), Input: provider.ContextInput{Query: "context fixture", Budget: 20, Path: budgetLiveFixturePath(), ResponseMode: auth.ResponseModeBoundedContent, Continuity: &provider.ContextContinuityInput{Mode: "issue"}}}
		data, code := call("POST", "/v1/provider/context", runtimeKey, body, &binding, false)
		if code != 200 {
			t.Fatal("continuity issue failed")
		}
		issued, err := contract.ValidateForRequest(data, scope, "provider-v1", body.Input, &caps.Limits)
		if err != nil {
			t.Fatal(err)
		}
		if issued.Result.Continuity.Outcome != "issued" || !provider.ValidContextReference(issued.Result.Continuity.Reference) {
			t.Fatal("continuity issue contract invalid")
		}
		ref := issued.Result.Continuity.Reference
		if stage == "initial" {
			raw, _ := json.Marshal(struct {
				Reference string `json:"reference"`
			}{ref})
			if err := os.WriteFile(continuityCheckpoint, raw, 0600); err != nil {
				t.Fatal("continuity checkpoint unavailable")
			}
		} else {
			raw, err := os.ReadFile(continuityCheckpoint)
			if err != nil {
				t.Fatal("continuity checkpoint missing")
			}
			var saved struct {
				Reference string `json:"reference"`
			}
			if json.Unmarshal(raw, &saved) != nil || !provider.ValidContextReference(saved.Reference) {
				t.Fatal("continuity checkpoint invalid")
			}
			ref = saved.Reference
		}
		body.Input.Continuity = &provider.ContextContinuityInput{Mode: "compare", Reference: ref}
		data, code = call("POST", "/v1/provider/context", runtimeKey, body, &binding, false)
		if code != 200 {
			t.Fatal("continuity compare failed")
		}
		comp, err := contract.ValidateForRequest(data, scope, "provider-v1", body.Input, &caps.Limits)
		if err != nil {
			t.Fatal(err)
		}
		if comp.Result.Continuity.ReadSemantics != "fresh_compare" || comp.Result.Continuity.Outcome != "unchanged" || comp.Result.ContextDigest.Value != issued.Result.ContextDigest.Value {
			t.Fatal("continuity compare was not stable")
		}
		report.ContinuityOutcomes = append(report.ContinuityOutcomes, "issued", "unchanged")
		report.ContinuityChecked = true
		report.ContextDigestHash = contextLiveHash([]string{issued.Result.ContextDigest.Value})
		return comp
	}
	flatten := func(r provider.DisclosedContextResponse) []provider.DisclosedContextItem {
		out := []provider.DisclosedContextItem{}
		for _, items := range [][]provider.DisclosedContextItem{r.Result.Profile, r.Result.RecentSession, r.Result.RecentEpisodes, r.Result.RelevantSummaries, r.Result.RelatedEntities} {
			out = append(out, items...)
		}
		return out
	}
	visiblePath, hiddenPath := "pc1/context/visible", "pc1/context/hidden"
	report = contextLiveReport{Result: "failed", Stage: stage, SchemaVersion: "provider-v1", DisclosureVersion: auth.DisclosurePolicyVersion, SourceMetadataVersion: memory.SourceMetadataVersion, OpenAPIDigest: contract.Digest, ImageDigest: os.Getenv("STELE_CONTEXT_VERIFY_IMAGE_DIGEST"), SourceRevision: os.Getenv("STELE_CONTEXT_VERIFY_SOURCE_REVISION"), SourceDigest: os.Getenv("STELE_CONTEXT_VERIFY_SOURCE_DIGEST"), ScopeHash: contextLiveHash(scope), ContextDigestVersion: provider.ContextDigestVersion, ContextContinuityVersion: provider.ContextContinuityVersion}
	report.BudgetContractVersion, report.BudgetPackingVersion = budgetCaps.ContractVersion, budgetCaps.PackingVersion
	if !strings.HasPrefix(report.ImageDigest, "sha256:") || len(report.SourceRevision) != 40 || !validContextDigest(report.SourceDigest) {
		t.Fatal("pinned build provenance missing")
	}
	var previous contextLiveReport
	if stage == "restart" {
		data, err := os.ReadFile(reportPath)
		if err != nil || json.Unmarshal(data, &previous) != nil || previous.Result != "passed" || previous.Stage != "initial" || previous.OpenAPIDigest != report.OpenAPIDigest || previous.ImageDigest != report.ImageDigest || previous.SourceDigest != report.SourceDigest || previous.ScopeHash != report.ScopeHash || previous.BudgetContractVersion != report.BudgetContractVersion || previous.BudgetPackingVersion != report.BudgetPackingVersion || previous.ContextDigestVersion != provider.ContextDigestVersion || previous.ContextContinuityVersion != provider.ContextContinuityVersion || len(previous.BudgetUsage) == 0 || len(previous.BudgetOutcomes) == 0 || !previous.ContinuityChecked {
			t.Fatal("initial live evidence missing or incompatible")
		}
		report.GovernanceCompleted = previous.GovernanceCompleted
		// The initial stage explicitly leaves this exact primary grant at content
		// capability. Observe the owned PostgreSQL authority before any restart
		// stage requests, so an unexpected default profile cannot be mistaken for
		// a retrieval or response-mode issue.
		grantDB, err := pgx.Connect(context.Background(), os.Getenv("STELE_CONTEXT_VERIFY_POSTGRES_DSN"))
		if err != nil {
			t.Fatal("restart authority observation unavailable")
		}
		defer grantDB.Close(context.Background())
		var defaultProfile string
		if err := grantDB.QueryRow(context.Background(), `SELECT default_disclosure_profile FROM access_scope_grants WHERE principal_id=$1 AND tenant=$2 AND project=$3 AND namespace=$4 AND status='active' ORDER BY created_at DESC, id DESC LIMIT 1`, binding.PrincipalID, scope.Tenant, scope.Project, scope.Namespace).Scan(&defaultProfile); err != nil || defaultProfile != auth.DisclosureProfileContent {
			t.Fatal("restart durable primary disclosure state changed")
		}
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
	// This separate non-root path keeps the earlier lifecycle/prefix fixture
	// unchanged while exercising UTF-8 source bytes and JSON escaping.
	budgetPath := budgetLiveFixturePath()
	budgetContent := "Durable context fixture for byte accounting. 中文🙂 quoted \"context\", backslash \\, and newline\nremain source data."
	budgetMeta := provider.OperationMetadata{RequestID: "pc3-budget-fixture", OperationID: "pc3-budget-fixture", IdempotencyKey: "pc3-budget-fixture", SchemaVersion: "provider-v1"}
	budgetBody := map[string]any{"metadata": budgetMeta, "event_type": "conversation.message", "memory_path": budgetPath, "content": budgetContent, "metadata_map": map[string]any{"fixture": "pc3"}, "source_metadata": memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationMedium}}
	budgetEventData, budgetEventStatus := call("POST", "/v1/provider/events", runtimeKey, budgetBody, &binding, false)
	var budgetEvent provider.IngestResult
	if budgetEventStatus != 201 || json.Unmarshal(budgetEventData, &budgetEvent) != nil || budgetEvent.EventID == "" || (stage == "restart" && !budgetEvent.Replayed) {
		t.Fatal("budget governed event write or durable replay failed")
	}
	budgetDeadline := time.Now().Add(45 * time.Second)
	for {
		data, code := call("GET", "/v1/admin/governance/raw-events/"+budgetEvent.EventID, adminKey, nil, nil, false)
		var raw governance.GovernanceRawEvent
		if code == 200 && json.Unmarshal(data, &raw) == nil && raw.State == governance.GovernanceRawEventStateProcessed && !raw.ProcessedAt.IsZero() {
			break
		}
		if time.Now().After(budgetDeadline) {
			t.Fatal("budget event governance completion missing within bound")
		}
		time.Sleep(250 * time.Millisecond)
	}
	if stage == "initial" {
		report.GovernanceCompleted++
	}
	readBudget := func(label, path, mode string, itemLimit int, limits map[string]int, optionalSections bool, outcome string) (provider.DisclosedContextResponse, liveBudgetUsage) {
		input := map[string]any{"query": "context fixture", "budget": itemLimit, "path": path, "response_mode": mode}
		if limits != nil {
			input["budget_limits"] = limits
		}
		if optionalSections {
			for _, flag := range []string{"include_experience_insights", "include_goal_context", "include_diagnostics", "include_feedback_diagnostics"} {
				input[flag] = true
			}
		}
		data, code := call("POST", "/v1/provider/context", runtimeKey, map[string]any{"metadata": metadata("budget-" + label), "input": input}, &binding, false)
		if code != 200 {
			t.Fatalf("budget fixture %s failed (HTTP %d)", label, code)
		}
		encodedInput, _ := json.Marshal(input)
		var originalInput provider.ContextInput
		if json.Unmarshal(encodedInput, &originalInput) != nil {
			t.Fatal("live request allocation unavailable")
		}
		result, err := contract.ValidateForRequest(data, scope, "provider-v1", originalInput, &caps.Limits)
		if err != nil {
			t.Fatal(err)
		}
		effective := map[string]int{"max_items": min(itemLimit, budgetCaps.Limits.Items), "max_content_bytes": budgetCaps.Limits.ContentBytes, "max_response_bytes": budgetCaps.Limits.ResponseBytes, "max_citations": budgetCaps.Limits.Citations}
		for key, value := range limits {
			effective[key] = min(value, effective[key])
		}
		usage, _, _, err := validateLiveBudgetAccounting(data, map[string]int{"items": effective["max_items"], "content_bytes": effective["max_content_bytes"], "response_bytes": effective["max_response_bytes"], "citations": effective["max_citations"]})
		if err != nil {
			t.Fatal(err)
		}
		var decoded struct {
			Result struct {
				Accounting struct {
					Requested map[string]*int `json:"requested"`
					Effective map[string]int  `json:"effective"`
					Selection struct {
						Outcome     string   `json:"outcome"`
						Reasons     []string `json:"omission_reasons"`
						Unsupported []string `json:"unsupported_sections"`
					} `json:"selection"`
				} `json:"budget_accounting"`
			} `json:"result"`
		}
		if json.Unmarshal(data, &decoded) != nil {
			t.Fatal("budget accounting decode failed")
		}
		a := decoded.Result.Accounting
		for key, value := range effective {
			if a.Effective[key] != value {
				t.Fatalf("budget fixture %s intersection mismatch: %s", label, key)
			}
			requested, exists := a.Requested[key]
			expected, specified := limits[key]
			if key == "max_items" {
				expected, specified = itemLimit, true
			}
			if !exists || (specified && (requested == nil || *requested != expected)) || (!specified && requested != nil) {
				t.Fatalf("budget fixture %s requested allocation mismatch: %s", label, key)
			}
		}
		if result.Result.ResponseMode != mode || a.Selection.Outcome != outcome {
			t.Fatalf("budget fixture %s mode=%s profile=%s outcome=%s usage=%d/%d reasons=%s, want mode=%s outcome=%s", label, result.Result.ResponseMode, result.Result.Disclosure.Profile, a.Selection.Outcome, usage.Items, usage.Citations, strings.Join(a.Selection.Reasons, ","), mode, outcome)
		}
		if outcome == "empty" && len(a.Selection.Reasons) != 0 {
			t.Fatal("ineligible evidence contributed budget reasons")
		}
		if outcome == "budget_exhausted" && len(a.Selection.Reasons) == 0 {
			t.Fatal("budget exhaustion reason missing")
		}
		if limit, specified := limits["max_citations"]; specified && limit == 0 && outcome == "budget_exhausted" && !strings.Contains(","+strings.Join(a.Selection.Reasons, ",")+",", ",citations,") {
			t.Fatal("zero citation budget reason missing")
		}
		if limit, specified := limits["max_content_bytes"]; specified && limit == 0 && outcome == "budget_exhausted" && !strings.Contains(","+strings.Join(a.Selection.Reasons, ",")+",", ",content_bytes,") {
			t.Fatal("zero content budget reason missing")
		}
		expectedUnsupported := ""
		if optionalSections {
			expectedUnsupported = "experience_insights,goal_context,diagnostics,feedback_diagnostics"
		}
		if strings.Join(a.Selection.Unsupported, ",") != expectedUnsupported {
			t.Fatal("fixed unsupported section accounting mismatch")
		}
		if mode == auth.ResponseModeReferenceOnly && usage.ContentBytes != 0 {
			t.Fatal("reference-only content usage was nonzero")
		}
		report.BudgetOutcomes = append(report.BudgetOutcomes, label+":"+outcome)
		report.BudgetUsage = append(report.BudgetUsage, map[string]int{"items": usage.Items, "content_bytes": usage.ContentBytes, "response_bytes": usage.ResponseBytes, "citations": usage.Citations})
		return result, usage
	}
	readContinuity()
	checkBudgetMode := func(mode string) {
		label := "content"
		if mode == auth.ResponseModeReferenceOnly {
			label = "reference"
		}
		ample, usage := readBudget(label+"-omitted", budgetPath, mode, 20, nil, false, "selected")
		if usage.Items == 0 || usage.Citations == 0 {
			t.Fatal("ample budget lost eligible evidence")
		}
		if mode == auth.ResponseModeBoundedContent {
			multibyte := false
			identities, citations := []string{}, []string{}
			for _, item := range flatten(ample) {
				multibyte = multibyte || strings.Contains(item.Memory.Content, "中文🙂")
				identities = append(identities, item.Memory.MemoryID+"|"+item.Memory.Version)
			}
			for _, citation := range ample.Result.Citations {
				citations = append(citations, citation.SourceKind+"|"+citation.Reference+"|"+citation.Version)
			}
			if !multibyte {
				t.Fatal("budget source did not retain multilingual content")
			}
			sort.Strings(identities)
			sort.Strings(citations)
			report.BudgetIdentityHash, report.BudgetCitationHash = contextLiveHash(identities), contextLiveHash(citations)
		}
		readBudget(label+"-empty-limits", budgetPath, mode, 20, map[string]int{}, false, "selected")
		readBudget(label+"-oversized", budgetPath, mode, 2147483647, map[string]int{"max_content_bytes": 2147483647, "max_response_bytes": 2147483647, "max_citations": 2147483647}, false, "selected")
		zeroContentOutcome := "budget_exhausted"
		if mode == auth.ResponseModeReferenceOnly {
			zeroContentOutcome = "selected"
		}
		readBudget(label+"-zero-content", budgetPath, mode, 20, map[string]int{"max_content_bytes": 0}, false, zeroContentOutcome)
		readBudget(label+"-zero-citations", budgetPath, mode, 20, map[string]int{"max_citations": 0}, false, "budget_exhausted")
		readBudget(label+"-unsupported", budgetPath, mode, 20, nil, true, "selected")
		for i, path := range []string{"pc1/context/absent", hiddenPath, "pc1/context/forgotten", "pc1/context/deleted", "pc1/context/high", "pc1/context/restricted"} {
			readBudget(fmt.Sprintf("%s-safe-empty-%d", label, i), path, mode, 20, map[string]int{"max_content_bytes": 0, "max_citations": 0}, false, "empty")
		}
		data, code := call("POST", "/v1/provider/context", runtimeKey, map[string]any{"metadata": metadata(label + "-minimum-response"), "input": map[string]any{"query": "context fixture", "budget": 20, "path": budgetPath, "response_mode": mode, "budget_limits": map[string]int{"max_response_bytes": 1}}}, &binding, false)
		var failure struct {
			Error  provider.ProviderError `json:"error"`
			Result json.RawMessage        `json:"result"`
		}
		if code != http.StatusRequestEntityTooLarge || json.Unmarshal(data, &failure) != nil || failure.Error.Validate() != nil || failure.Error.Code != "context_limit_exceeded" || len(failure.Result) != 0 || len(data) > 4096 || bytes.Contains(data, []byte(budgetContent)) {
			t.Fatal("infeasible response allocation did not fail without projection")
		}
		report.BudgetOutcomes = append(report.BudgetOutcomes, label+"-minimum-response:413")
	}
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
	checkBudgetMode(auth.ResponseModeBoundedContent)
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
	checkBudgetMode(auth.ResponseModeReferenceOnly)
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
	// Each stage leaves the primary runtime grant at content capability. Assert
	// that final public state now, so a restart-stage mismatch is attributable to
	// durable authority state rather than the fixture's own mutation sequence.
	finalContent := readMode(budgetPath, "", auth.ResponseModeBoundedContent)
	if finalContent.Result.Disclosure.Profile != auth.DisclosureProfileContent || len(flatten(finalContent)) == 0 {
		t.Fatal("final content disclosure state unavailable")
	}
	// PC4 comparison must reassemble current evidence rather than replaying the
	// issued body. Run this after every budget, disclosure, and restart read so
	// the destructive lifecycle fixture cannot invalidate later assertions.
	if stage == "restart" {
		input := provider.ContextInput{Query: "context fixture", Budget: 20, Path: budgetPath, ResponseMode: auth.ResponseModeBoundedContent, Continuity: &provider.ContextContinuityInput{Mode: "issue"}}
		data, code := call("POST", "/v1/provider/context", runtimeKey, provider.ContextRequest{Metadata: metadata("pc4-current-source-issue"), Input: input}, &binding, false)
		issued, err := contract.ValidateForRequest(data, scope, "provider-v1", input, &caps.Limits)
		if code != 200 || err != nil || len(flatten(issued)) == 0 {
			t.Fatal("current-source issue fixture unavailable")
		}
		for _, item := range flatten(issued) {
			_, code = call("POST", "/v1/admin/memories/"+item.Memory.MemoryID+":suppress", adminKey, map[string]string{"reason": "owned PC4 current-source fixture"}, nil, false)
			if code != 200 {
				t.Fatal("current-source suppression failed")
			}
		}
		input.Continuity = &provider.ContextContinuityInput{Mode: "compare", Reference: issued.Result.Continuity.Reference}
		data, code = call("POST", "/v1/provider/context", runtimeKey, provider.ContextRequest{Metadata: metadata("pc4-current-source-compare"), Input: input}, &binding, false)
		changed, err := contract.ValidateForRequest(data, scope, "provider-v1", input, &caps.Limits)
		if code != 200 || err != nil || changed.Result.Continuity.ReadSemantics != "fresh_compare" || changed.Result.Continuity.Outcome != "changed" || len(flatten(changed)) != 0 || bytes.Contains(data, []byte(budgetContent)) {
			t.Fatal("changed comparison retained historical source content")
		}
		report.ContinuityOutcomes = append(report.ContinuityOutcomes, "changed")

		// Expiry and removal are changed only through the owned PostgreSQL fixture;
		// public requests prove the two distinct bounded outcomes without logging a
		// reference or database state in the report.
		input.Continuity = &provider.ContextContinuityInput{Mode: "issue"}
		data, code = call("POST", "/v1/provider/context", runtimeKey, provider.ContextRequest{Metadata: metadata("pc4-expiry-issue"), Input: input}, &binding, false)
		expiring, err := contract.ValidateForRequest(data, scope, "provider-v1", input, &caps.Limits)
		if code != 200 || err != nil {
			t.Fatal("expiry issue fixture unavailable")
		}
		if _, err = db.Exec(ctx, `UPDATE provider_context_receipts SET issued_at=clock_timestamp()-interval '2 minutes', expires_at=clock_timestamp()-interval '1 minute' WHERE reference=$1`, expiring.Result.Continuity.Reference); err != nil {
			t.Fatal("owned expiry mutation unavailable")
		}
		input.Continuity = &provider.ContextContinuityInput{Mode: "compare", Reference: expiring.Result.Continuity.Reference}
		data, code = call("POST", "/v1/provider/context", runtimeKey, provider.ContextRequest{Metadata: metadata("pc4-expiry-compare"), Input: input}, &binding, false)
		if code != http.StatusConflict || !bytes.Contains(data, []byte(`"code":"continuity_reference_expired"`)) || bytes.Contains(data, []byte(budgetContent)) {
			t.Fatal("expired reference was not safely refused")
		}
		if _, err = db.Exec(ctx, `DELETE FROM provider_context_receipts WHERE reference=$1`, expiring.Result.Continuity.Reference); err != nil {
			t.Fatal("owned expired cleanup unavailable")
		}
		data, code = call("POST", "/v1/provider/context", runtimeKey, provider.ContextRequest{Metadata: metadata("pc4-unavailable-compare"), Input: input}, &binding, false)
		if code != http.StatusConflict || !bytes.Contains(data, []byte(`"code":"continuity_reference_unavailable"`)) || bytes.Contains(data, []byte(budgetContent)) {
			t.Fatal("cleaned reference was not safely unavailable")
		}
		report.ContinuityOutcomes = append(report.ContinuityOutcomes, "expired", "unavailable")
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
	report.Checks = append(report.Checks, "budget_discovery", "budget_actual_body_bytes", "budget_utf8_content_bytes", "budget_unique_citations", "budget_zero_content", "budget_zero_citations", "budget_omitted_limits", "budget_server_intersection", "budget_minimum_response_413", "budget_unsupported_sections", "budget_safe_empty_no_reasons")
	if stage == "restart" {
		report.Checks = append(report.Checks, "continuity_current_source_changed", "continuity_expired_refused", "continuity_cleaned_reference_unavailable")
	}
	if stage == "restart" {
		if previous.IdentityHash != report.IdentityHash || previous.CitationHash != report.CitationHash || previous.BudgetIdentityHash != report.BudgetIdentityHash || previous.BudgetCitationHash != report.BudgetCitationHash || previous.ContextDigestHash != report.ContextDigestHash {
			t.Fatal("restart changed context identities or provenance")
		}
		report.Checks = append(report.Checks, "api_worker_restart_identity")
	}
	report.Result = "passed"
}
