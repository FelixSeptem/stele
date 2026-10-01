package postgres

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mcpConformanceEvidence struct {
	SchemaVersion int                      `json:"schema_version"`
	Status        string                   `json:"status"`
	Categories    []mcpConformanceCategory `json:"categories"`
}

type mcpConformanceCategory struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Checks int    `json:"checks"`
}

type mcpConformanceRecorder struct {
	categories map[string]mcpConformanceCategory
}

func newMCPConformanceRecorder() *mcpConformanceRecorder {
	recorder := &mcpConformanceRecorder{categories: make(map[string]mcpConformanceCategory, len(mcpConformanceCategoryNames))}
	for name := range mcpConformanceCategoryNames {
		recorder.categories[name] = mcpConformanceCategory{Name: name, Status: "skipped"}
	}
	return recorder
}

func (r *mcpConformanceRecorder) mark(name, status string, checks int) {
	if r == nil {
		return
	}
	if _, ok := r.categories[name]; !ok {
		return
	}
	r.categories[name] = mcpConformanceCategory{Name: name, Status: status, Checks: checks}
}

func (r *mcpConformanceRecorder) report(status string) mcpConformanceEvidence {
	categories := make([]mcpConformanceCategory, 0, len(r.categories))
	for _, name := range []string{
		"capability_discovery", "identity", "scope_isolation", "scope_precedence", "read_only_grant", "retrieval_surface", "lifecycle_filtering", "temporal_filtering", "path_filtering", "remember_idempotency", "forget_preview", "forget_apply", "forget_replay", "response_redaction", "adapter_disabled", "openapi_non_regression",
	} {
		categories = append(categories, r.categories[name])
	}
	return mcpConformanceEvidence{SchemaVersion: 1, Status: status, Categories: categories}
}

func (r *mcpConformanceRecorder) writeFromEnvironment(t *testing.T) {
	t.Helper()
	path := strings.TrimSpace(os.Getenv("STELE_MCP_CONFORMANCE_EVIDENCE"))
	if path == "" {
		return
	}
	status := "passed"
	if t.Skipped() {
		status = "skipped"
	} else if t.Failed() {
		status = "failed"
	}
	if err := writeMCPConformanceEvidence(path, r.report(status)); err != nil {
		t.Errorf("write MCP conformance evidence: %v", err)
	}
}

var mcpConformanceCategoryNames = map[string]struct{}{
	"capability_discovery": {}, "identity": {}, "scope_isolation": {},
	"scope_precedence": {}, "read_only_grant": {}, "retrieval_surface": {},
	"lifecycle_filtering": {}, "temporal_filtering": {}, "path_filtering": {},
	"remember_idempotency": {}, "forget_preview": {}, "forget_apply": {},
	"forget_replay": {}, "response_redaction": {}, "adapter_disabled": {},
	"openapi_non_regression": {},
}

func writeMCPConformanceEvidence(path string, report mcpConformanceEvidence) error {
	if report.SchemaVersion != 1 || !validMCPConformanceStatus(report.Status) {
		return fmt.Errorf("invalid conformance evidence header")
	}
	if len(report.Categories) == 0 {
		return fmt.Errorf("conformance evidence has no categories")
	}
	for _, category := range report.Categories {
		if _, ok := mcpConformanceCategoryNames[category.Name]; !ok || !validMCPConformanceStatus(category.Status) || category.Checks < 0 {
			return fmt.Errorf("invalid conformance evidence category")
		}
	}
	seen := make(map[string]struct{}, len(report.Categories))
	for _, category := range report.Categories {
		if _, ok := seen[category.Name]; ok {
			return fmt.Errorf("duplicate conformance evidence category")
		}
		seen[category.Name] = struct{}{}
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode conformance evidence: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write conformance evidence: %w", err)
	}
	return nil
}

func validMCPConformanceStatus(status string) bool {
	switch status {
	case "passed", "failed", "skipped":
		return true
	default:
		return false
	}
}

func TestMCPConformanceEvidenceOnlyContainsAllowlistedCategories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evidence.json")
	report := mcpConformanceEvidence{
		SchemaVersion: 1,
		Status:        "passed",
		Categories: []mcpConformanceCategory{
			{Name: "scope_isolation", Status: "passed", Checks: 3},
		},
	}
	if err := writeMCPConformanceEvidence(path, report); err != nil {
		t.Fatalf("writeMCPConformanceEvidence() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded mcpConformanceEvidence
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("evidence is not valid JSON: %v", err)
	}
	if decoded.Status != "passed" || len(decoded.Categories) != 1 || decoded.Categories[0].Name != "scope_isolation" {
		t.Fatalf("decoded evidence = %+v, want bounded scope category", decoded)
	}
	for _, forbidden := range []string{"tenant-secret", "postgres://", "private query", "fixture content", "raw_payload", "api-key"} {
		if strings.Contains(strings.ToLower(string(data)), forbidden) {
			t.Errorf("evidence leaked forbidden value %q: %s", forbidden, data)
		}
	}
}

func TestMCPConformanceEvidenceRejectsUnknownCategoryAndStatus(t *testing.T) {
	for _, report := range []mcpConformanceEvidence{
		{SchemaVersion: 1, Status: "passed", Categories: []mcpConformanceCategory{{Name: "tenant-secret", Status: "passed", Checks: 1}}},
		{SchemaVersion: 1, Status: "raw payload", Categories: []mcpConformanceCategory{{Name: "scope_isolation", Status: "passed", Checks: 1}}},
	} {
		if err := writeMCPConformanceEvidence(filepath.Join(t.TempDir(), "evidence.json"), report); err == nil {
			t.Errorf("writeMCPConformanceEvidence(%+v) error = nil, want rejection", report)
		}
	}
}
