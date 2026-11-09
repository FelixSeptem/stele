package openapi

// proceduralVerificationPaths is intentionally privileged-only. There is no
// Provider, MCP, retrieval, or public API counterpart for this evidence ledger.
const proceduralVerificationPaths = `  /v1/admin/procedural-memory-verifications:
    post:
      operationId: createAdminProceduralMemoryVerification
      description: Append an attributable verification assertion for one active current procedural memory version. The server binds actor, scope, policy version, ID, and recorded time; raw evidence is never returned in this receipt.
      parameters:
        - {$ref: '#/components/parameters/AdminAPIKey'}
        - {$ref: '#/components/parameters/ActorHeader'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
      requestBody: {required: true, content: {application/json: {schema: {$ref: '#/components/schemas/ProceduralVerificationRecordRequest'}}}}
      responses:
        '201': {description: Immutable verification receipt, content: {application/json: {schema: {$ref: '#/components/schemas/ProceduralVerificationReceipt'}}}}
        '400': {description: Invalid bounded assertion or evidence}
        '401': {description: Missing or invalid administrator API key}
        '404': {description: "Target, source, or policy unavailable in the authorized scope"}
        '409': {description: Conflicting immutable verification replay}
    get:
      operationId: listAdminProceduralMemoryVerifications
      description: List redacted verification receipts for one exact-scope memory version. Hidden evidence bodies and corrected payloads are excluded.
      parameters:
        - {name: memory_id, in: query, required: true, schema: {type: string, minLength: 1, maxLength: 128}}
        - {name: target_version, in: query, required: true, schema: {type: integer, minimum: 1}}
        - {name: limit, in: query, required: false, schema: {type: integer, minimum: 1, maximum: 100, default: 50}}
        - {$ref: '#/components/parameters/AdminAPIKey'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
      responses:
        '200': {description: Bounded redacted verification history, content: {application/json: {schema: {$ref: '#/components/schemas/ProceduralVerificationHistoryResponse'}}}}
        '400': {description: Invalid exact-scope history query}
        '401': {description: Missing or invalid administrator API key}
        '404': {description: Verification history unavailable in the authorized scope}
  /v1/admin/procedural-memory-verifications/{verification_id}:
    get:
      operationId: getAdminProceduralMemoryVerification
      description: Read one redacted verification receipt after exact-scope authorization. This does not disclose raw hidden evidence or corrected content.
      parameters:
        - {name: verification_id, in: path, required: true, schema: {type: string, minLength: 1, maxLength: 128}}
        - {$ref: '#/components/parameters/AdminAPIKey'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
      responses:
        '200': {description: Redacted verification receipt, content: {application/json: {schema: {$ref: '#/components/schemas/ProceduralVerificationReceipt'}}}}
        '401': {description: Missing or invalid administrator API key}
        '404': {description: Verification unavailable in the authorized scope}
  /v1/admin/procedural-memory-verifications/{verification_id}:revalidate:
    post:
      operationId: requestAdminProceduralMemoryRevalidation
      description: Request bounded durable revalidation for one verification. This operation never semantically infers a result or mutates canonical memory.
      parameters:
        - {name: verification_id, in: path, required: true, schema: {type: string, minLength: 1, maxLength: 128}}
        - {$ref: '#/components/parameters/AdminAPIKey'}
        - {$ref: '#/components/parameters/ActorHeader'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
        - {name: X-Request-ID, in: header, required: true, schema: {type: string, minLength: 1, maxLength: 128}}
      requestBody: {required: true, content: {application/json: {schema: {$ref: '#/components/schemas/ProceduralVerificationRevalidationRequest'}}}}
      responses:
        '202': {description: Revalidation accepted, content: {application/json: {schema: {$ref: '#/components/schemas/ProceduralRevalidationRequestResponse'}}}}
        '400': {description: Invalid bounded revalidation request}
        '401': {description: Missing or invalid administrator API key}
        '404': {description: "Verification, evidence, target, or policy unavailable in the authorized scope"}
        '409': {description: Idempotent revalidation work conflict}
  /v1/admin/procedural-memory-verifications/{verification_id}:correct:
    post:
      operationId: submitAdminProceduralVerificationCorrection
      description: Submit the corrected content already recorded on a current refuted verification into the existing governed update pipeline. This operation never mutates canonical memory directly.
      parameters:
        - {name: verification_id, in: path, required: true, schema: {type: string, minLength: 1, maxLength: 128}}
        - {$ref: '#/components/parameters/AdminAPIKey'}
        - {$ref: '#/components/parameters/ActorHeader'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
        - {name: X-Request-ID, in: header, required: true, schema: {type: string, minLength: 1, maxLength: 128}}
      requestBody: {required: true, content: {application/json: {schema: {$ref: '#/components/schemas/ProceduralVerificationCorrectionRequest'}}}}
      responses:
        '202': {description: Governed correction intent accepted, content: {application/json: {schema: {$ref: '#/components/schemas/ProceduralVerificationCorrectionResponse'}}}}
        '400': {description: Invalid bounded correction request}
        '401': {description: Missing or invalid administrator API key}
        '404': {description: "Verification, evidence, target, or policy unavailable in the authorized scope"}
        '409': {description: Idempotency or immutable attribution conflict}
  /v1/admin/procedural-revalidation-runs/{run_id}:
    get:
      operationId: getAdminProceduralRevalidationRun
      description: Read bounded revalidation status without worker lease identity or internal queue work identity.
      parameters:
        - {name: run_id, in: path, required: true, schema: {type: string, minLength: 1, maxLength: 128}}
        - {$ref: '#/components/parameters/AdminAPIKey'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
      responses:
        '200': {description: Bounded revalidation run status, content: {application/json: {schema: {$ref: '#/components/schemas/ProceduralRevalidationRunResponse'}}}}
        '401': {description: Missing or invalid administrator API key}
        '404': {description: Revalidation run unavailable in the authorized scope}
`

