package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvContextCalibrationDefaultsAreDisabledAndBounded(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContextCalibration.Enabled {
		t.Fatal("context calibration is enabled by default")
	}
	if cfg.ContextCalibration.MaxSummaryAge <= 0 || cfg.ContextCalibration.MinimumEvidence <= 0 || cfg.ContextCalibration.ContributionCap <= 0 || cfg.ContextCalibration.MaxCandidates <= 0 || cfg.ContextCalibration.MaxElapsed <= 0 {
		t.Fatalf("context calibration defaults are not bounded: %+v", cfg.ContextCalibration)
	}
}

func TestLoadFromEnvRejectsUnsafeContextCalibrationBounds(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"STELE_CONTEXT_CALIBRATION_MINIMUM_EVIDENCE", "0"},
		{"STELE_CONTEXT_CALIBRATION_CONTRIBUTION_CAP", "1.1"},
		{"STELE_CONTEXT_CALIBRATION_MAX_CANDIDATES", "0"},
		{"STELE_CONTEXT_CALIBRATION_MAX_ELAPSED", "0s"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			t.Setenv("STELE_MODE", "api")
			t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
			t.Setenv(tc.key, tc.value)
			if _, err := LoadFromEnv(); err == nil {
				t.Fatalf("LoadFromEnv() error = nil for unsafe %s", tc.key)
			}
		})
	}
}

func TestLoadFromEnvParsesContextCalibrationBounds(t *testing.T) {
	t.Setenv("STELE_MODE", "api")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_CONTEXT_CALIBRATION_ENABLED", "true")
	t.Setenv("STELE_CONTEXT_CALIBRATION_MAX_SUMMARY_AGE", "2h")
	t.Setenv("STELE_CONTEXT_CALIBRATION_MINIMUM_EVIDENCE", "7")
	t.Setenv("STELE_CONTEXT_CALIBRATION_CONTRIBUTION_CAP", "0.25")
	t.Setenv("STELE_CONTEXT_CALIBRATION_MAX_ELAPSED", "150ms")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ContextCalibration.Enabled || cfg.ContextCalibration.MaxSummaryAge != 2*time.Hour || cfg.ContextCalibration.MinimumEvidence != 7 || cfg.ContextCalibration.ContributionCap != .25 || cfg.ContextCalibration.MaxElapsed != 150*time.Millisecond {
		t.Fatalf("context calibration config = %+v", cfg.ContextCalibration)
	}
}
