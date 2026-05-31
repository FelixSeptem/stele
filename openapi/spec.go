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
        - in: header
          name: X-API-Key
          required: true
          schema:
            type: string
        - in: header
          name: X-Stele-Tenant
          required: true
          schema:
            type: string
        - in: header
          name: X-Stele-Project
          required: true
          schema:
            type: string
        - in: header
          name: X-Stele-Namespace
          required: true
          schema:
            type: string
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
components:
  schemas:
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
`
}
