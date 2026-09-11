package retrieval

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

// DiversityPolicy identifies a bounded, replayable selection policy. It is
// deliberately independent of persistence so callers can apply it only after
// rollout and visibility validation.
type DiversityPolicy struct {
	Name                     string
	Version                  string
	MMRLambda                float64
	SemanticThreshold        float64
	MaxCandidates            int
	MaxPairwiseComparisons   int
	MaxEmbeddingDimensions   int
	MaxCitationsPerCandidate int
	CoverageWeights          DiversityCoverageWeights
}

// DiversityCoverageWeights are bounded bonuses for selecting evidence from a
// category not yet represented in the result. Unknown is an explicit local
// category; selection never performs an extra read to fill missing metadata.
type DiversityCoverageWeights struct {
	MemoryClass float64
	Session     float64
	Entity      float64
	TimeSlice   float64
	Unknown     float64
}

type DiversityDisposition string

const (
	DiversityDispositionSelected            DiversityDisposition = "selected"
	DiversityDispositionDuplicate           DiversityDisposition = "duplicate"
	DiversityDispositionOmittedByDiversity  DiversityDisposition = "omitted_by_diversity"
	DiversityDispositionInvalid             DiversityDisposition = "invalid"
	DiversityDispositionSemanticUnavailable DiversityDisposition = "semantic_unavailable"
)

const defaultMaxCitationsPerCandidate = 16

func DefaultDiversityPolicy() DiversityPolicy {
	return DiversityPolicy{
		Name: "mmr", Version: "mmr-v1", MMRLambda: 0.5, SemanticThreshold: 0.90,
		MaxCandidates: 200, MaxPairwiseComparisons: 5000, MaxEmbeddingDimensions: 4096, MaxCitationsPerCandidate: defaultMaxCitationsPerCandidate,
		CoverageWeights: DiversityCoverageWeights{MemoryClass: 0.05, Session: 0.05, Entity: 0.05, TimeSlice: 0.05, Unknown: 0},
	}
}

func (p DiversityPolicy) Validate() error {
	if strings.TrimSpace(p.Name) != "mmr" {
		return fmt.Errorf("unsupported diversity policy %q", p.Name)
	}
	if strings.TrimSpace(p.Version) == "" || len(p.Version) > 64 {
		return fmt.Errorf("diversity policy version is required and must not exceed 64 characters")
	}
	if math.IsNaN(p.MMRLambda) || math.IsInf(p.MMRLambda, 0) || p.MMRLambda < 0 || p.MMRLambda > 1 {
		return fmt.Errorf("MMR lambda must be between 0 and 1")
	}
	if math.IsNaN(p.SemanticThreshold) || math.IsInf(p.SemanticThreshold, 0) || p.SemanticThreshold < 0 || p.SemanticThreshold > 1 {
		return fmt.Errorf("semantic threshold must be between 0 and 1")
	}
	if p.MaxCandidates <= 0 || p.MaxCandidates > 5000 {
		return fmt.Errorf("diversity candidate limit must be between 1 and 5000")
	}
	if p.MaxPairwiseComparisons <= 0 || p.MaxPairwiseComparisons > 100000 {
		return fmt.Errorf("diversity pairwise comparison limit must be between 1 and 100000")
	}
	if p.MaxEmbeddingDimensions <= 0 || p.MaxEmbeddingDimensions > 4096 {
		return fmt.Errorf("diversity embedding dimension limit must be between 1 and 4096")
	}
	if p.MaxCitationsPerCandidate <= 0 || p.MaxCitationsPerCandidate > 64 {
		return fmt.Errorf("diversity citation limit must be between 1 and 64")
	}
	coverageWeights := []struct {
		name   string
		weight float64
	}{
		{name: "memory class", weight: p.CoverageWeights.MemoryClass},
		{name: "session", weight: p.CoverageWeights.Session},
		{name: "entity", weight: p.CoverageWeights.Entity},
		{name: "time slice", weight: p.CoverageWeights.TimeSlice},
		{name: "unknown", weight: p.CoverageWeights.Unknown},
	}
	for _, coverage := range coverageWeights {
		if math.IsNaN(coverage.weight) || math.IsInf(coverage.weight, 0) || coverage.weight < 0 || coverage.weight > 1 {
			return fmt.Errorf("%s coverage weight must be between 0 and 1", coverage.name)
		}
	}
	return nil
}

