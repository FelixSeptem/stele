package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

const (
	TaskSeriesSchemaVersion = "task-series-v1"
	TaskSeriesMetricVersion = "task-series-metrics-v1"

	maxTaskSeriesConditions         = 32
	maxTaskSeriesStagesPerCondition = 1024
	maxTaskSeriesIdentifierLength   = 128
	maxTaskSeriesEvidencePerStage   = 64
	maxTaskSeriesSourceBytes        = 64 * 1024 * 1024
)

type TaskSeriesVerdict string

const (
	TaskSeriesVerdictSucceeded    TaskSeriesVerdict = "succeeded"
	TaskSeriesVerdictFailed       TaskSeriesVerdict = "failed"
	TaskSeriesVerdictPartial      TaskSeriesVerdict = "partial"
	TaskSeriesVerdictInconclusive TaskSeriesVerdict = "inconclusive"
)

func (v TaskSeriesVerdict) Valid() bool {
	switch v {
	case TaskSeriesVerdictSucceeded, TaskSeriesVerdictFailed, TaskSeriesVerdictPartial, TaskSeriesVerdictInconclusive:
		return true
	default:
		return false
	}
}

type TaskSeriesMemoryLabel string

const (
	TaskSeriesMemoryLabelStale TaskSeriesMemoryLabel = "stale"
	TaskSeriesMemoryLabelNoisy TaskSeriesMemoryLabel = "noisy"
)

func (l TaskSeriesMemoryLabel) Valid() bool {
	return l == TaskSeriesMemoryLabelStale || l == TaskSeriesMemoryLabelNoisy
}

// TaskSeriesEvidence carries a bounded opaque fixture reference. Its value is
// not serialized into aggregate reports.
type TaskSeriesEvidence struct {
	Reference string                  `json:"reference"`
	Labels    []TaskSeriesMemoryLabel `json:"labels,omitempty"`
}

type TaskSeriesStage struct {
	ID         string               `json:"id"`
	QueryID    string               `json:"query_id"`
	Scope      memory.Scope         `json:"scope"`
	Verdict    TaskSeriesVerdict    `json:"verdict"`
	Superseded bool                 `json:"superseded,omitempty"`
	Evidence   []TaskSeriesEvidence `json:"evidence"`
}

type TaskSeriesCondition struct {
	ID       string            `json:"id"`
	Baseline bool              `json:"baseline,omitempty"`
	Stages   []TaskSeriesStage `json:"stages"`
}

// TaskSeriesManifest is a local, checksum-locked evaluation input. Its
// verdicts are externally supplied observations, never inferred by Stele.
type TaskSeriesManifest struct {
	SchemaVersion  string                `json:"schema_version"`
	MetricVersion  string                `json:"metric_version"`
	SourceChecksum string                `json:"source_checksum"`
	QRELChecksum   string                `json:"qrels_checksum"`
	Scope          memory.Scope          `json:"scope"`
	Conditions     []TaskSeriesCondition `json:"conditions"`
}

type TaskSeriesSourceBundle struct {
	Manifest         TaskSeriesManifest          `json:"manifest"`
	DatasetManifest  DatasetManifest             `json:"dataset_manifest,omitempty"`
	Corpora          map[string]NormalizedCorpus `json:"corpora"`
	SyntheticFixture bool                        `json:"synthetic_fixture"`
}

func LoadTaskSeriesSource(source []byte) (TaskSeriesSourceBundle, error) {
	return loadTaskSeriesSourceBytes(source)
}

func LoadTaskSeriesSourceReader(source io.Reader) (TaskSeriesSourceBundle, error) {
	if source == nil {
		return TaskSeriesSourceBundle{}, &StatusError{Status: StatusInvalidManifest, Message: "task-series source is required"}
	}
	data, err := io.ReadAll(io.LimitReader(source, maxTaskSeriesSourceBytes+1))
	if err != nil {
		return TaskSeriesSourceBundle{}, &StatusError{Status: StatusInvalidManifest, Message: "read task-series source", Cause: err}
	}
	return loadTaskSeriesSourceBytes(data)
}

