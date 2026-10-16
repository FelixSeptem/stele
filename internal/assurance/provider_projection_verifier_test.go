package assurance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The orchestration boundary is part of this verifier: a public pending state
// must be observed with consumers stopped, then ordinary consumers resume.
func TestProviderProjectionVerifierOwnsPendingWindowAndReports(t *testing.T) {
	b, err := os.ReadFile("../../scripts/stele-provider-context-verify.ps1")
	if err != nil {
		t.Fatal(err)
	}
	script := string(b)
	for _, required := range []string{"$ProjectionReportPath", "STELE_PROJECTION_VERIFY_REPORT", "STELE_PROJECTION_VERIFY_PHASE", "stop worker scheduler", "start worker scheduler", "TestProviderProjectionFreshPostgresLive", "projection_verification_incomplete", "provider-turn-outcome-feedback-live-v1", "pending_to_completed_checked", "feedback_identity_hash"} {
		if !strings.Contains(script, required) {
			t.Errorf("PC6 owned verifier boundary missing: %s", required)
		}
	}
	stop := strings.Index(script, "stop worker scheduler")
	admit := strings.Index(script, "$phase = \"projection_pending_admission\"")
	resume := strings.Index(script, "start worker scheduler")
	if stop < 0 || admit < stop || resume < admit {
		t.Error("PC6 must observe real admission while consumers are stopped, before resuming them")
	}
	for _, required := range []string{"$overridePath = $null", "$ownedOverride = $false", "$overrideID = [guid]::NewGuid().ToString('N')", "stele-context-$ProjectName-$overrideID.yml", "if ($ownedOverride -and $overridePath)", "$resolvedOverride -ne $expectedOverride"} {
		if !strings.Contains(script, required) {
			t.Errorf("owned override cleanup boundary missing: %s", required)
		}
	}
	preflight := strings.Index(script, "fresh context verification project already owns resources")
	allocate := strings.Index(script, "$overrideID = [guid]")
	write := strings.Index(script, "Set-Content -LiteralPath $overridePath")
	owned := strings.Index(script, "$ownedOverride = $true")
	if preflight < 0 || allocate < preflight || write < allocate || owned < write {
		t.Error("override ownership must follow project preflight and successful unique-file creation")
	}
}

func TestProviderProjectionVerifierInvalidProjectPreservesUnownedOverride(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("PowerShell unavailable")
	}
	dir := t.TempDir()
	// The previous predictable override path traversed through this directory
	// and deleted preserve.yml even though project validation rejected admission.
	if err := os.Mkdir(filepath.Join(dir, "stele-context-owned"), 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(dir, "preserve.yml")
	const contents = "unowned override must survive\n"
	if err := os.WriteFile(sentinel, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("../../scripts/stele-provider-context-verify.ps1")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, pwsh, "-NoProfile", "-NonInteractive", "-File", script, "-ProjectName", "owned/../preserve", "-ReportPath", filepath.Join(dir, "failure.json"))
	cmd.Env = append(os.Environ(), "TMP="+dir, "TEMP="+dir, "TMPDIR="+dir)
	if err := cmd.Run(); err == nil || ctx.Err() != nil {
		t.Fatal("invalid project must reject before Docker startup")
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != contents {
		t.Fatal("invalid project cleanup removed or changed an unowned override")
	}
}
