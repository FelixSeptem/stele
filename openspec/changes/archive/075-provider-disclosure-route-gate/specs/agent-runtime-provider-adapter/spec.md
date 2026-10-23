## ADDED Requirements

### Requirement: Public retrieval and turns use disclosure-safe projections

The Provider SHALL admit `POST /v1/provider/retrieve` and
`POST /v1/provider/turns` for public runtime principals only after validating the
server-issued runtime binding and current exact-scope disclosure grant.
Retrieval input SHALL accept an optional supported disclosure profile and
response mode, defaulting to the current grant's profile and `reference_only`.
Its response SHALL contain only active, same-scope, low/medium-classified,
versioned matches and citations derived from those returned matches. It MUST
omit content in `reference_only`, and MUST omit scores, diagnostics, raw query,
hidden candidates, foreign evidence, and arbitrary citations in every mode.
Turn creation SHALL use the bound session and a server-issued turn ID, apply
the resolved mode before evidence persistence, and return only the closed turn
projection containing turn/session identity, status, effective profile/mode,
and memory IDs/citations; summary MAY appear only with an authorized
`memory-content-v1` grant and explicit `bounded_content`. Caller scope, session,
turn ID, role, and diagnostic overrides MUST NOT widen selection or disclosure.

#### Scenario: Public runtime retrieves a reference

- **WHEN** a runtime with a valid exact-scope binding and current reference grant sends a retrieval request without a response mode
- **THEN** Stele returns active eligible references and citations, omits content/scores/diagnostics, and performs no lookup outside the binding scope

#### Scenario: Public runtime explicitly requests bounded content

- **WHEN** a runtime's current exact-scope grant allows `memory-content-v1` and the request explicitly selects `bounded_content`
- **THEN** Stele may include eligible content while retaining the same scope, lifecycle, source, citation, and closed-response gates

#### Scenario: Caller requests unauthorized content or foreign binding

- **WHEN** a reference-only principal requests bounded content, or a request uses a foreign/mismatched runtime binding or session header
- **THEN** Stele returns a bounded denial without protected content or record-existence detail and does not dispatch retrieval or turn creation

#### Scenario: Turn evidence is persisted

- **WHEN** a public turn is assembled under `reference_only` or `bounded_content`
- **THEN** diagnostics are removed before persistence, reference-only evidence has no summary, and bounded-content summary is retained only under its resolved grant

#### Scenario: Turn or retrieval output contains internal data

- **WHEN** an internal search result or turn contains raw query, scores, diagnostics, foreign/hidden candidates, or arbitrary citation IDs
- **THEN** the public response contains only fields admitted by its typed projection and citations for returned references
