package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/FelixSeptem/stele/internal/memory"
)

const TaskSeriesSchemaVersionV3 = "task-series-v3"
const TaskSeriesMetricVersionV3 = "task-series-metrics-v3"

type TaskSeriesLimits struct {
	MaxItems        int `json:"max_items"`
	MaxContentBytes int `json:"max_content_bytes"`
	MaxTokens       int `json:"max_tokens"`
	MaxCitations    int `json:"max_citations"`
	MaxLatencyMS    int `json:"max_latency_ms"`
}

type TaskSeriesExperiment struct {
	ModelDigest    string           `json:"model_digest"`
	RuntimeDigest  string           `json:"runtime_digest"`
	CriteriaDigest string           `json:"criteria_digest"`
	Seeds          []int64          `json:"seeds"`
	Limits         TaskSeriesLimits `json:"limits"`
}

// Windows describe authorized externally prepared immutable snapshots. Their
// existence does not attest that a production correction/representation ran.
type TaskSeriesSourceWindow struct {
	Reference      string                `json:"reference"`
	SourceVersion  int64                 `json:"source_version"`
	ContentDigest  string                `json:"content_digest"`
	AvailableStage int                   `json:"available_stage"`
	WithdrawnStage int                   `json:"withdrawn_stage,omitempty"`
	Metadata       memory.SourceMetadata `json:"metadata"`
}

func (w TaskSeriesSourceWindow) visible(stage int) bool {
	return w.AvailableStage <= stage && (w.WithdrawnStage == 0 || stage < w.WithdrawnStage)
}

func validateTaskSeriesExtension(m TaskSeriesManifest) error {
	v3 := m.SchemaVersion == TaskSeriesSchemaVersionV3
	if !v3 && m.Experiment != nil {
		return fmt.Errorf("v3 experiment requires task-series-v3")
	}
	for _, c := range m.Conditions {
		if !v3 && (c.Role != "" || len(c.Sources) > 0) {
			return fmt.Errorf("v3 condition requires task-series-v3")
		}
		for _, s := range c.Stages {
			if !v3 && (s.HeldOut || s.ModelTokens != nil || s.ToolCalls != nil) {
				return fmt.Errorf("v3 observation requires task-series-v3")
			}
			for _, n := range []*int64{s.ModelTokens, s.ToolCalls} {
				if n != nil && (*n < 0 || *n > maxTaskSeriesMaintenanceCost) {
					return fmt.Errorf("v3 external cost exceeds bounds")
				}
			}
		}
	}
	if !v3 {
		return nil
	}
	roles := []string{"no_memory", "source_records", "organized", "verified_corrected", "resident"}
	e := m.Experiment
	if e == nil || !sha256Pattern.MatchString(e.ModelDigest) || !sha256Pattern.MatchString(e.RuntimeDigest) || !sha256Pattern.MatchString(e.CriteriaDigest) || len(e.Seeds) < 1 || len(e.Seeds) > 32 {
		return fmt.Errorf("v3 experiment identity is incomplete")
	}
	seenSeeds := map[int64]bool{}
	for _, seed := range e.Seeds {
		if seed < 0 || seenSeeds[seed] {
			return fmt.Errorf("v3 seed set is invalid")
		}
		seenSeeds[seed] = true
	}
	l := e.Limits
	if l.MaxItems < 1 || l.MaxItems > 64 || l.MaxContentBytes < 1 || l.MaxContentBytes > 65536 || l.MaxTokens < 1 || l.MaxTokens > 16384 || l.MaxCitations < 1 || l.MaxCitations > 64 || l.MaxLatencyMS < 1 || l.MaxLatencyMS > 60000 {
		return fmt.Errorf("v3 budgets exceed bounds")
	}
	if m.Scope != m.Scope.Normalized() || len(m.Conditions) != len(roles) {
		return fmt.Errorf("v3 requires five exact-scope conditions")
	}
	first := m.Conditions[0]
	if len(first.Stages) < 2 || len(first.Stages) > 64 || first.Stages[0].HeldOut {
		return fmt.Errorf("v3 requires preparation and later held-out stages")
	}
	heldOut := false
	for _, s := range first.Stages {
		heldOut = heldOut || s.HeldOut
	}
	if !heldOut {
		return fmt.Errorf("v3 held-out task is missing")
	}
	for i, c := range m.Conditions {
		if c.Role != roles[i] || c.Baseline != (i == 0) || len(c.Sources) > 64 {
			return fmt.Errorf("v3 condition roles or sources are invalid")
		}
		for n, s := range c.Stages {
			base := first.Stages[n]
			if s.Scope != m.Scope || s.HeldOut != base.HeldOut || s.Superseded != base.Superseded {
				return fmt.Errorf("v3 stages must share held-out coverage and exact scope")
			}
		}
		if i == 0 && len(c.Sources) > 0 {
			return fmt.Errorf("v3 no-memory condition must have no sources")
		}
		seen := map[string]bool{}
		for _, w := range c.Sources {
			if validateTaskSeriesIdentifier("source", w.Reference) != nil || strings.TrimSpace(w.Reference) != w.Reference || seen[w.Reference] || w.SourceVersion < 1 || !sha256Pattern.MatchString(w.ContentDigest) || w.AvailableStage < 1 || w.AvailableStage > len(c.Stages) || (w.WithdrawnStage != 0 && (w.WithdrawnStage <= w.AvailableStage || w.WithdrawnStage > len(c.Stages)+1)) || w.Metadata.Validate() != nil || !w.Metadata.Disclosable() {
				return fmt.Errorf("v3 source window is invalid or restricted")
			}
			seen[w.Reference] = true
		}
	}
	return nil
}

