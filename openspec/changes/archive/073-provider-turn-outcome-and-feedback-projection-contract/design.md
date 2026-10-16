## Context

See `proposal.md` for motivation. The service already has public Provider turn
and outcome routes, scoped session history, append-only usefulness feedback,
governed insight derivation, context digest identity and PC5 durable references.
Those surfaces have generic or independently versioned contracts; they do not
yet provide one strict runtime evidence projection with correction and
pending-reference semantics. PostgreSQL remains the only source of record.

## Goals / Non-Goals

**Goals:**

- Add a dedicated `turn-outcome-feedback-projection-v1` contract whose closed
  inputs/outputs can evolve without changing legacy generic routes.
- Persist one logical projection with append-only revisions, durable idempotency,
  exact binding scope and auditable links to session/outcome, PC4 digest, PC5
  reference attribution and feedback.
- Reuse current feedback and insight pipelines while making their asynchronous,
  policy-gated effect explicit.

**Non-Goals:**

- No new runtime business-state table, task orchestration engine, autonomous
  reflection loop, direct ranking write or direct canonical-memory mutation.
- No content read capability from projection references and no bypass of PC5
  lifecycle, disclosure, provenance or exact-path behavior.

## Decisions

### Dedicated versioned endpoint over extending generic Provider turn routes

Add a named projection operation and capability rather than extending the
existing generic `input: object` route contract. It owns strict snake-case DTOs,
result unions, documented bounds and independently testable errors. Existing
turn/outcome routes remain supported as legacy/session primitives but cannot
silently implement projection semantics.

Alternative considered: extend generic routes. It lowers route count but leaves
the public behavior implicit, makes correction unions difficult to validate and
would require consumers to infer PC6 support from a base schema.

### Effect identity, revision and correction form an append-only state machine

Admission keys are scoped by resolved runtime binding, projection kind, effect
identity and idempotency key. The repository atomically claims an equivalent
normalized payload and returns its original result on replay. Reusing a key with
a different payload conflicts. A greater compatible source revision writes a new
evidence row with `supersedes` lineage; earlier rows remain immutable. Delayed or
out-of-order revisions receive a bounded stale/conflict disposition rather than
silently replacing the latest evidence.

Alternative considered: update one row in place. It simplifies reads but violates
the repository's version/provenance rule and destroys correction audit evidence.

### PC5 reference state is attribution, not access or completion authority

Admission resolves each supplied durable reference only under the already
validated exact binding. It stores a bounded state snapshot (`accepted`,
`pending`, `completed`, `failed`, `unavailable`) and lineage identifiers allowed
by PC5. A pending record is valid historical attribution but cannot feed eligible
feedback summary, ranked retrieval or derived insight activation. Consumers and
jobs re-evaluate current lifecycle/grants rather than trusting a stale snapshot.

Alternative considered: require completed references. It is simpler but prevents
runtime evidence from being durably associated with an accepted asynchronous
write; the chosen model retains that evidence while preventing false completion.

### Feedback is admitted separately from influence

Projection persistence may create or link scoped feedback evidence in the same
durable transaction only after projection/reference validation. Existing
summaries, calibration, quality and insight jobs consume eligible active feedback
later under their own versioned policies. No write returns a ranking or insight
activation claim.

Alternative considered: apply feedback to ranking at projection write time. It
creates unreviewed runtime-controlled ranking changes and bypasses existing
activation controls.

### Public assurance follows the PC5 owned-stack pattern

The verifier will bootstrap an owned PostgreSQL + pgvector environment through
official public paths, exercise public projection writes, then use bounded SQL
only to supplement lineage assertions. It repeats replay/read outcomes after
API/worker restart and produces redacted reports with source, image, OpenAPI,
contract and migration provenance.

## Risks / Trade-offs

- [Generic endpoint overlap confuses consumers] → capability negotiation and
  documentation explicitly distinguish legacy turn/outcome from PC6.
- [Pending reference becomes mistaken for usable memory] → state-discriminated
  DTOs, strict consumer checks and eligibility gates prohibit that interpretation.
- [Revision races duplicate feedback] → use transactional claim/locking over the
  scoped logical identity and test concurrent/restart replay.
- [Projection data becomes a covert business-state store] → closed DTOs reject
  state fields and reports carry an explicit runtime-reported-evidence label.
- [Feedback unexpectedly changes ranking] → separate persistence, summary and
  policy activation; test no-effect behavior without an authorized policy.

## Migration Plan

1. Publish the additive capability and closed OpenAPI schemas while retaining
   existing Provider turn/outcome and feedback routes unchanged.
2. Deploy the storage and public route behind advertised capability negotiation;
   unsupported consumers retain existing behavior and strict consumers fail
   closed when it is absent.
3. Run fresh owned-stack initial/restart verification before documenting PC6
   readiness. Roll back by disabling the advertised capability/route while
   retaining append-only evidence, provenance and feedback history; do not
   delete or reinterpret already accepted records.