// DiversityCandidate contains only metadata already validated by the retrieval
// pipeline. It never triggers additional source reads during selection.
type DiversityCandidate struct {
	FusedCandidate
	SourceEventID           string
	ParentMemoryID          string
	EmbeddingRevision       string
	EmbeddingRevisionActive bool
	Embedding               []float32
	SessionID               string
	EntityKey               string
	TimeSlice               string
	Citations               []Citation
}

// DiversityDispositions are aggregate-only counters suitable for authorized
// diagnostics and evaluation reports.
type DiversityDispositions struct {
	Duplicate           int
	Diversity           int
	Invalid             int
	SemanticUnavailable int
}

type DiversitySelection struct {
	Candidates          []DiversityCandidate
	Dispositions        DiversityDispositions
	PairwiseComparisons int
}

// DeduplicateDiversityCandidates keeps the first stable-fused representative
// for each connected canonical/source/parent lineage group. Invalid or foreign
// candidates are excluded before grouping and therefore cannot suppress valid
// evidence.
func DeduplicateDiversityCandidates(scope memory.Scope, candidates []DiversityCandidate) DiversitySelection {
	return deduplicateDiversityCandidates(scope, candidates, defaultMaxCitationsPerCandidate)
}

func deduplicateDiversityCandidates(scope memory.Scope, candidates []DiversityCandidate, maxCitations int) DiversitySelection {
	valid := make([]DiversityCandidate, 0, len(candidates))
	dispositions := DiversityDispositions{}
	for _, candidate := range candidates {
		if !validDiversityCandidate(scope, candidate) {
			dispositions.Invalid++
			continue
		}
		valid = append(valid, candidate)
	}
	if len(valid) == 0 {
		return DiversitySelection{Dispositions: dispositions}
	}

	parents := make([]int, len(valid))
	for i := range parents {
		parents[i] = i
	}
	find := func(index int) int { return index }
	find = func(index int) int {
		for parents[index] != index {
			parents[index] = parents[parents[index]]
			index = parents[index]
		}
		return index
	}
	union := func(left, right int) {
		left, right = find(left), find(right)
		if left != right {
			parents[right] = left
		}
	}
	firstByKey := make(map[string]int)
	for index, candidate := range valid {
		for _, key := range diversityIdentityKeys(candidate) {
			if first, exists := firstByKey[key]; exists {
				union(first, index)
			} else {
				firstByKey[key] = index
			}
		}
	}
	representative := make(map[int]int)
	for index := range valid {
		root := find(index)
		if current, exists := representative[root]; !exists || index < current {
			representative[root] = index
		}
	}
	dispositions.Duplicate += len(valid) - len(representative)
	for index, candidate := range valid {
		root := find(index)
		representativeIndex := representative[root]
		if index == representativeIndex {
			continue
		}
		valid[representativeIndex].Citations = mergeDiversityCitations(valid[representativeIndex].Citations, candidate.Citations, maxCitations)
	}
	result := make([]DiversityCandidate, 0, len(representative))
	for index := range valid {
		if representative[find(index)] == index {
			valid[index].Citations = mergeDiversityCitations(nil, valid[index].Citations, maxCitations)
			result = append(result, valid[index])
		}
	}
	return DiversitySelection{Candidates: result, Dispositions: dispositions}
}

