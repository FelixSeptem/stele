package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

type ContextCalibrationSignal struct {
	Scope      Scope
	SubjectKey string
	Weight     float64
	Confidence float64
	CreatedAt  time.Time
	Superseded bool
}

type ContextCalibrationBuildInput struct {
	Scope               Scope
	PolicyVersion       string
	SummaryVersion      string
	Now                 time.Time
	MinimumEvidence     int
	ConfidenceThreshold float64
	DecayWindow         time.Duration
	ContributionCap     float64
	Signals             []ContextCalibrationSignal
}

func BuildContextCalibrationSummary(input ContextCalibrationBuildInput) (ContextCalibrationSummary, error) {
	if err := input.Scope.Validate(); err != nil {
		return ContextCalibrationSummary{}, err
	}
	if input.PolicyVersion == "" || input.SummaryVersion == "" || input.Now.IsZero() || input.MinimumEvidence < 1 || input.DecayWindow <= 0 || input.ContributionCap <= 0 || input.ContributionCap > 1 || input.ConfidenceThreshold < 0 || input.ConfidenceThreshold > 1 {
		return ContextCalibrationSummary{}, fmt.Errorf("context calibration build limits are invalid")
	}
	type accepted struct {
		key          string
		created      time.Time
		contribution float64
	}
	acceptedSignals := make([]accepted, 0, len(input.Signals))
	for _, signal := range input.Signals {
		if signal.Scope.Normalized() != input.Scope.Normalized() || signal.Superseded || signal.CreatedAt.IsZero() || signal.CreatedAt.After(input.Now) || input.Now.Sub(signal.CreatedAt) > input.DecayWindow || signal.Confidence < input.ConfidenceThreshold || signal.SubjectKey == "" || math.IsNaN(signal.Weight) || math.IsInf(signal.Weight, 0) {
			continue
		}
		age := input.Now.Sub(signal.CreatedAt)
		decay := 1 - age.Seconds()/input.DecayWindow.Seconds()
		if decay < 0 {
			decay = 0
		}
		contribution := signal.Weight * signal.Confidence * decay
		if contribution > input.ContributionCap {
			contribution = input.ContributionCap
		}
		if contribution < -input.ContributionCap {
			contribution = -input.ContributionCap
		}
		acceptedSignals = append(acceptedSignals, accepted{key: signal.SubjectKey, created: signal.CreatedAt, contribution: contribution})
	}
	sort.Slice(acceptedSignals, func(i, j int) bool {
		if acceptedSignals[i].key == acceptedSignals[j].key {
			return acceptedSignals[i].created.Before(acceptedSignals[j].created)
		}
		return acceptedSignals[i].key < acceptedSignals[j].key
	})
	watermark := time.Time{}
	priority := 0.0
	for _, signal := range acceptedSignals {
		if signal.created.After(watermark) {
			watermark = signal.created
		}
		priority += signal.contribution
	}
	if watermark.IsZero() {
		watermark = input.Now
	}
	freshness := ContextCalibrationSummaryUnknown
	if len(acceptedSignals) >= input.MinimumEvidence {
		freshness = ContextCalibrationSummaryFresh
	}
	identity, _ := json.Marshal(struct {
		Scope           Scope
		Policy, Summary string
		Watermark       time.Time
		Count           int
	}{input.Scope.Normalized(), input.PolicyVersion, input.SummaryVersion, watermark, len(acceptedSignals)})
	hash := sha256.Sum256(identity)
	summary := ContextCalibrationSummary{ID: "calibration-" + hex.EncodeToString(hash[:]), Scope: input.Scope.Normalized(), PolicyVersion: input.PolicyVersion, SummaryVersion: input.SummaryVersion, SourceWatermark: watermark, EvidenceCount: len(acceptedSignals), ConfidenceSum: float64(len(acceptedSignals)), PrioritySum: priority, Freshness: freshness, CreatedAt: input.Now, UpdatedAt: input.Now}
	return summary, summary.Validate()
}
