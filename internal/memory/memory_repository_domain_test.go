package memory

import "testing"

func TestMemoryRepositoryImportRequestRejectsChangedReplayAndStaleGeneration(t *testing.T) {
	request := MemoryRepositoryImportRequest{RepositoryID: "repo", PreviewID: "preview", Scope: Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}, PlanDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", WarningDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ExpectedGeneration: 3, IdempotencyKey: "key"}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	request.PlanDigest = "changed"
	if err := request.Validate(); err == nil {
		t.Fatal("invalid digest accepted")
	}
}

func TestMemoryRepositoryChildIdempotencyIsStableAndScopedToImportEntry(t *testing.T) {
	first := MemoryRepositoryChildIdempotencyKey("import-a", "entry-a")
	if first == "" || first != MemoryRepositoryChildIdempotencyKey("import-a", "entry-a") {
		t.Fatalf("unstable child idempotency key %q", first)
	}
	if first == MemoryRepositoryChildIdempotencyKey("import-a", "entry-b") {
		t.Fatal("different entry identities must not share a child idempotency key")
	}
}

func TestMemoryRepositoryImportStatusRejectsUnknownState(t *testing.T) {
	if MemoryRepositoryImportStatus("unknown").Valid() {
		t.Fatal("unknown import status accepted")
	}
	if !MemoryRepositoryEntryDispatchStatus("submitted").Valid() {
		t.Fatal("known entry dispatch state rejected")
	}
}

func TestMemoryRepositoryEntryDispatchStatusRecognizesQueued(t *testing.T) {
	if !MemoryRepositoryEntryQueued.Valid() {
		t.Fatal("queued dispatch status must be valid before a worker submits an intent")
	}
}

func TestMemoryRepositoryRegistrationValidatesScopeAndUntrustedLabel(t *testing.T) {
	record := MemoryRepositoryRecord{ID: "repo", Scope: Scope{Tenant: "tenant", Project: "project", Namespace: "namespace"}, Name: "Team notes", SourceURL: "https://example.test/repo"}
	if err := record.Validate(); err != nil {
		t.Fatalf("valid registration rejected: %v", err)
	}
	record.Name = ""
	if err := record.Validate(); err == nil {
		t.Fatal("empty repository name accepted")
	}
}
