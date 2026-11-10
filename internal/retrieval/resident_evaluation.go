package retrieval

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FelixSeptem/stele/internal/memory"
)

const residentMaxSources = 256
const residentObservationMaxAge = 30 * 24 * time.Hour

// These independent evaluator limits do not describe Provider wire bytes or
// model tokens. Tokens here are strings.Fields counts over literal content.
type ResidentEvaluationLimits struct {
	MaxItems        int `json:"max_items"`
	MaxContentBytes int `json:"max_content_bytes"`
	MaxTokens       int `json:"max_tokens"`
	MaxCitations    int `json:"max_citations"`
}

func DefaultResidentEvaluationLimits() ResidentEvaluationLimits {
	return ResidentEvaluationLimits{8, 4096, 1024, 16}
}

func (l ResidentEvaluationLimits) validate() error {
	if l.MaxItems < 1 || l.MaxItems > residentMaxSources || l.MaxContentBytes < 1 || l.MaxContentBytes > 1_048_576 || l.MaxTokens < 1 || l.MaxTokens > 262144 || l.MaxCitations < 1 || l.MaxCitations > residentMaxSources {
		return fmt.Errorf("resident limits are invalid")
	}
	return nil
}

// ResidentSource is an authorized snapshot, including the resolved immutable
// version, rather than a caller's claim that an ID was resolved. AccessCount is
// deliberately unused, including in hashes and tie breaking.
type ResidentSource struct {
	Memory      memory.CanonicalMemory
	Version     memory.MemoryVersion
	Confidence  float64
	AccessCount int64
}

// Observations are decoded from independently attributed raw-event Content.
// They are observations supplied by the harness, never service-proven truth.
type ResidentSupportObservation struct {
	MemoryID         string    `json:"memory_id"`
	SourceVersion    int64     `json:"source_version"`
	ClaimFingerprint string    `json:"claim_fingerprint"`
	Actor            string    `json:"actor"`
	Stable           bool      `json:"stable"`
	Applicable       bool      `json:"applicable"`
	Stage            int       `json:"stage"`
	ExpiresAt        time.Time `json:"expires_at"`
}

type ResidentUtilityObservation struct {
	MemoryID         string    `json:"memory_id"`
	SourceVersion    int64     `json:"source_version"`
	ClaimFingerprint string    `json:"claim_fingerprint"`
	Actor            string    `json:"actor"`
	Stage            int       `json:"stage"`
	ExpiresAt        time.Time `json:"expires_at"`
	Useful           bool      `json:"useful"`
}

type ResidentEvaluationInput struct {
	Scope              memory.Scope
	EvaluatedAt        time.Time
	Policy             memory.ContextProjectionPolicy
	Limits             ResidentEvaluationLimits
	Sources            []ResidentSource
	CurrentResidentIDs []string
	Support            []memory.RawEvent
	Utility            []memory.RawEvent
}

type ResidentOmissionReason string

const (
	ResidentOmissionSafety  ResidentOmissionReason = "source_safety"
	ResidentOmissionPolicy  ResidentOmissionReason = "profile_policy"
	ResidentOmissionSupport ResidentOmissionReason = "insufficient_support"
	ResidentOmissionUtility ResidentOmissionReason = "insufficient_utility"
	ResidentOmissionBudget  ResidentOmissionReason = "budget"
)

type ResidentPlanMetrics struct {
	Items        int `json:"items"`
	ContentBytes int `json:"content_bytes"`
	Tokens       int `json:"tokens"`
	Citations    int `json:"citations"`
}

type ResidentEvaluationReport struct {
	Schema           string                         `json:"schema"`
	Status           string                         `json:"status"`
	Limits           ResidentEvaluationLimits       `json:"limits"`
	Baseline         ResidentPlanMetrics            `json:"baseline"`
	Candidate        ResidentPlanMetrics            `json:"candidate"`
	Promoted         int                            `json:"promoted"`
	Retained         int                            `json:"retained"`
	Demoted          int                            `json:"demoted"`
	Omissions        map[ResidentOmissionReason]int `json:"omissions"`
	SourceDigest     string                         `json:"source_digest"`
	PolicyDigest     string                         `json:"policy_digest"`
	OutputDigest     string                         `json:"output_digest"`
	RunHash          string                         `json:"run_hash"`
	RollbackTested   bool                           `json:"rollback_tested"`
	OutcomeStatus    string                         `json:"outcome_status"`
	BaselineOutcome  ResidentExternalMetrics        `json:"baseline_outcome"`
	CandidateOutcome ResidentExternalMetrics        `json:"candidate_outcome"`
	Delta            ResidentExternalDelta          `json:"delta"`
	OutcomeDigest    string                         `json:"outcome_digest"`
}

