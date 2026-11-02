package openapi

const providerCorrectionPaths = `  /v1/provider/corrections:
    post:
      operationId: providerSubmitCorrectionUpdate
      description: Submit one server-resolved projection correction as an ordinary asynchronously governed update. The correction source and PC5 target lineage are resolved under the current exact binding; no projection payload or lineage is returned.
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
            schema: {$ref: '#/components/schemas/ProviderCorrectionRequest'}
      responses:
        '202':
          description: Governed update intent accepted; canonical mutation remains asynchronous.
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ProviderCorrectionResponse'}
        '400': {description: 'Bounded invalid request or unsupported Provider schema'}
        '403': {description: 'Authentication, runtime binding or current disclosure denied without resource-existence detail'}
        '409': {description: 'Idempotency conflict or stale correction/target attribution'}
        '410': {description: 'Correction source unavailable without resource-existence detail'}
        '413': {description: 'Request exceeds the advertised byte bound'}
        '503': {description: 'Retryable authorization, storage or disclosure dependency failure'}
`

const providerCorrectionSchemas = `
    ProviderCorrectionCapabilities:
      type: object
      additionalProperties: false
      required: [enabled, contract_version, operation, max_request_bytes, max_content_bytes, max_reason_bytes, result_categories]
      properties:
        enabled: {type: boolean}
        contract_version: {type: string, enum: [provider-correction-governed-version-v1]}
        operation: {type: string, enum: ['POST /v1/provider/corrections']}
        max_request_bytes: {type: integer, enum: [32768]}
        max_content_bytes: {type: integer, enum: [16384]}
        max_reason_bytes: {type: integer, enum: [512]}
        result_categories: {type: array, minItems: 5, maxItems: 5, uniqueItems: true, items: {type: string, enum: [accepted, unavailable, stale, conflict, dependency]}}
    ProviderCorrectionUUID:
      type: string
      format: uuid
      minLength: 36
      maxLength: 36
      pattern: '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
      not: {enum: ['00000000-0000-0000-0000-000000000000']}
    ProviderCorrectionInput:
      type: object
      additionalProperties: false
      required: [contract_version, projection_id, target_memory_id, expected_version, content, reason]
      properties:
        contract_version: {type: string, enum: [provider-correction-governed-version-v1]}
        projection_id: {$ref: '#/components/schemas/ProviderCorrectionUUID'}
        target_memory_id: {$ref: '#/components/schemas/ProviderCorrectionUUID'}
        expected_version: {type: integer, minimum: 1, maximum: 9223372036854775807}
        content: {type: string, minLength: 1, maxLength: 16384}
        reason: {type: string, minLength: 1, maxLength: 512}
    ProviderCorrectionRequest:
      type: object
      additionalProperties: false
      required: [metadata, input]
      properties:
        metadata:
          allOf:
            - {$ref: '#/components/schemas/ProviderProjectionOperationMetadata'}
            - {type: object, required: [idempotency_key], properties: {idempotency_key: {type: string, minLength: 1, maxLength: 256}}}
        input: {$ref: '#/components/schemas/ProviderCorrectionInput'}
    ProviderCorrectionResult:
      type: object
      additionalProperties: false
      required: [intent_id, status, attribution_state]
      properties:
        intent_id: {$ref: '#/components/schemas/ProviderCorrectionUUID'}
        status: {type: string, enum: [accepted, replayed]}
        attribution_state: {type: string, enum: [verified]}
    ProviderCorrectionResponse:
      type: object
      additionalProperties: false
      required: [metadata, contract_version, result]
      properties:
        metadata: {$ref: '#/components/schemas/ProviderProjectionOperationMetadata'}
        contract_version: {type: string, enum: [provider-correction-governed-version-v1]}
        result: {$ref: '#/components/schemas/ProviderCorrectionResult'}
`
