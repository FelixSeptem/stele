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
	"reflect"

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

func (c *ProviderContextContract) Validate(data []byte, scope memory.Scope, schemaVersion string) (provider.DisclosedContextResponse, error) {
	var response provider.DisclosedContextResponse
	fail := func() (provider.DisclosedContextResponse, error) {
		return provider.DisclosedContextResponse{}, fmt.Errorf("provider context failed-contract")
	}
	if c == nil || c.schema == nil || scope.Validate() != nil {
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
	accounting := response.Result.BudgetAccounting
	u := provider.ContextBudgetUsage{ResponseBytes: len(data)}
	if len(data) == 0 || data[len(data)-1] == '\n' || accounting.ContractVersion != provider.ContextBudgetVersion || accounting.PackingPolicyVersion != provider.ContextPackingPolicyVersion {
		return fail()
	}
	authorized := make(map[provider.Citation]bool)
	for _, section := range [][]provider.DisclosedContextItem{response.Result.Profile, response.Result.RecentSession, response.Result.RecentEpisodes, response.Result.RelevantSummaries, response.Result.RelatedEntities} {
		for _, item := range section {
			u.Items++
			if item.Memory != nil {
				u.ContentBytes += len(item.Memory.Content)
			}
			ref := item.Reference
			if item.Memory != nil {
				ref = &item.Memory.ContextReference
			}
			if ref == nil || !ref.SourceMetadata.Disclosable() || ref.Availability != "available" || len(item.Citations) != 1 {
				return fail()
			}
			for _, citation := range item.Citations {
				if citation.SourceKind != "canonical_memory" || citation.Reference != ref.MemoryID || citation.Version != ref.Version || citation.Availability != "available" {
					return fail()
				}
				authorized[citation] = true
			}
		}
	}
	// Scope is authenticated by the HTTP binding. The closed public DTO omits
	// scope echoes; evidence may only name an item actually returned by that binding.
	for _, citations := range [][]provider.Citation{response.Citations, response.Result.Citations} {
		seen := map[provider.Citation]bool{}
		for _, citation := range citations {
			if !authorized[citation] || seen[citation] {
				return fail()
			}
			seen[citation] = true
		}
		if len(seen) != len(authorized) {
			return fail()
		}
	}
	u.Citations = len(authorized)
	e := accounting.Effective
	if accounting.Usage != u || u.Items > e.MaxItems || u.ContentBytes > e.MaxContentBytes || u.Citations > e.MaxCitations || u.ResponseBytes > e.MaxResponseBytes {
		return fail()
	}
	if e.MaxItems > accounting.Requested.MaxItems {
		return fail()
	}
	for _, x := range []struct {
		requested *int
		effective int
	}{{accounting.Requested.MaxContentBytes, e.MaxContentBytes}, {accounting.Requested.MaxResponseBytes, e.MaxResponseBytes}, {accounting.Requested.MaxCitations, e.MaxCitations}} {
		if x.requested != nil && x.effective > *x.requested {
			return fail()
		}
	}
	ordered := func(values, allowed []string) bool {
		last := -1
		for _, v := range values {
			index := -1
			for i, a := range allowed {
				if a == v {
					index = i
					break
				}
			}
			if index <= last {
				return false
			}
			last = index
		}
		return true
	}
	s := accounting.Selection
	if !ordered(s.OmissionReasons, []string{"item_limit", "content_bytes", "citations", "response_bytes"}) || !ordered(s.UnsupportedSections, []string{"experience_insights", "goal_context", "diagnostics", "feedback_diagnostics"}) {
		return fail()
	}
	if (u.Items > 0) != (s.Outcome == "selected") || s.Outcome == "empty" && len(s.OmissionReasons) > 0 || s.Outcome == "budget_exhausted" && len(s.OmissionReasons) == 0 {
		return fail()
	}
	return response, nil
}

type ProviderHTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// ValidateForRequest binds self-consistent accounting to the originating
// allocation. When discovery is available, it also verifies the exact ceilings.
func (c *ProviderContextContract) ValidateForRequest(data []byte, scope memory.Scope, schemaVersion string, input provider.ContextInput, limits *provider.ProviderLimits) (provider.DisclosedContextResponse, error) {
	r, err := c.Validate(data, scope, schemaVersion)
	if err != nil {
		return r, err
	}
	fail := func() (provider.DisclosedContextResponse, error) {
		return provider.DisclosedContextResponse{}, fmt.Errorf("provider context failed-contract")
	}
	if input.ValidateBudget() != nil {
		return fail()
	}
	want := provider.ContextBudgetRequested{MaxItems: input.Budget}
	if input.BudgetLimits != nil {
		want.MaxContentBytes = input.BudgetLimits.MaxContentBytes
		want.MaxResponseBytes = input.BudgetLimits.MaxResponseBytes
		want.MaxCitations = input.BudgetLimits.MaxCitations
	}
	accounting := r.Result.BudgetAccounting
	if !reflect.DeepEqual(want, accounting.Requested) {
		return fail()
	}
	unsupported := []string{}
	for _, section := range []struct {
		requested bool
		name      string
	}{{input.IncludeExperienceInsights, "experience_insights"}, {input.IncludeGoalContext, "goal_context"}, {input.IncludeDiagnostics, "diagnostics"}, {input.IncludeFeedbackDiagnostics, "feedback_diagnostics"}} {
		if section.requested {
			unsupported = append(unsupported, section.name)
		}
	}
	if !reflect.DeepEqual(unsupported, accounting.Selection.UnsupportedSections) {
		return fail()
	}
	if limits != nil {
		l := limits.Resolved()
		cap := func(requested *int, ceiling int) int {
			if requested != nil {
				return min(*requested, ceiling)
			}
			return ceiling
		}
		effective := provider.ContextBudgetEffective{MaxItems: min(input.Budget, l.MaxContextItems), MaxContentBytes: cap(want.MaxContentBytes, l.MaxContextContentBytes), MaxResponseBytes: cap(want.MaxResponseBytes, l.MaxContextBytes), MaxCitations: cap(want.MaxCitations, l.MaxCitations)}
		if effective != accounting.Effective {
			return fail()
		}
	}
	return r, nil
}

// ProviderContextHTTPExecutor requires the HTTP boundary and schema evidence.
// Credentials are transport configuration and never copied into reports.
type ProviderContextHTTPExecutor struct {
	Client        ProviderHTTPDoer
	BaseURL       string
	Headers       http.Header
	SchemaVersion string
	Contract      *ProviderContextContract
	Limits        *provider.ProviderLimits
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
	input := provider.ContextInput{Query: fixture.ID, Budget: 1}
	body, err := json.Marshal(provider.ContextRequest{Metadata: provider.OperationMetadata{RequestID: "conformance-" + fixture.ID, OperationID: fixture.Operation, SchemaVersion: e.SchemaVersion}, Input: input})
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
	if _, err = e.Contract.ValidateForRequest(data, binding.Scope, e.SchemaVersion, input, e.Limits); err != nil {
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
