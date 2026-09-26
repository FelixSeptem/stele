package mcp

import (
	"strings"
	"testing"
)

func TestValidateToolRequestRejectsUnknownOperationAndUnboundedInputs(t *testing.T) {
	if err := ValidateToolRequest("not_a_tool", nil, Limits{}); err == nil {
		t.Fatal("ValidateToolRequest() error = nil for unknown operation")
	}
	limits := Limits{MaxQueryBytes: 16, MaxPayloadBytes: 32, MaxResults: 2, MaxIDs: 2}
	if err := (SearchRequest{Query: strings.Repeat("q", 17), Limit: 1}).Validate(limits); err == nil {
		t.Fatal("SearchRequest.Validate() error = nil for oversized query")
	}
	if err := (ContextRequest{BudgetBytes: 33}).Validate(limits); err == nil {
		t.Fatal("ContextRequest.Validate() error = nil for oversized budget")
	}
	if err := (RememberRequest{Content: strings.Repeat("x", 33)}).Validate(limits); err == nil {
		t.Fatal("RememberRequest.Validate() error = nil for oversized payload")
	}
	if err := (ForgetApplyRequest{PreviewID: "p", MemoryIDs: []string{"1", "2", "3"}}).Validate(limits); err == nil {
		t.Fatal("ForgetApplyRequest.Validate() error = nil for oversized ID set")
	}
}

func TestToolDescriptorsKeepReadAndMutationContractsSeparate(t *testing.T) {
	descriptors := ToolDescriptors()
	if len(descriptors) < 6 {
		t.Fatalf("ToolDescriptors() length = %d, want at least six", len(descriptors))
	}
	seen := map[string]bool{}
	for _, descriptor := range descriptors {
		if descriptor.Name == "" || seen[descriptor.Name] {
			t.Fatalf("invalid or duplicate descriptor: %+v", descriptor)
		}
		seen[descriptor.Name] = true
	}
	if seen[ToolSearch] == false || seen[ToolContext] == false || seen[ToolBrowse] == false || seen[ToolRemember] == false || seen[ToolForgetPreview] == false || seen[ToolForgetApply] == false {
		t.Fatalf("descriptors missing required tool taxonomy: %+v", seen)
	}
}

func TestForgetApplyRequestRequiresReviewedUniqueBoundedIDs(t *testing.T) {
	limits := Limits{MaxQueryBytes: 64, MaxPayloadBytes: 128, MaxResults: 5, MaxIDs: 2}
	valid := ForgetApplyRequest{PreviewID: "fp_1", MemoryIDs: []string{"m1", "m2"}, Reason: "cleanup", IdempotencyKey: "idem-1"}
	if err := valid.Validate(limits); err != nil {
		t.Fatalf("valid forget apply rejected: %v", err)
	}
	valid.MemoryIDs = []string{"m1", "m1"}
	if err := valid.Validate(limits); err == nil {
		t.Fatal("duplicate forget IDs accepted")
	}
}

func TestForgetRequestRequiresOneBoundedMemoryIDReasonAndIdempotency(t *testing.T) {
	limits := Limits{MaxIDs: 2}
	valid := ForgetRequest{MemoryID: "m1", Action: "suppress", Reason: "user request", IdempotencyKey: "forget-1"}
	if err := valid.Validate(limits); err != nil {
		t.Fatalf("valid ForgetRequest.Validate() error = %v", err)
	}
	for name, request := range map[string]ForgetRequest{
		"missing ID":        {Action: "suppress", Reason: "r", IdempotencyKey: "k"},
		"missing reason":    {MemoryID: "m1", Action: "suppress", IdempotencyKey: "k"},
		"missing key":       {MemoryID: "m1", Action: "suppress", Reason: "r"},
		"invalid action":    {MemoryID: "m1", Action: "rewrite", Reason: "r", IdempotencyKey: "k"},
		"oversized binding": {MemoryID: "m1", Reason: "r", IdempotencyKey: "k", RuntimeBindingID: strings.Repeat("b", 257)},
	} {
		t.Run(name, func(t *testing.T) {
			if err := request.Validate(limits); err == nil {
				t.Fatal("ForgetRequest.Validate() error = nil for invalid request")
			}
		})
	}
}

func TestForgetRequestAcceptsEachPrivilegedLifecycleAction(t *testing.T) {
	for _, action := range []string{"suppress", "expire", "delete"} {
		t.Run(action, func(t *testing.T) {
			request := ForgetRequest{MemoryID: "m1", Action: action, Reason: "operator request", IdempotencyKey: "forget-" + action}
			if err := request.Validate(Limits{}); err != nil {
				t.Fatalf("ForgetRequest.Validate(%q) error = %v", action, err)
			}
		})
	}
}

func TestMutationRequestsBoundIdempotencyKeys(t *testing.T) {
	limits := Limits{MaxPayloadBytes: 128, MaxIDs: 2}
	tooLong := strings.Repeat("k", 257)
	if err := (RememberRequest{Content: "content", Reason: "reason", IdempotencyKey: tooLong}).Validate(limits); err == nil {
		t.Fatal("oversized remember idempotency key accepted")
	}
	if err := (ForgetApplyRequest{PreviewID: "fp_1", MemoryIDs: []string{"m1"}, Reason: "reason", IdempotencyKey: tooLong}).Validate(limits); err == nil {
		t.Fatal("oversized forget idempotency key accepted")
	}
}

func TestBrowseSchemaRejectsOffsetBeyondBoundedWindow(t *testing.T) {
	limits := Limits{MaxQueryBytes: 32, MaxPayloadBytes: 64, MaxResults: 10, MaxIDs: 5}
	if err := (BrowseRequest{Offset: 11, Limit: 1}).Validate(limits); err == nil {
		t.Fatal("BrowseRequest.Validate() error = nil for offset beyond bounded window")
	}
}

func TestForgetPreviewSchemaRejectsInvalidThreshold(t *testing.T) {
	limits := Limits{MaxQueryBytes: 32, MaxPayloadBytes: 64, MaxResults: 10, MaxIDs: 5}
	for _, threshold := range []float64{-0.1, 1.1} {
		if err := (ForgetPreviewRequest{Query: "q", Threshold: threshold}).Validate(limits); err == nil {
			t.Fatalf("ForgetPreviewRequest.Validate() error = nil for threshold %v", threshold)
		}
	}
}
