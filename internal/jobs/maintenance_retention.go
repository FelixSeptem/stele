package jobs

func IsDerivedMaintenanceRetentionCategory(category string) bool {
	switch category {
	case "job_execution", "projection_evidence", "conformance_evidence", "redacted_trajectory":
		return true
	default:
		return false
	}
}
