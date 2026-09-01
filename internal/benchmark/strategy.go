package benchmark

import (
	"fmt"
	"strings"

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
	Dataset            string                     `json:"dataset"`
	Version            string                     `json:"version"`
	NormalizedChecksum string                     `json:"normalized_checksum"`
	QRELChecksum       string                     `json:"qrels_checksum"`
	Profile            StrategyProfile            `json:"profile"`
	Report             retrieval.EvaluationReport `json:"report"`
}

func CompareStrategyReports(baseline, candidate StrategyReport, protectedCategories []string) (retrieval.EvaluationComparison, error) {
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
	if err := baseline.Profile.Validate(); err != nil {
		return retrieval.EvaluationComparison{}, err
	}
	if err := candidate.Profile.Validate(); err != nil {
		return retrieval.EvaluationComparison{}, err
	}
	return retrieval.CompareEvaluationReports(baseline.Report, candidate.Report, protectedCategories)
}
