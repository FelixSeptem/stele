package retrieval

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type RetrievalPlannerVersion string

const RetrievalPlannerVersionV1 RetrievalPlannerVersion = "retrieval-planner-v1"

type RetrievalPlanPolicyVersion string

const RetrievalPlanPolicyVersionV1 RetrievalPlanPolicyVersion = "retrieval-plan-policy-v1"

type RetrievalQueryFamily string

const (
	RetrievalQueryFamilyExactLookup    RetrievalQueryFamily = "exact_lookup"
	RetrievalQueryFamilySemantic       RetrievalQueryFamily = "semantic"
	RetrievalQueryFamilyTemporal       RetrievalQueryFamily = "temporal"
	RetrievalQueryFamilyEntityRelation RetrievalQueryFamily = "entity_relation"
	RetrievalQueryFamilyMultiHop       RetrievalQueryFamily = "multi_hop"
	RetrievalQueryFamilyProcedural     RetrievalQueryFamily = "procedural"
	RetrievalQueryFamilyGeneral        RetrievalQueryFamily = "general"
)

var retrievalQueryFamilies = []RetrievalQueryFamily{
	RetrievalQueryFamilyExactLookup,
	RetrievalQueryFamilySemantic,
	RetrievalQueryFamilyTemporal,
	RetrievalQueryFamilyEntityRelation,
	RetrievalQueryFamilyMultiHop,
	RetrievalQueryFamilyProcedural,
	RetrievalQueryFamilyGeneral,
}

func (family RetrievalQueryFamily) valid() bool {
	for _, candidate := range retrievalQueryFamilies {
		if family == candidate {
			return true
		}
	}
	return false
}

// RetrievalPlanComplexityCategory is a bounded, deterministic category derived
// only from validated query-analysis counts. It never depends on query text, an
// online model, or repository state, and it may only re-share the declared
// candidate envelope between channels, never enlarge it.
type RetrievalPlanComplexityCategory string

const (
	RetrievalPlanComplexitySimple   RetrievalPlanComplexityCategory = "simple"
	RetrievalPlanComplexityModerate RetrievalPlanComplexityCategory = "moderate"
	RetrievalPlanComplexityComplex  RetrievalPlanComplexityCategory = "complex"
)

var retrievalPlanComplexityCategories = []RetrievalPlanComplexityCategory{
	RetrievalPlanComplexitySimple,
	RetrievalPlanComplexityModerate,
	RetrievalPlanComplexityComplex,
}

func (category RetrievalPlanComplexityCategory) valid() bool {
	for _, candidate := range retrievalPlanComplexityCategories {
		if category == candidate {
			return true
		}
	}
	return false
}

type RetrievalPlanDisposition string

const (
	RetrievalPlanDispositionPlanned  RetrievalPlanDisposition = "planned"
	RetrievalPlanDispositionFallback RetrievalPlanDisposition = "fallback"
)

type RetrievalPlanFallback string

const RetrievalPlanFallbackBaseline RetrievalPlanFallback = "approved_baseline"

type RetrievalPlanHardLimits struct {
	MaxCandidates           int
	MaxCandidatesPerChannel int
	MaxPasses               int
	MaxLatency              time.Duration
	MaxContextItems         int
	MaxRerankerHeadroom     int
}

func DefaultRetrievalPlanHardLimits() RetrievalPlanHardLimits {
	return RetrievalPlanHardLimits{
		MaxCandidates: 200, MaxCandidatesPerChannel: 100, MaxPasses: 2,
		MaxLatency: 5 * time.Second, MaxContextItems: 100, MaxRerankerHeadroom: 100,
	}
}

func (limits RetrievalPlanHardLimits) Validate() error {
	if limits.MaxCandidates <= 0 || limits.MaxCandidates > 5000 {
		return fmt.Errorf("retrieval-plan max candidates must be between 1 and 5000")
	}
	if limits.MaxCandidatesPerChannel <= 0 || limits.MaxCandidatesPerChannel > limits.MaxCandidates {
		return fmt.Errorf("retrieval-plan max candidates per channel must be between 1 and max candidates")
	}
	if limits.MaxPasses < 1 || limits.MaxPasses > 2 {
		return fmt.Errorf("retrieval-plan max passes must be one or two")
	}
	if limits.MaxLatency <= 0 || limits.MaxLatency > 30*time.Second {
		return fmt.Errorf("retrieval-plan max latency must be between 1ns and 30s")
	}
	if limits.MaxContextItems <= 0 || limits.MaxContextItems > 1000 {
		return fmt.Errorf("retrieval-plan max context items must be between 1 and 1000")
	}
	if limits.MaxRerankerHeadroom < 0 || limits.MaxRerankerHeadroom > limits.MaxCandidates {
		return fmt.Errorf("retrieval-plan reranker headroom must be between zero and max candidates")
	}
	return nil
}

