## Context

Stele already implements the memory-domain loop and its service-side evidence
contracts. It supports `api`, `worker`, and `scheduler` modes, PostgreSQL with
pgvector, principal-backed scope authorization, idempotent ingestion, async
governance, retrieval, context assembly, and self-hosted assurance. The
remaining gap is product delivery: a fresh user cannot rely on the documented
Compose invocation, operators cannot safely evolve an existing schema, and the
repository has no black-box proof that its three runtimes survive the lifecycle
an external Agent Runtime will depend on.

The supported baseline introduced by this design is a single-node, self-hosted
deployment. Compose with bundled PostgreSQL is a local-evaluation profile;
production uses the same images with an operator-managed PostgreSQL DSN and
operator-provided secrets. PostgreSQL remains the only system of record. The
future Agent Runtime remains external and uses the published HTTP/OpenAPI
contract; Stele does not import or execute it.

## Goals / Non-Goals

**Goals:**

- Deliver a repeatable fresh-install path from container startup through
  bootstrap, durable credentials, scoped event ingest, worker processing, and
  retrieval/context verification.
- Make database evolution explicit, observable, serialized, restart-safe, and
  testable from a prior populated schema state.
- Bound HTTP resource use and make all runtime modes react predictably to
  process cancellation and container termination.
- Produce executable evidence that the supported stack works against real
  PostgreSQL and pgvector, including failure/restart/recovery boundaries.
- Give external consumers a runtime-published API document and compatibility
  facts sufficient to pin the service version they integrate with.
- Give operators safe, explicit backup/restore procedures and a way to turn a
  successful restore drill into existing assurance proof.

**Non-Goals:**

- No agent-runtime adapter, SDK, UI, cloud-managed deployment, or Kubernetes
  controller.
- No implicit migration downgrade, automatic data deletion, in-place destructive
  recovery, backup scheduling, or object-storage integration.
- No external dependency beyond PostgreSQL/pgvector and a selected mature Go
  migration library where it materially reduces migration correctness risk.
- No weakening of exact tenant/project/namespace grants, credential handling,
  canonical-memory versioning, or lifecycle visibility rules.

## Decisions

### Decision 1: Use an immutable, versioned migration ledger

Adopt a mature Go PostgreSQL migration library after a targeted `pkg.go.dev`
evaluation, with numbered SQL migrations embedded or shipped in the service
image. A database-owned migration metadata table records the applied version
and dirty state. Migration execution holds a PostgreSQL advisory lock, applies
only ordered pending migrations transactionally where the database permits, and
returns a clear non-zero failure for dirty, divergent, or incompatible state.

The current bootstrap schema becomes the immutable initial schema migration.
No runtime replays a mutable catch-all schema file. Runtime configuration offers
an explicit policy: `auto` applies pending forward migrations before the mode
starts; `validate` requires the database to be current; `off` is allowed only
for a separately documented, externally-managed migration workflow. A
standalone `stele migrate` command reports status and performs the same locked
forward-only operation.

Alternatives considered:

- Keep `CREATE ... IF NOT EXISTS` bootstrap replay: rejected because it cannot
  express upgrades, backfills, constraints, or the current version.
- Require operators to run raw SQL manually: rejected because it loses a
  discoverable ledger, serialization, dirty-state diagnostics, and testable
  upgrade behavior.
- Allow automatic down migrations: rejected because data-loss risk is not a
  safe default for a memory system; rollback is application/image rollback plus
  an operator-planned forward remediation migration.

### Decision 2: One canonical bootstrap-admin-first deployment path

The bundled Compose profile uses `STELE_AUTH_BOOTSTRAP_ADMIN_KEY` and an
explicit default scope, never the rejected static allow-list settings. The
bootstrap key only creates the first durable administrator for that exact scope;
the issued credential is returned once and is used to create a least-privilege
runtime principal and grant. Smoke fixtures use that public principal and
required idempotency key. Documentation distinguishes local generated/example
values from production-required secret injection and never presents an
insecure/default credential as a production deployment.

