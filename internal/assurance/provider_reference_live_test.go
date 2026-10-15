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
	"github.com/jackc/pgx/v5"
)

type referenceLiveReport struct {
	Result                     string   `json:"result"`
	Stage                      string   `json:"stage"`
	ContractVersion            string   `json:"contract_version"`
	SchemaVersion              string   `json:"schema_version"`
	FixtureVersion             string   `json:"fixture_version"`
	MigrationVersion           int64    `json:"migration_version"`
	MigrationManifestDigest    string   `json:"migration_manifest_digest"`
	MigrationIntegrityVerified bool     `json:"migration_integrity_verified"`
	OpenAPIDigest              string   `json:"openapi_digest"`
	ImageDigest                string   `json:"image_digest"`
	SourceRevision             string   `json:"source_revision"`
	SourceDigest               string   `json:"source_digest"`
	ScopeHash                  string   `json:"scope_hash"`
	IdentityHash               string   `json:"identity_hash"`
	GovernanceCompleted        int      `json:"governance_completed"`
	DurableLineageChecked      bool     `json:"durable_lineage_checked"`
	RestartChecked             bool     `json:"restart_checked"`
	LifecycleChecked           bool     `json:"lifecycle_checked"`
	SelectorReplacementChecked bool     `json:"selector_replacement_checked"`
	HistoryRetentionChecked    bool     `json:"history_retention_checked"`
	Checks                     []string `json:"checks"`
}

