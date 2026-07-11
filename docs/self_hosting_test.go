package docs_test

import (
	"os"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/openapi"
)

func TestSelfHostingSmokeLoopDocumentsReplayContextAndMetrics(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()

	for _, want := range []string{
		"Smoke fixture scope",
		"tenant-a",
		"project-a",
		"namespace-a",
		"smoke.provider_failure",
		"smoke.operator_recovery",
		"/v1/admin/derived-insight-replays:dry-run",
		"/v1/admin/derived-insight-replays",
		"/v1/admin/derived-insight-replays/<replay-run-id>/report",
		`"include_diagnostics":true`,
		"stele_derived_insight_replay_total",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting smoke docs missing %q", want)
		}
	}

	for _, route := range []string{
		"/v1/context/assemble",
		"/v1/admin/derived-insight-replays:dry-run",
		"/v1/admin/derived-insight-replays",
		"/v1/admin/derived-insight-replays/{replay_run_id}/report",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented route %q", route)
		}
	}

	for _, requiredConfig := range []string{
		"STELE_AUTH_API_KEYS",
		"STELE_AUTH_ADMIN_API_KEYS",
		"STELE_AUTH_DEFAULT_TENANT",
		"STELE_AUTH_DEFAULT_PROJECT",
		"STELE_AUTH_DEFAULT_NAMESPACE",
	} {
		if !strings.Contains(content, requiredConfig) {
			t.Fatalf("self-hosting smoke docs missing required config %q", requiredConfig)
		}
	}
}