type RetrievalPlanFollowUpRule struct {
	Enabled             bool
	MinimumVisible      int
	CandidateAllocation int
	Channels            []FusionChannel
}

type RetrievalPlanTemplate struct {
	Channels                  []FusionChannel
	ChannelCandidates         map[FusionChannel]int
	FallbackChannelCandidates map[FusionChannel]int
	// ComplexityCandidates optionally re-shares the same total candidate
	// envelope between declared channels for one bounded complexity category. A
	// category entry must allocate every declared channel, keep each channel
	// within the per-channel hard limit, and preserve TotalCandidates exactly so
	// a plan can never enlarge its envelope.
	ComplexityCandidates map[RetrievalPlanComplexityCategory]map[FusionChannel]int
	TotalCandidates      int
	Fusion               FusionStrategy
	MemoryClassQuotas    map[memory.MemoryClass]int
	ContextPriorities    []memory.MemoryClass
	RerankerEligible     bool
	RerankerHeadroom     int
	MaxPasses            int
	LatencyBudget        time.Duration
	ContextItems         int
	FollowUp             RetrievalPlanFollowUpRule
}

type RetrievalPlanPolicy struct {
	PlannerVersion           RetrievalPlannerVersion
	Version                  RetrievalPlanPolicyVersion
	HardLimits               RetrievalPlanHardLimits
	GraphTraversalHardLimits GraphTraversalLimits
	Templates                map[RetrievalQueryFamily]RetrievalPlanTemplate
}

func DefaultRetrievalPlanPolicy() RetrievalPlanPolicy {
	limits := DefaultRetrievalPlanHardLimits()
	templates := make(map[RetrievalQueryFamily]RetrievalPlanTemplate, len(retrievalQueryFamilies))
	for _, family := range retrievalQueryFamilies {
		channels := []FusionChannel{FusionChannelLexical, FusionChannelSemantic, FusionChannelRelation, FusionChannelChunk}
		channelCandidates := map[FusionChannel]int{
			FusionChannelLexical: 50, FusionChannelSemantic: 50,
			FusionChannelRelation: 50, FusionChannelChunk: 50,
		}
		template := RetrievalPlanTemplate{
			Channels: channels, ChannelCandidates: channelCandidates, TotalCandidates: 200,
			Fusion: DefaultRRFStrategy(), MemoryClassQuotas: map[memory.MemoryClass]int{},
			ContextPriorities: []memory.MemoryClass{memory.MemoryClassProfile, memory.MemoryClassSummary, memory.MemoryClassRelation, memory.MemoryClassEpisodic, memory.MemoryClassProcedural},
			RerankerEligible:  false, RerankerHeadroom: 0, MaxPasses: 1,
			LatencyBudget: 250 * time.Millisecond, ContextItems: 50,
		}
		templates[family] = template
	}
	procedural := templates[RetrievalQueryFamilyProcedural]
	procedural.MemoryClassQuotas = map[memory.MemoryClass]int{memory.MemoryClassProcedural: 25}
	procedural.ContextPriorities = []memory.MemoryClass{memory.MemoryClassProcedural, memory.MemoryClassSummary, memory.MemoryClassEpisodic, memory.MemoryClassProfile, memory.MemoryClassRelation}
	templates[RetrievalQueryFamilyProcedural] = procedural
	return RetrievalPlanPolicy{PlannerVersion: RetrievalPlannerVersionV1, Version: RetrievalPlanPolicyVersionV1, HardLimits: limits, GraphTraversalHardLimits: DefaultGraphTraversalLimits(), Templates: templates}
}

