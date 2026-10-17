package openapi

const memoryRepositoryPaths = `  /v1/memory-repositories:
    post:
      operationId: registerMemoryRepository
      description: Register a bounded external-memory source in the exact request scope. Registration does not access a filesystem, Git remote, archive, URL, or canonical memory.
      parameters:
        - {$ref: '#/components/parameters/PublicAPIKey'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
      requestBody:
        required: true
        content:
          application/json:
            schema: {$ref: '#/components/schemas/MemoryRepositoryRegistrationRequest'}
      responses:
        '201': {description: Scoped source registered, content: {application/json: {schema: {$ref: '#/components/schemas/MemoryRepositoryRegistrationResponse'}}}}
        '400': {description: Invalid bounded registration request}
        '401': {description: Authentication required}
        '403': {description: Exact scope or governed write authorization denied}
        '404': {description: Feature disabled}
  /v1/memory-repositories/{repository_id}/previews:
    post:
      operationId: previewMemoryRepositoryImport
      description: Parse only the supplied bounded snapshot and return a reviewable import plan. This operation does not clone, fetch, execute, enqueue, or write canonical memory.
      parameters:
        - {name: repository_id, in: path, required: true, schema: {type: string, minLength: 1, maxLength: 128}}
        - {$ref: '#/components/parameters/PublicAPIKey'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
      requestBody:
        required: true
        content:
          application/json:
            schema: {$ref: '#/components/schemas/MemoryRepositoryPreviewRequest'}
      responses:
        '201': {description: Immutable bounded import preview, content: {application/json: {schema: {$ref: '#/components/schemas/MemoryRepositoryPreviewResponse'}}}}
        '400': {description: Invalid bounded request}
        '401': {description: Authentication required}
        '403': {description: Exact scope or governed write authorization denied without repository disclosure}
        '404': {description: Repository unavailable in the authorized scope or feature disabled}
        '409': {description: Stale generation or conflicting claimed revision}
        '413': {description: Snapshot exceeds an effective size limit}
        '422': {description: "Invalid source format, path, metadata, or explicit selection"}
  /v1/memory-repositories/{repository_id}/imports:
    post:
      operationId: submitMemoryRepositoryImport
      description: Accept an immutable reviewed preview for durable asynchronous governed-intent dispatch. The service rechecks exact scope, policy, preview expiry, plan digest, warning acknowledgement, and repository generation.
      parameters:
        - {name: repository_id, in: path, required: true, schema: {type: string, minLength: 1, maxLength: 128}}
        - {$ref: '#/components/parameters/PublicAPIKey'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
        - {$ref: '#/components/parameters/IdempotencyKeyHeader'}
      requestBody:
        required: true
        content: {application/json: {schema: {$ref: '#/components/schemas/MemoryRepositoryImportRequest'}}}
      responses:
        '202': {description: Import accepted for asynchronous dispatch, content: {application/json: {schema: {$ref: '#/components/schemas/MemoryRepositoryImportResponse'}}}}
        '400': {description: Invalid bounded request}
        '401': {description: Authentication required}
        '403': {description: Exact scope or governed write authorization denied without repository disclosure}
        '404': {description: Repository unavailable in the authorized scope or feature disabled}
        '409': {description: "Preview expired, plan changed, stale generation, policy changed, or idempotency key conflicted"}
  /v1/memory-repositories/{repository_id}/imports/{import_id}:
    get:
      operationId: getMemoryRepositoryImport
      description: Read bounded import dispatch state. Completion means dispatch settled, not that every selected memory is active.
      parameters:
        - {name: repository_id, in: path, required: true, schema: {type: string, minLength: 1, maxLength: 128}}
        - {name: import_id, in: path, required: true, schema: {type: string, minLength: 1, maxLength: 128}}
        - {$ref: '#/components/parameters/PublicAPIKey'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
      responses:
        '200': {description: Current bounded import status, content: {application/json: {schema: {$ref: '#/components/schemas/MemoryRepositoryImportStatus'}}}}
        '401': {description: Authentication required}
        '403': {description: Exact scope or disclosure authorization denied without content or existence detail}
        '404': {description: Import unavailable in the authorized scope}
  /v1/memory-repositories/{repository_id}/manifest:
    get:
      operationId: getMemoryRepositoryManifest
      description: Read at most 100 currently eligible navigation items from the latest accepted import generation. The manifest does not inject context or reveal hidden targets.
      parameters:
        - {name: repository_id, in: path, required: true, schema: {type: string, minLength: 1, maxLength: 128}}
        - {$ref: '#/components/parameters/PublicAPIKey'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
      responses:
        '200': {description: Lifecycle and disclosure filtered navigation, content: {application/json: {schema: {$ref: '#/components/schemas/MemoryRepositoryManifest'}}}}
        '401': {description: Authentication required}
        '403': {description: Exact scope or disclosure authorization denied without target detail}
        '404': {description: Repository unavailable in the authorized scope or feature disabled}
`