func SelectDiverseCandidates(scope memory.Scope, policy DiversityPolicy, candidates []DiversityCandidate, limit int) (DiversitySelection, error) {
	if err := policy.Validate(); err != nil {
		return DiversitySelection{}, err
	}
	if err := scope.Validate(); err != nil {
		return DiversitySelection{}, fmt.Errorf("validate diversity scope: %w", err)
	}
	if limit < 0 {
		return DiversitySelection{}, fmt.Errorf("diversity selection limit must be greater than or equal to zero")
	}
	deduplicated := deduplicateDiversityCandidates(scope, candidates, policy.MaxCitationsPerCandidate)
	pool := deduplicated.Candidates
	if len(pool) > policy.MaxCandidates {
		pool = pool[:policy.MaxCandidates]
		deduplicated.Dispositions.Diversity += len(deduplicated.Candidates) - len(pool)
	}
	if limit == 0 {
		deduplicated.Dispositions.Diversity += len(pool)
		deduplicated.Candidates = nil
		return deduplicated, nil
	}
	if limit > len(pool) {
		limit = len(pool)
	}
	if len(pool) <= 1 {
		deduplicated.Candidates = append([]DiversityCandidate(nil), pool...)
		return deduplicated, nil
	}

	semantic, available := buildDiversitySemanticPool(policy, pool)
	if !available {
		deduplicated.Dispositions.SemanticUnavailable = 1
		deduplicated.Dispositions.Diversity += len(pool) - limit
		deduplicated.Candidates = append([]DiversityCandidate(nil), pool[:limit]...)
		return deduplicated, nil
	}
	deduplicated.PairwiseComparisons = semantic.comparisons
	deduplicated.Dispositions.Diversity += len(pool) - len(semantic.candidates)
	if limit > len(semantic.candidates) {
		limit = len(semantic.candidates)
	}

	selected := make([]DiversityCandidate, 0, limit)
	selectedOriginalIndexes := make([]int, 0, limit)
	remaining := append([]DiversityCandidate(nil), semantic.candidates...)
	remainingOriginalIndexes := append([]int(nil), semantic.originalIndexes...)
	coverage := newDiversityCoverageState()
	for len(selected) < limit && len(remaining) > 0 {
		best := 0
		bestScore := math.Inf(-1)
		for index, candidate := range remaining {
			maxSimilarity := 0.0
			for _, chosenOriginalIndex := range selectedOriginalIndexes {
				similarity := semantic.similarity(remainingOriginalIndexes[index], chosenOriginalIndex)
				if similarity > maxSimilarity {
					maxSimilarity = similarity
				}
			}
			score := policy.MMRLambda*candidate.Score - (1-policy.MMRLambda)*maxSimilarity
			if len(selected) > 0 {
				score += coverage.bonus(policy.CoverageWeights, candidate)
			}
			if score > bestScore || (score == bestScore && diversityCandidateLess(candidate, remaining[best])) {
				best, bestScore = index, score
			}
		}
		selected = append(selected, remaining[best])
		selectedOriginalIndexes = append(selectedOriginalIndexes, remainingOriginalIndexes[best])
		coverage.add(remaining[best])
		remaining = append(remaining[:best], remaining[best+1:]...)
		remainingOriginalIndexes = append(remainingOriginalIndexes[:best], remainingOriginalIndexes[best+1:]...)
	}
	deduplicated.Dispositions.Diversity += len(remaining)
	deduplicated.Candidates = selected
	return deduplicated, nil
}

type diversitySemanticPool struct {
	candidates      []DiversityCandidate
	originalIndexes []int
	similarities    [][]float64
	comparisons     int
}

