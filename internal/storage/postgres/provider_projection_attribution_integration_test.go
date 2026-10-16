package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/jackc/pgx/v5/pgxpool"
)

func projectionAttributionFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *Repository, b provider.RuntimeBinding) {
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	hash := strings.Repeat("a", 64)
	digest := provider.ContextDigest{ContractVersion: provider.ContextDigestVersion, CanonicalizationVersion: provider.ContextCanonicalVersion, Algorithm: "sha-256", SelectionPolicyVersion: provider.ContextSelectionVersion, SelectionPolicyFingerprint: hash, Value: hash}
	receipt, err := repo.IssueContextReceipt(ctx, b, hash, digest, provider.DefaultContextContinuityConfig(), func(provider.ContextReceipt) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	req := provider.ProjectionRequest{Metadata: provider.OperationMetadata{RequestID: "r", OperationID: "o", SchemaVersion: "v1", IdempotencyKey: "attribution"}, Input: provider.ProjectionInput{ContractVersion: provider.ProjectionContractVersion, Kind: "outcome", EffectID: "attribution", SourceRevision: 1, OccurredAt: "2026-10-16T00:00:00Z", ReportedOutcome: "succeeded", ContextDigest: hash}}
	reported, err := repo.AdmitProjection(ctx, b, req)
	if err != nil || reported.ContextDigestAttribution != "reported" {
		t.Fatalf("bare digest became a receipt: %+v %v", reported, err)
	}
	req.Input.EffectID = "verified-attribution"
	req.Input.ContextReference = receipt.Reference
	verified, err := repo.AdmitProjection(ctx, b, req)
	if err != nil || verified.ContextDigestAttribution != "verified_receipt" {
		t.Fatalf("owned receipt not verified: %+v %v", verified, err)
	}
	req.Input.EffectID = "mismatched-attribution"
	req.Input.ContextDigest = strings.Repeat("b", 64)
	if _, err = repo.AdmitProjection(ctx, b, req); !errors.Is(err, provider.ErrProjectionConflict) {
		t.Fatalf("receipt digest mismatch admitted: %v", err)
	}
	req.Input.ContextDigest = hash
	// A well-formed opaque receipt does not reveal whether it belongs to another binding.
	req.Input.ContextReference = "ctxc_" + strings.Repeat("f", 32)
	if _, err = repo.AdmitProjection(ctx, b, req); !errors.Is(err, provider.ErrProjectionUnavailable) {
		t.Fatalf("missing receipt admitted: %v", err)
	}
	exec(`UPDATE provider_context_receipts SET issued_at=clock_timestamp()-interval '2 seconds',expires_at=clock_timestamp()-interval '1 second' WHERE reference=$1`, receipt.Reference)
	req.Input.ContextReference = receipt.Reference
	if _, err = repo.AdmitProjection(ctx, b, req); !errors.Is(err, provider.ErrProjectionUnavailable) {
		t.Fatalf("expired receipt admitted: %v", err)
	}
	req.Input.ContextReference = ""
	req.Input.ContextDigest = ""
	req.Input.SessionRunID = b.SessionID
	req.Input.TurnID = "pc6_turn"
	req.Input.EffectID = "session-attribution"
	if _, err = repo.AdmitProjection(ctx, b, req); !errors.Is(err, provider.ErrProjectionUnavailable) {
		t.Fatalf("missing session/turn admitted: %v", err)
	}
	exec(`INSERT INTO memory_session_runs(id,tenant,project,namespace,status,verdict) VALUES($1,'t','p','n','running','unknown')`, b.SessionID)
	exec(`INSERT INTO memory_session_turns(id,session_id,tenant,project,namespace,status,query) VALUES('pc6_turn',$1,'t','p','n','recorded','fixture')`, b.SessionID)
	if _, err = repo.AdmitProjection(ctx, b, req); err != nil {
		t.Fatal("owned session/turn denied", err)
	}
	req.Input.SourceRevision = 2
	req.Input.ReportedOutcome = "failed"
	req.Metadata.IdempotencyKey = "session-correction"
	if _, err = repo.AdmitProjection(ctx, b, req); err != nil {
		t.Fatal("session correction denied", err)
	}
	var status string
	var events int
	if err = pool.QueryRow(ctx, `SELECT status,cardinality(outcome_event_ids) FROM memory_session_turns WHERE id='pc6_turn'`).Scan(&status, &events); err != nil || status != "recorded" || events != 0 {
		t.Fatalf("projection mutated legacy turn outcome: %s %d %v", status, events, err)
	}
	for _, mutate := range []func(*provider.ProjectionInput){
		func(i *provider.ProjectionInput) { i.SessionRunID = "foreign-session" },
		func(i *provider.ProjectionInput) { i.TurnID = "foreign-turn" },
	} {
		bad := req
		bad.Input.EffectID = "foreign-session-attribution"
		mutate(&bad.Input)
		if _, err = repo.AdmitProjection(ctx, b, bad); !errors.Is(err, provider.ErrProjectionUnavailable) {
			t.Fatalf("foreign session/turn admitted: %v", err)
		}
	}
	var orphaned int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM provider_projection_identities i WHERE NOT EXISTS(SELECT 1 FROM provider_projection_revisions r WHERE r.logical_id=i.logical_id)`).Scan(&orphaned); err != nil || orphaned != 0 {
		t.Fatal("failed attribution retained an identity", orphaned, err)
	}
}
