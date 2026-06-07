package openapi

func SpecYAML() string {
	return `openapi: 3.1.0
info:
  title: Stele API
  version: 0.1.0
paths:
  /health:
    get:
      operationId: getHealth
      responses:
        '200':
          description: Service is alive
  /ready:
    get:
      operationId: getReady
      responses:
        '200':
          description: Service is ready
        '503':
          description: Dependencies are not ready
  /v1/events:
    post:
      operationId: createEvent
      parameters:
        - $ref: '#/components/parameters/PublicAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/EventIngestRequest'
      responses:
        '201':
          description: Event ingested
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EventIngestResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid API key
  /v1/memories:
    get:
      operationId: listMemories
      parameters:
        - $ref: '#/components/parameters/PublicAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
        - in: query
          name: class
          required: false
          style: form
          explode: true
          schema:
            type: array
            items:
              type: string
        - in: query
          name: time_from
          required: false
          schema:
            type: string
            format: date-time
        - in: query
          name: time_to
          required: false
          schema:
            type: string
            format: date-time
        - in: query
          name: limit
          required: false
          schema:
            type: integer
      responses:
        '200':
          description: Canonical memories visible in the caller scope
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MemoryListResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid API key
  /v1/memories/{memory_id}:
    get:
      operationId: getMemory
      parameters:
        - $ref: '#/components/parameters/MemoryIDPath'
        - $ref: '#/components/parameters/PublicAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      responses:
        '200':
          description: Canonical memory resource
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/CanonicalMemory'
        '401':
          description: Missing or invalid API key
        '404':
          description: Memory not found or not visible
  /v1/memories/{memory_id}/history:
    get:
      operationId: getMemoryHistory
      parameters:
        - $ref: '#/components/parameters/MemoryIDPath'
        - $ref: '#/components/parameters/PublicAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      responses:
        '200':
          description: Lifecycle-safe append-only version history
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MemoryHistory'
        '401':
          description: Missing or invalid API key
        '404':
          description: Memory not found or not visible
  /v1/memories/{memory_id}/provenance:
    get:
      operationId: getMemoryProvenance
      parameters:
        - $ref: '#/components/parameters/MemoryIDPath'
        - $ref: '#/components/parameters/PublicAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      responses:
        '200':
          description: Lifecycle-safe provenance lineage for the memory
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MemoryProvenanceResponse'
        '401':
          description: Missing or invalid API key
        '404':
          description: Memory not found or not visible
  /v1/memories/search:
    post:
      operationId: searchMemories
      parameters:
        - $ref: '#/components/parameters/PublicAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/MemorySearchRequest'
      responses:
        '200':
          description: Search results
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MemorySearchResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid API key
  /v1/context/assemble:
    post:
      operationId: assembleContext
      parameters:
        - $ref: '#/components/parameters/PublicAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ContextAssembleRequest'
      responses:
        '200':
          description: Assembled context
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ContextAssembleResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid API key
  /v1/admin/jobs/governance/status:
    get:
      operationId: getAdminGovernanceStatus
      parameters:
        - $ref: '#/components/parameters/AdminAPIKey'
      responses:
        '200':
          description: Governance backlog snapshot
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/GovernanceStatus'
        '401':
          description: Missing or invalid admin API key
  /v1/admin/jobs/status:
    get:
      operationId: getAdminJobStatus
      parameters:
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
        - in: query
          name: limit
          required: false
          schema:
            type: integer
      responses:
        '200':
          description: Recent worker and scheduler execution records
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/JobExecutionListResponse'
        '401':
          description: Missing or invalid admin API key
  /v1/admin/memories/{memory_id}/history:
    get:
      operationId: getAdminMemoryHistory
      parameters:
        - $ref: '#/components/parameters/MemoryIDPath'
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      responses:
        '200':
          description: Full memory history including hidden lifecycle states
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MemoryHistory'
        '401':
          description: Missing or invalid admin API key
  /v1/admin/memories/{memory_id}:suppress:
    post:
      operationId: suppressMemory
      parameters:
        - $ref: '#/components/parameters/MemoryIDPath'
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/ActorHeader'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/LifecycleActionRequest'
      responses:
        '200':
          description: Memory was suppressed
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LifecycleActionResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
  /v1/admin/memories/{memory_id}:expire:
    post:
      operationId: expireMemory
      parameters:
        - $ref: '#/components/parameters/MemoryIDPath'
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/ActorHeader'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/LifecycleActionRequest'
      responses:
        '200':
          description: Memory was expired
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LifecycleActionResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
  /v1/admin/memories/{memory_id}:delete:
    post:
      operationId: deleteMemory
      parameters:
        - $ref: '#/components/parameters/MemoryIDPath'
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/ActorHeader'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/LifecycleActionRequest'
      responses:
        '200':
          description: Memory was deleted
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LifecycleActionResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
components:
  parameters:
    PublicAPIKey:
      in: header
      name: X-API-Key
      required: true
      schema:
        type: string
    AdminAPIKey:
      in: header
      name: X-API-Key
      required: true
      schema:
        type: string
    TenantHeader:
      in: header
      name: X-Stele-Tenant
      required: true
      schema:
        type: string
    ProjectHeader:
      in: header
      name: X-Stele-Project
      required: true
      schema:
        type: string
    NamespaceHeader:
      in: header
      name: X-Stele-Namespace
      required: true
      schema:
        type: string
    ActorHeader:
      in: header
      name: X-Stele-Actor
      required: true
      schema:
        type: string
    MemoryIDPath:
      in: path
      name: memory_id
      required: true
      schema:
        type: string
  schemas:
    Scope:
      type: object
      required:
        - tenant
        - project
        - namespace
      properties:
        tenant:
          type: string
        project:
          type: string
        namespace:
          type: string
    EventIngestRequest:
      type: object
      required:
        - event_type
        - content
      properties:
        event_type:
          type: string
        content:
          type: string
        metadata:
          type: object
          additionalProperties: true
        source_timestamp:
          type: string
          format: date-time
    EventIngestResponse:
      type: object
      required:
        - event_id
      properties:
        event_id:
          type: string
    CanonicalMemory:
      type: object
      required:
        - id
        - scope
        - class
        - state
        - content
      properties:
        id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        class:
          type: string
        state:
          type: string
        content:
          type: string
        created_at:
          type: string
          format: date-time
        modified_at:
          type: string
          format: date-time
    MemoryListResponse:
      type: object
      required:
        - items
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/CanonicalMemory'
    MemorySearchRequest:
      type: object
      required:
        - query
      properties:
        query:
          type: string
        query_embedding:
          type: array
          items:
            type: number
        time_from:
          type: string
          format: date-time
        time_to:
          type: string
          format: date-time
        top_k:
          type: integer
        include_summaries:
          type: boolean
        include_relations:
          type: boolean
        classes:
          type: array
          items:
            type: string
    MemoryCitation:
      type: object
      required:
        - memory_id
        - operation
      properties:
        memory_id:
          type: string
        raw_event_id:
          type: string
        operation:
          type: string
    MemoryScore:
      type: object
      required:
        - overall
        - lexical
        - semantic
        - relation
      properties:
        overall:
          type: number
        lexical:
          type: number
        semantic:
          type: number
        relation:
          type: number
    MemorySearchHit:
      type: object
      required:
        - memory
        - score
        - citations
      properties:
        memory:
          $ref: '#/components/schemas/CanonicalMemory'
        score:
          $ref: '#/components/schemas/MemoryScore'
        citations:
          type: array
          items:
            $ref: '#/components/schemas/MemoryCitation'
    MemorySearchResponse:
      type: object
      required:
        - hits
      properties:
        hits:
          type: array
          items:
            $ref: '#/components/schemas/MemorySearchHit'
    ContextAssembleRequest:
      type: object
      required:
        - query
        - budget
      properties:
        query:
          type: string
        budget:
          type: integer
        include_relations:
          type: boolean
    ContextAssembleResponse:
      type: object
      required:
        - profile
        - recent_session
        - recent_episodes
        - relevant_summaries
        - related_entities
        - citations
      properties:
        profile:
          type: array
          items:
            $ref: '#/components/schemas/MemorySearchHit'
        recent_session:
          type: array
          items:
            $ref: '#/components/schemas/MemorySearchHit'
        recent_episodes:
          type: array
          items:
            $ref: '#/components/schemas/MemorySearchHit'
        relevant_summaries:
          type: array
          items:
            $ref: '#/components/schemas/MemorySearchHit'
        related_entities:
          type: array
          items:
            $ref: '#/components/schemas/MemorySearchHit'
        citations:
          type: array
          items:
            $ref: '#/components/schemas/MemoryCitation'
    GovernanceStatus:
      type: object
      required:
        - pending_raw_events
        - leased_raw_events
        - processed_raw_events
      properties:
        pending_raw_events:
          type: integer
        leased_raw_events:
          type: integer
        processed_raw_events:
          type: integer
        oldest_pending_created_at:
          type: string
          format: date-time
        observed_at:
          type: string
          format: date-time
    MemoryVersion:
      type: object
      required:
        - id
        - memory_id
        - version
        - state
        - content
      properties:
        id:
          type: string
        memory_id:
          type: string
        version:
          type: integer
        state:
          type: string
        content:
          type: string
        created_at:
          type: string
          format: date-time
        modified_by:
          type: string
    ProvenanceRecord:
      type: object
      required:
        - id
        - scope
        - operation
      properties:
        id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        raw_event_id:
          type: string
        candidate_memory_id:
          type: string
        memory_id:
          type: string
        request_id:
          type: string
        actor:
          type: string
        operation:
          type: string
        created_at:
          type: string
          format: date-time
        source_context:
          type: object
          additionalProperties: true
    MemoryHistory:
      type: object
      required:
        - memory
        - versions
        - provenance
      properties:
        memory:
          $ref: '#/components/schemas/CanonicalMemory'
        versions:
          type: array
          items:
            $ref: '#/components/schemas/MemoryVersion'
        provenance:
          type: array
          items:
            $ref: '#/components/schemas/ProvenanceRecord'
    MemoryProvenanceResponse:
      type: object
      required:
        - provenance
      properties:
        provenance:
          type: array
          items:
            $ref: '#/components/schemas/ProvenanceRecord'
    LifecycleActionRequest:
      type: object
      required:
        - reason
      properties:
        reason:
          type: string
    LifecycleActionResponse:
      type: object
      required:
        - memory_id
        - action
        - reason
      properties:
        memory_id:
          type: string
        action:
          type: string
        reason:
          type: string
    JobExecutionRecord:
      type: object
      required:
        - job_name
        - scope
        - trigger_source
        - idempotency_key
        - status
        - attempt
        - processed_count
        - started_at
      properties:
        job_name:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        trigger_source:
          type: string
        idempotency_key:
          type: string
        status:
          type: string
          enum: [running, completed, failed]
        attempt:
          type: integer
        processed_count:
          type: integer
        error_message:
          type: string
        started_at:
          type: string
          format: date-time
        finished_at:
          type: string
          format: date-time
    JobExecutionListResponse:
      type: object
      required:
        - executions
      properties:
        executions:
          type: array
          items:
            $ref: '#/components/schemas/JobExecutionRecord'
`
}