func (policy RetrievalPlanPolicy) Validate() error {
	if policy.PlannerVersion != RetrievalPlannerVersionV1 {
		return fmt.Errorf("unsupported retrieval planner version %q", policy.PlannerVersion)
	}
	if policy.Version != RetrievalPlanPolicyVersionV1 {
		return fmt.Errorf("unsupported retrieval plan policy version %q", policy.Version)
	}
	if err := policy.HardLimits.Validate(); err != nil {
		return err
	}
	graphLimits := policy.GraphTraversalHardLimits
	if graphLimits == (GraphTraversalLimits{}) {
		graphLimits = DefaultGraphTraversalLimits()
	}
	if err := graphLimits.ValidateEffective(); err != nil {
		return err
	}
	for _, family := range retrievalQueryFamilies {
		template, ok := policy.Templates[family]
		if !ok {
			return fmt.Errorf("retrieval-plan template for %q is required", family)
		}
		if err := validateRetrievalPlanTemplate(template, policy.HardLimits); err != nil {
			return fmt.Errorf("validate retrieval-plan template %q: %w", family, err)
		}
	}
	for family := range policy.Templates {
		if !family.valid() {
			return fmt.Errorf("unsupported retrieval query family %q", family)
		}
	}
	return nil
}

type RetrievalPlanInput struct {
	AcceptedQuery        string
	Analysis             QueryAnalysisResult
	EmbeddingAvailable   bool
	Policy               RetrievalPlanPolicy
	Now                  time.Time
	TemporalConstraint   memory.TemporalConstraint
	GraphTraversalPolicy *memory.GraphTraversalPolicy
	GraphTraversalLimits GraphTraversalLimits
}

type RetrievalPlanIdentity struct {
	PlannerVersion            RetrievalPlannerVersion
	PolicyVersion             RetrievalPlanPolicyVersion
	Family                    RetrievalQueryFamily
	FallbackChannelCandidates map[FusionChannel]int
	// TemporalMode makes a historical plan distinguishable from a current one
	// in the identity string, so a replay can prove it re-ran the same
	// fact-valid selection instead of silently falling back to current.
	TemporalMode       memory.TemporalSelectionMode
	GraphPolicyVersion string
	GraphHops          int
}

func (identity RetrievalPlanIdentity) String() string {
	value := string(identity.PlannerVersion) + ":" + string(identity.PolicyVersion) + ":" + string(identity.Family)
	// Only an explicit historical selection appears. Ordinary current plans keep
	// their exact pre-temporal identity string, so existing fixtures and
	// comparisons are unaffected.
	if identity.TemporalMode != "" && identity.TemporalMode != memory.TemporalSelectionCurrent {
		value += ":" + string(identity.TemporalMode)
	}
	if identity.GraphPolicyVersion != "" {
		value += ":graph=" + identity.GraphPolicyVersion + ":hops=" + fmt.Sprint(identity.GraphHops)
	}
	if len(identity.FallbackChannelCandidates) == 0 {
		return value
	}
	channels := make([]FusionChannel, 0, len(identity.FallbackChannelCandidates))
	for channel := range identity.FallbackChannelCandidates {
		channels = append(channels, channel)
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i] < channels[j] })
	parts := make([]string, 0, len(channels))
	for _, channel := range channels {
		parts = append(parts, fmt.Sprintf("%s:%d", channel, identity.FallbackChannelCandidates[channel]))
	}
	return value + ":fallback=" + strings.Join(parts, ",")
}

type RetrievalPlan struct {
	Identity                  RetrievalPlanIdentity
	Family                    RetrievalQueryFamily
	ComplexityCategory        RetrievalPlanComplexityCategory
	Disposition               RetrievalPlanDisposition
	Channels                  []FusionChannel
	ChannelCandidates         map[FusionChannel]int
	FallbackChannelCandidates map[FusionChannel]int
	TotalCandidates           int
	Fusion                    FusionStrategy
	MemoryClassQuotas         map[memory.MemoryClass]int
	ContextPriorities         []memory.MemoryClass
	RerankerEligible          bool
	RerankerHeadroom          int
	MaxPasses                 int
	LatencyBudget             time.Duration
	ContextItems              int
	FollowUp                  RetrievalPlanFollowUpRule
	Fallback                  RetrievalPlanFallback
	TemporalConstraint        memory.TemporalConstraint
	GraphTraversalPolicy      *memory.GraphTraversalPolicy
	GraphTraversalLimits      GraphTraversalLimits
}