const memoryRepositorySchemas = `    MemoryRepositoryRegistrationRequest:
      type: object
      additionalProperties: false
      required: [name]
      properties:
        name: {type: string, minLength: 1, maxLength: 256}
        source_url: {type: string, maxLength: 2048, description: Opaque untrusted label; the service never fetches it.}
    MemoryRepositoryRegistrationResponse:
      type: object
      additionalProperties: false
      required: [repository_id, generation]
      properties:
        repository_id: {type: string, format: uuid}
        generation: {type: integer, minimum: 0}
    MemoryRepositoryPreviewRequest:
      type: object
      additionalProperties: false
      required: [actor, reason, expected_generation, snapshot, selections]
      properties:
        actor: {type: string, minLength: 1, maxLength: 256}
        reason: {type: string, minLength: 1, maxLength: 1024}
        expected_generation: {type: integer, minimum: 0}
        snapshot: {$ref: '#/components/schemas/MemoryRepositorySnapshot'}
        selections:
          type: array
          minItems: 1
          maxItems: 1000
          items: {$ref: '#/components/schemas/MemoryRepositorySelection'}
        source_metadata: {$ref: '#/components/schemas/SourceMetadata'}
    MemoryRepositorySnapshot:
      type: object
      additionalProperties: false
      required: [format_version, source_revision, files]
      properties:
        format_version: {type: string, enum: [memory-repo-v1]}
        source_revision: {type: string, minLength: 1, maxLength: 256, description: "Opaque claimed provenance, not an attestation or sortable version."}
        files:
          type: array
          minItems: 1
          maxItems: 128
          items: {$ref: '#/components/schemas/MemoryRepositorySnapshotFile'}
    MemoryRepositorySnapshotFile:
      type: object
      additionalProperties: false
      required: [path, content]
      properties:
        path: {type: string, minLength: 1, maxLength: 256}
        content: {type: string, maxLength: 65536}
    MemoryRepositorySelection:
      type: object
      additionalProperties: false
      required: [path, entry_key, memory_class, memory_path]
      properties:
        path: {type: string, minLength: 1, maxLength: 256}
        entry_key: {type: string, minLength: 1, maxLength: 256}
        memory_class: {type: string, enum: [profile, episodic, procedural, summary, relation]}
        memory_path: {type: string, minLength: 1, maxLength: 512}
    MemoryRepositoryPreviewResponse:
      type: object
      additionalProperties: false
      required: [preview_id, repository_id, generation, plan_digest, warning_digest, expires_at, parser_version, effective_limits]
      properties:
        preview_id: {type: string, format: uuid}
        repository_id: {type: string, format: uuid}
        generation: {type: integer, minimum: 0}
        plan_digest: {type: string, minLength: 64, maxLength: 64}
        warning_digest: {type: string, minLength: 64, maxLength: 64}
        expires_at: {type: string, format: date-time}
        parser_version: {type: string, enum: [memory-repo-parser-v1]}
        effective_limits: {$ref: '#/components/schemas/MemoryRepositoryLimits'}
    MemoryRepositoryLimits:
      type: object
      additionalProperties: false
      required: [max_files, max_file_bytes, max_content_bytes, max_entries, max_entry_bytes, max_path_bytes, max_revision_bytes]
      properties:
        max_files: {type: integer, maximum: 128}
        max_file_bytes: {type: integer, maximum: 65536}
        max_content_bytes: {type: integer, maximum: 1048576}
        max_entries: {type: integer, maximum: 1000}
        max_entry_bytes: {type: integer, maximum: 8192}
        max_path_bytes: {type: integer, maximum: 256}
        max_revision_bytes: {type: integer, maximum: 256}
    MemoryRepositoryImportRequest:
      type: object
      additionalProperties: false
      required: [preview_id, plan_digest, warning_digest, expected_generation]
      properties:
        preview_id: {type: string, format: uuid}
        plan_digest: {type: string, minLength: 64, maxLength: 64}
        warning_digest: {type: string, minLength: 64, maxLength: 64}
        expected_generation: {type: integer, minimum: 0}
    MemoryRepositoryImportResponse:
      type: object
      additionalProperties: false
      required: [import_id, repository_id, generation, status]
      properties:
        import_id: {type: string, format: uuid}
        repository_id: {type: string, format: uuid}
        generation: {type: integer, minimum: 1}
        status: {type: string, enum: [queued]}
        replayed: {type: boolean}
    MemoryRepositoryImportStatus:
      type: object
      additionalProperties: false
      required: [import_id, repository_id, generation, status, entries]
      properties:
        import_id: {type: string, format: uuid}
        repository_id: {type: string, format: uuid}
        generation: {type: integer, minimum: 1}
        status: {type: string, enum: [queued, running, completed]}
        entries: {type: array, maxItems: 1000, items: {$ref: '#/components/schemas/MemoryRepositoryImportEntryStatus'}}
    MemoryRepositoryImportEntryStatus:
      type: object
      additionalProperties: false
      required: [entry_key, status]
      properties:
        entry_key: {type: string, maxLength: 256}
        status: {type: string, enum: [queued, submitted, duplicate, rejected, failed]}
        intent_id: {type: string, format: uuid}
    MemoryRepositoryManifest:
      type: object
      additionalProperties: false
      required: [repository_id, generation, processing_status, items]
      properties:
        repository_id: {type: string, format: uuid}
        generation: {type: integer, minimum: 1}
        processing_status: {type: string, enum: [queued, running, completed]}
        items: {type: array, maxItems: 100, items: {$ref: '#/components/schemas/MemoryRepositoryManifestItem'}}
    MemoryRepositoryManifestItem:
      type: object
      additionalProperties: false
      required: [memory_id, version, source_path]
      properties:
        memory_id: {type: string, format: uuid}
        version: {type: integer, minimum: 1}
        source_path: {type: string, maxLength: 256}
`
