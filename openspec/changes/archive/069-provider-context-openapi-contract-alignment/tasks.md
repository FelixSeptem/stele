## 1. Public Contract And Regression Fixtures

- [x] 1.1 Add canonical Provider context input schemas and minimal/full request examples, with exact fields, validation, defaults, binding attribution, and rejection-only policy behavior; verify positive/negative schema cases with the existing OpenAPI validator. (At most 3 hours.)
- [x] 1.2 Add dedicated Provider context result/envelope and nested safe-item schemas, required empty arrays, optional sections, and inner/outer citation semantics; verify representative nonempty, empty, optional, and malformed result fixtures against the schemas. (At most 4 hours.)
- [x] 1.3 Add authenticated public-handler regressions using hand-authored snake-case JSON and captured assembler inputs, including unsupported schema, aliases, policy/attribution overrides, malformed objects, and trailing JSON; verify the baseline failures demonstrate the documented contract mismatch. (At most 3 hours.)

## 2. Explicit Request Mapping

- [x] 2.1 Introduce focused Provider context envelope/input DTOs and canonical key validation; reject missing/null required objects and malformed values, and verify route decode tests cover exact names rather than Go case folding. (At most 3 hours.)
- [x] 2.2 Map every supported option to existing assembly inputs using shared path validation, and reject nonempty `feedback_ranking_policy`; verify the captured service-input matrix and invalid query/budget/path tests pass. (At most 3 hours.)
- [x] 2.3 Integrate mapping into the existing Provider route while retaining supported-schema validation and binding-derived scope/session/precedence; verify assembler dispatch does not occur for unsupported schema or failed scope/binding requests. (At most 3 hours.)
- [x] 2.4 Verify ordinary diagnostics, optional insights/goals, path selection, and hidden-memory behavior remain within existing authorization and lifecycle gates; pass focused Provider/app/retrieval regressions without adding public overrides or changing ranking. (At most 3 hours.)

## 3. Typed Safe Response

- [x] 3.1 Implement explicit result/item serialization that preserves categorized order, identity, path, content, citations, and public temporal metadata while omitting scores/internal fields; verify schema and seeded-score redaction tests for every ordinary section. (At most 4 hours.)
- [x] 3.2 Implement allowlisted optional insight/goal/diagnostic serialization using existing visibility gates; verify authorized optional schemas and negative tests for raw feedback, plans, query text, and hidden identifiers. (At most 4 hours.)
- [x] 3.3 Return the typed envelope with normalized required arrays and existing bounded outer citations; verify empty-result JSON, inner/outer citation-limit behavior, and nonempty authenticated HTTP response validation. (At most 3 hours.)
- [x] 3.4 Publish the dedicated schemas on `/v1/provider/context` and verify served OpenAPI/capability digests reflect the repair, supported `provider-v1` examples pass, and unrelated native/Provider/MCP route contracts still pass their existing tests. (At most 3 hours.)

## 4. Consumer And Provider Conformance

- [x] 4.1 Extend existing Provider contract fixtures with valid nonempty/empty results and missing result/section, null/scalar sections, invalid item/citation types, and `references/content` mismatch cases; verify incompatible shapes yield failed-contract rather than empty success. (At most 3 hours.)
- [x] 4.2 Add public-boundary evidence to the context conformance path so an adapter-only successful call cannot stand in for HTTP/schema validation; verify failing/missing context shape evidence prevents a context-conformance pass. (At most 4 hours.)
- [x] 4.3 Add context safety/compatibility/redaction fixtures and bounded revision/OpenAPI provenance to the existing contract report; verify outcomes remain separate from retrieval metrics and contain no credential, authorization header, raw request/response, or unbounded content. (At most 3 hours.)

## 5. Fresh PostgreSQL And Restart Evidence

- [x] 5.1 Extend the owned self-hosted verifier with a Provider context fixture using official principal/runtime bootstrap, public governed ingestion, and bounded governance-completion checks; verify unavailable/missing dependencies produce nonpassing evidence rather than an implicit pass. (At most 4 hours.)
- [x] 5.2 Add non-root exact-path, explicit-prefix, genuine-empty, foreign-scope, and lifecycle-safe context reads to that fixture; verify serialized responses conform to the public schema and unrelated/hidden fixture evidence is absent. (At most 4 hours.)
- [x] 5.3 Run a pinned repaired image on fresh owned PostgreSQL/pgvector, repeat eligible context reads after API/worker restart, and retain only the bounded redacted report; verify governance completion, context identities/citations, build/schema/OpenAPI provenance, and restart outcomes before declaring this task complete. (At most 4 hours.)

## 6. Documentation And Delivery Checks

- [x] 6.1 Update the Provider guide with canonical HTTP examples, typed categorized output, error/compatibility migration matrix, and strict consumer empty-result handling; verify all examples against the OpenAPI schemas and keep PC2–PC4 residual gaps explicit. (At most 3 hours.)
- [x] 6.2 Review README, best practices, MCP integration guidance, and `skills/stele-memory/SKILL.md` for affected contract references, make necessary consistency edits, and update roadmap PC1 implementation status only when its evidence is complete; verify existing documentation checks pass and MCP tool signatures remain consistent. (At most 2 hours.)
- [x] 6.3 Run focused Provider/app/retrieval/assurance/OpenAPI/doc tests, required full Go tests and vet, strict change/all OpenSpec validation, and `git diff --check`; verify the final delivery report identifies fresh live evidence separately from unit/schema checks and does not claim PC2–PC6 completion. (At most 4 hours.)
