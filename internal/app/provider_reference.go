package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

// Reference routes reauthorize through the current grant before accessing any
// durable record. Missing, hidden and foreign records share unavailable.
func registerProviderReferenceRoutes(mux *http.ServeMux, deps HTTPDependencies) {
	for _, operation := range []string{"write", "status", "lookup"} {
		path := "/v1/provider/references"
		if operation != "write" {
			path += "/" + operation
		}
		mux.Handle("POST "+path, provider.RuntimeBindingMiddleware(deps.ProviderBindings, deps.PrincipalAuthorizer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := provider.RuntimeBindingFromContext(r.Context())
			data, err := io.ReadAll(io.LimitReader(r.Body, 32769))
			if err != nil {
				writeProviderError(w, 400, "validation", "invalid_request", "invalid reference request", false)
				return
			}
			if len(data) > 32768 {
				writeProviderError(w, 413, "validation", "reference_limit_exceeded", "reference request exceeds allocation", false)
				return
			}
			var meta provider.OperationMetadata
			var write provider.ReferenceWriteRequest
			var status provider.ReferenceStatusRequest
			var lookup provider.ReferenceLookupRequest
			switch operation {
			case "write":
				write, err = provider.DecodeReferenceWriteRequest(data)
				meta = write.Metadata
			case "status":
				status, err = provider.DecodeReferenceStatusRequest(data)
				meta = status.Metadata
			case "lookup":
				lookup, err = provider.DecodeReferenceLookupRequest(data)
				meta = lookup.Metadata
			}
			if err != nil {
				if strings.Contains(err.Error(), "unsupported reference contract") {
					writeProviderError(w, 409, "compatibility", "unsupported_reference_contract", "unsupported reference contract", false)
				} else {
					writeProviderError(w, 400, "validation", "invalid_request", "invalid reference request", false)
				}
				return
			}
			if !supportsProviderSchema(deps.ProviderSchemaVersions, meta.SchemaVersion) {
				writeProviderCompatibilityError(w, deps.ProviderSchemaVersions)
				return
			}
			boundScope := provider.CanonicalScopeDescriptor{Tenant: b.Scope.Tenant, Project: b.Scope.Project, Namespace: b.Scope.Namespace, AgentID: b.AgentID, SessionID: b.SessionID, ConversationID: b.ConversationID, ProviderInstanceID: b.ProviderInstanceID}
			if boundScope.Validate() != nil {
				writeProviderError(w, 400, "validation", "invalid_reference_scope", "runtime scope exceeds durable reference limits", false)
				return
			}
			e, ok := currentDisclosure(w, r, deps.PrincipalAuthorizer, b.PrincipalID, b.Scope)
			if !ok {
				return
			}
			if deps.ProviderReferences == nil || deps.ProviderAdapter == nil {
				writeProviderError(w, 503, "dependency", "reference_unavailable", "durable references unavailable", true)
				return
			}
			var out provider.ReferenceResponse
			if operation == "write" {
				if write.Input.SourceMetadata != nil && write.Input.SourceMetadata.Trust == memory.SourceTrustAttestedData && !e.SourceAttestation {
					writeProviderError(w, 403, "scope", "disclosure_denied", "disclosure denied", false)
					return
				}
				out, err = deps.ProviderAdapter.WriteReference(r.Context(), b, write)
			} else {
				profile, mode := status.Input.DisclosureProfile, status.Input.ResponseMode
				if operation == "lookup" {
					profile, mode = lookup.Input.DisclosureProfile, lookup.Input.ResponseMode
				}
				decision, resolveErr := e.Resolve(profile, mode)
				if resolveErr != nil {
					writeProviderError(w, 403, "scope", "disclosure_denied", "disclosure denied", false)
					return
				}
				if operation == "status" {
					out, err = deps.ProviderReferences.ReferenceStatus(r.Context(), b.Scope, b.PrincipalID, status.Input.EventID, decision.Profile, decision.ResponseMode)
				} else {
					out, err = deps.ProviderReferences.ReferenceLookup(r.Context(), b.Scope, b.PrincipalID, lookup.Input.ReferenceID, lookup.Input.MemoryPath, decision.Profile, decision.ResponseMode)
				}
				if decision.ResponseMode != auth.ResponseModeBoundedContent && out.Reference != nil {
					copy := *out.Reference
					copy.Value = ""
					out.Reference = &copy
				}
			}
			if err != nil {
				if errors.Is(err, memory.ErrIdempotencyConflict) || strings.Contains(err.Error(), "idempotency conflict") {
					writeProviderError(w, 409, "conflict", "reference_conflict", "reference write conflicts with prior request", false)
				} else {
					writeProviderError(w, 503, "dependency", "reference_unavailable", "durable references unavailable", true)
				}
				return
			}
			out.Metadata = meta
			out.ContractVersion = provider.DurableReferenceContractVersion
			out.Scope = boundScope
			payload, err := json.Marshal(out)
			if err != nil || len(payload) > provider.MaxReferenceResponseBytes {
				writeProviderError(w, 503, "dependency", "reference_unavailable", "durable references unavailable", true)
				return
			}
			code := 200
			if operation == "write" {
				code = 202
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			_, _ = w.Write(payload)
		})))
	}
}
