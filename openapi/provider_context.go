package openapi

// These schemas belong to the same authoritative document as the native API.
const providerContextSchemas = `    ProviderContextInput:
      type: object
      additionalProperties: false
      required: [query, budget]
      description: Exact snake_case fields only. Scope and session come from the authenticated runtime binding. Path selectors are mutually exclusive and use shared memory-path normalization. Budget counts items; it is not a serialized byte limit.
      properties:
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
        citations:
          type: array
          description: Bounded Provider source summary; may be shorter than the inner memory evidence list. Version and watermark are omitted when unavailable.
          items: {$ref: '#/components/schemas/ProviderContextSourceCitation'}
    ProviderContextResult:
      type: object
      additionalProperties: false
      required: [profile, recent_session, recent_episodes, relevant_summaries, related_entities, citations]
      properties:
        profile: {type: array, items: {$ref: '#/components/schemas/ProviderContextItem'}}
        recent_session: {type: array, items: {$ref: '#/components/schemas/ProviderContextItem'}}
        recent_episodes: {type: array, items: {$ref: '#/components/schemas/ProviderContextItem'}}
        relevant_summaries: {type: array, items: {$ref: '#/components/schemas/ProviderContextItem'}}
        related_entities: {type: array, items: {$ref: '#/components/schemas/ProviderContextItem'}}
        citations: {type: array, items: {$ref: '#/components/schemas/ProviderContextMemoryCitation'}}
        known_failures: {type: array, items: {$ref: '#/components/schemas/ProviderContextInsightItem'}}
        experience_lessons: {type: array, items: {$ref: '#/components/schemas/ProviderContextInsightItem'}}
        goal_context: {type: array, items: {$ref: '#/components/schemas/ProviderContextGoal'}}
        diagnostics: {type: array, items: {$ref: '#/components/schemas/ProviderContextDiagnostic'}}
    ProviderContextItem:
      type: object
      additionalProperties: false
      required: [memory, citations]
      properties:
        memory: {$ref: '#/components/schemas/ProviderContextMemory'}
        citations: {type: array, items: {$ref: '#/components/schemas/ProviderContextMemoryCitation'}}
    ProviderContextScope:
      type: object
      additionalProperties: false
      required: [tenant, project, namespace]
      properties:
        tenant: {type: string}
        project: {type: string}
        namespace: {type: string}
    ProviderContextMemory:
      type: object
      additionalProperties: false
      required: [id, scope, memory_path, class, state, content, created_at, modified_at, temporal_fact_id, ingested_at, valid_from, validity_source]
      properties:
        id: {type: string}
        scope: {$ref: '#/components/schemas/ProviderContextScope'}
        memory_path: {type: string, maxLength: 512}
        class: {type: string, enum: [profile, episodic, procedural, summary, relation]}
        state: {type: string, enum: [active]}
        content: {type: string}
        created_at: {type: string, format: date-time}
        modified_at: {type: string, format: date-time}
        temporal_fact_id: {type: string}
        ingested_at: {type: string, format: date-time}
        valid_from: {type: string, format: date-time}
        valid_to: {type: string, format: date-time}
        validity_source: {type: string, enum: ['', explicit, legacy_current_compatible, migrated]}
    ProviderContextMemoryCitation:
      type: object
      additionalProperties: false
      required: [memory_id, raw_event_id, operation]
      properties:
        memory_id: {type: string}
        raw_event_id: {type: string}
        operation: {type: string}
    ProviderContextSourceCitation:
      type: object
      additionalProperties: false
      required: [source_kind, reference, availability]
      properties:
        source_kind: {type: string}
        reference: {type: string}
        availability: {type: string}
        version: {type: string}
        watermark: {type: string}
    ProviderContextInsightItem:
      type: object
      additionalProperties: false
      required: [insight, citations]
      properties:
        insight: {$ref: '#/components/schemas/ProviderContextInsight'}
        citations: {type: array, items: {$ref: '#/components/schemas/ProviderContextInsightCitation'}}
    ProviderContextInsight:
      type: object
      additionalProperties: false
      required: [id, scope, type, state, title, summary, confidence, created_at, updated_at, last_observed_at]
      properties:
        id: {type: string}
        scope: {$ref: '#/components/schemas/ProviderContextScope'}
        type: {type: string}
        state: {type: string, enum: [active]}
        title: {type: string}
        summary: {type: string}
        confidence:
          type: object
          additionalProperties: false
          required: [score]
          properties:
            score: {type: number, minimum: 0, maximum: 1}
            method: {type: string}
        lesson:
          type: object
          additionalProperties: false
          required: [source_failure_pattern_id, guidance]
          properties:
            source_failure_pattern_id: {type: string}
            guidance: {type: string}
            avoid: {type: array, items: {type: string}}
            prefer: {type: array, items: {type: string}}
        created_at: {type: string, format: date-time}
        updated_at: {type: string, format: date-time}
        last_observed_at: {type: string, format: date-time}
    ProviderContextInsightCitation:
      type: object
      additionalProperties: false
      required: [insight_id, evidence_kind, evidence_id, relation]
      properties:
        insight_id: {type: string}
        evidence_kind: {type: string}
        evidence_id: {type: string}
        relation: {type: string}
    ProviderContextGoal:
      type: object
      additionalProperties: false
      required: [title, summary, state, review_state, policy_version]
      properties:
        title: {type: string}
        summary: {type: string}
        state: {type: string, enum: [proposed, active]}
        review_state: {type: string, enum: [review_required, review_approved, review_rejected]}
        policy_version: {type: string}
    ProviderContextDiagnostic:
      type: object
      additionalProperties: false
      required: [section, status]
      properties:
        section: {type: string}
        insight_type: {type: string}
        status: {type: string}
        reason: {type: string}
        available: {type: integer, minimum: 0}
        included: {type: integer, minimum: 0}
        omitted: {type: integer, minimum: 0}
        hidden: {type: integer, minimum: 0}
`
