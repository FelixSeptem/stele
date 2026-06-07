package telemetry

import (
	"context"
	"time"
)

type OperationEvent struct {
	Mode       string
	Component  string
	Operation  string
	Status     string
	Count      int
	Duration   time.Duration
	Error      string
	ObservedAt time.Time
}

type BacklogEvent struct {
	Mode       string
	Component  string
	Queue      string
	Status     string
	Pending    int64
	Leased     int64
	Processed  int64
	OldestAge  time.Duration
	Error      string
	ObservedAt time.Time
}

type Observer interface {
	RecordOperation(ctx context.Context, event OperationEvent)
	RecordBacklog(ctx context.Context, event BacklogEvent)
}

type noopObserver struct{}

func (noopObserver) RecordOperation(ctx context.Context, event OperationEvent) {}

func (noopObserver) RecordBacklog(ctx context.Context, event BacklogEvent) {}

func NoopObserver() Observer {
	return noopObserver{}
}
