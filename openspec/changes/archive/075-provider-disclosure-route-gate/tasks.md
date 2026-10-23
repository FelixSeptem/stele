## 1. Public route security

- [x] 1.1 Add red/green tests for exact public route admission, current disclosure, binding/session mismatch, and nonexistence-safe denial.
- [x] 1.2 Add disclosure-mode tests for retrieve reference/content projection, scope/lifecycle/classification filtering, citation allowlisting, and omission of scores/diagnostics/query.
- [x] 1.3 Project turn input/output through a closed DTO; keep session/scope binding-owned and turn identity server-issued.
- [x] 1.4 Project turn evidence before persistence and verify reference-only summary removal plus diagnostic removal in Provider modes.
- [x] 1.5 Admit only the exact `POST /v1/provider/retrieve` and `POST /v1/provider/turns` paths in the public route gate.

## 2. Contract and maintenance

- [x] 2.1 Update OpenAPI request/response/error schemas for both routes.
- [x] 2.2 Update README, Provider integration, disclosure/source-trust matrix, and MCP agent guidance.
- [x] 2.3 Validate OpenSpec delta and documentation consistency.

## 3. Verification

- [x] 3.1 Run focused app/memory/OpenAPI tests, full Go tests, vet, and repository quality/documentation checks.
- [x] 3.2 Review diff for leaked candidate/citation/score/query fields and verify `git diff --check`.
