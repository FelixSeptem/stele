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
  /livez:
    get:
      operationId: getLivez
      responses:
        '200':
          description: Runtime process is alive
  /readyz:
    get:
      operationId: getReadyz
      responses:
        '200':
          description: Runtime is ready for its configured mode
        '503':
          description: Runtime dependencies are not ready
  /metrics:
    get:
      operationId: getMetrics
      responses:
        '200':
          description: Prometheus metrics exposition
          content:
            text/plain:
              schema:
                type: string
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
  /v1/admin/governance/raw-events:
    get:
      operationId: listAdminGovernanceRawEvents
      parameters:
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
        - in: query
          name: state
          required: false
          schema:
            $ref: '#/components/schemas/GovernanceRawEventState'
        - in: query
          name: event_type
          required: false
          schema:
            type: string
        - in: query
          name: attempt_gte
          required: false
          schema:
            type: integer
        - in: query
          name: attempt_lte
          required: false
          schema:
            type: integer
        - in: query
          name: failed_from
          required: false
          schema:
            type: string
            format: date-time
        - in: query
          name: failed_to
          required: false
          schema:
            type: string
            format: date-time
        - in: query
          name: next_attempt_from
          required: false
          schema:
            type: string
            format: date-time
        - in: query
          name: next_attempt_to
          required: false
          schema:
            type: string
            format: date-time
        - in: query
          name: limit
          required: false
          schema:
            type: integer
        - in: query
          name: cursor
          required: false
          schema:
            type: string
      responses:
        '200':
          description: Filtered governance raw event inspection page
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/GovernanceRawEventListResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
  /v1/admin/governance/raw-events/{raw_event_id}:
    get:
      operationId: getAdminGovernanceRawEvent
      parameters:
        - $ref: '#/components/parameters/RawEventIDPath'
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      responses:
        '200':
          description: Governance raw event detail
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/GovernanceRawEvent'
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Raw event not found
  /v1/admin/governance/raw-events/{raw_event_id}/recovery-history:
    get:
      operationId: listAdminGovernanceRecoveryHistory
      parameters:
        - $ref: '#/components/parameters/RawEventIDPath'
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      responses:
        '200':
          description: Governance recovery history for one raw event
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/GovernanceRecoveryHistoryResponse'
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Raw event not found
  /v1/admin/governance/raw-events/{raw_event_id}:retry:
    post:
      operationId: retryAdminGovernanceRawEvent
      parameters:
        - $ref: '#/components/parameters/RawEventIDPath'
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
              $ref: '#/components/schemas/GovernanceRecoveryActionRequest'
      responses:
        '200':
          description: Governance raw event recovery applied
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/GovernanceRecoveryOutcome'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Raw event not found
        '409':
          description: Recovery conflict
        '422':
          description: Recovery rejected
  /v1/admin/governance/raw-events/{raw_event_id}:reschedule:
    post:
      operationId: rescheduleAdminGovernanceRawEvent
      parameters:
        - $ref: '#/components/parameters/RawEventIDPath'
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
              $ref: '#/components/schemas/GovernanceRecoveryActionRequest'
      responses:
        '200':
          description: Governance raw event recovery applied
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/GovernanceRecoveryOutcome'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Raw event not found
        '409':
          description: Recovery conflict
        '422':
          description: Recovery rejected
  /v1/admin/governance/raw-events/{raw_event_id}:requeue:
    post:
      operationId: requeueAdminGovernanceRawEvent
      parameters:
        - $ref: '#/components/parameters/RawEventIDPath'
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
              $ref: '#/components/schemas/GovernanceRecoveryActionRequest'
      responses:
        '200':
          description: Governance raw event recovery applied
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/GovernanceRecoveryOutcome'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Raw event not found
        '409':
          description: Recovery conflict
        '422':
          description: Recovery rejected
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
  /v1/admin/derived-insights:
    get:
      operationId: listAdminDerivedInsights
      parameters:
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
        - in: query
          name: type
          required: false
          schema:
            $ref: '#/components/schemas/DerivedInsightType'
        - in: query
          name: state
          required: false
          schema:
            $ref: '#/components/schemas/DerivedInsightState'
        - in: query
          name: min_confidence
          required: false
          schema:
            type: number
            minimum: 0
            maximum: 1
        - in: query
          name: min_evidence_count
          required: false
          schema:
            type: integer
            minimum: 0
        - in: query
          name: include_hidden
          required: false
          schema:
            type: boolean
        - in: query
          name: limit
          required: false
          schema:
            type: integer
      responses:
        '200':
          description: Scoped derived insight summaries for admin inspection
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DerivedInsightListResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
  /v1/admin/derived-insights/{insight_id}:
    get:
      operationId: getAdminDerivedInsight
      parameters:
        - in: path
          name: insight_id
          required: true
          schema:
            type: string
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
        - in: query
          name: include_hidden
          required: false
          schema:
            type: boolean
      responses:
        '200':
          description: Derived insight detail including evidence and lifecycle history
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DerivedInsightDetail'
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Derived insight not found
  /v1/admin/derived-insights/{insight_id}/feedback:
    get:
      operationId: listAdminDerivedInsightFeedback
      parameters:
        - in: path
          name: insight_id
          required: true
          schema:
            type: string
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
        - in: query
          name: type
          required: false
          schema:
            $ref: '#/components/schemas/InsightFeedbackType'
        - in: query
          name: include_superseded
          required: false
          schema:
            type: boolean
        - in: query
          name: limit
          required: false
          schema:
            type: integer
      responses:
        '200':
          description: Scoped derived insight feedback records
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DerivedInsightFeedbackListResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
    post:
      operationId: createAdminDerivedInsightFeedback
      parameters:
        - in: path
          name: insight_id
          required: true
          schema:
            type: string
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DerivedInsightFeedbackCreateRequest'
      responses:
        '201':
          description: Feedback recorded and audited
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DerivedInsightFeedback'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Derived insight not found
  /v1/admin/derived-insight-feedback/{feedback_id}:supersede:
    post:
      operationId: supersedeAdminDerivedInsightFeedback
      parameters:
        - in: path
          name: feedback_id
          required: true
          schema:
            type: string
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DerivedInsightFeedbackSupersedeRequest'
      responses:
        '200':
          description: Feedback superseded and removed from active summaries
          content:
            application/json:
              schema:
                type: object
                properties:
                  status:
                    type: string
                  feedback_id:
                    type: string
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Derived insight feedback not found
  /v1/admin/derived-insights/{insight_id}:suppress:
    post:
      operationId: suppressAdminDerivedInsight
      parameters:
        - in: path
          name: insight_id
          required: true
          schema:
            type: string
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DerivedInsightSuppressRequest'
      responses:
        '200':
          description: Derived insight suppressed and audited
          content:
            application/json:
              schema:
                type: object
                properties:
                  status:
                    type: string
                  insight_id:
                    type: string
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Derived insight not found
  /v1/admin/derived-insight-replays:dry-run:
    post:
      operationId: planAdminDerivedInsightReplay
      parameters:
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DerivedInsightReplayRequest'
      responses:
        '200':
          description: Replay dry-run plan without mutations
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DerivedInsightReplayReport'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
  /v1/admin/derived-insight-replays:
    get:
      operationId: listAdminDerivedInsightReplays
      parameters:
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
        - in: query
          name: status
          required: false
          schema:
            $ref: '#/components/schemas/DerivedInsightReplayStatus'
        - in: query
          name: mode
          required: false
          schema:
            $ref: '#/components/schemas/DerivedInsightReplayMode'
        - in: query
          name: limit
          required: false
          schema:
            type: integer
      responses:
        '200':
          description: Scoped derived insight replay runs
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DerivedInsightReplayListResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
    post:
      operationId: applyAdminDerivedInsightReplay
      parameters:
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DerivedInsightReplayRequest'
      responses:
        '202':
          description: Replay apply was accepted as durable background work
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DerivedInsightReplayRun'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
  /v1/admin/derived-insight-replays/{replay_run_id}:
    get:
      operationId: getAdminDerivedInsightReplay
      parameters:
        - in: path
          name: replay_run_id
          required: true
          schema:
            type: string
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      responses:
        '200':
          description: Derived insight replay run detail
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DerivedInsightReplayRun'
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Replay run not found
  /v1/admin/derived-insight-replays/{replay_run_id}/report:
    get:
      operationId: getAdminDerivedInsightReplayReport
      parameters:
        - in: path
          name: replay_run_id
          required: true
          schema:
            type: string
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      responses:
        '200':
          description: Derived insight replay report
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DerivedInsightReplayReport'
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Replay report not found
  /v1/admin/embedding/rebuilds:
    get:
      operationId: listAdminEmbeddingRebuilds
      parameters:
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
        - in: query
          name: status
          required: false
          schema:
            type: string
            enum: [pending, rebuilding, failed, current]
        - in: query
          name: requested_provider
          required: false
          schema:
            type: string
        - in: query
          name: requested_model
          required: false
          schema:
            type: string
        - in: query
          name: drifted
          required: false
          schema:
            type: boolean
        - in: query
          name: limit
          required: false
          schema:
            type: integer
      responses:
        '200':
          description: Scoped embedding rebuild backlog inspection
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingRebuildListResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
  /v1/admin/embedding/rebuilds/{memory_id}:retry:
    post:
      operationId: retryAdminEmbeddingRebuild
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
              $ref: '#/components/schemas/EmbeddingRecoveryActionRequest'
      responses:
        '200':
          description: Embedding recovery applied
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingRecoveryOutcome'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Memory not found
        '409':
          description: Recovery conflict
  /v1/admin/embedding/rebuilds/{memory_id}:requeue:
    post:
      operationId: requeueAdminEmbeddingRebuild
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
              $ref: '#/components/schemas/EmbeddingRecoveryActionRequest'
      responses:
        '200':
          description: Embedding recovery applied
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingRecoveryOutcome'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Memory not found
        '409':
          description: Recovery conflict
  /v1/admin/embedding/recovery-history:
    get:
      operationId: listAdminEmbeddingRecoveryHistory
      parameters:
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
        - in: query
          name: action
          required: false
          schema:
            $ref: '#/components/schemas/EmbeddingRecoveryAction'
        - in: query
          name: actor
          required: false
          schema:
            type: string
        - in: query
          name: cutover_plan_id
          required: false
          schema:
            type: string
        - in: query
          name: occurred_from
          required: false
          schema:
            type: string
            format: date-time
        - in: query
          name: occurred_to
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
          description: Scoped embedding recovery audit history
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingRecoveryHistoryResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
  /v1/admin/embedding/cutovers:
    get:
      operationId: listAdminEmbeddingCutoverPlans
      parameters:
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
        - in: query
          name: status
          required: false
          schema:
            $ref: '#/components/schemas/EmbeddingCutoverPlanStatus'
        - in: query
          name: limit
          required: false
          schema:
            type: integer
      responses:
        '200':
          description: Recent and active embedding cutover plans
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingCutoverPlanListResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
    post:
      operationId: createAdminEmbeddingCutoverPlan
      parameters:
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
              $ref: '#/components/schemas/EmbeddingCutoverPlanRequest'
      responses:
        '201':
          description: Embedding cutover plan created
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingCutoverPlan'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
  /v1/admin/embedding/cutovers/{cutover_plan_id}:
    get:
      operationId: getAdminEmbeddingCutoverPlan
      parameters:
        - $ref: '#/components/parameters/CutoverPlanIDPath'
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      responses:
        '200':
          description: One embedding cutover plan with progress and item state
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingCutoverPlan'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Cutover plan not found
  /v1/admin/embedding/cutovers/{cutover_plan_id}:preflight:
    post:
      operationId: preflightAdminEmbeddingCutoverPlan
      parameters:
        - $ref: '#/components/parameters/CutoverPlanIDPath'
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      responses:
        '200':
          description: Immediate embedding cutover admission report
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingCutoverPreflightReport'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Cutover plan not found
  /v1/admin/embedding/cutovers/{cutover_plan_id}:activate:
    post:
      operationId: activateAdminEmbeddingCutoverPlan
      parameters:
        - $ref: '#/components/parameters/CutoverPlanIDPath'
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
              $ref: '#/components/schemas/EmbeddingCutoverActionRequest'
      responses:
        '200':
          description: Embedding cutover plan activated
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingCutoverPlan'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Cutover plan not found
        '409':
          description: Cutover conflict
        '422':
          description: Cutover activation rejected by admission
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingCutoverPreflightReport'
  /v1/admin/embedding/cutovers/{cutover_plan_id}:pause:
    post:
      operationId: pauseAdminEmbeddingCutoverPlan
      parameters:
        - $ref: '#/components/parameters/CutoverPlanIDPath'
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
              $ref: '#/components/schemas/EmbeddingCutoverActionRequest'
      responses:
        '200':
          description: Embedding cutover plan paused
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingCutoverPlan'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Cutover plan not found
        '409':
          description: Cutover conflict
  /v1/admin/embedding/cutovers/{cutover_plan_id}:cancel:
    post:
      operationId: cancelAdminEmbeddingCutoverPlan
      parameters:
        - $ref: '#/components/parameters/CutoverPlanIDPath'
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
              $ref: '#/components/schemas/EmbeddingCutoverActionRequest'
      responses:
        '200':
          description: Embedding cutover plan cancelled
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingCutoverPlan'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Cutover plan not found
        '409':
          description: Cutover conflict
  /v1/admin/memories/{memory_id}/embedding:
    get:
      operationId: getAdminMemoryEmbedding
      parameters:
        - $ref: '#/components/parameters/MemoryIDPath'
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
      responses:
        '200':
          description: Embedding rebuild and revision inspection for one memory
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingMemoryInspection'
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Memory not found
  /v1/admin/memories/{memory_id}/embedding/recovery-history:
    get:
      operationId: listAdminMemoryEmbeddingRecoveryHistory
      parameters:
        - $ref: '#/components/parameters/MemoryIDPath'
        - $ref: '#/components/parameters/AdminAPIKey'
        - $ref: '#/components/parameters/TenantHeader'
        - $ref: '#/components/parameters/ProjectHeader'
        - $ref: '#/components/parameters/NamespaceHeader'
        - in: query
          name: action
          required: false
          schema:
            $ref: '#/components/schemas/EmbeddingRecoveryAction'
        - in: query
          name: actor
          required: false
          schema:
            type: string
        - in: query
          name: cutover_plan_id
          required: false
          schema:
            type: string
        - in: query
          name: occurred_from
          required: false
          schema:
            type: string
            format: date-time
        - in: query
          name: occurred_to
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
          description: Recovery audit history for one memory
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/EmbeddingRecoveryHistoryResponse'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Memory not found
  /v1/admin/memories:
    post:
      operationId: createAdminMemory
      parameters:
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
              $ref: '#/components/schemas/AdminCreateMemoryRequest'
      responses:
        '201':
          description: Canonical memory was created manually
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/CanonicalMemory'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
  /v1/admin/memories/{memory_id}:
    patch:
      operationId: updateAdminMemory
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
              $ref: '#/components/schemas/AdminUpdateMemoryRequest'
      responses:
        '200':
          description: Canonical memory was updated manually
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/CanonicalMemory'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Memory not found
        '409':
          description: Expected version does not match current version
  /v1/admin/memories/{memory_id}:merge:
    post:
      operationId: mergeAdminMemory
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
              $ref: '#/components/schemas/AdminMergeMemoryRequest'
      responses:
        '200':
          description: Canonical memory merge completed
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/CanonicalMemory'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Memory not found
        '409':
          description: Expected version does not match current version
  /v1/admin/memories/{memory_id}:reclassify:
    post:
      operationId: reclassifyAdminMemory
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
              $ref: '#/components/schemas/AdminReclassifyMemoryRequest'
      responses:
        '200':
          description: Canonical memory class was updated
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/CanonicalMemory'
        '400':
          description: Invalid request
        '401':
          description: Missing or invalid admin API key
        '404':
          description: Memory not found
        '409':
          description: Expected version does not match current version
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
    RawEventIDPath:
      in: path
      name: raw_event_id
      required: true
      schema:
        type: string
    CutoverPlanIDPath:
      in: path
      name: cutover_plan_id
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
    DerivedInsightType:
      type: string
      enum:
        - failure_pattern
        - lesson
        - hypothesis
        - goal
        - contradiction
        - causal_link
    DerivedInsightState:
      type: string
      enum:
        - candidate
        - active
        - suppressed
        - forgotten
        - deleted
    InsightFeedbackType:
      type: string
      enum:
        - useful
        - noisy
        - incorrect
        - stale
        - redundant
        - needs_review
    DerivedInsightConfidence:
      type: object
      required:
        - score
      properties:
        score:
          type: number
          minimum: 0
          maximum: 1
        method:
          type: string
    DerivedInsightDerivation:
      type: object
      required:
        - source
        - derivation_fingerprint
        - derived_at
      properties:
        source:
          type: string
        derivation_fingerprint:
          type: string
        fingerprint:
          type: string
          deprecated: true
        evidence_window_start:
          type: string
          format: date-time
        evidence_window_end:
          type: string
          format: date-time
        derived_at:
          type: string
          format: date-time
        metadata:
          type: object
          additionalProperties: true
    DerivedInsightEvidenceRef:
      type: object
      required:
        - kind
        - id
        - relation
      properties:
        kind:
          type: string
          enum:
            - raw_event
            - canonical_memory
            - procedural_memory
            - summary_memory
            - relation_memory
            - job_execution
            - embedding_rebuild
            - recovery_record
        id:
          type: string
        relation:
          type: string
          enum:
            - supports
            - updates
        observed_at:
          type: string
          format: date-time
        metadata:
          type: object
          additionalProperties: true
    DerivedInsightLesson:
      type: object
      required:
        - source_failure_pattern_id
        - guidance
      properties:
        source_failure_pattern_id:
          type: string
        guidance:
          type: string
        avoid:
          type: array
          items:
            type: string
        prefer:
          type: array
          items:
            type: string
    DerivedInsight:
      type: object
      required:
        - id
        - scope
        - type
        - state
        - title
        - summary
        - confidence
        - derivation
      properties:
        id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        type:
          $ref: '#/components/schemas/DerivedInsightType'
        state:
          $ref: '#/components/schemas/DerivedInsightState'
        title:
          type: string
        summary:
          type: string
        confidence:
          $ref: '#/components/schemas/DerivedInsightConfidence'
        payload:
          type: object
          additionalProperties: true
        lesson:
          $ref: '#/components/schemas/DerivedInsightLesson'
        derivation:
          $ref: '#/components/schemas/DerivedInsightDerivation'
        evidence:
          type: array
          items:
            $ref: '#/components/schemas/DerivedInsightEvidenceRef'
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time
        last_observed_at:
          type: string
          format: date-time
    DerivedInsightListResponse:
      type: object
      required:
        - items
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/DerivedInsight'
    DerivedInsightLifecycleRecord:
      type: object
      required:
        - insight_id
        - to_state
        - actor
        - reason
        - occurred_at
      properties:
        id:
          type: string
        insight_id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        from_state:
          $ref: '#/components/schemas/DerivedInsightState'
        to_state:
          $ref: '#/components/schemas/DerivedInsightState'
        actor:
          type: string
        reason:
          type: string
        occurred_at:
          type: string
          format: date-time
        metadata:
          type: object
          additionalProperties: true
    DerivedInsightDetail:
      type: object
      required:
        - insight
        - evidence
        - lifecycle
      properties:
        insight:
          $ref: '#/components/schemas/DerivedInsight'
        evidence:
          type: array
          items:
            $ref: '#/components/schemas/DerivedInsightEvidenceRef'
        lifecycle:
          type: array
          items:
            $ref: '#/components/schemas/DerivedInsightLifecycleRecord'
        feedback_summary:
          $ref: '#/components/schemas/DerivedInsightFeedbackSummary'
    DerivedInsightSuppressRequest:
      type: object
      required:
        - actor
        - reason
      properties:
        actor:
          type: string
        reason:
          type: string
    DerivedInsightFeedback:
      type: object
      required:
        - id
        - insight_id
        - scope
        - type
        - actor
        - reason
        - created_at
      properties:
        id:
          type: string
        insight_id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        type:
          $ref: '#/components/schemas/InsightFeedbackType'
        actor:
          type: string
        reason:
          type: string
        quality_score:
          type: number
          minimum: 0
          maximum: 1
        created_at:
          type: string
          format: date-time
        superseded_at:
          type: string
          format: date-time
        superseded_by_actor:
          type: string
        superseded_by_reason:
          type: string
        request_id:
          type: string
        metadata:
          type: object
          additionalProperties: true
    DerivedInsightFeedbackCreateRequest:
      type: object
      required:
        - type
        - actor
        - reason
      properties:
        type:
          $ref: '#/components/schemas/InsightFeedbackType'
        actor:
          type: string
        reason:
          type: string
        quality_score:
          type: number
          minimum: 0
          maximum: 1
        metadata:
          type: object
          additionalProperties: true
    DerivedInsightFeedbackListResponse:
      type: object
      required:
        - items
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/DerivedInsightFeedback'
    DerivedInsightFeedbackSupersedeRequest:
      type: object
      required:
        - actor
        - reason
      properties:
        actor:
          type: string
        reason:
          type: string
    DerivedInsightFeedbackSummary:
      type: object
      required:
        - counts
        - total_active
        - positive_count
        - negative_count
        - needs_review
      properties:
        insight_id:
          type: string
        counts:
          type: object
          additionalProperties:
            type: integer
        total_active:
          type: integer
        positive_count:
          type: integer
        negative_count:
          type: integer
        needs_review:
          type: boolean
        last_feedback_at:
          type: string
          format: date-time
    DerivedInsightReplayMode:
      type: string
      enum:
        - dry_run
        - apply
    DerivedInsightReplayStatus:
      type: string
      enum:
        - pending
        - running
        - completed
        - failed
        - continuation_required
    DerivedInsightReplayDecisionKind:
      type: string
      enum:
        - create
        - update
        - suppress
        - preserve
        - skip
    DerivedInsightReplayReason:
      type: string
      enum:
        - repeated_evidence
        - insufficient_evidence
        - unsupported_type
        - feedback_policy
        - lifecycle_hidden
        - out_of_scope
        - idempotent_duplicate
        - execution_failed
    DerivedInsightReplayRequest:
      type: object
      required:
        - evidence_window_start
        - evidence_window_end
        - evidence_limit
        - actor
        - reason
      properties:
        insight_types:
          type: array
          items:
            $ref: '#/components/schemas/DerivedInsightType'
        evidence_window_start:
          type: string
          format: date-time
        evidence_window_end:
          type: string
          format: date-time
        evidence_limit:
          type: integer
          minimum: 1
        actor:
          type: string
        reason:
          type: string
        idempotency_key:
          type: string
        metadata:
          type: object
          additionalProperties: true
    DerivedInsightReplayCounters:
      type: object
      properties:
        evidence_evaluated:
          type: integer
        created:
          type: integer
        updated:
          type: integer
        suppressed:
          type: integer
        preserved:
          type: integer
        skipped:
          type: integer
        failed:
          type: integer
    DerivedInsightReplayDecision:
      type: object
      required:
        - insight_type
        - fingerprint
        - decision
        - reason
      properties:
        insight_id:
          type: string
        insight_type:
          $ref: '#/components/schemas/DerivedInsightType'
        fingerprint:
          type: string
        decision:
          $ref: '#/components/schemas/DerivedInsightReplayDecisionKind'
        reason:
          $ref: '#/components/schemas/DerivedInsightReplayReason'
        evidence_count:
          type: integer
        message:
          type: string
    DerivedInsightReplayReport:
      type: object
      required:
        - run_id
        - scope
        - counters
        - generated_at
      properties:
        run_id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        counters:
          $ref: '#/components/schemas/DerivedInsightReplayCounters'
        decisions:
          type: array
          items:
            $ref: '#/components/schemas/DerivedInsightReplayDecision'
        failure:
          type: string
        generated_at:
          type: string
          format: date-time
    DerivedInsightReplayRun:
      type: object
      required:
        - id
        - scope
        - mode
        - status
        - request
        - actor
        - reason
        - created_at
        - updated_at
      properties:
        id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        mode:
          $ref: '#/components/schemas/DerivedInsightReplayMode'
        status:
          $ref: '#/components/schemas/DerivedInsightReplayStatus'
        request:
          $ref: '#/components/schemas/DerivedInsightReplayRequest'
        report:
          $ref: '#/components/schemas/DerivedInsightReplayReport'
        actor:
          type: string
        reason:
          type: string
        failure:
          type: string
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time
        started_at:
          type: string
          format: date-time
        finished_at:
          type: string
          format: date-time
    DerivedInsightReplayListResponse:
      type: object
      required:
        - items
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/DerivedInsightReplayRun'
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
        include_experience_insights:
          type: boolean
        include_diagnostics:
          type: boolean
    InsightCitation:
      type: object
      required:
        - insight_id
        - evidence_kind
        - evidence_id
        - relation
      properties:
        insight_id:
          type: string
        evidence_kind:
          type: string
        evidence_id:
          type: string
        relation:
          type: string
    ExperienceInsightContext:
      type: object
      required:
        - insight
        - citations
      properties:
        insight:
          $ref: '#/components/schemas/DerivedInsight'
        citations:
          type: array
          items:
            $ref: '#/components/schemas/InsightCitation'
    ContextDiagnostic:
      type: object
      required:
        - section
        - status
      properties:
        section:
          type: string
        insight_type:
          type: string
        status:
          type: string
        reason:
          type: string
        available:
          type: integer
        included:
          type: integer
        omitted:
          type: integer
        hidden:
          type: integer
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
        known_failures:
          type: array
          items:
            $ref: '#/components/schemas/ExperienceInsightContext'
        experience_lessons:
          type: array
          items:
            $ref: '#/components/schemas/ExperienceInsightContext'
        diagnostics:
          type: array
          items:
            $ref: '#/components/schemas/ContextDiagnostic'
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
    GovernanceRawEventState:
      type: string
      enum: [pending, retry_wait, leased, exhausted, processed]
    GovernanceRecoveryAction:
      type: string
      enum: [retry, reschedule, requeue]
    GovernanceRawEvent:
      type: object
      required:
        - id
        - scope
        - event_type
        - content
        - created_at
        - state
        - attempt
      properties:
        id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        event_type:
          type: string
        content:
          type: string
        source_timestamp:
          type: string
          format: date-time
        created_at:
          type: string
          format: date-time
        state:
          $ref: '#/components/schemas/GovernanceRawEventState'
        attempt:
          type: integer
        worker_id:
          type: string
        claimed_at:
          type: string
          format: date-time
        lease_until:
          type: string
          format: date-time
        last_failed_at:
          type: string
          format: date-time
        last_error:
          type: string
        next_attempt_at:
          type: string
          format: date-time
        exhausted_at:
          type: string
          format: date-time
        processed_at:
          type: string
          format: date-time
    GovernanceRawEventListResponse:
      type: object
      required:
        - items
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/GovernanceRawEvent'
        next_cursor:
          type: string
    GovernanceRecoverySnapshot:
      type: object
      required:
        - state
        - attempt
      properties:
        state:
          $ref: '#/components/schemas/GovernanceRawEventState'
        attempt:
          type: integer
        worker_id:
          type: string
        claimed_at:
          type: string
          format: date-time
        lease_until:
          type: string
          format: date-time
        last_failed_at:
          type: string
          format: date-time
        last_error:
          type: string
        next_attempt_at:
          type: string
          format: date-time
        exhausted_at:
          type: string
          format: date-time
        processed_at:
          type: string
          format: date-time
    GovernanceRecoveryRecord:
      type: object
      required:
        - id
        - raw_event_id
        - scope
        - action
        - actor
        - reason
        - before
        - after
        - occurred_at
      properties:
        id:
          type: string
        raw_event_id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        action:
          $ref: '#/components/schemas/GovernanceRecoveryAction'
        actor:
          type: string
        reason:
          type: string
        before:
          $ref: '#/components/schemas/GovernanceRecoverySnapshot'
        after:
          $ref: '#/components/schemas/GovernanceRecoverySnapshot'
        occurred_at:
          type: string
          format: date-time
    GovernanceRecoveryHistoryResponse:
      type: object
      required:
        - history
      properties:
        history:
          type: array
          items:
            $ref: '#/components/schemas/GovernanceRecoveryRecord'
    GovernanceRecoveryActionRequest:
      type: object
      required:
        - reason
      properties:
        reason:
          type: string
        scheduled_for:
          type: string
          format: date-time
    GovernanceRecoveryOutcome:
      type: object
      required:
        - raw_event
        - recovery
      properties:
        raw_event:
          $ref: '#/components/schemas/GovernanceRawEvent'
        recovery:
          $ref: '#/components/schemas/GovernanceRecoveryRecord'
    EmbeddingRuntimeStatus:
      type: object
      required:
        - configured
        - semantic_rebuild_enabled
      properties:
        configured:
          type: boolean
        semantic_rebuild_enabled:
          type: boolean
        registered_providers:
          type: array
          items:
            type: string
        reason:
          type: string
    EmbeddingRebuildView:
      type: object
      required:
        - memory_id
        - scope
        - class
        - state
        - status
        - drifted
      properties:
        memory_id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        class:
          type: string
        state:
          type: string
        status:
          type: string
          enum: [pending, rebuilding, failed, current]
        requested_provider:
          type: string
        requested_model:
          type: string
        requested_dimensions:
          type: integer
        active_vector_revision_id:
          type: string
        active_provider:
          type: string
        active_model:
          type: string
        active_dimensions:
          type: integer
        failure_reason:
          type: string
        drifted:
          type: boolean
        requested_at:
          type: string
          format: date-time
        last_attempted_at:
          type: string
          format: date-time
    EmbeddingRebuildListResponse:
      type: object
      required:
        - runtime
        - items
      properties:
        runtime:
          $ref: '#/components/schemas/EmbeddingRuntimeStatus'
        items:
          type: array
          items:
            $ref: '#/components/schemas/EmbeddingRebuildView'
    EmbeddingMemorySummary:
      type: object
      required:
        - id
        - scope
        - class
        - state
        - current_source_version
        - current_content_hash
      properties:
        id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        class:
          type: string
        state:
          type: string
        current_source_version:
          type: integer
        current_content_hash:
          type: string
    EmbeddingVectorRevision:
      type: object
      required:
        - id
        - provider
        - model
        - dimensions
        - status
        - source_version
        - content_hash
        - generated_at
      properties:
        id:
          type: string
        provider:
          type: string
        model:
          type: string
        dimensions:
          type: integer
        status:
          type: string
          enum: [generated, active, superseded, failed]
        failure_reason:
          type: string
        superseded_by:
          type: string
        source_version:
          type: integer
        content_hash:
          type: string
        generated_at:
          type: string
          format: date-time
        activated_at:
          type: string
          format: date-time
        last_rebuild_request_at:
          type: string
          format: date-time
    EmbeddingRecoveryAction:
      type: string
      enum: [retry, requeue]
    EmbeddingRecoveryActionRequest:
      type: object
      required:
        - reason
      properties:
        reason:
          type: string
    EmbeddingRecoverySnapshot:
      type: object
      required:
        - status
      properties:
        status:
          type: string
          enum: [pending, rebuilding, failed, current]
        requested_provider:
          type: string
        requested_model:
          type: string
        requested_dimensions:
          type: integer
        failure_reason:
          type: string
        requested_at:
          type: string
          format: date-time
        last_attempted_at:
          type: string
          format: date-time
        active_vector_revision_id:
          type: string
        active_provider:
          type: string
        active_model:
          type: string
        active_dimensions:
          type: integer
    EmbeddingRecoveryRecord:
      type: object
      required:
        - id
        - memory_id
        - scope
        - action
        - actor
        - reason
        - before
        - after
        - occurred_at
      properties:
        id:
          type: string
        memory_id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        cutover_plan_id:
          type: string
        action:
          $ref: '#/components/schemas/EmbeddingRecoveryAction'
        actor:
          type: string
        reason:
          type: string
        before:
          $ref: '#/components/schemas/EmbeddingRecoverySnapshot'
        after:
          $ref: '#/components/schemas/EmbeddingRecoverySnapshot'
        occurred_at:
          type: string
          format: date-time
    EmbeddingRecoveryOutcome:
      type: object
      required:
        - rebuild
        - recovery
      properties:
        rebuild:
          $ref: '#/components/schemas/EmbeddingRebuildView'
        recovery:
          $ref: '#/components/schemas/EmbeddingRecoveryRecord'
    EmbeddingRecoveryHistoryResponse:
      type: object
      required:
        - history
      properties:
        history:
          type: array
          items:
            $ref: '#/components/schemas/EmbeddingRecoveryRecord'
    EmbeddingCutoverPlanStatus:
      type: string
      enum: [draft, active, paused, cancelled, completed]
    EmbeddingCutoverItemStatus:
      type: string
      enum: [queued, rebuilding, current, failed, skipped, paused, cancelled]
    EmbeddingCutoverTarget:
      type: object
      required:
        - provider
        - model
        - dimensions
      properties:
        provider:
          type: string
        model:
          type: string
        dimensions:
          type: integer
    EmbeddingCutoverProgress:
      type: object
      required:
        - total
        - queued
        - rebuilding
        - current
        - failed
        - skipped
        - paused
        - cancelled
      properties:
        total:
          type: integer
        queued:
          type: integer
        rebuilding:
          type: integer
        current:
          type: integer
        failed:
          type: integer
        skipped:
          type: integer
        paused:
          type: integer
        cancelled:
          type: integer
    EmbeddingCutoverItem:
      type: object
      required:
        - plan_id
        - memory_id
        - scope
        - class
        - status
      properties:
        plan_id:
          type: string
        memory_id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        class:
          type: string
        status:
          $ref: '#/components/schemas/EmbeddingCutoverItemStatus'
        failure_reason:
          type: string
        active_vector_revision_id:
          type: string
        active_provider:
          type: string
        active_model:
          type: string
        active_dimensions:
          type: integer
        requested_at:
          type: string
          format: date-time
        last_attempted_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time
    EmbeddingCutoverPlanRequest:
      type: object
      required:
        - target
        - wave_size
        - reason
      properties:
        target:
          $ref: '#/components/schemas/EmbeddingCutoverTarget'
        classes:
          type: array
          items:
            type: string
        wave_size:
          type: integer
        reason:
          type: string
    EmbeddingCutoverActionRequest:
      type: object
      required:
        - reason
      properties:
        reason:
          type: string
    DiagnosticFinding:
      type: object
      required:
        - severity
        - code
      properties:
        severity:
          type: string
          enum: [blocker, warning]
        code:
          type: string
        message:
          type: string
        component:
          type: string
        metadata:
          type: object
          additionalProperties:
            type: string
    EmbeddingCutoverClassBreakdown:
      type: object
      required:
        - class
        - eligible
        - drifted
        - missing_active_vector
        - missing_route
      properties:
        class:
          type: string
        eligible:
          type: integer
        drifted:
          type: integer
        missing_active_vector:
          type: integer
        missing_route:
          type: integer
    EmbeddingCutoverPlanSummary:
      type: object
      required:
        - id
        - status
      properties:
        id:
          type: string
        status:
          $ref: '#/components/schemas/EmbeddingCutoverPlanStatus'
    EmbeddingCutoverPreflightReport:
      type: object
      required:
        - component
        - decision
        - target
        - scope
        - eligible_total
        - observed_at
      properties:
        component:
          type: string
        decision:
          type: string
          enum: [allow, deny]
        blockers:
          type: array
          items:
            $ref: '#/components/schemas/DiagnosticFinding'
        warnings:
          type: array
          items:
            $ref: '#/components/schemas/DiagnosticFinding'
        target:
          $ref: '#/components/schemas/EmbeddingCutoverTarget'
        scope:
          $ref: '#/components/schemas/Scope'
        eligible_total:
          type: integer
        class_breakdown:
          type: array
          items:
            $ref: '#/components/schemas/EmbeddingCutoverClassBreakdown'
        conflicting_plan:
          $ref: '#/components/schemas/EmbeddingCutoverPlanSummary'
        observed_at:
          type: string
          format: date-time
    EmbeddingCutoverPlan:
      type: object
      required:
        - id
        - scope
        - target
        - wave_size
        - status
        - reason
        - created_by
        - created_at
        - progress
      properties:
        id:
          type: string
        scope:
          $ref: '#/components/schemas/Scope'
        target:
          $ref: '#/components/schemas/EmbeddingCutoverTarget'
        classes:
          type: array
          items:
            type: string
        wave_size:
          type: integer
        status:
          $ref: '#/components/schemas/EmbeddingCutoverPlanStatus'
        reason:
          type: string
        created_by:
          type: string
        created_at:
          type: string
          format: date-time
        last_action_by:
          type: string
        last_action_reason:
          type: string
        last_action_at:
          type: string
          format: date-time
        activated_at:
          type: string
          format: date-time
        paused_at:
          type: string
          format: date-time
        cancelled_at:
          type: string
          format: date-time
        completed_at:
          type: string
          format: date-time
        progress:
          $ref: '#/components/schemas/EmbeddingCutoverProgress'
        items:
          type: array
          items:
            $ref: '#/components/schemas/EmbeddingCutoverItem'
    EmbeddingCutoverPlanListResponse:
      type: object
      required:
        - plans
      properties:
        plans:
          type: array
          items:
            $ref: '#/components/schemas/EmbeddingCutoverPlan'
    EmbeddingMemoryInspection:
      type: object
      required:
        - runtime
        - memory
        - rebuild
        - revisions
      properties:
        runtime:
          $ref: '#/components/schemas/EmbeddingRuntimeStatus'
        memory:
          $ref: '#/components/schemas/EmbeddingMemorySummary'
        rebuild:
          $ref: '#/components/schemas/EmbeddingRebuildView'
        revisions:
          type: array
          items:
            $ref: '#/components/schemas/EmbeddingVectorRevision'
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
    AdminCreateMemoryRequest:
      type: object
      required:
        - class
        - content
        - reason
      properties:
        class:
          type: string
        content:
          type: string
        reason:
          type: string
    AdminUpdateMemoryRequest:
      type: object
      required:
        - content
        - expected_version
        - reason
      properties:
        content:
          type: string
        expected_version:
          type: integer
        reason:
          type: string
    AdminMergeMemoryRequest:
      type: object
      required:
        - source_memory_id
        - content
        - expected_version
        - reason
      properties:
        source_memory_id:
          type: string
        content:
          type: string
        expected_version:
          type: integer
        reason:
          type: string
    AdminReclassifyMemoryRequest:
      type: object
      required:
        - target_class
        - expected_version
        - reason
      properties:
        target_class:
          type: string
        expected_version:
          type: integer
        reason:
          type: string
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
