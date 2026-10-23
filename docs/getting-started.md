# Getting started with Stele

**Role:** tutorial. This guide starts a local development stack; use
[Self-hosting](self-hosting.md) for production deployment, migrations, backup,
restore, and recovery.

Stele is a self-hosted memory service for agent runtimes. PostgreSQL is the only
system of record; retrieval indexes and derived projections do not replace its
versioned, governed memory records.

PostgreSQL is the only system of record.

## Before you start

Install Docker 26 or later with Docker Compose v2. The bundled stack uses
PostgreSQL with pgvector. See [Self-hosting](self-hosting.md) for supported
versions, deployment configuration, and operator-owned backup procedures.

## Start the local stack

Create a local environment file from the example, then replace its development
placeholders with values appropriate for your machine:

```powershell
Copy-Item .env.local.example .env.local
docker compose --env-file .env.local up --build -d
```

Keep `.env.local` private and do not commit it. The full configuration and
bootstrap details are documented in [Self-hosting](self-hosting.md).

## Verify process health and readiness

The API exposes these unauthenticated process checks:

- `GET /health` and `GET /livez` report that the service/runtime process is
  alive.
- `GET /ready` and `GET /readyz` report baseline dependency readiness (and, for
  `/readyz`, readiness for the configured runtime mode).

They do not establish semantic, release, or application-level readiness. Check
the deployed contract at `GET /openapi.yaml` and use the documented bootstrap
and smoke workflows before relying on a deployment.

## Understand the first safe workflow

Protected memory operations require a durable administrator to provision
principals and an exact tenant/project/namespace grant for the runtime
principal. A bootstrap-admin key creates that first administrator; it is not a
general-purpose runtime credential. Configure disclosure for the intended
scope before requesting public Provider context.

The [Provider reference](agent-runtime-memory-provider.md) documents public
runtime capability discovery, binding, context, and write operations. The
[MCP integration guide](integrations/mcp-agent-skill.md) documents
administrator-operated workflows. Do not use direct database writes in an
integration example: writes must follow Stele's governed API or MCP contract.

## Choose your next integration path

- For a public agent runtime, continue with the [Provider reference](agent-runtime-memory-provider.md).
- For an administrator reviewing memory, use the [MCP integration guide](integrations/mcp-agent-skill.md).
- For a custom endpoint-level client, use the [OpenAPI contract](../openapi/spec.go).
- For deployment operations, continue with [Self-hosting](self-hosting.md).

## Clean up and continue

Stop the local services when finished:

```powershell
docker compose --env-file .env.local down
```

This leaves the named PostgreSQL volume intact. Consult [Self-hosting](self-hosting.md)
before removing persistent data or performing any destructive cleanup. For the
system model and package boundaries, continue to [Architecture](architecture.md)
and [Key components](components.md).
