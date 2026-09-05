# Stele Self-Hosting

## Overview

`Stele` runs as three process modes against one PostgreSQL instance:

- `api`: public ingest, memory read, retrieval, context assembly, and admin inspection or lifecycle routes
- `worker`: continuous governance processing loop for claimed raw events
- `scheduler`: periodic maintenance dispatch for embedding rebuild, derived insight derivation, summary compaction, retention sweep, and job execution cleanup

The repository ships a production-oriented `Dockerfile` plus a local `docker-compose.yml` so operators can boot the full stack without reconstructing runtime wiring by hand.

## Prerequisites

- Docker 26+ and Docker Compose v2
- PostgreSQL 17+ with:
  - `pgcrypto`
  - `vector` from `pgvector`
- One reachable DSN for all three Stele modes

The bundled compose file uses `pgvector/pgvector:pg17`, which already includes the required `vector` extension.
The PostgreSQL data volume is mounted at `/var/lib/postgresql` so the same
configuration also supports PostgreSQL 18's major-version-specific data
directory layout.

## Runtime Variables

Required in all modes:

- `STELE_MODE`: one of `api`, `worker`, `scheduler`
- `STELE_POSTGRES_DSN`: PostgreSQL connection string

Common optional variables:

- `STELE_HTTP_ADDR`: listen address for `api` mode, default `:8080`
- `STELE_AUTH_BOOTSTRAP_ADMIN_KEY`: one temporary operator secret used only to create the first durable admin principal
- `STELE_AUTH_API_KEYS` and `STELE_AUTH_ADMIN_API_KEYS`: deprecated unrestricted key lists; startup rejects them. Do not copy these names into a deployment; use the constrained bootstrap flow below.
- `STELE_DATABASE_MIGRATION_POLICY`: `auto` (default), `validate`, or `off` for externally managed forward migrations
- `STELE_HTTP_MAX_REQUEST_BODY_BYTES`, `STELE_HTTP_MAX_HEADER_BYTES`: bounded request limits
- `STELE_HTTP_READ_HEADER_TIMEOUT`, `STELE_HTTP_READ_TIMEOUT`, `STELE_HTTP_WRITE_TIMEOUT`, `STELE_HTTP_IDLE_TIMEOUT`, `STELE_HTTP_SHUTDOWN_TIMEOUT`: bounded HTTP and drain timeouts
- `STELE_AUTH_DEFAULT_TENANT`: default scheduler tenant scope
- `STELE_AUTH_DEFAULT_PROJECT`: default scheduler project scope
- `STELE_AUTH_DEFAULT_NAMESPACE`: default scheduler namespace scope

Migration policy is evaluated before API traffic or background job claims. Use
`auto` for the bundled single-node Compose profile, or run the standalone
migration command before starting all modes and use `validate` when migrations
are managed separately. `off` does not bypass compatibility checks; it is only
for an explicitly documented external migration workflow.

The standalone migration commands are:

```bash
STELE_POSTGRES_DSN='<operator-managed-dsn>' stele migrate status
STELE_POSTGRES_DSN='<operator-managed-dsn>' stele migrate up
STELE_MIGRATION_OUTPUT=json STELE_POSTGRES_DSN='<operator-managed-dsn>' stele migrate status
```

`migrate up` is forward-only and uses the same PostgreSQL migration ledger and
serialization as runtime startup. It never performs an automatic downgrade.

`migrate status` reports the driver state (`status`, `current_version`,
`latest_version`, `dirty`, and `pending`) together with
`integrity_status`/`integrity_rows`. A `verified` integrity status means every
applied numbered migration has the expected embedded SHA-256 record in
`stele_schema_migration_ledger`. A clean older supported deployment can report
`legacy` until the next serialized `migrate up` records that immutable prefix;
do not accept runtime traffic under validate-only policy until status is both
`current` and `verified`.

If `migrate status` reports `dirty`, `incompatible`, or `divergent`, do not
restart the application modes with `auto` and do not edit a historical
migration asset or either migration ledger by hand. Stop the deployment,
preserve a PostgreSQL backup, inspect the bounded diagnostic, and either run
the documented forward-remediation migration or restore the verified backup
into an explicit target. Automatic down migration is prohibited; an older image
may only be used when its schema compatibility is confirmed. A failed migration
must be repaired or restored before readiness is considered valid.

## PostgreSQL Backup, Restore, And Verification

Backup and restore are operator-owned procedures. Install the PostgreSQL client
tools (`pg_dump`, `pg_restore`, and `psql`) on the operator host, provide an
explicit source/target DSN, and keep the resulting artifact and manifest in a
protected directory. The scripts never print connection strings or passwords.

Create a checksum-backed custom-format backup:

```powershell
pwsh -File scripts/stele-backup.ps1 `
  -SourceDsn $env:STELE_POSTGRES_DSN `
  -Destination .\backups\stele-$(Get-Date -Format yyyyMMdd-HHmmss).dump `
  -ServiceVersion $env:STELE_SERVICE_VERSION
```

Restore only into a distinct disposable or replacement target. The target must
be explicit and `-ConfirmDestructive` is mandatory; source-equal and template
databases are refused:

```powershell
pwsh -File scripts/stele-restore.ps1 `
  -Artifact .\backups\stele.dump `
  -Manifest .\backups\stele.dump.manifest.json `
  -SourceDsn $env:STELE_POSTGRES_DSN `
  -TargetDsn $env:STELE_VERIFY_POSTGRES_DSN `
  -ConfirmDestructive
```

Start Stele against the restored target, then run bounded migration and
scope-safe read verification:

```powershell
pwsh -File scripts/stele-restore-verify.ps1 `
  -TargetDsn $env:STELE_VERIFY_POSTGRES_DSN `
  -Manifest .\backups\stele.dump.manifest.json `
  -BaseUrl http://localhost:8080 `
  -ApiKey $env:STELE_RUNTIME_API_KEY `
  -Tenant $env:STELE_AUTH_DEFAULT_TENANT `
  -Project $env:STELE_AUTH_DEFAULT_PROJECT `
  -Namespace $env:STELE_AUTH_DEFAULT_NAMESPACE
```

Only a successful verification should be recorded through the authenticated
assurance recovery-verification workflow as `backup_restore_proof`. Backups,
retention, scheduling, storage durability, and recovery point/recovery time
objectives remain the operator's responsibility; Stele does not schedule or
upload backups. Never overwrite the source database implicitly.

API mode publishes the running contract at `GET /openapi.yaml` and bounded
build/schema compatibility metadata at `GET /version`. Both endpoints are
unauthenticated discovery surfaces and intentionally exclude DSNs, credentials,
scope values, migration SQL, and operational backlog details.

## Principal Bootstrap And Scoped Access

Protected requests are authenticated against PostgreSQL-backed principals. Every
request must include `X-API-Key`, `X-Stele-Tenant`, `X-Stele-Project`, and
`X-Stele-Namespace`; the exact scope must be granted to the principal. Public
principals can use public memory and event routes. Admin routes additionally
require the `admin` role.

For a first deployment, configure all four bootstrap variables and start the API:

```bash
export STELE_AUTH_BOOTSTRAP_ADMIN_KEY='<temporary-bootstrap-secret>'
export STELE_AUTH_DEFAULT_TENANT='tenant-a'
export STELE_AUTH_DEFAULT_PROJECT='project-a'
export STELE_AUTH_DEFAULT_NAMESPACE='namespace-a'
```

Create the first durable administrator within that default scope:

```bash
curl -X POST http://localhost:8080/v1/admin/principals \
  -H 'X-API-Key: <temporary-bootstrap-secret>' \
  -H 'X-Stele-Tenant: tenant-a' -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -H 'Content-Type: application/json' \
  -d '{"role":"admin","label":"operator"}'
```

The `credential_secret` in this response is returned once. Store it in a secret
manager, remove the bootstrap key from the environment, and rotate or revoke
credentials through the admin endpoints. Principal list/read, grant, and audit
responses never contain raw credentials or digests. Grant administration is
exact-scope only; a principal cannot create or request a different scope by
changing headers.

Event clients must send a bounded, stable `Idempotency-Key` on every
`POST /v1/events`. An exact retry returns the original `event_id` with
`replayed=true`; reusing a key for another payload returns `409`. A leased
in-progress claim also returns `409` with `Retry-After: 1`. Admission rejection
returns `422` and does not create a completed idempotency result, so clients may
retry after the admission condition is resolved.

Embedding routing variables:

- `STELE_EMBEDDING_DEFAULT_PROVIDER`: default provider name recorded on rebuild eligibility and vector revisions
- `STELE_EMBEDDING_DEFAULT_MODEL`: default model name recorded on rebuild eligibility and vector revisions
- `STELE_EMBEDDING_DEFAULT_DIMENSIONS`: default vector dimension target used for drift detection and rebuild eligibility
- `STELE_EMBEDDING_CLASS_ROUTES`: optional comma-separated per-class overrides in `class=provider:model:dimensions` format
- `STELE_EMBEDDING_OPENAI_API_KEY`: API key for the built-in OpenAI-compatible embedding provider
- `STELE_EMBEDDING_OPENAI_BASE_URL`: optional base URL override for OpenAI-compatible deployments, default `https://api.openai.com/v1`
- `STELE_EMBEDDING_OPENAI_TIMEOUT`: outbound embedding request timeout, default `30s`