func BuildRetrievalPlan(input RetrievalPlanInput) (RetrievalPlan, error) {
	if strings.TrimSpace(input.AcceptedQuery) == "" {
		return RetrievalPlan{}, fmt.Errorf("retrieval-plan accepted query is required")
	}
	if input.Now.IsZero() {
		return RetrievalPlan{}, fmt.Errorf("retrieval-plan time is required")
	}
	if err := input.Policy.Validate(); err != nil {
		return RetrievalPlan{}, err
	}
	temporalConstraint := input.TemporalConstraint
	if temporalConstraint.Mode == "" {
		temporalConstraint = memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}
	}
	if err := temporalConstraint.Validate(); err != nil {
		return RetrievalPlan{}, fmt.Errorf("validate retrieval-plan temporal constraint: %w", err)
	}
	family := classifyRetrievalQueryFamily(input.Analysis, input.EmbeddingAvailable)
	graphLimits := input.GraphTraversalLimits
	if graphLimits == (GraphTraversalLimits{}) {
		graphLimits = input.Policy.GraphTraversalHardLimits
	}
	if graphLimits == (GraphTraversalLimits{}) {
		graphLimits = DefaultGraphTraversalLimits()
	}
	if err := graphLimits.Validate(); err != nil {
		return RetrievalPlan{}, fmt.Errorf("validate input graph limits: %w", err)
	}
	var graphPolicy *memory.GraphTraversalPolicy
	effectiveGraphLimits := graphLimits
	if input.GraphTraversalPolicy != nil && input.GraphTraversalPolicy.EnablesFamily(string(family)) {
		copyPolicy := *input.GraphTraversalPolicy
		var err error
		effectiveGraphLimits, err = EffectiveGraphTraversalLimits(graphLimits, copyPolicy)
		if err != nil {
			return RetrievalPlan{}, err
		}
		graphPolicy = &copyPolicy
	}
	if input.GraphTraversalPolicy != nil && !input.GraphTraversalPolicy.EnablesFamily(string(family)) && (family == RetrievalQueryFamilyEntityRelation || family == RetrievalQueryFamilyMultiHop) {
		// A policy may target only the other graph family; this plan remains
		// baseline and carries no graph disposition.
		graphPolicy = nil
	}
	complexity := classifyRetrievalPlanComplexity(input.Analysis)
	template := input.Policy.Templates[family]
	channelCandidates := template.ChannelCandidates
	if allocation, ok := template.ComplexityCandidates[complexity]; ok {
		channelCandidates = allocation
	}
	fallbackCandidates := cloneChannelCandidates(template.FallbackChannelCandidates)
	plan := RetrievalPlan{
		Identity: RetrievalPlanIdentity{PlannerVersion: input.Policy.PlannerVersion, PolicyVersion: input.Policy.Version, Family: family, FallbackChannelCandidates: cloneChannelCandidates(fallbackCandidates), TemporalMode: temporalConstraint.Mode},
		Family:   family, ComplexityCategory: complexity, Disposition: RetrievalPlanDispositionPlanned,
		Channels:                  append([]FusionChannel(nil), template.Channels...),
		ChannelCandidates:         cloneChannelCandidates(channelCandidates),
		FallbackChannelCandidates: fallbackCandidates,
		TotalCandidates:           template.TotalCandidates, Fusion: cloneFusionStrategy(template.Fusion),
		MemoryClassQuotas: cloneClassQuotas(template.MemoryClassQuotas),
		ContextPriorities: append([]memory.MemoryClass(nil), template.ContextPriorities...),
		RerankerEligible:  template.RerankerEligible, RerankerHeadroom: template.RerankerHeadroom,
		MaxPasses: template.MaxPasses, LatencyBudget: template.LatencyBudget,
		ContextItems: template.ContextItems, FollowUp: cloneFollowUpRule(template.FollowUp),
		Fallback:             RetrievalPlanFallbackBaseline,
		TemporalConstraint:   temporalConstraint,
		GraphTraversalPolicy: graphPolicy, GraphTraversalLimits: effectiveGraphLimits,
	}
	if graphPolicy != nil {
		plan.Identity.GraphPolicyVersion = graphPolicy.PolicyVersion
		plan.Identity.GraphHops = effectiveGraphLimits.MaxHops
	}
	canonicalizeRetrievalPlan(&plan)
	if err := plan.Validate(input.Policy.HardLimits); err != nil {
		return RetrievalPlan{}, fmt.Errorf("validate graph retrieval plan: %w", err)
	}
	return plan, nil
}