Compose receives a named local profile plus an example environment file that
contains variable names and non-secret placeholders. Production guidance uses
an external DSN and environment/secret-file injection; the service remains
agnostic to the secret manager. All three modes use the same migration policy,
scope defaults, and credential model.

Alternatives considered:

- Re-enable static API key allow lists for Compose: rejected because it creates
  a second authorization model and bypasses durable principal audit.
- Build an interactive installer: rejected because Stele is a service and its
  public API already supplies bootstrap and principal lifecycle operations.

### Decision 3: Treat real-stack verification as a first-class product gate

Add a hermetic, CI-runnable verification harness using a real PostgreSQL image
with pgvector and the built service/Compose configuration. It creates isolated
test scopes and credentials through the documented API, proves exact-grant
denial, sends a retried idempotent event, observes durable worker completion,
checks retrieval and context assembly, restarts a runtime, and proves the same
scope remains usable without duplicate ingestion. It also runs a migration
fixture from a previous schema/data state and a backup/restore fixture in a
disposable database.

The harness has bounded timeouts, generated test secrets, unique scope values,
and cleanup limited to its own labelled containers, volumes, and databases. It
must not use the developer's Docker volume or production DSN. A concise smoke
script can be run by operators; a longer integration suite is suitable for CI.

Alternatives considered:

- Depend only on pgxmock and handler tests: rejected because it cannot prove
  extensions, migration ordering, container wiring, process lifecycle, or real
  transaction behavior.
- Use an external Agent Runtime as the test client: rejected because that would
  couple the service release gate to a separate product before its adapter
  exists.

### Decision 4: Make shutdown and request budgets runtime-owned

Configuration gains positive, bounded settings for read-header, request read,
write, idle, and shutdown timeouts; maximum header bytes; and maximum JSON
request bytes. API middleware applies `http.MaxBytesReader` before decoding
untrusted request bodies and maps oversized/malformed input to bounded client
errors. Server construction sets all relevant `http.Server` limits.

`cmd/stele` starts the runner from a signal-aware context for `SIGINT` and
`SIGTERM`. API mode stops accepting new work, performs `Server.Shutdown` using
a dedicated bounded drain context, then closes pool/provider resources. Worker
and scheduler loops observe cancellation, stop claiming new jobs, release or
leave durable claims according to their existing retry semantics, and close
dependencies. Cleanup executes exactly once on normal termination, startup
failure after allocation, and cancellation. Liveness may stay successful while
draining, while readiness becomes non-ready before traffic handoff.

Alternatives considered:

- Leave timeouts to a reverse proxy: rejected because direct self-hosted use
  needs a safe service baseline; proxies can still impose tighter limits.
- Force immediate process exit on a signal: rejected because it risks aborted
  HTTP requests and unclear job ownership during rolling deployment.

### Decision 5: Publish an immutable runtime contract and compatibility facts

API mode serves the same generated/embedded OpenAPI source used by repository
validation at a stable unauthenticated discovery endpoint, with content type,
cache headers, and an ETag or digest. A separate bounded version endpoint
returns service version, commit/build timestamp when supplied at build time,
OpenAPI digest/version, migration compatibility range, and current schema
version. It returns no DSN, secret, scope, principal, migration SQL, or
operational details.

The future Agent Runtime pins an accepted OpenAPI digest/version and verifies
compatibility during startup or CI. This change does not create that consumer;
it makes the provider contract discoverable and testable.

Alternatives considered:

- Expose raw source repository paths only: rejected because self-hosted
  deployments can run a different image than the checkout used by an integrator.
- Add a provider-specific protocol now: rejected because HTTP/OpenAPI remains
  Stele's declared public boundary and avoids premature adapter coupling.

### Decision 6: Recovery is operator-executed, service-evidenced