Job tuning variables:

- `STELE_JOBS_MAINTENANCE_INTERVAL`: scheduler cadence, default `15m`
- `STELE_JOBS_SUMMARY_COMPACTION_INTERVAL`: summary compaction cadence, default `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_RETENTION_INTERVAL`: retention and expiry sweep cadence, default `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_CLEANUP_INTERVAL`: maintenance cleanup cadence, default `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_DERIVED_INSIGHT_DERIVATION_INTERVAL`: governed experience insight derivation cadence, default `STELE_JOBS_MAINTENANCE_INTERVAL`
- `STELE_JOBS_DERIVED_INSIGHT_BATCH_SIZE`: maximum failure evidence records read per scope and derivation run, default `100`
- `STELE_JOBS_DERIVED_INSIGHT_MINIMUM_EVIDENCE`: minimum repeated evidence count required before activating a `failure_pattern`, default `2`
- `STELE_JOBS_JOB_EXECUTION_RETENTION`: retention window for finished job execution records, default `168h`
- `STELE_JOBS_WORKER_POLL_INTERVAL`: worker idle poll delay, default `5s`
- `STELE_JOBS_WORKER_ERROR_BACKOFF`: worker retry backoff, default `15s`
- `STELE_JOBS_SCHEDULER_ERROR_BACKOFF`: scheduler retry backoff, default `30s`
- `STELE_JOBS_GOVERNANCE_MAX_ATTEMPTS`: automatic retry budget for claimed governance raw events, default `5`
- `STELE_JOBS_GOVERNANCE_RETRY_BACKOFF`: retry wait after a failed governance attempt, default `30s`
- `STELE_JOBS_GOVERNANCE_LEASE_RENEW_INTERVAL`: cadence for renewing an in-flight governance claim lease, default `30s`
- `STELE_JOBS_MAINTENANCE_SCOPE_BATCH_LIMIT`: maximum discovered scopes evaluated per scheduler tick, default `100`

## Embedding Deployment Modes

Lexical-only deployment:

- Leave `STELE_EMBEDDING_DEFAULT_PROVIDER`, `STELE_EMBEDDING_DEFAULT_MODEL`, `STELE_EMBEDDING_DEFAULT_DIMENSIONS`, and `STELE_EMBEDDING_CLASS_ROUTES` unset.
- `api`, `worker`, and `scheduler` still start normally.
- Hybrid retrieval remains available through lexical and relation signals, while semantic rebuild execution stays intentionally inactive.
- Admin embedding diagnostics report `semantic_rebuild_enabled=false` together with the degraded runtime reason.

Provider-backed deployment:

- Configure at least one embedding route target through `STELE_EMBEDDING_DEFAULT_*` or `STELE_EMBEDDING_CLASS_ROUTES`.
- Provide the matching provider credentials and endpoint settings, for example `STELE_EMBEDDING_OPENAI_API_KEY` for the built-in OpenAI-compatible provider.
- All three runtime modes use the same provider registration rules, so `api`, `worker`, and `scheduler` resolve the same provider names.
- Startup fails fast if a configured route points at an unknown provider or if provider-specific settings are incomplete.

Example provider-backed environment:

```bash
export STELE_EMBEDDING_DEFAULT_PROVIDER=openai
export STELE_EMBEDDING_DEFAULT_MODEL=text-embedding-3-small
export STELE_EMBEDDING_DEFAULT_DIMENSIONS=1536
export STELE_EMBEDDING_CLASS_ROUTES='profile=openai:text-embedding-3-small:1536,episodic=openai:text-embedding-3-small:1536'
export STELE_EMBEDDING_OPENAI_API_KEY='<provider-key>'
```

## Local Bootstrap With Compose

Start the stack:

```bash
docker compose up --build -d
```

Watch the service come up:

```bash
docker compose ps
docker compose logs -f api worker scheduler
```

Stop the stack:

```bash
docker compose down
```

Reset the stack including PostgreSQL data:

```bash
docker compose down -v
```

## Manual Image Build

Build the service image directly:

```bash
docker build -t stele:local .
```

Run API mode manually with the same bootstrap-admin configuration:

```bash
docker run --rm -p 8080:8080 \
  -e STELE_MODE=api \
  -e STELE_HTTP_ADDR=:8080 \
  -e STELE_POSTGRES_DSN='postgres://stele:stele@host.docker.internal:5432/stele?sslmode=disable' \
  -e STELE_AUTH_BOOTSTRAP_ADMIN_KEY='<bootstrap-secret>' \
  -e STELE_AUTH_DEFAULT_TENANT=tenant-a \
  -e STELE_AUTH_DEFAULT_PROJECT=project-a \
  -e STELE_AUTH_DEFAULT_NAMESPACE=namespace-a \
  stele:local
```

## Smoke Check

### First-ten-minutes operational loop

Use this loop after `docker compose up --build -d` to prove the self-hosted `api`, `worker`, and `scheduler` modes are wired end to end. The loop uses explicit scope headers throughout; do not omit them or reuse data from another namespace.

Smoke fixture scope:

```text
tenant=tenant-a
project=project-a
namespace=namespace-a
bootstrap_admin_key=<bootstrap-secret>
runtime_api_key=<durable-runtime-credential-created-by-bootstrap>
actor=operator-a
```

For a repeatable PowerShell smoke flow, run the repository script against the
running API. It creates the durable admin, creates a public runtime principal,
writes each one-time credential only to the explicitly supplied directory, and
checks that the bootstrap credential is rejected after the durable admin exists:

```powershell
pwsh -File scripts/stele-bootstrap-smoke.ps1 `
  -BootstrapKey $env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY `
  -Tenant $env:STELE_AUTH_DEFAULT_TENANT `
  -Project $env:STELE_AUTH_DEFAULT_PROJECT `
  -Namespace $env:STELE_AUTH_DEFAULT_NAMESPACE `
  -CredentialOutputDirectory .\.stele-smoke-credentials
```

The lifecycle portion is enabled by default and sends an idempotent event retry,
then exercises scoped memory listing, search, and context assembly with the new
runtime credential. Use `-SkipLifecycle` only when you are splitting bootstrap
from a separately orchestrated worker/retrieval verification.

For the isolated real-stack product gate, run:

```powershell
pwsh -File scripts/stele-product-verify.ps1
```

This creates a unique Compose project and generated credentials, runs the
bootstrap/lifecycle smoke, and removes only its own containers, network, and
volume. On a developer machine without Docker or a running daemon it exits with
status `2` and prints `SKIP`; this is explicitly not a pass. CI sets
`STELE_PRODUCT_VERIFY_CI=1`, where missing Docker prerequisites fail the job.
Use `-KeepResources` only to retain the uniquely named verification resources
for bounded diagnostics.

For restricted networks, set `STELE_PRODUCT_VERIFY_POSTGRES_IMAGE` to an image
reference available from your configured registry mirror. Compose also accepts
`STELE_POSTGRES_IMAGE`, `STELE_POSTGRES_HOST_PORT`, and `STELE_HTTP_HOST_PORT`
overrides; the isolated verifier selects random host ports automatically.

For example, when the `1ms.run` Docker registry proxy is reachable:

```powershell
$env:STELE_PRODUCT_VERIFY_POSTGRES_IMAGE = "docker.1ms.run/pgvector/pgvector:pg17"
$env:STELE_PRODUCT_VERIFY_GO_IMAGE = "docker.1ms.run/library/golang:1.25-bookworm"
$env:STELE_PRODUCT_VERIFY_RUNTIME_IMAGE = "docker.1ms.run/library/debian:bookworm-slim"
$env:STELE_PRODUCT_VERIFY_GOPROXY = "https://goproxy.cn,direct"
pwsh -File scripts/stele-product-verify.ps1
```

The proxy host is an operator choice and is not contacted unless this override
is explicitly set.

The same build-image overrides are available directly in Compose as
`STELE_GO_IMAGE`, `STELE_RUNTIME_IMAGE`, and `STELE_GOPROXY`. The defaults
remain the official Docker Hub and Go module proxy references.

Treat `.stele-smoke-credentials` as secret material: move the values into an
operator-managed secret store and remove the directory after the smoke run.

Before ingesting the smoke fixture, create the first durable admin with the
bootstrap key and then create an exact-scope public/runtime principal. The
bootstrap secret is only valid for the configured default scope and is rejected
after a durable admin exists. Store each returned `credential_secret` once and
use the runtime principal for normal event and retrieval requests.

1. Confirm process liveness and readiness:

```bash
curl http://localhost:8080/livez
curl http://localhost:8080/readyz
```

2. Ingest scoped smoke fixture events. The repeated `smoke.provider_failure` events create eligible evidence for derived insight derivation and replay; `smoke.operator_recovery` gives later retrieval and context assembly an operator-resolution phrase to find.

```bash
curl -X POST http://localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <durable-runtime-credential-created-by-bootstrap>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"event_type":"smoke.provider_failure","content":"smoke fixture: embedding provider unavailable during rebuild attempt","source_timestamp":"2026-07-11T00:00:00Z"}'

