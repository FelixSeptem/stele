package provider

import (
	"encoding/json"
	"fmt"
	"github.com/FelixSeptem/stele/internal/auth"
	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

const ContextBudgetVersion = "context-budget-v1"
const ContextPackingPolicyVersion = "context-packing-v1"
const MaxPortableBudget = 2147483647

type ContextBudgetLimits struct {
	MaxContentBytes  *int `json:"max_content_bytes,omitempty"`
	MaxResponseBytes *int `json:"max_response_bytes,omitempty"`
	MaxCitations     *int `json:"max_citations,omitempty"`
}
type ContextBudgetRequested struct {
	MaxItems         int  `json:"max_items"`
	MaxContentBytes  *int `json:"max_content_bytes"`
	MaxResponseBytes *int `json:"max_response_bytes"`
	MaxCitations     *int `json:"max_citations"`
}
type ContextBudgetEffective struct {
	MaxItems         int `json:"max_items"`
	MaxContentBytes  int `json:"max_content_bytes"`
	MaxResponseBytes int `json:"max_response_bytes"`
	MaxCitations     int `json:"max_citations"`
}
type ContextBudgetUsage struct {
	Items         int `json:"items"`
	ContentBytes  int `json:"content_bytes"`
	ResponseBytes int `json:"response_bytes"`
	Citations     int `json:"citations"`
}
type ContextBudgetSelection struct {
	Outcome             string   `json:"outcome"`
	OmissionReasons     []string `json:"omission_reasons"`
	UnsupportedSections []string `json:"unsupported_sections"`
}
type ContextBudgetAccounting struct {
	ContractVersion      string                 `json:"contract_version"`
	PackingPolicyVersion string                 `json:"packing_policy_version"`
	Requested            ContextBudgetRequested `json:"requested"`
	Effective            ContextBudgetEffective `json:"effective"`
	Usage                ContextBudgetUsage     `json:"usage"`
	Selection            ContextBudgetSelection `json:"selection"`
}

func (i ContextInput) ValidateBudget() error {
	if i.Budget < 1 || i.Budget > MaxPortableBudget {
		return fmt.Errorf("invalid item budget")
	}
	if i.BudgetLimits != nil {
		for _, v := range []*int{i.BudgetLimits.MaxContentBytes, i.BudgetLimits.MaxCitations} {
			if v != nil && (*v < 0 || *v > MaxPortableBudget) {
				return fmt.Errorf("invalid budget limit")
			}
		}
		if v := i.BudgetLimits.MaxResponseBytes; v != nil && (*v < 1 || *v > MaxPortableBudget) {
			return fmt.Errorf("invalid response budget")
		}
	}
	return nil
}

// Resolved supports older in-process dependency literals. Config loading always
// supplies the independent item field, so an explicit zero content cap survives.
func (l ProviderLimits) Resolved() ProviderLimits {
	if l.MaxContextBytes <= 0 {
		l.MaxContextBytes = 1 << 20
	}
	if l.MaxContextItems == 0 {
		l.MaxContextItems = 100
		l.MaxContextContentBytes = l.MaxContextBytes
	}
	if l.MaxCitations <= 0 {
		l.MaxCitations = 100
	}
	return l
}

type ContextAdmission struct {
	turnID        string
	continuity    ContextContinuityResult
	fingerprint   string
	scope         memory.Scope
	decision      auth.DisclosureDecision
	metadata      OperationMetadata
	accounting    ContextBudgetAccounting
	selected      retrieval.AssembledContext
	eligible      bool
	reasons       map[string]bool
	reservedBytes int
	sectionCounts map[string]int
	citationSeen  map[Citation]bool
}

func NewContextAdmission(input ContextInput, limits ProviderLimits, scope memory.Scope, decision auth.DisclosureDecision, meta OperationMetadata) (*ContextAdmission, error) {
	if err := input.ValidateBudget(); err != nil {
		return nil, err
	}
	limits = limits.Resolved()
	r := ContextBudgetRequested{MaxItems: input.Budget}
	if input.BudgetLimits != nil {
		r.MaxContentBytes = input.BudgetLimits.MaxContentBytes
		r.MaxResponseBytes = input.BudgetLimits.MaxResponseBytes
		r.MaxCitations = input.BudgetLimits.MaxCitations
	}
	cap := func(p *int, ceiling int) int {
		if p != nil && *p < ceiling {
			return *p
		}
		return ceiling
	}
	a := &ContextAdmission{scope: scope, decision: decision, metadata: SafeOperationMetadata(meta), reasons: map[string]bool{}, accounting: ContextBudgetAccounting{ContractVersion: ContextBudgetVersion, PackingPolicyVersion: ContextPackingPolicyVersion, Requested: r, Effective: ContextBudgetEffective{MaxItems: min(input.Budget, limits.MaxContextItems), MaxContentBytes: cap(r.MaxContentBytes, limits.MaxContextContentBytes), MaxResponseBytes: cap(r.MaxResponseBytes, limits.MaxContextBytes), MaxCitations: cap(r.MaxCitations, limits.MaxCitations)}, Selection: ContextBudgetSelection{Outcome: "empty", OmissionReasons: []string{}, UnsupportedSections: []string{}}}}
	for _, s := range []struct {
		yes  bool
		name string
	}{{input.IncludeExperienceInsights, "experience_insights"}, {input.IncludeGoalContext, "goal_context"}, {input.IncludeDiagnostics, "diagnostics"}, {input.IncludeFeedbackDiagnostics, "feedback_diagnostics"}} {
		if s.yes {
			a.accounting.Selection.UnsupportedSections = append(a.accounting.Selection.UnsupportedSections, s.name)
		}
	}
	a.fingerprint = SelectionPolicyFingerprint([]byte(DefaultContextPolicyManifest))
	a.continuity = ContextContinuityResult{ContractVersion: ContextContinuityVersion, ReadSemantics: "fresh", Outcome: "not_requested"}
	if input.Continuity != nil {
		a.continuity.Outcome = "issued"
		if input.Continuity.Mode == "compare" {
			a.continuity.ReadSemantics = "fresh_compare"
			a.continuity.Outcome = "unchanged"
		}
		a.continuity.Reference = "ctxc_00000000000000000000000000000000"
		a.continuity.ExpiresAt = "2000-01-01T00:00:00Z"
	}
	// Preflight the largest bounded empty accounting envelope before retrieval.
	response := a.response(a.selected)
	a.reservedBytes = a.reservedSize(response)
	a.sectionCounts = map[string]int{}
	a.citationSeen = map[Citation]bool{}
	if a.reservedBytes > a.accounting.Effective.MaxResponseBytes {
		return nil, fmt.Errorf("context_limit_exceeded")
	}
	return a, nil
}

func appendContextHit(out *retrieval.AssembledContext, section string, hit retrieval.SearchHit) {
	switch section {
	case "profile":
		out.Profile = append(out.Profile, hit)
	case "recent_session":
		out.RecentSession = append(out.RecentSession, hit)
	case "recent_episodes":
		out.RecentEpisodes = append(out.RecentEpisodes, hit)
	case "relevant_summaries":
		out.RelevantSummaries = append(out.RelevantSummaries, hit)
	case "related_entities":
		out.RelatedEntities = append(out.RelatedEntities, hit)
	}
}
func (a *ContextAdmission) AdmitContextItem(section string, hit retrieval.SearchHit) bool {
	// Project first: ineligible evidence contributes no costs or public reasons.
	one := retrieval.AssembledContext{}
	appendContextHit(&one, section, hit)
	if contextUsage(ShapeDisclosedContextResponse(one, a.metadata, a.decision, 1000, a.scope)).Items == 0 {
		return false
	}
	a.eligible = true
	projected := ShapeDisclosedContextResponse(one, a.metadata, a.decision, 1000, a.scope)
	var item DisclosedContextItem
	for _, items := range [][]DisclosedContextItem{projected.Result.Profile, projected.Result.RecentSession, projected.Result.RecentEpisodes, projected.Result.RelevantSummaries, projected.Result.RelatedEntities} {
		if len(items) > 0 {
			item = items[0]
			break
		}
	}
	itemBytes, err := json.Marshal(item)
	if err != nil {
		return false
	}
	usage := a.accounting.Usage
	usage.Items++
	if item.Memory != nil {
		usage.ContentBytes += len(item.Memory.Content)
	}
	delta := len(itemBytes)
	if a.sectionCounts[section] > 0 {
		delta++
	}
	citation := item.Citations[0]
	if !a.citationSeen[citation] {
		usage.Citations++
		citationBytes, err := json.Marshal(citation)
		if err != nil {
			return false
		}
		delta += 2 * len(citationBytes)
		if len(a.citationSeen) > 0 {
			delta += 2
		}
	}
	e := a.accounting.Effective
	rejected := false
	for _, x := range []struct {
		over   bool
		reason string
	}{{usage.Items > e.MaxItems, "item_limit"}, {usage.ContentBytes > e.MaxContentBytes, "content_bytes"}, {usage.Citations > e.MaxCitations, "citations"}, {a.reservedBytes+delta > e.MaxResponseBytes, "response_bytes"}} {
		if x.over {
			a.reasons[x.reason] = true
			rejected = true
		}
	}
	if rejected {
		return false
	}
	appendContextHit(&a.selected, section, hit)
	a.accounting.Usage = usage
	a.reservedBytes += delta
	a.sectionCounts[section]++
	a.citationSeen[citation] = true
	return true
}
func (a *ContextAdmission) NoteContextItemLimit(hit retrieval.SearchHit) {
	// Internal planner quotas may stop selection before the public allocation.
	// Only a binding public item ceiling can produce this public reason.
	if a.accounting.Usage.Items < a.accounting.Effective.MaxItems {
		return
	}
	one := retrieval.AssembledContext{Profile: []retrieval.SearchHit{hit}}
	if contextUsage(ShapeDisclosedContextResponse(one, a.metadata, a.decision, 1000, a.scope)).Items > 0 {
		a.eligible = true
		a.reasons["item_limit"] = true
	}
}
func (a *ContextAdmission) response(out retrieval.AssembledContext) DisclosedContextResponse {
	r := ShapeDisclosedContextResponse(out, a.metadata, a.decision, 1000, a.scope)
	r.Result.TurnID = a.turnID
	r.Result.BudgetAccounting = a.accounting
	r.Result.Continuity = a.continuity
	r.Result.ContextDigest = ContextDigest{ContextDigestVersion, "sha-256", ContextCanonicalVersion, ContextSelectionVersion, a.fingerprint, a.fingerprint}
	return r
}
func contextUsage(r DisclosedContextResponse) ContextBudgetUsage {
	u := ContextBudgetUsage{Citations: len(r.Citations)}
	for _, section := range [][]DisclosedContextItem{r.Result.Profile, r.Result.RecentSession, r.Result.RecentEpisodes, r.Result.RelevantSummaries, r.Result.RelatedEntities} {
		for _, item := range section {
			u.Items++
			if item.Memory != nil {
				u.ContentBytes += len(item.Memory.Content)
			}
		}
	}
	return u
}
func (a *ContextAdmission) reservedSize(r DisclosedContextResponse) int {
	r.Result.BudgetAccounting.Usage = ContextBudgetUsage{Items: 1000, ContentBytes: 4 << 20, ResponseBytes: 4 << 20, Citations: 1000}
	r.Result.BudgetAccounting.Selection.Outcome = "budget_exhausted"
	r.Result.BudgetAccounting.Selection.OmissionReasons = []string{"item_limit", "content_bytes", "citations", "response_bytes"}
	b, err := json.Marshal(r)
	if err != nil {
		return MaxPortableBudget
	}
	return len(b)
}
func (a *ContextAdmission) Finalize(out retrieval.AssembledContext, meta OperationMetadata) ([]byte, error) {
	a.metadata = SafeOperationMetadata(meta)
	r := a.response(out)
	u := contextUsage(r)
	// The assembler must return precisely admitted evidence, not bypass admission.
	admitted, _ := json.Marshal(a.response(a.selected))
	actual, _ := json.Marshal(r)
	if string(admitted) != string(actual) {
		return nil, fmt.Errorf("context admission mismatch")
	}
	a.accounting.Usage = u
	if u.Items > 0 {
		a.accounting.Selection.Outcome = "selected"
	} else if a.eligible {
		a.accounting.Selection.Outcome = "budget_exhausted"
	}
	a.accounting.Selection.OmissionReasons = []string{}
	for _, reason := range []string{"item_limit", "content_bytes", "citations", "response_bytes"} {
		if a.reasons[reason] {
			a.accounting.Selection.OmissionReasons = append(a.accounting.Selection.OmissionReasons, reason)
		}
	}
	r.Result.BudgetAccounting = a.accounting
	digest, err := ComputeContextDigest(a.scope, r, a.fingerprint)
	if err != nil {
		return nil, err
	}
	r.Result.ContextDigest = digest
	for n := 0; n < 8; n++ {
		b, err := json.Marshal(r)
		if err != nil {
			return nil, err
		}
		if len(b) == r.Result.BudgetAccounting.Usage.ResponseBytes {
			e := a.accounting.Effective
			if len(b) > e.MaxResponseBytes || u.Items > e.MaxItems || u.ContentBytes > e.MaxContentBytes || u.Citations > e.MaxCitations {
				return nil, fmt.Errorf("context_limit_exceeded")
			}
			return b, nil
		}
		r.Result.BudgetAccounting.Usage.ResponseBytes = len(b)
	}
	return nil, fmt.Errorf("context accounting did not converge")
}

func (a *ContextAdmission) PreflightResponse() DisclosedContextResponse {
	return a.response(a.selected)
}
func (a *ContextAdmission) SetPolicyFingerprint(s string) error {
	if !ValidContextHash(s) {
		return fmt.Errorf("invalid policy identity")
	}
	a.fingerprint = s
	return nil
}
func (a *ContextAdmission) SetContinuity(r ContextContinuityResult) { a.continuity = r }
func (a *ContextAdmission) SetTurnID(id string)                     { a.turnID = id }
