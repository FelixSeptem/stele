# External Developer Documentation Portal Design

## Goal

Make Stele understandable and usable for an external developer who has not
read the source tree. The documentation must explain the service's design
principles, establish a reliable first-run path, show how to choose an
integration boundary, and point maintainers and operators to authoritative
deep references.

The work is documentation-only. It must not change the service's public
runtime behavior, API schemas, storage model, or deployment defaults.

## Audience and Outcomes

The primary audience is an external developer evaluating or integrating Stele.
After reading the primary path, that developer should be able to:

1. Explain what Stele owns and what remains owned by the agent runtime or
   product application.
2. Describe the API, worker, and scheduler runtime modes and why PostgreSQL is
   the system of record.
3. Start a local self-hosted instance and distinguish health from readiness.
4. Choose Provider, administrator MCP, or native OpenAPI access without
   accidentally crossing an authorization boundary.
5. Trace a memory from event ingestion through governance, canonical versions,
   lifecycle filtering, retrieval, citations, and asynchronous maintenance.
6. Find the authoritative contract, operational guide, test command, and
   change-management rule for a deeper task.

Maintainers and operators retain dedicated links for package ownership,
migrations, backups, recovery, observability, assurance, and contribution
workflow. These details should be discoverable without interrupting the first
integration path.

## Information Architecture

The root README is the project landing page and contains only the product
boundary, a minimal quick start, a compact integration decision table, and the
documentation map. It links to a new documentation portal at
`docs/README.md`.

The portal is organized by task rather than by package:

```text
Understand -> Run -> Integrate -> Extend and Maintain
```

The portal links to these paths:

- `docs/getting-started.md`: a short, copyable first-run tutorial with the
  smallest safe flow and links to the full self-hosting guide.
- `docs/architecture.md`: system boundaries, runtime data flow, authorization
  chain, memory lifecycle, and extension boundaries.
- `docs/components.md`: package ownership and stable service boundaries for
  maintainers.
- `docs/self-hosting.md`: complete deployment, configuration, migration,
  backup, restore, and verification procedures.
- `docs/integrations/mcp-agent-skill.md`: administrator MCP integration and
  the portable agent skill.
- `docs/agent-runtime-memory-provider.md`: public Provider integration and
  versioned context/reference/projection contracts.
- `docs/best-practices.md`: safe operational and application patterns.
- `openapi/spec.go`: generated-in-source public HTTP contract.
- `openspec/config.yaml` and active/archive change directories: planned change
  and governance workflow.

Each page must state its role near the beginning: overview, tutorial,
reference, or operations. Deep pages should link back to the portal and avoid
duplicating normative definitions from OpenAPI or versioned contract docs.

## Canonical Concepts

The portal and architecture page use one consistent vocabulary:

- **Principal**: an authenticated actor or runtime identity.
- **Exact scope**: the immutable tenant/project/namespace boundary for an
  operation.
- **Runtime binding**: the server-issued binding that combines principal,
  scope, agent, session, conversation, provider instance, and expiry.
- **Disclosure profile**: the server-authorized projection policy that controls
  whether references or bounded content may be returned.
- **Memory lifecycle**: event, candidate, active, suppressed, forgotten, and
  deleted states with immutable provenance and history.
- **System of record**: PostgreSQL; pgvector and full-text indexes are
  retrieval capabilities, not independent stores.
- **Governed write**: an event or intent accepted for asynchronous processing,
  never an inline overwrite of canonical memory.

The docs must explicitly distinguish context bytes from model tokens, current
fresh context from historical snapshots, and runtime evidence from product
task or generation state.

## Integration Decision Guidance

The portal includes a compact decision table:

| Need | Boundary | Authority |
| --- | --- | --- |
| Public agent context, governed writes, session correlation | Provider | Exact runtime binding and disclosure profile |
| Administrator inspection and reviewed memory operations | MCP | Durable administrator credential and exact scope |
| Service-to-service or custom client integration | OpenAPI | Endpoint-specific principal and scope contract |
| Internal implementation or extension | Go packages | Existing service interfaces and PostgreSQL repository rules |

The table links to detailed contracts and warns that public Provider clients
must not fall back to native or MCP reads after a Provider denial.

## Visual Model

The architecture page adds four small diagrams, each with a prose explanation:

1. Runtime modes over PostgreSQL.
2. Authorization and disclosure admission before retrieval.
3. Event-to-memory lifecycle and asynchronous worker path.
4. Agent integration decision flow.

Diagrams are explanatory only; the adjacent prose remains authoritative when a
diagram cannot express an error, rollback, or compatibility rule.

## Validation and Maintenance

Documentation changes must preserve local links, avoid credential-looking
examples and direct SQL writes, and keep MCP tool coverage synchronized. The
documentation consistency checker is extended to include the portal and
getting-started page, and tests assert required navigation headings and key
boundary terms.

The verification set is:

```text
go test ./docs ./internal/docscheck -count=1
pwsh -NoProfile -File scripts/check-documentation.ps1
git diff --check
openspec validate --all --strict
```

The final review must compare the portal links with the actual file tree and
confirm that normative behavior remains in OpenAPI or the relevant versioned
contract document.

## Non-goals

- No runtime code, endpoint, schema, migration, or configuration change.
- No new SDK, UI, hosted control plane, or product workflow.
- No replacement of detailed Provider, MCP, or self-hosting references.
- No claim that optional real PostgreSQL/pgvector evidence exists when the
  environment cannot run it.