curl -X POST http://localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <durable-runtime-credential-created-by-bootstrap>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"event_type":"smoke.provider_failure","content":"smoke fixture: embedding provider unavailable during rebuild retry","source_timestamp":"2026-07-11T00:01:00Z"}'

curl -X POST http://localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <durable-runtime-credential-created-by-bootstrap>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"event_type":"smoke.operator_recovery","content":"smoke fixture: operator waits for provider recovery before retrying rebuilds","source_timestamp":"2026-07-11T00:02:00Z"}'
```

3. Inspect worker and scheduler progress without querying PostgreSQL directly:

```bash
curl http://localhost:8080/v1/admin/jobs/governance/status \
  -H 'X-API-Key: <admin-credential>'

curl 'http://localhost:8080/v1/admin/jobs/status?limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

If governance remains pending, inspect worker logs and `STELE_POSTGRES_DSN`. If scheduler executions are absent, verify the `scheduler` process is running and that `STELE_AUTH_DEFAULT_TENANT`, `STELE_AUTH_DEFAULT_PROJECT`, and `STELE_AUTH_DEFAULT_NAMESPACE` match the fixture scope.

4. Verify retrieval and context assembly. The diagnostics flag is safe for this authenticated request and returns category/count explanations such as `included`, `omitted_by_budget`, `omitted_by_quality`, or `hidden_by_lifecycle_or_scope`.

```bash
curl -X POST http://localhost:8080/v1/memories/search \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"query":"provider recovery rebuild","top_k":5,"include_summaries":true,"include_relations":true}'

curl -X POST http://localhost:8080/v1/context/assemble \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"query":"avoid repeated provider rebuild failures","budget":6,"include_experience_insights":true,"include_diagnostics":true}'
```

5. Run bounded derived insight replay dry-run. This previews replay decisions without mutating derived insights or canonical memory.

```bash
curl -X POST http://localhost:8080/v1/admin/derived-insight-replays:dry-run \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"insight_types":["failure_pattern","lesson"],"evidence_window_start":"2026-07-11T00:00:00Z","evidence_window_end":"2026-07-11T00:10:00Z","evidence_limit":20,"actor":"operator-a","reason":"first-ten-minutes smoke replay dry-run"}'
```

Expected report shape includes `counters.evidence_evaluated`, decision categories such as `create` or `skip`, and stable reason codes such as `repeated_evidence` or `insufficient_evidence`.

6. Queue bounded replay apply and inspect the durable run. Apply returns a replay run id; broad mutations are executed later by scheduler-owned background work.

```bash
curl -X POST http://localhost:8080/v1/admin/derived-insight-replays \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"insight_types":["failure_pattern","lesson"],"evidence_window_start":"2026-07-11T00:00:00Z","evidence_window_end":"2026-07-11T00:10:00Z","evidence_limit":20,"actor":"operator-a","reason":"first-ten-minutes smoke replay apply","idempotency_key":"smoke-replay-tenant-a-project-a-namespace-a"}'

curl 'http://localhost:8080/v1/admin/derived-insight-replays?mode=apply&limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl http://localhost:8080/v1/admin/derived-insight-replays/<replay-run-id>/report \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

If the run stays `pending`, verify scheduler mode is running and inspect `derived_insight_replay_execution` in `/v1/admin/jobs/status`. If the run is `failed`, read the replay report first; it contains partial counters and failure summary. If the run is `continuation_required`, repeat replay with a later bounded window or adjusted `evidence_limit`.

7. Confirm replay output can become context-visible only through ordinary lifecycle, quality, scope, and budget rules:

```bash
curl -X POST http://localhost:8080/v1/context/assemble \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"query":"avoid repeated provider rebuild failures","budget":8,"include_experience_insights":true,"include_diagnostics":true}'
```

Successful replay visibility is shown by `known_failures` or `experience_lessons` entries with citations. Absence is not automatically a failure; use `diagnostics` to distinguish budget pressure, quality ranking, lifecycle hiding, or lack of eligible evidence.

8. Scrape metrics for low-cardinality replay and smoke signals:

```bash
curl http://localhost:8080/metrics | grep 'stele_derived_insight_replay_total'
curl http://localhost:8080/metrics | grep 'stele_operations_total'
```

Replay metrics use labels such as `mode`, `result`, `insight_type`, `decision`, and `reason`. They intentionally exclude tenant, project, namespace, replay run id, insight id, actor, reason text, and raw error messages.

### Durable scope proof and memory session loop

Use the durable proof workflow when you need an auditable answer to whether a tenant/project/namespace can accept memory, process it, retrieve it, assemble context, surface quality findings, recommend repair, and be rerun after remediation. This replaces treating unrelated smoke endpoint responses as the primary source of truth.

1. Create a scoped proof run:

```bash
curl -X POST http://localhost:8080/v1/admin/scope-proofs \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"checks":["scope_resolution","ingestion","governance","retrieval","context","replay","quality","repair"],"fixture_mode":"smoke","actor":"operator-a","reason":"prove fresh scope memory loop"}'
```

2. Inspect the proof and report. The run contains step state; the report adds linked evidence and stable next actions.

```bash
curl http://localhost:8080/v1/admin/scope-proofs/<proof-run-id> \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl http://localhost:8080/v1/admin/scope-proofs/<proof-run-id>/report \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

3. Diagnose failures from the report's `next_actions` instead of guessing from raw HTTP responses:

- `inspect_governance_status` or `inspect_worker_jobs`: read `/v1/admin/jobs/governance/status` and `/v1/admin/jobs/status`.
- `inspect_retrieval_quality`: create or open `/v1/admin/memory-quality/evaluations`.
- `inspect_context_diagnostics`: rerun `/v1/context/assemble` with `"include_diagnostics":true`.
- `open_quality_evaluation`: inspect `/v1/admin/memory-quality/evaluations/<evaluation-run-id>/findings`.
- `open_repair_plan`: inspect or approve `/v1/admin/memory-quality/repair-plans/<repair-plan-id>` only after review.
- `inspect_derived_insight_replay`: read `/v1/admin/derived-insight-replays/<replay-run-id>/report`.

4. Rerun after remediation. Reruns create a new proof linked to the previous run; they do not overwrite the original evidence.

```bash
curl -X POST http://localhost:8080/v1/admin/scope-proofs/<proof-run-id>:rerun \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-a","reason":"rerun after proof remediation"}'
```

5. Exercise the external-agent memory session contract. Stele assembles context and records memory-relevant outcomes, but the external agent still owns model calls, prompt orchestration, and final answers.

```bash
curl -X POST http://localhost:8080/v1/memory-sessions \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"agent-a","reason":"serve user turn","metadata":{"integration":"self-hosting-smoke"}}'

curl -X POST http://localhost:8080/v1/memory-sessions/<session-id>/turns \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"query":"remember deployment preference","context_budget":1200,"include_relations":true,"include_experience_insights":true,"include_diagnostics":true}'

curl -X POST http://localhost:8080/v1/memory-sessions/<session-id>/turns/<turn-id>:outcome \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"outcome_event_ids":["<event-id>"],"expected_recall":["<event-id>"]}'

curl -X POST http://localhost:8080/v1/memory-sessions/<session-id>:verify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"turn_id":"<turn-id>","expected_recall":["<event-id>"]}'

curl http://localhost:8080/v1/memory-sessions/<session-id>/report \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

6. Scrape proof/session metrics. These metrics are intentionally low-cardinality and exclude tenant, project, namespace, proof id, session id, turn id, event id, memory id, actor, and reason text.

```bash
curl http://localhost:8080/metrics | grep 'stele_scope_proof_steps_total'
curl http://localhost:8080/metrics | grep 'stele_memory_session_verifications_total'
```

This closes the service-side loop: `scope proof -> session context -> external agent turn -> turn outcome ingestion -> governance -> retrieval/context verification -> quality/repair recommendation -> rerun proof/session`.

### Production-readiness assurance and conformance loop

Use this loop after the smoke, scope proof, and memory session checks when an operator needs a durable answer to whether one tenant/project/namespace is ready for production. The routes are admin-only and must use the same scope headers throughout.

1. Run a health evaluation. The request can include service-owned capacity/load proof and operator-supplied backup/restore proof evidence; Stele stores bounded categories and counters, not raw deployment secrets or external backup payloads.

```bash
curl -X POST http://localhost:8080/v1/admin/assurance/health-evaluations \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"capacity_load_proof":{"status":"healthy","severity":"info","reason":"capacity_within_thresholds","evidence":{"backlog_depth":0,"worker_latency_ms":120}},"backup_restore_proof":{"status":"healthy","severity":"info","reason":"backup_restore_fresh","evidence":{"marker":"restore-check-2026-07-13"}}}'
```

2. Define the external-agent integration evidence contract. Conformance profiles state what Stele evidence an external agent is expected to leave behind; Stele does not execute the agent, invoke models, build prompts, or generate final answers.

```bash
curl -X POST http://localhost:8080/v1/admin/assurance/conformance-profiles \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"expected_evidence":[{"kind":"session","minimum_count":1,"freshness_window":"24h"},{"kind":"context","minimum_count":1,"freshness_window":"24h"},{"kind":"outcome","minimum_count":1,"freshness_window":"24h"},{"kind":"verification","minimum_count":1,"freshness_window":"24h"},{"kind":"usefulness_feedback","minimum_count":1,"freshness_window":"24h"},{"kind":"task_evaluation","minimum_count":1,"freshness_window":"24h"},{"kind":"proof","minimum_count":1,"freshness_window":"24h"}],"actor":"operator-a","reason":"production readiness evidence chain"}'
```

3. Run conformance and inspect missing-evidence diagnostics. Missing evidence uses stable categories such as `session_without_outcome`, `turn_without_context`, `verification_missing`, `feedback_without_subject`, `task_evaluation_missing_evidence`, `repair_without_verification`, or `rollout_without_dry_run`.

```bash
curl -X POST http://localhost:8080/v1/admin/assurance/conformance-runs \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"profile_id":"<conformance-profile-id>"}'

