# Change: Governed Memory Intents, Reflection Runs, and Compaction Evidence

## Summary

Add a governed intent-to-reflection-to-compaction evidence path that integrates with Stele's existing versioned memory and context projection contracts.

## Motivation

Stele now has versioned context projections, bounded assembly, sessions, durable workers, lifecycle-safe retrieval, and summary compaction primitives. The remaining P2 gap is the governed bridge between external memory requests, asynchronous reflection, review, and evidence-backed compaction. Without that bridge, derived insights are difficult to deduplicate, resume, audit, replay, or safely project into context.

## What Changes

- Add an append-only, idempotent memory-intent ledger for `remember`, `update`, `forget`, `contradiction`, and `feedback`.
- Add durable reflection-run records with input watermarks, transcript schema versions, processed offsets, leases, retry budgets, and bounded failure categories.
- Add an internal/admin review contract for accepting, suppressing, rejecting, or requesting more evidence for reflection candidates.
- Add compaction evidence records that bind summaries to source event ranges, watermarks, canonical versions, derivation versions, token estimates, and evidence coverage.
- Integrate approved/eligible derived outputs with existing class-aware context projections while preserving scope and lifecycle fail-closed behavior.
- Add replay, restart, isolation, lifecycle, and recovery conformance coverage and bounded diagnostics.

## Out of Scope

- A user-facing review UI.
- A second canonical store or Git/MemFS persistence model.
- Direct agent/provider writes to canonical memory.
- New SDKs, client products, MCP/WebSocket authorization boundaries, or provider-specific APIs.

## Impact

Affected areas include PostgreSQL schema/repositories, memory governance services, worker/scheduler orchestration, compaction and projection integration, OpenAPI contracts, diagnostics, and tests/docs.