const proceduralVerificationSchemas = `
    ProceduralVerificationEvidenceRequest:
      type: object
      required: [kind, source_kind, id, source_version]
      properties:
        kind: {type: string, enum: [tool_observation, user_correction, external_evaluation, transcript, model_assertion]}
        source_kind: {type: string, enum: [raw_event, memory_version, usefulness_feedback, external_evaluation]}
        polarity: {type: string, enum: [supporting, contrary]}
        id: {type: string, minLength: 1, maxLength: 128}
        source_version: {type: integer, minimum: 1}
        corroborates: {type: boolean}
    ProceduralVerificationRecordRequest:
      type: object
      required: [memory_id, target_version, claim_fingerprint, applicability_fingerprint, disposition, evidence, reason, source_watermark]
      properties:
        memory_id: {type: string, minLength: 1, maxLength: 128}
        target_version: {type: integer, minimum: 1}
        claim_fingerprint: {type: string, pattern: '^[a-fA-F0-9]{64}$'}
        applicability_fingerprint: {type: string, pattern: '^[a-fA-F0-9]{64}$'}
        disposition: {type: string, enum: [supported, refuted, inconclusive, needs_revalidation]}
        evidence: {type: array, minItems: 1, maxItems: 32, items: {$ref: '#/components/schemas/ProceduralVerificationEvidenceRequest'}}
        reason: {type: string, minLength: 1, maxLength: 2048}
        source_watermark: {type: string, minLength: 1, maxLength: 256}
        corrected_content: {type: string, maxLength: 16384, description: Accepted only for a refuted verification; never included in default responses.}
    ProceduralVerificationReceipt:
      type: object
      required: [id, memory_id, target_version, disposition, policy_version, source_watermark, created_at]
      properties:
        id: {type: string}
        memory_id: {type: string}
        target_version: {type: integer, minimum: 1}
        disposition: {type: string, enum: [supported, refuted, inconclusive, needs_revalidation]}
        policy_version: {type: string}
        source_watermark: {type: string}
        created_at: {type: string, format: date-time}
    ProceduralVerificationHistoryResponse:
      type: object
      required: [verifications]
      properties:
        verifications: {type: array, maxItems: 100, items: {$ref: '#/components/schemas/ProceduralVerificationReceipt'}}
    ProceduralVerificationRevalidationRequest:
      type: object
      required: [reason]
      properties: {reason: {type: string, minLength: 1, maxLength: 2048}}
    ProceduralVerificationCorrectionRequest:
      type: object
      required: [reason, operation_id, idempotency_key]
      properties:
        reason: {type: string, minLength: 1, maxLength: 2048}
        operation_id: {type: string, minLength: 1, maxLength: 128}
        idempotency_key: {type: string, minLength: 1, maxLength: 128}
    ProceduralRevalidationRequestResponse:
      type: object
      required: [id, verification_id, status, trigger, attempt, max_attempts, created_at]
      properties:
        id: {type: string}
        verification_id: {type: string}
        status: {type: string}
        trigger: {type: string, enum: [administrator]}
        attempt: {type: integer, minimum: 0}
        max_attempts: {type: integer, minimum: 1}
        created_at: {type: string, format: date-time}
    ProceduralVerificationCorrectionResponse:
      type: object
      required: [intent_id, status]
      properties:
        intent_id: {type: string}
        status: {type: string}
    ProceduralRevalidationRunResponse:
      type: object
      required: [id, verification_id, memory_id, target_version, source_watermark, trigger, status, attempt, max_attempts, created_at]
      properties:
        id: {type: string}
        verification_id: {type: string}
        memory_id: {type: string}
        target_version: {type: integer, minimum: 1}
        source_watermark: {type: string}
        trigger: {type: string, enum: [administrator, source_drift, correction_signal, stale_feedback, needs_review_feedback]}
        status: {type: string, enum: [pending, running, completed, failed, exhausted]}
        attempt: {type: integer, minimum: 0}
        max_attempts: {type: integer, minimum: 1, maximum: 100}
        terminal_verification_id: {type: string}
        failure_category: {type: string}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
`