curl 'http://localhost:8080/v1/admin/assurance/conformance-runs?profile_id=<conformance-profile-id>' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

4. Generate and inspect the scope readiness report. Readiness combines latest health evaluation, conformance run, capacity/load proof, backup/restore proof, active incident counters, alert candidate counters, and recommended admin surfaces.

```bash
curl -X POST http://localhost:8080/v1/admin/assurance/readiness-reports \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{}'

curl 'http://localhost:8080/v1/admin/assurance/readiness-reports' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

5. Review incidents and alert candidates. Incidents preserve append-only operational degradation history. Alert delivery adapters are intentionally limited to `disabled`, `stdout`, and generic `webhook`; webhook delivery is HTTPS-by-default, uses unsafe target rejection, bounds timeout and payload size, redacts configured secrets, and does not add vendor-specific alert behavior.

```bash
curl 'http://localhost:8080/v1/admin/assurance/incidents' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl 'http://localhost:8080/v1/admin/assurance/alert-candidates' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl 'http://localhost:8080/v1/admin/assurance/alert-candidates/<alert-candidate-id>/delivery-attempts' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

6. Remediate through the recommended runbook hints, then verify recovery. Recovery verification links the remediation evidence to an incident, alert candidate, conformance run, repair result, ranking rollback, proof run, session verification, capacity/load proof, or backup/restore proof without overwriting prior history.

```bash
curl -X POST http://localhost:8080/v1/admin/assurance/recovery-verifications \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"target":"incident","target_id":"<incident-id>","status":"healthy","result_category":"recovered","checked_surfaces":["readiness","conformance","capacity_load"],"actor":"operator-a","reason":"verified after remediation"}'
```

7. Scrape assurance metrics and logs. Metrics use series such as `stele_assurance_health_evaluations_total`, `stele_assurance_incidents_total`, `stele_assurance_alert_candidates_total`, `stele_assurance_alert_delivery_total`, `stele_assurance_cleanup_total`, `stele_conformance_runs_total`, `stele_conformance_missing_evidence_total`, `stele_operational_proofs_total`, `stele_readiness_reports_total`, and `stele_recovery_verifications_total`. Structured lifecycle logs use `component=assurance event=lifecycle` and `component=conformance event=lifecycle`.

Metrics and lifecycle logs intentionally exclude tenant, project, namespace, record ids, actor, reason text, query text, webhook URL, and recipient fields. Use the admin inspection routes above for scoped record details.

Runtime startup and drain telemetry is also bounded to `mode`, `component`,
`operation`, and `status`. It records migration validation, startup, signal,
readiness/drain, timeout, and cleanup outcomes without emitting DSNs,
credentials, scopes, principals, or raw error text. Product verification emits
only phase/result categories; detailed diagnostics remain in the local test
output or explicitly retained owned Compose resources.

Remaining product gaps after this service loop are explicit: SDK/UI onboarding, external agent runtime integration, vendor-specific alert routing, hosted incident management, and adaptive scoring calibration remain outside the service-owned scope.

### Baseline startup

1. Check liveness:

```bash
curl http://localhost:8080/health
```

Expected:

```json
{"status":"ok"}
```

2. Check readiness:

```bash
curl http://localhost:8080/ready
```

Expected:

```json
{"status":"ready"}
```

`/ready` only confirms baseline process dependencies such as PostgreSQL connectivity. It does not prove semantic rebuild execution is wired.

3. Ingest one event:

```bash
curl -X POST http://localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"event_type":"conversation.message","content":"user prefers concise answers"}'
```

4. Inspect governance backlog:

```bash
curl http://localhost:8080/v1/admin/jobs/governance/status \
  -H 'X-API-Key: <admin-credential>'
```

5. Inspect recent scheduler and maintenance executions:

```bash
curl 'http://localhost:8080/v1/admin/jobs/status?limit=5' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

6. Inspect a memory history after governance promotion:

```bash
curl http://localhost:8080/v1/admin/memories/<memory-id>/history \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

7. Inspect governance recovery candidates in one scope:

```bash
curl 'http://localhost:8080/v1/admin/governance/raw-events?state=retry_wait&attempt_gte=1&limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

### Semantic readiness

Provider-backed deployment:

1. Confirm embedding runtime wiring through backlog inspection:

```bash
curl 'http://localhost:8080/v1/admin/embedding/rebuilds?limit=5' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

Expected runtime shape:

```json
{
  "runtime": {
    "configured": true,
    "semantic_rebuild_enabled": true,
    "registered_providers": ["openai"]
  },
  "items": []
}
```

2. Inspect one memory after governance promotion to verify rebuild state, active vector lineage, drift visibility, and failure context:

```bash
curl http://localhost:8080/v1/admin/memories/<memory-id>/embedding \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

Lexical-only deployment:

1. Run the same backlog inspection call.
2. Expect baseline startup to stay healthy while semantic wiring is reported as intentionally inactive:

```json
{
  "runtime": {
    "configured": false,
    "semantic_rebuild_enabled": false,
    "reason": "semantic rebuild execution is inactive because no embedding routes are configured"
  },
  "items": []
}
```

3. Treat `semantic_rebuild_enabled=false` plus the degraded reason as the success condition for a lexical-only deployment, not as an incident.

### Embedding provider cutover rollout

Use cutover plans when you need to migrate one scope toward a new embedding provider or model target without rewriting existing vector lineage.

1. Create a cutover plan with an explicit immutable target snapshot and bounded wave size:

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"target":{"provider":"openai","model":"text-embedding-3-large","dimensions":3072},"classes":["profile","episodic"],"wave_size":25,"reason":"migrate semantic target"}'
```

2. Preflight the draft plan before activation. The report is immediate and not persisted, so activation still reruns admission against current scope and runtime state:

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:preflight \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

Allowed reports have `decision:"allow"`. Warning-only reports still allow activation; for example `many_waves` means the scheduler will need multiple bounded dispatch waves. Denied reports have `decision:"deny"` and stable blocker codes such as `target_unresolved`, `unsupported_class_route`, `scoped_plan_conflict`, or `zero_eligible_memory`.

```json
{
  "component": "embedding_cutover",
  "decision": "deny",
  "blockers": [
    {
      "severity": "blocker",
      "code": "zero_eligible_memory"
    }
  ],
  "eligible_total": 0
}
```

3. Activate the plan only after preflight allows it and runtime inspection confirms the target provider is currently registered:

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:activate \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"start bounded rollout"}'
```

4. Monitor the rollout through both plan summary and item detail. `queued` shows unscheduled future work, `rebuilding` shows the currently dispatched wave, and `failed` highlights the hotspot set that needs operator attention:

```bash
curl 'http://localhost:8080/v1/admin/embedding/cutovers?status=active&limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl http://localhost:8080/v1/admin/embedding/cutovers/<plan-id> \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

5. Pause or cancel only when you want to stop future waves. Already rebuilding work keeps normal worker ownership. `pause` preserves unscheduled work for a later resume, while `cancel` leaves historical and already-dispatched work auditable but prevents the remaining queued set from advancing:

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:pause \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"hold next wave during incident review"}'

curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:cancel \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"abort rollout after provider regression"}'
```

6. During rollout incidents, inspect recovery audit at both scope and memory granularity. These queries let you confirm which retries or requeues happened inside the plan and who applied them:

```bash
curl 'http://localhost:8080/v1/admin/embedding/recovery-history?cutover_plan_id=<plan-id>&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl 'http://localhost:8080/v1/admin/memories/<memory-id>/embedding/recovery-history?cutover_plan_id=<plan-id>&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

7. Scrape runtime health and metrics while the rollout is active:

```bash
curl http://localhost:8080/livez
curl http://localhost:8080/readyz
curl http://localhost:8080/metrics | grep 'stele_embedding_'
```

`livez` only proves the process can respond. `readyz` checks PostgreSQL in `api` mode; `worker` and `scheduler` readiness also include embedding provider reachability when semantic rebuild or cutover execution is enabled. Provider network failures are not cutover activation blockers, but they appear as readiness degradation and `stele_embedding_provider_probe_total` metrics.

The metrics surface includes admission decisions and finding codes, active or paused cutover plan counts, cutover item status counts, rebuild backlog gauges, provider probe results, and scheduler wave dispatch counters. Labels are intentionally low-cardinality and do not include memory ids, raw event ids, cutover plan ids, or free-form error messages.

8. Rollback is modeled as a new forward cutover plan toward the prior provider, model, and dimensions. Do not mutate `vector_revisions` in place and do not try to reactivate an old revision as an ad hoc rollback shortcut.

### Memory quality and repair loop

Use this loop when ingestion still works but recall quality, semantic projection, governance backlog, or repair feasibility looks degraded. The loop stays service-side: it creates durable evaluation and repair records, then approved repair actions are picked up by worker execution rather than being performed inline by the admin request.

1. Check the admission metadata returned by event ingestion. A normal response includes `event_id`; degraded or queued responses also include `admission.decision` and stable finding codes such as `semantic_projection_degraded` or `governance_backlog_high`.

2. Create a scoped quality evaluation:

```bash
curl -X POST http://localhost:8080/v1/admin/memory-quality/evaluations \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"checks":["retrieval","context","admission_pressure","repair_pressure"],"actor":"operator-a","reason":"investigate degraded recall"}'
```

3. Inspect findings for the evaluation:

```bash
curl http://localhost:8080/v1/admin/memory-quality/evaluations/<evaluation-run-id>/findings \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

4. Create a repair plan from actionable findings. Use `dry_run:true` first when you only want to inspect the mapped actions.

```bash
curl -X POST http://localhost:8080/v1/admin/memory-quality/repair-plans \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"evaluation_run_id":"<evaluation-run-id>","actor":"operator-a","reason":"repair degraded projection and backlog","dry_run":true}'
```

5. Recreate or inspect the plan with `dry_run:false`, then approve it when the action list is safe:

```bash
curl -X POST http://localhost:8080/v1/admin/memory-quality/repair-plans/<repair-plan-id>:approve \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-a","reason":"approve bounded repair actions"}'
```

6. Monitor repair execution through the repair plan, job status, and metrics:

```bash
curl http://localhost:8080/v1/admin/memory-quality/repair-plans/<repair-plan-id> \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl http://localhost:8080/metrics | grep 'stele_quality_'
```

7. Run post-repair verification after executable actions finish:

```bash
curl -X POST http://localhost:8080/v1/admin/memory-quality/repair-plans/<repair-plan-id>:verify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"checks":["retrieval","admission_pressure","repair_pressure"],"actor":"operator-a","reason":"verify repair outcome"}'
```

Repair plans never rewrite canonical memory content, provenance, or version history in place. Unsupported findings become `manual_review` actions, and executable actions are constrained to existing governed paths such as embedding retry, governance requeue, and derived insight replay.

### External-agent memory feedback loop

Use this loop when Stele is integrated with an external agent runtime and you need to close the product feedback loop from recalled memory to quality remediation. Stele remains the memory service only: it does not invoke models, build prompts, run the external agent, or generate final answers.

1. Create a scoped memory session before the external agent turn:

```bash
curl -X POST http://localhost:8080/v1/memory-sessions \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"agent-a","reason":"serve external turn","metadata":{"integration":"external-agent"}}'
```

2. Create an idempotent turn to assemble memory context:

```bash
curl -X POST http://localhost:8080/v1/memory-sessions/<session-id>/turns \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"idempotency_key":"turn-1","query":"deployment preference","context_budget":1200,"include_relations":true,"include_experience_insights":true,"include_diagnostics":true,"include_feedback_diagnostics":true}'
```

3. After the external agent finishes its own turn, record outcome references or bounded outcome payloads through the existing ingestion path:

```bash
curl -X POST http://localhost:8080/v1/memory-sessions/<session-id>/turns/<turn-id>:outcome \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"idempotency_key":"outcome-1","outcome_event_ids":["evt_existing"],"event_payloads":[{"event_type":"agent_observation","content":"User prefers staged rollout","metadata":{"source":"external-agent"}}],"expected_recall":["evt_existing"]}'
```

4. Request recall verification. Multiple attempts are preserved as session verification history:

```bash
curl -X POST http://localhost:8080/v1/memory-sessions/<session-id>:verify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"turn_id":"<turn-id>","expected_recall":["evt_existing"]}'
```

5. Record usefulness feedback for returned memory, citations, the session, turn, verification, or missing expected recall. Known expected-recall targets use a typed `kind` plus `id`; opaque caller expectations use `kind:"opaque"` and `opaque_token` and are not treated as internal identifiers.

```bash
curl -X POST http://localhost:8080/v1/usefulness-feedback \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"type":"missing_expected","source_surface":"verification","subjects":[{"kind":"expected_recall","expected_recall_target":{"kind":"memory","id":"mem_expected"}}],"actor":"agent-a","reason":"expected memory was absent","idempotency_key":"feedback-expected-1"}'

curl -X POST http://localhost:8080/v1/usefulness-feedback \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"type":"noisy","source_surface":"context","subjects":[{"kind":"memory","id":"mem_noisy"}],"actor":"agent-a","reason":"retrieved but not useful","idempotency_key":"feedback-noisy-1"}'
```

6. Inspect active summaries and feedback history through admin routes. Public callers only see bounded summaries through their own authorized session reports.

```bash
curl 'http://localhost:8080/v1/admin/usefulness-feedback?subject_kind=memory&subject_id=mem_noisy&include_superseded=true' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl 'http://localhost:8080/v1/admin/usefulness-feedback/summary?subject_kind=memory&subject_id=mem_noisy' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

7. Correct bad feedback without deleting audit history:

```bash
curl -X POST http://localhost:8080/v1/admin/usefulness-feedback/<feedback-id>:supersede \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-a","reason":"feedback was attached to the wrong subject"}'
```

8. Convert repeated active feedback into quality findings, then create an approval-gated repair recommendation:

```bash
curl -X POST http://localhost:8080/v1/admin/memory-quality/evaluations \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"checks":["retrieval","context"],"actor":"operator-a","reason":"inspect active usefulness feedback"}'

curl -X POST http://localhost:8080/v1/admin/memory-quality/repair-plans \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"evaluation_run_id":"<evaluation-run-id>","actor":"operator-a","reason":"review feedback-derived findings","dry_run":true}'
```

Feedback-derived repair actions remain admin-gated. No public feedback request can approve a plan, suppress memory, retry embeddings, inspect governance, replay insights, or execute repair inline.

9. Rerun verification after operator remediation:

```bash
curl -X POST http://localhost:8080/v1/admin/memory-quality/repair-plans/<repair-plan-id>:verify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"checks":["retrieval","context"],"actor":"operator-a","reason":"verify feedback remediation"}'

curl http://localhost:8080/v1/memory-sessions/<session-id>/report \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

The session report exposes bounded feedback summaries, verification attempts, quality evaluation ids, quality finding ids and codes, repair plan ids, and next actions. It does not expose hidden memory content or out-of-scope evidence through public report fields.

### External-agent task success loop

Use this loop when an external agent integration needs to record task-level success evidence and inspect bounded reports. Stele stores caller-provided verdicts and evidence; it does not run the task or infer success.

1. Record a task evaluation after the external run completes:

```bash
curl -X POST http://localhost:8080/v1/task-evaluations \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"objective":"validate memory recall","success_criteria":["return only scoped memory"],"verdict":"partial","contribution_categories":["memory_missing","hidden_memory"],"evidence":[{"kind":"session","id":"session_1"},{"kind":"expected_recall","id":"mem_expected_1"},{"kind":"opaque","opaque_token":"caller-opaque-evidence"}],"actor":"agent-a","reason":"record external task outcome","idempotency_key":"task_eval_1"}'
```

2. Read the bounded task report to inspect linked evidence and next actions:

```bash
curl http://localhost:8080/v1/task-evaluations/<task-evaluation-id>/report \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

3. Inspect or correct task evaluations through the admin boundary:

