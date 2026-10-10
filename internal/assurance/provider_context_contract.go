package assurance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
	"github.com/FelixSeptem/stele/openapi"
	"github.com/getkin/kin-openapi/openapi3"
)

// ProviderContextContract validates actual public JSON against the authoritative
// OpenAPI response. Errors are deliberately bounded; schema errors may contain
// customer values and must not be retained as conformance evidence.
type ProviderContextContract struct {
	schema *openapi3.Schema
	Digest string
}

func NewProviderContextContract(spec []byte) (*ProviderContextContract, error) {
	doc, err := openapi3.NewLoader().LoadFromData(spec)
	if err != nil || doc == nil {
		return nil, fmt.Errorf("context OpenAPI unavailable")
	}
	if err = doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("context OpenAPI invalid")
	}
	path := doc.Paths.Value("/v1/provider/context")
	if path == nil || path.Post == nil || path.Post.Responses.Value("200") == nil {
		return nil, fmt.Errorf("context response schema missing")
	}
	media := path.Post.Responses.Value("200").Value.Content.Get("application/json")
	if media == nil || media.Schema == nil || media.Schema.Ref != "#/components/schemas/ProviderContextResponse" || media.Schema.Value == nil {
		return nil, fmt.Errorf("dedicated context response schema missing")
	}
	sum := sha256.Sum256(spec)
	return &ProviderContextContract{schema: media.Schema.Value, Digest: hex.EncodeToString(sum[:])}, nil
}

func (c *ProviderContextContract) Validate(data []byte, scope memory.Scope, schemaVersion string) (provider.ContextResponse, error) {
	var response provider.ContextResponse
	fail := func() (provider.ContextResponse, error) {
		return provider.ContextResponse{}, fmt.Errorf("provider context failed-contract")
	}
	if c == nil || c.schema == nil {
		return fail()
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return fail()
	}
	if err := c.schema.VisitJSON(value); err != nil {
		return fail()
	}
	if err := provider.DecodeStrict(data, &response); err != nil {
		return fail()
	}
	if err := response.Metadata.NormalizeValidate(); err != nil || response.Metadata.SchemaVersion != schemaVersion {
		return fail()
	}
	for _, section := range [][]provider.ContextItem{response.Result.Profile, response.Result.RecentSession, response.Result.RecentEpisodes, response.Result.RelevantSummaries, response.Result.RelatedEntities} {
		for _, item := range section {
			if item.Memory.Scope.Normalized() != scope.Normalized() || item.Memory.State != memory.MemoryStateActive {
				return fail()
			}
		}
	}
	for _, section := range [][]provider.ContextInsightItem{response.Result.KnownFailures, response.Result.ExperienceLessons} {
		for _, item := range section {
			if item.Insight.Scope.Normalized() != scope.Normalized() || item.Insight.State != memory.DerivedInsightStateActive {
				return fail()
			}
		}
	}
	return response, nil
}

type ProviderHTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// ProviderContextHTTPExecutor requires the HTTP boundary and schema evidence.
// Credentials are transport configuration and never copied into reports.
type ProviderContextHTTPExecutor struct {
	Client        ProviderHTTPDoer
	BaseURL       string
	Headers       http.Header
	SchemaVersion string
	Contract      *ProviderContextContract
	Fallback      ProviderFixtureExecutor
}

func (e ProviderContextHTTPExecutor) ExecuteProviderFixture(ctx context.Context, binding provider.RuntimeBinding, fixture ProviderFixture) (ProviderFixtureOutcome, error) {
	if fixture.Kind != ProviderFixtureContext {
		if e.Fallback != nil {
			return e.Fallback.ExecuteProviderFixture(ctx, binding, fixture)
		}
		return ProviderFixtureOutcome{}, fmt.Errorf("HTTP fixture unavailable")
	}
	if e.Client == nil || e.Contract == nil || e.SchemaVersion == "" {
		return ProviderFixtureOutcome{}, fmt.Errorf("context public evidence unavailable")
	}
	body, err := json.Marshal(provider.ContextRequest{Metadata: provider.OperationMetadata{RequestID: "conformance-" + fixture.ID, OperationID: fixture.Operation, SchemaVersion: e.SchemaVersion}, Input: provider.ContextInput{Query: fixture.ID, Budget: 1}})
	if err != nil {
		return ProviderFixtureOutcome{}, fmt.Errorf("context fixture invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.BaseURL+"/v1/provider/context", bytes.NewReader(body))
	if err != nil {
		return ProviderFixtureOutcome{}, fmt.Errorf("context transport invalid")
	}
	request.Header = e.Headers.Clone()
	if request.Header == nil {
		request.Header = make(http.Header)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(provider.HeaderRuntimeBinding, binding.BindingID)
	request.Header.Set(provider.HeaderRuntimeSession, binding.SessionID)
	request.Header.Set("X-Stele-Tenant", binding.Scope.Tenant)
	request.Header.Set("X-Stele-Project", binding.Scope.Project)
	request.Header.Set("X-Stele-Namespace", binding.Scope.Namespace)
	response, err := e.Client.Do(request)
	if err != nil {
		return ProviderFixtureOutcome{}, fmt.Errorf("context HTTP unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ProviderFixtureOutcome{}, fmt.Errorf("context HTTP failed-contract")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20+1))
	if err != nil || len(data) > 4<<20 {
		return ProviderFixtureOutcome{}, fmt.Errorf("context fixture response unavailable")
	}
	if _, err = e.Contract.Validate(data, binding.Scope, e.SchemaVersion); err != nil {
		return ProviderFixtureOutcome{}, err
	}
	return ProviderFixtureOutcome{Passed: true, Evidence: []ProviderEvidenceKind{ProviderEvidenceContextHTTP, ProviderEvidenceContextSchema}, OpenAPIDigest: e.Contract.Digest}, nil
}

func DefaultProviderContextContract() (*ProviderContextContract, error) {
	return NewProviderContextContract([]byte(openapi.SpecYAML()))
}

func validContextDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}