func (plan RetrievalPlan) Validate(limits RetrievalPlanHardLimits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if plan.Identity.PlannerVersion != RetrievalPlannerVersionV1 || plan.Identity.PolicyVersion != RetrievalPlanPolicyVersionV1 || !plan.Family.valid() || plan.Identity.Family != plan.Family || !channelCandidatesEqual(plan.Identity.FallbackChannelCandidates, plan.FallbackChannelCandidates) {
		return fmt.Errorf("invalid retrieval-plan identity")
	}
	// The identity must name the same selection the plan will actually apply,
	// otherwise a replay could report a historical plan while running a current
	// one.
	if plan.Identity.TemporalMode != plan.TemporalConstraint.Mode {
		return fmt.Errorf("invalid retrieval-plan identity temporal mode")
	}
	if plan.Disposition != RetrievalPlanDispositionPlanned && plan.Disposition != RetrievalPlanDispositionFallback {
		return fmt.Errorf("invalid retrieval-plan disposition")
	}
	if !plan.ComplexityCategory.valid() {
		return fmt.Errorf("invalid retrieval-plan complexity category %q", plan.ComplexityCategory)
	}
	template := RetrievalPlanTemplate{
		Channels: plan.Channels, ChannelCandidates: plan.ChannelCandidates, FallbackChannelCandidates: plan.FallbackChannelCandidates, TotalCandidates: plan.TotalCandidates,
		Fusion: plan.Fusion, MemoryClassQuotas: plan.MemoryClassQuotas, ContextPriorities: plan.ContextPriorities,
		RerankerEligible: plan.RerankerEligible, RerankerHeadroom: plan.RerankerHeadroom,
		MaxPasses: plan.MaxPasses, LatencyBudget: plan.LatencyBudget, ContextItems: plan.ContextItems, FollowUp: plan.FollowUp,
	}
	if err := validateRetrievalPlanTemplate(template, limits); err != nil {
		return err
	}
	if plan.Fallback != RetrievalPlanFallbackBaseline {
		return fmt.Errorf("unsupported retrieval-plan fallback %q", plan.Fallback)
	}
	if err := plan.TemporalConstraint.Validate(); err != nil {
		return fmt.Errorf("validate retrieval-plan temporal constraint: %w", err)
	}
	graphLimits := plan.GraphTraversalLimits
	if graphLimits == (GraphTraversalLimits{}) {
		graphLimits = DefaultGraphTraversalLimits()
	}
	if err := graphLimits.ValidateEffective(); err != nil {
		return err
	}
	if plan.GraphTraversalPolicy == nil {
		if plan.Identity.GraphPolicyVersion != "" || plan.Identity.GraphHops != 0 {
			return fmt.Errorf("retrieval-plan graph identity requires a graph policy")
		}
	} else {
		if !plan.GraphTraversalPolicy.EnablesFamily(string(plan.Family)) {
			return fmt.Errorf("retrieval-plan graph policy does not enable family")
		}
		if err := plan.GraphTraversalPolicy.Validate(); err != nil {
			return err
		}
		if plan.GraphTraversalPolicy.HopsSet && plan.GraphTraversalPolicy.MaxHops != graphLimits.MaxHops {
			return fmt.Errorf("retrieval-plan graph hop disposition is inconsistent")
		}
		if !plan.GraphTraversalPolicy.HopsSet && graphLimits.MaxHops < 1 {
			return fmt.Errorf("retrieval-plan default graph hop disposition is invalid")
		}
		if plan.Identity.GraphPolicyVersion != plan.GraphTraversalPolicy.PolicyVersion || plan.Identity.GraphHops != graphLimits.MaxHops {
			return fmt.Errorf("retrieval-plan graph identity is inconsistent")
		}
	}
	return nil
}

