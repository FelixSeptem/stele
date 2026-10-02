package workqueue

import (
	"context"
	"fmt"
)

type QueueAdapter interface {
	Enqueue(context.Context, EnqueueInput) (BufferStatus, error)
	Flush(context.Context) (FlushReport, error)
	Stats() BufferStats
	DrainReport() DrainReport
	Close()
}

// BackgroundFlusher is implemented by adapters that need a process-owned
// flush loop. It is optional so callers can continue to use explicit Flush in
// tests and one-shot jobs.
type BackgroundFlusher interface {
	Start(context.Context)
}

type DrainReport struct {
	Mode                QueueMode
	Buffered            int
	Durable             uint64
	DroppedOverflow     uint64
	DroppedFlushFailure uint64
	DroppedProcessLoss  uint64
}

type durableAdapter struct {
	sink  DurableSink
	stats BufferStats
}

func NewAdapter(config QueueConfig, sink DurableSink) (QueueAdapter, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if sink == nil {
		return nil, fmt.Errorf("durable sink is required")
	}
	if config.Mode == QueueModeMemoryBuffer {
		return NewMemoryBuffer(config, sink)
	}
	return &durableAdapter{sink: sink}, nil
}

func (a *durableAdapter) Enqueue(ctx context.Context, input EnqueueInput) (BufferStatus, error) {
	if err := input.Validate(); err != nil {
		return BufferStatusDropped, err
	}
	if _, err := a.sink.EnqueueDerivedWork(ctx, input); err != nil {
		return BufferStatusDropped, err
	}
	a.stats.Durable++
	return BufferStatusDurable, nil
}

func (a *durableAdapter) Flush(context.Context) (FlushReport, error) {
	return FlushReport{}, nil
}

func (a *durableAdapter) Stats() BufferStats { return a.stats }

func (a *durableAdapter) DrainReport() DrainReport {
	return DrainReport{Mode: QueueModePostgresDurable, Durable: a.stats.Durable}
}

func (a *durableAdapter) Close() {}

func (a *durableAdapter) Start(context.Context) {}
