package openapi

const providerContextSchemas = `    ProviderContextInput:
      type: object
      additionalProperties: false
      required: [query, budget]
      description: Exact snake_case fields only. Scope and session come from the authenticated runtime binding. Path selectors are mutually exclusive and use shared memory-path normalization. Budget counts returned item placements. Optional limits allocate UTF-8 content bytes, exact complete JSON body bytes and unique canonical citation identities independently; omitted fields use server ceilings. Explicit nulls, duplicates, aliases, fractional or overflowing integers, unknown/token fields and trailing JSON are invalid. Bytes are never token estimates. Optional public insight, goal and diagnostic sections remain unsupported.
      properties:
        turn_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        continuity: {$ref: '#/components/schemas/ProviderContextContinuityInput'}
        projection_intent: {type: string, enum: [conversation, task_context, compilation, handoff], default: conversation}
        disclosure_profile: {type: string, maxLength: 64, pattern: '^[A-Za-z0-9_.:-]+$'}
        response_mode: {type: string, enum: [reference_only, bounded_content], default: reference_only}
        query: {type: string, minLength: 1, pattern: '\S'}
        budget: {type: integer, minimum: 1, maximum: 2147483647}
        budget_limits: {$ref: '#/components/schemas/ProviderContextBudgetLimits'}
        path: {type: string, maxLength: 512}
        path_prefix: {type: string, maxLength: 512}
        include_relations: {type: boolean, default: false}
        include_experience_insights: {type: boolean, default: false}
        include_goal_context: {type: boolean, default: false}
        include_diagnostics: {type: boolean, default: false}
        include_feedback_diagnostics: {type: boolean, default: false}
        feedback_aware_ranking: {type: boolean, default: false}
        feedback_ranking_policy: {type: string, enum: [''], description: Rejection-only compatibility field; every nonempty value is rejected.}
      examples:
        - {query: 'current task', budget: 4}
        - {query: 'current task', budget: 4, continuity: {mode: issue}}
        - {query: 'current task', budget: 4, continuity: {mode: compare, reference: ctxc_0123456789abcdef0123456789abcdef}}
        - {query: 'current task', budget: 4, budget_limits: {max_content_bytes: 0, max_response_bytes: 8192, max_citations: 4}}
        - {query: 'current task', budget: 8, path_prefix: 'tasks/demo', include_relations: true, include_experience_insights: true, include_goal_context: true, include_diagnostics: true, include_feedback_diagnostics: true, feedback_aware_ranking: true, feedback_ranking_policy: ''}
    ProviderContextDigest:
      type: object
      additionalProperties: false
      required: [contract_version, algorithm, canonicalization_version, selection_policy_version, selection_policy_fingerprint, value]
      description: Domain-separated SHA-256 of the final authorized semantic projection. Scope comes from the authenticated binding. This digest is neither a credential, a snapshot nor a business-state receipt. See the canonical byte vectors and escaping rules in the Provider integration guide.
      properties:
        contract_version: {type: string, enum: [context-digest-v1, context-turn-digest-v1]}
        algorithm: {type: string, enum: [sha-256]}
        canonicalization_version: {type: string, enum: [context-canonical-v1, context-turn-canonical-v1]}
        selection_policy_version: {type: string, enum: [context-selection-v1]}
        selection_policy_fingerprint: {type: string, pattern: '^[0-9a-f]{64}$'}
        value: {type: string, pattern: '^[0-9a-f]{64}$'}
    ProviderContextContinuityInput:
      description: Optional durable metadata-only continuity. Nulls, duplicate keys, aliases, unknown fields and invalid Unicode are rejected before assembly. Tokens are opaque references, never credentials.
      oneOf:
        - type: object
          additionalProperties: false
          required: [mode]
          properties:
            mode: {type: string, enum: [issue]}
        - type: object
          additionalProperties: false
          required: [mode, reference]
          properties:
            mode: {type: string, enum: [compare]}
            reference: {type: string, pattern: '^ctxc_[0-9a-f]{32}$', minLength: 37, maxLength: 37}
    ProviderContextContinuityResult:
      description: Every success is a complete fresh read. Comparison never returns historical bodies, 304, exact snapshot replay or a context watermark. Expiry is UTC RFC3339 seconds. No receipt write occurs when continuity is omitted.
      oneOf:
        - type: object
          additionalProperties: false
          required: [contract_version, read_semantics, outcome]
          properties:
            contract_version: {type: string, enum: [context-continuity-v1]}
            read_semantics: {type: string, enum: [fresh]}
            outcome: {type: string, enum: [not_requested]}
        - type: object
          additionalProperties: false
          required: [contract_version, read_semantics, outcome, reference, expires_at]
          properties:
            contract_version: {type: string, enum: [context-continuity-v1]}
            read_semantics: {type: string, enum: [fresh]}
            outcome: {type: string, enum: [issued]}
            reference: {type: string, pattern: '^ctxc_[0-9a-f]{32}$', minLength: 37, maxLength: 37}
            expires_at: {type: string, format: date-time, pattern: '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$', minLength: 20, maxLength: 20}
        - type: object
          additionalProperties: false
          required: [contract_version, read_semantics, outcome, reference, expires_at]
          properties:
            contract_version: {type: string, enum: [context-continuity-v1]}
            read_semantics: {type: string, enum: [fresh_compare]}
            outcome: {type: string, enum: [unchanged, changed]}
            reference: {type: string, pattern: '^ctxc_[0-9a-f]{32}$', minLength: 37, maxLength: 37}
            expires_at: {type: string, format: date-time, pattern: '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$', minLength: 20, maxLength: 20}
    ProviderContextDigestCapabilities:
      type: object
      additionalProperties: false
      required: [contract_version, algorithm, canonicalization_version, selection_policy_version]
      properties:
        contract_version: {type: string, enum: [context-digest-v1, context-turn-digest-v1]}
        algorithm: {type: string, enum: [sha-256]}
        canonicalization_version: {type: string, enum: [context-canonical-v1, context-turn-canonical-v1]}
        selection_policy_version: {type: string, enum: [context-selection-v1]}
    ProviderContextContinuityCapabilities:
      type: object
      additionalProperties: false
      required: [enabled, contract_version, fresh_compare, max_reference_bytes, ttl_seconds, exact_snapshot_replay, context_watermark]
      properties:
        enabled: {type: boolean}
        contract_version: {type: string, enum: [context-continuity-v1]}
        fresh_compare: {type: boolean}
        max_reference_bytes: {type: integer, enum: [37]}
        ttl_seconds: {type: integer, minimum: 60, maximum: 604800, default: 86400}
        exact_snapshot_replay: {type: boolean, enum: [false]}
        context_watermark: {type: boolean, enum: [false]}
    ProviderContextTurnBindingCapabilities:
      type: object
      additionalProperties: false
      required: [enabled, contract_version, digest_contract_version, canonicalization_version]
      properties:
        enabled: {type: boolean}
        contract_version: {type: string, enum: [context-turn-binding-v1]}
        digest_contract_version: {type: string, enum: [context-turn-digest-v1]}
        canonicalization_version: {type: string, enum: [context-turn-canonical-v1]}
    ProviderContextBudgetLimits:
      type: object
      additionalProperties: false
      description: Empty object is valid. Omitted limits inherit the corresponding published server ceiling. Explicit zero content or citations is preserved without changing response mode. Request nulls are invalid. Each whole item and its required citations must fit all effective limits together.
      properties:
        max_content_bytes: {type: integer, minimum: 0, maximum: 2147483647, description: Sum of unescaped UTF-8 bytes of returned content strings. Reference-only mode consumes zero content bytes.}
        max_response_bytes: {type: integer, minimum: 1, maximum: 2147483647, description: Exact uncompressed JSON body bytes including metadata and accounting; excludes headers/framing and has no appended newline.}
        max_citations: {type: integer, minimum: 0, maximum: 2147483647, description: Unique canonical citation identities by kind/id/version. Repeated serialized citations still consume response bytes.}
    ProviderContextBudgetAccounting:
      type: object
      additionalProperties: false
      required: [contract_version, packing_policy_version, requested, effective, usage, selection]
      description: Mandatory safe final-output accounting in both modes, independently of include_diagnostics. Assembly admits whole authorized items in existing order with a deterministic conservative accounting reserve; maximal packing is not guaranteed. Usage measures the actual emitted response. No hidden evidence counts or privileged ranking state are exposed.
      properties:
        contract_version: {type: string, enum: [context-budget-v1]}
        packing_policy_version: {type: string, enum: [context-packing-v1]}
        requested: {$ref: '#/components/schemas/ProviderContextRequestedBudget'}
        effective: {$ref: '#/components/schemas/ProviderContextEffectiveBudget'}
        usage: {$ref: '#/components/schemas/ProviderContextBudgetUsage'}
        selection: {$ref: '#/components/schemas/ProviderContextBudgetSelection'}
      examples:
        - contract_version: context-budget-v1
          packing_policy_version: context-packing-v1
          requested: {max_items: 4, max_content_bytes: null, max_response_bytes: null, max_citations: null}
          effective: {max_items: 4, max_content_bytes: 1048576, max_response_bytes: 1048576, max_citations: 100}
          usage: {items: 0, content_bytes: 0, response_bytes: 1024, citations: 0}
          selection: {outcome: empty, omission_reasons: [], unsupported_sections: []}
    ProviderContextRequestedBudget:
      type: object
      additionalProperties: false
      required: [max_items, max_content_bytes, max_response_bytes, max_citations]
      description: Original caller allocation. Optional request fields are present as response-only null markers when omitted; an explicit zero is reported as zero.
      properties:
        max_items: {type: integer, minimum: 1, maximum: 2147483647}
        max_content_bytes: {type: [integer, 'null'], minimum: 0, maximum: 2147483647}
        max_response_bytes: {type: [integer, 'null'], minimum: 1, maximum: 2147483647}
        max_citations: {type: [integer, 'null'], minimum: 0, maximum: 2147483647}
    ProviderContextEffectiveBudget:
      type: object
      additionalProperties: false
      required: [max_items, max_content_bytes, max_response_bytes, max_citations]
      description: Independent minima of caller allocation and resolved server ceilings, applied before retrieval fanout arithmetic.
      properties:
        max_items: {type: integer, minimum: 1, maximum: 1000}
        max_content_bytes: {type: integer, minimum: 0, maximum: 4194304}
        max_response_bytes: {type: integer, minimum: 1, maximum: 4194304}
        max_citations: {type: integer, minimum: 0, maximum: 1000}
    ProviderContextBudgetUsage:
      type: object
      additionalProperties: false
      required: [items, content_bytes, response_bytes, citations]
      description: Actual returned usage must not exceed its corresponding effective allocation. Item appearances and content count across all categories; unique citations count canonical kind/id/version identities. Response bytes equal the exact emitted body length, including this accounting block and repeated citation arrays.
      properties:
        items: {type: integer, minimum: 0, maximum: 1000}
        content_bytes: {type: integer, minimum: 0, maximum: 4194304}
        response_bytes: {type: integer, minimum: 1, maximum: 4194304}
        citations: {type: integer, minimum: 0, maximum: 1000}
    ProviderContextBudgetSelection:
      type: object
      additionalProperties: false
      required: [outcome, omission_reasons, unsupported_sections]
      properties:
        outcome:
          type: string
          enum: [selected, empty, budget_exhausted]
          description: Selected means at least one item returned, even with omissions. Empty means the bounded stream had no disclosable candidates. Budget-exhausted means eligible candidates existed but no whole item fit. Dependency failures are non-success errors.
        omission_reasons:
          type: array
          maxItems: 4
          uniqueItems: true
          description: Only evaluated disclosable candidates contribute reasons. Entries follow item_limit, content_bytes, citations, response_bytes order and expose no rejected-source counts, identity or text.
          items: {type: string, enum: [item_limit, content_bytes, citations, response_bytes]}
        unsupported_sections:
          type: array
          maxItems: 4
          uniqueItems: true
          description: Only explicitly requested unsupported public capabilities appear, ordered experience_insights, goal_context, diagnostics, feedback_diagnostics. Labels describe service capability, not stored evidence.
          items: {type: string, enum: [experience_insights, goal_context, diagnostics, feedback_diagnostics]}
    ProviderContextResponse:
      type: object
      additionalProperties: false
      required: [metadata, result, citations]
      properties:
        metadata: {$ref: '#/components/schemas/ProviderOperationMetadata'}
        result: {$ref: '#/components/schemas/ProviderContextResult'}
        citations: {type: array, items: {$ref: '#/components/schemas/ProviderContextSourceCitation'}}
      examples:
        - metadata: {request_id: req-reference, operation_id: providerAssembleContext, schema_version: provider-v1}
          result:
            context_digest: {contract_version: context-digest-v1, algorithm: sha-256, canonicalization_version: context-canonical-v1, selection_policy_version: context-selection-v1, selection_policy_fingerprint: "55dcb1869c32b2b972fa0f628fa5a948457403cf9da803267a4297574c4dfac2", value: "e150aeaab65c4c28303cd9b5f215770e3a374f8127fb1f11b5613aafe5e36a78"}
            continuity: {contract_version: context-continuity-v1, read_semantics: fresh, outcome: not_requested}
            response_mode: reference_only
            disclosure: {profile: memory-reference-v1, policy_version: context-disclosure-v1, projection_intent: conversation, source_metadata_version: source-metadata-v1}
            budget_accounting: {contract_version: context-budget-v1, packing_policy_version: context-packing-v1, requested: {max_items: 1, max_content_bytes: null, max_response_bytes: null, max_citations: null}, effective: {max_items: 1, max_content_bytes: 1048576, max_response_bytes: 1048576, max_citations: 100}, usage: {items: 0, content_bytes: 0, response_bytes: 1387, citations: 0}, selection: {outcome: empty, omission_reasons: [], unsupported_sections: []}}
            profile: []
            recent_session: []
            recent_episodes: []
            relevant_summaries: []
            related_entities: []
            citations: []
          citations: []
        - metadata: {request_id: req-content, operation_id: providerAssembleContext, schema_version: provider-v1}
          result:
            context_digest: {contract_version: context-digest-v1, algorithm: sha-256, canonicalization_version: context-canonical-v1, selection_policy_version: context-selection-v1, selection_policy_fingerprint: "55dcb1869c32b2b972fa0f628fa5a948457403cf9da803267a4297574c4dfac2", value: "e150aeaab65c4c28303cd9b5f215770e3a374f8127fb1f11b5613aafe5e36a78"}
            continuity: {contract_version: context-continuity-v1, read_semantics: fresh, outcome: not_requested}
            response_mode: bounded_content
            disclosure: {profile: memory-content-v1, policy_version: context-disclosure-v1, projection_intent: conversation, source_metadata_version: source-metadata-v1}
            budget_accounting: {contract_version: context-budget-v1, packing_policy_version: context-packing-v1, requested: {max_items: 1, max_content_bytes: 0, max_response_bytes: 65536, max_citations: 1}, effective: {max_items: 1, max_content_bytes: 0, max_response_bytes: 65536, max_citations: 1}, usage: {items: 0, content_bytes: 0, response_bytes: 1369, citations: 0}, selection: {outcome: empty, omission_reasons: [], unsupported_sections: []}}
            profile: []
            recent_session: []
            recent_episodes: []
            relevant_summaries: []
            related_entities: []
            citations: []
          citations: []
    ProviderContextResult:
      oneOf:
        - {$ref: '#/components/schemas/ProviderReferenceContextResult'}
        - {$ref: '#/components/schemas/ProviderContentContextResult'}
      discriminator:
        propertyName: response_mode
        mapping:
          reference_only: '#/components/schemas/ProviderReferenceContextResult'
          bounded_content: '#/components/schemas/ProviderContentContextResult'
    ContextDisclosureDecision:
      type: object
      additionalProperties: false
      required: [profile, policy_version, projection_intent, source_metadata_version]
      properties:
        profile: {type: string, enum: [memory-reference-v1, memory-content-v1]}
        policy_version: {type: string, enum: [context-disclosure-v1]}
        projection_intent: {type: string, enum: [conversation, task_context, compilation, handoff]}
        source_metadata_version: {type: string, enum: [source-metadata-v1]}
    ProviderContextReference:
      type: object
      additionalProperties: false
      required: [kind, id, version, memory_path, class, availability, source_metadata]
      properties:
        kind: {type: string, enum: [memory]}
        id: {type: string, minLength: 1}
        version: {type: string, pattern: '^[1-9][0-9]*$'}
        memory_path: {type: string, minLength: 1, maxLength: 512}
        class: {type: string, enum: [profile, episodic, procedural, summary, relation]}
        availability: {type: string, enum: [available]}
        source_metadata: {$ref: '#/components/schemas/SourceMetadata'}
    ProviderContextMemory:
      type: object
      additionalProperties: false
      required: [kind, id, version, memory_path, class, availability, source_metadata, content]
      properties:
        kind: {type: string, enum: [memory]}
        id: {type: string, minLength: 1}
        version: {type: string, pattern: '^[1-9][0-9]*$'}
        memory_path: {type: string, minLength: 1, maxLength: 512}
        class: {type: string, enum: [profile, episodic, procedural, summary, relation]}
        availability: {type: string, enum: [available]}
        source_metadata: {$ref: '#/components/schemas/SourceMetadata'}
        content: {type: string}
    ProviderContextSourceCitation:
      type: object
      additionalProperties: false
      required: [source_kind, reference, version, availability]
      properties:
        source_kind: {type: string, enum: [canonical_memory]}
        reference: {type: string, minLength: 1}
        availability: {type: string, enum: [available]}
        version: {type: string, pattern: '^[1-9][0-9]*$'}
    ProviderReferenceContextItem:
      type: object
      additionalProperties: false
      required: [reference, citations]
      properties:
        reference: {$ref: '#/components/schemas/ProviderContextReference'}
        citations: {type: array, items: {$ref: '#/components/schemas/ProviderContextSourceCitation'}}
    ProviderReferenceContextResult:
      type: object
      additionalProperties: false
      required: [context_digest, continuity, response_mode, disclosure, budget_accounting, profile, recent_session, recent_episodes, relevant_summaries, related_entities, citations]
      properties:
        turn_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        context_digest: {$ref: '#/components/schemas/ProviderContextDigest'}
        continuity: {$ref: '#/components/schemas/ProviderContextContinuityResult'}
        response_mode: {type: string, enum: [reference_only]}
        disclosure: {$ref: '#/components/schemas/ContextDisclosureDecision'}
        budget_accounting: {$ref: '#/components/schemas/ProviderContextBudgetAccounting'}
        profile: {type: array, items: {$ref: '#/components/schemas/ProviderReferenceContextItem'}}
        recent_session: {type: array, items: {$ref: '#/components/schemas/ProviderReferenceContextItem'}}
        recent_episodes: {type: array, items: {$ref: '#/components/schemas/ProviderReferenceContextItem'}}
        relevant_summaries: {type: array, items: {$ref: '#/components/schemas/ProviderReferenceContextItem'}}
        related_entities: {type: array, items: {$ref: '#/components/schemas/ProviderReferenceContextItem'}}
        citations: {type: array, items: {$ref: '#/components/schemas/ProviderContextSourceCitation'}}
    ProviderContentContextItem:
      type: object
      additionalProperties: false
      required: [memory, citations]
      properties:
        memory: {$ref: '#/components/schemas/ProviderContextMemory'}
        citations: {type: array, items: {$ref: '#/components/schemas/ProviderContextSourceCitation'}}
    ProviderContentContextResult:
      type: object
      additionalProperties: false
      required: [context_digest, continuity, response_mode, disclosure, budget_accounting, profile, recent_session, recent_episodes, relevant_summaries, related_entities, citations]
      properties:
        turn_id: {type: string, minLength: 1, maxLength: 128, pattern: '^[A-Za-z0-9_.:-]+$'}
        context_digest: {$ref: '#/components/schemas/ProviderContextDigest'}
        continuity: {$ref: '#/components/schemas/ProviderContextContinuityResult'}
        response_mode: {type: string, enum: [bounded_content]}
        disclosure: {$ref: '#/components/schemas/ContextDisclosureDecision'}
        budget_accounting: {$ref: '#/components/schemas/ProviderContextBudgetAccounting'}
        profile: {type: array, items: {$ref: '#/components/schemas/ProviderContentContextItem'}}
        recent_session: {type: array, items: {$ref: '#/components/schemas/ProviderContentContextItem'}}
        recent_episodes: {type: array, items: {$ref: '#/components/schemas/ProviderContentContextItem'}}
        relevant_summaries: {type: array, items: {$ref: '#/components/schemas/ProviderContentContextItem'}}
        related_entities: {type: array, items: {$ref: '#/components/schemas/ProviderContentContextItem'}}
        citations: {type: array, items: {$ref: '#/components/schemas/ProviderContextSourceCitation'}}
`