func validateTaskSeriesV3Corpora(m TaskSeriesManifest, corpora map[string]NormalizedCorpus) error {
	var taskTexts []string
	for ci, c := range m.Conditions {
		corpus := corpora[c.ID]
		if len(corpus.Conversations) > 0 || len(corpus.Events) != len(c.Sources) {
			return fmt.Errorf("v3 corpus must map every event to a source window")
		}
		if ci == 0 && (len(corpus.Events) > 0 || len(corpus.QRELs) > 0) {
			return fmt.Errorf("v3 no-memory corpus is not empty")
		}
		windows := map[string]TaskSeriesSourceWindow{}
		for _, w := range c.Sources {
			windows[w.Reference] = w
		}
		for _, event := range corpus.Events {
			w, ok := windows[event.ID]
			validClass := event.Class == memory.MemoryClassProfile || event.Class == memory.MemoryClassEpisodic || event.Class == memory.MemoryClassProcedural || event.Class == memory.MemoryClassSummary || event.Class == memory.MemoryClassRelation
			if !ok || event.Scope != m.Scope || event.SessionID != "series" || len(event.Text) > 65536 || !utf8.ValidString(event.Text) || checksumBytes([]byte(event.Text)) != w.ContentDigest || !validClass {
				return fmt.Errorf("v3 source identity mismatch")
			}
		}
		queries := map[string]BenchmarkQuery{}
		for _, q := range corpus.Queries {
			queries[q.ID] = q
		}
		seenQueries := map[string]bool{}
		for si, s := range c.Stages {
			q := queries[s.QueryID]
			if seenQueries[q.ID] || q.Scope != m.Scope || q.SessionID != "series" || len(q.Text) > 16384 || !utf8.ValidString(q.Text) {
				return fmt.Errorf("v3 task identity mismatch")
			}
			seenQueries[q.ID] = true
			if ci == 0 {
				taskTexts = append(taskTexts, q.Text)
			} else if q.Text != taskTexts[si] {
				return fmt.Errorf("v3 tasks differ between conditions")
			}
			if ci == 0 && (len(s.Evidence) > 0 || len(q.EvidenceGroups) > 0) {
				return fmt.Errorf("v3 no-memory stage expects evidence")
			}
			for _, e := range s.Evidence {
				if !windows[e.Reference].visible(si+1) || windows[e.Reference].AvailableStage == 0 {
					return fmt.Errorf("v3 stage expects future or withdrawn evidence")
				}
			}
			for _, g := range q.EvidenceGroups {
				for _, ref := range g.EvidenceIDs {
					if w, ok := windows[ref]; !ok || !w.visible(si+1) {
						return fmt.Errorf("v3 query includes future or withdrawn evidence")
					}
				}
			}
		}
	}
	return nil
}

