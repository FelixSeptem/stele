package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type ChunkSourceKind string

const (
	ChunkSourceKindRawEvent         ChunkSourceKind = "raw_event"
	ChunkSourceKindCanonicalVersion ChunkSourceKind = "canonical_version"
)

func (k ChunkSourceKind) Valid() bool {
	return k == ChunkSourceKindRawEvent || k == ChunkSourceKindCanonicalVersion
}

type ChunkSourceReference struct {
	Kind           ChunkSourceKind `json:"kind"`
	ID             string          `json:"id"`
	Version        int64           `json:"version"`
	MemoryID       string          `json:"memory_id,omitempty"`
	Scope          Scope           `json:"scope"`
	SessionID      string          `json:"session_id,omitempty"`
	UserID         string          `json:"user_id,omitempty"`
	LifecycleState MemoryState     `json:"lifecycle_state,omitempty"`
}

func (s ChunkSourceReference) Validate() error {
	if !s.Kind.Valid() {
		return fmt.Errorf("invalid chunk source kind %q", s.Kind)
	}
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("chunk source id is required")
	}
	if err := s.Scope.Validate(); err != nil {
		return fmt.Errorf("chunk source scope: %w", err)
	}
	if s.Version <= 0 {
		return fmt.Errorf("chunk source version must be greater than zero")
	}
	if s.Kind == ChunkSourceKindCanonicalVersion && strings.TrimSpace(s.MemoryID) == "" {
		return fmt.Errorf("canonical chunk source memory id is required")
	}
	if s.LifecycleState != "" && s.LifecycleState != MemoryStateActive {
		return fmt.Errorf("chunk source lifecycle state %q is not visible", s.LifecycleState)
	}
	return nil
}

type ChunkRange struct {
	Start, End int `json:"start"`
}

func (r ChunkRange) Validate() error {
	if r.Start < 0 || r.End <= r.Start {
		return fmt.Errorf("invalid chunk source range [%d,%d)", r.Start, r.End)
	}
	return nil
}

type MemoryChunk struct {
	ID              string               `json:"id"`
	Scope           Scope                `json:"scope"`
	Source          ChunkSourceReference `json:"source"`
	Class           MemoryClass          `json:"class"`
	Ordinal         int                  `json:"ordinal"`
	Content         string               `json:"content"`
	SourceRange     ChunkRange           `json:"source_range"`
	CharacterCount  int                  `json:"character_count"`
	TokenCount      int                  `json:"token_count"`
	LifecycleState  MemoryState          `json:"lifecycle_state"`
	PolicyVersion   string               `json:"policy_version"`
	RendererVersion string               `json:"renderer_version"`
	CreatedAt       time.Time            `json:"created_at"`
}

// ChunkAdjacentOptions bounds parent-local evidence expansion. SessionID and
// UserID are optional assertions; a mismatch must fail closed.
type ChunkAdjacentOptions struct {
	SessionID     string
	UserID        string
	Before        int
	After         int
	MaxCharacters int
	MaxTokens     int
}

func (c MemoryChunk) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("chunk id is required")
	}
	if err := c.Scope.Validate(); err != nil {
		return fmt.Errorf("chunk scope: %w", err)
	}
	if c.Scope.Normalized() != c.Source.Scope.Normalized() {
		return fmt.Errorf("chunk source scope does not match chunk scope")
	}
	if err := c.Source.Validate(); err != nil {
		return err
	}
	if !validMemoryClass(c.Class) {
		return fmt.Errorf("invalid chunk memory class %q", c.Class)
	}
	if c.Ordinal < 0 {
		return fmt.Errorf("chunk ordinal cannot be negative")
	}
	if strings.TrimSpace(c.Content) == "" {
		return fmt.Errorf("chunk content is required")
	}
	if err := c.SourceRange.Validate(); err != nil {
		return err
	}
	if c.CharacterCount != len([]rune(c.Content)) || c.CharacterCount <= 0 {
		return fmt.Errorf("chunk character count mismatch")
	}
	if c.TokenCount != countChunkTokens(c.Content) || c.TokenCount <= 0 {
		return fmt.Errorf("chunk token count mismatch")
	}
	if c.LifecycleState != MemoryStateActive {
		return fmt.Errorf("chunk lifecycle state %q is not visible", c.LifecycleState)
	}
	if strings.TrimSpace(c.PolicyVersion) == "" || strings.TrimSpace(c.RendererVersion) == "" {
		return fmt.Errorf("chunk policy and renderer versions are required")
	}
	return nil
}

func countChunkTokens(s string) int { return len(strings.Fields(s)) }

func chunkIdentity(source ChunkSourceReference, policy, renderer string, ordinal int, r ChunkRange, content string) string {
	v := fmt.Sprintf("%s|%s|%d|%s|%s|%d|%d|%s", source.Kind, source.ID, source.Version, policy, renderer, ordinal, r.Start, content)
	d := sha256.Sum256([]byte(v))
	return "chunk-" + hex.EncodeToString(d[:])
}
