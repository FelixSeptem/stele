package benchmark

import (
	"errors"
	"os"
	"sort"
	"strings"
)

type StressDataset string

const (
	StressDatasetNeedle      StressDataset = "needle-haystack"
	StressDatasetMRCR        StressDataset = "openai-mrcr"
	StressDatasetLongBenchV2 StressDataset = "longbench-v2"
	StressDatasetVTCBench    StressDataset = "vtcbench"
)

type StressConfig struct {
	ContextLength    int    `json:"context_length"`
	MaxContextLength int    `json:"max_context_length"`
	SampleCount      int    `json:"sample_count,omitempty"`
	MaxSamples       int    `json:"max_samples,omitempty"`
	NeedleCount      int    `json:"needle_count,omitempty"`
	MaxNeedleCount   int    `json:"max_needle_count,omitempty"`
	TimeoutMS        int    `json:"timeout_ms,omitempty"`
	MaxTimeoutMS     int    `json:"max_timeout_ms,omitempty"`
	Mode             string `json:"mode"`
	VisualAvailable  bool   `json:"visual_available,omitempty"`
}

// StressSample is a local, controlled sample. ContextLength is an explicit
// declared budget input rather than an inferred tokenizer-dependent value.
type StressSample struct {
	ID            string         `json:"id"`
	Context       string         `json:"context"`
	Query         string         `json:"query"`
	ContextLength int            `json:"context_length"`
	NeedleCount   int            `json:"needle_count,omitempty"`
	NeedleDepths  map[string]int `json:"needle_depths,omitempty"`
	TimeoutMS     int            `json:"timeout_ms"`
}

type StressSubset struct {
	Dataset StressDataset  `json:"dataset"`
	Config  StressConfig   `json:"config"`
	Samples []StressSample `json:"samples"`
}

type NeedlePlacement struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Depth int    `json:"depth"`
}

type NeedleSubsetRequest struct {
	Config   StressConfig      `json:"config"`
	ID       string            `json:"id"`
	Haystack string            `json:"haystack"`
	Query    string            `json:"query"`
	Needles  []NeedlePlacement `json:"needles"`
}

// LongBenchV2Metadata identifies a controlled LongBench-v2 subset without
// implying that its answer metric measures Stele memory-provider quality.
type LongBenchV2Metadata struct {
	Subset       string `json:"subset"`
	Language     string `json:"language"`
	TaskType     string `json:"task_type"`
	AnswerMetric string `json:"answer_metric"`
}

type LongBenchV2Subset struct {
	Metadata LongBenchV2Metadata `json:"metadata"`
	Config   StressConfig        `json:"config"`
}

type LongBenchV2Plan struct {
	Dataset                       StressDataset       `json:"dataset"`
	Metadata                      LongBenchV2Metadata `json:"metadata"`
	Config                        StressConfig        `json:"config"`
	Status                        Status              `json:"status"`
	NonGating                     bool                `json:"non_gating"`
	AnswerAccuracyIsMemoryQuality bool                `json:"answer_accuracy_is_memory_quality"`
}

type VTCBenchSubset struct {
	Config         StressConfig `json:"config"`
	TextArtifact   string       `json:"text_artifact,omitempty"`
	VisualArtifact string       `json:"visual_artifact,omitempty"`
}

type VTCBenchLoadedSubset struct {
	Dataset  StressDataset `json:"dataset"`
	Mode     string        `json:"mode"`
	Artifact string        `json:"artifact"`
}

type StressBucketOutcome struct {
	ContextLength int     `json:"context_length"`
	NeedleCount   int     `json:"needle_count,omitempty"`
	NeedleDepth   int     `json:"needle_depth,omitempty"`
	Mode          string  `json:"mode"`
	LatencyMS     float64 `json:"latency_ms"`
	Status        Status  `json:"status"`
}

// StressReport is intentionally separate from retrieval and memory quality
// reports. Its buckets describe degradation and host capacity only.
type StressReport struct {
	Family           DatasetFamily         `json:"family"`
	NonGating        bool                  `json:"non_gating"`
	Buckets          []StressBucketOutcome `json:"buckets"`
	CapacityFailures int                   `json:"capacity_failures"`
}

