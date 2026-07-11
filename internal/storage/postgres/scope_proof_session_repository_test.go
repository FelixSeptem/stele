package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestRepositoryCreatesReadsAndClaimsScopeProofStep(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 16, 30, 0, 0, time.UTC)
	run := memory.ScopeProofRun{
		ID:          "proof_1",
		Scope:       scope,
		Status:      memory.ScopeProofStatusPending,
		Verdict:     memory.ScopeProofVerdictPending,
		Checks:      []memory.ScopeProofCheck{memory.ScopeProofCheckIngestion, memory.ScopeProofCheckContext},
		FixtureMode: memory.ScopeProofFixtureModeSmoke,
		Actor:       "operator-a",
		Reason:      "prove onboarding",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	step := memory.ScopeProofStep{
		ID:        "proof_step_1",
		ProofID:   run.ID,
		Scope:     scope,
		Step:      memory.ScopeProofStepIngestion,
		Status:    memory.ScopeProofStepStatusPending,
		Evidence:  map[string]any{"event_id": "evt_1"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	mock.ExpectQuery("INSERT INTO scope_proof_runs").
		WithArgs(run.ID, scope.Tenant, scope.Project, scope.Namespace, run.Status, run.Verdict, []string{"ingestion", "context"}, run.FixtureMode, run.Actor, run.Reason, nil, nil, nil, []byte(`{}`), now, now, nil, nil).
		WillReturnRows(scopeProofRunRows().AddRow(run.ID, scope.Tenant, scope.Project, scope.Namespace, run.Status, run.Verdict, []string{"ingestion", "context"}, run.FixtureMode, run.Actor, run.Reason, nil, nil, nil, nil, now, now, nil, nil))
	mock.ExpectQuery("INSERT INTO scope_proof_steps").
		WithArgs(step.ID, step.ProofID, scope.Tenant, scope.Project, scope.Namespace, step.Step, step.Status, nil, nil, []byte(`{"event_id":"evt_1"}`), 0, nil, nil, nil, nil, now, now, nil).
		WillReturnRows(scopeProofStepRows().AddRow(step.ID, step.ProofID, scope.Tenant, scope.Project, scope.Namespace, step.Step, step.Status, nil, nil, []byte(`{"event_id":"evt_1"}`), 0, nil, nil, nil, nil, now, now, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM scope_proof_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, run.ID).
		WillReturnRows(scopeProofRunRows().AddRow(run.ID, scope.Tenant, scope.Project, scope.Namespace, run.Status, run.Verdict, []string{"ingestion", "context"}, run.FixtureMode, run.Actor, run.Reason, nil, nil, nil, nil, now, now, nil, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM scope_proof_steps").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, run.ID).
		WillReturnRows(scopeProofStepRows().AddRow(step.ID, step.ProofID, scope.Tenant, scope.Project, scope.Namespace, step.Step, step.Status, nil, nil, []byte(`{"event_id":"evt_1"}`), 0, nil, nil, nil, nil, now, now, nil))
	mock.ExpectQuery("WITH claimed AS").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "worker-a", now, now.Add(time.Minute), 1, memory.ScopeProofStepStatusRunning, memory.ScopeProofStepStatusPending, memory.ScopeProofStepStatusFailed).
		WillReturnRows(scopeProofStepRows().AddRow(step.ID, step.ProofID, scope.Tenant, scope.Project, scope.Namespace, step.Step, memory.ScopeProofStepStatusRunning, nil, nil, []byte(`{"event_id":"evt_1"}`), 1, "worker-a", now.Add(time.Minute), nil, nil, now, now, nil))

	repo := NewRepository(mock)
	if _, err := repo.CreateScopeProofRun(context.Background(), run); err != nil {
		t.Fatalf("CreateScopeProofRun() error = %v", err)
	}
	if _, err := repo.CreateScopeProofStep(context.Background(), step); err != nil {
		t.Fatalf("CreateScopeProofStep() error = %v", err)
	}
	read, err := repo.ReadScopeProofRun(context.Background(), memory.ReadScopeProofRunInput{Scope: scope, ProofID: run.ID})
	if err != nil {
		t.Fatalf("ReadScopeProofRun() error = %v", err)
	}
	if read.Scope != scope || len(read.Steps) != 1 {
		t.Fatalf("read = %+v, want scoped run with one step", read)
	}
	claimed, err := repo.ClaimScopeProofSteps(context.Background(), memory.ClaimScopeProofStepsInput{
		Scope:         scope,
		WorkerID:      "worker-a",
		Now:           now,
		LeaseDuration: time.Minute,
		Limit:         1,
	})
	if err != nil {
		t.Fatalf("ClaimScopeProofSteps() error = %v", err)
	}
	if len(claimed) != 1 || claimed[0].Status != memory.ScopeProofStepStatusRunning || claimed[0].WorkerID != "worker-a" {
		t.Fatalf("claimed = %+v, want running proof step for worker-a", claimed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryCreatesAndReadsMemorySessionWithTurn(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 17, 0, 0, 0, time.UTC)
	session := memory.MemorySessionRun{
		ID:        "session_1",
		Scope:     scope,
		Status:    memory.MemorySessionStatusActive,
		Verdict:   memory.ScopeProofVerdictPending,
		Actor:     "agent-a",
		Reason:    "serve user turn",
		Metadata:  map[string]any{"integration": "test-agent"},
		CreatedAt: now,
		UpdatedAt: now,
	}
	turn := memory.MemorySessionTurn{
		ID:              "turn_1",
		SessionID:       session.ID,
		Scope:           scope,
		Status:          memory.MemorySessionTurnStatusContextAssembled,
		Query:           "remember deployment preference",
		ContextEvidence: map[string]any{"memory_ids": []any{"mem_1"}},
		OutcomeEventIDs: []string{"evt_1"},
		ExpectedRecall:  []string{"evt_1"},
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	mock.ExpectQuery("INSERT INTO memory_session_runs").
		WithArgs(session.ID, scope.Tenant, scope.Project, scope.Namespace, session.Status, session.Verdict, session.Actor, session.Reason, []byte(`{"integration":"test-agent"}`), nil, now, now, nil, nil).
		WillReturnRows(memorySessionRunRows().AddRow(session.ID, scope.Tenant, scope.Project, scope.Namespace, session.Status, session.Verdict, session.Actor, session.Reason, []byte(`{"integration":"test-agent"}`), nil, now, now, nil, nil))
	mock.ExpectQuery("INSERT INTO memory_session_turns").
		WithArgs(turn.ID, turn.SessionID, scope.Tenant, scope.Project, scope.Namespace, turn.Status, turn.Query, []byte(`{"memory_ids":["mem_1"]}`), []string{"evt_1"}, []string{"evt_1"}, nil, nil, now, now, nil).
		WillReturnRows(memorySessionTurnRows().AddRow(turn.ID, turn.SessionID, scope.Tenant, scope.Project, scope.Namespace, turn.Status, turn.Query, []byte(`{"memory_ids":["mem_1"]}`), []string{"evt_1"}, []string{"evt_1"}, nil, nil, now, now, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM memory_session_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, session.ID).
		WillReturnRows(memorySessionRunRows().AddRow(session.ID, scope.Tenant, scope.Project, scope.Namespace, session.Status, session.Verdict, session.Actor, session.Reason, []byte(`{"integration":"test-agent"}`), nil, now, now, nil, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM memory_session_turns").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, session.ID).
		WillReturnRows(memorySessionTurnRows().AddRow(turn.ID, turn.SessionID, scope.Tenant, scope.Project, scope.Namespace, turn.Status, turn.Query, []byte(`{"memory_ids":["mem_1"]}`), []string{"evt_1"}, []string{"evt_1"}, nil, nil, now, now, nil))

	repo := NewRepository(mock)
	if _, err := repo.CreateMemorySessionRun(context.Background(), session); err != nil {
		t.Fatalf("CreateMemorySessionRun() error = %v", err)
	}
	if _, err := repo.CreateMemorySessionTurn(context.Background(), turn); err != nil {
		t.Fatalf("CreateMemorySessionTurn() error = %v", err)
	}
	read, err := repo.ReadMemorySessionRun(context.Background(), memory.ReadMemorySessionRunInput{Scope: scope, SessionID: session.ID})
	if err != nil {
		t.Fatalf("ReadMemorySessionRun() error = %v", err)
	}
	if read.Scope != scope || len(read.Turns) != 1 || read.Turns[0].OutcomeEventIDs[0] != "evt_1" {
		t.Fatalf("read = %+v, want scoped session with turn evidence", read)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryListsScopeProofRunsAndMemorySessionsAndCreatesEvidenceLink(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 17, 30, 0, 0, time.UTC)
	link := memory.MemoryLoopEvidenceLink{
		ID:           "evidence_1",
		Scope:        scope,
		OwnerKind:    memory.MemoryLoopEvidenceOwnerProof,
		OwnerID:      "proof_1",
		EvidenceKind: memory.MemoryLoopEvidenceKindEvent,
		EvidenceID:   "evt_1",
		Metadata:     map[string]any{"fixture": "smoke"},
		CreatedAt:    now,
	}

	mock.ExpectQuery("SELECT[\\s\\S]*FROM scope_proof_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, 10).
		WillReturnRows(scopeProofRunRows().AddRow("proof_1", scope.Tenant, scope.Project, scope.Namespace, memory.ScopeProofStatusCompleted, memory.ScopeProofVerdictPassed, []string{"ingestion"}, memory.ScopeProofFixtureModeSmoke, "operator-a", "prove", nil, nil, nil, []byte(`{}`), now, now, nil, nil))
	mock.ExpectQuery("SELECT[\\s\\S]*FROM memory_session_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, 10).
		WillReturnRows(memorySessionRunRows().AddRow("session_1", scope.Tenant, scope.Project, scope.Namespace, memory.MemorySessionStatusCompleted, memory.ScopeProofVerdictPassed, "agent-a", "turn", []byte(`{}`), nil, now, now, nil, nil))
	mock.ExpectQuery("INSERT INTO memory_loop_evidence_links").
		WithArgs(link.ID, scope.Tenant, scope.Project, scope.Namespace, link.OwnerKind, link.OwnerID, link.EvidenceKind, link.EvidenceID, []byte(`{"fixture":"smoke"}`), now).
		WillReturnRows(memoryLoopEvidenceLinkRows().AddRow(link.ID, scope.Tenant, scope.Project, scope.Namespace, link.OwnerKind, link.OwnerID, link.EvidenceKind, link.EvidenceID, []byte(`{"fixture":"smoke"}`), now))

	repo := NewRepository(mock)
	proofs, err := repo.ListScopeProofRuns(context.Background(), memory.ListScopeProofRunsInput{Scope: scope, Limit: 10})
	if err != nil {
		t.Fatalf("ListScopeProofRuns() error = %v", err)
	}
	if len(proofs) != 1 || proofs[0].ID != "proof_1" {
		t.Fatalf("proofs = %+v, want proof_1", proofs)
	}
	sessions, err := repo.ListMemorySessionRuns(context.Background(), memory.ListMemorySessionRunsInput{Scope: scope, Limit: 10})
	if err != nil {
		t.Fatalf("ListMemorySessionRuns() error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "session_1" {
		t.Fatalf("sessions = %+v, want session_1", sessions)
	}
	created, err := repo.CreateMemoryLoopEvidenceLink(context.Background(), link)
	if err != nil {
		t.Fatalf("CreateMemoryLoopEvidenceLink() error = %v", err)
	}
	if created.OwnerID != "proof_1" || created.EvidenceID != "evt_1" {
		t.Fatalf("created = %+v, want proof/event link", created)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryUpdatesProofAndSessionStatusAndCreatesSessionVerification(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 18, 0, 0, 0, time.UTC)
	verification := memory.MemorySessionVerification{
		ID:             "verification_1",
		SessionID:      "session_1",
		TurnID:         "turn_1",
		Scope:          scope,
		Status:         memory.ScopeProofStepStatusPending,
		Verdict:        memory.ScopeProofVerdictPending,
		ExpectedRecall: []string{"evt_1"},
		Evidence:       map[string]any{"memory_id": "mem_1"},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	mock.ExpectQuery("UPDATE scope_proof_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "proof_1", memory.ScopeProofStatusCompleted, memory.ScopeProofVerdictPassed, nil, []byte(`{"passed_steps":3}`), now, nil, now).
		WillReturnRows(scopeProofRunRows().AddRow("proof_1", scope.Tenant, scope.Project, scope.Namespace, memory.ScopeProofStatusCompleted, memory.ScopeProofVerdictPassed, []string{"ingestion"}, memory.ScopeProofFixtureModeSmoke, "operator-a", "prove", nil, nil, nil, []byte(`{"passed_steps":3}`), now, now, nil, now))
	mock.ExpectQuery("UPDATE memory_session_runs").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "session_1", memory.MemorySessionStatusCompleted, memory.ScopeProofVerdictPassed, nil, now, nil, now).
		WillReturnRows(memorySessionRunRows().AddRow("session_1", scope.Tenant, scope.Project, scope.Namespace, memory.MemorySessionStatusCompleted, memory.ScopeProofVerdictPassed, "agent-a", "turn", []byte(`{}`), nil, now, now, nil, now))
	mock.ExpectQuery("INSERT INTO memory_session_verifications").
		WithArgs(verification.ID, verification.SessionID, verification.TurnID, scope.Tenant, scope.Project, scope.Namespace, verification.Status, verification.Verdict, []string{"evt_1"}, []byte(`{"memory_id":"mem_1"}`), nil, 0, nil, nil, nil, nil, now, now, nil).
		WillReturnRows(memorySessionVerificationRows().AddRow(verification.ID, verification.SessionID, verification.TurnID, scope.Tenant, scope.Project, scope.Namespace, verification.Status, verification.Verdict, []string{"evt_1"}, []byte(`{"memory_id":"mem_1"}`), nil, 0, nil, nil, nil, nil, now, now, nil))

	repo := NewRepository(mock)
	updatedProof, err := repo.UpdateScopeProofRunStatus(context.Background(), memory.UpdateScopeProofRunStatusInput{
		Scope:      scope,
		ProofID:    "proof_1",
		Status:     memory.ScopeProofStatusCompleted,
		Verdict:    memory.ScopeProofVerdictPassed,
		Summary:    map[string]any{"passed_steps": 3},
		UpdatedAt:  now,
		FinishedAt: now,
	})
	if err != nil {
		t.Fatalf("UpdateScopeProofRunStatus() error = %v", err)
	}
	if updatedProof.Status != memory.ScopeProofStatusCompleted || updatedProof.Verdict != memory.ScopeProofVerdictPassed {
		t.Fatalf("updatedProof = %+v, want completed passed", updatedProof)
	}
	updatedSession, err := repo.UpdateMemorySessionRunStatus(context.Background(), memory.UpdateMemorySessionRunStatusInput{
		Scope:      scope,
		SessionID:  "session_1",
		Status:     memory.MemorySessionStatusCompleted,
		Verdict:    memory.ScopeProofVerdictPassed,
		UpdatedAt:  now,
		FinishedAt: now,
	})
	if err != nil {
		t.Fatalf("UpdateMemorySessionRunStatus() error = %v", err)
	}
	if updatedSession.Status != memory.MemorySessionStatusCompleted || updatedSession.Verdict != memory.ScopeProofVerdictPassed {
		t.Fatalf("updatedSession = %+v, want completed passed", updatedSession)
	}
	createdVerification, err := repo.CreateMemorySessionVerification(context.Background(), verification)
	if err != nil {
		t.Fatalf("CreateMemorySessionVerification() error = %v", err)
	}
	if createdVerification.ID != verification.ID || createdVerification.ExpectedRecall[0] != "evt_1" {
		t.Fatalf("createdVerification = %+v, want persisted verification", createdVerification)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestRepositoryUpdatesMemorySessionTurnOutcomeWithScope(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer mock.Close()

	scope := memory.Scope{Tenant: "tenant-a", Project: "project-a", Namespace: "namespace-a"}
	now := time.Date(2026, 7, 11, 20, 30, 0, 0, time.UTC)

	mock.ExpectQuery("UPDATE memory_session_turns").
		WithArgs(scope.Tenant, scope.Project, scope.Namespace, "session_1", "turn_1", memory.MemorySessionTurnStatusOutcomeRecorded, []string{"evt_2"}, []string{"evt_2"}, string(memory.ScopeProofVerdictPending), nil, now, nil).
		WillReturnRows(memorySessionTurnRows().AddRow("turn_1", "session_1", scope.Tenant, scope.Project, scope.Namespace, memory.MemorySessionTurnStatusOutcomeRecorded, "remember deployment preference", []byte(`{"summary":"profile"}`), []string{"evt_2"}, []string{"evt_2"}, string(memory.ScopeProofVerdictPending), nil, now, now, nil))

	repo := NewRepository(mock)
	turn, err := repo.UpdateMemorySessionTurnOutcome(context.Background(), memory.UpdateMemorySessionTurnOutcomeInput{
		Scope:              scope,
		SessionID:          "session_1",
		TurnID:             "turn_1",
		Status:             memory.MemorySessionTurnStatusOutcomeRecorded,
		OutcomeEventIDs:    []string{"evt_2"},
		ExpectedRecall:     []string{"evt_2"},
		VerificationStatus: memory.ScopeProofVerdictPending,
		UpdatedAt:          now,
	})
	if err != nil {
		t.Fatalf("UpdateMemorySessionTurnOutcome() error = %v", err)
	}
	if turn.Scope != scope || turn.Status != memory.MemorySessionTurnStatusOutcomeRecorded || turn.OutcomeEventIDs[0] != "evt_2" {
		t.Fatalf("turn = %+v, want scoped outcome update", turn)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func scopeProofRunRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "tenant", "project", "namespace", "status", "verdict", "checks", "fixture_mode",
		"actor", "reason", "rerun_of", "linked_session_id", "failure_category", "summary",
		"created_at", "updated_at", "started_at", "finished_at",
	})
}

func scopeProofStepRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "proof_id", "tenant", "project", "namespace", "step", "status", "verdict", "failure_category",
		"evidence", "attempt", "worker_id", "lease_until", "last_error", "next_attempt_at",
		"created_at", "updated_at", "completed_at",
	})
}

func memorySessionRunRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "tenant", "project", "namespace", "status", "verdict", "actor", "reason", "metadata",
		"failure_category", "created_at", "updated_at", "started_at", "finished_at",
	})
}

func memorySessionTurnRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "session_id", "tenant", "project", "namespace", "status", "query", "context_evidence",
		"outcome_event_ids", "expected_recall", "verification_status", "failure_category",
		"created_at", "updated_at", "verified_at",
	})
}

func memoryLoopEvidenceLinkRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "tenant", "project", "namespace", "owner_kind", "owner_id", "evidence_kind", "evidence_id", "metadata", "created_at",
	})
}

func memorySessionVerificationRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "session_id", "turn_id", "tenant", "project", "namespace", "status", "verdict", "expected_recall", "evidence",
		"failure_category", "attempt", "worker_id", "lease_until", "last_error", "next_attempt_at",
		"created_at", "updated_at", "completed_at",
	})
}