type ResidentEvaluationResult struct {
	Report      ResidentEvaluationReport `json:"report"`
	baseline    memory.ContextProjection
	candidate   memory.ContextProjection
	scope       memory.Scope
	evaluatedAt time.Time
	latestStage int
}

// Returned plans are internal-only and copied so a caller cannot mutate the
// retained rollback plan through a slice alias.
func (r ResidentEvaluationResult) BaselineProjection() memory.ContextProjection {
	return cloneResidentProjection(r.baseline)
}
func (r ResidentEvaluationResult) CandidateProjection() memory.ContextProjection {
	return cloneResidentProjection(r.candidate)
}
func (r ResidentEvaluationResult) SelectedProjection(disabled bool) memory.ContextProjection {
	if disabled {
		return r.BaselineProjection()
	}
	return r.CandidateProjection()
}
func cloneResidentProjection(p memory.ContextProjection) memory.ContextProjection {
	p.Items = append([]memory.ContextProjectionItem{}, p.Items...)
	for i := range p.Items {
		p.Items[i].TemporalValidity = cloneResidentValidity(p.Items[i].TemporalValidity)
	}
	p.SourceWatermark.CanonicalVersionIDs = append([]string{}, p.SourceWatermark.CanonicalVersionIDs...)
	p.SourceWatermark.RawEventIDs = append([]string{}, p.SourceWatermark.RawEventIDs...)
	return p
}

func cloneResidentValidity(v memory.TemporalValidity) memory.TemporalValidity {
	if v.ValidTo != nil {
		end := *v.ValidTo
		v.ValidTo = &end
	}
	return v
}

func ResidentClaimFingerprint(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func residentDigest(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("resident evidence cannot be encoded")
	}
	return ResidentClaimFingerprint(string(data)), nil
}

