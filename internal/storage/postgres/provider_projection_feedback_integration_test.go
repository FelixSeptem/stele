package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func projectionFeedbackScopeFairnessFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *Repository, binding provider.RuntimeBinding) {
	// Three scopes exceed a one-scope dispatch batch. They deliberately have no
	// canonical memory: durable pending projections also need policy checks.
	want := map[memory.Scope]bool{}
	for _, namespace := range []string{"fair-a", "fair-b", "fair-c"} {
		b := binding
		b.Scope.Namespace = namespace
		b.BindingID += "_" + namespace
		if _, err := pool.Exec(ctx, `INSERT INTO access_scope_grants(id,principal_id,tenant,project,namespace,status,allowed_disclosure_profiles,default_disclosure_profile) VALUES($1,$2,$3,$4,$5,'active',ARRAY['memory-reference-v1'],'memory-reference-v1')`, "grant-"+namespace, b.PrincipalID, b.Scope.Tenant, b.Scope.Project, b.Scope.Namespace); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO provider_runtime_bindings(binding_id,principal_id,tenant,project,namespace,agent_id,session_id,conversation_id,provider_instance_id,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, append(receiptIdentityArgs(b), b.CreatedAt, b.ExpiresAt)...); err != nil {
			t.Fatal(err)
		}
		request := provider.ProjectionRequest{Metadata: provider.OperationMetadata{RequestID: "fair-r", OperationID: "fair-o", SchemaVersion: "v1", IdempotencyKey: "fair-key"}, Input: provider.ProjectionInput{ContractVersion: provider.ProjectionContractVersion, Kind: "outcome", EffectID: "fair-effect", SourceRevision: 1, OccurredAt: "2026-10-16T00:00:00Z", ReportedOutcome: "unknown"}}
		if _, err := repo.AdmitProjection(ctx, b, request); err != nil {
			t.Fatal(err)
		}
		if namespace == "fair-a" {
			// More identities than one reconciliation batch leave a backlog in
			// the first scope. It must yield to scopes never checked before.
			for n := range 100 {
				request.Input.EffectID = fmt.Sprintf("backlog-%d", n)
				if _, err := repo.AdmitProjection(ctx, b, request); err != nil {
					t.Fatal(err)
				}
			}
		}
		want[b.Scope] = false
	}
	seen := map[memory.Scope]bool{}
	for range len(want) + 1 { // includes the existing transaction fixture's scope
		// A fresh repository each round proves progress survives process restarts.
		restarted := NewRepository(pool)
		source, ok := any(restarted).(interface {
			ListProjectionFeedbackScopes(context.Context, int) ([]memory.Scope, error)
		})
		list := restarted.ListMaintenanceScopes
		if ok {
			list = source.ListProjectionFeedbackScopes
		}
		scopes, err := list(ctx, 1)
		if err != nil || len(scopes) > 1 {
			t.Fatalf("bounded scope discovery: %+v %v", scopes, err)
		}
		for _, scope := range scopes {
			if seen[scope] {
				t.Errorf("scope %s repeated before other scopes received a first check", scope.Namespace)
			}
			seen[scope] = true
			if _, exists := want[scope]; exists {
				want[scope] = true
			}
			if _, err := restarted.ReconcileProjectionFeedback(ctx, scope, 100); err != nil {
				t.Fatal(err)
			}
		}
	}
	for scope, visited := range want {
		if !visited {
			t.Errorf("scope %s never scheduled across batches/restarts", scope.Namespace)
		}
	}
}

func projectionFeedbackFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *Repository, b provider.RuntimeBinding) {
	reconciler, ok := any(repo).(interface {
		ReconcileProjectionFeedback(context.Context, memory.Scope, int) (int, error)
	})
	if !ok {
		t.Fatal("missing asynchronous projection feedback reconciler")
	}
	event, candidate, canonical, version, provenance := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	scalar := func(q string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	const sm = `'{"version":"source-metadata-v1","trust":"untrusted","classification":"low"}'::jsonb`
	exec(`INSERT INTO raw_events(id,tenant,project,namespace,event_type,content,memory_path,metadata,source_metadata) VALUES($1,'t','p','n','provider.reference.task','fixture','tasks/pc6',jsonb_build_object('reference_contract_version','durable-reference-v1','reference_id','pc6-task','reference_binding_id','pc6_binding','reference_principal_id','pc6_principal'),`+sm+`)`, event)
	req := provider.ProjectionRequest{Metadata: provider.OperationMetadata{RequestID: "r", OperationID: "o", SchemaVersion: "v1", IdempotencyKey: "feedback1"}, Input: provider.ProjectionInput{ContractVersion: provider.ProjectionContractVersion, Kind: "feedback", EffectID: "feedback-effect", SourceRevision: 1, OccurredAt: "2026-10-16T00:00:00Z", ReportedOutcome: "succeeded", References: []provider.ProjectionReferenceSelector{{EventID: event}}, Feedback: &provider.ProjectionFeedback{Signal: "useful", ReasonCode: "helped"}}}
	first, err := repo.AdmitProjection(ctx, b, req)
	if err != nil || first.FeedbackState != "pending" || first.CurrentReferences[0].State != "pending" {
		t.Fatalf("pending admission: %+v %v", first, err)
	}
	reconcile := func() {
		t.Helper()
		if _, err := reconciler.ReconcileProjectionFeedback(ctx, b.Scope, 100); err != nil {
			t.Fatal(err)
		}
	}
	reconcile()
	if n := scalar(`SELECT count(*) FROM usefulness_feedback`); n != 0 {
		t.Fatal("pending feedback materialized", n)
	}
	exec(`INSERT INTO candidate_memories(id,source_raw_event_id,tenant,project,namespace,class,content,confidence,importance,freshness,sensitivity,mutability,retention_class,status,memory_path,source_metadata) VALUES($1,$2,'t','p','n','episodic','fixture',0.8,0.5,1,'low','mutable','durable','promoted','tasks/pc6',`+sm+`)`, candidate, event)
	exec(`INSERT INTO canonical_memories(id,tenant,project,namespace,class,state,content,memory_path,source_metadata) VALUES($1,'t','p','n','episodic','active','fixture','tasks/pc6',`+sm+`)`, canonical)
	exec(`INSERT INTO memory_versions(id,memory_id,version,state,content,modified_by,memory_path,source_metadata) VALUES($1,$2,1,'active','fixture','fixture','tasks/pc6',`+sm+`)`, version, canonical)
	exec(`INSERT INTO provenance_links(id,raw_event_id,candidate_memory_id,memory_id,tenant,project,namespace,operation) VALUES($1,$2,$3,$4,'t','p','n','promote_candidate')`, provenance, event, candidate, canonical)
	exec(`UPDATE raw_events SET governance_processed_at=clock_timestamp() WHERE id=$1`, event)
	reconcile()
	reconcile()
	if n := scalar(`SELECT count(*) FROM usefulness_feedback`); n != 1 {
		t.Fatal("completed retry duplicate/missing feedback", n)
	}
	subject := memory.UsefulnessFeedbackSubject{Kind: memory.UsefulnessFeedbackSubjectMemory, ID: canonical}
	summary := func(want int) {
		t.Helper()
		s, e := repo.SummarizeUsefulnessFeedback(ctx, memory.SummarizeUsefulnessFeedbackInput{Scope: b.Scope, Subject: subject})
		if e != nil || s.TotalActive != want {
			t.Fatalf("active feedback=%+v want %d err %v", s, want, e)
		}
	}
	summary(1)
	// Mutate the same durable lineage to verify that read-time eligibility keeps
	// PC5's record validation gates even before reconciliation runs. A new version
	// would already be excluded by the stored lineage identity comparisons.
	t.Run("live_reference_validation", func(t *testing.T) {
		type validationCase struct {
			name, value, path, referenceID string
			valid                          bool
		}
		cases := []validationCase{
			{name: "content_at_byte_limit", value: strings.Repeat("x", 4096), valid: true},
			{name: "content_over_byte_limit", value: strings.Repeat("x", 4097)},
			{name: "unicode_content_at_byte_limit", value: strings.Repeat("忆", 1365) + "x", valid: true},
			{name: "unicode_content_over_byte_limit", value: strings.Repeat("忆", 1366)},
			{name: "reference_id_empty", referenceID: ""},
			{name: "reference_id_over_byte_limit", referenceID: strings.Repeat("x", 129)},
			{name: "reference_id_invalid_token", referenceID: "task/id"},
			{name: "reference_id_unicode", referenceID: "任务"},
			{name: "reference_id_at_byte_limit", referenceID: strings.Repeat("x", 128), valid: true},
			{name: "reference_id_valid_punctuation", referenceID: "Task_1.2:3-4", valid: true},
			{name: "path_root", path: "/"},
			{name: "path_leading_slash", path: "/tasks/pc6"},
			{name: "path_trailing_slash", path: "tasks/pc6/"},
			{name: "path_empty_segment", path: "tasks//pc6"},
			{name: "path_dot_segment", path: "tasks/./pc6"},
			{name: "path_parent_segment", path: "tasks/../pc6"},
			{name: "path_backslash", path: `tasks\pc6`},
			{name: "path_star", path: "tasks/*"},
			{name: "path_question", path: "tasks/?"},
			{name: "path_open_bracket", path: "tasks/[pc6"},
			{name: "path_close_bracket", path: "tasks/pc6]"},
			{name: "path_encoded_slash", path: "tasks/pc%2F6"},
			{name: "path_encoded_backslash", path: "tasks/pc%5c6"},
			{name: "path_over_segment_byte_limit", path: "tasks/" + strings.Repeat("忆", 43)},
			{name: "path_at_segment_byte_limit", path: "tasks/" + strings.Repeat("忆", 42) + "ab", valid: true},
			{name: "path_at_total_byte_limit", path: strings.Repeat("x", 128) + "/" + strings.Repeat("y", 128) + "/" + strings.Repeat("z", 128) + "/" + strings.Repeat("w", 125), valid: true},
			{name: "path_over_total_byte_limit", path: strings.Repeat("x", 128) + "/" + strings.Repeat("y", 128) + "/" + strings.Repeat("z", 128) + "/" + strings.Repeat("w", 126)},
			{name: "path_valid_unicode", path: "任务/记忆🙂", valid: true},
			{name: "path_internal_spaces", path: "tasks/ child \u2003/end", valid: true},
		}
		for _, r := range []rune{'\x01', '\x1f', '\x7f', '\u0085', '\u009f'} {
			cases = append(cases, validationCase{name: fmt.Sprintf("path_control_%04x", r), path: "tasks/pc" + string(r) + "6"})
		}
		// Go's strings.TrimSpace uses the Unicode White_Space property. Include
		// every non-control member so SQL cannot accidentally use ASCII-only trim.
		for _, r := range []rune{' ', '\u00a0', '\u1680', '\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006', '\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000'} {
			cases = append(cases,
				validationCase{name: fmt.Sprintf("path_leading_space_%04x", r), path: string(r) + "tasks/pc6"},
				validationCase{name: fmt.Sprintf("path_trailing_space_%04x", r), path: "tasks/pc6" + string(r)})
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				value, path, referenceID := tc.value, tc.path, tc.referenceID
				if value == "" {
					value = "fixture"
				}
				if path == "" {
					path = "tasks/pc6"
				}
				if !strings.HasPrefix(tc.name, "reference_id_") {
					referenceID = "pc6-task"
				}
				set := func(value, path, referenceID string) {
					exec(`UPDATE raw_events SET memory_path=$2, metadata=jsonb_set(metadata,'{reference_id}',to_jsonb($3::text)) WHERE id=$1`, event, path, referenceID)
					exec(`UPDATE candidate_memories SET memory_path=$2 WHERE id=$1`, candidate, path)
					exec(`UPDATE canonical_memories SET memory_path=$2 WHERE id=$1`, canonical, path)
					exec(`UPDATE memory_versions SET memory_path=$2, content=$3 WHERE id=$1`, version, path, value)
				}
				defer set("fixture", "tasks/pc6", "pc6-task")
				set(value, path, referenceID)
				ref, err := repo.ReferenceStatus(ctx, b.Scope, b.PrincipalID, event, "memory-reference-v1", "reference_only")
				wantStatus, wantTotal := provider.ReferenceStatusUnavailable, 0
				if tc.valid {
					wantStatus, wantTotal = provider.ReferenceStatusCompleted, 1
				}
				if err != nil || ref.Status != wantStatus {
					t.Errorf("PC5 status=%s want %s err=%v", ref.Status, wantStatus, err)
				}
				s, err := repo.SummarizeUsefulnessFeedback(ctx, memory.SummarizeUsefulnessFeedbackInput{Scope: b.Scope, Subject: subject})
				if err != nil || s.TotalActive != wantTotal {
					t.Errorf("live feedback total=%d want %d err=%v", s.TotalActive, wantTotal, err)
				}
			})
		}
	})
	summary(1)
	// Admission is not a ranking or insight activation operation.
	if scalar(`SELECT count(*) FROM ranking_rollout_policies`)+scalar(`SELECT count(*) FROM canonical_memories WHERE memory_path LIKE 'insights/%'`) != 0 {
		t.Fatal("projection activated ranking or insight")
	}
	// Read-time gates close the window before the next asynchronous scan.
	exec(`UPDATE canonical_memories SET state='suppressed' WHERE id=$1`, canonical)
	summary(0)
	reconcile()
	if scalar(`SELECT count(*) FROM usefulness_feedback_supersessions`) != 1 {
		t.Fatal("missing lifecycle supersession audit")
	}
	exec(`UPDATE canonical_memories SET state='active' WHERE id=$1`, canonical)
	reconcile()
	summary(0)
	if scalar(`SELECT count(*) FROM usefulness_feedback`) != 1 {
		t.Fatal("old revision reactivated after lifecycle restoration")
	}
	req.Input.SourceRevision = 2
	req.Metadata.IdempotencyKey = "feedback2"
	req.Input.Feedback.Signal = "irrelevant"
	second, err := repo.AdmitProjection(ctx, b, req)
	if err != nil || second.SupersedesID != first.ProjectionID {
		t.Fatal("feedback correction failed", err)
	}
	reconcile()
	summary(1)
	req.Input.SourceRevision = 3
	req.Metadata.IdempotencyKey = "feedback3"
	req.Input.Feedback.Signal = "needs_review"
	if _, err = repo.AdmitProjection(ctx, b, req); err != nil {
		t.Fatal(err)
	}
	summary(0)
	reconcile()
	summary(1)
	if scalar(`SELECT count(*) FROM usefulness_feedback`) != 3 || scalar(`SELECT count(*) FROM usefulness_feedback_supersessions`) != 2 {
		t.Fatal("correction feedback lineage lost")
	}
	checkLists := func(active int) {
		t.Helper()
		for _, filter := range []memory.UsefulnessFeedbackSubject{{}, subject} {
			for _, audit := range []bool{false, true} {
				want := active
				if audit {
					want = 3
				}
				records, err := repo.ListUsefulnessFeedback(ctx, memory.ListUsefulnessFeedbackInput{Scope: b.Scope, Subject: filter, IncludeSuperseded: audit})
				if err != nil || len(records) != want {
					t.Errorf("subject=%q include_superseded=%t records=%d want=%d err=%v", filter.Kind, audit, len(records), want, err)
				}
			}
		}
	}
	checkLists(1)
	exec(`UPDATE access_scope_grants SET status='revoked' WHERE id='pc6_grant'`)
	summary(0)
	checkLists(0)
	reconcile()
	exec(`UPDATE access_scope_grants SET status='active' WHERE id='pc6_grant'`)
	reconcile()
	summary(0)
	if scalar(`SELECT count(*) FROM usefulness_feedback`) != 3 {
		t.Fatal("revoked revision reactivated")
	}
	checkLists(0)
}