// AdmitStress rejects locally unsafe inputs before corpus import. Visual mode
// is never silently downgraded to text mode.
func AdmitStress(config StressConfig) Status {
	if config.ContextLength <= 0 || config.MaxContextLength <= 0 || config.SampleCount < 0 || config.MaxSamples < 0 || config.NeedleCount < 0 || config.MaxNeedleCount < 0 || config.TimeoutMS < 0 || config.MaxTimeoutMS < 0 {
		return StatusInvalidManifest
	}
	if config.ContextLength > config.MaxContextLength || (config.MaxSamples > 0 && config.SampleCount > config.MaxSamples) || (config.MaxNeedleCount > 0 && config.NeedleCount > config.MaxNeedleCount) || (config.MaxTimeoutMS > 0 && config.TimeoutMS > config.MaxTimeoutMS) {
		return StatusCapacityRefused
	}
	if config.Mode != "text" && config.Mode != "visual" {
		return StatusInvalidManifest
	}
	if config.Mode == "visual" && !config.VisualAvailable {
		return StatusPrerequisiteMissing
	}
	return StatusSuccess
}

// GenerateNeedleSubset deterministically places each needle by depth within a
// caller-provided local haystack. It produces a single controlled sample and
// carries all admission inputs into the resulting artifact.
func GenerateNeedleSubset(request NeedleSubsetRequest) (StressSubset, error) {
	if strings.TrimSpace(request.ID) == "" || strings.TrimSpace(request.Haystack) == "" || strings.TrimSpace(request.Query) == "" {
		return StressSubset{}, &StatusError{Status: StatusInvalidManifest, Message: "needle subset id, haystack, and query are required"}
	}
	if request.Config.SampleCount != 1 || request.Config.NeedleCount != len(request.Needles) || request.Config.TimeoutMS <= 0 || request.Config.MaxTimeoutMS <= 0 {
		return StressSubset{}, &StatusError{Status: StatusInvalidManifest, Message: "needle subset must declare one sample, matching needles, and timeout budgets"}
	}
	if status := AdmitStress(request.Config); status != StatusSuccess {
		return StressSubset{}, &StatusError{Status: status, Message: "needle subset exceeds stress budget"}
	}
	placements := append([]NeedlePlacement(nil), request.Needles...)
	sort.SliceStable(placements, func(i, j int) bool {
		if placements[i].Depth == placements[j].Depth {
			return placements[i].ID < placements[j].ID
		}
		return placements[i].Depth < placements[j].Depth
	})
	depths := make(map[string]int, len(placements))
	for _, needle := range placements {
		if strings.TrimSpace(needle.ID) == "" || strings.TrimSpace(needle.Text) == "" || needle.Depth < 0 || needle.Depth > 100 {
			return StressSubset{}, &StatusError{Status: StatusInvalidManifest, Message: "needle id, text, and depth between zero and one hundred are required"}
		}
		if _, duplicate := depths[needle.ID]; duplicate {
			return StressSubset{}, &StatusError{Status: StatusInvalidManifest, Message: "needle ids must be unique"}
		}
		depths[needle.ID] = needle.Depth
	}
	context := placeNeedles(request.Haystack, placements)
	return StressSubset{Dataset: StressDatasetNeedle, Config: request.Config, Samples: []StressSample{{ID: request.ID, Context: context, Query: request.Query, ContextLength: request.Config.ContextLength, NeedleCount: len(placements), NeedleDepths: depths, TimeoutMS: request.Config.TimeoutMS}}}, nil
}

// LoadMRCRSubset validates a pre-cached, local MRCR subset before its samples
// can be passed to any runner. It performs no network access or fallback.
func LoadMRCRSubset(subset StressSubset) (StressSubset, error) {
	if subset.Dataset != StressDatasetMRCR {
		return StressSubset{}, &StatusError{Status: StatusInvalidManifest, Message: "MRCR loader requires an openai-mrcr subset"}
	}
	if status := AdmitStress(subset.Config); status != StatusSuccess {
		return StressSubset{}, &StatusError{Status: status, Message: "MRCR subset exceeds stress budget"}
	}
	if subset.Config.SampleCount != len(subset.Samples) || subset.Config.TimeoutMS <= 0 || subset.Config.MaxTimeoutMS <= 0 {
		return StressSubset{}, &StatusError{Status: StatusInvalidManifest, Message: "MRCR subset sample count and timeout budgets are required"}
	}
	for _, sample := range subset.Samples {
		if strings.TrimSpace(sample.ID) == "" || strings.TrimSpace(sample.Context) == "" || strings.TrimSpace(sample.Query) == "" || sample.ContextLength <= 0 || sample.TimeoutMS <= 0 {
			return StressSubset{}, &StatusError{Status: StatusInvalidManifest, Message: "MRCR sample id, context, query, context length, and timeout are required"}
		}
		if sample.ContextLength > subset.Config.ContextLength || sample.NeedleCount > subset.Config.MaxNeedleCount && subset.Config.MaxNeedleCount > 0 || sample.TimeoutMS > subset.Config.TimeoutMS {
			return StressSubset{}, &StatusError{Status: StatusCapacityRefused, Message: "MRCR sample exceeds declared stress budget"}
		}
	}
	return subset, nil
}