// EvaluateResidentContext runs two bounded materializations. It has no store,
// model, public API, worker, or live strategy activation dependency.
func EvaluateResidentContext(ctx context.Context, in ResidentEvaluationInput) (ResidentEvaluationResult, error) {
	if in.Scope.Validate() != nil || in.Scope != in.Scope.Normalized() || in.EvaluatedAt.IsZero() || in.Policy.Validate() != nil || in.Limits.validate() != nil {
		return ResidentEvaluationResult{}, fmt.Errorf("resident evaluation input is invalid")
	}
	if len(in.Sources) > residentMaxSources || len(in.Support) > residentMaxSources || len(in.Utility) > residentMaxSources || len(in.CurrentResidentIDs) > residentMaxSources {
		return ResidentEvaluationResult{}, fmt.Errorf("resident evidence exceeds bounds")
	}
	sources := append([]ResidentSource{}, in.Sources...)
	sort.Slice(sources, func(i, j int) bool { return sources[i].Memory.ID < sources[j].Memory.ID })
	seen := map[string]bool{}
	for _, s := range sources {
		if !residentID(s.Memory.ID) || seen[s.Memory.ID] || math.IsNaN(s.Confidence) || math.IsInf(s.Confidence, 0) || s.Confidence < 0 || s.Confidence > 1 || len(s.Memory.Content) > 65536 || len(s.Version.Content) > 65536 {
			return ResidentEvaluationResult{}, fmt.Errorf("resident source snapshot is invalid")
		}
		seen[s.Memory.ID] = true
	}
	current := map[string]bool{}
	for _, id := range in.CurrentResidentIDs {
		if !residentID(id) || current[id] {
			return ResidentEvaluationResult{}, fmt.Errorf("resident current set is invalid")
		}
		current[id] = true
	}
	if err := residentEventBounds(in.Support, in.Utility); err != nil {
		return ResidentEvaluationResult{}, err
	}
	r := ResidentEvaluationResult{Report: ResidentEvaluationReport{Schema: "resident-evaluation-v1", Status: "shadow", Limits: in.Limits, Omissions: map[ResidentOmissionReason]int{}}}
	r.scope = in.Scope
	r.evaluatedAt = in.EvaluatedAt
	r.Report.OutcomeStatus = "inconclusive"
	var baseline, candidate []memory.ContextProjectionCandidate
	for _, s := range sources {
		if err := ctx.Err(); err != nil {
			return ResidentEvaluationResult{}, err
		}
		if !residentSourceSafe(s, in.Scope, in.EvaluatedAt) {
			r.Report.Omissions[ResidentOmissionSafety]++
			continue
		}
		m := s.Memory
		if !memory.ResolveContextProjectionPolicy(in.Policy, memory.ContextProjectionKindAlwaysVisible, m.Class, m.State, s.Confidence, len(strings.TrimSpace(m.Content)), false).Include {
			r.Report.Omissions[ResidentOmissionPolicy]++
			continue
		}
		p := memory.ContextProjectionCandidate{Source: memory.ContextProjectionSource{Kind: memory.ContextProjectionSourceCanonicalVersion, ID: s.Version.ID, Version: m.SourceVersion, MemoryID: m.ID, Scope: in.Scope, LifecycleState: m.State}, Class: m.Class, State: m.State, Content: m.Content, Confidence: s.Confidence, ObservedAt: m.ModifiedAt}
		baseline = append(baseline, p)
		support, event, ok := residentSupport(s, in)
		if !ok {
			r.Report.Omissions[ResidentOmissionSupport]++
			continue
		}
		if support.Stage > r.latestStage {
			r.latestStage = support.Stage
		}
		utility, ok := residentUtility(s, support, event, in)
		if !ok {
			r.Report.Omissions[ResidentOmissionUtility]++
			continue
		}
		if utility.Stage > r.latestStage {
			r.latestStage = utility.Stage
		}
		candidate = append(candidate, p)
	}
	var err error
	r.baseline, _, err = materializeResident(ctx, in, baseline, 1)
	if err != nil {
		return ResidentEvaluationResult{}, err
	}
	rollback, _, err := materializeResident(ctx, in, baseline, 1)
	if err != nil {
		return ResidentEvaluationResult{}, err
	}
	baselineDigest, err := residentDigest(r.baseline)
	if err != nil {
		return ResidentEvaluationResult{}, err
	}
	rollbackDigest, err := residentDigest(rollback)
	if err != nil || rollbackDigest != baselineDigest {
		return ResidentEvaluationResult{}, fmt.Errorf("resident rollback differs from baseline")
	}
	r.Report.RollbackTested = true
	var budgetOmitted int
	r.candidate, budgetOmitted, err = materializeResident(ctx, in, candidate, 2)
	if err != nil {
		return ResidentEvaluationResult{}, err
	}
	if budgetOmitted > 0 {
		r.Report.Omissions[ResidentOmissionBudget] = budgetOmitted
	}
	r.Report.Baseline = residentMetrics(r.baseline.Items)
	r.Report.Candidate = residentMetrics(r.candidate.Items)
	selected := map[string]bool{}
	for _, item := range r.candidate.Items {
		id := item.Source.MemoryID
		selected[id] = true
		if current[id] {
			r.Report.Retained++
		} else {
			r.Report.Promoted++
		}
	}
	for id := range current {
		if !selected[id] {
			r.Report.Demoted++
		}
	}
	// Explicitly include SourceVersion: CanonicalMemory deliberately hides it
	// from public JSON. Exclude popularity and materializer wall-clock duration.
	type sourceIdentity struct {
		Memory        memory.CanonicalMemory
		SourceVersion int64
		Version       memory.MemoryVersion
		Confidence    float64
	}
	identities := make([]sourceIdentity, 0, len(sources))
	for _, s := range sources {
		identities = append(identities, sourceIdentity{s.Memory, s.Memory.SourceVersion, s.Version, s.Confidence})
	}
	ids := append([]string{}, in.CurrentResidentIDs...)
	sort.Strings(ids)
	support := append([]memory.RawEvent{}, in.Support...)
	utility := append([]memory.RawEvent{}, in.Utility...)
	sort.Slice(support, func(i, j int) bool { return support[i].ID < support[j].ID })
	sort.Slice(utility, func(i, j int) bool { return utility[i].ID < utility[j].ID })
	r.Report.SourceDigest, err = residentDigest(struct {
		Scope            memory.Scope
		At               time.Time
		Sources          []sourceIdentity
		Current          []string
		Support, Utility []memory.RawEvent
	}{in.Scope, in.EvaluatedAt, identities, ids, support, utility})
	if err != nil {
		return ResidentEvaluationResult{}, err
	}
	r.Report.PolicyDigest, err = residentDigest(struct {
		Policy memory.ContextProjectionPolicy
		Limits ResidentEvaluationLimits
	}{in.Policy, in.Limits})
	if err != nil {
		return ResidentEvaluationResult{}, err
	}
	r.Report.OutputDigest, err = residentDigest(struct{ Baseline, Candidate memory.ContextProjection }{r.baseline, r.candidate})
	if err != nil {
		return ResidentEvaluationResult{}, err
	}
	r.Report.RunHash, err = residentDigest(r.Report)
	return r, err
}

