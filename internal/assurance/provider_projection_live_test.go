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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/openapi"
	"github.com/jackc/pgx/v5"
)

type projectionLiveReport struct {
	Result                       string   `json:"result"`
	Stage                        string   `json:"stage"`
	ContractVersion              string   `json:"contract_version"`
	SchemaVersion                string   `json:"schema_version"`
	FixtureVersion               string   `json:"fixture_version"`
	MigrationVersion             int64    `json:"migration_version"`
	MigrationManifestDigest      string   `json:"migration_manifest_digest"`
	MigrationIntegrityVerified   bool     `json:"migration_integrity_verified"`
	OpenAPIDigest                string   `json:"openapi_digest"`
	ImageDigest                  string   `json:"image_digest"`
	SourceRevision               string   `json:"source_revision"`
	SourceDigest                 string   `json:"source_digest"`
	ScopeHash                    string   `json:"scope_hash"`
	IdentityHash                 string   `json:"identity_hash"`
	FeedbackIdentityHash         string   `json:"feedback_identity_hash"`
	PendingChecked               bool     `json:"pending_checked"`
	PendingToCompletedChecked    bool     `json:"pending_to_completed_checked"`
	CorrectionHistoryChecked     bool     `json:"correction_history_checked"`
	ExactScopeChecked            bool     `json:"exact_scope_checked"`
	ContextReceiptChecked        bool     `json:"context_receipt_checked"`
	FeedbackNonactivationChecked bool     `json:"feedback_nonactivation_checked"`
	AsyncFeedbackChecked         bool     `json:"async_feedback_checked"`
	RestartChecked               bool     `json:"restart_checked"`
	LifecycleChecked             bool     `json:"lifecycle_checked"`
	PrivilegedHistoryChecked     bool     `json:"privileged_history_checked"`
	Checks                       []string `json:"checks"`
}

// Checkpoints contain attribution, never public evidence artifacts. The official
// harness owns and removes their credential directory with the fresh stack.
type projectionLiveCheckpoint struct {
	Request        provider.ProjectionRequest `json:"request"`
	Initial        provider.ProjectionRecord  `json:"initial"`
	EventID        string                     `json:"event_id"`
	ForeignEventID string                     `json:"foreign_event_id"`
	ForeignBinding provider.RuntimeBinding    `json:"foreign_binding"`
	Corrected      provider.ProjectionRecord  `json:"corrected"`
}

