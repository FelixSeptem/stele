package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/provider"
)

func writeProjectionAccessError(w http.ResponseWriter, message string, status int) {
	switch {
	case status == http.StatusForbidden && message == "unauthorized":
		writeProviderError(w, status, "authentication", "unauthorized", "authentication denied", false)
	case status == http.StatusForbidden:
		writeProviderError(w, status, "scope", "scope_denied", "scope denied", false)
	case status == http.StatusBadRequest:
		writeProviderError(w, status, "validation", "invalid_scope", "invalid scope", false)
	default:
		writeProviderError(w, 503, "dependency", "authorization_unavailable", "authorization unavailable", true)
	}
}

// Dedicated routes stay guarded even when the projection store is unavailable.
func registerProviderProjectionRoutes(mux *http.ServeMux, deps HTTPDependencies) {
	for _, history := range []bool{false, true} {
		path := "/v1/provider/projections"
		if history {
			path += "/history"
		}
		mux.Handle("POST "+path, provider.RuntimeBindingMiddlewareWithDenialWriter(deps.ProviderBindings, deps.PrincipalAuthorizer, func(w http.ResponseWriter) {
			writeProviderError(w, 403, "scope", "runtime_binding_denied", "runtime binding denied", false)
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := provider.RuntimeBindingFromContext(r.Context())
			data, err := io.ReadAll(io.LimitReader(r.Body, provider.MaxProjectionRequestBytes+1))
			if err != nil {
				writeProviderError(w, 400, "validation", "invalid_request", "invalid projection request", false)
				return
			}
			if len(data) > provider.MaxProjectionRequestBytes {
				writeProviderError(w, 413, "validation", "projection_limit_exceeded", "projection request exceeds allocation", false)
				return
			}
			var req provider.ProjectionRequest
			var hist provider.ProjectionHistoryRequest
			var meta provider.OperationMetadata
			if history {
				hist, err = provider.DecodeProjectionHistoryRequest(data)
				meta = hist.Metadata
			} else {
				req, err = provider.DecodeProjectionRequest(data)
				meta = req.Metadata
			}
			if err != nil {
				if strings.Contains(err.Error(), "unsupported projection contract") {
					writeProviderError(w, 409, "compatibility", "unsupported_projection_contract", "unsupported projection contract", false)
				} else {
					writeProviderError(w, 400, "validation", "invalid_request", "invalid projection request", false)
				}
				return
			}
			if !supportsProviderSchema(deps.ProviderSchemaVersions, meta.SchemaVersion) {
				writeProviderError(w, 409, "compatibility", "unsupported_schema", "unsupported provider schema version", false)
				return
			}
			scope := provider.ProjectionScope(b)
			if scope.Validate() != nil {
				writeProviderError(w, 400, "validation", "invalid_projection_scope", "runtime scope exceeds projection limits", false)
				return
			}
			e, ok := currentDisclosure(w, r, deps.PrincipalAuthorizer, b.PrincipalID, b.Scope)
			if !ok {
				return
			}
			if _, err = e.Resolve("", auth.ResponseModeReferenceOnly); err != nil {
				writeProviderError(w, 403, "scope", "disclosure_denied", "disclosure denied", false)
				return
			}
			if deps.ProviderProjections == nil {
				writeProjectionError(w, errors.New("unconfigured"))
				return
			}
			var out any
			code := http.StatusAccepted
			if history {
				var records []provider.ProjectionRecord
				var more bool
				records, more, err = deps.ProviderProjections.ProjectionHistory(r.Context(), b, hist.Input)
				if err == nil {
					if records == nil {
						records = []provider.ProjectionRecord{}
					}
					if len(records) > hist.Input.Limit {
						err = errors.New("history allocation")
					}
					for _, record := range records {
						if validateErr := record.Validate(); validateErr != nil {
							err = validateErr
							break
						}
					}
				}
				out = provider.ProjectionHistoryResponse{Metadata: meta, ContractVersion: provider.ProjectionContractVersion, Scope: scope, Records: records, HasMore: more}
				code = http.StatusOK
			} else {
				var record provider.ProjectionRecord
				record, err = deps.ProviderProjections.AdmitProjection(r.Context(), b, req)
				if err == nil {
					err = record.Validate()
				}
				out = provider.ProjectionResponse{Metadata: meta, ContractVersion: provider.ProjectionContractVersion, Scope: scope, Record: record}
			}
			if err != nil {
				writeProjectionError(w, err)
				return
			}
			payload, err := json.Marshal(out)
			if err != nil || len(payload) > provider.MaxProjectionResponseBytes {
				writeProjectionError(w, errors.New("response allocation"))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			_, _ = w.Write(payload)
		})))
	}
}

func writeProjectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, provider.ErrProjectionConflict):
		writeProviderError(w, 409, "conflict", "projection_conflict", "projection conflicts with prior evidence", false)
	case errors.Is(err, provider.ErrProjectionStale):
		writeProviderError(w, 409, "stale", "projection_stale", "projection revision is stale", false)
	case errors.Is(err, provider.ErrProjectionUnavailable):
		writeProviderError(w, 410, "lifecycle", "projection_unavailable", "projection unavailable", false)
	default:
		writeProviderError(w, 503, "dependency", "projection_dependency_unavailable", "projection service unavailable", true)
	}
}