```bash
curl 'http://localhost:8080/v1/admin/task-evaluations?verdict=partial&limit=25' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl http://localhost:8080/v1/admin/task-evaluations/<task-evaluation-id> \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl -X POST http://localhost:8080/v1/admin/task-evaluations/<task-evaluation-id>/supersede \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-a","reason":"corrected verdict"}'

curl 'http://localhost:8080/v1/admin/task-evaluations/summary?evidence_target_kind=session&evidence_target_id=session_1' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

Task reports expose only bounded linked ids, evidence categories, and next actions. Opaque caller evidence tokens are stored for audit purposes but are not treated as internal Stele identifiers.

### Remaining feedback-loop product gaps

This repository now closes the service-side task-success and governed ranking rollout loop: external task evaluations can feed scoped summaries, ranking policies can be dry-run, activated, inspected, disabled, and rolled back, and default search/context ranking changes only through a governed ranking rollout policy. Several product surfaces remain intentionally outside this proposal:

- SDK/UI collection surfaces for callers to attach sessions, feedback subjects, task evidence, and rollout inspection without hand-building JSON.
- External agent runtime integration that decides when to call session, outcome, verification, task evaluation, feedback, and ranking rollout routes.
- Hosted incident management and vendor-specific alert integrations beyond the generic self-hosted webhook adapter.
- advanced scoring calibration, including adaptive weights, traffic splitting, long-term memory usefulness scoring, and offline evaluation harnesses beyond the conservative bounded signals implemented here.
- End-user prompt orchestration and model invocation, which remain outside Stele's service boundary.

### Remaining assurance product gaps

The assurance and conformance loop is service-owned and self-host friendly, but it deliberately stops at durable diagnostics, admin inspection, generic alert delivery, and recovery verification. Remaining product gaps are outside this repository boundary:

- SDK/UI collection surfaces that guide callers through session, context, outcome, verification, usefulness feedback, task evaluation, proof, repair, and ranking rollout evidence capture.
- external agent runtime implementation, including agent execution, tool orchestration, and deciding when to write each evidence record.
- vendor-specific alert integrations for Slack, PagerDuty, email providers, ticketing systems, or hosted incident-management products.
- hosted incident management workflows such as escalation policies, on-call schedules, paging, acknowledgement ownership, and post-incident review tooling.
- adaptive scoring calibration, including online learning, traffic splitting, long-term memory usefulness scoring, and offline evaluation harnesses.
- model invocation, prompt orchestration, and final-answer generation, which remain outside Stele's service boundary.

## Memory Management Surface

`Stele` now exposes two distinct memory management boundaries:

- public read surface, authenticated by a durable runtime principal credential
- privileged lifecycle surface, authenticated by a durable admin principal credential

Public read routes:

- `GET /v1/memories`
- `GET /v1/memories/{memory_id}`
- `GET /v1/memories/{memory_id}/history`
- `GET /v1/memories/{memory_id}/provenance`

Privileged lifecycle routes:

- `POST /v1/admin/memories`
- `PATCH /v1/admin/memories/{memory_id}`
- `POST /v1/admin/memories/{memory_id}:merge`
- `POST /v1/admin/memories/{memory_id}:reclassify`
- `POST /v1/admin/memories/{memory_id}:suppress`
- `POST /v1/admin/memories/{memory_id}:expire`
- `POST /v1/admin/memories/{memory_id}:delete`

Privileged governance recovery routes:

- `GET /v1/admin/governance/raw-events`
- `GET /v1/admin/governance/raw-events/{raw_event_id}`
- `GET /v1/admin/governance/raw-events/{raw_event_id}/recovery-history`
- `POST /v1/admin/governance/raw-events/{raw_event_id}:retry`
- `POST /v1/admin/governance/raw-events/{raw_event_id}:reschedule`
- `POST /v1/admin/governance/raw-events/{raw_event_id}:requeue`

Privileged embedding inspection and recovery routes:

- `GET /v1/admin/embedding/rebuilds`
- `GET /v1/admin/embedding/cutovers`
- `POST /v1/admin/embedding/cutovers`
- `GET /v1/admin/embedding/cutovers/{cutover_plan_id}`
- `POST /v1/admin/embedding/cutovers/{cutover_plan_id}:activate`
- `POST /v1/admin/embedding/cutovers/{cutover_plan_id}:pause`
- `POST /v1/admin/embedding/cutovers/{cutover_plan_id}:cancel`
- `GET /v1/admin/embedding/recovery-history`
- `GET /v1/admin/memories/{memory_id}/embedding`
- `GET /v1/admin/memories/{memory_id}/embedding/recovery-history`
- `POST /v1/admin/embedding/rebuilds/{memory_id}:retry`
- `POST /v1/admin/embedding/rebuilds/{memory_id}:requeue`

Privileged derived insight inspection routes:

- `GET /v1/admin/derived-insights`
- `GET /v1/admin/derived-insights/{insight_id}`
- `GET /v1/admin/derived-insights/{insight_id}/feedback`
- `POST /v1/admin/derived-insights/{insight_id}/feedback`
- `POST /v1/admin/derived-insight-feedback/{feedback_id}:supersede`
- `POST /v1/admin/derived-insights/{insight_id}:suppress`
- `POST /v1/admin/derived-insight-replays:dry-run`
- `GET /v1/admin/derived-insight-replays`
- `POST /v1/admin/derived-insight-replays`
- `GET /v1/admin/derived-insight-replays/{replay_run_id}`
- `GET /v1/admin/derived-insight-replays/{replay_run_id}/report`

Privileged memory quality and repair routes:

- `POST /v1/admin/memory-quality/evaluations`
- `GET /v1/admin/memory-quality/evaluations/{evaluation_run_id}`
- `GET /v1/admin/memory-quality/evaluations/{evaluation_run_id}/findings`
- `POST /v1/admin/memory-quality/repair-plans`
- `GET /v1/admin/memory-quality/repair-plans/{repair_plan_id}`
- `POST /v1/admin/memory-quality/repair-plans/{repair_plan_id}:approve`
- `POST /v1/admin/memory-quality/repair-plans/{repair_plan_id}:verify`

Task success inspection routes:

- `POST /v1/task-evaluations`
- `GET /v1/task-evaluations/{evaluation_id}/report`
- `GET /v1/admin/task-evaluations`
- `GET /v1/admin/task-evaluations/{evaluation_id}`
- `GET /v1/admin/task-evaluations/summary`
- `POST /v1/admin/task-evaluations/{evaluation_id}/supersede`

Privileged manual mutation and lifecycle actions require:

- admin API key
- scoped headers
- `X-Stele-Actor`
- JSON body with `reason`

Privileged governance recovery actions use the same admin boundary and additionally require:

- `X-Stele-Actor` for every recovery action
- `reason` in the JSON body for every recovery action
- `scheduled_for` only for `:reschedule`

Privileged embedding recovery actions use the same admin boundary and require:

- `X-Stele-Actor` for every recovery action
- `reason` in the JSON body for every recovery action

Privileged derived insight suppression uses the same admin boundary and requires:

- `actor` in the JSON body
- `reason` in the JSON body

Privileged derived insight feedback uses the same admin boundary and requires:

- `actor` in the JSON body
- `reason` in the JSON body
- bounded `type` values: `useful`, `noisy`, `incorrect`, `stale`, `redundant`, or `needs_review`

Privileged memory quality and repair actions use the same admin boundary and require:

- scoped headers for every evaluation and repair plan
- `actor` in the JSON body for evaluation, repair plan creation, and approval
- `reason` in the JSON body for repair plan creation and approval
- bounded evaluation checks: `retrieval`, `context`, `admission_pressure`, or `repair_pressure`
- bounded repair actions generated from findings: `embedding_retry`, `governance_requeue`, `derived_insight_replay`, or `manual_review`

Governance recovery query filters:

- `state`
- `event_type`
- `attempt_gte`
- `attempt_lte`
- `failed_from`
- `failed_to`
- `next_attempt_from`
- `next_attempt_to`
- `limit`
- `cursor`

Manual mutation notes:

- `POST /v1/admin/memories` only supports operator-authored primary classes; derived `summary` memory is excluded.
- `PATCH /v1/admin/memories/{memory_id}` only updates canonical content. Scope, lifecycle, and class stay on dedicated surfaces.
- merge and reclassify require `expected_version` optimistic concurrency.
- `reclassify` only targets `profile`, `episodic`, or `procedural`; `relation` and `summary` stay excluded in this phase.
- material manual mutation clears stale semantic participation immediately and records durable rebuild eligibility for later background regeneration.

Governance recovery notes:

- Admin governance inspection returns derived raw event states: `pending`, `retry_wait`, `leased`, `exhausted`, and `processed`.
- `retry` only targets `retry_wait` items and pulls `next_attempt_at` forward to immediate worker eligibility.
- `reschedule` only targets `pending` or `retry_wait` items and moves `next_attempt_at` to the operator-provided future timestamp.
- `requeue` only targets `exhausted` items, clears `governance_exhausted_at`, resets the automatic attempt counter, and returns the event to the ordinary worker claim path.
- `leased` and `processed` items are rejected rather than being force-taken over.
- Every recovery action is written to the append-only `governance_recovery_ledger` with actor, reason, and before or after state snapshots.
- Recovery never triggers direct execution. The existing governance worker picks the event up on a later poll through the normal durable claim path.
- First phase boundaries stay narrow: single-item recovery only, no bulk remediation, no leased takeover, and no `ignore` or `drop` terminal action.

## Governed Experience Insights

- Derived insights are separate governed records, not canonical memories. They do not rewrite canonical memory rows, memory versions, vector revisions, or provenance in place.
- The first active insight type is `failure_pattern`. The scheduler derives it from repeated scoped evidence such as failed job executions, failed embedding rebuild records, raw event governance errors, recovery history, and failure-related canonical memory content.
- `lesson` insights are deterministic projections from an existing `failure_pattern`. A lesson must cite evidence and reference its source failure pattern.
- `hypothesis`, `goal`, `contradiction`, and `causal_link` are reserved vocabulary only in this phase. Stele does not autonomously infer or activate those types.
- Derivation is asynchronous. Ingest, manual mutation, retrieval, and context assembly requests do not run failure pattern derivation in the foreground.
- Default context assembly excludes derived insights. Callers must set `include_experience_insights=true` on `POST /v1/context/assemble` to request `known_failures` and `experience_lessons` sections.
- Authenticated callers can set `include_diagnostics=true` on `POST /v1/context/assemble` to receive category/count diagnostics for included, budget-omitted, quality-omitted, and lifecycle-hidden experience insight sections.
- Suppressed, forgotten, deleted, and out-of-scope insights are excluded from context assembly. Admin inspection can still read hidden insight state, evidence, and lifecycle history with `include_hidden=true`.
- `POST /v1/admin/derived-insights/{insight_id}:suppress` records an audited lifecycle transition and preserves linked evidence history.
- Operators can record quality feedback for derived insights without rewriting the insight body or deleting evidence. Active feedback summaries are included in admin insight detail responses and are used by scheduler derivation and optional context assembly ranking.
- Feedback-driven policy can suppress noisy or incorrect insights through an audited lifecycle transition. A single feedback record is not a destructive delete and can be superseded while remaining visible in feedback history.
- Feedback metrics are exposed through `/metrics` as low-cardinality `stele_insight_feedback_total` series. Labels intentionally exclude tenant, project, namespace, insight id, actor, and reason text.
- Replay metrics are exposed through `/metrics` as low-cardinality `stele_derived_insight_replay_total` series. Labels intentionally exclude tenant, project, namespace, replay run id, insight id, actor, reason text, and raw error messages.
- This feature does not add MCP tools, a reasoning-provider abstraction, SDK behavior, UI behavior, global agent-self namespaces, or autonomous causal or goal inference.

Derived insight inspection example:

```bash
curl 'http://localhost:8080/v1/admin/derived-insights?type=failure_pattern&state=active&min_evidence_count=2&limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

