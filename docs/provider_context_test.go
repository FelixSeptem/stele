package docs_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/openapi"
	"github.com/getkin/kin-openapi/openapi3"
)

// Validate the examples readers actually copy, rather than duplicated test JSON.
func TestProviderContextGuideExamplesMatchOpenAPI(t *testing.T) {
	data, err := os.ReadFile("agent-runtime-memory-provider.md")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := openapi3.NewLoader().LoadFromData([]byte(openapi.SpecYAML()))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "## Provider context JSON contract")
	end := strings.Index(text, "## Snapshot and reconnect synchronization")
	if start < 0 || end <= start {
		t.Fatal("context guide section missing")
	}
	section := text[start:end]
	count := 0
	for _, schemaName := range []string{"ProviderContextRequest", "ProviderContextResponse"} {
		remaining := section
		marker := "(" + schemaName + "):"
		for {
			index := strings.Index(remaining, marker)
			if index < 0 {
				break
			}
			remaining = remaining[index+len(marker):]
			fence := strings.Index(remaining, "```json\n")
			if fence < 0 {
				t.Fatal("example JSON fence missing")
			}
			remaining = remaining[fence+len("```json\n"):]
			close := strings.Index(remaining, "```")
			if close < 0 {
				t.Fatal("unterminated example")
			}
			var value any
			if err = json.Unmarshal([]byte(remaining[:close]), &value); err != nil {
				t.Fatal(err)
			}
			if err = doc.Components.Schemas[schemaName].Value.VisitJSON(value); err != nil {
				t.Fatalf("%s documentation example: %v", schemaName, err)
			}
			count++
			remaining = remaining[close+3:]
		}
	}
	if count != 3 {
		t.Fatalf("validated %d examples, want 3", count)
	}
}
