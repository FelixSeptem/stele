package postgres

import (
	"strings"
	"testing"
)

func TestPC4ReceiptMigrationMetadataOnly(t *testing.T) {
	b, err := migrationFS.ReadFile("migrations/0036_provider_context_continuity.up.sql")
	if err != nil {
		t.Fatalf("receipt migration absent: %v", err)
	}
	s := string(b)
	for _, required := range []string{"principal_id text", "request_fingerprint", "digest_value", "expires_at", "provider_context_receipts"} {
		if !strings.Contains(s, required) {
			t.Fatalf("missing %s", required)
		}
	}
	for _, bad := range []string{"content ", "query ", "citations ", "principal_id uuid"} {
		if strings.Contains(s, bad) {
			t.Fatalf("receipt retained %s", bad)
		}
	}
}
