## 1. Documentation information architecture

- [x] 1.1 Inventory existing README, self-hosting, roadmap, provider, OpenAPI, and MCP documents; produce a link map and verify every proposed page has one clear owner.
- [x] 1.2 Write `docs/architecture.md` with runtime modes, request/data flow, PostgreSQL/pgvector boundaries, lifecycle, isolation, and extension points; verify the guide matches `AGENTS.md` and the current service wiring.
- [x] 1.3 Write `docs/components.md` with responsibilities and interfaces for API, worker, scheduler, storage, retrieval, governance, provider, MCP, telemetry, and assurance; verify each component links to its authoritative implementation or contract.
- [x] 1.4 Write `docs/best-practices.md` with scope, lifecycle, idempotency, retrieval, citations, redaction, deployment, backup, and recovery guidance; verify examples do not introduce unsupported behavior or direct database writes.

## 2. MCP integration and Agent Skill

- [x] 2.1 Write `docs/integrations/mcp-agent-skill.md` with configuration, authentication, runtime binding, current tool matrix, request examples, annotations, limits, errors, and safe workflows; verify names and fields against `internal/mcp/schema.go` and adapter registration.
- [x] 2.2 Add `skills/stele-memory/SKILL.md` as the canonical portable agent skill; verify it teaches `who_am_i`, read workflows, governed remember, preview/apply forgetting, single-memory forget, retries, scope safety, citations, and disabled-MCP recovery.
- [x] 2.3 Add a minimal Docker-to-first-MCP-call walkthrough with placeholder credentials and exact-scope headers; verify it links to the existing self-hosting and MCP conformance commands and contains no secret-looking literals.
- [x] 2.4 Review the skill and MCP guide against the existing Stash, Letta Code, and Supermemory reference notes; verify references are clearly labeled as patterns and do not imply unsupported Stele features.

## 3. README entry point

- [x] 3.1 Expand `README.md` with product positioning, architecture summary, runtime-mode diagram or equivalent, and documentation navigation; verify the page remains concise and all local links resolve.
- [x] 3.2 Add the shortest self-hosting and MCP onboarding path to README, including the Agent Skill location and tool matrix link; verify commands match `docker-compose.yml`, `.env.local.example`, and current MCP configuration names.
- [x] 3.3 Add contribution/documentation maintenance guidance requiring MCP tool and skill updates to stay aligned; verify it points to the relevant OpenSpec and validation commands.

## 4. Documentation verification and release evidence

- [x] 4.1 Implement an offline documentation consistency check for local links, current MCP tool names, required safety rules, and forbidden credential/direct-SQL examples; verify failures identify the file and rule.
- [x] 4.2 Add or update docs and MCP tests to exercise the checker and preserve the existing tool descriptors/annotations; verify `go test ./internal/mcp ./docs` passes.
- [x] 4.3 Run README/documentation checks, MCP focused tests, strict OpenSpec validation, and `git diff --check`; verify no stale tool names, broken local links, or undocumented safety boundaries remain.
- [x] 4.4 Review the final documentation set as an operator and an agent consumer; verify a fresh reader can reach Docker startup, exact-scope MCP initialization, first read, governed write, and reviewed forget without consulting source code.