func residentID(s string) bool {
	return strings.TrimSpace(s) != "" && len(s) <= 256 && utf8.ValidString(s)
}

func residentEventBounds(groups ...[]memory.RawEvent) error {
	ids := map[string]bool{}
	for _, events := range groups {
		for _, e := range events {
			if !residentID(e.ID) || ids[e.ID] || len(e.Content) > 16384 || len(e.Metadata) > 32 || !utf8.ValidString(e.Content) {
				return fmt.Errorf("resident observation record is invalid")
			}
			ids[e.ID] = true
			data, err := json.Marshal(e)
			if err != nil || len(data) > 32768 {
				return fmt.Errorf("resident observation record exceeds bounds")
			}
		}
	}
	return nil
}

func residentSourceSafe(s ResidentSource, scope memory.Scope, at time.Time) bool {
	m, v := s.Memory, s.Version
	return m.Scope == scope && m.State == memory.MemoryStateActive && m.SourceMetadata.Disclosable() && v.SourceMetadata.Disclosable() && residentID(v.ID) && v.MemoryID == m.ID && v.Version == m.SourceVersion && v.Version > 0 && v.State == memory.MemoryStateActive && v.Content == m.Content && utf8.ValidString(m.Content) && !m.ModifiedAt.IsZero() && !m.ModifiedAt.After(at) && !v.CreatedAt.IsZero() && !v.CreatedAt.After(at) &&
		(memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}).Matches(m.TemporalValidity, at) && (memory.TemporalConstraint{Mode: memory.TemporalSelectionCurrent}).Matches(v.TemporalValidity, at)
}

func residentRecordSafe(e memory.RawEvent, scope memory.Scope, at time.Time) bool {
	return e.Scope == scope && e.SourceMetadata.Disclosable() && e.SourceMetadata.Effective().Trust == memory.SourceTrustAttestedData && memory.ProceduralVerificationEvidenceKind(e.EventType).IndependentlyCorroborating() && !e.SourceTimestamp.IsZero() && !e.SourceTimestamp.After(at) && at.Sub(e.SourceTimestamp) <= residentObservationMaxAge && !e.CreatedAt.IsZero() && !e.CreatedAt.Before(e.SourceTimestamp) && !e.CreatedAt.After(at)
}

// Strict field and duplicate-key rejection keeps a raw record's interpretation
// unambiguous. The body is bounded before this routine is called.
func decodeResidentObservation(content string, out any) bool {
	// encoding/json accepts case-insensitive struct aliases. Reject every key
	// outside the exact schema before decoding so aliases cannot override facts.
	var fields string
	switch out.(type) {
	case *ResidentSupportObservation:
		fields = "memory_id source_version claim_fingerprint actor stable applicable stage expires_at"
	case *ResidentUtilityObservation:
		fields = "memory_id source_version claim_fingerprint actor stage expires_at useful"
	case *ResidentConditionObservation:
		fields = "condition source_digest output_digest actor stage task_succeeded repeated_errors stale_evidence_harm maintenance_cost tool_calls model_tokens"
	default:
		return false
	}
	allowed := map[string]bool{}
	for _, field := range strings.Fields(fields) {
		allowed[field] = true
	}
	d := json.NewDecoder(strings.NewReader(content))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return false
	}
	keys := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return false
		}
		s, ok := key.(string)
		if !ok || !allowed[s] || keys[s] {
			return false
		}
		keys[s] = true
		var raw json.RawMessage
		if d.Decode(&raw) != nil || bytes.Equal(raw, []byte("null")) {
			return false
		}
	}
	if _, err = d.Token(); err != nil {
		return false
	}
	if _, err = d.Token(); err != io.EOF {
		return false
	}
	d = json.NewDecoder(strings.NewReader(content))
	d.DisallowUnknownFields()
	return d.Decode(out) == nil
}

