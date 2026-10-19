# Provider Context Turn Binding Design

## Objective and boundaries

Add an independently discoverable, optional turn binding to
`POST /v1/provider/context`. A successful turn-bound response proves that the
turn was created under the current server-issued runtime binding and its exact
session and tenant/project/namespace scope. The turn is attribution only: it
does not select, widen, filter, or authorize memory, change the disclosure
profile, or grant access to any other memory path.

Requests without `turn_id` retain the existing session/conversation behavior,
response shape, canonical bytes, digest, continuity fingerprint, and receipt
compatibility. This change does not add historical context replay, business-role
authorization, token accounting, or Danny task, generation, or outbox state.

## Authority and turn creation

The existing `memory_session_turns` table proves only turn/session/scope
membership; it does not prove runtime-binding ownership. Persist a separate,
immutable association between a provider-created turn and the complete
server-owned binding identity, including binding ID, principal, agent, session,
conversation, provider instance, and exact scope. PostgreSQL is the sole system
of record. The association and turn creation commit atomically in the trusted
Provider turn creation path; a caller-supplied binding or scope never establishes
ownership. Enforce that the persisted turn and session match the binding scope
and session.

Retain the existing generic turn path. Existing and generic turns without a
trusted association are not silently claimed or retroactively treated as
turn-bound. A retry of a provider turn creation may reuse an idempotent result
only if its immutable association belongs to the same binding. An idempotency or
turn-ID collision with an unassociated or differently associated turn is a
non-disclosing failure, not an opportunity to claim it. Concurrent attempts
must not create a turn without its association or transfer ownership.

The existing Provider turn flow assembles its own session context before the
turn is created. A strict consumer first creates the bound turn, then performs
the new Provider Context read; the latter is a fresh read, not a replay of the
earlier assembly. Bindings remain subject to existing expiry, revocation,
principal, session, exact-scope grant, and disclosure checks on every request.
A replacement binding, even for the same session, cannot claim the former
binding's turn.

## Public contract and authorization

Add optional `input.turn_id`: a nonempty ASCII token of at most 128 bytes,
matching `^[A-Za-z0-9_.:-]+$`, with no whitespace normalization. Null, empty,
duplicate, aliased, malformed, oversized, or unknown fields fail the existing
strict request validation. If present, successful `result.turn_id` is the
validated value; if absent, the response does not add the field. Add the field
to both disclosure-mode response schemas and count its encoded bytes in the
existing exact response budget and admission preflight.

After authenticating the runtime binding and validating the request and schema,
authorize the association before continuity lookup, context retrieval, or any
context-result side effect. Check the current exact scope, session, full
binding identity, and the stored turn/session association. Missing, foreign,
unassociated, wrong-session, and wrong-binding turns use the same bounded
non-disclosing denial. Binding middleware failures retain their existing generic
denial. Storage errors or a missing required turn-authority dependency return a
bounded retryable dependency failure, never a successful unverified response.
No turn data other than the verified identifier is returned.

Capability discovery advertises a separate turn-context contract version,
whether turn creation and authorization dependencies are operational, and the
turn-bound digest/canonicalization versions. Strict consumers require this
capability and its exact version before sending a turn ID; base `provider-v1`,
PC4, PC5, and PC6 do not imply it. If unavailable, turn-bound reads fail closed;
legacy turnless reads continue to work.

## Digest, continuity, citations, and audit

For a turn-bound response, use separately versioned canonicalization and digest
contracts, with a domain-separated SHA-256 over the same authorized public
projection as today plus the validated turn ID. Publish the exact field order,
JSON encoding, domain separator, version tuple, and independently verifiable
canonical byte/hash fixtures in the implementation's maintenance docs. A
turnless response uses the unmodified v1 algorithm and fixtures.

Bind the verified turn ID into the continuity request fingerprint only when it
is present, retaining existing fingerprint bytes for turnless requests. Same
binding, effective request, and turn compare normally. Comparing requests across
different turns, or between a turn-bound and a turnless request in either
direction, yields the existing request-changed continuity outcome before
retrieval; it must never be treated as an unchanged comparison. Receipt
validation and storage must support both digest versions
without weakening existing receipt checks. Existing unexpired turnless receipts
remain usable. Continuity is still a fresh comparison and stores no historical
context body.

The turn is associated with the response as a whole; existing public citations
remain limited to selected, currently authorized provenance. Do not add hidden
candidates, scores, internal ranking, source content, or other turn data to
citations or audit records. Persist an append-only, bounded-metadata attribution
record for each admitted turn-bound response containing a reference to its
immutable binding/turn association, exact scope, request/operation identifiers,
digest identity, optional continuity reference, and admission time. The audit
record must not expose the runtime binding bearer value. It records successful
admission, not proof of network delivery or business completion. Do not record
request query, context content, selected memory IDs, citations, credentials, or
hidden candidates. A failure to persist required attribution prevents a
successful response; where a continuity receipt is issued, attribution and
receipt must be committed atomically, after final response sizing succeeds.

PC6 receipt verification must not represent a mismatched turn as verified:
when a projection supplies a turn-bound receipt, it must also supply its turn,
and that turn and full binding must match the receipt's stored turn attribution.
Missing or mismatched turn attribution cannot be verified. Legacy turnless
receipt behavior remains unchanged. An unaccompanied digest continues to be
merely reported, not verified, and is not an authorization credential.

## Operations and compatibility

Use an additive migration with explicit constraints and indexes for association
and audit lookup, plus a non-destructive rollback strategy consistent with
existing Provider migrations. Document migration ordering, capability rollout,
the inability to claim legacy turns, binding expiry/replacement behavior,
appropriate audit retention and access controls, failure recovery, and mixed
version deployment requirements. Never advertise the capability from an API
instance that cannot enforce all dependencies. No turnless request requires a
new association, audit write, or receipt migration to succeed.

Update served OpenAPI, capability descriptions, Provider integration and
maintenance guides, README, and affected agent-facing skills. Document exact
request and response shapes, validation and errors, authorization, defaults,
digest vectors, receipt semantics, and compatibility. No SDK or UI is added.

## Verification

Use test-first coverage for strict parsing and schemas, capability negotiation,
both response modes, budgets, canonical golden vectors, digest and fingerprint
stability/change, and receipt compatibility. Exercise real PostgreSQL ownership
and atomicity: concurrent creation, retries, old/unassociated turns, distinct
bindings sharing a session, wrong session, every scope dimension, foreign and
missing turns, revoked or expired bindings, dependency failures, restart, and
audit/receipt transaction failures. Verify mismatched PC6 receipt attribution
cannot become verified, and verify legacy turnless paths and receipts remain
unchanged.

Check identical non-disclosing rejection for foreign and nonexistent turns;
ensure no retrieval or continuity lookup precedes turn authorization and that
changing only the authorized turn changes digest, fingerprint, and audit
attribution without changing disclosure, memory selection, or citations. Test
hidden/suppressed/forgotten/deleted memory remains inaccessible and audit and
responses contain neither hidden candidates nor ranking internals. Run existing
Provider, ingestion, consolidation, retrieval, forgetting, isolation, OpenAPI,
documentation, and applicable integration suites; report any unavailable live
PostgreSQL/pgvector verification rather than claiming it passed.