Provide versioned PowerShell runbooks/scripts for PostgreSQL backup, restore to
an explicit target, and restore verification. Scripts require an explicit DSN
or container target, refuse broad/default restore targets, separate source and
target checks, bind password input through supported PostgreSQL environment
mechanisms without echoing it, and report command prerequisites. Backup emits a
manifest with service/OpenAPI/schema information and checksum; restore
verification validates that migrations are current and runs a bounded read-only
scope proof after operators provision valid credentials.

The scripts do not schedule backups, upload archives, or overwrite the source.
After an operator completes a restore drill, an authenticated admin can submit
or record the resulting bounded verification metadata through the existing
assurance proof surface. This connects practical recovery evidence to readiness
without moving backup infrastructure into Stele.

Alternatives considered:

- Implement backup in `scheduler`: rejected because it would require ownership
  of storage, credentials, retention, and disaster recovery policy.
- Limit documentation to `pg_dump` prose: rejected because an executable
  constrained command is less ambiguous and can be continuously tested.

### Decision 7: Documentation and tests form a single release contract

The canonical quick start, Compose configuration, sample environment, OpenAPI
examples, and smoke scripts share named variables and route sequences. A
documentation contract test rejects obsolete authentication keys and verifies
that referenced commands, public endpoints, environment variables, and
bootstrap sequence remain valid. The release/CI gate runs unit tests, OpenSpec
validation, migration/upgrade tests, static docs/contract checks, and the
real-stack product verification suite when its container prerequisite is
available; a clear skip is permitted only for local environments without Docker,
never for the release CI job.

## Risks / Trade-offs

- [Migration library behavior and existing database compatibility differ from
  current bootstrap] -> preserve a fixture made from the released base schema,
  test forward upgrade and dirty state, take an operator backup before the
  release migration, and document no automatic downgrade.
- [Auto-migration across three modes races at rollout] -> advisory locking and
  a migration ledger serialize execution; `validate` policy supports operators
  who migrate before scaling application modes.
- [Container tests can be slow or unavailable locally] -> maintain a fast unit
  tier and a clearly labelled mandatory CI integration tier with bounded
  timeouts and diagnostics.
- [Request caps reject legitimate large memory events] -> use conservative
  documented defaults, configurable upper bounds, and return a stable error
  that directs callers to split/chunk input rather than silently truncating.
- [Graceful shutdown can exceed platform grace period] -> make drain timeout
  explicit, bound it below the documented container grace period, and preserve
  durable job retry semantics for unfinished work.
- [Backup scripts could damage data] -> require explicit non-empty target,
  block source=target restore, default to disposable verification targets, and
  never remove or overwrite a database implicitly.
- [Runtime discovery becomes a fingerprinting surface] -> publish only public
  compatibility metadata and keep all operational and scoped data authenticated.

## Migration Plan

1. Add the migration runner, ledger, lock, command, policy validation, immutable
   initial migration, and a fixture representing the pre-change database.
2. Ship the runtime resource limits and signal-aware lifecycle with default-safe
   values and documented configuration overrides; retain existing protected API
   route semantics.
3. Update Compose/environment examples and documentation to the bootstrap-admin
   procedure; add runtime OpenAPI/version endpoints.
4. Add backup/restore/verification scripts and bridge successful restore
   verification to assurance evidence.
5. Add unit, real-PostgreSQL, Compose, migration-upgrade, restart, shutdown,
   and recovery tests. Make the product verification suite a release gate.
6. Release image and docs together. Operators backing an existing deployment up
   first run `stele migrate status`, then use a serialized forward migration or
   `auto` policy, then run the product smoke and scope readiness proof.

Rollback is application/image rollback only when schema-compatible. A migration
that changes persisted state is corrected by a new forward migration; no
automatic down migration is attempted. Operators restore a verified backup into
an explicit target if rollback cannot be made schema-compatible.

## Open Questions

- None for the implementation start. The migration library choice is an
  implementation spike with the explicit acceptance criteria in the migration
  specification; it must be selected before production code is committed.
