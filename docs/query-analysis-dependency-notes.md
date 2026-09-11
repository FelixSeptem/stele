# Query-analysis dependency decision

## Decision

The first deterministic query-analysis implementation uses the Go standard
library for explicit time layouts and `golang.org/x/text` for Unicode
normalization and locale-independent case handling. It does not add a general
natural-language date parser, tokenizer, online model client, or query-planning
provider.

The repository already resolves `golang.org/x/text v0.31.0` transitively. If
query analysis imports its `cases` or `unicode/norm` packages directly, the
module must be promoted to a direct dependency with `go mod tidy` so ownership
is explicit.

## Package review

The following package pages were reviewed on pkg.go.dev on 2026-09-11:

- Go standard-library `strings` and `unicode`: stable, BSD-3-Clause, widely
  imported primitives for deterministic rune classification and explicit
  delimiter handling. They are sufficient for the first policy's bounded term
  extraction, whose rules must remain repository-owned and versioned.
- `github.com/blevesearch/segment` `v0.9.1`: stable, Apache-2.0, imported by 164
  packages on the reviewed page. It is a compact Unicode text segmenter, but
  its generic word-boundary behavior does not remove the need for explicit
  mixed-language policy rules.
- `github.com/ikawaha/kagome/v2/tokenizer` `v2.11.0`: stable, MIT, imported by
  103 packages on the reviewed page. Its Japanese morphological dictionaries
  and language-specific runtime surface are unnecessary for the initial
  language-agnostic bounded policy.
- `golang.org/x/text/cases`: stable module, BSD-3-Clause. It provides
  deterministic Unicode-aware case mapping without a remote service.
- `golang.org/x/text/unicode/norm`: stable module, BSD-3-Clause. It provides the
  Unicode normalization forms needed to compare and deduplicate bounded terms.
- `github.com/araddon/dateparse`: stable package, MIT, but its broad heuristic
  date recognition is larger and less policy-explicit than this rollout needs.
- `github.com/markusmobius/go-dateparser`: stable module, BSD-3-Clause, but adds
  a general natural-language parser and transitive surface that is unnecessary
  for the first bounded deterministic policy.

## Rationale and verification

Explicit accepted layouts and a fixed clock/time zone keep temporal behavior
versionable and replayable. Unicode normalization and case handling solve a
narrow standards problem, while entity, intent, and multi-hop rules remain
repository-owned policy. This avoids silent behavior changes from broad
heuristic parsers and keeps malformed or ambiguous dates in an explicit
unknown/fallback category.

The repository has no dedicated dependency-license checker in `scripts/` or
its product-verification workflow. The decision was therefore verified with
the available repository and Go controls: `go list -m -json golang.org/x/text`
resolved the existing checksummed `v0.31.0` module, `go mod graph` confirmed its
module ownership, and `go mod download -json golang.org/x/text@v0.31.0`
reported the same module and `go.mod` sums recorded in `go.sum`. A full
`go mod verify` was also attempted, but it failed because the machine-wide Go
cache contains a modified pre-existing
`github.com/docker/docker v28.3.3+incompatible` directory. That module is not a
dependency introduced or selected by this change; the failure is retained as
an environment finding rather than described as a passing repository check.

No new tokenizer or date-parser module was added, so there is no new transitive
license surface. Once `x/text` is imported directly, `go mod tidy` must promote
it from indirect to direct and the module, tests, repository quality commands,
and a clean-cache `go mod verify` must be rerun.

Any future parser dependency requires a policy-version change and a new review
of maintenance, license, deterministic behavior, and transitive dependencies.
