# Context Projections

Context projections are derived, append-only PostgreSQL read models. They are
scoped by exact `tenant`, `project`, and `namespace` values and reference
canonical memory versions or raw-event evidence; they never replace canonical
memory or delete provenance.

Projection reads are opt-in and disabled by default. Set
`STELE_CONTEXT_PROJECTION_CONSUMPTION_ENABLED=true` only after operators inspect
the projection kind,
version, status, source watermark, policy version, renderer version, and
bounded item counts before enabling consumption. A rebuild reads the authorized
source snapshot and creates a new projection version, retaining prior versions
for audit and rollback.

Ordinary context excludes suppressed, forgotten, expired, and deleted sources.
Profile items may enter `always_visible` only after confidence and size gates;
summaries may enter `session`; episodic, procedural, and relation memories stay
on-demand; raw events remain archival evidence only.

If a rollout is stale or unsafe, disable projection consumption and continue
with live retrieval. Do not widen scope, increase the request budget, mutate
canonical memory, or delete projection history as a rollback operation.

## Evidence-gated resident context evaluation

Maintainers can evaluate a stricter `always_visible` profile candidate against
the existing confidence/size policy with `EvaluateResidentContext`. This is an
internal offline/shadow comparison. It adds no public request fields, response
shapes, authorization roles, Provider capability, MCP tool, runtime flag, worker,
or migration. Ordinary projection consumption retains its existing default.

Both conditions use the existing materializer and equal, independently enforced
limits: 8 items, 4096 UTF-8 content bytes, 1024 whitespace-delimited tokens, and
16 citations by default. The existing per-item profile gate also applies.
Whitespace counts are evaluator accounting, not model-token usage or Provider
wire/body bytes. Whole items are omitted when a limit would be exceeded; source
facts remain available through authorized on-demand retrieval.

Candidate selection requires a current active canonical snapshot and its exact
immutable version/content, valid time, exact scope, and disclosable metadata on
both snapshots. Missing/legacy restricted source metadata cannot be assumed
disclosable. An attributable raw-event support observation must bind the memory
ID, source version and SHA-256 content fingerprint, declare stability and
applicability, and be fresh. Only `tool_observation`, `user_correction`, or
`external_evaluation` can corroborate; transcripts and model assertions cannot.
Support, utility and comparison events must also have disclosable,
`attested_data` source metadata under the existing exact-scope source-attestation
contract. The body actor alone never establishes attribution. Attestation
establishes authorized source attribution, not truth or instruction authority.
A separate positive utility observation must bind the same version and occur
at a later held-out stage and time. Observations are limited to 30 days of age
and validity. Ambiguous, expired, negative or absent observations exclude the
candidate; currently resident entries that are excluded are proposed for
demotion. Access frequency never affects selection, ordering, or run identity.

Inputs are bounded to 256 sources/current IDs/support records/utility records,
with at most 16 KiB of content and 32 KiB encoded size per observation. Invalid
structural bounds return an internal error; ineligible sources and insufficient
evidence produce aggregate omission reasons. No evaluation writes canonical
memory, evidence, or projection history. Disablement returns the copied baseline
plan; a future live policy rollout needs separate contract and release work.
Observation JSON accepts exact canonical lowercase field names only; unknown,
duplicate, case-alias and null fields fail closed. Only compatible validated
support/utility for evaluated eligible sources advances the comparison stage;
discarded or unrelated observations cannot block later outcomes.

Serialized reports contain counts, fixed reasons and hashes of actual inputs
and plans, without source text, IDs, scope or raw observations. Separate later
external comparison observations supply task outcomes and costs. Missing values
remain unobserved, including when another metric is observed as zero or false;
condition deltas are observational. Hashes identify integrity, not authenticated
truth or activation authority. See [self-hosting](self-hosting.md#evidence-gated-resident-context-evaluation)
for the owned PostgreSQL/pgvector verifier and persistence/restart checks.
