package memory

import (
	"fmt"
	"strings"
)

type ChunkOmissionReason string

const (
	ChunkOmissionReasonLifecycle ChunkOmissionReason = "lifecycle"
	ChunkOmissionReasonScope     ChunkOmissionReason = "scope"
	ChunkOmissionReasonClass     ChunkOmissionReason = "class"
	ChunkOmissionReasonPolicy    ChunkOmissionReason = "policy"
	ChunkOmissionReasonStale     ChunkOmissionReason = "stale"
)

type ChunkClassPolicy struct {
	MaxCharacters int  `json:"max_characters"`
	MaxTokens     int  `json:"max_tokens"`
	PreferAtomic  bool `json:"prefer_atomic"`
}

func (p ChunkClassPolicy) Validate() error {
	if p.MaxCharacters <= 0 {
		return fmt.Errorf("chunk max characters must be positive")
	}
	if p.MaxTokens <= 0 {
		return fmt.Errorf("chunk max tokens must be positive")
	}
	return nil
}

type ChunkPolicy struct {
	Version         string                           `json:"version"`
	RendererVersion string                           `json:"renderer_version"`
	CounterVersion  string                           `json:"counter_version"`
	Classes         map[MemoryClass]ChunkClassPolicy `json:"classes"`
}

func DefaultChunkPolicy(version, renderer string) ChunkPolicy {
	if strings.TrimSpace(version) == "" {
		version = "chunk-policy-v1"
	}
	if strings.TrimSpace(renderer) == "" {
		renderer = "chunk-renderer-v1"
	}
	return ChunkPolicy{Version: version, RendererVersion: renderer, CounterVersion: "whitespace-v1", Classes: map[MemoryClass]ChunkClassPolicy{
		MemoryClassProfile:    {MaxCharacters: 512, MaxTokens: 64, PreferAtomic: true},
		MemoryClassEpisodic:   {MaxCharacters: 1024, MaxTokens: 256},
		MemoryClassProcedural: {MaxCharacters: 1200, MaxTokens: 256},
		MemoryClassSummary:    {MaxCharacters: 2048, MaxTokens: 384},
		MemoryClassRelation:   {MaxCharacters: 512, MaxTokens: 64, PreferAtomic: true},
	}}
}

func (p ChunkPolicy) Validate() error {
	if strings.TrimSpace(p.Version) == "" || strings.TrimSpace(p.RendererVersion) == "" || strings.TrimSpace(p.CounterVersion) == "" {
		return fmt.Errorf("chunk policy, renderer, and counter versions are required")
	}
	for _, class := range []MemoryClass{MemoryClassProfile, MemoryClassEpisodic, MemoryClassProcedural, MemoryClassSummary, MemoryClassRelation} {
		cp, ok := p.Classes[class]
		if !ok {
			return fmt.Errorf("missing chunk policy for class %q", class)
		}
		if err := cp.Validate(); err != nil {
			return fmt.Errorf("class %q: %w", class, err)
		}
	}
	return nil
}

func (p ChunkPolicy) ClassPolicy(class MemoryClass) (ChunkClassPolicy, bool) {
	cp, ok := p.Classes[class]
	return cp, ok
}

type ChunkRolloutMode string

const (
	ChunkRolloutModeDefaultOff ChunkRolloutMode = "default_off"
	ChunkRolloutModeShadow     ChunkRolloutMode = "shadow"
	ChunkRolloutModeActive     ChunkRolloutMode = "active"
)

func (m ChunkRolloutMode) Valid() bool {
	return m == ChunkRolloutModeDefaultOff || m == ChunkRolloutModeShadow || m == ChunkRolloutModeActive
}