Context assembly opt-in example:

```bash
curl -X POST http://localhost:8080/v1/context/assemble \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"query":"prepare the next embedding rebuild run","budget":6,"include_experience_insights":true,"include_diagnostics":true}'
```

Derived insight suppression example:

```bash
curl -X POST http://localhost:8080/v1/admin/derived-insights/<insight-id>:suppress \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-a","reason":"noisy duplicate"}'
```

Derived insight feedback example:

```bash
curl -X POST http://localhost:8080/v1/admin/derived-insights/<insight-id>/feedback \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"type":"noisy","actor":"operator-a","reason":"too broad for context assembly","quality_score":0.2}'
```

Derived insight feedback history example:

```bash
curl 'http://localhost:8080/v1/admin/derived-insights/<insight-id>/feedback?include_superseded=true&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

Derived insight feedback supersession example:

```bash
curl -X POST http://localhost:8080/v1/admin/derived-insight-feedback/<feedback-id>:supersede \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-b","reason":"replaced by more accurate review"}'
```

## Embedding Lifecycle

- Semantic retrieval reads only the active vector revision linked to the current canonical projection. Superseded, failed, stale, or missing revisions are excluded from default semantic recall.
- When a canonical memory has no active semantic projection, hybrid retrieval still completes through lexical and relation recall. The request does not fail just because semantic projection is unavailable.
- Manual create, update, merge, and reclassify paths invalidate stale active semantic participation synchronously, then rely on scheduler-driven rebuild execution to restore semantic recall later.
- The scheduler discovers missing embeddings and provider or model drift through durable `embedding_rebuilds` state, then claims rebuild work without blocking write paths.
- Every generation attempt is audited in append-only `vector_revisions`, including failed attempts and superseded lineage. Activation uses compare-and-promote guards so stale content never becomes the active semantic projection.
- Routing metadata comes from `STELE_EMBEDDING_DEFAULT_*` and optional `STELE_EMBEDDING_CLASS_ROUTES` overrides. These settings define the desired target used for eligibility, drift detection, and audit history.
- Provider implementations remain an internal runtime integration point. The built-in runtime currently wires an OpenAI-compatible adapter from environment configuration.
- If no embedding routes are configured, the service stays in lexical-only mode and surfaces semantic inactivity through admin embedding diagnostics.
- If an embedding route is configured but the matching provider cannot be registered, startup fails instead of silently degrading.
- Admin embedding inspection exposes rebuild backlog, one-memory vector lineage, provider or model drift, and failed attempt context without requiring direct database access.
- Provider cutovers stay durable as operator-owned plans with immutable target snapshots, scoped membership, bounded wave size, and append-only audit context.
- The scheduler advances active cutover plans in bounded waves through the ordinary `embedding_rebuilds` path before claiming work for execution.
- Pausing a cutover stops future waves while leaving the already dispatched or rebuilding wave under ordinary worker ownership.
- Cancelling a cutover prevents unscheduled remaining items from advancing but does not erase already completed, rebuilding, or failed rollout history.
- `POST /v1/admin/embedding/rebuilds/{memory_id}:retry` only restores failed rebuild work to `pending`.
- `POST /v1/admin/embedding/rebuilds/{memory_id}:requeue` returns eligible current or failed rebuild work to `pending` so the ordinary background rebuild job can pick it up again.
- Rebuild records already in `rebuilding` state are rejected rather than being force-taken over.
- Every embedding recovery action is written to append-only `embedding_recovery_ledger` with actor, reason, and before or after rebuild snapshots.
- Recovery history can be filtered by `cutover_plan_id`, actor, action, and time window so rollout incidents remain explainable without direct database access.
- Embedding recovery never mutates `vector_revisions` directly. Background execution still owns append, compare, and promote behavior.
- Reverse cutover is the only supported rollback model. Create a new plan toward the earlier target instead of rewriting vector history in place.

## Example Flow

1. List visible canonical memory:

```bash
curl 'http://localhost:8080/v1/memories?class=profile&limit=10' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

2. Inspect one memory history:

```bash
curl http://localhost:8080/v1/memories/<memory-id>/history \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

3. Inspect provenance lineage:

```bash
curl http://localhost:8080/v1/memories/<memory-id>/provenance \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

4. Suppress a memory through the admin boundary:

```bash
curl -X POST http://localhost:8080/v1/admin/memories/<memory-id>:suppress \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"manual override"}'
```

5. Seed one canonical memory directly through the admin boundary:

```bash
curl -X POST http://localhost:8080/v1/admin/memories \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"class":"profile","content":"user prefers concise answers","reason":"seed curated memory"}'
```

6. Correct the memory content with optimistic concurrency:

```bash
curl -X PATCH http://localhost:8080/v1/admin/memories/<memory-id> \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"content":"user strongly prefers concise answers","expected_version":1,"reason":"correct curated fact"}'
```

7. Merge one duplicate memory into the surviving target:

```bash
curl -X POST http://localhost:8080/v1/admin/memories/<target-memory-id>:merge \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"source_memory_id":"<source-memory-id>","content":"user strongly prefers concise answers","expected_version":2,"reason":"merge duplicate canonical facts"}'
```

8. Reclassify a memory into the right governed class:

```bash
curl -X POST http://localhost:8080/v1/admin/memories/<memory-id>:reclassify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"target_class":"procedural","expected_version":3,"reason":"fix canonical class"}'
```

9. List governance raw events that are waiting for retry:

```bash
curl 'http://localhost:8080/v1/admin/governance/raw-events?state=retry_wait&attempt_gte=1&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

10. Read one raw event detail before remediation:

```bash
curl http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id> \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

11. Retry one waiting raw event immediately:

```bash
curl -X POST http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id>:retry \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"retry now after transient dependency recovery"}'
```

12. Reschedule one raw event into a later maintenance window:

```bash
curl -X POST http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id>:reschedule \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"delay until downstream system is stable","scheduled_for":"2026-06-13T03:00:00Z"}'
```

13. Requeue one exhausted raw event back into normal worker polling:

