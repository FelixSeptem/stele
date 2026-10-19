package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"unicode/utf8"

	"github.com/FelixSeptem/stele/internal/memory"
)

const ContextDigestVersion = "context-digest-v1"
const ContextCanonicalVersion = "context-canonical-v1"
const ContextTurnDigestVersion = "context-turn-digest-v1"
const ContextTurnCanonicalVersion = "context-turn-canonical-v1"
const ContextSelectionVersion = "context-selection-v1"
const ContextContinuityVersion = "context-continuity-v1"

var continuityReferencePattern = regexp.MustCompile(`^ctxc_[0-9a-f]{32}$`)
var contextHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type ContextContinuityInput struct {
	Mode      string `json:"mode"`
	Reference string `json:"reference,omitempty"`
}
type ContextContinuityResult struct {
	ContractVersion string `json:"contract_version"`
	ReadSemantics   string `json:"read_semantics"`
	Outcome         string `json:"outcome"`
	Reference       string `json:"reference,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
}
type ContextDigest struct {
	ContractVersion            string `json:"contract_version"`
	Algorithm                  string `json:"algorithm"`
	CanonicalizationVersion    string `json:"canonicalization_version"`
	SelectionPolicyVersion     string `json:"selection_policy_version"`
	SelectionPolicyFingerprint string `json:"selection_policy_fingerprint"`
	Value                      string `json:"value"`
}

func ValidContextReference(s string) bool { return continuityReferencePattern.MatchString(s) }
func ValidContextHash(s string) bool      { return contextHashPattern.MatchString(s) }
func hashContext(domain string, b []byte) string {
	sum := sha256.Sum256(append([]byte(domain), b...))
	return hex.EncodeToString(sum[:])
}

// ValidJSONUnicode rejects encoding/json's lossy replacement of invalid UTF-8
// and unpaired surrogate escapes, before any public context string is decoded.
func ValidJSONUnicode(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}
	inside := false
	for i := 0; i < len(data); i++ {
		if data[i] == '"' {
			inside = !inside
			continue
		}
		if !inside || data[i] != '\\' {
			continue
		}
		i++
		if i >= len(data) {
			return false
		}
		if data[i] != 'u' {
			continue
		}
		if i+4 >= len(data) {
			return false
		}
		v, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if v >= 0xdc00 && v <= 0xdfff {
			return false
		}
		if v >= 0xd800 && v <= 0xdbff {
			if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
				return false
			}
			lo, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
			if err != nil || lo < 0xdc00 || lo > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return true
}

type canonicalScope struct {
	Tenant    string `json:"tenant"`
	Project   string `json:"project"`
	Namespace string `json:"namespace"`
}
type canonicalDisclosure struct {
	PolicyVersion         string `json:"policy_version"`
	Profile               string `json:"profile"`
	SourceMetadataVersion string `json:"source_metadata_version"`
}
type canonicalPolicy struct {
	DisclosureVersion          string `json:"disclosure_version"`
	SourceMetadataVersion      string `json:"source_metadata_version"`
	BudgetVersion              string `json:"budget_version"`
	PackingVersion             string `json:"packing_version"`
	SelectionPolicyVersion     string `json:"selection_policy_version"`
	SelectionPolicyFingerprint string `json:"selection_policy_fingerprint"`
}
type canonicalCitation struct {
	SourceKind   string `json:"source_kind"`
	Reference    string `json:"reference"`
	Version      string `json:"version"`
	Watermark    string `json:"watermark"`
	Availability string `json:"availability"`
}
type canonicalItem struct {
	Kind           string                `json:"kind"`
	ID             string                `json:"id"`
	Version        string                `json:"version"`
	MemoryPath     string                `json:"memory_path"`
	Class          memory.MemoryClass    `json:"class"`
	Availability   string                `json:"availability"`
	SourceMetadata memory.SourceMetadata `json:"source_metadata"`
	Content        *string               `json:"content,omitempty"`
	Citations      []canonicalCitation   `json:"citations"`
}
type canonicalCategory struct {
	Name  string          `json:"name"`
	Items []canonicalItem `json:"items"`
}
type canonicalContext struct {
	ContractVersion         string                 `json:"contract_version"`
	CanonicalizationVersion string                 `json:"canonicalization_version"`
	Scope                   canonicalScope         `json:"scope"`
	ResponseMode            string                 `json:"response_mode"`
	ProjectionIntent        string                 `json:"projection_intent"`
	Disclosure              canonicalDisclosure    `json:"disclosure"`
	Budget                  ContextBudgetEffective `json:"budget"`
	SelectionPolicy         canonicalPolicy        `json:"selection_policy"`
	Selection               ContextBudgetSelection `json:"selection"`
	Categories              []canonicalCategory    `json:"categories"`
	Citations               []canonicalCitation    `json:"citations"`
}

func canonicalCitations(in []Citation) []canonicalCitation {
	out := make([]canonicalCitation, 0, len(in))
	for _, c := range in {
		out = append(out, canonicalCitation{c.SourceKind, c.Reference, c.Version, c.Watermark, c.Availability})
	}
	return out
}

// CanonicalContextBytes consumes the final public projection only. It never
// retrieves, ranks, packs or uses hidden content to identify reference output.
func CanonicalContextBytes(scope memory.Scope, r DisclosedContextResponse, fingerprint string) ([]byte, error) {
	if scope.Validate() != nil || !ValidContextHash(fingerprint) || !reflect.DeepEqual(r.Citations, r.Result.Citations) {
		return nil, fmt.Errorf("invalid canonical context")
	}
	d := r.Result.Disclosure
	a := r.Result.BudgetAccounting
	c := canonicalContext{ContextDigestVersion, ContextCanonicalVersion, canonicalScope{scope.Tenant, scope.Project, scope.Namespace}, r.Result.ResponseMode, d.ProjectionIntent, canonicalDisclosure{d.PolicyVersion, d.Profile, d.SourceMetadataVersion}, a.Effective, canonicalPolicy{d.PolicyVersion, d.SourceMetadataVersion, a.ContractVersion, a.PackingPolicyVersion, ContextSelectionVersion, fingerprint}, a.Selection, []canonicalCategory{}, canonicalCitations(r.Citations)}
	if c.Selection.OmissionReasons == nil {
		c.Selection.OmissionReasons = []string{}
	}
	if c.Selection.UnsupportedSections == nil {
		c.Selection.UnsupportedSections = []string{}
	}
	for _, section := range []struct {
		name  string
		items []DisclosedContextItem
	}{{"profile", r.Result.Profile}, {"recent_session", r.Result.RecentSession}, {"recent_episodes", r.Result.RecentEpisodes}, {"relevant_summaries", r.Result.RelevantSummaries}, {"related_entities", r.Result.RelatedEntities}} {
		cat := canonicalCategory{Name: section.name, Items: []canonicalItem{}}
		for _, item := range section.items {
			ref := item.Reference
			var content *string
			if item.Memory != nil {
				ref = &item.Memory.ContextReference
				content = &item.Memory.Content
			}
			if ref == nil || (item.Reference == nil) == (item.Memory == nil) || (r.Result.ResponseMode == "bounded_content") != (content != nil) {
				return nil, fmt.Errorf("invalid canonical item")
			}
			cat.Items = append(cat.Items, canonicalItem{ref.Kind, ref.MemoryID, ref.Version, ref.MemoryPath, ref.Class, ref.Availability, ref.SourceMetadata, content, canonicalCitations(item.Citations)})
		}
		c.Categories = append(c.Categories, cat)
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	// The service must not silently hash replacement characters for bad sources.
	if !validStrings(reflect.ValueOf(c)) {
		return nil, fmt.Errorf("invalid context Unicode")
	}
	return b, nil
}
func validStrings(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return utf8.ValidString(v.String())
	case reflect.Pointer:
		if !v.IsNil() {
			return validStrings(v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !validStrings(v.Field(i)) {
				return false
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if !validStrings(v.Index(i)) {
				return false
			}
		}
	}
	return true
}
func ComputeContextDigest(scope memory.Scope, r DisclosedContextResponse, fingerprint string) (ContextDigest, error) {
	b, err := CanonicalContextBytes(scope, r, fingerprint)
	if err != nil {
		return ContextDigest{}, err
	}
	if r.Result.TurnID == "" {
		return ContextDigest{ContextDigestVersion, "sha-256", ContextCanonicalVersion, ContextSelectionVersion, fingerprint, hashContext("stele-context-digest-v1\n", b)}, nil
	}
	if !boundedToken(r.Result.TurnID, 128) {
		return ContextDigest{}, fmt.Errorf("invalid context turn")
	}
	turnBytes, err := json.Marshal(struct {
		TurnID     string          `json:"turn_id"`
		Projection json.RawMessage `json:"projection"`
	}{r.Result.TurnID, b})
	if err != nil || !ValidJSONUnicode(turnBytes) {
		return ContextDigest{}, fmt.Errorf("invalid turn canonical context")
	}
	return ContextDigest{ContextTurnDigestVersion, "sha-256", ContextTurnCanonicalVersion, ContextSelectionVersion, fingerprint, hashContext("stele-context-turn-digest-v1\n", turnBytes)}, nil
}

func SelectionPolicyFingerprint(manifest []byte) string {
	return hashContext("stele-context-selection-v1\n", manifest)
}

const DefaultContextPolicyManifest = `{"manifest_version":"context-selection-manifest-v1","eligibility_version":"active-current-disclosable-v1","selection_version":"context-selection-v1","fusion_version":"rrf-v1","quality_version":"quality-v1","disclosure_version":"context-disclosure-v1","source_metadata_version":"source-metadata-v1","budget_version":"context-budget-v1","packing_version":"context-packing-v1"}`

func ContextReadFingerprint(b RuntimeBinding, input ContextInput, r DisclosedContextResponse) (string, error) {
	selector, err := memory.NewMemoryPathSelector(input.Path, input.PathPrefix)
	if err != nil {
		return "", err
	}
	if selector.Kind == memory.MemoryPathSelectorNone {
		selector = memory.MemoryPathSelector{Kind: memory.MemoryPathSelectorPrefix, Value: memory.MemoryPathRoot}
	}
	// Remove volatile binding timestamps; all attribution strings remain bound.
	identity := struct {
		BindingID          string         `json:"binding_id"`
		PrincipalID        string         `json:"principal_id"`
		Scope              canonicalScope `json:"scope"`
		AgentID            string         `json:"agent_id"`
		SessionID          string         `json:"session_id"`
		ConversationID     string         `json:"conversation_id"`
		ProviderInstanceID string         `json:"provider_instance_id"`
	}{b.BindingID, b.PrincipalID, canonicalScope{b.Scope.Tenant, b.Scope.Project, b.Scope.Namespace}, b.AgentID, b.SessionID, b.ConversationID, b.ProviderInstanceID}
	doc := struct {
		ContractVersion string                 `json:"contract_version"`
		Identity        any                    `json:"identity"`
		Query           string                 `json:"query"`
		Selector        any                    `json:"selector"`
		Mode            string                 `json:"response_mode"`
		Profile         string                 `json:"profile"`
		Intent          string                 `json:"projection_intent"`
		Budget          ContextBudgetEffective `json:"budget"`
		Options         [6]bool                `json:"options"`
		TurnID          string                 `json:"turn_id,omitempty"`
	}{"context-read-v1", identity, input.Query, struct {
		Kind  memory.MemoryPathSelectorKind `json:"kind"`
		Value string                        `json:"value"`
	}{selector.Kind, selector.Value}, r.Result.ResponseMode, r.Result.Disclosure.Profile, r.Result.Disclosure.ProjectionIntent, r.Result.BudgetAccounting.Effective, [6]bool{input.IncludeRelations, input.IncludeExperienceInsights, input.IncludeGoalContext, input.IncludeDiagnostics, input.IncludeFeedbackDiagnostics, input.FeedbackAwareRanking}, input.TurnID}
	data, err := json.Marshal(doc)
	if err != nil || !ValidJSONUnicode(data) {
		return "", fmt.Errorf("invalid originating read")
	}
	return hashContext("stele-context-read-v1\n", bytes.TrimSpace(data)), nil
}