func buildDiversitySemanticPool(policy DiversityPolicy, candidates []DiversityCandidate) (diversitySemanticPool, bool) {
	count := len(candidates)
	requiredComparisons := int64(count) * int64(count-1) / 2
	if requiredComparisons > int64(policy.MaxPairwiseComparisons) {
		return diversitySemanticPool{}, false
	}
	revision := strings.TrimSpace(candidates[0].EmbeddingRevision)
	dimensions := len(candidates[0].Embedding)
	if dimensions > policy.MaxEmbeddingDimensions {
		return diversitySemanticPool{}, false
	}
	for _, candidate := range candidates {
		if !candidate.EmbeddingRevisionActive || revision == "" || strings.TrimSpace(candidate.EmbeddingRevision) != revision || len(candidate.Embedding) != dimensions || !validDiversityEmbedding(candidate.Embedding) {
			return diversitySemanticPool{}, false
		}
	}

	similarities := make([][]float64, count)
	parents := make([]int, count)
	for index := range parents {
		parents[index] = index
	}
	find := func(index int) int { return index }
	find = func(index int) int {
		for parents[index] != index {
			parents[index] = parents[parents[index]]
			index = parents[index]
		}
		return index
	}
	union := func(left, right int) {
		left, right = find(left), find(right)
		if left != right {
			parents[right] = left
		}
	}
	comparisons := 0
	for right := 1; right < count; right++ {
		similarities[right] = make([]float64, right)
		for left := 0; left < right; left++ {
			similarity, comparable := diversitySimilarity(candidates[left], candidates[right])
			if !comparable {
				return diversitySemanticPool{}, false
			}
			similarities[right][left] = similarity
			comparisons++
			if similarity >= policy.SemanticThreshold {
				union(left, right)
			}
		}
	}

	representatives := make(map[int]int, count)
	for index := range candidates {
		root := find(index)
		if current, exists := representatives[root]; !exists || index < current {
			representatives[root] = index
		}
	}
	clustered := append([]DiversityCandidate(nil), candidates...)
	for index := range candidates {
		representativeIndex := representatives[find(index)]
		if index != representativeIndex {
			clustered[representativeIndex].Citations = mergeDiversityCitations(clustered[representativeIndex].Citations, candidates[index].Citations, policy.MaxCitationsPerCandidate)
		}
	}
	result := diversitySemanticPool{similarities: similarities, comparisons: comparisons}
	for index := range clustered {
		if representatives[find(index)] != index {
			continue
		}
		clustered[index].Citations = mergeDiversityCitations(nil, clustered[index].Citations, policy.MaxCitationsPerCandidate)
		result.candidates = append(result.candidates, clustered[index])
		result.originalIndexes = append(result.originalIndexes, index)
	}
	return result, true
}

func (p diversitySemanticPool) similarity(left, right int) float64 {
	if left == right {
		return 1
	}
	if left < right {
		left, right = right, left
	}
	return p.similarities[left][right]
}

type diversityCoverageCategory struct {
	value   string
	unknown bool
}

type diversityCoverageState struct {
	memoryClasses map[diversityCoverageCategory]struct{}
	sessions      map[diversityCoverageCategory]struct{}
	entities      map[diversityCoverageCategory]struct{}
	timeSlices    map[diversityCoverageCategory]struct{}
}

func newDiversityCoverageState() diversityCoverageState {
	return diversityCoverageState{
		memoryClasses: make(map[diversityCoverageCategory]struct{}),
		sessions:      make(map[diversityCoverageCategory]struct{}),
		entities:      make(map[diversityCoverageCategory]struct{}),
		timeSlices:    make(map[diversityCoverageCategory]struct{}),
	}
}

func (s diversityCoverageState) add(candidate DiversityCandidate) {
	s.memoryClasses[diversityCoverageCategoryFor(string(candidate.Memory.Class))] = struct{}{}
	s.sessions[diversityCoverageCategoryFor(candidate.SessionID)] = struct{}{}
	s.entities[diversityCoverageCategoryFor(candidate.EntityKey)] = struct{}{}
	s.timeSlices[diversityCoverageCategoryFor(candidate.TimeSlice)] = struct{}{}
}

func (s diversityCoverageState) bonus(weights DiversityCoverageWeights, candidate DiversityCandidate) float64 {
	return diversityCoverageBonus(s.memoryClasses, diversityCoverageCategoryFor(string(candidate.Memory.Class)), weights.MemoryClass, weights.Unknown) +
		diversityCoverageBonus(s.sessions, diversityCoverageCategoryFor(candidate.SessionID), weights.Session, weights.Unknown) +
		diversityCoverageBonus(s.entities, diversityCoverageCategoryFor(candidate.EntityKey), weights.Entity, weights.Unknown) +
		diversityCoverageBonus(s.timeSlices, diversityCoverageCategoryFor(candidate.TimeSlice), weights.TimeSlice, weights.Unknown)
}

