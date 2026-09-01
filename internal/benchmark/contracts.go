package benchmark

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

const SchemaVersion = "v1"

type RedistributionStatus string

const (
	RedistributionPermitted  RedistributionStatus = "permitted"
	RedistributionRestricted RedistributionStatus = "restricted"
	RedistributionUnknown    RedistributionStatus = "unknown"
)

type SupportState string

const (
	SupportRunnable     SupportState = "runnable"
	SupportMetadataOnly SupportState = "metadata-only"
	SupportPlanned      SupportState = "planned"
)

type EmbeddingProfile struct {
	Name          string `json:"name"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	Revision      string `json:"revision,omitempty"`
	Dimensions    int    `json:"dimensions"`
	Normalization string `json:"normalization"`
	VectorSource  string `json:"vector_source,omitempty"`
}

func (p EmbeddingProfile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("embedding profile name is required")
	}
	if p.Dimensions < 0 {
		return errors.New("embedding dimensions cannot be negative")
	}
	if strings.TrimSpace(p.Normalization) == "" {
		return errors.New("embedding normalization is required")
	}
	return nil
}

type SplitSpec struct {
	Source     string `json:"source"`
	MaxQueries int    `json:"max_queries,omitempty"`
	Checksum   string `json:"sha256,omitempty"`
}

type DatasetManifest struct {
	SchemaVersion     string               `json:"schema_version"`
	Name              string               `json:"name"`
	Version           string               `json:"version"`
	License           string               `json:"license"`
	UpstreamURL       string               `json:"upstream_url"`
	UpstreamRevision  string               `json:"upstream_revision"`
	SHA256            string               `json:"sha256"`
	SourcePath        string               `json:"source_path"`
	ConversionVersion string               `json:"conversion_version"`
	Redistribution    RedistributionStatus `json:"redistribution"`
	Support           SupportState         `json:"support"`
	Splits            map[string]SplitSpec `json:"splits"`
	Embedding         EmbeddingProfile     `json:"embedding"`
}

var sha256Pattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

func (m DatasetManifest) Validate() error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported manifest schema %q", m.SchemaVersion)
	}
	for name, value := range map[string]string{
		"name": m.Name, "version": m.Version, "license": m.License,
		"upstream_url": m.UpstreamURL, "upstream_revision": m.UpstreamRevision,
		"source_path": m.SourcePath, "conversion_version": m.ConversionVersion,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if !sha256Pattern.MatchString(m.SHA256) {
		return errors.New("sha256 must be a 64-character hexadecimal digest")
	}
	if m.Redistribution == "" {
		return errors.New("redistribution status is required")
	}
	if m.Support == "" {
		return errors.New("support state is required")
	}
	if len(m.Splits) == 0 {
		return errors.New("at least one split is required")
	}
	for name, split := range m.Splits {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(split.Source) == "" {
			return errors.New("split name and source are required")
		}
		if split.MaxQueries < 0 {
			return fmt.Errorf("split %q max_queries cannot be negative", name)
		}
		if split.Checksum != "" && !sha256Pattern.MatchString(split.Checksum) {
			return fmt.Errorf("split %q checksum is malformed", name)
		}
	}
	if err := m.Embedding.Validate(); err != nil {
		return err
	}
	return nil
}

func LoadDatasetManifest(source io.Reader) (DatasetManifest, error) {
	if source == nil {
		return DatasetManifest{}, errors.New("dataset manifest source is required")
	}
	decoder := json.NewDecoder(source)
	decoder.DisallowUnknownFields()
	var manifest DatasetManifest
	if err := decoder.Decode(&manifest); err != nil {
		return DatasetManifest{}, fmt.Errorf("decode dataset manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return DatasetManifest{}, err
	}
	return manifest, nil
}

func checksumBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func checksumCanonicalText(data []byte) string {
	return checksumBytes(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
}

type ConversationTurn struct {
	ID        string `json:"id"`
	Speaker   string `json:"speaker,omitempty"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp,omitempty"`
	Source    string `json:"source,omitempty"`
}