// PlanLongBenchV2 validates local capacity before any long-context subset is
// run. Its result is always non-gating: answer accuracy stays a task-level
// long-context observation rather than a memory-provider score.
func PlanLongBenchV2(subset LongBenchV2Subset) (LongBenchV2Plan, error) {
	plan := LongBenchV2Plan{Dataset: StressDatasetLongBenchV2, Metadata: subset.Metadata, Config: subset.Config, NonGating: true}
	if strings.TrimSpace(subset.Metadata.Subset) == "" || strings.TrimSpace(subset.Metadata.Language) == "" || strings.TrimSpace(subset.Metadata.TaskType) == "" || strings.TrimSpace(subset.Metadata.AnswerMetric) == "" {
		plan.Status = StatusInvalidManifest
		return plan, &StatusError{Status: plan.Status, Message: "LongBench-v2 subset, language, task type, and answer metric are required"}
	}
	if subset.Config.SampleCount <= 0 || subset.Config.TimeoutMS <= 0 || subset.Config.MaxTimeoutMS <= 0 {
		plan.Status = StatusInvalidManifest
		return plan, &StatusError{Status: plan.Status, Message: "LongBench-v2 sample and timeout budgets are required"}
	}
	plan.Status = AdmitStress(subset.Config)
	if plan.Status != StatusSuccess {
		return plan, &StatusError{Status: plan.Status, Message: "LongBench-v2 subset exceeds stress budget"}
	}
	return plan, nil
}

// LoadVTCBenchSubset selects exactly the requested local VTCBench mode.
// Visual mode is refused when either the local capability or its artifact is
// absent; it is never downgraded to text mode.
func LoadVTCBenchSubset(subset VTCBenchSubset) (VTCBenchLoadedSubset, error) {
	if status := AdmitStress(subset.Config); status != StatusSuccess {
		return VTCBenchLoadedSubset{}, &StatusError{Status: status, Message: "VTCBench subset exceeds stress prerequisites"}
	}
	if subset.Config.SampleCount <= 0 || subset.Config.TimeoutMS <= 0 || subset.Config.MaxTimeoutMS <= 0 {
		return VTCBenchLoadedSubset{}, &StatusError{Status: StatusInvalidManifest, Message: "VTCBench sample and timeout budgets are required"}
	}
	artifact := subset.TextArtifact
	if subset.Config.Mode == "visual" {
		artifact = subset.VisualArtifact
	}
	if strings.TrimSpace(artifact) == "" {
		return VTCBenchLoadedSubset{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "requested VTCBench mode artifact is missing"}
	}
	info, err := os.Stat(artifact)
	if errors.Is(err, os.ErrNotExist) {
		return VTCBenchLoadedSubset{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "requested VTCBench mode artifact is unavailable"}
	}
	if err != nil {
		return VTCBenchLoadedSubset{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "inspect requested VTCBench mode artifact", Cause: err}
	}
	if info.IsDir() {
		return VTCBenchLoadedSubset{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "requested VTCBench mode artifact must be a file"}
	}
	return VTCBenchLoadedSubset{Dataset: StressDatasetVTCBench, Mode: subset.Config.Mode, Artifact: artifact}, nil
}

// BuildStressReport keeps degradation dimensions visible instead of averaging
// them into a result that could be mistaken for memory-provider quality.
func BuildStressReport(outcomes []StressBucketOutcome) StressReport {
	buckets := append([]StressBucketOutcome(nil), outcomes...)
	sort.SliceStable(buckets, func(i, j int) bool {
		if buckets[i].ContextLength != buckets[j].ContextLength {
			return buckets[i].ContextLength < buckets[j].ContextLength
		}
		if buckets[i].NeedleDepth != buckets[j].NeedleDepth {
			return buckets[i].NeedleDepth < buckets[j].NeedleDepth
		}
		if buckets[i].NeedleCount != buckets[j].NeedleCount {
			return buckets[i].NeedleCount < buckets[j].NeedleCount
		}
		return buckets[i].Mode < buckets[j].Mode
	})
	report := StressReport{Family: FamilyStress, NonGating: true, Buckets: buckets}
	for _, bucket := range buckets {
		if bucket.Status == StatusCapacityRefused {
			report.CapacityFailures++
		}
	}
	return report
}

func placeNeedles(haystack string, needles []NeedlePlacement) string {
	var output strings.Builder
	last := 0
	for _, needle := range needles {
		position := len(haystack) * needle.Depth / 100
		if position < last {
			position = last
		}
		output.WriteString(haystack[last:position])
		output.WriteString("\n")
		output.WriteString(needle.Text)
		output.WriteString("\n")
		last = position
	}
	output.WriteString(haystack[last:])
	return output.String()
}
