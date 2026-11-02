## Why

Stele's current `bfcl-memory` runner verifies repository-owned BFCL-shaped
operation fixtures, but it cannot show whether a locally supplied official BFCL
V4 memory input is compatible with the Provider contract. The roadmap keeps
BFCL V4 as a distinct provider/tool-contract track, so this gap should be
closed without claiming a full BFCL agent or leaderboard result.

## What Changes

- Add an offline, checksum-locked BFCL V4 memory-source adapter for the
  `memory_kv`, `memory_rec_sum`, and `memory_vector` subsets.
- Normalize supported source prerequisite/snapshot state into bounded Provider
  contract cases and identify unsupported source operations explicitly.
- Extend the contract report with source compatibility, supported/unsupported
  coverage, stable prerequisite/incomplete outcomes, source provenance, and
  cleanup evidence while keeping it separate from retrieval-ranking metrics.
- Add local fixtures, unit tests, CLI documentation, and a retained bounded
  evidence workflow that does not fetch datasets, call models, or run agents.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `runtime-memory-provider-contract`: accept compatible checksum-locked BFCL
  V4 memory inputs and report their Provider-contract conformance honestly.
- `benchmark-family-reporting-and-governance`: retain bounded, auditable
  provenance and cleanup evidence for official-input Provider-contract runs.

## Non-goals

- Running the BFCL agent harness, invoking an LLM, calling remote tools, or
  producing an official BFCL leaderboard score.
- Changing Stele's public Provider API, retrieval ranking behavior, lifecycle
  rules, production data, or ownership boundaries.
- Downloading or redistributing BFCL source data from this repository.

## Impact

- Affected code: `internal/benchmark`, `cmd/stele`, benchmark tests and local
  test fixtures.
- Affected documentation: `docs/local-agent-memory-benchmark.md`, the V1
  roadmap, and the relevant OpenSpec specifications.
- Validation commands: `go test ./internal/benchmark -count=1`,
  `go test ./... -count=1 -timeout 15m`, and
  `openspec validate --all --strict`.
- Related workflow guidance: `.codex/skills/openspec-apply-change/SKILL.md`
  and `scripts/openspec-archive-seq.ps1`.