func loadTaskSeriesSourceBytes(source []byte) (TaskSeriesSourceBundle, error) {
	if len(source) == 0 || len(source) > maxTaskSeriesSourceBytes {
		return TaskSeriesSourceBundle{}, &StatusError{Status: StatusInvalidManifest, Message: "task-series source is empty or exceeds the byte limit"}
	}
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.DisallowUnknownFields()
	var bundle TaskSeriesSourceBundle
	if err := decoder.Decode(&bundle); err != nil {
		return TaskSeriesSourceBundle{}, &StatusError{Status: StatusInvalidManifest, Message: "decode task-series source", Cause: err}
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return TaskSeriesSourceBundle{}, &StatusError{Status: StatusInvalidManifest, Message: "decode task-series source", Cause: err}
	}
	checksum, err := TaskSeriesSourceChecksum(bundle)
	if err != nil {
		return TaskSeriesSourceBundle{}, err
	}
	if !strings.EqualFold(checksum, bundle.Manifest.SourceChecksum) {
		return TaskSeriesSourceBundle{}, &StatusError{Status: StatusChecksumMismatch, Message: "task-series source checksum does not match its lock"}
	}
	if err := ValidateTaskSeriesCorpora(bundle.Manifest, bundle.Corpora); err != nil {
		return TaskSeriesSourceBundle{}, &StatusError{Status: StatusInvalidManifest, Message: "validate task-series source", Cause: err}
	}
	if !bundle.SyntheticFixture {
		if err := bundle.DatasetManifest.Validate(); err != nil || bundle.DatasetManifest.Support != SupportRunnable {
			return TaskSeriesSourceBundle{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "non-synthetic task-series source requires a runnable dataset manifest", Cause: err}
		}
		if !strings.EqualFold(bundle.DatasetManifest.QRELChecksum, bundle.Manifest.QRELChecksum) {
			return TaskSeriesSourceBundle{}, &StatusError{Status: StatusChecksumMismatch, Message: "non-synthetic task-series dataset manifest qrel checksum does not match the task-series lock"}
		}
	}
	return bundle, nil
}

// TaskSeriesSourceChecksum hashes canonical bundle JSON with its lock field
// cleared, avoiding a self-referential checksum while covering outcomes and
// normalized condition corpora.
func TaskSeriesSourceChecksum(bundle TaskSeriesSourceBundle) (string, error) {
	bundle.Manifest.SourceChecksum = ""
	encoded, err := json.Marshal(bundle)
	if err != nil {
		return "", fmt.Errorf("marshal task-series checksum input: %w", err)
	}
	return checksumBytes(encoded), nil
}

func TaskSeriesSourceLocks(bundle TaskSeriesSourceBundle) (qrelChecksum, sourceChecksum string, err error) {
	qrelChecksum, err = TaskSeriesQRELChecksum(bundle.Corpora)
	if err != nil {
		return "", "", err
	}
	bundle.Manifest.QRELChecksum = qrelChecksum
	bundle.Manifest.SourceChecksum = ""
	sourceChecksum, err = TaskSeriesSourceChecksum(bundle)
	return qrelChecksum, sourceChecksum, err
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("unexpected trailing JSON value")
	}
	return err
}

