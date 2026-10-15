# Key components

This page maps the public architecture to the repository's implementation
areas. The linked Go packages and contracts are authoritative when this guide
and an implementation differ.

## API runtime

- `internal/app`: HTTP wiring, dependency composition, health/readiness, admin
  routes, provider routes, and runtime mode startup.
- `openapi`: OpenAPI-first schemas and route publication.
- `internal/auth`: API-key/principal authentication and role checks.
- `internal/provider`: runtime binding, provider capabilities, governed adapter,
  and synchronization cursor contract.

The API resolves identity and exact scope before repository access. It should
not contain a second persistence or authorization implementation for an adapter.

## Storage and memory governance

- `internal/storage/postgres`: PostgreSQL repositories, migrations, projections,
  append-only history, leases, and scope predicates.
- `internal/memory`: memory classes, canonical versions, intents, sessions,
  lifecycle inputs, validation, and service-level contracts.
- `internal/policy`: forgetting and retention policy decisions.
- `internal/governance`: raw-event governance, claims, recovery, and audit.

Storage methods must apply exact `tenant/project/namespace` predicates and return
safe not-found or denied outcomes without confirming foreign records.

## Retrieval and context

- `internal/retrieval`: lexical, semantic, hybrid, fusion, reranking,
  context assembly, citations, temporal constraints, and bounded diagnostics.
- PostgreSQL full-text search supplies the lexical path.
- pgvector supplies semantic retrieval when configured.

Retrieval output is evidence for an agent response. It is not canonical memory
and must not be written back unchanged as if it were a new fact.

For public `context-budget-v1`, retrieval owns optional final-projection admission
inside existing context selection. `internal/provider` costs the authorized
mode-specific item DTO, resolves independent caller/server item/content/body/
citation minima and reserves bounded accounting space under `context-packing-v1`.
It finalizes required closed requested/effective/usage/selection accounting and
exact self-counting JSON size. `internal/app` writes those finalized bytes without
a newline; it never runs another packer. `internal/config` resolves item default
100, content inherited from response default 1 MiB, citation default 100 and their
hard bounds; invalid settings fail startup. `openapi` publishes portable integers,
response-only omission nulls, fixed identities and typed bounded errors.

The mandatory envelope preflight and final refusal return 413 without a partial
projection. Current exact-scope disclosure/source/version eligibility precedes
budget reasons; denied/hidden evidence contributes no private counters. Native
assembly retains its previous defaults, and public optional sections stay closed.
`internal/assurance` validates actual serialized usage and fresh initial/restart
evidence separately from ranking metrics and archived PC1/PC2 readiness. Consumers
must discover PC3 and migrate strict schemas; rollback disables dependent access
before removing the extension. See [Provider contract](agent-runtime-memory-provider.md)
for allocation units, zero/omission, errors and compatibility.

## Worker and scheduler

- `internal/jobs`: worker loops, scheduler dispatch, leases, retry/backoff,
  maintenance jobs, and durable run history.
- `internal/workqueue`: durable derived-work queue and checkpoint contracts.

Workers own asynchronous consolidation and recovery. Schedulers decide when
bounded work is dispatched; they do not directly mutate API authorization state.

## MCP adapter

- `internal/mcp/schema.go`: tool names, request shapes, validation limits, and
  tool descriptors.
- `internal/mcp/adapter.go`: Streamable HTTP MCP server, request scope
  resolution, read tools, governed remember, and lifecycle flows.
- `internal/mcp/contract_test.go`: protocol-level behavior and redaction
  coverage.

The current tools are documented in the [MCP integration guide](integrations/mcp-agent-skill.md).
MCP remains optional and disabled by default; OpenAPI remains the primary
service boundary.

## Provider adapter and synchronization

The additive PC5 durable-reference capability exposes exact task/run/handoff/
compilation reference status and lookup over existing governed memory lineage.
It is negotiated independently from context and does not own business state.

The provider adapter composes existing event, intent, retrieval, context,
lifecycle, and status services. The synchronization contract adds bounded
initial snapshots, ordered replay, cursor acknowledgment, and explicit
`resync_required` recovery. It does not create a client-owned memory store.

See [agent runtime provider](agent-runtime-memory-provider.md) and the
transport-neutral synchronization contract in
`openspec/specs/runtime-capability-and-event-sync-contract/spec.md`.

## Telemetry and assurance

- `internal/telemetry`: low-cardinality metrics and bounded lifecycle events.
- `internal/assurance`: conformance fixtures, health/readiness evidence,
  operational proofs, and recovery verification.

Telemetry must not contain raw queries, payloads, scopes, credentials, cursors,
or unbounded identifiers. Assurance evidence is separate from retrieval quality
metrics.

## Context disclosure and source trust

Public runtime credentials require explicit exact-scope disclosure provisioning.
Provider context defaults to structured references; bounded content requires the
content profile and explicit mode. Unsupported native/MCP data routes deny public
credentials before lookup. Source labels persist through governance and history;
missing lineage is untrusted/restricted. Attested source data grants no instruction,
tool, write or business-state authority. See the [disclosure/source guide](context-disclosure-and-source-trust.md)
for endpoint behavior, administration, versioning and upgrade/rollback.

### Provider context digest and continuity

The Provider HTTP component finalizes `context-digest-v1` only after disclosure,
selection and exact body accounting have completed. Its digest canonicalizer is
versioned and domain-separated; the independent assurance component recomputes
it without reusing retrieval, ranking or packing code. The PostgreSQL receipt
component added by migration `0036_provider_context_continuity` stores only the
opaque reference, full binding identity, normalized request fingerprint, digest
identity and authoritative issue/expiry times. The scheduler performs bounded
expired-only receipt cleanup. It does not delete active receipts and retained
expired rows remain subject to capacity until cleanup. No component stores a
context response as an implementation of comparison: compare reads current
authorized data and returns a full fresh body.
