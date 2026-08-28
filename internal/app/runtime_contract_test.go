package app

import "testing"

func TestRuntimeContractDefaultsAreBounded(t *testing.T) {
	contract := RuntimeContract{}
	contract.Normalize()
	if contract.ServiceVersion == "" || contract.BuildID == "" || contract.BuildTimestamp == "" || contract.SchemaVersion <= 0 {
		t.Fatalf("normalized contract = %+v, want bounded defaults", contract)
	}
}