func validateRetrievalPlanTemplate(template RetrievalPlanTemplate, limits RetrievalPlanHardLimits) error {
	if len(template.Channels) == 0 || len(template.Channels) > 4 {
		return fmt.Errorf("retrieval-plan requires between one and four channels")
	}
	seenChannels := make(map[FusionChannel]struct{}, len(template.Channels))
	allocated := 0
	for _, channel := range template.Channels {
		if !channel.valid() {
			return fmt.Errorf("unsupported retrieval-plan channel %q", channel)
		}
		if _, duplicate := seenChannels[channel]; duplicate {
			return fmt.Errorf("duplicate retrieval-plan channel %q", channel)
		}
		seenChannels[channel] = struct{}{}
		count, ok := template.ChannelCandidates[channel]
		if !ok || count <= 0 || count > limits.MaxCandidatesPerChannel {
			return fmt.Errorf("invalid retrieval-plan candidate allocation for %q", channel)
		}
		allocated += count
	}
	if len(template.ChannelCandidates) != len(seenChannels) {
		return fmt.Errorf("retrieval-plan candidate allocations must match channels")
	}
	if template.TotalCandidates <= 0 || template.TotalCandidates > limits.MaxCandidates || allocated != template.TotalCandidates {
		return fmt.Errorf("invalid retrieval-plan total candidate budget")
	}
	for _, category := range retrievalPlanComplexityCategories {
		allocation, declared := template.ComplexityCandidates[category]
		if !declared {
			continue
		}
		if len(allocation) != len(template.Channels) {
			return fmt.Errorf("retrieval-plan complexity allocation for %q must cover declared channels", category)
		}
		complexityAllocated := 0
		for _, channel := range template.Channels {
			count, ok := allocation[channel]
			if !ok || count <= 0 || count > limits.MaxCandidatesPerChannel {
				return fmt.Errorf("invalid retrieval-plan complexity allocation for %q channel %q", category, channel)
			}
			complexityAllocated += count
		}
		if complexityAllocated != template.TotalCandidates {
			return fmt.Errorf("retrieval-plan complexity allocation for %q must preserve the total candidate envelope", category)
		}
	}
	for category := range template.ComplexityCandidates {
		if !category.valid() {
			return fmt.Errorf("unsupported retrieval-plan complexity category %q", category)
		}
	}
	fallbackAllocated := 0
	for channel, count := range template.FallbackChannelCandidates {
		if !channel.valid() || count <= 0 || count > limits.MaxCandidatesPerChannel {
			return fmt.Errorf("invalid retrieval-plan fallback candidate allocation for %q", channel)
		}
		if _, planned := seenChannels[channel]; planned {
			return fmt.Errorf("retrieval-plan fallback channel %q overlaps planned channels", channel)
		}
		fallbackAllocated += count
	}
	if err := template.Fusion.Validate(); err != nil {
		return fmt.Errorf("validate retrieval-plan fusion: %w", err)
	}
	if template.Fusion.TotalCandidates > template.TotalCandidates || template.Fusion.TotalCandidates > limits.MaxCandidates {
		return fmt.Errorf("retrieval-plan fusion total exceeds candidate envelope")
	}
	if template.Fusion.PerChannelCandidate > limits.MaxCandidatesPerChannel {
		return fmt.Errorf("retrieval-plan fusion channel limit exceeds hard limit")
	}
	for _, channel := range template.Channels {
		if template.Fusion.PerChannelCandidate > template.ChannelCandidates[channel] {
			return fmt.Errorf("retrieval-plan fusion channel limit exceeds allocation for %q", channel)
		}
		for _, category := range retrievalPlanComplexityCategories {
			allocation, declared := template.ComplexityCandidates[category]
			if !declared {
				continue
			}
			if template.Fusion.PerChannelCandidate > allocation[channel] {
				return fmt.Errorf("retrieval-plan fusion channel limit exceeds complexity allocation for %q channel %q", category, channel)
			}
		}
	}
	if template.MaxPasses < 1 || template.MaxPasses > limits.MaxPasses {
		return fmt.Errorf("invalid retrieval-plan pass limit")
	}
	if template.LatencyBudget <= 0 || template.LatencyBudget > limits.MaxLatency {
		return fmt.Errorf("invalid retrieval-plan latency budget")
	}
	if template.ContextItems <= 0 || template.ContextItems > limits.MaxContextItems {
		return fmt.Errorf("invalid retrieval-plan context item budget")
	}
	if template.RerankerHeadroom < 0 || template.RerankerHeadroom > limits.MaxRerankerHeadroom || template.RerankerHeadroom > template.TotalCandidates {
		return fmt.Errorf("invalid retrieval-plan reranker headroom")
	}
	if !template.RerankerEligible && template.RerankerHeadroom != 0 {
		return fmt.Errorf("retrieval-plan reranker headroom requires eligibility")
	}
	if fallbackAllocated+template.RerankerHeadroom >= template.TotalCandidates {
		return fmt.Errorf("retrieval-plan fallback and reranker reserves exhaust planned candidate capacity")
	}
	if template.FollowUp.Enabled {
		if template.MaxPasses != 2 || template.FollowUp.MinimumVisible < 0 || template.FollowUp.CandidateAllocation <= 0 || template.FollowUp.CandidateAllocation > template.TotalCandidates || len(template.FollowUp.Channels) == 0 {
			return fmt.Errorf("invalid retrieval-plan follow-up rule")
		}
		followUpAllocation := 0
		seenFollowUpChannels := make(map[FusionChannel]struct{}, len(template.FollowUp.Channels))
		for _, channel := range template.FollowUp.Channels {
			if !channel.valid() || channel == FusionChannelChunk {
				return fmt.Errorf("unsupported retrieval-plan follow-up channel %q", channel)
			}
			if _, duplicate := seenFollowUpChannels[channel]; duplicate {
				return fmt.Errorf("duplicate retrieval-plan follow-up channel %q", channel)
			}
			seenFollowUpChannels[channel] = struct{}{}
			if _, declared := seenChannels[channel]; !declared || template.ChannelCandidates[channel] <= 0 {
				return fmt.Errorf("retrieval-plan follow-up channel %q has no declared allocation", channel)
			}
			followUpAllocation += template.ChannelCandidates[channel]
		}
		if followUpAllocation <= 0 || template.FollowUp.CandidateAllocation > followUpAllocation {
			return fmt.Errorf("retrieval-plan follow-up allocation exceeds declared channels")
		}
	} else if template.MaxPasses != 1 || template.FollowUp.MinimumVisible != 0 || template.FollowUp.CandidateAllocation != 0 || len(template.FollowUp.Channels) != 0 {
		return fmt.Errorf("disabled retrieval-plan follow-up must be empty")
	}
	for class, quota := range template.MemoryClassQuotas {
		if !validPlanMemoryClass(class) || quota <= 0 || quota > template.ContextItems {
			return fmt.Errorf("invalid retrieval-plan quota for memory class %q", class)
		}
	}
	seenClasses := make(map[memory.MemoryClass]struct{}, len(template.ContextPriorities))
	for _, class := range template.ContextPriorities {
		if !validPlanMemoryClass(class) {
			return fmt.Errorf("invalid retrieval-plan context priority %q", class)
		}
		if _, duplicate := seenClasses[class]; duplicate {
			return fmt.Errorf("duplicate retrieval-plan context priority %q", class)
		}
		seenClasses[class] = struct{}{}
	}
	return nil
}

