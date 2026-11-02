## 1. Source Adapter and Safety Mapping

- [x] 1.1 Define a versioned local BFCL V4 source-envelope and bounded compatibility/coverage result types; verify parser rejects missing manifests, unknown versions, and invalid checksum declarations with stable statuses.
- [x] 1.2 Implement strict normalization of `memory_kv`, `memory_rec_sum`, and `memory_vector` prerequisite/snapshot records into existing Provider contract cases; verify compatible records preserve order, scope, session, lifecycle, and refusal expectations.
- [x] 1.3 Implement explicit `unsupported` and `incomplete` outcomes for unmapped operations, ambiguous arguments, and missing prerequisite state; verify no fallback maps these records to a supported operation.
- [x] 1.4 Add source-adapter regression fixtures and tests for valid input, checksum mismatch, unavailable input, foreign scope, and hidden lifecycle targets; verify `go test ./internal/benchmark -count=1` passes.

## 2. Conformance Report and CLI

- [x] 2.1 Extend the Provider-contract report with redacted BFCL source provenance, compatibility state, supported/unsupported/incomplete counts, prerequisite digest, and cleanup evidence; verify report serialization excludes raw source, scopes, and memory identifiers.
- [x] 2.2 Add a distinct offline CLI path for local BFCL source conformance while retaining `benchmark contract --fixture` compatibility; verify valid runs are deterministic and unavailable inputs do not invoke network fallback.
- [x] 2.3 Ensure an incomplete, unsupported, checksum-mismatched, or safety-failing source run cannot be reported as passing official BFCL conformance; verify metrics remain family-scoped and are not merged with retrieval ranking results.
- [x] 2.4 Add command-level tests for compatible source output, stable failures, and existing normalized fixture replay; verify `go test ./cmd/stele -count=1` passes.

## 3. Documentation and Verification

- [x] 3.1 Document local source preparation, manifest/checksum locking, supported boundaries, report interpretation, cleanup, and the explicit non-equivalence to official BFCL scoring; verify documentation checks pass.
- [x] 3.2 Update the V1 roadmap with the implemented BFCL official-input conformance scope and retained-evidence prerequisites; verify roadmap status remains consistent with OpenSpec artifacts.
- [x] 3.3 Run `go test ./... -count=1 -timeout 15m`, `go test -race ./... -count=1 -timeout 20m`, `openspec validate --all --strict`, and `git diff --check`; retain bounded verification evidence before archival.
