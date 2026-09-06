package benchmark

import (
	"encoding/json"
	"fmt"
	"sort"
)

type FamilyReport struct {
	SchemaVersion string                     `json:"schema_version"`
	Family        BenchmarkFamily            `json:"family"`
	Reports       map[string]json.RawMessage `json:"reports"`
	Status        Status                     `json:"status"`
	Incomparable  []string                   `json:"incomparable,omitempty"`
	NonComparable bool                       `json:"non_comparable"`
	Comparability string                     `json:"comparability"`
}

// BuildFamilyReport keeps heterogeneous benchmark outputs addressable without
// pretending that contract, memory, generic IR, and stress metrics are equal.
func BuildFamilyReport(family BenchmarkFamily, reports map[string]any) (FamilyReport, error) {
	if !validBenchmarkFamily(string(family)) {
		return FamilyReport{}, fmt.Errorf("unsupported benchmark family %q", family)
	}
	result := FamilyReport{SchemaVersion: SchemaVersion, Family: family, Reports: map[string]json.RawMessage{}, Status: StatusSuccess, NonComparable: true, Comparability: "family-scoped metrics are not comparable across benchmark families"}
	keys := make([]string, 0, len(reports))
	for key := range reports {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		encoded, err := json.Marshal(reports[key])
		if err != nil {
			return FamilyReport{}, err
		}
		result.Reports[key] = encoded
	}
	if len(result.Reports) == 0 {
		result.Status = StatusPrerequisiteMissing
	}
	return result, nil
}

func MarshalFamilyReport(report FamilyReport) ([]byte, error) {
	report.SchemaVersion = SchemaVersion
	return json.Marshal(report)
}

// CompareFamilyReports deliberately refuses to compare reports from different
// benchmark families. Even reports in the same family are only grouped here;
// metric-level comparison still requires the strategy/corpus comparator.
func CompareFamilyReports(left, right FamilyReport) error {
	if left.Family == "" || right.Family == "" {
		return fmt.Errorf("family identity is required")
	}
	if left.Family != right.Family {
		return fmt.Errorf("benchmark family reports are non-comparable: %s vs %s", left.Family, right.Family)
	}
	return nil
}

// RenderFamilyReport emits the stable machine-readable family envelope used by
// CLI/report consumers. It intentionally does not flatten nested reports.
func RenderFamilyReport(report FamilyReport) ([]byte, error) {
	if report.Family == "" {
		return nil, fmt.Errorf("family identity is required")
	}
	if report.Comparability == "" {
		report.Comparability = "family-scoped metrics are not comparable across benchmark families"
	}
	report.NonComparable = true
	report.SchemaVersion = SchemaVersion
	return json.Marshal(report)
}
