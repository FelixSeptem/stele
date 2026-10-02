package reasoning

import "fmt"

// InsightDiagnostics is safe for operator surfaces: it contains only bounded
// counters and categories, never scope values, candidate IDs, prompts, or
// provider payloads.
type InsightDiagnostics struct {
	AuthorizedScope bool   `json:"authorized_scope"`
	Mode            string `json:"mode"`
	InsightType     string `json:"insight_type"`
	Result          string `json:"result"`
	Eligibility     string `json:"eligibility"`
	Freshness       string `json:"freshness"`
	Fallback        string `json:"fallback"`
	Candidates      int    `json:"candidates"`
	WouldActivate   int    `json:"would_activate"`
	Quarantined     int    `json:"quarantined"`
	Rejected        int    `json:"rejected"`
}

func (d InsightDiagnostics) Validate() error {
	if !d.AuthorizedScope {
		return fmt.Errorf("reasoning diagnostics require an authorized scope")
	}
	if boundedReasoningCategory(d.Mode, "offline", "shadow", "apply") == "unknown" || boundedReasoningCategory(d.InsightType, "hypothesis", "goal", "contradiction", "causal_link", "unknown") == "unknown" && d.InsightType != "unknown" {
		return fmt.Errorf("reasoning diagnostics category is invalid")
	}
	if d.Candidates < 0 || d.WouldActivate < 0 || d.Quarantined < 0 || d.Rejected < 0 {
		return fmt.Errorf("reasoning diagnostic counters cannot be negative")
	}
	return nil
}

func boundedReasoningCategory(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return candidate
		}
	}
	return "unknown"
}
