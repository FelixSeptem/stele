package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/policy"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

const (
	MaxOperationMetadataBytes = 4096
	MaxOperationTokenBytes    = 256
)

type OperationMetadata struct {
	RequestID      string `json:"request_id"`
	OperationID    string `json:"operation_id"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	EventSeq       int64  `json:"event_seq,omitempty"`
	SchemaVersion  string `json:"schema_version"`
}

// Validate checks metadata without mutating the caller's envelope.
func (m OperationMetadata) Validate() error { return (&m).NormalizeValidate() }

func (m *OperationMetadata) NormalizeValidate() error {
	if m == nil {
		return fmt.Errorf("operation metadata is required")
	}
	m.RequestID = strings.TrimSpace(m.RequestID)
	m.OperationID = strings.TrimSpace(m.OperationID)
	m.IdempotencyKey = strings.TrimSpace(m.IdempotencyKey)
	m.SchemaVersion = strings.TrimSpace(m.SchemaVersion)
	for name, value := range map[string]string{"request_id": m.RequestID, "operation_id": m.OperationID, "schema_version": m.SchemaVersion} {
		if !boundedToken(value, MaxOperationTokenBytes) {
			return fmt.Errorf("%s is invalid", name)
		}
	}
	if m.IdempotencyKey != "" && !boundedToken(m.IdempotencyKey, MaxOperationTokenBytes) {
		return fmt.Errorf("idempotency_key is invalid")
	}
	if m.EventSeq < 0 {
		return fmt.Errorf("event_seq must be non-negative")
	}
	return nil
}

type SequenceDisposition string

const (
	SequenceAccepted  SequenceDisposition = "accepted"
	SequenceDuplicate SequenceDisposition = "duplicate"
	SequenceStale     SequenceDisposition = "stale"
)

type SequenceTracker struct {
	mu   sync.Mutex
	last map[string]int64
}

func NewSequenceTracker() *SequenceTracker { return &SequenceTracker{last: make(map[string]int64)} }
func (s *SequenceTracker) Accept(session string, seq int64) SequenceDisposition {
	if s == nil {
		return SequenceAccepted
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if seq <= 0 {
		return SequenceAccepted
	}
	prev, ok := s.last[session]
	if ok && seq == prev {
		return SequenceDuplicate
	}
	if ok && seq < prev {
		return SequenceStale
	}
	s.last[session] = seq
	return SequenceAccepted
}

type AdapterDependencies struct {
	Ingestor           memory.EventIngestor
	IdempotentIngestor memory.IdempotentEventIngestor
	Intent             MemoryIntentSubmitter
	Searcher           retrieval.MemorySearcher
	Assembler          retrieval.ContextAssembler
	Lifecycle          LifecycleApplier
	Session            SessionAdapter
}
type MemoryIntentSubmitter interface {
	Submit(context.Context, memory.MemoryIntentInput) (memory.MemoryIntentRecord, error)
}
type LifecycleApplier interface {
	Apply(context.Context, memory.LifecycleActionInput) error
}
type SessionAdapter interface {
	CreateTurn(context.Context, memory.CreateMemorySessionTurnInput) (memory.MemorySessionTurn, error)
	RecordTurnOutcome(context.Context, memory.RecordMemorySessionTurnOutcomeInput) (memory.MemorySessionTurn, error)
}
type Adapter struct {
	deps          AdapterDependencies
	seq           *SequenceTracker
	mu            sync.Mutex
	ingestResults map[string]IngestResult
}

func NewAdapter(deps AdapterDependencies) *Adapter {
	return &Adapter{deps: deps, seq: NewSequenceTracker(), ingestResults: make(map[string]IngestResult)}
}

type IngestResult struct {
	EventID   string                          `json:"event_id"`
	Replayed  bool                            `json:"replayed"`
	Admission *memory.AdmissionPressureReport `json:"admission,omitempty"`
	Metadata  OperationMetadata               `json:"metadata"`
}

func (a *Adapter) Ingest(ctx context.Context, binding RuntimeBinding, meta OperationMetadata, input memory.IngestEventInput) (IngestResult, error) {
	if err := a.validate(binding, &meta); err != nil {
		return IngestResult{}, err
	}
	input.Scope = binding.Scope
	if err := input.Validate(); err != nil {
		return IngestResult{}, err
	}
	disposition := a.seq.Accept(binding.SessionID, meta.EventSeq)
	if disposition == SequenceStale {
		return IngestResult{Metadata: meta}, fmt.Errorf("stale event sequence")
	}
	if disposition == SequenceDuplicate && meta.IdempotencyKey != "" {
		a.mu.Lock()
		cached, ok := a.ingestResults[binding.SessionID+":"+meta.IdempotencyKey]
		a.mu.Unlock()
		if ok {
			cached.Replayed = true
			return cached, nil
		}
	}
	if a.deps.IdempotentIngestor != nil && meta.IdempotencyKey != "" {
		r, err := a.deps.IdempotentIngestor.IngestIdempotent(ctx, input, binding.PrincipalID, meta.IdempotencyKey)
		out := IngestResult{EventID: r.Event.ID, Replayed: r.Replayed, Admission: r.Event.Admission, Metadata: meta}
		if err == nil && meta.IdempotencyKey != "" {
			a.mu.Lock()
			a.ingestResults[binding.SessionID+":"+meta.IdempotencyKey] = out
			a.mu.Unlock()
		}
		return out, err
	}
	if a.deps.Ingestor == nil {
		return IngestResult{}, fmt.Errorf("provider ingest service is not configured")
	}
	e, err := a.deps.Ingestor.Ingest(ctx, input)
	out := IngestResult{EventID: e.ID, Admission: e.Admission, Metadata: meta}
	if err == nil && meta.IdempotencyKey != "" {
		a.mu.Lock()
		a.ingestResults[binding.SessionID+":"+meta.IdempotencyKey] = out
		a.mu.Unlock()
	}
	return out, err
}
func (a *Adapter) SubmitIntent(ctx context.Context, binding RuntimeBinding, meta OperationMetadata, input memory.MemoryIntentInput) (memory.MemoryIntentRecord, error) {
	if err := a.validate(binding, &meta); err != nil {
		return memory.MemoryIntentRecord{}, err
	}
	input.Scope = binding.Scope
	input.RequestID = meta.RequestID
	input.OperationID = meta.OperationID
	input.IdempotencyKey = meta.IdempotencyKey
	if a.deps.Intent == nil {
		return memory.MemoryIntentRecord{}, fmt.Errorf("provider intent service is not configured")
	}
	return a.deps.Intent.Submit(ctx, input)
}

func (a *Adapter) CreateTurn(ctx context.Context, binding RuntimeBinding, meta OperationMetadata, input memory.CreateMemorySessionTurnInput) (memory.MemorySessionTurn, error) {
	if err := a.validate(binding, &meta); err != nil {
		return memory.MemorySessionTurn{}, err
	}
	if a.deps.Session == nil {
		return memory.MemorySessionTurn{}, fmt.Errorf("provider session service is not configured")
	}
	input.Scope = binding.Scope
	if input.SessionID == "" {
		input.SessionID = binding.SessionID
	}
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = meta.IdempotencyKey
	}
	return a.deps.Session.CreateTurn(ctx, input)
}

func (a *Adapter) RecordTurnOutcome(ctx context.Context, binding RuntimeBinding, meta OperationMetadata, input memory.RecordMemorySessionTurnOutcomeInput) (memory.MemorySessionTurn, error) {
	if err := a.validate(binding, &meta); err != nil {
		return memory.MemorySessionTurn{}, err
	}
	if a.deps.Session == nil {
		return memory.MemorySessionTurn{}, fmt.Errorf("provider session service is not configured")
	}
	input.Scope = binding.Scope
	if input.SessionID == "" {
		input.SessionID = binding.SessionID
	}
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = meta.IdempotencyKey
	}
	return a.deps.Session.RecordTurnOutcome(ctx, input)
}
func (a *Adapter) Search(ctx context.Context, binding RuntimeBinding, meta OperationMetadata, input retrieval.SearchInput) (retrieval.SearchResult, OperationMetadata, error) {
	if err := a.validate(binding, &meta); err != nil {
		return retrieval.SearchResult{}, meta, err
	}
	input.Scope = binding.Scope
	if a.deps.Searcher == nil {
		return retrieval.SearchResult{}, meta, fmt.Errorf("provider search service is not configured")
	}
	r, e := a.deps.Searcher.Search(ctx, input)
	return r, meta, e
}
func (a *Adapter) AssembleContext(ctx context.Context, binding RuntimeBinding, meta OperationMetadata, input retrieval.AssembleContextInput) (retrieval.AssembledContext, OperationMetadata, error) {
	if err := a.validate(binding, &meta); err != nil {
		return retrieval.AssembledContext{}, meta, err
	}
	input.Scope = binding.Scope
	if a.deps.Assembler == nil {
		return retrieval.AssembledContext{}, meta, fmt.Errorf("provider context service is not configured")
	}
	r, e := a.deps.Assembler.AssembleContext(ctx, input)
	return r, meta, e
}
func (a *Adapter) ApplyLifecycle(ctx context.Context, binding RuntimeBinding, meta OperationMetadata, memoryID string, action policy.ForgettingAction, reason, actor string) (OperationOutcome, error) {
	if err := a.validate(binding, &meta); err != nil {
		return OperationOutcome{Metadata: meta}, err
	}
	if a.deps.Lifecycle == nil {
		return OperationOutcome{Metadata: meta}, fmt.Errorf("provider lifecycle service is not configured")
	}
	err := a.deps.Lifecycle.Apply(ctx, memory.LifecycleActionInput{Scope: binding.Scope, MemoryID: strings.TrimSpace(memoryID), Action: action, Reason: strings.TrimSpace(reason), Actor: strings.TrimSpace(actor), RequestID: meta.RequestID})
	if err != nil {
		return OperationOutcome{Metadata: meta}, err
	}
	return OperationOutcome{Metadata: meta, Citations: []Citation{{SourceKind: "lifecycle", Reference: strings.TrimSpace(memoryID), Availability: "available"}}}, nil
}
func (a *Adapter) validate(binding RuntimeBinding, meta *OperationMetadata) error {
	if err := binding.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(binding.BindingID) == "" || strings.TrimSpace(binding.PrincipalID) == "" || strings.TrimSpace(binding.SessionID) == "" {
		return fmt.Errorf("runtime binding is invalid")
	}
	return meta.NormalizeValidate()
}

func ShapeSearchCitations(result retrieval.SearchResult) []Citation {
	out := make([]Citation, 0)
	seen := make(map[string]struct{})
	for _, h := range result.Hits {
		if h.Memory.State != memory.MemoryStateActive {
			continue
		}
		for _, c := range h.Citations {
			ref := c.MemoryID
			if ref == "" {
				ref = h.Memory.ID
			}
			key := "memory:" + ref
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, Citation{SourceKind: "memory", Reference: ref, Availability: "available"})
			if len(out) >= 100 {
				return out
			}
		}
	}
	return out
}

// ShapeContextCitations projects only lifecycle-visible memory references from
// assembled context. Query text, scores and internal diagnostics are omitted.
func ShapeContextCitations(ctx retrieval.AssembledContext) []Citation {
	result := retrieval.SearchResult{}
	result.Hits = append(result.Hits, ctx.Profile...)
	result.Hits = append(result.Hits, ctx.RecentSession...)
	result.Hits = append(result.Hits, ctx.RecentEpisodes...)
	result.Hits = append(result.Hits, ctx.RelevantSummaries...)
	result.Hits = append(result.Hits, ctx.RelatedEntities...)
	return ShapeSearchCitations(result)
}

type OperationOutcome struct {
	Metadata  OperationMetadata `json:"metadata"`
	Citations []Citation        `json:"citations,omitempty"`
}

func ShapeIntentCitation(record memory.MemoryIntentRecord) []Citation {
	if strings.TrimSpace(record.ID) == "" {
		return nil
	}
	return []Citation{{SourceKind: "intent", Reference: record.ID, Version: fmt.Sprintf("v%d", record.TargetVersion), Availability: "available"}}
}