func classifyRetrievalQueryFamily(analysis QueryAnalysisResult, embeddingAvailable bool) RetrievalQueryFamily {
	if analysis.Identity.PolicyVersion != QueryAnalysisPolicyVersionV1 || analysis.Identity.LimitsVersion != QueryAnalysisLimitsVersionV1 {
		return RetrievalQueryFamilyGeneral
	}
	if analysisHintValue(analysis.Hints, QueryAnalysisHintMemoryClass) == string(memory.MemoryClassProcedural) {
		return RetrievalQueryFamilyProcedural
	}
	subqueries := 0
	for _, signal := range analysis.Signals {
		if signal.Kind == QueryAnalysisSignalSubquery {
			subqueries++
		}
	}
	if subqueries >= 2 {
		return RetrievalQueryFamilyMultiHop
	}
	if analysisHintPresent(analysis.Hints, QueryAnalysisHintTemporal) {
		return RetrievalQueryFamilyTemporal
	}
	if analysisHintPresent(analysis.Hints, QueryAnalysisHintEntity) {
		return RetrievalQueryFamilyEntityRelation
	}
	if analysisHintValue(analysis.Hints, QueryAnalysisHintIntent) == "lookup" {
		return RetrievalQueryFamilyExactLookup
	}
	if embeddingAvailable {
		return RetrievalQueryFamilySemantic
	}
	return RetrievalQueryFamilyGeneral
}

