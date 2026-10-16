package openapi

import "strings"

// Both operations are additive. Legacy turn/outcome routes are not aliases.
func providerProjectionPaths() string {
	var out strings.Builder
	for _, op := range []struct{ path, id, request, response, code, description string }{
		{"/v1/provider/projections", "providerAdmitProjection", "ProviderProjectionRequest", "ProviderProjectionResponse", "202", "Append bounded runtime-reported evidence; equivalent normalized delivery returns the original identity and disposition. Greater revisions append corrections. Admission never changes task, lease, commit, publication, delivery, ranking or insight state."},
		{"/v1/provider/projections/history", "providerProjectionHistory", "ProviderProjectionHistoryRequest", "ProviderProjectionHistoryResponse", "200", "Read newest-first binding-owned evidence history under current grants and lifecycle. Missing and foreign effects share an empty page. Unavailable references omit all identities. has_more indicates truncation, not a complete ledger."},
	} {
		out.WriteString("  " + op.path + ":\n    post:\n      operationId: " + op.id + "\n      description: " + op.description + " Request body maximum 32768 bytes; serialized response maximum 65536 bytes.\n      security:\n        - PublicAPIKey: []\n      parameters:\n")
		for _, p := range []string{"RuntimeBindingHeader", "RuntimeSessionHeader", "TenantHeader", "ProjectHeader", "NamespaceHeader"} {
			out.WriteString("        - {$ref: '#/components/parameters/" + p + "'}\n")
		}
		out.WriteString("      requestBody:\n        required: true\n        content:\n          application/json:\n            schema: {$ref: '#/components/schemas/" + op.request + "'}\n      responses:\n        '" + op.code + "':\n          description: Bounded runtime-reported evidence, never business-state authority\n          content:\n            application/json:\n              schema: {$ref: '#/components/schemas/" + op.response + "'}\n")
		for _, e := range []struct{ code, description string }{{"400", "validation; invalid_request, invalid_scope or invalid_projection_scope; malformed, null, duplicate, unknown, authority, alias or out-of-bounds input"}, {"403", "authentication/unauthorized or scope/scope_denied, runtime_binding_denied or disclosure_denied; no foreign existence disclosure"}, {"409", "compatibility/unsupported_projection_contract, unsupported_schema, conflict/projection_conflict or stale/projection_stale; no duplicate effects"}, {"410", "lifecycle/projection_unavailable; linked evidence or binding unavailable without existence details"}, {"413", "validation/projection_limit_exceeded; body exceeds 32768 bytes"}, {"503", "dependency; projection_dependency_unavailable, authorization_unavailable or disclosure_unavailable; bounded retryable dependency failure"}} {
			out.WriteString("        '" + e.code + "':\n          description: " + e.description + "; closed error response maximum 4096 bytes\n          content:\n            application/json:\n              schema: {$ref: '#/components/schemas/ProviderProjectionErrorResponse'}\n")
		}
	}
	return out.String()
}

