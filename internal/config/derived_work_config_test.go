package config

import (
	"testing"
	"time"

	"github.com/FelixSeptem/stele/internal/workqueue"
)

func TestLoadFromEnvDefaultsDerivedWorkQueueToDurable(t *testing.T) {
	t.Setenv("STELE_MODE", "worker")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DerivedWork.Mode != workqueue.QueueModePostgresDurable || cfg.DerivedWork.Capacity <= 0 || cfg.DerivedWork.BatchSize <= 0 {
		t.Fatalf("derived work defaults = %+v", cfg.DerivedWork)
	}
}

func TestLoadFromEnvParsesDerivedWorkQueueSettings(t *testing.T) {
	t.Setenv("STELE_MODE", "worker")
	t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
	t.Setenv("STELE_DERIVED_WORK_QUEUE_MODE", "memory_buffer")
	t.Setenv("STELE_DERIVED_WORK_QUEUE_CAPACITY", "42")
	t.Setenv("STELE_DERIVED_WORK_QUEUE_BATCH_SIZE", "7")
	t.Setenv("STELE_DERIVED_WORK_QUEUE_FLUSH_INTERVAL", "250ms")
	t.Setenv("STELE_DERIVED_WORK_QUEUE_LEASE_DURATION", "2m")
	t.Setenv("STELE_DERIVED_WORK_QUEUE_MAX_ATTEMPTS", "4")
	t.Setenv("STELE_DERIVED_WORK_QUEUE_RETENTION", "48h")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	want := workqueue.QueueConfig{Mode: workqueue.QueueModeMemoryBuffer, Capacity: 42, BatchSize: 7, FlushInterval: 250 * time.Millisecond, LeaseDuration: 2 * time.Minute, MaxAttempts: 4, Retention: 48 * time.Hour}
	if cfg.DerivedWork != want {
		t.Fatalf("derived work config = %+v, want %+v", cfg.DerivedWork, want)
	}
}

func TestLoadFromEnvRejectsUnsafeDerivedWorkQueueSettings(t *testing.T) {
	for _, tc := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "mode", key: "STELE_DERIVED_WORK_QUEUE_MODE", value: "redis"},
		{name: "capacity", key: "STELE_DERIVED_WORK_QUEUE_CAPACITY", value: "0"},
		{name: "batch", key: "STELE_DERIVED_WORK_QUEUE_BATCH_SIZE", value: "999999"},
		{name: "flush", key: "STELE_DERIVED_WORK_QUEUE_FLUSH_INTERVAL", value: "0s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("STELE_MODE", "worker")
			t.Setenv("STELE_POSTGRES_DSN", "postgres://example")
			t.Setenv(tc.key, tc.value)
			if _, err := LoadFromEnv(); err == nil {
				t.Fatalf("LoadFromEnv() error = nil for %s=%s", tc.key, tc.value)
			}
		})
	}
}
