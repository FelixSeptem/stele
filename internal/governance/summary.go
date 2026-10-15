package governance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FelixSeptem/stele/internal/memory"
)

type SummaryCluster struct {
	Scope      memory.Scope
	Candidates []CandidateMemory
}

type SummaryMemoryRecord struct {
	MemoryID            string
	VersionID           string
	Scope               memory.Scope
	MemoryPath          string
	Class               memory.MemoryClass
	Content             string
	EvidenceRawEventIDs []string
	CreatedAt           time.Time
}

func (r SummaryMemoryRecord) Validate() error {
	path, err := memory.NormalizeMemoryPath(r.MemoryPath)
	if err != nil {
		return fmt.Errorf("summary memory path: %w", err)
	}
	if path != r.MemoryPath {
		return fmt.Errorf("summary memory path must be normalized")
	}
	switch {
	case strings.TrimSpace(r.MemoryID) == "":
		return fmt.Errorf("summary memory id is required")
	case strings.TrimSpace(r.VersionID) == "":
		return fmt.Errorf("summary version id is required")
	case r.Scope.Validate() != nil:
		return r.Scope.Validate()
	case strings.TrimSpace(r.Content) == "":
		return fmt.Errorf("summary content is required")
	case len(r.EvidenceRawEventIDs) == 0:
		return fmt.Errorf("summary evidence is required")
	case r.CreatedAt.IsZero():
		return fmt.Errorf("summary created at is required")
	default:
		return nil
	}
}

type CompactionSource interface {
	ListCandidatesForCompaction(ctx context.Context, scope memory.Scope, cutoff time.Time, limit int) ([]CandidateMemory, error)
}

type SummaryRepository interface {
	CreateSummaryMemory(ctx context.Context, input SummaryMemoryRecord) (memory.CanonicalMemory, memory.MemoryVersion, error)
}

type Summarizer interface {
	Summarize(ctx context.Context, cluster SummaryCluster) (SummaryMemoryRecord, error)
}

type DeterministicSummarizer struct{}

func (DeterministicSummarizer) Summarize(ctx context.Context, cluster SummaryCluster) (SummaryMemoryRecord, error) {
	if err := cluster.Scope.Validate(); err != nil {
		return SummaryMemoryRecord{}, err
	}

	if len(cluster.Candidates) == 0 {
		return SummaryMemoryRecord{}, fmt.Errorf("summary cluster is empty")
	}
	path, err := memory.NormalizeMemoryPath(cluster.Candidates[0].MemoryPath)
	if err != nil {
		return SummaryMemoryRecord{}, fmt.Errorf("summary candidate path: %w", err)
	}

	parts := make([]string, 0, len(cluster.Candidates))
	evidence := make([]string, 0, len(cluster.Candidates))
	for _, candidate := range cluster.Candidates {
		if candidate.Scope != cluster.Scope {
			return SummaryMemoryRecord{}, fmt.Errorf("summary candidate scope does not match cluster scope")
		}
		candidatePath, pathErr := memory.NormalizeMemoryPath(candidate.MemoryPath)
		if pathErr != nil || candidatePath != path {
			return SummaryMemoryRecord{}, fmt.Errorf("summary candidates must share one normalized memory path")
		}
		parts = append(parts, strings.TrimSpace(candidate.Content))
		evidence = append(evidence, candidate.SourceRawEventID)
	}

	return SummaryMemoryRecord{
		Class:               memory.MemoryClassSummary,
		Scope:               cluster.Scope,
		MemoryPath:          path,
		Content:             "Summary: " + strings.Join(parts, " "),
		EvidenceRawEventIDs: evidence,
	}, nil
}

type SummaryProcessor struct {
	Source         CompactionSource
	Repository     SummaryRepository
	Summarizer     Summarizer
	Now            func() time.Time
	NewMemoryID    func() string
	NewVersionID   func() string
	MinClusterSize int
	ClusterLimit   int
}

func (p SummaryProcessor) CompactScope(ctx context.Context, scope memory.Scope, cutoff time.Time) error {
	if err := scope.Validate(); err != nil {
		return err
	}

	if p.Source == nil {
		return fmt.Errorf("compaction source is required")
	}

	if p.Repository == nil {
		return fmt.Errorf("summary repository is required")
	}

	if p.Summarizer == nil {
		return fmt.Errorf("summarizer is required")
	}

	if p.NewMemoryID == nil {
		return fmt.Errorf("summary memory id generator is required")
	}

	if p.NewVersionID == nil {
		return fmt.Errorf("summary version id generator is required")
	}

	limit := p.ClusterLimit
	if limit <= 0 {
		limit = 100
	}

	minSize := p.MinClusterSize
	if minSize <= 0 {
		minSize = 2
	}

	candidates, err := p.Source.ListCandidatesForCompaction(ctx, scope, cutoff, limit)
	if err != nil {
		return err
	}

	now := time.Now
	if p.Now != nil {
		now = p.Now
	}

	clusters := make(map[string][]CandidateMemory)
	order := make([]string, 0)
	for _, candidate := range candidates {
		path, pathErr := memory.NormalizeMemoryPath(candidate.MemoryPath)
		if pathErr != nil {
			return fmt.Errorf("candidate memory path: %w", pathErr)
		}
		if _, ok := clusters[path]; !ok {
			order = append(order, path)
		}
		clusters[path] = append(clusters[path], candidate)
	}
	for _, path := range order {
		cluster := clusters[path]
		if len(cluster) < minSize {
			continue
		}
		record, err := p.Summarizer.Summarize(ctx, SummaryCluster{Scope: scope, Candidates: cluster})
		if err != nil {
			return err
		}
		record.MemoryID, record.VersionID = p.NewMemoryID(), p.NewVersionID()
		record.Scope, record.MemoryPath, record.Class, record.CreatedAt = scope, path, memory.MemoryClassSummary, now().UTC()
		if err := record.Validate(); err != nil {
			return err
		}
		if _, _, err = p.Repository.CreateSummaryMemory(ctx, record); err != nil {
			return err
		}
	}
	return nil
}