const providerProjectionSchemas = `
    ProviderProjectionErrorResponse:
      type: object
      additionalProperties: false
      required: [error]
      properties:
        error: {$ref: '#/components/schemas/ProviderProjectionError'}
    ProviderProjectionError:
      type: object
      additionalProperties: false
      required: [category, code, message, retryable]
      description: PC6 errors are limited to 4096 serialized bytes. Message is nonempty and at most 256 UTF-8 bytes. HTTP 400 uses validation/invalid_request, invalid_scope or invalid_projection_scope; 403 authentication/unauthorized or scope/scope_denied, runtime_binding_denied or disclosure_denied; 409 compatibility/unsupported_projection_contract or unsupported_schema, conflict/projection_conflict or stale/projection_stale; 410 lifecycle/projection_unavailable; 413 validation/projection_limit_exceeded; 503 dependency/projection_dependency_unavailable, authorization_unavailable or disclosure_unavailable. Only dependency errors are retryable. The strict consumer additionally checks HTTP status and UTF-8 byte bounds.
      properties:
        category: {type: string, enum: [authentication, scope, validation, compatibility, conflict, stale, lifecycle, dependency]}
        code: {type: string, minLength: 1, maxLength: 64, pattern: '^[A-Za-z0-9_.:-]+$'}
        message: {type: string, minLength: 1, maxLength: 256}
        retryable: {type: boolean}
      oneOf:
        - properties: {category: {enum: [authentication]}, code: {enum: [unauthorized]}, retryable: {enum: [false]}}
        - properties: {category: {enum: [scope]}, code: {enum: [scope_denied, runtime_binding_denied, disclosure_denied]}, retryable: {enum: [false]}}
        - properties: {category: {enum: [validation]}, code: {enum: [invalid_request, invalid_scope, invalid_projection_scope, projection_limit_exceeded]}, retryable: {enum: [false]}}
        - properties: {category: {enum: [compatibility]}, code: {enum: [unsupported_projection_contract, unsupported_schema]}, retryable: {enum: [false]}}
        - properties: {category: {enum: [conflict]}, code: {enum: [projection_conflict]}, retryable: {enum: [false]}}
        - properties: {category: {enum: [stale]}, code: {enum: [projection_stale]}, retryable: {enum: [false]}}
        - properties: {category: {enum: [lifecycle]}, code: {enum: [projection_unavailable]}, retryable: {enum: [false]}}
        - properties: {category: {enum: [dependency]}, code: {enum: [projection_dependency_unavailable, authorization_unavailable, disclosure_unavailable]}, retryable: {enum: [true]}}
    ProviderProjectionOperationMetadata:
      type: object
      additionalProperties: false
      description: Canonical PC6 metadata; tokens and paths must already be normalized. The serialized typed metadata object is limited to 4096 bytes, including JSON escaping, and memory paths retain their 512-byte total and 128-byte segment limits. These aggregate byte and path-normalization constraints are additionally checked by the producer and strict consumer.
      required: [request_id, operation_id, schema_version]
      properties:
        request_id: {type: string, minLength: 1, maxLength: 256, pattern: '^[A-Za-z0-9_.:-]+$'}
        operation_id: {type: string, minLength: 1, maxLength: 256, pattern: '^[A-Za-z0-9_.:-]+$'}
        idempotency_key: {type: string, maxLength: 256, pattern: '^[A-Za-z0-9_.:-]*$'}
        event_seq: {type: integer, minimum: 0, maximum: 9223372036854775807}
        schema_version: {type: string, minLength: 1, maxLength: 64, pattern: '^[A-Za-z0-9_.:-]+$'}
        memory_path: {type: string, maxLength: 512}
        precedence_version: {type: string, maxLength: 4096}
        precedence_stage: {type: string, maxLength: 4096}
        precedence_outcome: {type: string, maxLength: 4096}
    ProviderProjectionUUID:
      type: string
      format: uuid
      minLength: 36
      maxLength: 36
      pattern: '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
      not: {enum: ['00000000-0000-0000-0000-000000000000']}
    ProviderProjectionRequest:
      type: object
      additionalProperties: false
      required: [metadata, input]
      properties:
        metadata:
          allOf:
            - {$ref: '#/components/schemas/ProviderProjectionOperationMetadata'}
            - {type: object, required: [idempotency_key], properties: {idempotency_key: {type: string, minLength: 1, maxLength: 256, pattern: '^[A-Za-z0-9_.:-]+$'}}}
        input: {$ref: '#/components/schemas/ProviderProjectionInput'}
    ProviderProjectionHistoryRequest:
      type: object
      additionalProperties: false
      required: [metadata, input]
      properties:
        metadata: {$ref: '#/components/schemas/ProviderProjectionOperationMetadata'}
        input: {$ref: '#/components/schemas/ProviderProjectionHistoryInput'}
    ProviderProjectionInput:
      type: object
      additionalProperties: false
      required: [contract_version, kind, effect_id, source_revision, occurred_at, reported_outcome]
      properties:
        contract_version: {type: string, enum: [turn-outcome-feedback-projection-v1]}
        kind: {type: string, enum: [turn, outcome, feedback]}
        effect_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        source_revision: {type: integer, minimum: 1, maximum: 2147483647}
        occurred_at: {type: string, format: date-time, minLength: 20, maxLength: 20, pattern: '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$', not: {enum: ['0001-01-01T00:00:00Z']}}
        reported_outcome: {type: string, enum: [unknown, succeeded, failed, cancelled]}
        session_run_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        turn_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        context_digest: {type: string, minLength: 64, maxLength: 64, pattern: '^[0-9a-f]{64}$'}
        context_reference: {type: string, minLength: 37, maxLength: 37, pattern: '^ctxc_[0-9a-f]{32}$'}
        references: {type: array, maxItems: 8, uniqueItems: true, items: {$ref: '#/components/schemas/ProviderProjectionReferenceSelector'}}
        feedback: {$ref: '#/components/schemas/ProviderProjectionFeedback'}
        provenance: {type: array, maxItems: 8, uniqueItems: true, items: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}}
      allOf:
        - anyOf:
            - {not: {required: [turn_id]}}
            - {required: [session_run_id]}
        - anyOf:
            - {not: {required: [context_reference]}}
            - {required: [context_digest]}
        - anyOf:
            - {not: {required: [feedback]}}
            - {required: [references], properties: {references: {type: array, minItems: 1}}}
        - anyOf:
            - {properties: {kind: {enum: [turn, outcome]}}}
            - {required: [feedback]}
    ProviderProjectionReferenceSelector:
      type: object
      additionalProperties: false
      required: [event_id]
      properties:
        event_id: {$ref: '#/components/schemas/ProviderProjectionUUID'}
    ProviderProjectionFeedback:
      type: object
      additionalProperties: false
      required: [signal, reason_code]
      properties:
        signal: {type: string, enum: [useful, irrelevant, needs_review]}
        reason_code: {type: string, minLength: 1, maxLength: 64, pattern: '^[A-Za-z0-9_.:-]+$'}
    ProviderProjectionHistoryInput:
      type: object
      additionalProperties: false
      required: [contract_version, kind, effect_id, limit]
      properties:
        contract_version: {type: string, enum: [turn-outcome-feedback-projection-v1]}
        kind: {type: string, enum: [turn, outcome, feedback]}
        effect_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        limit: {type: integer, minimum: 1, maximum: 8}
    ProviderProjectionLineage:
      type: object
      additionalProperties: false
      required: [candidate_id, memory_id, version_id, provenance_id, version]
      properties:
        candidate_id: {$ref: '#/components/schemas/ProviderProjectionUUID'}
        memory_id: {$ref: '#/components/schemas/ProviderProjectionUUID'}
        version_id: {$ref: '#/components/schemas/ProviderProjectionUUID'}
        provenance_id: {$ref: '#/components/schemas/ProviderProjectionUUID'}
        version: {type: integer, minimum: 1, maximum: 9223372036854775807}
    ProviderProjectionAttribution:
      type: object
      additionalProperties: false
      required: [state, eligible]
      properties:
        event_id: {$ref: '#/components/schemas/ProviderProjectionUUID'}
        state: {type: string, enum: [accepted, pending, completed, failed, unavailable]}
        eligible: {type: boolean}
        lineage: {$ref: '#/components/schemas/ProviderProjectionLineage'}
      oneOf:
        - required: [event_id, lineage]
          properties: {state: {enum: [completed]}, eligible: {enum: [true]}}
        - required: [event_id]
          properties: {state: {enum: [accepted, pending, failed]}, eligible: {enum: [false]}}
          not: {required: [lineage]}
        - properties: {state: {enum: [unavailable]}, eligible: {enum: [false]}}
          not:
            anyOf:
              - {required: [event_id]}
              - {required: [lineage]}
    ProviderProjectionRecord:
      type: object
      additionalProperties: false
      required: [projection_id, logical_id, source_revision, disposition, evidence_kind, references, current_references, feedback_state, context_digest_attribution]
      properties:
        projection_id: {$ref: '#/components/schemas/ProviderProjectionUUID'}
        logical_id: {$ref: '#/components/schemas/ProviderProjectionUUID'}
        source_revision: {type: integer, minimum: 1, maximum: 2147483647}
        supersedes_id: {$ref: '#/components/schemas/ProviderProjectionUUID'}
        disposition: {type: string, enum: [accepted, corrected]}
        evidence_kind: {type: string, enum: [runtime_reported_evidence]}
        references: {type: array, maxItems: 8, items: {$ref: '#/components/schemas/ProviderProjectionAttribution'}}
        current_references: {type: array, maxItems: 8, items: {$ref: '#/components/schemas/ProviderProjectionAttribution'}}
        feedback_state: {type: string, enum: [none, pending], description: Admission only; pending never claims materialized feedback or ranking/insight activation.}
        context_digest_attribution: {type: string, enum: [none, reported, verified_receipt], description: verified_receipt only attests a same-binding live matching PC4 receipt at admission; never completion or current access.}
      oneOf:
        - properties: {disposition: {enum: [accepted]}}
          not: {required: [supersedes_id]}
        - required: [supersedes_id]
          properties: {disposition: {enum: [corrected]}}
    ProviderProjectionResponse:
      type: object
      additionalProperties: false
      required: [metadata, contract_version, scope, record]
      properties:
        metadata: {$ref: '#/components/schemas/ProviderProjectionOperationMetadata'}
        contract_version: {type: string, enum: [turn-outcome-feedback-projection-v1]}
        scope: {$ref: '#/components/schemas/CanonicalScopeDescriptor'}
        record: {$ref: '#/components/schemas/ProviderProjectionRecord'}
    ProviderProjectionHistoryResponse:
      type: object
      additionalProperties: false
      required: [metadata, contract_version, scope, records, has_more]
      properties:
        metadata: {$ref: '#/components/schemas/ProviderProjectionOperationMetadata'}
        contract_version: {type: string, enum: [turn-outcome-feedback-projection-v1]}
        scope: {$ref: '#/components/schemas/CanonicalScopeDescriptor'}
        records: {type: array, maxItems: 8, items: {$ref: '#/components/schemas/ProviderProjectionRecord'}}
        has_more: {type: boolean}
    ProviderProjectionCapabilities:
      type: object
      additionalProperties: false
      required: [enabled, contract_version, operations, kinds, dispositions, reference_states, error_categories, max_request_bytes, max_response_bytes, max_references, max_history, feedback_policy_version]
      properties:
        enabled: {type: boolean}
        contract_version: {type: string, enum: [turn-outcome-feedback-projection-v1]}
        operations: {type: array, minItems: 2, maxItems: 2, uniqueItems: true, items: {type: string, enum: [providerAdmitProjection, providerProjectionHistory]}}
        kinds: {type: array, minItems: 3, maxItems: 3, uniqueItems: true, items: {type: string, enum: [turn, outcome, feedback]}}
        dispositions: {type: array, minItems: 2, maxItems: 2, uniqueItems: true, items: {type: string, enum: [accepted, corrected]}}
        reference_states: {type: array, minItems: 5, maxItems: 5, uniqueItems: true, items: {type: string, enum: [accepted, pending, completed, failed, unavailable]}}
        error_categories: {type: array, minItems: 8, maxItems: 8, uniqueItems: true, items: {type: string, enum: [authentication, scope, validation, compatibility, conflict, stale, lifecycle, dependency]}}
        max_request_bytes: {type: integer, enum: [32768]}
        max_response_bytes: {type: integer, enum: [65536]}
        max_references: {type: integer, enum: [8]}
        max_history: {type: integer, enum: [8]}
        feedback_policy_version: {type: string, enum: [projection-feedback-v1]}
`