type ConversationRecord struct {
	ID         string             `json:"id"`
	Scope      memory.Scope       `json:"scope"`
	Turns      []ConversationTurn `json:"turns"`
	Provenance map[string]string  `json:"provenance,omitempty"`
}

type MemoryEventRecord struct {
	ID            string             `json:"id"`
	Scope         memory.Scope       `json:"scope"`
	SessionID     string             `json:"session_id,omitempty"`
	SourceTurnID  string             `json:"source_turn_id,omitempty"`
	Class         memory.MemoryClass `json:"class"`
	Text          string             `json:"text"`
	ObservedAt    string             `json:"observed_at,omitempty"`
	ExpectedState memory.MemoryState `json:"expected_state"`
	Provenance    map[string]string  `json:"provenance,omitempty"`
}

type EvidenceGroup struct {
	ID          string   `json:"id"`
	EvidenceIDs []string `json:"evidence_ids"`
	Required    bool     `json:"required"`
}

type BenchmarkQuery struct {
	ID                 string          `json:"id"`
	Scope              memory.Scope    `json:"scope"`
	SessionID          string          `json:"session_id,omitempty"`
	Text               string          `json:"text"`
	QueryType          string          `json:"query_type,omitempty"`
	QuestionDate       string          `json:"question_date,omitempty"`
	AnswerSessionIDs   []string        `json:"answer_session_ids,omitempty"`
	AbstentionExpected bool            `json:"abstention_expected,omitempty"`
	UpdateType         string          `json:"update_type,omitempty"`
	EvidenceGroups     []EvidenceGroup `json:"evidence_groups,omitempty"`
	MustNotReturnIDs   []string        `json:"must_not_return_ids,omitempty"`
}

type QREL struct {
	QueryID     string `json:"query_id"`
	EvidenceID  string `json:"evidence_id"`
	Grade       int    `json:"grade"`
	Role        string `json:"role,omitempty"`
	GroupID     string `json:"group_id,omitempty"`
	Expectation string `json:"expectation,omitempty"`
}

type NormalizedCorpus struct {
	SchemaVersion string               `json:"schema_version"`
	Conversations []ConversationRecord `json:"conversations"`
	Events        []MemoryEventRecord  `json:"events"`
	Queries       []BenchmarkQuery     `json:"queries"`
	QRELs         []QREL               `json:"qrels"`
}