// Return a fresh physical stage corpus: nothing from a future stage, including
// qrels and answer annotations, is handed to a fixture seeder or SQL search.
func taskSeriesStageSnapshot(m TaskSeriesManifest, c TaskSeriesCondition, corpus NormalizedCorpus, index int) (NormalizedCorpus, error) {
	if index < 0 || index >= len(c.Stages) {
		return NormalizedCorpus{}, fmt.Errorf("v3 stage index is invalid")
	}
	s := c.Stages[index]
	visible := map[string]bool{}
	for _, w := range c.Sources {
		if w.visible(index + 1) {
			visible[w.Reference] = true
		}
	}
	out := NormalizedCorpus{SchemaVersion: SchemaVersion}
	for _, event := range corpus.Events {
		if visible[event.ID] {
			out.Events = append(out.Events, event)
		}
	}
	for _, query := range corpus.Queries {
		if query.ID == s.QueryID {
			// Build a minimal query without later answers or task annotations.
			q := BenchmarkQuery{ID: query.ID, Scope: m.Scope, SessionID: "series", Text: query.Text}
			for _, g := range query.EvidenceGroups {
				copy := EvidenceGroup{ID: g.ID, Required: g.Required}
				for _, ref := range g.EvidenceIDs {
					if visible[ref] {
						copy.EvidenceIDs = append(copy.EvidenceIDs, ref)
					}
				}
				if len(copy.EvidenceIDs) > 0 {
					q.EvidenceGroups = append(q.EvidenceGroups, copy)
				}
			}
			out.Queries = append(out.Queries, q)
		}
	}
	for _, qrel := range corpus.QRELs {
		if qrel.QueryID == s.QueryID && visible[qrel.EvidenceID] {
			out.QRELs = append(out.QRELs, qrel)
		}
	}
	if len(out.Queries) != 1 {
		return NormalizedCorpus{}, fmt.Errorf("v3 stage query is missing")
	}
	return out, out.Validate()
}

// Decode against exact JSON field names while walking tokens, since Go's
// standard struct decoder permits duplicate keys and case-insensitive aliases.
func validateTaskSeriesV3JSON(source []byte) error {
	d := json.NewDecoder(bytes.NewReader(source))
	return taskSeriesJSONValue(d, reflect.TypeOf(TaskSeriesSourceBundle{}))
}

func taskSeriesJSONValue(d *json.Decoder, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	if token == nil {
		if t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
			return nil
		}
		return fmt.Errorf("null scalar is invalid")
	}
	switch token {
	case json.Delim('{'):
		fields := map[string]reflect.Type{}
		if t.Kind() == reflect.Struct {
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				name := strings.Split(f.Tag.Get("json"), ",")[0]
				if name != "-" && name != "" {
					fields[name] = f.Type
				}
			}
		} else if t.Kind() != reflect.Map {
			return fmt.Errorf("object shape is invalid")
		}
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate object key")
			}
			seen[name] = true
			var child reflect.Type
			if t.Kind() == reflect.Map {
				child = t.Elem()
			} else {
				child = fields[name]
				if child == nil {
					return fmt.Errorf("noncanonical field name")
				}
			}
			if err := taskSeriesJSONValue(d, child); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	case json.Delim('['):
		if t.Kind() != reflect.Slice {
			return fmt.Errorf("array shape is invalid")
		}
		for d.More() {
			if err := taskSeriesJSONValue(d, t.Elem()); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	default:
		return nil
	}
}

// Extension presence is refused on legacy JSON even when omitempty would erase
// its zero/null value before checksum verification. Map keys remain data.
func rejectLegacyTaskSeriesV3Fields(source []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(source, &raw); err != nil {
		return err
	}
	field := func(object map[string]json.RawMessage, name string) json.RawMessage {
		for k, v := range object {
			if strings.EqualFold(k, name) {
				return v
			}
		}
		return nil
	}
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(field(raw, "manifest"), &manifest); err != nil {
		return err
	}
	if field(manifest, "experiment") != nil {
		return fmt.Errorf("v3 experiment field on legacy schema")
	}
	var conditions []map[string]json.RawMessage
	if err := json.Unmarshal(field(manifest, "conditions"), &conditions); err != nil {
		return err
	}
	for _, c := range conditions {
		if field(c, "role") != nil || field(c, "sources") != nil {
			return fmt.Errorf("v3 condition field on legacy schema")
		}
		var stages []map[string]json.RawMessage
		if err := json.Unmarshal(field(c, "stages"), &stages); err != nil {
			return err
		}
		for _, s := range stages {
			for _, name := range []string{"held_out", "model_tokens", "tool_calls"} {
				if field(s, name) != nil {
					return fmt.Errorf("v3 observation field on legacy schema")
				}
			}
		}
	}
	return nil
}
