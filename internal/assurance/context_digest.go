package assurance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/provider"
)

// This consumer deliberately builds canonical bytes independently of the
// service's canonical structs, admission, selection and hash implementation.
// Ordered objects use explicitly named fields; citation optionals become "".
func consumerContextDigest(scope memory.Scope, r provider.DisclosedContextResponse) (string, error) {
	field := func(k string, v any) string { b, _ := json.Marshal(v); return `"` + k + `":` + string(b) }
	object := func(fields ...string) string {
		var b bytes.Buffer
		b.WriteByte('{')
		for i, f := range fields {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(f)
		}
		b.WriteByte('}')
		return b.String()
	}
	citations := func(cs []provider.Citation) json.RawMessage {
		out := []json.RawMessage{}
		for _, c := range cs {
			out = append(out, json.RawMessage(object(field("source_kind", c.SourceKind), field("reference", c.Reference), field("version", c.Version), field("watermark", c.Watermark), field("availability", c.Availability))))
		}
		b, _ := json.Marshal(out)
		return b
	}
	d, a := r.Result.Disclosure, r.Result.BudgetAccounting
	categories := []json.RawMessage{}
	for _, section := range []struct {
		name  string
		items []provider.DisclosedContextItem
	}{{"profile", r.Result.Profile}, {"recent_session", r.Result.RecentSession}, {"recent_episodes", r.Result.RecentEpisodes}, {"relevant_summaries", r.Result.RelevantSummaries}, {"related_entities", r.Result.RelatedEntities}} {
		items := []json.RawMessage{}
		for _, item := range section.items {
			ref := item.Reference
			if item.Memory != nil {
				ref = &item.Memory.ContextReference
			}
			if ref == nil {
				return "", fmt.Errorf("canonical item unavailable")
			}
			fields := []string{field("kind", ref.Kind), field("id", ref.MemoryID), field("version", ref.Version), field("memory_path", ref.MemoryPath), field("class", ref.Class), field("availability", ref.Availability), field("source_metadata", ref.SourceMetadata)}
			if item.Memory != nil {
				fields = append(fields, field("content", item.Memory.Content))
			}
			fields = append(fields, field("citations", citations(item.Citations)))
			items = append(items, json.RawMessage(object(fields...)))
		}
		categories = append(categories, json.RawMessage(object(field("name", section.name), field("items", items))))
	}
	canonical := object(field("contract_version", "context-digest-v1"), field("canonicalization_version", "context-canonical-v1"), field("scope", json.RawMessage(object(field("tenant", scope.Tenant), field("project", scope.Project), field("namespace", scope.Namespace)))), field("response_mode", r.Result.ResponseMode), field("projection_intent", d.ProjectionIntent), field("disclosure", json.RawMessage(object(field("policy_version", d.PolicyVersion), field("profile", d.Profile), field("source_metadata_version", d.SourceMetadataVersion)))), field("budget", a.Effective), field("selection_policy", json.RawMessage(object(field("disclosure_version", d.PolicyVersion), field("source_metadata_version", d.SourceMetadataVersion), field("budget_version", a.ContractVersion), field("packing_version", a.PackingPolicyVersion), field("selection_policy_version", "context-selection-v1"), field("selection_policy_fingerprint", r.Result.ContextDigest.SelectionPolicyFingerprint)))), field("selection", a.Selection), field("categories", categories), field("citations", citations(r.Citations)))
	sum := sha256.Sum256([]byte("stele-context-digest-v1\n" + canonical))
	return hex.EncodeToString(sum[:]), nil
}

// Duplicate JSON members are ambiguous even when a schema accepts the decoded
// map. Validate before decoding, including nested objects.
func uniqueJSON(data []byte) bool {
	d := json.NewDecoder(bytes.NewReader(data))
	var visit func() bool
	visit = func() bool {
		tok, err := d.Token()
		if err != nil {
			return false
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				name, ok := key.(string)
				if err != nil || !ok || seen[name] {
					return false
				}
				seen[name] = true
				if !visit() {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim('}')
		case '[':
			for d.More() {
				if !visit() {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim(']')
		}
		return false
	}
	if !visit() {
		return false
	}
	_, err := d.Token()
	return err == io.EOF
}

func validContinuityResult(c provider.ContextContinuityResult) bool {
	if c.ContractVersion != provider.ContextContinuityVersion {
		return false
	}
	if c.ReadSemantics == "fresh" && c.Outcome == "not_requested" {
		return c.Reference == "" && c.ExpiresAt == ""
	}
	if !(c.ReadSemantics == "fresh" && c.Outcome == "issued" || c.ReadSemantics == "fresh_compare" && (c.Outcome == "changed" || c.Outcome == "unchanged")) || !provider.ValidContextReference(c.Reference) || len(c.ExpiresAt) != 20 {
		return false
	}
	t, err := time.Parse(time.RFC3339, c.ExpiresAt)
	return err == nil && t.UTC().Format(time.RFC3339) == c.ExpiresAt
}
