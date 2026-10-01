package evaluation

import (
	"fmt"
	"strings"
	"time"
)

type RetentionInput struct {
	ArtifactCategory string
	Result           string
	DeletedCount     int64
	ReasonCategory   string
	At               time.Time
}

type RetentionOutcome struct {
	ArtifactCategory string    `json:"artifact_category"`
	Result           string    `json:"result"`
	DeletedCount     int64     `json:"deleted_count"`
	ReasonCategory   string    `json:"reason_category"`
	CreatedAt        time.Time `json:"created_at"`
}

func BuildRetentionOutcome(input RetentionInput) (RetentionOutcome, error) {
	category := normalizeCategory(input.ArtifactCategory, "trajectory", "integrity_report", "retention_outcome", "unknown")
	if category == "unknown" {
		return RetentionOutcome{}, fmt.Errorf("artifact category is not derived")
	}
	result := normalizeCategory(input.Result, "deleted", "skipped", "failed", "unknown")
	if result == "unknown" {
		return RetentionOutcome{}, fmt.Errorf("retention result is invalid")
	}
	if input.DeletedCount < 0 || input.DeletedCount > 2147483647 {
		return RetentionOutcome{}, fmt.Errorf("deleted count is outside the supported range")
	}
	reason := strings.TrimSpace(input.ReasonCategory)
	if reason == "" {
		reason = "expired"
	}
	reason = normalizeCategory(reason, "expired", "not_expired", "scope_filtered", "unknown")
	created := input.At.UTC()
	if created.IsZero() {
		created = time.Unix(0, 0).UTC()
	}
	return RetentionOutcome{ArtifactCategory: category, Result: result, DeletedCount: input.DeletedCount, ReasonCategory: reason, CreatedAt: created}, nil
}