func TestProviderProjectionFreshPostgresLive(t *testing.T) {
	base, stage, phase := os.Getenv("STELE_CONTEXT_VERIFY_URL"), os.Getenv("STELE_CONTEXT_VERIFY_STAGE"), os.Getenv("STELE_PROJECTION_VERIFY_PHASE")
	reportPath, dir := os.Getenv("STELE_PROJECTION_VERIFY_REPORT"), os.Getenv("STELE_CONTEXT_VERIFY_CREDENTIAL_DIR")
	if base == "" && reportPath == "" {
		t.Skip("owned live fixture not configured")
	}
	if base == "" || reportPath == "" || dir == "" || (stage != "initial" && stage != "restart") || (phase != "pending" && phase != "settled") || (phase == "pending" && stage != "initial") {
		t.Fatal("projection live verifier configuration missing")
	}
	report := projectionLiveReport{Result: "failed", Stage: stage, ContractVersion: ProjectionContractVersion, SchemaVersion: "provider-v1", FixtureVersion: "provider-turn-outcome-feedback-live-v1", ImageDigest: os.Getenv("STELE_CONTEXT_VERIFY_IMAGE_DIGEST"), SourceRevision: os.Getenv("STELE_CONTEXT_VERIFY_SOURCE_REVISION"), SourceDigest: os.Getenv("STELE_CONTEXT_VERIFY_SOURCE_DIGEST")}
	defer func() {
		if t.Failed() {
			report.Result = "failed"
		}
		data, _ := json.MarshalIndent(report, "", "  ")
		if os.WriteFile(reportPath, data, 0600) != nil {
			t.Error("bounded projection report unavailable")
		}
	}()
	readSecret := func(name string, out any) {
		t.Helper()
		data, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil || json.Unmarshal(data, out) != nil {
			t.Fatal("owned projection checkpoint unavailable")
		}
	}
	credential := func(name string) string {
		data, e := os.ReadFile(filepath.Join(dir, name+".credential"))
		if e != nil || len(bytes.TrimSpace(data)) == 0 {
			t.Fatal("owned projection credential unavailable")
		}
		return strings.TrimSpace(string(data))
	}
	runtimeKey, adminKey := credential("runtime"), credential("admin")
	var binding provider.RuntimeBinding
	readSecret("context-binding.json", &binding)
	scope := provider.ProjectionScope(binding)
	if scope.Validate() != nil || binding.Validate(time.Now()) != nil {
		t.Fatal("projection exact binding unavailable")
	}
	report.ScopeHash = contextLiveHash(scope)
	client := &http.Client{Timeout: 10 * time.Second}
	call := func(method, path, key string, body any, b provider.RuntimeBinding, override http.Header) ([]byte, int) {
		t.Helper()
		var payload []byte
		var e error
		if raw, ok := body.([]byte); ok {
			payload = raw
		} else if body != nil {
			payload, e = json.Marshal(body)
			if e != nil {
				t.Fatal("projection fixture encoding unavailable")
			}
		}
		req, e := http.NewRequest(method, base+path, bytes.NewReader(payload))
		if e != nil {
			t.Fatal("projection request unavailable")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", key)
		req.Header.Set("X-Stele-Tenant", scope.Tenant)
		req.Header.Set("X-Stele-Project", scope.Project)
		req.Header.Set("X-Stele-Namespace", scope.Namespace)
		req.Header.Set("X-Stele-Actor", "projection-verifier")
		if b.BindingID != "" {
			req.Header.Set(provider.HeaderRuntimeBinding, b.BindingID)
			req.Header.Set(provider.HeaderRuntimeSession, b.SessionID)
			req.Header.Set(provider.HeaderRuntimeAgent, b.AgentID)
			if b.ConversationID != "" {
				req.Header.Set(provider.HeaderRuntimeConversation, b.ConversationID)
			}
		}
		for k, vs := range override {
			req.Header.Del(k)
			for _, value := range vs {
				req.Header.Add(k, value)
			}
		}
		res, e := client.Do(req)
		if e != nil {
			t.Fatal("projection HTTP unavailable")
		}
		defer res.Body.Close()
		data, e := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
		if e != nil || len(data) > 4<<20 {
			t.Fatal("projection response allocation exceeded")
		}
		return data, res.StatusCode
	}
	spec, code := call("GET", "/openapi.yaml", "", nil, binding, nil)
	if code != 200 {
		t.Fatal("served projection OpenAPI unavailable")
	}
	contract, e := NewProviderProjectionContract(spec)
	if e != nil {
		t.Fatal("served projection contract unavailable")
	}
	local, e := NewProviderProjectionContract([]byte(openapi.SpecYAML()))
	if e != nil || local.digest != contract.digest {
		t.Fatal("image and local projection contracts differ")
	}
	report.OpenAPIDigest = contract.digest
	caps, code := call("GET", "/v1/provider/capabilities", runtimeKey, nil, binding, nil)
	if code != 200 || contract.NegotiateDocument(caps) != nil {
		t.Fatal("served projection capability incompatible")
	}
	var capability struct {
		SchemaDigest  string `json:"schema_digest"`
		SchemaVersion string `json:"schema_version"`
	}
	if json.Unmarshal(caps, &capability) != nil || capability.SchemaDigest != "sha256:"+contract.digest || capability.SchemaVersion != report.SchemaVersion {
		t.Fatal("projection capability provenance mismatch")
	}
	if !strings.HasPrefix(report.ImageDigest, "sha256:") || len(report.ImageDigest) != 71 || len(report.SourceRevision) != 40 || !validContextDigest(report.SourceDigest) {
		t.Fatal("projection build provenance missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	db, e := pgx.Connect(ctx, os.Getenv("STELE_CONTEXT_VERIFY_POSTGRES_DSN"))
	if e != nil {
		t.Fatal("owned projection PostgreSQL unavailable")
	}
	defer db.Close(context.Background())
	scalar := func(query string, args ...any) int {
		t.Helper()
		var n int
		if db.QueryRow(ctx, query, args...).Scan(&n) != nil {
			t.Fatal("bounded projection SQL observation unavailable")
		}
		return n
	}
	var dirty bool
	if db.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&report.MigrationVersion, &dirty) != nil || dirty {
		t.Fatal("projection migration integrity unavailable")
	}
	assets, e := filepath.Glob("../storage/postgres/migrations/*.up.sql")
	if e != nil || len(assets) == 0 {
		t.Fatal("projection migration manifest unavailable")
	}
	manifest := []string{}
	for n, path := range assets {
		name := filepath.Base(path)
		prefix, _, _ := strings.Cut(name, "_")
		version, err := strconv.ParseInt(prefix, 10, 64)
		data, readErr := os.ReadFile(path)
		if err != nil || readErr != nil || version != int64(n+1) {
			t.Fatal("projection migration source invalid")
		}
		sum := sha256.Sum256(data)
		digest := hex.EncodeToString(sum[:])
		if scalar(`SELECT count(*) FROM stele_schema_migration_ledger WHERE version=$1 AND migration_name=$2 AND checksum_sha256=$3`, version, name, digest) != 1 {
			t.Fatal("projection migration checksum differs")
		}
		manifest = append(manifest, name+"|"+digest)
	}
	if report.MigrationVersion != int64(len(assets)) || scalar(`SELECT count(*) FROM stele_schema_migration_ledger`) != len(assets) {
		t.Fatal("projection migration ledger mismatch")
	}
	report.MigrationManifestDigest = contextLiveHash(manifest)
	report.MigrationIntegrityVerified = true
	meta := func(id string) provider.OperationMetadata {
		return provider.OperationMetadata{RequestID: "pc6-" + id, OperationID: "pc6-" + id, IdempotencyKey: "pc6-" + id, SchemaVersion: "provider-v1"}
	}
	encode := func(v any) []byte {
		t.Helper()
		data, e := json.Marshal(v)
		if e != nil {
			t.Fatal("projection encoding failed")
		}
		return data
	}
	admit := func(req provider.ProjectionRequest) provider.ProjectionRecord {
		t.Helper()
		payload := encode(req)
		if _, e := contract.ValidateRequest(payload, "provider-v1"); e != nil {
			t.Fatal("projection serialized request failed-contract")
		}
		data, status := call("POST", "/v1/provider/projections", runtimeKey, payload, binding, nil)
		if status != 202 {
			bounded, err := contract.ValidateError(data, status)
			if err != nil {
				t.Fatalf("projection admission HTTP status %d category=failed_contract code=invalid_error_contract", status)
			}
			t.Fatalf("projection admission HTTP status %d category=%s code=%s", status, bounded.Category, bounded.Code)
		}
		out, e := contract.Validate(data, scope, "provider-v1")
		if e != nil {
			t.Fatal("projection response failed-contract")
		}
		return out.Record
	}
	history := func(kind, effect string, limit int, b provider.RuntimeBinding) provider.ProjectionHistoryResponse {
		t.Helper()
		req := provider.ProjectionHistoryRequest{Metadata: meta("history"), Input: provider.ProjectionHistoryInput{ContractVersion: ProjectionContractVersion, Kind: kind, EffectID: effect, Limit: limit}}
		payload := encode(req)
		if _, e := contract.ValidateHistoryRequest(payload, "provider-v1"); e != nil {
			t.Fatal("projection history request failed-contract")
		}
		data, status := call("POST", "/v1/provider/projections/history", runtimeKey, payload, b, nil)
		out, e := contract.ValidateHistory(data, provider.ProjectionScope(b), "provider-v1", limit)
		if status != 200 || e != nil {
			t.Fatal("projection history failed-contract")
		}
		return out
	}
	refContract, e := NewProviderReferenceContract(spec)
	if e != nil {
		t.Fatal("projection reference contract unavailable")
	}
	reference := func(req provider.ReferenceWriteRequest, b provider.RuntimeBinding) string {
		data, status := call("POST", "/v1/provider/references", runtimeKey, req, b, nil)
		out, e := refContract.Validate(data, provider.ProjectionScope(b), "provider-v1", "reference_only")
		if status != 202 || e != nil || !validUUID(out.EventID) {
			t.Fatal("PC6 reference admission unavailable")
		}
		return out.EventID
	}
	checkpointPath := filepath.Join(dir, "pc6-projection.json")
	var cp projectionLiveCheckpoint
	feedbackIDs := func(logical string) []string {
		t.Helper()
		rows, e := db.Query(ctx, `SELECT l.feedback_id FROM provider_projection_feedback_links l JOIN usefulness_feedback f ON f.id=l.feedback_id AND f.tenant=l.tenant AND f.project=l.project AND f.namespace=l.namespace WHERE l.logical_id=$1 AND l.tenant=$2 AND l.project=$3 AND l.namespace=$4 AND l.policy_version='projection-feedback-v1' ORDER BY l.projection_id LIMIT 9`, logical, scope.Tenant, scope.Project, scope.Namespace)
		if e != nil {
			t.Fatal("projection feedback identity observation unavailable")
		}
		defer rows.Close()
		ids := []string{}
		for rows.Next() {
			var id string
			if rows.Scan(&id) != nil {
				t.Fatal("projection feedback identity invalid")
			}
			ids = append(ids, id)
		}
		if rows.Err() != nil || len(ids) > 8 {
			t.Fatal("projection feedback bound exceeded")
		}
		return ids
	}
	nonactivation := func() {
		t.Helper()
		if scalar(`SELECT (SELECT count(*) FROM ranking_rollout_policies WHERE tenant=$1 AND project=$2 AND namespace=$3)+(SELECT count(*) FROM reserved_insight_activation_decisions WHERE tenant=$1 AND project=$2 AND namespace=$3)+(SELECT count(*) FROM canonical_memories WHERE tenant=$1 AND project=$2 AND namespace=$3 AND memory_path LIKE 'insights/%')`, scope.Tenant, scope.Project, scope.Namespace) != 0 {
			t.Fatal("projection activated ranking or insight without policy")
		}
		report.FeedbackNonactivationChecked = true
	}
	contextContract, e := NewProviderContextContract(spec)
	if e != nil {
		t.Fatal("projection context contract unavailable")
	}
	readContext := func(issue bool) provider.DisclosedContextResponse {
		t.Helper()
		input := provider.ContextInput{Query: "PC6 projection fixture", Budget: 10, Path: "pc6/references/main", ResponseMode: "bounded_content"}
		if issue {
			input.Continuity = &provider.ContextContinuityInput{Mode: "issue"}
		}
		data, status := call("POST", "/v1/provider/context", runtimeKey, provider.ContextRequest{Metadata: meta("context"), Input: input}, binding, nil)
		out, e := contextContract.ValidateForRequest(data, binding.Scope, "provider-v1", input, nil)
		if status != 200 || e != nil {
			t.Fatal("projection PC4 context response failed-contract")
		}
		return out
	}
	if phase == "pending" {
		if _, err := os.Stat(checkpointPath); !os.IsNotExist(err) {
			t.Fatal("PC6 requires a fresh projection checkpoint")
		}
		source := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}
		write := provider.ReferenceWriteRequest{Metadata: meta("reference-main"), Input: provider.ReferenceWriteInput{ContractVersion: DurableReferenceContractVersion, ReferenceID: "pc6-main", MemoryPath: "pc6/references/main", EventType: "provider.reference.task", Value: "PC6 projection fixture evidence 中文🙂", SourceMetadata: &source}}
		cp.EventID = reference(write, binding)
		init := provider.RuntimeInitialization{Scope: binding.Scope, AgentID: "pc6-other-agent", SessionID: "pc6-other-session", ConversationID: "pc6-other-conversation", ProviderInstanceID: "pc6-other-instance"}
		data, status := call("POST", "/v1/provider/runtime", runtimeKey, init, provider.RuntimeBinding{}, nil)
		if status != 201 || json.Unmarshal(data, &cp.ForeignBinding) != nil || cp.ForeignBinding.Validate(time.Now()) != nil {
			t.Fatal("foreign binding fixture unavailable")
		}
		write.Metadata = meta("reference-foreign")
		write.Input.ReferenceID = "pc6-foreign"
		write.Input.MemoryPath = "pc6/references/foreign"
		cp.ForeignEventID = reference(write, cp.ForeignBinding)
		issued := readContext(true)
		if issued.Result.Continuity.Outcome != "issued" {
			t.Fatal("PC6 receipt not issued")
		}
		cp.Request = provider.ProjectionRequest{Metadata: meta("projection-r1"), Input: provider.ProjectionInput{ContractVersion: ProjectionContractVersion, Kind: "feedback", EffectID: "pc6-main-effect", SourceRevision: 1, OccurredAt: "2026-10-16T00:00:00Z", ReportedOutcome: "unknown", References: []provider.ProjectionReferenceSelector{{EventID: cp.EventID}}, Feedback: &provider.ProjectionFeedback{Signal: "useful", ReasonCode: "pc6_observed"}, ContextDigest: issued.Result.ContextDigest.Value, ContextReference: issued.Result.Continuity.Reference, Provenance: []string{"pc6-source-a", "pc6-source-b"}}}
		cp.Initial = admit(cp.Request)
		if len(cp.Initial.References) != 1 || cp.Initial.References[0].State != "pending" || cp.Initial.References[0].Eligible || cp.Initial.References[0].Lineage != nil || cp.Initial.ContextDigestAttribution != "verified_receipt" || cp.Initial.FeedbackState != "pending" || cp.Initial.Disposition != "accepted" {
			t.Fatal("pending projection conflated completion or receipt")
		}
		replay := cp.Request
		replay.Input.Provenance = []string{"pc6-source-b", "pc6-source-a"}
		if r := admit(replay); r.ProjectionID != cp.Initial.ProjectionID || r.LogicalID != cp.Initial.LogicalID {
			t.Fatal("normalized pending replay changed identity")
		}
		if len(feedbackIDs(cp.Initial.LogicalID)) != 0 {
			t.Fatal("pending reference materialized feedback")
		}
		nonactivation()
		before := issued.Result.ContextDigest.Value
		after := readContext(false).Result.ContextDigest.Value
		if before != after {
			t.Fatal("pending projection changed public context ranking")
		}
		if os.WriteFile(checkpointPath, encode(cp), 0600) != nil {
			t.Fatal("owned projection checkpoint write unavailable")
		}
		report.PendingChecked = true
		report.ContextReceiptChecked = true
		report.IdentityHash = contextLiveHash([]string{cp.Initial.LogicalID, cp.Initial.ProjectionID, cp.EventID})
		report.Result = "pending_checked"
		report.Checks = []string{"served_closed_contract", "full_capability_negotiation", "migration_checksum_manifest", "public_pending_reference", "normalized_equivalent_replay", "pending_feedback_excluded", "pc4_verified_receipt", "public_context_unchanged"}
		return
	}
	readSecret("pc6-projection.json", &cp)
	if !validUUID(cp.EventID) || !validUUID(cp.Initial.LogicalID) || len(cp.Request.Input.References) != 1 || len(cp.Initial.References) != 1 || cp.Request.Input.References[0].EventID != cp.EventID || cp.Initial.References[0].State != "pending" {
		t.Fatal("pending projection checkpoint invalid")
	}
	var previous projectionLiveReport
	previousData, err := os.ReadFile(reportPath)
	if err != nil || json.Unmarshal(previousData, &previous) != nil || previous.OpenAPIDigest != report.OpenAPIDigest || previous.ImageDigest != report.ImageDigest || previous.SourceDigest != report.SourceDigest || previous.ScopeHash != report.ScopeHash || previous.MigrationManifestDigest != report.MigrationManifestDigest || !previous.MigrationIntegrityVerified {
		t.Fatal("projection prior-stage provenance unavailable")
	}
	if stage == "initial" && (previous.Result != "pending_checked" || !previous.PendingChecked) || stage == "restart" && (previous.Result != "passed" || previous.Stage != "initial" || !previous.AsyncFeedbackChecked) {
		t.Fatal("projection prior-stage evidence incomplete")
	}
	statusReq := provider.ReferenceStatusRequest{Metadata: meta("reference-status"), Input: provider.ReferenceStatusInput{ContractVersion: DurableReferenceContractVersion, EventID: cp.EventID, ResponseMode: "reference_only"}}
	var completed ReferenceConsumerResponse
	deadline := time.Now().Add(60 * time.Second)
	for {
		data, status := call("POST", "/v1/provider/references/status", runtimeKey, statusReq, binding, nil)
		completed, e = refContract.Validate(data, scope, "provider-v1", "reference_only")
		if status != 200 || e != nil {
			t.Fatal("PC6 governed reference status failed-contract")
		}
		if completed.Status == "completed" {
			break
		}
		if completed.Status == "failed" || completed.Status == "unavailable" || time.Now().After(deadline) {
			t.Fatal("PC6 public reference governance incomplete")
		}
		time.Sleep(250 * time.Millisecond)
	}
	if completed.Reference == nil || completed.Reference.MemoryPath != "pc6/references/main" {
		t.Fatal("PC6 reference completion lost exact path")
	}
	replayed := admit(cp.Request)
	if replayed.ProjectionID != cp.Initial.ProjectionID || replayed.LogicalID != cp.Initial.LogicalID || replayed.References[0].State != "pending" || replayed.CurrentReferences[0].State != "completed" || !replayed.CurrentReferences[0].Eligible || replayed.CurrentReferences[0].Lineage == nil {
		t.Fatal("pending restart replay lost historical/current attribution")
	}
	report.PendingChecked = true
	report.PendingToCompletedChecked = true
	report.ContextReceiptChecked = replayed.ContextDigestAttribution == "verified_receipt"
	if !report.ContextReceiptChecked {
		t.Fatal("PC4 receipt attribution did not survive restart")
	}
	deadline = time.Now().Add(60 * time.Second)
	wantFeedback := 1
	if stage == "restart" {
		wantFeedback = 2
	}
	for len(feedbackIDs(cp.Initial.LogicalID)) != wantFeedback {
		if time.Now().After(deadline) {
			t.Fatal("ordinary asynchronous PC6 feedback mapping incomplete")
		}
		time.Sleep(250 * time.Millisecond)
	}
	expectError := func(req provider.ProjectionRequest, want int, errorCode string, headers http.Header) {
		t.Helper()
		payload := encode(req)
		if _, e := contract.ValidateRequest(payload, "provider-v1"); e != nil {
			t.Fatal("negative serialized projection fixture invalid")
		}
		data, status := call("POST", "/v1/provider/projections", runtimeKey, payload, binding, headers)
		bounded, err := contract.ValidateError(data, status)
		if status != want || err != nil || bounded.Code != errorCode {
			t.Fatalf("projection rejection category mismatch HTTP %d", status)
		}
	}
	conflict := cp.Request
	conflict.Input.ReportedOutcome = "failed"
	expectError(conflict, 409, "projection_conflict", nil)
	wrongDigest := cp.Request
	wrongDigest.Metadata = meta("receipt-mismatch")
	wrongDigest.Input.EffectID = "pc6-receipt-mismatch"
	wrongDigest.Input.ContextDigest = strings.Repeat("0", 64)
	expectError(wrongDigest, 409, "projection_conflict", nil)
	missingReceipt := cp.Request
	missingReceipt.Metadata = meta("receipt-unavailable")
	missingReceipt.Input.EffectID = "pc6-receipt-unavailable"
	missingReceipt.Input.ContextReference = "ctxc_00000000000000000000000000000000"
	expectError(missingReceipt, 410, "projection_unavailable", nil)
	foreignReceipt := cp.Request
	foreignReceipt.Metadata = meta("receipt-foreign")
	foreignReceipt.Input.EffectID = "pc6-receipt-foreign"
	foreignReceiptPayload := encode(foreignReceipt)
	if _, err := contract.ValidateRequest(foreignReceiptPayload, "provider-v1"); err != nil {
		t.Fatal("foreign receipt fixture failed-contract")
	}
	foreignReceiptData, foreignReceiptCode := call("POST", "/v1/provider/projections", runtimeKey, foreignReceiptPayload, cp.ForeignBinding, nil)
	if out, err := contract.ValidateError(foreignReceiptData, foreignReceiptCode); err != nil || foreignReceiptCode != 410 || out.Code != "projection_unavailable" {
		t.Fatal("foreign PC4 receipt did not fail closed")
	}
	// Validate every transport identity override independently before any write.
	deniedHistory := encode(provider.ProjectionHistoryRequest{Metadata: meta("denied-history"), Input: provider.ProjectionHistoryInput{ContractVersion: ProjectionContractVersion, Kind: "feedback", EffectID: cp.Request.Input.EffectID, Limit: 8}})
	if _, err := contract.ValidateHistoryRequest(deniedHistory, "provider-v1"); err != nil {
		t.Fatal("denied history fixture failed-contract")
	}
	for _, header := range []string{"X-Stele-Tenant", "X-Stele-Project", "X-Stele-Namespace", provider.HeaderRuntimeBinding, provider.HeaderRuntimeSession, provider.HeaderRuntimeAgent, provider.HeaderRuntimeConversation} {
		data, status := call("POST", "/v1/provider/projections", runtimeKey, encode(cp.Request), binding, http.Header{header: []string{"pc6-foreign"}})
		if _, err := contract.ValidateError(data, status); status != 403 || err != nil {
			t.Fatalf("projection exact-binding override HTTP %d", status)
		}
		data, status = call("POST", "/v1/provider/projections/history", runtimeKey, deniedHistory, binding, http.Header{header: []string{"pc6-foreign"}})
		if _, err := contract.ValidateError(data, status); status != 403 || err != nil {
			t.Fatal("projection history exact-binding override not denied")
		}
	}
	if len(history("feedback", cp.Request.Input.EffectID, 8, cp.ForeignBinding).Records) != 0 || len(history("feedback", "pc6-absent-effect", 8, binding).Records) != 0 {
		t.Fatal("foreign and missing projection history diverged")
	}
	report.ExactScopeChecked = true
	// Foreign same-tenant references and nonexistent UUIDs have identical closed
	// unavailable projections. Neither can become eligible feedback evidence.
	for n, event := range []string{cp.ForeignEventID, "ffffffff-ffff-4fff-8fff-ffffffffffff"} {
		req := cp.Request
		req.Metadata = meta("unavailable-" + strconv.Itoa(n))
		req.Input.EffectID = "pc6-unavailable-" + strconv.Itoa(n)
		req.Input.References = []provider.ProjectionReferenceSelector{{EventID: event}}
		req.Input.ContextDigest = ""
		req.Input.ContextReference = ""
		r := admit(req)
		if len(r.References) != 1 || r.References[0].State != "unavailable" || r.References[0].EventID != "" || r.References[0].Eligible || r.References[0].Lineage != nil || r.CurrentReferences[0].State != "unavailable" || len(feedbackIDs(r.LogicalID)) != 0 {
			t.Fatal("foreign or absent reference disclosed existence or feedback")
		}
	}
	beforeContext := readContext(false).Result.ContextDigest.Value
	correction := cp.Request
	correction.Metadata = meta("projection-r2")
	correction.Input.SourceRevision = 2
	correction.Input.ReportedOutcome = "succeeded"
	correction.Input.Feedback = &provider.ProjectionFeedback{Signal: "needs_review", ReasonCode: "pc6_corrected"}
	corrected := admit(correction)
	if corrected.LogicalID != cp.Initial.LogicalID || corrected.ProjectionID == cp.Initial.ProjectionID || corrected.Disposition != "corrected" || corrected.SupersedesID != cp.Initial.ProjectionID || corrected.SourceRevision != 2 || corrected.References[0].State != "completed" || !corrected.References[0].Eligible {
		t.Fatal("projection correction lost append-only lineage")
	}
	if stage == "initial" {
		cp.Corrected = corrected
		if os.WriteFile(checkpointPath, encode(cp), 0600) != nil {
			t.Fatal("corrected owned checkpoint unavailable")
		}
	} else if cp.Corrected.ProjectionID != corrected.ProjectionID || cp.Corrected.LogicalID != corrected.LogicalID {
		t.Fatal("restart correction replay changed identity")
	}
	if again := admit(correction); again.ProjectionID != corrected.ProjectionID {
		t.Fatal("correction replay changed revision identity")
	}
	if after := readContext(false).Result.ContextDigest.Value; after != beforeContext {
		t.Fatal("projection correction changed public context ranking")
	}
	nonactivation()
	stale := cp.Request
	stale.Metadata = meta("stale-r1")
	expectError(stale, 409, "projection_stale", nil)
	full := history("feedback", cp.Request.Input.EffectID, 8, binding)
	page := history("feedback", cp.Request.Input.EffectID, 1, binding)
	if len(full.Records) != 2 || full.HasMore || full.Records[0].ProjectionID != corrected.ProjectionID || full.Records[1].ProjectionID != cp.Initial.ProjectionID || !page.HasMore || len(page.Records) != 1 || page.Records[0].ProjectionID != corrected.ProjectionID {
		t.Fatal("projection bounded correction history incomplete")
	}
	report.CorrectionHistoryChecked = true
	deadline = time.Now().Add(60 * time.Second)
	for len(feedbackIDs(cp.Initial.LogicalID)) != 2 {
		if time.Now().After(deadline) {
			t.Fatal("corrected asynchronous feedback mapping incomplete")
		}
		time.Sleep(250 * time.Millisecond)
	}
	feedback := feedbackIDs(cp.Initial.LogicalID)
	if !validUUID(feedback[0]) || !validUUID(feedback[1]) || feedback[0] == feedback[1] {
		t.Fatal("feedback correction identity invalid")
	}
	if scalar(`SELECT count(*) FROM provider_projection_identities WHERE logical_id=$1 AND binding_id=$2 AND principal_id=$3 AND tenant=$4 AND project=$5 AND namespace=$6 AND agent_id=$7 AND session_id=$8 AND conversation_id=$9 AND provider_instance_id=$10 AND effect_id=$11`, cp.Initial.LogicalID, binding.BindingID, binding.PrincipalID, scope.Tenant, scope.Project, scope.Namespace, scope.AgentID, scope.SessionID, scope.ConversationID, scope.ProviderInstanceID, cp.Request.Input.EffectID) != 1 || scalar(`SELECT count(*) FROM provider_projection_revisions WHERE logical_id=$1 AND tenant=$2 AND project=$3 AND namespace=$4`, cp.Initial.LogicalID, scope.Tenant, scope.Project, scope.Namespace) != 2 {
		t.Fatal("privileged projection identity/revision cardinality mismatch")
	}
	if scalar(`SELECT count(*) FROM provider_projection_revisions WHERE projection_id=$1 AND supersedes_id=$2 AND logical_id=$3 AND source_revision=2 AND tenant=$4 AND project=$5 AND namespace=$6`, corrected.ProjectionID, cp.Initial.ProjectionID, cp.Initial.LogicalID, scope.Tenant, scope.Project, scope.Namespace) != 1 {
		t.Fatal("privileged correction provenance unavailable")
	}
	if scalar(`SELECT count(*) FROM eligible_usefulness_feedback f JOIN provider_projection_feedback_links l ON l.feedback_id=f.id WHERE l.logical_id=$1 AND l.tenant=$2 AND l.project=$3 AND l.namespace=$4 AND f.superseded_at IS NULL`, cp.Initial.LogicalID, scope.Tenant, scope.Project, scope.Namespace) != 1 {
		t.Fatal("correction retained stale eligible feedback")
	}
	report.IdentityHash = contextLiveHash([]string{cp.Initial.LogicalID, cp.Initial.ProjectionID, corrected.ProjectionID, cp.EventID, completed.Reference.CandidateID, completed.Reference.MemoryID, completed.Reference.VersionID, completed.Reference.ProvenanceID})
	report.FeedbackIdentityHash = contextLiveHash(feedback)
	report.AsyncFeedbackChecked = true
	if stage == "restart" {
		if previous.IdentityHash != report.IdentityHash || previous.FeedbackIdentityHash != report.FeedbackIdentityHash || !previous.PendingToCompletedChecked {
			t.Fatal("API/worker restart changed evidence or duplicated feedback")
		}
		report.RestartChecked = true
		_, status := call("POST", "/v1/admin/memories/"+completed.Reference.MemoryID+":suppress", adminKey, map[string]string{"reason": "owned PC6 lifecycle verification"}, provider.RuntimeBinding{}, nil)
		if status != 200 {
			t.Fatalf("PC6 public lifecycle suppression HTTP status %d", status)
		}
		hidden := history("feedback", cp.Request.Input.EffectID, 8, binding)
		for _, r := range hidden.Records {
			if len(r.References) != 1 || r.References[0].State != "unavailable" || r.References[0].EventID != "" || r.References[0].Lineage != nil || r.CurrentReferences[0].State != "unavailable" || r.CurrentReferences[0].Eligible {
				t.Fatal("projection history resurrected lifecycle-hidden attribution")
			}
		}
		if scalar(`SELECT count(*) FROM eligible_usefulness_feedback f JOIN provider_projection_feedback_links l ON l.feedback_id=f.id WHERE l.logical_id=$1 AND l.tenant=$2 AND l.project=$3 AND l.namespace=$4 AND f.superseded_at IS NULL`, cp.Initial.LogicalID, scope.Tenant, scope.Project, scope.Namespace) != 0 {
			t.Fatal("lifecycle-hidden projection feedback stayed eligible")
		}
		if scalar(`SELECT count(*) FROM provider_projection_revisions r JOIN provider_projection_feedback_links l USING(projection_id,logical_id,tenant,project,namespace) JOIN usefulness_feedback f ON f.id=l.feedback_id AND f.tenant=l.tenant AND f.project=l.project AND f.namespace=l.namespace WHERE r.logical_id=$1 AND r.tenant=$2 AND r.project=$3 AND r.namespace=$4 AND r.admission->'references'->0->>'event_id'=$5`, cp.Initial.LogicalID, scope.Tenant, scope.Project, scope.Namespace, cp.EventID) != 2 {
			t.Fatal("privileged history lost immutable attribution or feedback")
		}
		if scalar(`SELECT count(*) FROM canonical_memories m JOIN memory_versions v ON v.memory_id=m.id JOIN provenance_links p ON p.memory_id=m.id WHERE m.id=$1 AND v.id=$2 AND p.id=$3 AND p.raw_event_id=$4 AND p.candidate_memory_id=$5 AND m.tenant=$6 AND m.project=$7 AND m.namespace=$8 AND m.state='suppressed'`, completed.Reference.MemoryID, completed.Reference.VersionID, completed.Reference.ProvenanceID, cp.EventID, completed.Reference.CandidateID, scope.Tenant, scope.Project, scope.Namespace) != 1 {
			t.Fatal("privileged lifecycle history lost governed reference provenance")
		}
		if len(feedbackIDs(cp.Initial.LogicalID)) != 2 {
			t.Fatal("lifecycle check duplicated feedback")
		}
		report.LifecycleChecked = true
		report.PrivilegedHistoryChecked = true
	}
	nonactivation()
	report.Checks = []string{"served_closed_contract", "full_capability_negotiation", "migration_checksum_manifest", "pending_after_api_worker_recovery", "pending_to_completed_current_attribution", "normalized_equivalent_replay", "conflicting_replay", "greater_revision_correction", "stale_delivery", "bounded_history", "full_binding_scope_denials", "foreign_and_absent_reference_unavailable", "pc4_verified_receipt", "pc4_invalid_receipt_denials", "strict_error_contract", "public_context_ranking_unchanged", "async_versioned_feedback", "one_logical_two_revisions_two_feedback"}
	if stage == "restart" {
		report.Checks = append(report.Checks, "api_worker_restart_stable_identities", "lifecycle_current_unavailable", "lifecycle_feedback_excluded", "privileged_immutable_history")
	}
	report.Result = "passed"
}
