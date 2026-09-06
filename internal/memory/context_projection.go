package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// MaxContextProjectionItemTextBytes bounds the rendered text persisted in a
// projection item. Source payloads remain in their canonical tables.
const MaxContextProjectionItemTextBytes = 4096

type ContextProjectionKind string

const (
	ContextProjectionKindAlwaysVisible   ContextProjectionKind = "always_visible"
	ContextProjectionKindSession         ContextProjectionKind = "session"
	ContextProjectionKindRetrieval       ContextProjectionKind = "retrieval"
	ContextProjectionKindArchivalHistory ContextProjectionKind = "archival_history"
)

func (k ContextProjectionKind) Valid() bool {
	switch k {
	case ContextProjectionKindAlwaysVisible, ContextProjectionKindSession,
		ContextProjectionKindRetrieval, ContextProjectionKindArchivalHistory:
		return true
	default:
		return false
	}
}

type ContextProjectionStatus string

const (
	ContextProjectionStatusBuilding   ContextProjectionStatus = "building"
	ContextProjectionStatusActive     ContextProjectionStatus = "active"
	ContextProjectionStatusSuperseded ContextProjectionStatus = "superseded"
	ContextProjectionStatusFailed     ContextProjectionStatus = "failed"
)

func (s ContextProjectionStatus) Valid() bool {
	switch s {
	case ContextProjectionStatusBuilding, ContextProjectionStatusActive,
		ContextProjectionStatusSuperseded, ContextProjectionStatusFailed:
		return true
	default:
		return false
	}
}

type ContextProjectionSourceKind string

const (
	ContextProjectionSourceCanonicalVersion ContextProjectionSourceKind = "canonical_version"
	ContextProjectionSourceRawEvent         ContextProjectionSourceKind = "raw_event"
)

func (k ContextProjectionSourceKind) Valid() bool {
	return k == ContextProjectionSourceCanonicalVersion || k == ContextProjectionSourceRawEvent
}

type ContextProjectionSource struct {
	Kind     ContextProjectionSourceKind `json:"kind"`
	ID       string                      `json:"id"`
	Version  int64                       `json:"version,omitempty"`
	MemoryID string                      `json:"memory_id,omitempty"`
	Scope    Scope                       `json:"scope"`
	// LifecycleState is optional for legacy source references. When present,
	// only active evidence is eligible for default projection.
	LifecycleState MemoryState `json:"lifecycle_state,omitempty"`
}

func (s ContextProjectionSource) Validate() error {
	if !s.Kind.Valid() {
		return fmt.Errorf("invalid projection source kind %q", s.Kind)
	}
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("projection source id is required")
	}
	if err := s.Scope.Validate(); err != nil {
		return fmt.Errorf("projection source scope: %w", err)
	}
	if s.Kind == ContextProjectionSourceCanonicalVersion && s.Version <= 0 {
		return fmt.Errorf("canonical projection source version must be greater than zero")
	}
	if s.Kind == ContextProjectionSourceRawEvent && s.Version != 0 {
		return fmt.Errorf("raw event projection source cannot have a version")
	}
	if s.LifecycleState != "" && !s.LifecycleState.Valid() {
		return fmt.Errorf("invalid projection source lifecycle state %q", s.LifecycleState)
	}
	return nil
}

type ContextProjectionWatermark struct {
	CanonicalVersionIDs []string  `json:"canonical_version_ids,omitempty"`
	RawEventIDs         []string  `json:"raw_event_ids,omitempty"`
	WindowFrom          time.Time `json:"window_from,omitempty"`
	WindowTo            time.Time `json:"window_to,omitempty"`
}

func (w ContextProjectionWatermark) Validate() error {
	if !w.WindowFrom.IsZero() && !w.WindowTo.IsZero() && w.WindowFrom.After(w.WindowTo) {
		return fmt.Errorf("projection watermark window_from must be before or equal to window_to")
	}
	return nil
}

type ContextProjectionItem struct {
	ID             string                  `json:"id"`
	Source         ContextProjectionSource `json:"source"`
	Text           string                  `json:"text"`
	Class          MemoryClass             `json:"class"`
	LifecycleState MemoryState             `json:"lifecycle_state"`
	SortKey        string                  `json:"sort_key"`
	Citation       ProjectionCitation      `json:"citation,omitempty"`
}

