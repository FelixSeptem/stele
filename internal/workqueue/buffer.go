package workqueue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrBufferFull = errors.New("derived work memory buffer is full")

type DurableSink interface {
	EnqueueDerivedWork(context.Context, EnqueueInput) (DerivedWorkItem, error)
}

type BufferStatus string

const (
	BufferStatusBuffered BufferStatus = "buffered"
	BufferStatusDurable  BufferStatus = "durable"
	BufferStatusDropped  BufferStatus = "dropped"
)

type FlushReport struct {
	Attempted           int
	Durable             int
	DroppedFlushFailure int
}

type BufferStats struct {
	Buffered            int
	Durable             uint64
	DroppedOverflow     uint64
	DroppedFlushFailure uint64
	DroppedProcessLoss  uint64
}

type MemoryBuffer struct {
	config QueueConfig
	sink   DurableSink
	queue  chan EnqueueInput

	mu                 sync.Mutex
	durable            uint64
	droppedOverflow    uint64
	droppedFlushFailed uint64
	droppedProcessLoss uint64
	closed             bool
	startOnce          sync.Once
}

func NewMemoryBuffer(config QueueConfig, sink DurableSink) (*MemoryBuffer, error) {
	if config.Mode != QueueModeMemoryBuffer {
		return nil, fmt.Errorf("memory buffer requires mode %q", QueueModeMemoryBuffer)
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if sink == nil {
		return nil, fmt.Errorf("durable sink is required")
	}
	return &MemoryBuffer{config: config, sink: sink, queue: make(chan EnqueueInput, config.Capacity)}, nil
}

func (b *MemoryBuffer) Enqueue(ctx context.Context, input EnqueueInput) (BufferStatus, error) {
	if err := input.Validate(); err != nil {
		return BufferStatusDropped, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return BufferStatusDropped, fmt.Errorf("memory buffer is closed")
	}
	select {
	case b.queue <- input:
		return BufferStatusBuffered, nil
	case <-ctx.Done():
		return BufferStatusDropped, ctx.Err()
	default:
		b.droppedOverflow++
		return BufferStatusDropped, ErrBufferFull
	}
}

func (b *MemoryBuffer) Flush(ctx context.Context) (FlushReport, error) {
	var report FlushReport
	var firstErr error
	for report.Attempted < b.config.BatchSize {
		select {
		case input := <-b.queue:
			report.Attempted++
			if _, err := b.sink.EnqueueDerivedWork(ctx, input); err != nil {
				b.mu.Lock()
				b.droppedFlushFailed++
				b.mu.Unlock()
				report.DroppedFlushFailure++
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			b.mu.Lock()
			b.durable++
			b.mu.Unlock()
			report.Durable++
		default:
			return report, firstErr
		}
	}
	return report, firstErr
}

// Start begins the bounded periodic flusher. It is safe to call more than
// once; the first process-owned context controls shutdown.
func (b *MemoryBuffer) Start(ctx context.Context) {
	b.startOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(b.config.FlushInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_, _ = b.Flush(ctx)
				}
			}
		}()
	})
}

func (b *MemoryBuffer) Stats() BufferStats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return BufferStats{Buffered: len(b.queue), Durable: b.durable, DroppedOverflow: b.droppedOverflow, DroppedFlushFailure: b.droppedFlushFailed, DroppedProcessLoss: b.droppedProcessLoss}
}

func (b *MemoryBuffer) DrainReport() DrainReport {
	stats := b.Stats()
	return DrainReport{Mode: QueueModeMemoryBuffer, Buffered: stats.Buffered, Durable: stats.Durable, DroppedOverflow: stats.DroppedOverflow, DroppedFlushFailure: stats.DroppedFlushFailure, DroppedProcessLoss: stats.DroppedProcessLoss}
}

// Close records all items still in memory as process loss. It never claims
// they were durable because only a successful sink call earns that status.
func (b *MemoryBuffer) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	b.droppedProcessLoss += uint64(len(b.queue))
	b.mu.Unlock()
	for {
		select {
		case <-b.queue:
		default:
			return
		}
	}
}
