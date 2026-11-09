package benchmark

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
	"github.com/FelixSeptem/stele/internal/retrieval"
)

func TestTaskSeriesManifestValidateAcceptsCompleteCompatibleConditions(t *testing.T) {
	manifest := validTaskSeriesManifest()
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateTaskSeriesCorporaBindsStagesToExactScopeAndEvidence(t *testing.T) {
	manifest := validTaskSeriesManifest()
	corpora := validTaskSeriesCorpora(manifest)
	if err := ValidateTaskSeriesCorpora(manifest, corpora); err != nil {
		t.Fatalf("ValidateTaskSeriesCorpora() error = %v", err)
	}

	foreign := validTaskSeriesCorpora(manifest)
	foreign["memory"].Queries[0].Scope = memory.Scope{Tenant: "foreign", Project: "task-series", Namespace: "fixture"}
	if err := ValidateTaskSeriesCorpora(manifest, foreign); err == nil || !strings.Contains(err.Error(), "scope mismatch") {
		t.Fatalf("foreign corpus error = %v, want scope mismatch", err)
	}

	missing := validTaskSeriesCorpora(manifest)
	baseline := missing["baseline"]
	baseline.Events = baseline.Events[:1]
	missing["baseline"] = baseline
	if err := ValidateTaskSeriesCorpora(manifest, missing); err == nil || (!strings.Contains(err.Error(), "evidence mapping") && !strings.Contains(err.Error(), "unknown evidence")) {
		t.Fatalf("missing evidence error = %v, want evidence mapping failure", err)
	}
}

func TestLoadTaskSeriesSourceRequiresLockedOutcomesAndQRELs(t *testing.T) {
	manifest := validTaskSeriesManifest()
	bundle := TaskSeriesSourceBundle{Manifest: manifest, Corpora: validTaskSeriesCorpora(manifest), SyntheticFixture: true}
	checksum, err := TaskSeriesSourceChecksum(bundle)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Manifest.SourceChecksum = checksum
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTaskSeriesSource(encoded); err != nil {
		t.Fatalf("LoadTaskSeriesSource() error = %v", err)
	}

	bundle.Manifest.SourceChecksum = testChecksum
	encoded, err = json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTaskSeriesSource(encoded); StatusOf(err) != StatusChecksumMismatch {
		t.Fatalf("checksum mismatch status = %q, err = %v", StatusOf(err), err)
	}
}

func TestLoadTaskSeriesSourceRejectsUnprovenNonSyntheticBundle(t *testing.T) {
	manifest := validTaskSeriesManifest()
	bundle := TaskSeriesSourceBundle{Manifest: manifest, Corpora: validTaskSeriesCorpora(manifest), SyntheticFixture: false}
	_, sourceChecksum, err := TaskSeriesSourceLocks(bundle)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Manifest.SourceChecksum = sourceChecksum
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTaskSeriesSource(encoded); StatusOf(err) != StatusPrerequisiteMissing {
		t.Fatalf("non-synthetic manifest status = %q, err = %v", StatusOf(err), err)
	}
}

func TestMarshalTaskSeriesReportRejectsNonSyntheticWithoutDatasetProvenance(t *testing.T) {
	manifest := validTaskSeriesManifest()
	report, err := EvaluateTaskSeries(manifest, taskSeriesObservationsForManifest(manifest))
	if err != nil {
		t.Fatal(err)
	}
	report.SyntheticFixture = false
	if _, err := MarshalTaskSeriesReport(report); err == nil || !strings.Contains(err.Error(), "dataset provenance") {
		t.Fatalf("MarshalTaskSeriesReport() error = %v, want dataset provenance failure", err)
	}
}

func TestTaskSeriesObservationsFromReplayUsesReturnedCandidatesAndSafetyFindings(t *testing.T) {
	manifest := validTaskSeriesManifest()
	condition := manifest.Conditions[1]
	scope := memory.Scope{Tenant: "benchmark", Project: "condition", Namespace: "case"}
	replay := retrieval.EvaluationReplay{Cases: []retrieval.EvaluationReplayCase{{
		CaseID: "query-stage-01",
		Scope:  scope,
		Candidates: []retrieval.EvaluationReplayCandidate{{
			Alias: "evidence-stage-01", Scope: scope, State: memory.MemoryStateActive,
			Citations: []retrieval.Citation{{}},
		}},
	}}}
	retrievalReport := retrieval.EvaluationReport{Cases: []retrieval.EvaluationCaseReport{{
		CaseID:         "query-stage-01",
		SafetyFailures: []retrieval.EvaluationSafetyFailure{{Category: "scope_leak", Count: 1}},
	}}}

	observations, err := taskSeriesObservationsFromReplay(condition, replay, retrievalReport)
	if err != nil {
		t.Fatalf("taskSeriesObservationsFromReplay() error = %v", err)
	}
	stage := observations["stage-01"]
	if len(stage.Evidence) != 1 || stage.Evidence[0].Reference != "evidence-stage-01" || !stage.Evidence[0].ProvenanceVerified {
		t.Fatalf("stage evidence = %#v", stage.Evidence)
	}
	if stage.SafetyFailures != 1 {
		t.Fatalf("safety failures = %d, want 1", stage.SafetyFailures)
	}
}

func TestEvaluateTaskSeriesSeparatesUtilityHarmProvenanceAndSafety(t *testing.T) {
	manifest := validTaskSeriesManifest()
	manifest.Conditions[0].Stages[1].Verdict = TaskSeriesVerdictInconclusive
	manifest.Conditions[1].Stages[0].Verdict = TaskSeriesVerdictPartial
	manifest.Conditions[1].Stages[0].Evidence[0].Labels = []TaskSeriesMemoryLabel{TaskSeriesMemoryLabelNoisy}
	observations := taskSeriesObservationsForManifest(manifest)
	stageObservation := observations["memory"]["stage-01"]
	stageObservation.Evidence[0].ProvenanceVerified = false
	observations["memory"]["stage-01"] = stageObservation
	stageObservation = observations["memory"]["stage-02"]
	stageObservation.Evidence[0].LifecycleVisible = false
	observations["memory"]["stage-02"] = stageObservation

	report, err := EvaluateTaskSeries(manifest, observations)
	if err != nil {
		t.Fatalf("EvaluateTaskSeries() error = %v", err)
	}
	if report.SafetyStatus != TaskSeriesSafetyFailed || report.Status != StatusQualityGateFailed {
		t.Fatalf("safety/status = %q/%q, want failed/quality_gate_failed", report.SafetyStatus, report.Status)
	}
	baseline := report.Conditions[0]
	if baseline.EvaluatedStages != 1 || baseline.InconclusiveStages != 1 || baseline.Utility != 1 {
		t.Fatalf("baseline metrics = %#v", baseline)
	}
	memoryCondition := report.Conditions[1]
	if memoryCondition.Utility != 0.25 || memoryCondition.NoisyStages != 1 || memoryCondition.StaleStages != 1 {
		t.Fatalf("memory utility/harm = %#v", memoryCondition)
	}
	if memoryCondition.ProvenanceCoverage != 0 || memoryCondition.SafetyFailures != 1 {
		t.Fatalf("memory provenance/safety = %#v", memoryCondition)
	}
	if memoryCondition.ObservedUtilityDifference != -0.75 {
		t.Fatalf("observed utility difference = %v, want -0.75", memoryCondition.ObservedUtilityDifference)
	}
}

func TestEvaluateTaskSeriesCountsAtMostOneStaleOrNoisyHarmPerStage(t *testing.T) {
	manifest := validTaskSeriesManifest()
	manifest.Conditions[1].Stages[1].Evidence[0].Labels = nil
	manifest.Conditions[1].Stages[0].Evidence = append(manifest.Conditions[1].Stages[0].Evidence, TaskSeriesEvidence{Reference: "second-evidence", Labels: []TaskSeriesMemoryLabel{TaskSeriesMemoryLabelStale, TaskSeriesMemoryLabelNoisy}})
	observations := taskSeriesObservationsForManifest(manifest)

	report, err := EvaluateTaskSeries(manifest, observations)
	if err != nil {
		t.Fatalf("EvaluateTaskSeries() error = %v", err)
	}
	if got := report.Conditions[1].StaleStages; got != 1 {
		t.Fatalf("stale stages = %d, want 1", got)
	}
	if got := report.Conditions[1].NoisyStages; got != 1 {
		t.Fatalf("noisy stages = %d, want 1", got)
	}
}

func TestEvaluateTaskSeriesRejectsConditionWithoutCurrentOutcomes(t *testing.T) {
	manifest := validTaskSeriesManifest()
	for index := range manifest.Conditions[0].Stages {
		manifest.Conditions[0].Stages[index].Superseded = true
	}

	report, err := EvaluateTaskSeries(manifest, taskSeriesObservationsForManifest(manifest))
	if err != nil {
		t.Fatalf("EvaluateTaskSeries() error = %v", err)
	}
	if report.Status != StatusQualityGateFailed {
		t.Fatalf("status = %q, want quality_gate_failed", report.Status)
	}
}

func TestEvaluateTaskSeriesRejectsUndeclaredObservationEvidence(t *testing.T) {
	manifest := validTaskSeriesManifest()
	observations := taskSeriesObservationsForManifest(manifest)
	stageObservation := observations["memory"]["stage-01"]
	stageObservation.Evidence = append(stageObservation.Evidence, TaskSeriesObservedEvidence{Reference: "foreign-alias", ProvenanceVerified: true, LifecycleVisible: true, ScopeEligible: true})
	observations["memory"]["stage-01"] = stageObservation

	if _, err := EvaluateTaskSeries(manifest, observations); err == nil || !strings.Contains(err.Error(), "undeclared observed evidence") {
		t.Fatalf("EvaluateTaskSeries() error = %v, want undeclared observed evidence", err)
	}
}

func TestTaskSeriesReportIsStableAndRedacted(t *testing.T) {
	manifest := validTaskSeriesManifest()
	manifest.Scope = memory.Scope{Tenant: "secret-tenant", Project: "secret-project", Namespace: "secret-namespace"}
	for conditionIndex := range manifest.Conditions {
		for stageIndex := range manifest.Conditions[conditionIndex].Stages {
			stage := &manifest.Conditions[conditionIndex].Stages[stageIndex]
			stage.ID = "secret-task-query-" + stage.ID
			stage.Scope = manifest.Scope
			stage.Evidence[0].Reference = "secret-memory-id-" + stage.Evidence[0].Reference
		}
	}
	report, err := EvaluateTaskSeries(manifest, taskSeriesObservationsForManifest(manifest))
	if err != nil {
		t.Fatal(err)
	}
	first, err := MarshalTaskSeriesReport(report)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarshalTaskSeriesReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("report JSON is not stable:\n%s\n%s", first, second)
	}
	for _, forbidden := range []string{"secret-tenant", "secret-project", "secret-namespace", "secret-task-query", "secret-memory-id", "baseline", "memory"} {
		if bytes.Contains(first, []byte(forbidden)) {
			t.Fatalf("report leaked %q: %s", forbidden, first)
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if decoded["comparison_semantics"] != "observational" {
		t.Fatalf("comparison_semantics = %v, want observational", decoded["comparison_semantics"])
	}
}

func TestEvaluateTaskSeriesExcludesSupersededAndMissingOutcomes(t *testing.T) {
	manifest := validTaskSeriesManifest()
	manifest.Conditions[0].Stages[0].Superseded = true
	manifest.Conditions[1].Stages[1].Verdict = TaskSeriesVerdictInconclusive

	report, err := EvaluateTaskSeries(manifest, taskSeriesObservationsForManifest(manifest))
	if err != nil {
		t.Fatalf("EvaluateTaskSeries() error = %v", err)
	}
	if got := report.Conditions[0].EvaluatedStages; got != 1 {
		t.Fatalf("baseline evaluated stages = %d, want 1", got)
	}
	if got := report.Conditions[0].SupersededStages; got != 1 {
		t.Fatalf("baseline superseded stages = %d, want 1", got)
	}
	if got := report.Conditions[1].InconclusiveStages; got != 1 {
		t.Fatalf("memory inconclusive stages = %d, want 1", got)
	}
}

func TestTaskSeriesManifestValidateRejectsUnsafeOrIncompatibleInput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*TaskSeriesManifest)
		want   string
	}{
		{
			name: "condition misses stage",
			mutate: func(manifest *TaskSeriesManifest) {
				manifest.Conditions[1].Stages = manifest.Conditions[1].Stages[:1]
			},
			want: "stage coverage",
		},
		{
			name: "foreign stage scope",
			mutate: func(manifest *TaskSeriesManifest) {
				manifest.Conditions[1].Stages[1].Scope = memory.Scope{Tenant: "foreign", Project: "tasks", Namespace: "series"}
			},
			want: "scope mismatch",
		},
		{
			name: "unsupported verdict",
			mutate: func(manifest *TaskSeriesManifest) {
				manifest.Conditions[0].Stages[0].Verdict = TaskSeriesVerdict("guessed")
			},
			want: "verdict",
		},
		{
			name: "unknown harm label",
			mutate: func(manifest *TaskSeriesManifest) {
				manifest.Conditions[0].Stages[1].Evidence[0].Labels = []TaskSeriesMemoryLabel{"hallucinated"}
			},
			want: "memory label",
		},
		{
			name: "malformed checksum",
			mutate: func(manifest *TaskSeriesManifest) {
				manifest.SourceChecksum = "bad"
			},
			want: "source checksum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := validTaskSeriesManifest()
			tt.mutate(&manifest)
			err := manifest.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestTaskSeriesV2SeparatesMissingRepeatedErrorAndMaintenanceObservations(t *testing.T) {
	manifest := validTaskSeriesManifest()
	manifest.SchemaVersion = TaskSeriesSchemaVersionV2
	manifest.MetricVersion = TaskSeriesMetricVersionV2
	no := false
	yes := true
	baselineMaintenance := int64(3)
	correctedMaintenance := int64(5)
	manifest.Conditions[0].Stages[0].RepeatedError = &no
	manifest.Conditions[0].Stages[0].MaintenanceCost = &baselineMaintenance
	manifest.Conditions[1].Stages[0].RepeatedError = &yes
	manifest.Conditions[1].Stages[0].MaintenanceCost = &correctedMaintenance

	report, err := EvaluateTaskSeries(manifest, taskSeriesObservationsForManifest(manifest))
	if err != nil {
		t.Fatalf("EvaluateTaskSeries() error = %v", err)
	}
	baseline, corrected := report.Conditions[0], report.Conditions[1]
	if baseline.RepeatedErrorCoverage != 0.5 || baseline.RepeatedErrorRate == nil || *baseline.RepeatedErrorRate != 0 {
		t.Fatalf("baseline repeated-error metrics = %#v, want half coverage and zero observed rate", baseline)
	}
	if corrected.RepeatedErrorCoverage != 0.5 || corrected.RepeatedErrorRate == nil || *corrected.RepeatedErrorRate != 1 {
		t.Fatalf("corrected repeated-error metrics = %#v, want half coverage and one observed rate", corrected)
	}
	if baseline.MaintenanceCoverage != 0.5 || baseline.MaintenanceCostTotal != 3 || baseline.MaintenanceCostAverage == nil || *baseline.MaintenanceCostAverage != 3 {
		t.Fatalf("baseline maintenance metrics = %#v", baseline)
	}
	if corrected.MaintenanceCoverage != 0.5 || corrected.MaintenanceCostTotal != 5 || corrected.MaintenanceCostAverage == nil || *corrected.MaintenanceCostAverage != 5 {
		t.Fatalf("corrected maintenance metrics = %#v", corrected)
	}
	encoded, err := MarshalTaskSeriesReport(report)
	if err != nil {
		t.Fatalf("MarshalTaskSeriesReport() error = %v", err)
	}
	for _, forbidden := range []string{"stage-01", "evidence-stage-01", `"maintenance_cost":`} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Fatalf("v2 aggregate report leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestTaskSeriesV1RejectsV2ObservationFields(t *testing.T) {
	manifest := validTaskSeriesManifest()
	repeated := true
	manifest.Conditions[0].Stages[0].RepeatedError = &repeated
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "v2") {
		t.Fatalf("Validate() error = %v, want v2 compatibility refusal", err)
	}
}

func TestTaskSeriesV2LocksSourceAndRefusesUnknownFields(t *testing.T) {
	manifest := validTaskSeriesManifest()
	manifest.SchemaVersion = TaskSeriesSchemaVersionV2
	manifest.MetricVersion = TaskSeriesMetricVersionV2
	repeated := true
	manifest.Conditions[1].Stages[0].RepeatedError = &repeated
	bundle := TaskSeriesSourceBundle{Manifest: manifest, Corpora: validTaskSeriesCorpora(manifest), SyntheticFixture: true}
	qrelChecksum, sourceChecksum, err := TaskSeriesSourceLocks(bundle)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Manifest.QRELChecksum = qrelChecksum
	bundle.Manifest.SourceChecksum = sourceChecksum
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTaskSeriesSource(encoded); err != nil {
		t.Fatalf("LoadTaskSeriesSource(v2) error = %v", err)
	}

	tampered := bytes.Replace(encoded, []byte(`"repeated_error":true`), []byte(`"repeated_error":false`), 1)
	if _, err := LoadTaskSeriesSource(tampered); StatusOf(err) != StatusChecksumMismatch {
		t.Fatalf("tampered v2 source status = %q, err = %v", StatusOf(err), err)
	}
	unknown := bytes.Replace(encoded, []byte(`"synthetic_fixture"`), []byte(`"unknown_v2_field":true,"synthetic_fixture"`), 1)
	if _, err := LoadTaskSeriesSource(unknown); StatusOf(err) != StatusInvalidManifest {
		t.Fatalf("unknown v2 field status = %q, err = %v", StatusOf(err), err)
	}
}

func TestVerifiedMemoryCorrectionTaskSeries(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("testdata", "verified-memory-correction-task-series-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var unlocked TaskSeriesSourceBundle
	if err := json.Unmarshal(source, &unlocked); err != nil {
		t.Fatalf("decode synthetic fixture: %v", err)
	}
	qrelChecksum, sourceChecksum, err := TaskSeriesSourceLocks(unlocked)
	if err != nil {
		t.Fatal(err)
	}
	if unlocked.Manifest.QRELChecksum != qrelChecksum || unlocked.Manifest.SourceChecksum != sourceChecksum {
		t.Fatalf("fixture locks qrel=%s source=%s; want qrel=%s source=%s", unlocked.Manifest.QRELChecksum, unlocked.Manifest.SourceChecksum, qrelChecksum, sourceChecksum)
	}
	bundle, err := LoadTaskSeriesSource(source)
	if err != nil {
		t.Fatalf("LoadTaskSeriesSource() error = %v", err)
	}
	report, err := EvaluateTaskSeries(bundle.Manifest, taskSeriesObservationsForManifest(bundle.Manifest))
	if err != nil {
		t.Fatalf("EvaluateTaskSeries() error = %v", err)
	}
	if report.SchemaVersion != TaskSeriesSchemaVersionV2 || report.MetricVersion != TaskSeriesMetricVersionV2 || report.ComparisonSemantics != "observational" || len(report.Conditions) != 3 {
		t.Fatalf("report = %#v, want v2 observational three-condition output", report)
	}
	if report.Conditions[0].Utility != 0 || report.Conditions[1].ObservedUtilityDifference != 0.25 || report.Conditions[2].ObservedUtilityDifference != 0.5 {
		t.Fatalf("observational utility differences = %#v", report.Conditions)
	}
	if report.Conditions[0].RepeatedErrorRate == nil || *report.Conditions[0].RepeatedErrorRate != 1 || report.Conditions[2].RepeatedErrorRate == nil || *report.Conditions[2].RepeatedErrorRate != 0.5 || report.Conditions[2].MaintenanceCostAverage == nil || *report.Conditions[2].MaintenanceCostAverage != 2.5 {
		t.Fatalf("repeated-error/maintenance observations = %#v", report.Conditions)
	}
	encoded, err := MarshalTaskSeriesReport(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"verified-later-evidence", "no_memory", "source_retrieval", "verified_corrected", `"maintenance_cost":`} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Fatalf("observational aggregate report leaked %q: %s", forbidden, encoded)
		}
	}
}

func validTaskSeriesManifest() TaskSeriesManifest {
	scope := memory.Scope{Tenant: "benchmark", Project: "task-series", Namespace: "fixture"}
	stage := func(id string, verdict TaskSeriesVerdict, labels ...TaskSeriesMemoryLabel) TaskSeriesStage {
		return TaskSeriesStage{ID: id, QueryID: "query-" + id, Scope: scope, Verdict: verdict, Evidence: []TaskSeriesEvidence{{Reference: "evidence-" + id, Labels: labels}}}
	}
	manifest := TaskSeriesManifest{
		SchemaVersion:  TaskSeriesSchemaVersion,
		MetricVersion:  TaskSeriesMetricVersion,
		SourceChecksum: testChecksum,
		QRELChecksum:   testChecksum,
		Scope:          scope,
		Conditions: []TaskSeriesCondition{
			{ID: "baseline", Baseline: true, Stages: []TaskSeriesStage{stage("stage-01", TaskSeriesVerdictSucceeded), stage("stage-02", TaskSeriesVerdictPartial)}},
			{ID: "memory", Stages: []TaskSeriesStage{stage("stage-01", TaskSeriesVerdictSucceeded), stage("stage-02", TaskSeriesVerdictFailed, TaskSeriesMemoryLabelStale)}},
		},
	}
	qrelChecksum, err := TaskSeriesQRELChecksum(validTaskSeriesCorpora(manifest))
	if err != nil {
		panic(err)
	}
	manifest.QRELChecksum = qrelChecksum
	return manifest
}

func taskSeriesObservationsForManifest(manifest TaskSeriesManifest) TaskSeriesObservations {
	observations := TaskSeriesObservations{}
	for _, condition := range manifest.Conditions {
		stages := map[string]TaskSeriesStageObservation{}
		for _, stage := range condition.Stages {
			items := make([]TaskSeriesObservedEvidence, 0, len(stage.Evidence))
			for _, evidence := range stage.Evidence {
				items = append(items, TaskSeriesObservedEvidence{Reference: evidence.Reference, ProvenanceVerified: true, LifecycleVisible: true, ScopeEligible: true})
			}
			stages[stage.ID] = TaskSeriesStageObservation{Evidence: items}
		}
		observations[condition.ID] = stages
	}
	return observations
}

func validTaskSeriesCorpora(manifest TaskSeriesManifest) map[string]NormalizedCorpus {
	corpora := map[string]NormalizedCorpus{}
	for _, condition := range manifest.Conditions {
		corpus := NormalizedCorpus{SchemaVersion: SchemaVersion}
		for _, stage := range condition.Stages {
			corpus.Queries = append(corpus.Queries, BenchmarkQuery{ID: stage.QueryID, Scope: manifest.Scope, SessionID: stage.ID, Text: "private evidence query", EvidenceGroups: []EvidenceGroup{{ID: "required-" + stage.ID, EvidenceIDs: []string{stage.Evidence[0].Reference}, Required: true}}})
			corpus.Events = append(corpus.Events, MemoryEventRecord{ID: stage.Evidence[0].Reference, Scope: manifest.Scope, SessionID: stage.ID, Class: memory.MemoryClassEpisodic, Text: "private evidence", ExpectedState: memory.MemoryStateActive})
			corpus.QRELs = append(corpus.QRELs, QREL{QueryID: stage.QueryID, EvidenceID: stage.Evidence[0].Reference, Grade: 2})
		}
		corpora[condition.ID] = corpus
	}
	return corpora
}

const testChecksum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
