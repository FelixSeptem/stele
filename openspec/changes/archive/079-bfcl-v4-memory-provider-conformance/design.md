## Context

See [proposal.md](proposal.md) for motivation. `benchmark contract` currently
accepts repository-owned normalized `ContractCase` JSON and evaluates only its
local operation rules. It does not prove that a local, checksum-locked official
BFCL V4 memory source can be ingested without changing Provider boundaries, and
its report cannot distinguish compatible source coverage from a synthetic
fixture outcome.

The upstream BFCL V4 memory track includes `memory_kv`, `memory_rec_sum`, and
`memory_vector` backends, prerequisite/snapshot state, and tool-call grading.
Stele owns memory-provider conformance only: no model invocation, agent loop,
remote tool execution, upstream judge, or leaderboard score belongs here.

## Goals / Non-Goals

**Goals:**

- Introduce a local source-envelope format that locks the supplied BFCL source,
  records the source revision/checksum, and can retain source-specific mappings
  outside Git.
- Normalize compatible source records into the existing `ContractCase` surface
  without weakening scope, lifecycle, or refusal semantics.
- Produce a family-scoped report that separates source compatibility and
  coverage from operation and safety results, with stable failure categories.
- Keep existing hand-authored `benchmark contract --fixture` behavior
  compatible for current offline regression coverage.

**Non-Goals:**

- Implement the BFCL SDK, its model execution, Python evaluator, remote tools,
  or a score comparable to the official leaderboard.
- Fetch, mirror, or commit restricted BFCL corpus files.
- Change public Provider routes, memory storage, retrieval rankings, or
  production data semantics.

## Decisions

### 1. Add a versioned source envelope beside the existing normalized fixture

The CLI will accept a separate BFCL-source input rather than infer official
compatibility from an arbitrary `[]ContractCase`. The envelope contains a
source manifest, selected memory subset, raw-source digest, conversion version,
and bounded records with their prerequisite/snapshot references. The converter
emits existing contract cases plus source-coverage metadata.

This keeps the existing fixture format stable and makes official-input claims
auditable. Replacing `ContractCase` with a BFCL-shaped type would break local
fixtures and couple core replay to upstream source shape.

### 2. Use explicit capability mapping, not permissive field passthrough

The converter maps only supported BFCL memory operations and state forms.
Unknown operation names, ambiguous arguments, missing prerequisite state, and
unrepresentable snapshot semantics become explicit `unsupported` or
`incomplete` outcomes. It never converts them into a nearby supported call.

Permissive passthrough was rejected because it could silently widen a scope or
make a source compatibility result look more complete than it is.

### 3. Preserve existing Provider contract invariants at normalization time

The CLI supplies an exact trusted project/tenant/namespace scope independently
from the source. Normalization accepts only source cases that match it exactly,
then copies the declared session/lifecycle expectation into the local case.
Source-provided identifiers do not grant cross-scope access. Hidden lifecycle
targets stay refusal cases. The existing replay stays responsible for local
contract validity, while the source adapter reports input compatibility
independently.

This division avoids rebuilding provider semantics in a benchmark parser and
keeps safety assertions testable without PostgreSQL or a model.

### 4. Report source completeness separately from conformance success

The report gains a BFCL-source provenance/coverage section: source manifest
identity, raw and normalized digests, mapping version, prerequisite digest,
compatible/supported/unsupported/incomplete counts, and cleanup state. A
source run can be deterministic yet incomplete; it is never called a passing
official BFCL evaluation unless every selected record is compatible and all
contract/safety outcomes pass.

Reusing only `OperationAccuracy` was rejected because it hides skipped or
unsupported source records and encourages false equivalence with BFCL scoring.

### 5. Keep the source local and the CLI offline by default

The runner reads a caller-provided file, verifies its manifest and checksum,
and returns a prerequisite or checksum status if unavailable. It neither calls
an upstream URL nor resolves a model/tool dependency. Reports contain hashes
and counts only, not raw source payloads or scope/memory identity.

## Risks / Trade-offs

- [Official BFCL source schemas may evolve] → version the envelope and mapper;
  reject unknown versions explicitly instead of guessing.
- [Some BFCL semantics may not map to the Provider contract] → retain exact
  unsupported/incomplete counts and prevent a passing completion conclusion.
- [Source digests can reveal input provenance] → report only declared source
  identity/revision and cryptographic digests, not source contents or paths.
- [New report fields may be confused with leaderboard metrics] → document the
  `provider_contract` family boundary and label reports as local conformance.
- [Temporary source artifacts may outlive a run] → keep source ownership with
  the caller and have cleanup verify only runner-created normalized/report
  artifacts and benchmark-owned scope data.

## Migration Plan

1. Add failing adapter/report tests for valid compatible source, unavailable or
   drifted source, unsupported operation, and scope/lifecycle refusal.
2. Implement the source envelope parser, strict mapper, source coverage report,
   and a distinct CLI path while retaining the existing fixture path.
3. Add a representative local compatibility fixture and update benchmark docs
   with preparation, report interpretation, and non-goal boundaries.
4. Run targeted and full test suites plus strict OpenSpec validation. A local
   real-source run remains a separately retained evidence artifact; absence
   must stay explicit rather than falling back to a synthetic fixture.

## Open Questions

None. The adapter will version-gate the local envelope and report unsupported
upstream forms explicitly, so future source-shape changes do not alter this
task's contract or safety boundary.