// The fixture shares only the official bootstrap and owned runtime binding
// with PC1-PC4. Reference data is created through ordinary public HTTP writes.
func TestProviderReferenceFreshPostgresLive(t *testing.T) {
	base, stage, reportPath, dir := os.Getenv("STELE_CONTEXT_VERIFY_URL"), os.Getenv("STELE_CONTEXT_VERIFY_STAGE"), os.Getenv("STELE_REFERENCE_VERIFY_REPORT"), os.Getenv("STELE_CONTEXT_VERIFY_CREDENTIAL_DIR")
	if base == "" && reportPath == "" {
		t.Skip("owned live fixture not configured")
	}
	if base == "" || reportPath == "" || dir == "" || (stage != "initial" && stage != "restart") {
		t.Fatal("reference live verifier configuration missing")
	}
	credentials := func(name string) string {
		b, e := os.ReadFile(filepath.Join(dir, name+".credential"))
		if e != nil || len(b) == 0 {
			t.Fatal("live credential unavailable")
		}
		return strings.TrimSpace(string(b))
	}
	runtimeKey, adminKey := credentials("runtime"), credentials("admin")
	var binding provider.RuntimeBinding
	b, err := os.ReadFile(filepath.Join(dir, "context-binding.json"))
	if err != nil || json.Unmarshal(b, &binding) != nil {
		t.Fatal("runtime binding checkpoint unavailable")
	}
	scope := provider.CanonicalScopeDescriptor{Tenant: binding.Scope.Tenant, Project: binding.Scope.Project, Namespace: binding.Scope.Namespace, AgentID: binding.AgentID, SessionID: binding.SessionID, ConversationID: binding.ConversationID, ProviderInstanceID: binding.ProviderInstanceID}
	if scope.Validate() != nil {
		t.Fatal("reference exact scope unavailable")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	call := func(method, path, key string, body any, foreign bool) ([]byte, int) {
		var data []byte
		if body != nil {
			data, err = json.Marshal(body)
			if err != nil {
				t.Fatal("reference fixture encoding failed")
			}
		}
		req, e := http.NewRequest(method, base+path, bytes.NewReader(data))
		if e != nil {
			t.Fatal("reference request invalid")
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
		req.Header.Set("X-Stele-Actor", "provider-reference-verifier")
		req.Header.Set(provider.HeaderRuntimeBinding, binding.BindingID)
		req.Header.Set(provider.HeaderRuntimeSession, binding.SessionID)
		res, e := client.Do(req)
		if e != nil {
			t.Fatal("reference HTTP unavailable")
		}
		defer res.Body.Close()
		out, e := io.ReadAll(io.LimitReader(res.Body, 4<<20+1))
		if e != nil || len(out) > 4<<20 {
			t.Fatal("reference response unavailable")
		}
		return out, res.StatusCode
	}
	spec, code := call("GET", "/openapi.yaml", "", nil, false)
	if code != 200 {
		t.Fatal("served reference OpenAPI unavailable")
	}
	contract, err := NewProviderReferenceContract(spec)
	if err != nil {
		t.Fatal(err)
	}
	local, err := DefaultProviderReferenceContract()
	if err != nil || local.digest != contract.digest {
		t.Fatal("image and local reference schemas differ")
	}
	caps, code := call("GET", "/v1/provider/capabilities", runtimeKey, nil, false)
	var capDoc map[string]json.RawMessage
	if code != 200 || json.Unmarshal(caps, &capDoc) != nil {
		t.Fatal("reference capability unavailable")
	}
	raw, _ := json.Marshal(map[string]json.RawMessage{"durable_references": capDoc["durable_references"]})
	if ValidateProviderReferenceCapability(raw) != nil {
		t.Fatal("reference capability incompatible")
	}
	var schemaDigest string
	_ = json.Unmarshal(capDoc["schema_digest"], &schemaDigest)
	if schemaDigest != "sha256:"+contract.digest {
		t.Fatal("reference capability digest mismatch")
	}
	report := referenceLiveReport{Result: "failed", Stage: stage, ContractVersion: DurableReferenceContractVersion, OpenAPIDigest: contract.digest, ImageDigest: os.Getenv("STELE_CONTEXT_VERIFY_IMAGE_DIGEST"), SourceRevision: os.Getenv("STELE_CONTEXT_VERIFY_SOURCE_REVISION"), SourceDigest: os.Getenv("STELE_CONTEXT_VERIFY_SOURCE_DIGEST"), ScopeHash: contextLiveHash(scope)}
	_ = json.Unmarshal(capDoc["schema_version"], &report.SchemaVersion)
	report.FixtureVersion = "provider-durable-reference-live-v1"
	defer func() {
		if t.Failed() {
			report.Result = "failed"
		}
		out, _ := json.MarshalIndent(report, "", "  ")
		if os.WriteFile(reportPath, out, 0600) != nil {
			t.Error("bounded reference report unavailable")
		}
	}()
	if !strings.HasPrefix(report.ImageDigest, "sha256:") || len(report.SourceRevision) != 40 || !validContextDigest(report.SourceDigest) {
		t.Fatal("reference build provenance missing")
	}
	if report.SchemaVersion != "provider-v1" {
		t.Fatal("reference Provider schema incompatible")
	}
	var previous referenceLiveReport
	if stage == "restart" {
		b, e := os.ReadFile(reportPath)
		if e != nil || json.Unmarshal(b, &previous) != nil || previous.Result != "passed" || previous.Stage != "initial" || previous.OpenAPIDigest != report.OpenAPIDigest || previous.ImageDigest != report.ImageDigest || previous.SourceDigest != report.SourceDigest || previous.ScopeHash != report.ScopeHash || !previous.DurableLineageChecked {
			t.Fatal("initial reference evidence unavailable")
		}
	}
	metadata := func(id string) provider.OperationMetadata {
		return provider.OperationMetadata{RequestID: "pc5-" + id, OperationID: "pc5-" + id, IdempotencyKey: "pc5-" + id, SchemaVersion: "provider-v1"}
	}
	decode := func(data []byte, code, wantCode int, mode string) ReferenceConsumerResponse {
		if code != wantCode {
			t.Fatalf("reference HTTP status %d, expected %d", code, wantCode)
		}
		r, e := contract.Validate(data, scope, "provider-v1", mode)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := pgx.Connect(ctx, os.Getenv("STELE_CONTEXT_VERIFY_POSTGRES_DSN"))
	if err != nil {
		t.Fatal("owned PostgreSQL reference evidence unavailable")
	}
	defer db.Close(context.Background())
	var migrationVersion int64
	var migrationDirty bool
	err = db.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&migrationVersion, &migrationDirty)
	if err != nil || migrationDirty {
		t.Fatal("reference migration integrity not verified")
	}
	assets, err := filepath.Glob("../storage/postgres/migrations/*.up.sql")
	if err != nil {
		t.Fatal("reference migration manifest unavailable")
	}
	type migrationAsset struct {
		Version        int64
		Name           string
		ChecksumSHA256 string
	}
	manifest := make([]migrationAsset, 0, len(assets))
	for index, path := range assets {
		name := filepath.Base(path)
		prefix, _, _ := strings.Cut(name, "_")
		version, e := strconv.ParseInt(prefix, 10, 64)
		contents, readErr := os.ReadFile(path)
		if e != nil || readErr != nil || version != int64(index+1) {
			t.Fatal("reference migration source manifest invalid")
		}
		digest := sha256.Sum256(contents)
		asset := migrationAsset{Version: version, Name: name, ChecksumSHA256: hex.EncodeToString(digest[:])}
		var matches int
		err = db.QueryRow(ctx, `SELECT count(*) FROM stele_schema_migration_ledger WHERE version=$1 AND migration_name=$2 AND checksum_sha256=$3`, asset.Version, asset.Name, asset.ChecksumSHA256).Scan(&matches)
		if err != nil || matches != 1 {
			t.Fatal("reference migration checksum mismatch")
		}
		manifest = append(manifest, asset)
	}
	var ledgerRows int
	err = db.QueryRow(ctx, `SELECT count(*) FROM stele_schema_migration_ledger`).Scan(&ledgerRows)
	if err != nil || len(manifest) == 0 || migrationVersion != int64(len(manifest)) || ledgerRows != len(manifest) {
		t.Fatal("reference migration ledger differs from source manifest")
	}
	report.MigrationVersion = migrationVersion
	report.MigrationManifestDigest = contextLiveHash(manifest)
	report.MigrationIntegrityVerified = true
	if stage == "restart" && (previous.SchemaVersion != report.SchemaVersion || previous.FixtureVersion != report.FixtureVersion || previous.MigrationVersion != report.MigrationVersion || previous.MigrationManifestDigest != report.MigrationManifestDigest || !previous.MigrationIntegrityVerified) {
		t.Fatal("restart changed reference schema, migration or fixture provenance")
	}
	records := map[string]ReferenceConsumerRecord{}
	identities := []string{}
	var firstWrite provider.ReferenceWriteRequest
	for _, kind := range []string{"task", "run", "handoff", "compilation"} {
		id, path, value := "pc5-"+kind, "pc5/references/"+kind, "Durable "+kind+" fixture evidence 中文🙂"
		source := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}
		write := provider.ReferenceWriteRequest{Metadata: metadata("write-" + kind), Input: provider.ReferenceWriteInput{ContractVersion: DurableReferenceContractVersion, ReferenceID: id, MemoryPath: path, EventType: "provider.reference." + kind, Value: value, SourceMetadata: &source}}
		write.Metadata.EventSeq = int64(len(records) + 1)
		if kind == "task" {
			firstWrite = write
		}
		data, code := call("POST", "/v1/provider/references", runtimeKey, write, false)
		ack := decode(data, code, 202, "reference_only")
		data, code = call("POST", "/v1/provider/references", runtimeKey, write, false)
		replay := decode(data, code, 202, "reference_only")
		if replay.EventID != ack.EventID || !replay.Replayed {
			t.Fatal("reference replay changed event identity")
		}
		changed := write
		changed.Input.Value += " changed"
		_, code = call("POST", "/v1/provider/references", runtimeKey, changed, false)
		if code != 409 {
			t.Fatal("reference changed replay was not rejected")
		}
		statusRequest := provider.ReferenceStatusRequest{Metadata: metadata("status-" + kind), Input: provider.ReferenceStatusInput{ContractVersion: DurableReferenceContractVersion, EventID: ack.EventID, ResponseMode: "reference_only"}}
		var completed ReferenceConsumerResponse
		deadline := time.Now().Add(30 * time.Second)
		for {
			data, code = call("POST", "/v1/provider/references/status", runtimeKey, statusRequest, false)
			completed = decode(data, code, 200, "reference_only")
			if completed.Status == "completed" {
				break
			}
			if completed.Status == "failed" || completed.Status == "unavailable" || time.Now().After(deadline) {
				t.Fatal("reference governance did not complete")
			}
			time.Sleep(250 * time.Millisecond)
		}
		r := *completed.Reference
		if r.ReferenceID != id || r.MemoryPath != path || r.EventID != ack.EventID || r.EventType != "provider.reference."+kind {
			t.Fatal("reference promotion changed identity or path")
		}
		lookup := provider.ReferenceLookupRequest{Metadata: metadata("lookup-" + kind), Input: provider.ReferenceLookupInput{ContractVersion: DurableReferenceContractVersion, ReferenceID: id, MemoryPath: path, ResponseMode: "bounded_content"}}
		data, code = call("POST", "/v1/provider/references/lookup", runtimeKey, lookup, false)
		found := decode(data, code, 200, "bounded_content")
		if found.Status != "completed" || found.Reference == nil || found.Reference.Value == nil || *found.Reference.Value != value || found.Reference.VersionID != r.VersionID {
			t.Fatal("reference exact content round trip failed")
		}
		lookup.Input.MemoryPath = path + "/descendant"
		data, code = call("POST", "/v1/provider/references/lookup", runtimeKey, lookup, false)
		if decode(data, code, 200, "bounded_content").Status != "unavailable" {
			t.Fatal("reference lookup broadened exact path")
		}
		lookup.Input.MemoryPath = path
		_, code = call("POST", "/v1/provider/references/lookup", runtimeKey, lookup, true)
		if code != 403 {
			t.Fatal("foreign reference scope was not denied")
		}
		var rows int
		err = db.QueryRow(ctx, `SELECT count(*) FROM raw_events r JOIN candidate_memories c ON c.id=$2 AND c.source_raw_event_id=r.id JOIN canonical_memories m ON m.id=$3 JOIN memory_versions v ON v.id=$4 AND v.memory_id=m.id JOIN provenance_links p ON p.id=$5 AND p.raw_event_id=r.id AND p.candidate_memory_id=c.id AND p.memory_id=m.id WHERE r.id=$1 AND r.tenant=$6 AND r.project=$7 AND r.namespace=$8 AND c.tenant=r.tenant AND c.project=r.project AND c.namespace=r.namespace AND m.tenant=r.tenant AND m.project=r.project AND m.namespace=r.namespace AND p.tenant=r.tenant AND p.project=r.project AND p.namespace=r.namespace AND r.governance_processed_at IS NOT NULL AND c.status='promoted' AND m.state='active' AND v.state='active' AND v.version=$9 AND v.version=(SELECT max(version) FROM memory_versions WHERE memory_id=m.id) AND r.memory_path=$10 AND c.memory_path=r.memory_path AND m.memory_path=r.memory_path AND v.memory_path=r.memory_path AND p.operation='promote_candidate'`, r.EventID, r.CandidateID, r.MemoryID, r.VersionID, r.ProvenanceID, scope.Tenant, scope.Project, scope.Namespace, r.Version, path).Scan(&rows)
		if err != nil || rows != 1 {
			t.Fatal("reference durable UUID/path/promotion lineage incomplete")
		}
		records[kind] = r
		identities = append(identities, r.EventID+"|"+r.CandidateID+"|"+r.MemoryID+"|"+r.VersionID+"|"+r.ProvenanceID)
		report.GovernanceCompleted++
	}
	// An older equivalent request still resolves through durable idempotency
	// after newer writes, including when this sequence repeats after restart.
	data, code := call("POST", "/v1/provider/references", runtimeKey, firstWrite, false)
	replayedFirst := decode(data, code, 202, "reference_only")
	if !replayedFirst.Replayed || replayedFirst.EventID != records["task"].EventID {
		t.Fatal("older reference replay lost its durable identity")
	}
	report.IdentityHash = contextLiveHash(identities)
	report.DurableLineageChecked = true
	if stage == "restart" {
		if previous.IdentityHash != report.IdentityHash || previous.GovernanceCompleted != report.GovernanceCompleted {
			t.Fatal("restart changed durable reference lineage")
		}
		report.RestartChecked = true
		// A later independently governed episodic source replaces the exact
		// selector without rewriting the previous canonical memory in place.
		oldHandoff := records["handoff"]
		newValue := "Replacement handoff evidence after restart 中文🙂"
		source := memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustUntrusted, Classification: memory.SourceClassificationLow}
		replacement := provider.ReferenceWriteRequest{Metadata: metadata("write-handoff-replacement"), Input: provider.ReferenceWriteInput{ContractVersion: DurableReferenceContractVersion, ReferenceID: oldHandoff.ReferenceID, MemoryPath: oldHandoff.MemoryPath, EventType: oldHandoff.EventType, Value: newValue, SourceMetadata: &source}}
		data, code := call("POST", "/v1/provider/references", runtimeKey, replacement, false)
		ack := decode(data, code, 202, "reference_only")
		if ack.EventID == oldHandoff.EventID {
			t.Fatal("replacement reused the earlier source identity")
		}
		status := provider.ReferenceStatusRequest{Metadata: metadata("status-handoff-replacement"), Input: provider.ReferenceStatusInput{ContractVersion: DurableReferenceContractVersion, EventID: ack.EventID, ResponseMode: "reference_only"}}
		deadline := time.Now().Add(30 * time.Second)
		var current ReferenceConsumerResponse
		for {
			data, code = call("POST", "/v1/provider/references/status", runtimeKey, status, false)
			current = decode(data, code, 200, "reference_only")
			if current.Status == "completed" {
				break
			}
			if current.Status == "failed" || current.Status == "unavailable" || time.Now().After(deadline) {
				t.Fatal("replacement reference governance did not complete")
			}
			time.Sleep(250 * time.Millisecond)
		}
		if current.Reference == nil || current.Reference.EventID != ack.EventID || current.Reference.MemoryID == oldHandoff.MemoryID {
			t.Fatal("replacement did not create an independently governed canonical source")
		}
		lookup := provider.ReferenceLookupRequest{Metadata: metadata("lookup-handoff-replacement"), Input: provider.ReferenceLookupInput{ContractVersion: DurableReferenceContractVersion, ReferenceID: oldHandoff.ReferenceID, MemoryPath: oldHandoff.MemoryPath, ResponseMode: "bounded_content"}}
		data, code = call("POST", "/v1/provider/references/lookup", runtimeKey, lookup, false)
		found := decode(data, code, 200, "bounded_content")
		if found.Status != "completed" || found.Reference == nil || found.Reference.EventID != ack.EventID || found.Reference.Value == nil || *found.Reference.Value != newValue {
			t.Fatal("exact selector did not choose the latest governed source")
		}
		status.Input.EventID = oldHandoff.EventID
		data, code = call("POST", "/v1/provider/references/status", runtimeKey, status, false)
		older := decode(data, code, 200, "reference_only")
		if older.Status != "completed" || older.Reference == nil || older.Reference.VersionID != oldHandoff.VersionID {
			t.Fatal("replacement overwrote the earlier independently addressable source")
		}
		records["handoff"] = *current.Reference
		for _, fixture := range []struct{ kind, action, state string }{{"task", "suppress", "suppressed"}, {"run", "expire", "forgotten"}, {"compilation", "delete", "deleted"}, {"handoff", "suppress", "suppressed"}} {
			r := records[fixture.kind]
			_, code = call("POST", "/v1/admin/memories/"+r.MemoryID+":"+fixture.action, adminKey, map[string]string{"reason": "owned PC5 lifecycle verification"}, false)
			if code != 200 {
				t.Fatalf("reference lifecycle %s fixture HTTP status %d", fixture.action, code)
			}
			var currentState string
			err = db.QueryRow(ctx, `SELECT state FROM canonical_memories WHERE id=$1 AND tenant=$2 AND project=$3 AND namespace=$4`, r.MemoryID, scope.Tenant, scope.Project, scope.Namespace).Scan(&currentState)
			if err != nil || currentState != fixture.state {
				t.Fatal("reference lifecycle durable state mismatch")
			}
			lookup := provider.ReferenceLookupRequest{Metadata: metadata("removed-lookup-" + fixture.kind), Input: provider.ReferenceLookupInput{ContractVersion: DurableReferenceContractVersion, ReferenceID: r.ReferenceID, MemoryPath: r.MemoryPath, ResponseMode: "bounded_content"}}
			data, code := call("POST", "/v1/provider/references/lookup", runtimeKey, lookup, false)
			if decode(data, code, 200, "bounded_content").Status != "unavailable" {
				t.Fatal("reference lookup resurrected removed memory")
			}
			status := provider.ReferenceStatusRequest{Metadata: metadata("removed-status-" + fixture.kind), Input: provider.ReferenceStatusInput{ContractVersion: DurableReferenceContractVersion, EventID: r.EventID, ResponseMode: "bounded_content"}}
			data, code = call("POST", "/v1/provider/references/status", runtimeKey, status, false)
			if decode(data, code, 200, "bounded_content").Status != "unavailable" {
				t.Fatal("reference status disclosed removed memory")
			}
		}
		report.LifecycleChecked = true
		report.SelectorReplacementChecked = true
		var retained int
		err = db.QueryRow(ctx, `SELECT count(*) FROM canonical_memories m JOIN memory_versions v ON v.memory_id=m.id JOIN provenance_links p ON p.memory_id=m.id WHERE m.id=$1 AND v.id=$2 AND p.id=$3 AND p.raw_event_id=$4 AND p.candidate_memory_id=$5 AND m.tenant=$6 AND m.project=$7 AND m.namespace=$8 AND p.tenant=m.tenant AND p.project=m.project AND p.namespace=m.namespace AND m.memory_path=$9 AND v.memory_path=m.memory_path AND v.version=$10 AND p.operation='promote_candidate'`, oldHandoff.MemoryID, oldHandoff.VersionID, oldHandoff.ProvenanceID, oldHandoff.EventID, oldHandoff.CandidateID, scope.Tenant, scope.Project, scope.Namespace, oldHandoff.MemoryPath, oldHandoff.Version).Scan(&retained)
		if err != nil || retained != 1 {
			t.Fatal("privileged history lost earlier canonical version or provenance after replacement and removal")
		}
		report.HistoryRetentionChecked = true
	}
	report.Checks = []string{"served_schema_digest", "capability_negotiation", "migration_integrity_verified", "task_run_handoff_compilation", "governance_completed", "identical_replay", "older_sequence_durable_replay", "changed_replay_conflict", "exact_nonroot_path", "uuid_durable_lineage", "reference_only_omits_value", "bounded_content", "foreign_scope_denied"}
	if report.RestartChecked {
		report.Checks = append(report.Checks, "api_worker_restart_lineage", "suppressed_forgotten_deleted_unavailable", "same_selector_new_source", "hidden_latest_no_older_fallback", "privileged_history_retained")
	}
	report.Result = "passed"
}
