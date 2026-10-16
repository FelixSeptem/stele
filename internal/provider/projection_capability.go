package provider

type ProjectionCapabilities struct {
	Enabled               bool     `json:"enabled"`
	ContractVersion       string   `json:"contract_version"`
	Operations            []string `json:"operations"`
	Kinds                 []string `json:"kinds"`
	Dispositions          []string `json:"dispositions"`
	ReferenceStates       []string `json:"reference_states"`
	ErrorCategories       []string `json:"error_categories"`
	MaxRequestBytes       int      `json:"max_request_bytes"`
	MaxResponseBytes      int      `json:"max_response_bytes"`
	MaxReferences         int      `json:"max_references"`
	MaxHistory            int      `json:"max_history"`
	FeedbackPolicyVersion string   `json:"feedback_policy_version"`
}

func ProjectionCapability(enabled bool) ProjectionCapabilities {
	return ProjectionCapabilities{Enabled: enabled, ContractVersion: ProjectionContractVersion,
		Operations: []string{"providerAdmitProjection", "providerProjectionHistory"}, Kinds: []string{"turn", "outcome", "feedback"},
		Dispositions: []string{"accepted", "corrected"}, ReferenceStates: []string{"accepted", "pending", "completed", "failed", "unavailable"},
		ErrorCategories: []string{"authentication", "scope", "validation", "compatibility", "conflict", "stale", "lifecycle", "dependency"},
		MaxRequestBytes: MaxProjectionRequestBytes, MaxResponseBytes: MaxProjectionResponseBytes, MaxReferences: MaxProjectionReferences, MaxHistory: MaxProjectionHistory, FeedbackPolicyVersion: ProjectionFeedbackPolicy}
}
