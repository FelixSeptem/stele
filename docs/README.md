# Stele documentation

This is the documentation entry point for developers evaluating, operating,
integrating, or extending Stele. The [repository README](../README.md) is the
short project overview; this page maps the next steps to the right level of
detail.

## Start here

1. Read [Getting started](getting-started.md) to run a local development stack.
2. Read [Architecture](architecture.md) to understand runtime and memory flow.
3. Choose an integration boundary in [Integrate Stele](#integrate-stele).

## Understand Stele

- [Architecture](architecture.md) explains the governing flows and boundaries.
- [Key components](components.md) maps Go packages to their responsibilities.
- [Best practices](best-practices.md) covers safe application integration.
- [Disclosure and source trust](context-disclosure-and-source-trust.md) explains
  how source labels and disclosure policy constrain returned context.

## Run Stele

- [Getting started](getting-started.md) is the short local first-run tutorial.
- [Self-hosting](self-hosting.md) is the operations reference for configuration,
  migrations, backup, restore, and recovery.

## Integrate Stele

- Use the [Provider reference](agent-runtime-memory-provider.md) for public
  agent runtimes that need governed context or memory writes.
- Use the [MCP administrator guide](integrations/mcp-agent-skill.md) for
  administrator-operated inspection and review workflows.
- Use the [OpenAPI source](../openapi/spec.go) when building an endpoint-specific
  service client. Preserve the authentication and exact-scope rules.

## Extend and maintain Stele

- Read [Key components](components.md) before changing Go package ownership.
- Use [Self-hosting](self-hosting.md) for deployment and recovery operations.
- Follow the repository's [tests](../docs/documentation_test.go) and
  [OpenSpec changes](../openspec/changes) when proposing changes.
- Treat the [OpenAPI contract](../openapi/spec.go), versioned Provider
  contracts, and OpenSpec requirements as authoritative for exact behavior;
  overview prose explains intent but does not override those contracts.
- Read the [OpenSpec configuration](../openspec/config.yaml) for the repository
  change process and [archived changes](../openspec/changes/archive) for prior
  design and implementation records.

## Authoritative references

| Concern | Authority |
| --- | --- |
| HTTP request/response shapes | [OpenAPI source](../openapi/spec.go) |
| Public runtime integration | [Provider reference](agent-runtime-memory-provider.md) and its versioned capabilities |
| Administrator tools | [MCP integration guide](integrations/mcp-agent-skill.md) |
| Deployment and operations | [Self-hosting reference](self-hosting.md) |
| Change requirements and design rationale | [OpenSpec config](../openspec/config.yaml) and [active changes](../openspec/changes) |
