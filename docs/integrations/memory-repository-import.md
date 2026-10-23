# Memory repository snapshot import

**Role:** integration reference for the optional review-first Markdown import
boundary. Start at the [documentation portal](../README.md) to choose between
this path, Provider, MCP, and native OpenAPI.

Stele accepts a bounded `memory-repo-v1` Markdown snapshot through HTTP only.
It is a one-way, review-first adapter: PostgreSQL remains the system of record,
and accepted facts enter existing governed `remember` intent processing rather
than writing canonical memory directly.

## Boundary and enablement

Admission defaults to disabled. Set `STELE_MEMORY_REPOSITORY_IMPORT_ENABLED=true`
to permit registration, preview, and submission. Disabling it later blocks new
admission but preserves accepted work, audit history, and governed worker drain.

The service never clones or fetches Git repositories, reads caller filesystem
paths, retrieves URLs, extracts archives, follows links outside the snapshot,
or executes Markdown, code fences, commands, URLs, or artifacts. V1 adds no
MCP tool.

Every request uses the exact tenant/project/namespace scope and current
disclosure authorization. An unauthorized scope is rejected before repository,
preview, replay, generation, or source evidence can be disclosed. Markdown
metadata never grants trust, authority, scope, lifecycle, or policy privilege.

## Review-first HTTP protocol

1. `POST /v1/memory-repositories` registers opaque labels only.
2. `POST /v1/memory-repositories/{repository_id}/previews` supplies the whole
   JSON snapshot and explicit `(entry_key, memory_class, memory_path)` mappings.
3. Review diagnostics, effective trust, plan digest, warning digest, and its
   24-hour expiry. Submit the unchanged plan with `Idempotency-Key`, matching
   generation, plan digest, and warning digest.
4. Poll import status or read the manifest. `queued`, `running`, and
   `completed` describe dispatch, not canonical activation.

Status returns bounded `queued`, `submitted`, `duplicate`, `rejected`, or
`failed` entry outcomes without source payload. The maximum 100-item manifest
is current lifecycle- and disclosure-eligible navigation only; it never adds
always-visible context, ranking boosts, or hidden-target counts.

## Format, limits, and lifecycle

`MEMORY.md` is required. V1 accepts UTF-8 NFC slash-relative `.md` files and a
narrow subset: top-level single-line fact bullets, bounded trailing metadata,
and standalone root-relative wiki links outside fences. Unsupported prose,
nested or multiline lists, malformed metadata, and unresolved links are
diagnosed, not interpreted as instructions.

Maximums are 128 files, 64 KiB per file, 1 MiB decoded content, 2 MiB request
body, 1,000 entries, 8 KiB per entry, 256-byte paths/revisions, 16 metadata
pairs, and 2,048 bytes per metadata value. Limits and invalid paths reject the
whole request without partial import.

Acceptance retains immutable plan/revision evidence, generation, attempts, and
deduplication identities. Same-key equivalent replay returns the original
import; changed requests conflict. Missing files never update, forget, delete,
or resurrect canonical memory. Migration `0038_memory_repository_imports` is
additive and intentionally non-destructive; disable admission before rollback
and retain worker/schema compatibility until accepted work drains.
