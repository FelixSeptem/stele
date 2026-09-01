package benchmark

import (
	"testing"

	"github.com/FelixSeptem/stele/internal/retrieval"
)

func TestCompareStrategyReportsRequiresAlignedProvenance(t *testing.T) {
	baseline := StrategyReport{Dataset: "fixture", Version: "v1", NormalizedChecksum: "norm-1", QRELChecksum: "qrels-1", Profile: StrategyProfileLexical, Report: validStrategyEvaluationReport("lexical-v1")}
	candidate := StrategyReport{Dataset: "fixture", Version: "v1", NormalizedChecksum: "norm-1", QRELChecksum: "qrels-1", Profile: StrategyProfileHybridRank, Report: validStrategyEvaluationReport("hybrid-rank-v1")}
	comparison, err := CompareStrategyReports(baseline, candidate, nil)
	if err != nil {
		t.Fatal(err)
	}
	if comparison.BaselineRankingVersion != "lexical-v1" || comparison.CandidateRankingVersion != "hybrid-rank-v1" {
		t.Fatalf("unexpected comparison: %#v", comparison)
	}
	candidate.NormalizedChecksum = "norm-2"
	if _, err := CompareStrategyReports(baseline, candidate, nil); err == nil {
		t.Fatal("expected normalized corpus mismatch")
	}
}

func TestStrategyProfileValidationCoversExpansionProfiles(t *testing.T) {
	for _, profile := range []StrategyProfile{StrategyProfileLexical, StrategyProfileSemantic, StrategyProfileHybrid, StrategyProfileChunk, StrategyProfileHybridRank} {
		if err := profile.Validate(); err != nil {
			t.Fatalf("profile %q rejected: %v", profile, err)
		}
	}
	if err := StrategyProfile("unknown").Validate(); err == nil {
		t.Fatal("expected unknown strategy profile to be rejected")
	}
}

func validStrategyEvaluationReport(ranking string) retrieval.EvaluationReport {
	return retrieval.EvaluationReport{Metadata: retrieval.EvaluationRankingMetadata{FixtureVersion: "fixture-v1", RepresentationVersion: "normalized-v1", RankingVersion: ranking, CompatibleEmbeddingRevision: "embedding-v1", PolicyVersion: "policy-v1"}}
}
