package openapi

const providerContextSchemas = `    ProviderContextInput:
      type: object
      additionalProperties: false
      required: [query, budget]
      description: Exact snake_case fields only. Scope and session come from the authenticated runtime binding. Path selectors are mutually exclusive and use shared memory-path normalization. Budget counts items; it is not a serialized byte limit.
      properties:
        projection_intent: {type: string, enum: [conversation, task_context, compilation, handoff], default: conversation}
        disclosure_profile: {type: string, maxLength: 64, pattern: '^[A-Za-z0-9_.:-]+$'}
        response_mode: {type: string, enum: [reference_only, bounded_content], default: reference_only}
        query: {type: string, minLength: 1, pattern: '\S'}
        budget: {type: integer, minimum: 1}
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
        - {query: 'current task', budget: 8, path_prefix: 'tasks/demo', include_relations: true, include_experience_insights: true, include_goal_context: true, include_diagnostics: true, include_feedback_diagnostics: true, feedback_aware_ranking: true, feedback_ranking_policy: ''}
    ProviderContextResponse:
      type: object
      additionalProperties: false
      required: [metadata, result, citations]
      properties:
        metadata: {$ref: '#/components/schemas/ProviderOperationMetadata'}
        result: {$ref: '#/components/schemas/ProviderContextResult'}
        citations: {type: array, items: {$ref: '#/components/schemas/ProviderContextSourceCitation'}}
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
      required: [response_mode, disclosure, profile, recent_session, recent_episodes, relevant_summaries, related_entities, citations]
      properties:
        response_mode: {type: string, enum: [reference_only]}
        disclosure: {$ref: '#/components/schemas/ContextDisclosureDecision'}
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
      required: [response_mode, disclosure, profile, recent_session, recent_episodes, relevant_summaries, related_entities, citations]
      properties:
        response_mode: {type: string, enum: [bounded_content]}
        disclosure: {$ref: '#/components/schemas/ContextDisclosureDecision'}
        profile: {type: array, items: {$ref: '#/components/schemas/ProviderContentContextItem'}}
        recent_session: {type: array, items: {$ref: '#/components/schemas/ProviderContentContextItem'}}
        recent_episodes: {type: array, items: {$ref: '#/components/schemas/ProviderContentContextItem'}}
        relevant_summaries: {type: array, items: {$ref: '#/components/schemas/ProviderContentContextItem'}}
        related_entities: {type: array, items: {$ref: '#/components/schemas/ProviderContentContextItem'}}
        citations: {type: array, items: {$ref: '#/components/schemas/ProviderContextSourceCitation'}}
`
