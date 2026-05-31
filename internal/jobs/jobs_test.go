package jobs

import "testing"

func TestNoopWorkerStart(t *testing.T) {
	var worker NoopWorker
	if err := worker.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}

func TestNoopSchedulerStart(t *testing.T) {
	var scheduler NoopScheduler
	if err := scheduler.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}