// classifyRetrievalPlanComplexity derives a bounded complexity category from
// validated query-analysis counts only. It is a pure function of the accepted
// analysis identity and derived signal categories, so equivalent inputs always
// produce the same category and the planner never inspects query text here.
func classifyRetrievalPlanComplexity(analysis QueryAnalysisResult) RetrievalPlanComplexityCategory {
	if analysis.Identity.PolicyVersion != QueryAnalysisPolicyVersionV1 || analysis.Identity.LimitsVersion != QueryAnalysisLimitsVersionV1 {
		return RetrievalPlanComplexitySimple
	}
	subqueries := 0
	derived := 0
	for _, signal := range analysis.Signals {
		if signal.Kind == QueryAnalysisSignalOriginal {
			continue
		}
		derived++
		if signal.Kind == QueryAnalysisSignalSubquery {
			subqueries++
		}
	}
	switch {
	case subqueries >= 2 || derived >= 4:
		return RetrievalPlanComplexityComplex
	case subqueries == 1 || derived >= 2:
		return RetrievalPlanComplexityModerate
	default:
		return RetrievalPlanComplexitySimple
	}
}

func analysisHintPresent(hints []QueryAnalysisHint, kind QueryAnalysisHintKind) bool {
	return analysisHintValue(hints, kind) != ""
}

func analysisHintValue(hints []QueryAnalysisHint, kind QueryAnalysisHintKind) string {
	for _, hint := range hints {
		if hint.Kind == kind && hint.Disposition == QueryAnalysisHintPresent {
			return hint.Value
		}
	}
	return ""
}

func canonicalizeRetrievalPlan(plan *RetrievalPlan) {
	sort.Slice(plan.Channels, func(i, j int) bool { return plan.Channels[i] < plan.Channels[j] })
	sort.Slice(plan.FollowUp.Channels, func(i, j int) bool { return plan.FollowUp.Channels[i] < plan.FollowUp.Channels[j] })
}

func cloneChannelCandidates(source map[FusionChannel]int) map[FusionChannel]int {
	result := make(map[FusionChannel]int, len(source))
	for channel, count := range source {
		result[channel] = count
	}
	return result
}

func cloneComplexityCandidates(source map[RetrievalPlanComplexityCategory]map[FusionChannel]int) map[RetrievalPlanComplexityCategory]map[FusionChannel]int {
	if len(source) == 0 {
		return nil
	}
	result := make(map[RetrievalPlanComplexityCategory]map[FusionChannel]int, len(source))
	for category, allocation := range source {
		result[category] = cloneChannelCandidates(allocation)
	}
	return result
}

func channelCandidatesEqual(left, right map[FusionChannel]int) bool {
	if len(left) != len(right) {
		return false
	}
	for channel, count := range left {
		if right[channel] != count {
			return false
		}
	}
	return true
}

func cloneClassQuotas(source map[memory.MemoryClass]int) map[memory.MemoryClass]int {
	result := make(map[memory.MemoryClass]int, len(source))
	for class, quota := range source {
		result[class] = quota
	}
	return result
}

func cloneFusionStrategy(source FusionStrategy) FusionStrategy {
	result := source
	result.ChannelWeights = make(map[FusionChannel]float64, len(source.ChannelWeights))
	for channel, weight := range source.ChannelWeights {
		result.ChannelWeights[channel] = weight
	}
	return result
}

func cloneFollowUpRule(source RetrievalPlanFollowUpRule) RetrievalPlanFollowUpRule {
	result := source
	result.Channels = append([]FusionChannel(nil), source.Channels...)
	return result
}

func validPlanMemoryClass(class memory.MemoryClass) bool {
	switch class {
	case memory.MemoryClassProfile, memory.MemoryClassEpisodic, memory.MemoryClassProcedural, memory.MemoryClassSummary, memory.MemoryClassRelation:
		return true
	default:
		return false
	}
}