func residentSupport(s ResidentSource, in ResidentEvaluationInput) (ResidentSupportObservation, memory.RawEvent, bool) {
	var found ResidentSupportObservation
	var record memory.RawEvent
	count := 0
	for _, e := range in.Support {
		var body ResidentSupportObservation
		if !decodeResidentObservation(e.Content, &body) || body.MemoryID != s.Memory.ID {
			continue
		}
		count++
		found = body
		record = e
	}
	ok := count == 1 && residentRecordSafe(record, in.Scope, in.EvaluatedAt) && found.SourceVersion == s.Memory.SourceVersion && found.ClaimFingerprint == ResidentClaimFingerprint(s.Memory.Content) && residentID(found.Actor) && found.Stable && found.Applicable && found.Stage > 0 && found.Stage <= 1_000_000 && found.ExpiresAt.After(in.EvaluatedAt) && found.ExpiresAt.Sub(record.SourceTimestamp) <= residentObservationMaxAge && !record.SourceTimestamp.Before(s.Memory.ModifiedAt)
	return found, record, ok
}

func residentUtility(s ResidentSource, support ResidentSupportObservation, event memory.RawEvent, in ResidentEvaluationInput) (ResidentUtilityObservation, bool) {
	var found ResidentUtilityObservation
	var record memory.RawEvent
	count := 0
	for _, e := range in.Utility {
		var body ResidentUtilityObservation
		if !decodeResidentObservation(e.Content, &body) || body.MemoryID != s.Memory.ID {
			continue
		}
		count++
		found = body
		record = e
	}
	ok := count == 1 && residentRecordSafe(record, in.Scope, in.EvaluatedAt) && found.SourceVersion == s.Memory.SourceVersion && found.ClaimFingerprint == ResidentClaimFingerprint(s.Memory.Content) && residentID(found.Actor) && found.Stage > support.Stage && found.Stage <= 1_000_000 && found.ExpiresAt.After(in.EvaluatedAt) && found.ExpiresAt.Sub(record.SourceTimestamp) <= residentObservationMaxAge && record.SourceTimestamp.After(event.SourceTimestamp) && found.Useful
	return found, ok
}

func materializeResident(ctx context.Context, in ResidentEvaluationInput, sources []memory.ContextProjectionCandidate, version int64) (memory.ContextProjection, int, error) {
	p, err := memory.MaterializeContextProjection(ctx, memory.MaterializeContextProjectionInput{Scope: in.Scope, Kind: memory.ContextProjectionKindAlwaysVisible, Version: version, SchemaVersion: "context-projection-v1", Policy: in.Policy, RendererVersion: "resident-evaluation-v1", Sources: sources})
	if err != nil {
		return memory.ContextProjection{}, 0, err
	}
	selected := make([]memory.ContextProjectionItem, 0, len(p.Items))
	validity := make(map[string]memory.TemporalValidity, len(in.Sources))
	for _, source := range in.Sources {
		validity[source.Version.ID] = source.Version.TemporalValidity
	}
	omitted := 0
	for _, item := range p.Items {
		// The shared materializer accepts no fact-validity snapshot. Preserve
		// this evaluator's resolved immutable interval before returning a plan.
		item.TemporalValidity = cloneResidentValidity(validity[item.Source.ID])
		metrics := residentMetrics(append(selected, item))
		if metrics.Items > in.Limits.MaxItems || metrics.ContentBytes > in.Limits.MaxContentBytes || metrics.Tokens > in.Limits.MaxTokens || metrics.Citations > in.Limits.MaxCitations {
			omitted++
			continue
		}
		selected = append(selected, item)
	}
	p.Items = selected
	p.SourceWatermark.CanonicalVersionIDs = nil
	for _, item := range selected {
		p.SourceWatermark.CanonicalVersionIDs = append(p.SourceWatermark.CanonicalVersionIDs, item.Source.ID)
	}
	sort.Strings(p.SourceWatermark.CanonicalVersionIDs)
	p.CreatedAt = in.EvaluatedAt.UTC()
	p.UpdatedAt = p.CreatedAt
	p.FreshnessDuration = 0
	return p, omitted, p.Validate()
}

func residentMetrics(items []memory.ContextProjectionItem) ResidentPlanMetrics {
	m := ResidentPlanMetrics{Items: len(items)}
	for _, item := range items {
		m.ContentBytes += len(item.Text)
		m.Tokens += len(strings.Fields(item.Text))
		m.Citations++
	}
	return m
}