func diversityCoverageCategoryFor(value string) diversityCoverageCategory {
	value = strings.TrimSpace(value)
	return diversityCoverageCategory{value: value, unknown: value == ""}
}

func diversityCoverageBonus(seen map[diversityCoverageCategory]struct{}, category diversityCoverageCategory, knownWeight, unknownWeight float64) float64 {
	if _, exists := seen[category]; exists {
		return 0
	}
	if category.unknown {
		return unknownWeight
	}
	return knownWeight
}

func validDiversityEmbedding(embedding []float32) bool {
	if len(embedding) == 0 {
		return false
	}
	norm := 0.0
	for _, component := range embedding {
		value := float64(component)
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
		norm += value * value
	}
	return norm > 0 && !math.IsInf(norm, 0)
}

func mergeDiversityCitations(left, right []Citation, limit int) []Citation {
	if limit <= 0 {
		return nil
	}
	result := make([]Citation, 0, min(limit, len(left)+len(right)))
	seen := make(map[Citation]struct{}, min(limit, len(left)+len(right)))
	for _, citations := range [][]Citation{left, right} {
		for _, citation := range citations {
			if _, exists := seen[citation]; exists {
				continue
			}
			seen[citation] = struct{}{}
			result = append(result, citation)
			if len(result) == limit {
				return result
			}
		}
	}
	return result
}

func validDiversityCandidate(scope memory.Scope, candidate DiversityCandidate) bool {
	return scope.Validate() == nil && strings.TrimSpace(candidate.Memory.ID) != "" && candidate.Memory.Scope.Normalized() == scope.Normalized() && candidate.Memory.State == memory.MemoryStateActive && !math.IsNaN(candidate.Score) && !math.IsInf(candidate.Score, 0)
}

func diversityIdentityKeys(candidate DiversityCandidate) []string {
	keys := []string{"memory:" + candidate.Memory.ID}
	if value := strings.TrimSpace(candidate.SourceEventID); value != "" {
		keys = append(keys, "source:"+value)
	}
	if value := strings.TrimSpace(candidate.ParentMemoryID); value != "" {
		keys = append(keys, "parent:"+value)
	}
	return keys
}

func sortDiversityCandidates(candidates []DiversityCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool { return diversityCandidateLess(candidates[i], candidates[j]) })
}

func diversityCandidateLess(left, right DiversityCandidate) bool {
	if left.Score != right.Score {
		return left.Score > right.Score
	}
	leftClass, rightClass := fusionClassPriority(left.Memory.Class), fusionClassPriority(right.Memory.Class)
	if leftClass != rightClass {
		return leftClass < rightClass
	}
	if !left.Memory.ModifiedAt.Equal(right.Memory.ModifiedAt) {
		return left.Memory.ModifiedAt.After(right.Memory.ModifiedAt)
	}
	return left.Memory.ID < right.Memory.ID
}

func diversitySimilarity(left, right DiversityCandidate) (float64, bool) {
	if !left.EmbeddingRevisionActive || !right.EmbeddingRevisionActive || strings.TrimSpace(left.EmbeddingRevision) == "" || strings.TrimSpace(left.EmbeddingRevision) != strings.TrimSpace(right.EmbeddingRevision) || len(left.Embedding) != len(right.Embedding) || !validDiversityEmbedding(left.Embedding) || !validDiversityEmbedding(right.Embedding) {
		return 0, false
	}
	dot, leftNorm, rightNorm := 0.0, 0.0, 0.0
	for index := range left.Embedding {
		leftValue, rightValue := float64(left.Embedding[index]), float64(right.Embedding[index])
		dot += leftValue * rightValue
		leftNorm += leftValue * leftValue
		rightNorm += rightValue * rightValue
	}
	similarity := dot / math.Sqrt(leftNorm*rightNorm)
	if math.IsNaN(similarity) || math.IsInf(similarity, 0) {
		return 0, false
	}
	if similarity > 1 {
		similarity = 1
	} else if similarity < -1 {
		similarity = -1
	}
	return similarity, true
}
