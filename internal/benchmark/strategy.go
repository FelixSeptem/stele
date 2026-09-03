package benchmark

import (
	"fmt"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

type StrategyProfile string

const (
	StrategyProfileLexical    StrategyProfile = "lexical"
	StrategyProfileSemantic   StrategyProfile = "semantic"
	StrategyProfileHybrid     StrategyProfile = "hybrid"
	StrategyProfileChunk      StrategyProfile = "chunk"
	StrategyProfileHybridRank StrategyProfile = "hybrid-rank"
)

func (p StrategyProfile) Validate() error {
	switch p {
	case StrategyProfileLexical, StrategyProfileSemantic, StrategyProfileHybrid, StrategyProfileChunk, StrategyProfileHybridRank:
		return nil
	default:
		return fmt.Errorf("unsupported benchmark strategy profile %q", p)
	}
}

type StrategyReport struct {
	Family                BenchmarkFamily            `json:"family"`
	Dataset               string                     `json:"dataset"`
	Version               string                     `json:"version"`
	NormalizedChecksum    string                     `json:"normalized_checksum"`
	QRELChecksum          string                     `json:"qrels_checksum"`
	Profile               StrategyProfile            `json:"profile"`
	EmbeddingIdentity     string                     `json:"embedding_identity,omitempty"`
	NormalizationIdentity string                     `json:"normalization_identity,omitempty"`
	Scope                 memory.Scope               `json:"scope"`
	Report                retrieval.EvaluationReport `json:"report"`
}

func CompareStrategyReports(baseline, candidate StrategyReport, protectedCategories []string) (retrieval.EvaluationComparison, error) {
	if baseline.Family != "" && candidate.Family != "" && baseline.Family != candidate.Family {
		return retrieval.EvaluationComparison{}, fmt.Errorf("incompatible benchmark families")
	}
	if strings.TrimSpace(baseline.Dataset) == "" || strings.TrimSpace(candidate.Dataset) == "" || baseline.Dataset != candidate.Dataset {
		return retrieval.EvaluationComparison{}, fmt.Errorf("incompatible benchmark dataset")
	}
	if strings.TrimSpace(baseline.Version) == "" || baseline.Version != candidate.Version {
		return retrieval.EvaluationComparison{}, fmt.Errorf("incompatible benchmark dataset version")
	}
	if strings.TrimSpace(baseline.NormalizedChecksum) == "" || baseline.NormalizedChecksum != candidate.NormalizedChecksum {
		return retrieval.EvaluationComparison{}, fmt.Errorf("incompatible normalized corpus checksum")
	}
	if strings.TrimSpace(baseline.QRELChecksum) == "" || baseline.QRELChecksum != candidate.QRELChecksum {
		return retrieval.EvaluationComparison{}, fmt.Errorf("incompatible qrels checksum")
	}
	if baseline.EmbeddingIdentity != candidate.EmbeddingIdentity {
		return retrieval.EvaluationComparison{}, fmt.Errorf("incompatible embedding identity")
	}
	if baseline.NormalizationIdentity != candidate.NormalizationIdentity {
		return retrieval.EvaluationComparison{}, fmt.Errorf("incompatible normalization identity")
	}
	if err := validateGenericBenchmarkScope(baseline.Scope); err != nil {
		return retrieval.EvaluationComparison{}, err
	}
	if err := validateGenericBenchmarkScope(candidate.Scope); err != nil {
		return retrieval.EvaluationComparison{}, err
	}
	if err := baseline.Profile.Validate(); err != nil {
		return retrieval.EvaluationComparison{}, err
	}
	if err := candidate.Profile.Validate(); err != nil {
		return retrieval.EvaluationComparison{}, err
	}
	return retrieval.CompareEvaluationReports(baseline.Report, candidate.Report, protectedCategories)
}

func validateGenericBenchmarkScope(scope memory.Scope) error {
	if scope == (memory.Scope{}) {
		return nil
	}
	if err := scope.Validate(); err != nil {
		return fmt.Errorf("invalid generic benchmark scope: %w", err)
	}
	if strings.EqualFold(scope.Project, "production") || strings.EqualFold(scope.Namespace, "production") {
		return fmt.Errorf("generic benchmark cannot access production scope")
	}
	if !strings.HasPrefix(strings.ToLower(scope.Project), "benchmark-") && !strings.HasPrefix(strings.ToLower(scope.Project), "bench-") {
		return fmt.Errorf("generic benchmark scope must use benchmark project")
	}
	return nil
}
