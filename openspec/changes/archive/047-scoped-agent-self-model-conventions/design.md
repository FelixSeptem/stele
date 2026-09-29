## Context

See `proposal.md` for motivation and scope. The service already has normalized scoped memory paths, `profile` and `procedural` memory classes, governed memory intents, version history, provenance, lifecycle controls, exact-path/prefix retrieval, context assembly, and provider capability discovery. The design must compose those existing contracts rather than create an agent identity or authority subsystem.

## Goals / Non-Goals

**Goals:**

- Provide one optional path convention and category-to-existing-class mapping for agent self-model facts.
- Keep self-model reads explicit and bounded through existing path-aware search/context APIs.
- Make the authority boundary between remembered descriptions and server-enforced capabilities unambiguous.

**Non-Goals:**

- Adding endpoints, MCP tools, tables, memory classes, automatic context sections, or inference jobs.
- Defining how agent IDs are authenticated or granted; the convention applies only after an existing exact scope has been resolved.

## Decisions

### Path and categories

Use `agents/{agent-id}/self/{category}` within one resolved tenant/project/namespace. The supported initial categories are `capabilities`, `preferences`, `limitations`, and `lessons`. `{agent-id}` is an opaque, caller/runtime-stable path segment that must already satisfy shared memory-path validation; it is not a scope selector or credential. The path is optional, and callers that do not adopt the convention retain ordinary whole-scope behavior.

Map descriptive identity/preferences/capability/limitation statements to existing `profile` memories. Map reusable operational lessons and procedures to existing `procedural` memories. Category placement does not override memory-class validation, admission, retention, provenance, version, or lifecycle rules.

Alternative considered: introduce a dedicated `agent_self_model` class. Rejected because it would fork existing lifecycle and retrieval semantics for information that fits current classes.

### Authority and consumption

Treat self-model content as descriptive, potentially stale memory. It may inform an agent when explicitly requested using existing exact path or bounded prefix selectors. It MUST NOT authorize API calls, change grants, enable disabled server capabilities, or override server-published capability/limit documents and runtime configuration. No automatic injection occurs by default.

Alternative considered: create a dedicated self-model context tool or always-on context section. Rejected to keep MCP an adapter over existing OpenAPI behavior and avoid widening default context/budget behavior.

### Writes and lifecycle

Writes, corrections, and forgetting continue through existing governed intents or authorized administrative lifecycle operations. Updates append normal versions with provenance; this convention provides no direct canonical mutation path and no autonomous self-reflection writer.

## Risks / Trade-offs

- [Risk] Different runtimes may encode the same agent under different IDs or category names. -> Mitigation: document the initial four categories, require a stable already-valid agent ID per integration, and keep the convention optional.
- [Risk] An agent may mistake stale remembered capability or limit statements for enforceable facts. -> Mitigation: state authority precedence normatively and test that capability discovery/configuration remain independent.
- [Risk] Consumers may use a prefix and pull more self-model material than intended. -> Mitigation: recommend exact category paths for targeted reads and retain existing bounded result/context budgets and exact scope checks.

## Migration Plan

No database migration or rollout flag is required. Publish the convention in OpenSpec and operator/developer documentation. Existing memories and clients are unchanged; adopting agents may write new governed memories at the convention path. Rollback consists of ceasing to write or request those paths; persisted memories remain ordinary path-scoped records and can be managed through existing lifecycle APIs.
