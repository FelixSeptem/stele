package openapi

const providerReferencePaths = `  /v1/provider/references:
    post:
      operationId: providerWriteReference
      description: Accept a bounded reference event through ordinary asynchronous governance. The UTF-8 JSON request body is limited to 32768 bytes. Acceptance does not establish canonical completion. Identical scoped idempotent replay returns the original event; changed reuse conflicts.
      security:
        - PublicAPIKey: []
      parameters:
        - {$ref: '#/components/parameters/RuntimeBindingHeader'}
        - {$ref: '#/components/parameters/RuntimeSessionHeader'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
      requestBody:
        required: true
        content:
          application/json:
            schema: {$ref: '#/components/schemas/ProviderReferenceWriteRequest'}
      responses:
        '202':
          description: Reference event accepted for asynchronous governance
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderReferenceResponse'}
        '400':
          description: Bounded invalid_request for malformed, null, duplicate, unknown or invalid fields and limits. Unsupported base Provider schema returns the existing compatibility error.
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '401':
          description: Authentication required
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '403':
          description: Current runtime binding, exact scope, grant or source-attestation authority denied
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '409':
          description: Unsupported durable-reference contract or conflicting idempotency-key reuse
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '413':
          description: Request body exceeds 32768 bytes
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '503':
          description: Retryable authoritative dependency failure
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
  /v1/provider/references/status:
    post:
      operationId: providerReferenceStatus
      description: Reauthorize the current exact scope and disclosure grant before resolving a durable event. Missing, foreign, lifecycle-hidden and source-hidden evidence share unavailable without record identities or failure details. The request body is limited to 32768 bytes.
      security:
        - PublicAPIKey: []
      parameters:
        - {$ref: '#/components/parameters/RuntimeBindingHeader'}
        - {$ref: '#/components/parameters/RuntimeSessionHeader'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
      requestBody:
        required: true
        content:
          application/json:
            schema: {$ref: '#/components/schemas/ProviderReferenceStatusRequest'}
      responses:
        '200':
          description: Scoped bounded governance status
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderReferenceResponse'}
        '400':
          description: Bounded validation error; unsupported base Provider schema returns the existing compatibility error
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '401':
          description: Authentication required
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '403':
          description: Current runtime binding, exact scope, grant or requested disclosure denied; no record-existence detail
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '409':
          description: Unsupported durable-reference contract
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '413':
          description: Request body exceeds 32768 bytes
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '503':
          description: Retryable authoritative dependency failure
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
  /v1/provider/references/lookup:
    post:
      operationId: providerLookupReference
      description: Resolve the newest exact reference ID and normalized non-root path inside the current authorized scope, independently of ranked search. Select the newest write before visibility checks; a hidden latest write cannot reveal an older value. Missing and hidden evidence share unavailable. The request body is limited to 32768 bytes.
      security:
        - PublicAPIKey: []
      parameters:
        - {$ref: '#/components/parameters/RuntimeBindingHeader'}
        - {$ref: '#/components/parameters/RuntimeSessionHeader'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
      requestBody:
        required: true
        content:
          application/json:
            schema: {$ref: '#/components/schemas/ProviderReferenceLookupRequest'}
      responses:
        '200':
          description: Scoped exact-path reference result
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderReferenceResponse'}
        '400':
          description: Bounded validation error; unsupported base Provider schema returns the existing compatibility error
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '401':
          description: Authentication required
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '403':
          description: Current runtime binding, exact scope, grant or requested disclosure denied; no record-existence detail
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '409':
          description: Unsupported durable-reference contract
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '413':
          description: Request body exceeds 32768 bytes
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
        '503':
          description: Retryable authoritative dependency failure
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderErrorResponse'}
`

