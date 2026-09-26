package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/governance"
	"github.com/FelixSeptem/stele/internal/mcp"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/retrieval"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	protocolmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// This integration test requires a dedicated, disposable PostgreSQL + pgvector
// database. It applies migrations but never creates or drops the database; the
// caller owns that lifecycle. It seeds a unique fixture scope and removes each
// created record with exact identifiers before returning.
func TestMCPForgetLedgerPostgresPersistsReviewAndReplayAcrossRepositories(t *testing.T) {
	dsn := os.Getenv("STELE_TEST_POSTGRES_MCP_DSN")
	if dsn == "" {
		t.Skip("STELE_TEST_POSTGRES_MCP_DSN is not configured; skipping MCP PostgreSQL + pgvector conformance test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := NewMigrationRunner().Apply(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	var vectorVersion string
	if err := pool.QueryRow(ctx, `SELECT extversion FROM pg_extension WHERE extname='vector'`).Scan(&vectorVersion); err != nil || vectorVersion == "" {
		t.Fatalf("pgvector extension version=%q error=%v, want an owned pgvector database", vectorVersion, err)
	}

	suffix := uuid.NewString()
	scope := memory.Scope{Tenant: "mcp-it-" + suffix, Project: "project", Namespace: "namespace"}
	foreignScope := scope
	foreignScope.Tenant = "mcp-foreign-" + suffix
	preview := mcp.ForgetPreviewRecord{ID: "mcp-preview-" + suffix, Principal: "mcp-principal-" + suffix, Scope: scope, MemoryIDs: []string{"memory-" + suffix}, ExpiresAt: time.Now().UTC().Add(10 * time.Minute).Truncate(time.Microsecond)}
	var fixtureRecords []mcpPostgresFixtureRecord
	var visibleID, hiddenID, foreignID string
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		for _, statement := range []struct {
			query string
			args  []any
		}{
			{`DELETE FROM mcp_forget_previews WHERE preview_id=$1`, []any{preview.ID}},
			{`DELETE FROM mcp_forget_apply_operations WHERE tenant=$1 AND project=$2 AND namespace=$3 AND principal_id=$4 AND idempotency_key=$5`, []any{scope.Tenant, scope.Project, scope.Namespace, preview.Principal, "apply-" + suffix}},
			{`DELETE FROM mcp_forget_apply_operations WHERE tenant=$1 AND project=$2 AND namespace=$3 AND principal_id=$4 AND idempotency_key=$5`, []any{foreignScope.Tenant, foreignScope.Project, foreignScope.Namespace, preview.Principal, "apply-" + suffix}},
		} {
			if _, err := pool.Exec(cleanupCtx, statement.query, statement.args...); err != nil {
				t.Errorf("clean MCP conformance operational fixture data: %v", err)
			}
		}
		if err := cleanupMCPPostgresFixtures(cleanupCtx, pool, fixtureRecords); err != nil {
			t.Errorf("clean MCP conformance governed fixture records: %v", err)
		}
	}()
	firstRepository := NewRepository(pool)
	if err := firstRepository.SaveForgetPreview(ctx, preview); err != nil {
		t.Fatalf("SaveForgetPreview() error = %v", err)
	}

	// A newly constructed repository simulates an API process restart while
	// keeping PostgreSQL as the only durable source for this review manifest.
	secondRepository := NewRepository(pool)
	loaded, err := secondRepository.LoadForgetPreview(ctx, preview.ID)
	if err != nil {
		t.Fatalf("LoadForgetPreview() after repository recreation error = %v", err)
	}
	if loaded.Principal != preview.Principal || loaded.Scope != scope || !reflect.DeepEqual(loaded.MemoryIDs, preview.MemoryIDs) || !loaded.ExpiresAt.Equal(preview.ExpiresAt) {
		t.Fatalf("durable preview = %+v, want exact principal/scope/ID manifest/expiry", loaded)
	}

	claim := mcp.ForgetApplyClaim{PrincipalID: preview.Principal, Scope: scope, IdempotencyKey: "apply-" + suffix, RequestFingerprint: "request-fingerprint", ClaimID: "claim-first"}
	claimed, err := firstRepository.ClaimForgetApply(ctx, claim)
	if err != nil || claimed.Disposition != mcp.ForgetApplyClaimed {
		t.Fatalf("ClaimForgetApply() = %+v, %v; want claimed", claimed, err)
	}
	wantOutcome := mcp.ForgetApplyResponse{PreviewID: preview.ID, AppliedIDs: append([]string(nil), preview.MemoryIDs...)}
	if err := firstRepository.CompleteForgetApply(ctx, claim, wantOutcome); err != nil {
		t.Fatalf("CompleteForgetApply() error = %v", err)
	}

	retry := claim
	retry.ClaimID = "claim-retry"
	replayed, err := secondRepository.ClaimForgetApply(ctx, retry)
	if err != nil || replayed.Disposition != mcp.ForgetApplyReplayed || !reflect.DeepEqual(replayed.Response, wantOutcome) {
		t.Fatalf("durable retry = %+v, %v; want original completed outcome", replayed, err)
	}

	conflict := retry
	conflict.RequestFingerprint = "different-payload"
	conflicted, err := secondRepository.ClaimForgetApply(ctx, conflict)
	if err != nil || conflicted.Disposition != mcp.ForgetApplyConflict {
		t.Fatalf("conflicting idempotency reuse = %+v, %v; want conflict", conflicted, err)
	}

	foreignClaim := retry
	foreignClaim.Scope = foreignScope
	foreignClaim.RequestFingerprint = claim.RequestFingerprint
	foreignClaim.ClaimID = "claim-foreign"
	foreign, err := secondRepository.ClaimForgetApply(ctx, foreignClaim)
	if err != nil || foreign.Disposition != mcp.ForgetApplyClaimed {
		t.Fatalf("foreign exact scope claim = %+v, %v; want an isolated independent claim", foreign, err)
	}
	if err := secondRepository.ReleaseForgetApply(ctx, foreignClaim); err != nil {
		t.Fatalf("ReleaseForgetApply(foreign) error = %v", err)
	}

	var storedIDs []string
	if err := pool.QueryRow(ctx, `SELECT memory_ids FROM mcp_forget_previews WHERE preview_id=$1`, preview.ID).Scan(&storedIDs); err != nil {
		t.Fatalf("read persisted review IDs: %v", err)
	}
	if !reflect.DeepEqual(storedIDs, preview.MemoryIDs) {
		t.Fatalf("persisted preview IDs = %v, want %v", storedIDs, preview.MemoryIDs)
	}

	// Seed only unique fixture scopes through ingest, candidate admission,
	// canonical promotion, and lifecycle repository boundaries. Cleanup below
	// uses the exact IDs returned by these governed operations.
	visibleFixture, err := seedMCPPostgresFixtureMemory(ctx, firstRepository, scope, "visible mcp conformance memory", memory.MemoryStateActive)
	fixtureRecords = append(fixtureRecords, visibleFixture)
	if err != nil {
		t.Fatalf("seed visible governed MCP fixture: %v", err)
	}
	visibleID = visibleFixture.MemoryID
	hiddenFixture, err := seedMCPPostgresFixtureMemory(ctx, firstRepository, scope, "hidden mcp conformance memory", memory.MemoryStateSuppressed)
	fixtureRecords = append(fixtureRecords, hiddenFixture)
	if err != nil {
		t.Fatalf("seed suppressed governed MCP fixture: %v", err)
	}
	hiddenID = hiddenFixture.MemoryID
	foreignFixture, err := seedMCPPostgresFixtureMemory(ctx, firstRepository, foreignScope, "foreign mcp conformance memory", memory.MemoryStateActive)
	fixtureRecords = append(fixtureRecords, foreignFixture)
	if err != nil {
		t.Fatalf("seed foreign governed MCP fixture: %v", err)
	}
	foreignID = foreignFixture.MemoryID
	visible, err := firstRepository.ListCanonicalMemories(ctx, scope, false)
	if err != nil {
		t.Fatalf("ListCanonicalMemories(in-scope) error = %v", err)
	}
	if len(visible) != 1 || visible[0].ID != visibleID || strings.Contains(visible[0].Content, "hidden") {
		t.Fatalf("ordinary MCP browse repository result = %+v, want one visible exact-scope memory", visible)
	}
	foreignVisible, err := firstRepository.ListCanonicalMemories(ctx, foreignScope, false)
	if err != nil || len(foreignVisible) != 1 || foreignVisible[0].ID != foreignID {
		t.Fatalf("foreign exact scope repository result = %+v, %v; want only foreign fixture memory", foreignVisible, err)
	}
	searchHits, err := firstRepository.SearchLexical(ctx, retrieval.SearchInput{Scope: scope, Query: "visible conformance", TopK: 10})
	if err != nil || len(searchHits) != 1 || searchHits[0].Memory.ID != visibleID {
		t.Fatalf("scoped lexical search = %+v, %v; want only visible in-scope hit", searchHits, err)
	}
	fixtureMemoryIDs := make([]string, 0, len(fixtureRecords))
	for _, record := range fixtureRecords {
		fixtureMemoryIDs = append(fixtureMemoryIDs, record.MemoryID)
	}
	var versionedFixtureCount, promotedFixtureCount, hiddenLifecycleProvenanceCount int
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT memory_id) FROM memory_versions WHERE memory_id::text = ANY($1::text[])`, fixtureMemoryIDs).Scan(&versionedFixtureCount); err != nil {
		t.Fatalf("count governed canonical versions: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT memory_id) FROM provenance_links WHERE memory_id::text = ANY($1::text[]) AND operation='promote_candidate'`, fixtureMemoryIDs).Scan(&promotedFixtureCount); err != nil {
		t.Fatalf("count governed promotion provenance: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM provenance_links WHERE memory_id::text=$1 AND operation='suppress_memory'`, hiddenID).Scan(&hiddenLifecycleProvenanceCount); err != nil {
		t.Fatalf("count hidden lifecycle provenance: %v", err)
	}
	if versionedFixtureCount != len(fixtureRecords) || promotedFixtureCount != len(fixtureRecords) || hiddenLifecycleProvenanceCount == 0 {
		t.Fatalf("MCP conformance fixture bypassed governed setup: versioned=%d promoted=%d fixtures=%d hidden lifecycle provenance=%d", versionedFixtureCount, promotedFixtureCount, len(fixtureRecords), hiddenLifecycleProvenanceCount)
	}

	// Exercise the actual Streamable HTTP adapter rather than only calling its
	// repository dependencies. The request crosses the API-key principal check,
	// exact scope grant check, MCP schema, retrieval service, and PostgreSQL
	// lexical query before the response-shaping contract is inspected.
	principal := auth.Principal{ID: "mcp-http-" + suffix, Role: auth.PrincipalRolePublic, Status: auth.PrincipalStatusActive}
	adapter := mcp.NewAdapter(mcp.AdapterOptions{
		Enabled:    true,
		Authorizer: mcpPostgresAuthorizer{principal: principal, credential: auth.Credential{ID: "mcp-credential-" + suffix, PrincipalID: principal.ID, Status: auth.CredentialStatusActive, CredentialID: "mcp-credential-" + suffix, Salt: []byte("salt"), Digest: []byte("digest"), CreatedAt: time.Now().UTC()}, secret: "mcp-conformance-key", scope: scope},
		Searcher:   retrieval.NewService(retrieval.ServiceDependencies{Lexical: firstRepository, Semantic: firstRepository, Relations: firstRepository, GraphTraversal: firstRepository, Citations: firstRepository}),
		Limits:     mcp.Limits{MaxQueryBytes: 128, MaxPayloadBytes: 4096, MaxResults: 10, MaxIDs: 10},
	})
	httpServer := httptest.NewServer(adapter)
	defer httpServer.Close()
	client := protocolmcp.NewClient(&protocolmcp.Implementation{Name: "mcp-postgres-conformance", Version: "v1"}, nil)
	session, err := client.Connect(ctx, &protocolmcp.StreamableClientTransport{Endpoint: httpServer.URL, DisableStandaloneSSE: true, HTTPClient: &http.Client{Transport: mcpHeaderTransport{next: http.DefaultTransport, apiKey: "mcp-conformance-key"}}}, nil)
	if err != nil {
		t.Fatalf("connect Streamable HTTP MCP client: %v", err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &protocolmcp.CallToolParams{Name: mcp.ToolSearch, Arguments: map[string]any{"tenant": scope.Tenant, "project": scope.Project, "namespace": scope.Namespace, "query": "visible conformance", "limit": 10}})
	if err != nil || result.IsError {
		t.Fatalf("MCP memory_search result=%+v error=%v", result, err)
	}
	response, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{visibleID, "visible mcp conformance memory"} {
		if !strings.Contains(string(response), required) {
			t.Errorf("MCP search omitted visible in-scope evidence %q: %s", required, response)
		}
	}
	for _, forbidden := range []string{hiddenID, foreignID, "hidden mcp conformance memory", "foreign mcp conformance memory", "score", "tenant", "project", "namespace"} {
		if strings.Contains(string(response), forbidden) {
			t.Errorf("MCP search leaked filtered/internal field %q: %s", forbidden, response)
		}
	}
}

type mcpPostgresFixtureRecord struct {
	MemoryID    string
	CandidateID string
	RawEventID  string
}

func seedMCPPostgresFixtureMemory(ctx context.Context, repository *Repository, scope memory.Scope, content string, state memory.MemoryState) (mcpPostgresFixtureRecord, error) {
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	record := mcpPostgresFixtureRecord{MemoryID: uuid.NewString(), CandidateID: uuid.NewString()}
	actor := "mcp-postgres-conformance"
	event, err := repository.IngestEvent(ctx, memory.IngestEventInput{
		Scope:           scope,
		EventType:       "mcp_conformance_fixture",
		Content:         content,
		Metadata:        map[string]any{"fixture": "mcp-postgres-conformance"},
		SourceTimestamp: createdAt,
	}, memory.ProvenanceRecord{
		ID:        uuid.NewString(),
		Scope:     scope,
		Operation: "mcp_conformance_ingest",
		Actor:     actor,
		CreatedAt: createdAt,
	})
	if err != nil {
		return record, err
	}
	record.RawEventID = event.ID
	candidate := governance.CandidateMemory{
		ID:               record.CandidateID,
		SourceRawEventID: event.ID,
		Scope:            scope,
		Class:            memory.MemoryClassEpisodic,
		Content:          content,
		Confidence:       1,
		Importance:       1,
		Freshness:        1,
		Sensitivity:      governance.SensitivityLow,
		Mutability:       governance.MutabilityImmutable,
		RetentionClass:   policy.RetentionClassPermanent,
		Status:           governance.CandidateStatusPending,
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
	}
	if _, err := repository.CreateCandidate(ctx, candidate, memory.ProvenanceRecord{
		ID:         uuid.NewString(),
		Scope:      scope,
		RawEventID: event.ID,
		Operation:  "mcp_conformance_candidate",
		Actor:      actor,
		CreatedAt:  createdAt,
	}); err != nil {
		return record, err
	}
	canonical, _, err := repository.PromoteCandidate(ctx, governance.CanonicalPromotion{
		Candidate: candidate,
		MemoryID:  record.MemoryID,
		VersionID: uuid.NewString(),
		Version:   1,
		CreatedAt: createdAt,
	})
	if err != nil {
		return record, err
	}
	if _, err := repository.TransitionCandidateStatus(ctx, governance.CandidateStatusTransition{
		CandidateID: candidate.ID,
		ToStatus:    governance.CandidateStatusPromoted,
		UpdatedAt:   createdAt,
	}, memory.ProvenanceRecord{
		ID:         uuid.NewString(),
		Scope:      scope,
		RawEventID: event.ID,
		MemoryID:   canonical.ID,
		Operation:  "mcp_conformance_candidate_promoted",
		Actor:      actor,
		CreatedAt:  createdAt,
	}); err != nil {
		return record, err
	}
	if state == memory.MemoryStateSuppressed {
		if _, err := repository.ApplyLifecycleAction(ctx, governance.LifecycleAction{
			MemoryID:  canonical.ID,
			Scope:     scope,
			Action:    policy.ForgettingActionSuppress,
			Reason:    "MCP conformance hidden-memory fixture",
			Actor:     actor,
			RequestID: uuid.NewString(),
			AppliedAt: createdAt.Add(time.Second),
		}); err != nil {
			return record, err
		}
	}
	return record, nil
}

func cleanupMCPPostgresFixtures(ctx context.Context, pool *pgxpool.Pool, records []mcpPostgresFixtureRecord) error {
	if len(records) == 0 {
		return nil
	}
	memoryIDs := make([]string, 0, len(records))
	candidateIDs := make([]string, 0, len(records))
	rawEventIDs := make([]string, 0, len(records))
	for _, record := range records {
		memoryIDs = append(memoryIDs, record.MemoryID)
		candidateIDs = append(candidateIDs, record.CandidateID)
		if record.RawEventID != "" {
			rawEventIDs = append(rawEventIDs, record.RawEventID)
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`DELETE FROM provenance_links WHERE memory_id::text = ANY($1::text[]) OR candidate_memory_id::text = ANY($2::text[]) OR raw_event_id::text = ANY($3::text[])`, []any{memoryIDs, candidateIDs, rawEventIDs}},
		{`DELETE FROM deletion_markers WHERE memory_id::text = ANY($1::text[])`, []any{memoryIDs}},
		{`DELETE FROM relation_projections WHERE memory_id::text = ANY($1::text[])`, []any{memoryIDs}},
		{`DELETE FROM memory_versions WHERE memory_id::text = ANY($1::text[])`, []any{memoryIDs}},
		{`DELETE FROM canonical_memories WHERE id::text = ANY($1::text[])`, []any{memoryIDs}},
		{`DELETE FROM candidate_memories WHERE id::text = ANY($1::text[])`, []any{candidateIDs}},
		{`DELETE FROM raw_events WHERE id::text = ANY($1::text[])`, []any{rawEventIDs}},
	} {
		if _, err := tx.Exec(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type mcpPostgresAuthorizer struct {
	principal  auth.Principal
	credential auth.Credential
	secret     string
	scope      memory.Scope
}

func (a mcpPostgresAuthorizer) Authenticate(_ context.Context, secret string) (auth.Principal, auth.Credential, error) {
	if secret != a.secret {
		return auth.Principal{}, auth.Credential{}, os.ErrPermission
	}
	return a.principal, a.credential, nil
}

func (a mcpPostgresAuthorizer) AuthorizeScope(_ context.Context, principalID string, scope memory.Scope) (bool, error) {
	return principalID == a.principal.ID && scope.Normalized() == a.scope.Normalized(), nil
}

func (a mcpPostgresAuthorizer) AuthorizeScopeAccess(_ context.Context, principalID string, scope memory.Scope) (auth.ScopeGrantAccessMode, error) {
	if principalID != a.principal.ID || scope.Normalized() != a.scope.Normalized() {
		return "", os.ErrPermission
	}
	return auth.ScopeGrantAccessReadWrite, nil
}

type mcpHeaderTransport struct {
	next   http.RoundTripper
	apiKey string
}

func (t mcpHeaderTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set(auth.HeaderAPIKey, t.apiKey)
	return t.next.RoundTrip(clone)
}
