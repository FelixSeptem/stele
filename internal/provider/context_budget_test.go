package provider

import (
	"fmt"
	"testing"
)

func TestContextBudgetStrictRequest(t *testing.T) {
	for _, limits := range []string{`{}`, `{"max_content_bytes":0,"max_citations":0,"max_response_bytes":1}`, `{"max_content_bytes":2147483647}`} {
		body := fmt.Sprintf(`{"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1"},"input":{"query":"q","budget":2147483647,"budget_limits":%s}}`, limits)
		if _, err := DecodeContextRequest([]byte(body)); err != nil {
			t.Errorf("valid limits %s: %v", limits, err)
		}
	}
	for _, limits := range []string{`null`, `{"max_content_bytes":null}`, `{"max_content_bytes":-1}`, `{"max_content_bytes":2147483648}`, `{"max_response_bytes":0}`, `{"max_citations":1.5}`, `{"max_citations":"1"}`, `{"MaxCitations":1}`, `{"max_citations":1,"max_citations":2}`, `{"max_tokens":1}`, `{"tokenizer":"x"}`} {
		body := fmt.Sprintf(`{"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1"},"input":{"query":"q","budget":1,"budget_limits":%s}}`, limits)
		if _, err := DecodeContextRequest([]byte(body)); err == nil {
			t.Errorf("accepted %s", limits)
		}
	}
	for _, budget := range []string{`0`, `-1`, `2147483648`, `1.5`, `null`} {
		body := fmt.Sprintf(`{"metadata":{"request_id":"r","operation_id":"o","schema_version":"provider-v1"},"input":{"query":"q","budget":%s}}`, budget)
		if _, err := DecodeContextRequest([]byte(body)); err == nil {
			t.Errorf("accepted budget %s", budget)
		}
	}
}