func TaskSeriesQRELChecksum(corpora map[string]NormalizedCorpus) (string, error) {
	type conditionQRELs struct {
		ConditionID string `json:"condition_id"`
		QRELs       []QREL `json:"qrels"`
	}
	keys := make([]string, 0, len(corpora))
	for key := range corpora {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	combined := make([]conditionQRELs, 0, len(keys))
	for _, key := range keys {
		combined = append(combined, conditionQRELs{ConditionID: key, QRELs: corpora[key].Canonical().QRELs})
	}
	encoded, err := json.Marshal(combined)
	if err != nil {
		return "", fmt.Errorf("marshal task-series qrels: %w", err)
	}
	return checksumBytes(encoded), nil
}

type TaskSeriesObservedEvidence struct {
	Reference          string
	ProvenanceVerified bool
	LifecycleVisible   bool
	ScopeEligible      bool
}

type TaskSeriesStageObservation struct {
	Evidence       []TaskSeriesObservedEvidence
	SafetyFailures int
}

type TaskSeriesObservations map[string]map[string]TaskSeriesStageObservation

func (m TaskSeriesManifest) Validate() error {
	if m.SchemaVersion != TaskSeriesSchemaVersion {
		return fmt.Errorf("unsupported task-series schema %q", m.SchemaVersion)
	}
	if m.MetricVersion != TaskSeriesMetricVersion {
		return fmt.Errorf("unsupported task-series metric version %q", m.MetricVersion)
	}
	if !sha256Pattern.MatchString(m.SourceChecksum) {
		return fmt.Errorf("source checksum must be a 64-character hexadecimal digest")
	}
	if !sha256Pattern.MatchString(m.QRELChecksum) {
		return fmt.Errorf("qrel checksum must be a 64-character hexadecimal digest")
	}
	if err := m.Scope.Validate(); err != nil {
		return fmt.Errorf("task-series scope: %w", err)
	}
	if len(m.Conditions) < 2 || len(m.Conditions) > maxTaskSeriesConditions {
		return fmt.Errorf("task-series requires 2 to %d conditions", maxTaskSeriesConditions)
	}

	conditionIDs := make(map[string]struct{}, len(m.Conditions))
	var requiredStages []string
	baselineCount := 0
	for conditionIndex, condition := range m.Conditions {
		if err := validateTaskSeriesIdentifier("condition", condition.ID); err != nil {
			return err
		}
		if _, exists := conditionIDs[condition.ID]; exists {
			return fmt.Errorf("duplicate task-series condition %q", condition.ID)
		}
		conditionIDs[condition.ID] = struct{}{}
		if condition.Baseline {
			baselineCount++
		}
		if len(condition.Stages) == 0 || len(condition.Stages) > maxTaskSeriesStagesPerCondition {
			return fmt.Errorf("condition %q must contain 1 to %d stages", condition.ID, maxTaskSeriesStagesPerCondition)
		}
		stageIDs := make([]string, 0, len(condition.Stages))
		seenStages := make(map[string]struct{}, len(condition.Stages))
		for _, stage := range condition.Stages {
			if err := validateTaskSeriesStage(m.Scope, stage); err != nil {
				return fmt.Errorf("condition %q: %w", condition.ID, err)
			}
			if _, exists := seenStages[stage.ID]; exists {
				return fmt.Errorf("condition %q has duplicate stage %q", condition.ID, stage.ID)
			}
			seenStages[stage.ID] = struct{}{}
			stageIDs = append(stageIDs, stage.ID)
		}
		if conditionIndex == 0 {
			requiredStages = stageIDs
			continue
		}
		if !sameTaskSeriesStageCoverage(requiredStages, stageIDs) {
			return fmt.Errorf("condition %q stage coverage does not match other conditions", condition.ID)
		}
	}
	if baselineCount != 1 {
		return fmt.Errorf("task-series requires exactly one baseline condition")
	}
	return nil
}

func validateTaskSeriesStage(scope memory.Scope, stage TaskSeriesStage) error {
	if err := validateTaskSeriesIdentifier("stage", stage.ID); err != nil {
		return err
	}
	if err := validateTaskSeriesIdentifier("query", stage.QueryID); err != nil {
		return err
	}
	if err := stage.Scope.Validate(); err != nil {
		return fmt.Errorf("stage %q scope: %w", stage.ID, err)
	}
	if stage.Scope.Normalized() != scope.Normalized() {
		return fmt.Errorf("stage %q scope mismatch", stage.ID)
	}
	if !stage.Verdict.Valid() {
		return fmt.Errorf("stage %q verdict %q is invalid", stage.ID, stage.Verdict)
	}
	if len(stage.Evidence) > maxTaskSeriesEvidencePerStage {
		return fmt.Errorf("stage %q must contain at most %d evidence entries", stage.ID, maxTaskSeriesEvidencePerStage)
	}
	seen := map[string]struct{}{}
	for _, evidence := range stage.Evidence {
		if err := validateTaskSeriesIdentifier("evidence reference", evidence.Reference); err != nil {
			return fmt.Errorf("stage %q: %w", stage.ID, err)
		}
		if _, exists := seen[evidence.Reference]; exists {
			return fmt.Errorf("stage %q has duplicate evidence reference", stage.ID)
		}
		seen[evidence.Reference] = struct{}{}
		if len(evidence.Labels) > 2 {
			return fmt.Errorf("stage %q has too many evidence labels", stage.ID)
		}
		labels := map[TaskSeriesMemoryLabel]struct{}{}
		for _, label := range evidence.Labels {
			if !label.Valid() {
				return fmt.Errorf("stage %q memory label %q is invalid", stage.ID, label)
			}
			if _, exists := labels[label]; exists {
				return fmt.Errorf("stage %q has duplicate memory label %q", stage.ID, label)
			}
			labels[label] = struct{}{}
		}
	}
	return nil
}

func validateTaskSeriesIdentifier(kind, value string) error {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxTaskSeriesIdentifierLength {
		return fmt.Errorf("%s must contain 1 to %d bytes", kind, maxTaskSeriesIdentifierLength)
	}
	return nil
}

func sameTaskSeriesStageCoverage(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// ValidateTaskSeriesCorpora binds each ordered task stage and expected source
// alias to one normalized exact-scope corpus for its declared condition.
func ValidateTaskSeriesCorpora(manifest TaskSeriesManifest, corpora map[string]NormalizedCorpus) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	if len(corpora) != len(manifest.Conditions) {
		return fmt.Errorf("task-series corpus condition coverage mismatch")
	}
	for _, condition := range manifest.Conditions {
		corpus, ok := corpora[condition.ID]
		if !ok {
			return fmt.Errorf("condition %q corpus is missing", condition.ID)
		}
		if err := corpus.Validate(); err != nil {
			return fmt.Errorf("condition %q corpus: %w", condition.ID, err)
		}
		if len(corpus.Queries) != len(condition.Stages) {
			return fmt.Errorf("condition %q query stage coverage mismatch", condition.ID)
		}
		queries := make(map[string]BenchmarkQuery, len(corpus.Queries))
		for _, query := range corpus.Queries {
			queries[query.ID] = query
		}
		events := make(map[string]MemoryEventRecord, len(corpus.Events))
		for _, event := range corpus.Events {
			events[event.ID] = event
		}
		qrels := make(map[string]map[string]struct{}, len(corpus.QRELs))
		for _, qrel := range corpus.QRELs {
			if qrels[qrel.QueryID] == nil {
				qrels[qrel.QueryID] = map[string]struct{}{}
			}
			qrels[qrel.QueryID][qrel.EvidenceID] = struct{}{}
		}
		for _, stage := range condition.Stages {
			query, exists := queries[stage.QueryID]
			if !exists {
				return fmt.Errorf("condition %q stage query mapping is missing", condition.ID)
			}
			if query.Scope.Normalized() != manifest.Scope.Normalized() || stage.Scope.Normalized() != manifest.Scope.Normalized() {
				return fmt.Errorf("condition %q stage scope mismatch", condition.ID)
			}
			for _, expected := range stage.Evidence {
				event, exists := events[expected.Reference]
				if !exists || event.Scope.Normalized() != manifest.Scope.Normalized() || event.ExpectedState != memory.MemoryStateActive {
					return fmt.Errorf("condition %q evidence mapping is missing or ineligible", condition.ID)
				}
				if _, mapped := qrels[stage.QueryID][expected.Reference]; !mapped {
					return fmt.Errorf("condition %q evidence mapping is not linked to its task query", condition.ID)
				}
			}
		}
	}
	qrelChecksum, err := TaskSeriesQRELChecksum(corpora)
	if err != nil {
		return err
	}
	if !strings.EqualFold(qrelChecksum, manifest.QRELChecksum) {
		return &StatusError{Status: StatusChecksumMismatch, Message: "task-series qrel checksum does not match its lock"}
	}
	return nil
}

type TaskSeriesSafetyStatus string

const (
	TaskSeriesSafetyPassed TaskSeriesSafetyStatus = "passed"
	TaskSeriesSafetyFailed TaskSeriesSafetyStatus = "failed"
)

type TaskSeriesConditionReport struct {
	ConditionIndex            int     `json:"condition_index"`
	Baseline                  bool    `json:"-"`
	ReferenceCondition        bool    `json:"reference_condition"`
	StageCount                int     `json:"stage_count"`
	EvaluatedStages           int     `json:"evaluated_stages"`
	InconclusiveStages        int     `json:"inconclusive_stages"`
	SupersededStages          int     `json:"superseded_stages"`
	EvidenceStages            int     `json:"evidence_stages"`
	Utility                   float64 `json:"utility"`
	ObservedUtilityDifference float64 `json:"observed_utility_difference"`
	StaleStages               int     `json:"stale_stages"`
	NoisyStages               int     `json:"noisy_stages"`
	ProvenanceCoverage        float64 `json:"provenance_coverage"`
	EvidenceCount             int     `json:"evidence_count"`
	VerifiedProvenanceCount   int     `json:"verified_provenance_count"`
	SafetyFailures            int     `json:"safety_failures"`
}

type TaskSeriesReport struct {
	SchemaVersion       string                      `json:"schema_version"`
	MetricVersion       string                      `json:"metric_version"`
	Dataset             string                      `json:"dataset,omitempty"`
	DatasetVersion      string                      `json:"dataset_version,omitempty"`
	ManifestChecksum    string                      `json:"manifest_checksum,omitempty"`
	SourceChecksum      string                      `json:"source_checksum"`
	NormalizedChecksum  string                      `json:"normalized_checksum,omitempty"`
	QRELChecksum        string                      `json:"qrels_checksum"`
	Strategy            StrategyProfile             `json:"strategy,omitempty"`
	Runtime             *PostgresRuntimeIdentity    `json:"runtime,omitempty"`
	SyntheticFixture    bool                        `json:"synthetic_fixture"`
	ComparisonSemantics string                      `json:"comparison_semantics"`
	Status              Status                      `json:"status"`
	SafetyStatus        TaskSeriesSafetyStatus      `json:"safety_status"`
	CleanupVerified     bool                        `json:"cleanup_verified"`
	Conditions          []TaskSeriesConditionReport `json:"conditions"`
}

func EvaluateTaskSeries(manifest TaskSeriesManifest, observations TaskSeriesObservations) (TaskSeriesReport, error) {
	if err := manifest.Validate(); err != nil {
		return TaskSeriesReport{}, &StatusError{Status: StatusInvalidManifest, Message: "validate task-series manifest", Cause: err}
	}
	if err := validateTaskSeriesObservations(manifest, observations); err != nil {
		return TaskSeriesReport{}, &StatusError{Status: StatusInvalidManifest, Message: "validate task-series observations", Cause: err}
	}
	report := TaskSeriesReport{
		SchemaVersion:       manifest.SchemaVersion,
		MetricVersion:       manifest.MetricVersion,
		SourceChecksum:      manifest.SourceChecksum,
		QRELChecksum:        manifest.QRELChecksum,
		ComparisonSemantics: "observational",
		SyntheticFixture:    true,
		Status:              StatusSuccess,
		SafetyStatus:        TaskSeriesSafetyPassed,
		Conditions:          make([]TaskSeriesConditionReport, 0, len(manifest.Conditions)),
	}
	baselineUtility := 0.0
	for conditionIndex, condition := range manifest.Conditions {
		conditionReport := TaskSeriesConditionReport{ConditionIndex: conditionIndex, Baseline: condition.Baseline, ReferenceCondition: condition.Baseline, StageCount: len(condition.Stages)}
		utilityTotal := 0.0
		for _, stage := range condition.Stages {
			if stage.Superseded {
				conditionReport.SupersededStages++
				continue
			}
			if stage.Verdict == TaskSeriesVerdictInconclusive {
				conditionReport.InconclusiveStages++
			} else {
				conditionReport.EvaluatedStages++
				utilityTotal += taskSeriesUtility(stage.Verdict)
			}
			stageObservation := observations[condition.ID][stage.ID]
			labels := taskSeriesEvidenceLabels(stage.Evidence)
			stageHasEvidence := false
			stageStale := false
			stageNoisy := false
			conditionReport.SafetyFailures += max(0, stageObservation.SafetyFailures)
			for _, evidence := range stageObservation.Evidence {
				stageHasEvidence = true
				conditionReport.EvidenceCount++
				if _, ok := labels[evidence.Reference][TaskSeriesMemoryLabelStale]; ok {
					stageStale = true
				}
				if _, ok := labels[evidence.Reference][TaskSeriesMemoryLabelNoisy]; ok {
					stageNoisy = true
				}
				if evidence.ProvenanceVerified && evidence.ScopeEligible && evidence.LifecycleVisible {
					conditionReport.VerifiedProvenanceCount++
				}
				if !evidence.ScopeEligible || !evidence.LifecycleVisible {
					conditionReport.SafetyFailures++
				}
			}
			if stageHasEvidence {
				conditionReport.EvidenceStages++
			}
			if stageStale {
				conditionReport.StaleStages++
			}
			if stageNoisy {
				conditionReport.NoisyStages++
			}
		}
		if conditionReport.EvaluatedStages > 0 {
			conditionReport.Utility = utilityTotal / float64(conditionReport.EvaluatedStages)
		}
		if conditionReport.EvidenceCount > 0 {
			conditionReport.ProvenanceCoverage = float64(conditionReport.VerifiedProvenanceCount) / float64(conditionReport.EvidenceCount)
		}
		if conditionReport.EvaluatedStages == 0 || conditionReport.InconclusiveStages > 0 || conditionReport.EvaluatedStages+conditionReport.InconclusiveStages+conditionReport.SupersededStages != conditionReport.StageCount {
			report.Status = StatusQualityGateFailed
		}
		if conditionReport.SafetyFailures > 0 {
			report.SafetyStatus = TaskSeriesSafetyFailed
			report.Status = StatusQualityGateFailed
		}
		if condition.Baseline {
			baselineUtility = conditionReport.Utility
		}
		report.Conditions = append(report.Conditions, conditionReport)
	}
	for index := range report.Conditions {
		if !report.Conditions[index].Baseline {
			report.Conditions[index].ObservedUtilityDifference = report.Conditions[index].Utility - baselineUtility
		}
	}
	return report, nil
}

func validateTaskSeriesObservations(manifest TaskSeriesManifest, observations TaskSeriesObservations) error {
	if len(observations) != len(manifest.Conditions) {
		return fmt.Errorf("task-series observation condition coverage mismatch")
	}
	for _, condition := range manifest.Conditions {
		stageObservations, ok := observations[condition.ID]
		if !ok || len(stageObservations) != len(condition.Stages) {
			return fmt.Errorf("task-series observation stage coverage mismatch")
		}
		for _, stage := range condition.Stages {
			observation, ok := stageObservations[stage.ID]
			if !ok || observation.SafetyFailures < 0 || observation.SafetyFailures > maxTaskSeriesEvidencePerStage {
				return fmt.Errorf("task-series observation is missing or out of bounds")
			}
			declared := make(map[string]struct{}, len(stage.Evidence))
			for _, evidence := range stage.Evidence {
				declared[evidence.Reference] = struct{}{}
			}
			seen := map[string]struct{}{}
			for _, evidence := range observation.Evidence {
				if _, ok := declared[evidence.Reference]; !ok {
					return fmt.Errorf("task-series contains undeclared observed evidence")
				}
				if _, ok := seen[evidence.Reference]; ok {
					return fmt.Errorf("task-series contains duplicate observed evidence")
				}
				seen[evidence.Reference] = struct{}{}
			}
		}
	}
	return nil
}

func MarshalTaskSeriesReport(report TaskSeriesReport) ([]byte, error) {
	if report.SchemaVersion != TaskSeriesSchemaVersion || report.MetricVersion != TaskSeriesMetricVersion {
		return nil, fmt.Errorf("unsupported task-series report version")
	}
	if !sha256Pattern.MatchString(report.SourceChecksum) || !sha256Pattern.MatchString(report.QRELChecksum) {
		return nil, fmt.Errorf("task-series report checksums are malformed")
	}
	if !report.SyntheticFixture && (strings.TrimSpace(report.Dataset) == "" || strings.TrimSpace(report.DatasetVersion) == "" || !sha256Pattern.MatchString(report.ManifestChecksum)) {
		return nil, fmt.Errorf("non-synthetic task-series report requires dataset provenance")
	}
	if !report.SyntheticFixture && !report.CleanupVerified {
		return nil, fmt.Errorf("task-series report requires verified cleanup")
	}
	if report.ComparisonSemantics != "observational" {
		return nil, fmt.Errorf("task-series comparison semantics must be observational")
	}
	if report.Status == "" || (report.SafetyStatus != TaskSeriesSafetyPassed && report.SafetyStatus != TaskSeriesSafetyFailed) {
		return nil, fmt.Errorf("task-series report status is invalid")
	}
	if len(report.Conditions) < 2 {
		return nil, fmt.Errorf("task-series report requires at least two conditions")
	}
	referenceCount := 0
	for index, condition := range report.Conditions {
		if condition.ConditionIndex != index || condition.StageCount < 1 || condition.EvaluatedStages < 0 || condition.InconclusiveStages < 0 || condition.SupersededStages < 0 {
			return nil, fmt.Errorf("task-series report condition ordering or counts are invalid")
		}
		if condition.ReferenceCondition {
			referenceCount++
		}
		if condition.ProvenanceCoverage < 0 || condition.ProvenanceCoverage > 1 || condition.Utility < 0 || condition.Utility > 1 {
			return nil, fmt.Errorf("task-series report metrics are out of range")
		}
	}
	if referenceCount != 1 {
		return nil, fmt.Errorf("task-series report requires exactly one reference condition")
	}
	return json.Marshal(report)
}

func taskSeriesUtility(verdict TaskSeriesVerdict) float64 {
	switch verdict {
	case TaskSeriesVerdictSucceeded:
		return 1
	case TaskSeriesVerdictPartial:
		return 0.5
	default:
		return 0
	}
}

func taskSeriesEvidenceLabels(evidence []TaskSeriesEvidence) map[string]map[TaskSeriesMemoryLabel]struct{} {
	labels := make(map[string]map[TaskSeriesMemoryLabel]struct{}, len(evidence))
	for _, item := range evidence {
		labels[item.Reference] = make(map[TaskSeriesMemoryLabel]struct{}, len(item.Labels))
		for _, label := range item.Labels {
			labels[item.Reference][label] = struct{}{}
		}
	}
	return labels
}
