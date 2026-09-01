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
}

// BuildFamilyReport keeps heterogeneous benchmark outputs addressable without
// pretending that contract, memory, generic IR, and stress metrics are equal.
func BuildFamilyReport(family BenchmarkFamily, reports map[string]any) (FamilyReport, error) {
	if !validBenchmarkFamily(string(family)) {
		return FamilyReport{}, fmt.Errorf("unsupported benchmark family %q", family)
	}
	result := FamilyReport{SchemaVersion: SchemaVersion, Family: family, Reports: map[string]json.RawMessage{}, Status: StatusSuccess}
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
