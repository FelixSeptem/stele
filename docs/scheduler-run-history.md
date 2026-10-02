# Durable Scheduler Run History

Stele stores scheduler run summaries and append-only attempt details in
PostgreSQL. The stable run key is derived from the job class, normalized exact
tenant/project/namespace scope, and cadence window. A duplicate scheduler tick
records a duplicate disposition and does not execute the maintenance body.

## Inspecting Runs

Use the admin surface with an exact scope:

```text
GET /v1/admin/jobs/run-history?limit=20&state=completed&cursor=<opaque>
X-API-Key: <admin-key>
X-Stele-Tenant: <tenant>
X-Stele-Project: <project>
X-Stele-Namespace: <namespace>
```

Supported filters are `job_class`, `state`, `recovery`, `observed_from`, and
`observed_to`. The response is ordered by observed time and run key and returns
an opaque continuation cursor. The repository applies the exact scope before
resolving run existence; unauthorized scopes do not reveal counts or records.

Read one run and its bounded attempt history with:

```text
GET /v1/admin/jobs/run-history/<run-key>
```

Raw errors, credentials, payloads, source content, and unrelated scope data are
never included in the response or scheduler telemetry.

## Recovery And Cancellation

An operator may cancel a pending, retrying, or failed run through the governed
admin action:

```text
POST /v1/admin/jobs/run-history/<run-key>?action=cancel
```

Cancellation fails while an active lease exists. Recovery uses the same action
with `action=recover`; it records the prior terminal history and returns the run
to the ordinary scheduler claim path. It never seizes an active lease or runs a
maintenance body inline.

Retry exhaustion is terminal and is not automatically re-enqueued. A recovery
must be explicit and remains visible as a recovery transition.

## Retention And Freshness

High-volume attempt details are pruned after the configured detail window. The
terminal summary, attempt count, checkpoint/source-watermark identity, terminal
disposition, and required audit transitions remain retained. Cleanup is
idempotent and emits bounded retention telemetry. Stale or divergent watermark
evidence is not eligible for readiness or conformance claims.

The scheduler lifecycle metric is `stele_scheduler_runs_total`. Its labels are
fixed categories for operation, result, state, lease, retry, recovery, record,
freshness, SLO, and duration bucket; scope values and identifiers are rejected
or normalized to `unknown`.