```bash
curl -X POST http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id>:requeue \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"clear exhausted state after operator review"}'
```

14. Read recovery audit history for one raw event:

```bash
curl http://localhost:8080/v1/admin/governance/raw-events/<raw-event-id>/recovery-history \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

15. Inspect embedding rebuild backlog for one scope:

```bash
curl 'http://localhost:8080/v1/admin/embedding/rebuilds?status=failed&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

16. Inspect one memory's embedding lineage:

```bash
curl http://localhost:8080/v1/admin/memories/<memory-id>/embedding \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

17. Retry one failed embedding rebuild:

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/rebuilds/<memory-id>:retry \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"retry after provider recovery"}'
```

18. Requeue one current or previously failed embedding rebuild back into ordinary scheduler ownership:

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/rebuilds/<memory-id>:requeue \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"requeue after routing update"}'
```

19. List recent and active embedding cutover plans:

```bash
curl 'http://localhost:8080/v1/admin/embedding/cutovers?limit=10' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

20. Read one embedding cutover plan in detail:

```bash
curl http://localhost:8080/v1/admin/embedding/cutovers/<plan-id> \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

21. Create one embedding cutover plan:

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"target":{"provider":"openai","model":"text-embedding-3-large","dimensions":3072},"classes":["profile"],"wave_size":10,"reason":"test provider cutover"}'
```

22. Preflight one cutover plan:

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:preflight \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

23. Activate or pause one cutover plan:

```bash
curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:activate \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"start rollout"}'

curl -X POST http://localhost:8080/v1/admin/embedding/cutovers/<plan-id>:pause \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Actor: operator-a' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"reason":"stop next wave"}'
```

24. Inspect scope-level embedding recovery history during a rollout incident:

```bash
curl 'http://localhost:8080/v1/admin/embedding/recovery-history?cutover_plan_id=<plan-id>&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

25. Inspect one memory's embedding recovery history during the same incident:

```bash
curl 'http://localhost:8080/v1/admin/memories/<memory-id>/embedding/recovery-history?cutover_plan_id=<plan-id>&limit=20' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

## Integration evidence workflow golden path

The workflow contract records whether an external integration has captured the service-side evidence needed for one agent turn, task, or job. It does not execute the external agent or mutate the linked source evidence.

1. Create an admin-owned template for the scope. Template steps use bounded kinds and evidence categories:

```bash
curl -X POST http://localhost:8080/v1/admin/workflows/templates \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"integration_kind":"agent_turn","completion_policy":"strict","actor":"operator-a","reason":"golden integration path","steps":[{"kind":"session_started","requirement":"required","allowed_evidence":["session"],"minimum_count":1,"requires_internal":true,"freshness_window":"24h","completion_window":"1h","position":1},{"kind":"turn_outcome_recorded","requirement":"required","allowed_evidence":["outcome"],"minimum_count":1,"requires_internal":true,"freshness_window":"24h","completion_window":"1h","position":2}]}'
```

2. An external integration starts its scoped workflow run. Repeating the same idempotency key resumes the original run:

```bash
curl -X POST http://localhost:8080/v1/workflows/runs \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"template_id":"<workflow-template-id>","idempotency_key":"external-turn-001","actor":"agent-integration","reason":"capture evidence for one turn"}'
```

3. After writing the normal memory-session/context/outcome/verification/feedback/task evidence through their own routes, attach the relevant scoped record as a workflow step:

```bash
curl -X POST http://localhost:8080/v1/workflows/runs/<workflow-run-id>/steps \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"kind":"session_started","actor":"agent-integration","reason":"session evidence recorded","evidence_links":[{"kind":"session","source":"public_api","target_id":"<memory-session-id>"}]}'
```

4. Read bounded public guidance for the next missing step. This response intentionally excludes workflow IDs, scope IDs, hidden evidence, prompts, and model output:

```bash
curl http://localhost:8080/v1/workflows/runs/<workflow-run-id>/next-actions \
  -H 'X-API-Key: <runtime-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'
```

5. Administrators inspect diagnosed gaps and may supersede a bad evidence link without changing the source record:

```bash
curl http://localhost:8080/v1/admin/workflows/runs/<workflow-run-id>/diagnostics \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a'

curl -X POST http://localhost:8080/v1/admin/workflows/evidence-links/<evidence-link-id>/supersede \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <admin-credential>' \
  -H 'X-Stele-Tenant: tenant-a' \
  -H 'X-Stele-Project: project-a' \
  -H 'X-Stele-Namespace: namespace-a' \
  -d '{"actor":"operator-a","reason":"replace invalid evidence link"}'
```

6. Run the existing conformance and readiness routes for the same scope, then create a recovery verification when a workflow-related incident has been remediated. Workflow evidence participates in assurance as bounded categories; it never resolves incidents automatically.

Set `STELE_WORKFLOW_MAINTENANCE_ENABLED=true` only when scheduler-driven stale diagnostics and high-volume workflow history cleanup are desired. It is disabled by default. `STELE_WORKFLOW_DIAGNOSTIC_INTERVAL`, `STELE_WORKFLOW_STALE_RUN_WINDOW`, `STELE_WORKFLOW_DIAGNOSTIC_SCAN_LIMIT`, `STELE_WORKFLOW_NEXT_ACTION_REFRESH_LIMIT`, and `STELE_WORKFLOW_HISTORY_RETENTION` bound that maintenance path. Monitor `stele_workflow_lifecycle_total` for template/run/step/evidence/diagnostic/next-action/cleanup categories.

Stele does not provide SDK/UI surfaces, external agent execution, model invocation, prompt orchestration, tool orchestration, or final-answer generation. Those remain with the external integration; Stele owns the scoped memory and evidence records only.

## Operational Notes

## Retrieval Quality Evaluation

Stele keeps a repository-owned retrieval fixture at
`internal/retrieval/testdata/retrieval-evaluation-fixture-v1.json`. It covers
single-fact, multi-hop, temporal, memory-class, contradiction, noise, duplicate,
lifecycle, and scope-isolation cases. Fixture sources are test-only and are never
production data or generated answers.

Run deterministic unit coverage from the repository root:

```powershell
go test ./internal/retrieval ./internal/telemetry -count=1
```

The real PostgreSQL replay is intentionally opt-in and requires a disposable,
harness-owned DSN in `STELE_TEST_RETRIEVAL_EVALUATION_DSN`:

```powershell
$env:STELE_TEST_RETRIEVAL_EVALUATION_DSN = '<owned-test-dsn>'
pwsh -File scripts/retrieval-evaluation.ps1
```

Without that variable the command prints
`SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED` and exits with code `2`; it never falls
back to `STELE_POSTGRES_DSN` or another ambient database. The seeder uses the
reserved `eval` tenant and `retrieval-baseline` project, writes through normal
raw-event/candidate/canonical/provenance/lifecycle paths, and tags records with
the fixture version for ownership. Use a disposable database and remove its
fixture scope after replay; never point this command at an operator or production
database.

Reports record `fixture_version`, `representation_version`, `ranking_version`,
compatible embedding revision, policy version, evidence-group metrics, candidate
pool size, and bounded latency. Safety failures for cross-scope or hidden lifecycle
results override all quality scores. Threshold edits require a new policy version;
baseline and candidate reports must use compatible fixture and representation
versions before comparison.

- `api` logs request completion and panic recovery in structured key-value style.
- `GET /livez`, `GET /readyz`, and `GET /metrics` provide process liveness, mode-aware readiness, and Prometheus-style runtime metrics for self-hosted orchestration.
- `worker` logs polling loop failures and successful batch execution.
- `worker` also logs scope proof step transitions and memory session verification transitions in structured key-value style.
- `scheduler` logs maintenance job execution results and backoff retries.
- The worker persists retryable governance failures with bounded retry state instead of relying only on lease expiry.
- Raw events that hit the retry ceiling are marked exhausted and stop automatic claim until an explicit admin recovery action intervenes.
- The scheduler dispatch path is independent from public request traffic.
- The embedding rebuild scheduler records backlog, provider probe, cutover wave dispatch, execution telemetry, and error paths through the shared observer hook.
- Active embedding cutovers are advanced by the scheduler in bounded waves before the normal lifecycle discovery pass, so provider migrations reuse the same durable rebuild execution path as drift correction.
- Summary compaction and retention sweep are dispatched per eligible discovered scope, with the configured default scope used only as a fallback when discovery returns none.
- Job execution cleanup remains runtime-global and runs once per cadence window instead of being fanned out per discovered scope.
- Telemetry hook points are wired for ingest, governance worker execution, retrieval, forgetting, governance backlog inspection, derived insight feedback, embedding admission decisions, cutover state snapshots, provider readiness probes, scheduler wave dispatch, embedding rebuild backlog plus execution inspection, scope proof runs and steps, memory session runs and turns, and memory session verification outcomes. The default runtime installs a Prometheus-style in-process metrics observer for the API metrics endpoint.