const providerReferenceSchemas = `    ProviderReferenceWriteRequest:
      type: object
      description: Reference operation metadata is bounded to 4096 bytes of normalized JSON, including all optional fields. event_seq is echoed compatibility attribution and does not order reference writes or gate durable idempotent replay; latest sources are ordered by durable created_at and id.
      additionalProperties: false
      required: [metadata, input]
      properties:
        metadata: {$ref: '#/components/schemas/ProviderReferenceWriteMetadata'}
        input: {$ref: '#/components/schemas/ProviderReferenceWriteInput'}
    ProviderReferenceStatusRequest:
      type: object
      description: Reference operation metadata is bounded to 4096 bytes of normalized JSON, including all optional fields.
      additionalProperties: false
      required: [metadata, input]
      properties:
        metadata: {$ref: '#/components/schemas/ProviderOperationMetadata'}
        input: {$ref: '#/components/schemas/ProviderReferenceStatusInput'}
    ProviderReferenceLookupRequest:
      type: object
      description: Reference operation metadata is bounded to 4096 bytes of normalized JSON, including all optional fields.
      additionalProperties: false
      required: [metadata, input]
      properties:
        metadata: {$ref: '#/components/schemas/ProviderOperationMetadata'}
        input: {$ref: '#/components/schemas/ProviderReferenceLookupInput'}
    ProviderReferenceWriteMetadata:
      allOf:
        - {$ref: '#/components/schemas/ProviderOperationMetadata'}
        - type: object
          required: [idempotency_key]
          properties:
            idempotency_key: {type: string, minLength: 1, maxLength: 256}
    ProviderReferenceWriteInput:
      type: object
      additionalProperties: false
      required: [contract_version, reference_id, memory_path, event_type, value]
      properties:
        contract_version: {type: string, enum: [durable-reference-v1]}
        reference_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        memory_path: {type: string, minLength: 1, maxLength: 512, description: Exact memory path normalized by the shared memory-path rules. Surrounding whitespace and leading/trailing slashes normalize to the same path; root paths are rejected.}
        event_type: {type: string, enum: [provider.reference.task, provider.reference.run, provider.reference.handoff, provider.reference.compilation]}
        value: {type: string, minLength: 1, maxLength: 4096, description: Valid UTF-8 value bounded to 4096 bytes; blank or surrounding-whitespace values are rejected. Admission also measures JSON escaping against the 16384-byte completed-response allocation with maximum permitted scope/metadata/lineage reserved. Values that exceed this allocation fail validation before ingestion.}
        source_metadata:
          allOf:
            - {$ref: '#/components/schemas/SourceMetadata'}
          description: Omission uses the shared untrusted/restricted default. Attested data requires current source-attestation authority, including replay.
    ProviderReferenceStatusInput:
      type: object
      additionalProperties: false
      required: [contract_version, event_id]
      properties:
        contract_version: {type: string, enum: [durable-reference-v1]}
        event_id: {type: string, format: uuid}
        disclosure_profile: {type: string, maxLength: 64, pattern: '^[A-Za-z0-9_.:-]+$'}
        response_mode: {type: string, enum: [reference_only, bounded_content], default: reference_only}
    ProviderReferenceLookupInput:
      type: object
      additionalProperties: false
      required: [contract_version, reference_id, memory_path]
      properties:
        contract_version: {type: string, enum: [durable-reference-v1]}
        reference_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        memory_path: {type: string, minLength: 1, maxLength: 512, description: Exact path normalized by the shared memory-path rules before lookup; root is rejected.}
        disclosure_profile: {type: string, maxLength: 64, pattern: '^[A-Za-z0-9_.:-]+$'}
        response_mode: {type: string, enum: [reference_only, bounded_content], default: reference_only}
    CanonicalScopeDescriptor:
      type: object
      description: Reference routes require each runtime scope dimension to be an ASCII token of at most 128 bytes before accessing durable records. Unsupported bound dimensions return HTTP 400 invalid_reference_scope before ingestion or lookup; existing non-reference runtime limits are unchanged.
      additionalProperties: false
      required: [tenant, project, namespace, agent_id, session_id, provider_instance_id]
      properties:
        tenant: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        project: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        namespace: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        agent_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        session_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        conversation_id: {type: string, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]*$'}
        provider_instance_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
    ProviderReferenceRecord:
      type: object
      additionalProperties: false
      required: [reference_id, memory_path, event_type, event_id, candidate_id, memory_id, version_id, provenance_id, version]
      properties:
        reference_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        memory_path: {type: string, minLength: 1, maxLength: 512}
        event_type: {type: string, enum: [provider.reference.task, provider.reference.run, provider.reference.handoff, provider.reference.compilation]}
        event_id: {type: string, format: uuid}
        candidate_id: {type: string, format: uuid}
        memory_id: {type: string, format: uuid}
        version_id: {type: string, format: uuid}
        provenance_id: {type: string, format: uuid}
        version: {type: integer, minimum: 1}
        value: {type: string, maxLength: 4096}
    ProviderReferenceResponse:
      type: object
      additionalProperties: false
      required: [metadata, contract_version, scope, status, retry_after_ms, retryable]
      properties:
        metadata: {$ref: '#/components/schemas/ProviderOperationMetadata'}
        contract_version: {type: string, enum: [durable-reference-v1]}
        scope: {$ref: '#/components/schemas/CanonicalScopeDescriptor'}
        status: {type: string, enum: [accepted, pending, failed, completed, unavailable]}
        event_id: {type: string, format: uuid}
        replayed: {type: boolean}
        reference: {$ref: '#/components/schemas/ProviderReferenceRecord'}
        retry_after_ms: {type: integer, minimum: 0, maximum: 30000}
        retryable: {type: boolean}
        error_category: {type: string, enum: [retryable, dependency]}
    ProviderDurableReferenceCapabilities:
      type: object
      additionalProperties: false
      required: [enabled, contract_version, event_types, statuses, max_value_bytes, max_reference_id_bytes, max_path_bytes, max_response_bytes, poll_after_ms]
      properties:
        enabled: {type: boolean}
        contract_version: {type: string, enum: [durable-reference-v1]}
        event_types: {type: array, maxItems: 4, uniqueItems: true, items: {type: string, enum: [provider.reference.task, provider.reference.run, provider.reference.handoff, provider.reference.compilation]}}
        statuses: {type: array, maxItems: 5, uniqueItems: true, items: {type: string, enum: [accepted, pending, failed, completed, unavailable]}}
        max_value_bytes: {type: integer, enum: [4096]}
        max_reference_id_bytes: {type: integer, enum: [128]}
        max_path_bytes: {type: integer, enum: [512]}
        max_response_bytes: {type: integer, enum: [16384]}
        poll_after_ms: {type: integer, enum: [1000]}
`
