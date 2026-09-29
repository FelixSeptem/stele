## 1. Contract and documentation

- [x] 1.1 Add the optional path, four initial categories, and `profile`/`procedural` mapping to the self-model OpenSpec capability; verify the resulting spec covers each category and mapping.
- [x] 1.2 Document that agent IDs are path data only and remembered facts cannot grant authority or override server capability discovery, grants, configuration, and runtime limits; verify those boundaries are stated normatively.
- [x] 1.3 Add exact-category and bounded-subtree retrieval examples using existing scoped APIs; verify each example carries the exact tenant/project/namespace scope and does not introduce an MCP-specific contract.
- [x] 1.4 Update the v1 roadmap to mark P8.2 complete, list `scoped-agent-self-model-conventions` as the next proposal, and keep later reasoning-provider work separate; verify with the roadmap status contract test in `go test ./docs -run TestRoadmapTracksScopedMemoryAndSelfModelProposalStatus -count=1` (the originally named checker script is absent in this checkout).

## 2. Conformance and validation

- [x] 2.1 Add contract or documentation tests for category-to-class mapping and the non-authoritative treatment of remembered capabilities and limits; verify the focused contract tests pass.
- [x] 2.2 Add or update example conformance checks for exact scope, shared path validation, lifecycle-visible retrieval, and existing result/context budgets; verify the self-hosting contract test passes.
- [x] 2.3 Run `openspec validate --all --strict` and `pwsh -File scripts/check-self-hosting-smoke-docs.ps1`; verify both commands pass (the originally named docs-consistency script is absent in this checkout).