type ProjectionCitation struct {
	MemoryID   string `json:"memory_id,omitempty"`
	RawEventID string `json:"raw_event_id,omitempty"`
	Operation  string `json:"operation,omitempty"`
}

func (i ContextProjectionItem) Validate(parent Scope) error {
	if strings.TrimSpace(i.ID) == "" {
		return fmt.Errorf("projection item id is required")
	}
	if err := i.Source.Validate(); err != nil {
		return err
	}
	if i.Source.Scope.Normalized() != parent.Normalized() {
		return fmt.Errorf("projection item source scope does not match projection scope")
	}
	if len([]byte(i.Text)) == 0 {
		return fmt.Errorf("projection item text is required")
	}
	if len([]byte(i.Text)) > MaxContextProjectionItemTextBytes {
		return fmt.Errorf("projection item text exceeds %d bytes", MaxContextProjectionItemTextBytes)
	}
	if !validMemoryClass(i.Class) {
		return fmt.Errorf("invalid projection item memory class %q", i.Class)
	}
	if i.LifecycleState != MemoryStateActive {
		return fmt.Errorf("projection item lifecycle state %q is not visible", i.LifecycleState)
	}
	if strings.TrimSpace(i.SortKey) == "" {
		return fmt.Errorf("projection item sort key is required")
	}
	return nil
}

type ContextProjection struct {
	ID              string                     `json:"id"`
	Scope           Scope                      `json:"scope"`
	Kind            ContextProjectionKind      `json:"kind"`
	Version         int64                      `json:"version"`
	SchemaVersion   string                     `json:"schema_version"`
	PolicyVersion   string                     `json:"policy_version"`
	RendererVersion string                     `json:"renderer_version"`
	SourceWatermark ContextProjectionWatermark `json:"source_watermark"`
	Status          ContextProjectionStatus    `json:"status"`
	Items           []ContextProjectionItem    `json:"items"`
	CreatedAt       time.Time                  `json:"created_at"`
	UpdatedAt       time.Time                  `json:"updated_at"`
	SupersededAt    time.Time                  `json:"superseded_at,omitempty"`
}

func (p ContextProjection) SourceWatermarkHash() string {
	encoded, _ := json.Marshal(p.SourceWatermark)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (p ContextProjection) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("projection id is required")
	}
	if err := p.Scope.Validate(); err != nil {
		return fmt.Errorf("projection scope: %w", err)
	}
	if !p.Kind.Valid() {
		return fmt.Errorf("invalid projection kind %q", p.Kind)
	}
	if p.Version <= 0 {
		return fmt.Errorf("projection version must be greater than zero")
	}
	if strings.TrimSpace(p.SchemaVersion) == "" || strings.TrimSpace(p.PolicyVersion) == "" || strings.TrimSpace(p.RendererVersion) == "" {
		return fmt.Errorf("projection schema, policy, and renderer versions are required")
	}
	if !p.Status.Valid() {
		return fmt.Errorf("invalid projection status %q", p.Status)
	}
	if err := p.SourceWatermark.Validate(); err != nil {
		return err
	}
	for _, item := range p.Items {
		if err := item.Validate(p.Scope); err != nil {
			return err
		}
	}
	return nil
}

// SortContextProjectionItems imposes a stable order independent of database
// row order or map iteration.
func SortContextProjectionItems(items []ContextProjectionItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortKey != items[j].SortKey {
			return items[i].SortKey < items[j].SortKey
		}
		if items[i].Source.Kind != items[j].Source.Kind {
			return items[i].Source.Kind < items[j].Source.Kind
		}
		if items[i].Source.ID != items[j].Source.ID {
			return items[i].Source.ID < items[j].Source.ID
		}
		return items[i].ID < items[j].ID
	})
}

func validMemoryClass(class MemoryClass) bool {
	switch class {
	case MemoryClassProfile, MemoryClassEpisodic, MemoryClassProcedural, MemoryClassSummary, MemoryClassRelation:
		return true
	default:
		return false
	}
}
