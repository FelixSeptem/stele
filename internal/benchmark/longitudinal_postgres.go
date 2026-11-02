package benchmark

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

// RunTaskSeriesPostgres evaluates each declared condition in its own
// benchmark-owned scope using the existing PostgreSQL fixture seeder and
// retrieval replay path. Outcomes remain caller-supplied and observational.
func RunTaskSeriesPostgres(ctx context.Context, dsn string, manifest TaskSeriesManifest, corpora map[string]NormalizedCorpus, baseScope memory.Scope, datasetManifest DatasetManifest, syntheticFixture bool) (TaskSeriesReport, error) {
	if err := ValidateTaskSeriesCorpora(manifest, corpora); err != nil {
		return TaskSeriesReport{}, &StatusError{Status: StatusInvalidManifest, Message: "validate task-series inputs", Cause: err}
	}
	if !syntheticFixture {
		if err := datasetManifest.Validate(); err != nil || datasetManifest.Support != SupportRunnable {
			return TaskSeriesReport{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "non-synthetic task-series source requires a runnable dataset manifest", Cause: err}
		}
	}
	observations := TaskSeriesObservations{}
	var runtime PostgresRuntimeIdentity
	conditionCorpusChecksums := make([]string, 0, len(manifest.Conditions))
	for index, condition := range manifest.Conditions {
		corpus := corpora[condition.ID]
		corpusChecksum, err := corpus.Checksum()
		if err != nil {
			return TaskSeriesReport{}, err
		}
		conditionCorpusChecksums = append(conditionCorpusChecksums, corpusChecksum)
		runID := fmt.Sprintf("task-series-condition-%02d", index+1)
		run, err := NewRunScope(baseScope, "task-series", runID)
		if err != nil {
			return TaskSeriesReport{}, err
		}
		datasetManifest := taskSeriesDatasetManifest(manifest, len(corpus.Queries))
		result, replay, err := RunPostgresCorpusDetailed(ctx, dsn, datasetManifest, corpus, run, syntheticFixture)
		if err != nil {
			return TaskSeriesReport{}, err
		}
		if index == 0 {
			runtime = result.Runtime
		} else if runtime != result.Runtime {
			return TaskSeriesReport{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "task-series condition runtime identities differ"}
		}
		conditionObservations, err := taskSeriesObservationsFromReplay(condition, replay, result.RetrievalReport)
		if err != nil {
			return TaskSeriesReport{}, err
		}
		observations[condition.ID] = conditionObservations
	}
	report, err := EvaluateTaskSeries(manifest, observations)
	if err != nil {
		return TaskSeriesReport{}, err
	}
	checksumInput, err := json.Marshal(conditionCorpusChecksums)
	if err != nil {
		return TaskSeriesReport{}, fmt.Errorf("marshal normalized task-series checksums: %w", err)
	}
	report.NormalizedChecksum = checksumBytes(checksumInput)
	report.Strategy = StrategyProfileLexical
	report.Runtime = &runtime
	report.SyntheticFixture = syntheticFixture
	report.CleanupVerified = true
	if datasetManifest.Name != "" {
		encodedManifest, err := json.Marshal(datasetManifest)
		if err != nil {
			return TaskSeriesReport{}, fmt.Errorf("marshal task-series dataset manifest: %w", err)
		}
		report.Dataset = datasetManifest.Name
		report.DatasetVersion = datasetManifest.Version
		report.ManifestChecksum = checksumBytes(encodedManifest)
	}
	return report, nil
}

func taskSeriesDatasetManifest(manifest TaskSeriesManifest, queryCount int) DatasetManifest {
	return DatasetManifest{
		SchemaVersion:     SchemaVersion,
		Name:              "task-series",
		Family:            string(FamilyMemory),
		Version:           manifest.SchemaVersion,
		License:           "local-evaluation-source",
		UpstreamURL:       "https://local.invalid/task-series",
		UpstreamRevision:  manifest.SourceChecksum,
		SHA256:            manifest.SourceChecksum,
		SourcePath:        "local-task-series-source",
		ConversionVersion: manifest.MetricVersion,
		SplitIdentity:     "task-series",
		QRELChecksum:      manifest.QRELChecksum,
		Redistribution:    RedistributionRestricted,
		Support:           SupportRunnable,
		Splits:            map[string]SplitSpec{"task-series": {Source: "normalized-task-series", MaxQueries: queryCount}},
		Embedding:         EmbeddingProfile{Name: "lexical-only", Normalization: "none"},
	}
}

func taskSeriesObservationsFromReplay(condition TaskSeriesCondition, replay retrieval.EvaluationReplay, report retrieval.EvaluationReport) (map[string]TaskSeriesStageObservation, error) {
	queryToStage := make(map[string]TaskSeriesStage, len(condition.Stages))
	for _, stage := range condition.Stages {
		queryToStage[stage.QueryID] = stage
	}
	observations := make(map[string]TaskSeriesStageObservation, len(condition.Stages))
	for _, stage := range condition.Stages {
		observations[stage.ID] = TaskSeriesStageObservation{}
	}
	for _, replayCase := range replay.Cases {
		stage, ok := queryToStage[replayCase.CaseID]
		if !ok {
			return nil, fmt.Errorf("task-series replay returned an undeclared query")
		}
		stageObservation := observations[stage.ID]
		for _, candidate := range replayCase.Candidates {
			stageObservation.Evidence = append(stageObservation.Evidence, TaskSeriesObservedEvidence{
				Reference:          candidate.Alias,
				ProvenanceVerified: len(candidate.Citations) > 0,
				LifecycleVisible:   candidate.State == memory.MemoryStateActive,
				ScopeEligible:      candidate.Scope.Normalized() == replayCase.Scope.Normalized(),
			})
		}
		observations[stage.ID] = stageObservation
	}
	stageIDs := make(map[string]string, len(queryToStage))
	for _, stage := range condition.Stages {
		stageIDs[stage.QueryID] = stage.ID
	}
	for _, caseReport := range report.Cases {
		stageID, ok := stageIDs[caseReport.CaseID]
		if !ok {
			return nil, fmt.Errorf("task-series report returned an undeclared query")
		}
		stageObservation := observations[stageID]
		for _, failure := range caseReport.SafetyFailures {
			stageObservation.SafetyFailures += failure.Count
		}
		observations[stageID] = stageObservation
	}
	return observations, nil
}
