package openapi

const disclosureSchemas = `    DisclosureEntitlements:
      type: object
      additionalProperties: false
      required: [allowed_profiles, default_profile, source_attestation]
      description: Administrator-owned exact-scope disclosure ceiling. The default must belong to the allowed profiles; source attestation conveys no instruction authority.
      properties:
        allowed_profiles:
          type: array
          minItems: 1
          maxItems: 2
          uniqueItems: true
          items: {type: string, enum: [memory-reference-v1, memory-content-v1]}
        default_profile: {type: string, enum: [memory-reference-v1, memory-content-v1]}
        source_attestation: {type: boolean, default: false}
    GrantDisclosureRequest:
      type: object
      additionalProperties: false
      required: [disclosure, actor, reason]
      properties:
        disclosure: {$ref: '#/components/schemas/DisclosureEntitlements'}
        actor: {type: string, minLength: 1, maxLength: 128}
        reason: {type: string, minLength: 1, maxLength: 512}
    SourceMetadata:
      type: object
      additionalProperties: false
      required: [version, trust, classification]
      description: Missing source metadata resolves to untrusted/restricted. Attested data verifies authorized source attribution, never correctness or instruction/tool/write authority.
      properties:
        version: {type: string, enum: [source-metadata-v1]}
        trust: {type: string, enum: [untrusted, attested_data]}
        classification: {type: string, enum: [low, medium, high, restricted]}
`

const disclosurePaths = `  /v1/admin/principals/{principal_id}/grants/{grant_id}/disclosure:
    put:
      operationId: updateScopeGrantDisclosure
      summary: Provision or narrow the current exact-scope disclosure ceiling
      security: [{AdminAPIKey: []}]
      parameters:
        - {$ref: '#/components/parameters/AdminAPIKey'}
        - {$ref: '#/components/parameters/TenantHeader'}
        - {$ref: '#/components/parameters/ProjectHeader'}
        - {$ref: '#/components/parameters/NamespaceHeader'}
        - {name: principal_id, in: path, required: true, schema: {type: string}}
        - {name: grant_id, in: path, required: true, schema: {type: string}}
      requestBody:
        required: true
        content:
          application/json:
            schema: {$ref: '#/components/schemas/GrantDisclosureRequest'}
      responses:
        '204': {description: Grant updated and actor/reason audit persisted atomically}
        '400': {description: Invalid closed request or disclosure configuration}
        '403': {description: Administrator or exact-scope grant denied}
        '503': {description: Authoritative grant store unavailable}
`
