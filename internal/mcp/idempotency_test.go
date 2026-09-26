package mcp

import "testing"

func TestFingerprintIsStableForEquivalentPayloads(t *testing.T) {
	first := fingerprint(map[string]string{"scope": "s", "content": "x"})
	second := fingerprint(map[string]string{"content": "x", "scope": "s"})
	if first != second {
		t.Fatalf("equivalent payload fingerprints = %q and %q, want stable", first, second)
	}
	if first == fingerprint(map[string]string{"scope": "s", "content": "changed"}) {
		t.Fatal("conflicting payload fingerprint unexpectedly matched")
	}
}