func (c NormalizedCorpus) Validate() error {
	if c.SchemaVersion != "" && c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported corpus schema %q", c.SchemaVersion)
	}
	ids := map[string]string{}
	eventIDs := map[string]struct{}{}
	queryIDs := map[string]struct{}{}
	addID := func(id, kind string) error {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("%s id is required", kind)
		}
		if previous, ok := ids[id]; ok {
			return fmt.Errorf("duplicate id %q in %s and %s", id, previous, kind)
		}
		ids[id] = kind
		return nil
	}
	for _, conversation := range c.Conversations {
		if err := addID(conversation.ID, "conversation"); err != nil {
			return err
		}
		if err := conversation.Scope.Validate(); err != nil {
			return fmt.Errorf("conversation %s: %w", conversation.ID, err)
		}
		turnIDs := map[string]struct{}{}
		for _, turn := range conversation.Turns {
			if strings.TrimSpace(turn.ID) == "" || strings.TrimSpace(turn.Text) == "" {
				return fmt.Errorf("conversation %s has malformed turn", conversation.ID)
			}
			if _, ok := turnIDs[turn.ID]; ok {
				return fmt.Errorf("conversation %s has duplicate turn %q", conversation.ID, turn.ID)
			}
			turnIDs[turn.ID] = struct{}{}
		}
	}
	for _, event := range c.Events {
		if err := addID(event.ID, "event"); err != nil {
			return err
		}
		if err := event.Scope.Validate(); err != nil {
			return fmt.Errorf("event %s: %w", event.ID, err)
		}
		if strings.TrimSpace(event.Text) == "" {
			return fmt.Errorf("event %s text is required", event.ID)
		}
		eventIDs[event.ID] = struct{}{}
	}
	for _, query := range c.Queries {
		if err := addID(query.ID, "query"); err != nil {
			return err
		}
		if err := query.Scope.Validate(); err != nil {
			return fmt.Errorf("query %s: %w", query.ID, err)
		}
		if strings.TrimSpace(query.Text) == "" {
			return fmt.Errorf("query %s text is required", query.ID)
		}
		queryIDs[query.ID] = struct{}{}
		for _, group := range query.EvidenceGroups {
			if strings.TrimSpace(group.ID) == "" || len(group.EvidenceIDs) == 0 {
				return fmt.Errorf("query %s has malformed evidence group", query.ID)
			}
			for _, evidenceID := range group.EvidenceIDs {
				if _, found := eventIDs[evidenceID]; !found {
					return fmt.Errorf("query %s references unknown evidence %s", query.ID, evidenceID)
				}
			}
		}
	}
	for _, qrel := range c.QRELs {
		if strings.TrimSpace(qrel.QueryID) == "" || strings.TrimSpace(qrel.EvidenceID) == "" {
			return errors.New("qrel query_id and evidence_id are required")
		}
		if qrel.Grade < 0 {
			return fmt.Errorf("qrel %s/%s grade cannot be negative", qrel.QueryID, qrel.EvidenceID)
		}
		if _, found := queryIDs[qrel.QueryID]; !found {
			return fmt.Errorf("qrel references unknown query %s", qrel.QueryID)
		}
		if _, found := eventIDs[qrel.EvidenceID]; !found {
			return fmt.Errorf("qrel references unknown evidence %s", qrel.EvidenceID)
		}
	}
	return nil
}

func (c NormalizedCorpus) Checksum() (string, error) {
	copy := c.Canonical()
	data, err := json.Marshal(copy)
	if err != nil {
		return "", fmt.Errorf("marshal normalized corpus: %w", err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func (c NormalizedCorpus) Canonical() NormalizedCorpus {
	copy := c
	copy.SchemaVersion = SchemaVersion
	copy.Conversations = append([]ConversationRecord(nil), c.Conversations...)
	copy.Events = append([]MemoryEventRecord(nil), c.Events...)
	copy.Queries = append([]BenchmarkQuery(nil), c.Queries...)
	copy.QRELs = append([]QREL(nil), c.QRELs...)
	sort.Slice(copy.Conversations, func(i, j int) bool { return copy.Conversations[i].ID < copy.Conversations[j].ID })
	sort.Slice(copy.Events, func(i, j int) bool { return copy.Events[i].ID < copy.Events[j].ID })
	sort.Slice(copy.Queries, func(i, j int) bool { return copy.Queries[i].ID < copy.Queries[j].ID })
	sort.Slice(copy.QRELs, func(i, j int) bool {
		if copy.QRELs[i].QueryID == copy.QRELs[j].QueryID {
			return copy.QRELs[i].EvidenceID < copy.QRELs[j].EvidenceID
		}
		return copy.QRELs[i].QueryID < copy.QRELs[j].QueryID
	})
	return copy
}

func (c NormalizedCorpus) CanonicalJSONL() ([]byte, error) {
	copy := c.Canonical()
	var output bytes.Buffer
	write := func(kind string, record any) error {
		line, err := json.Marshal(struct {
			Kind   string `json:"kind"`
			Record any    `json:"record"`
		}{Kind: kind, Record: record})
		if err != nil {
			return err
		}
		output.Write(line)
		output.WriteByte('\n')
		return nil
	}
	for _, record := range copy.Conversations {
		if err := write("conversation", record); err != nil {
			return nil, err
		}
	}
	for _, record := range copy.Events {
		if err := write("event", record); err != nil {
			return nil, err
		}
	}
	for _, record := range copy.Queries {
		if err := write("query", record); err != nil {
			return nil, err
		}
	}
	for _, record := range copy.QRELs {
		if err := write("qrel", record); err != nil {
			return nil, err
		}
	}
	return output.Bytes(), nil
}
