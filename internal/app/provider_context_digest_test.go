package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPC4MandatoryFreshDigest(t *testing.T) {
	a := &contextContractAssembler{}
	w := contextContractHandler(t, a)(`{` + contextMetadata + `,"input":{"query":"q","budget":2}}`)
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	var body struct {
		Result struct {
			Digest     map[string]string `json:"context_digest"`
			Continuity map[string]string `json:"continuity"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Result.Digest["contract_version"] != "context-digest-v1" || len(body.Result.Digest["value"]) != 64 || body.Result.Continuity["outcome"] != "not_requested" {
		t.Fatal("missing PC4 digest/continuity contract")
	}
}

func TestPC4ValidContinuityAcceptedBeforeDependency(t *testing.T) {
	for _, c := range []string{`{"mode":"issue"}`, `{"mode":"compare","reference":"ctxc_0123456789abcdef0123456789abcdef"}`} {
		a := &contextContractAssembler{}
		w := contextContractHandler(t, a)(`{` + contextMetadata + `,"input":{"query":"q","budget":2,"continuity":` + c + `}}`)
		if w.Code != 503 || a.calls != 0 {
			t.Fatalf("expected missing durable receipt dependency, status=%d calls=%d", w.Code, a.calls)
		}
	}
}

func TestPC4RejectInvalidUnicodeAndContinuity(t *testing.T) {
	inputs := []string{`{"query":"\ud800","budget":1}`, `{"query":"\udc00","budget":1}`, "{\"query\":\"" + string([]byte{0xff}) + "\",\"budget\":1}"}
	for _, c := range []string{`null`, `{}`, `{"Mode":"issue"}`, `{"mode":"issue","reference":"ctxc_0123456789abcdef0123456789abcdef"}`, `{"mode":"issue","mode":"issue"}`, `{"mode":"compare"}`, `{"mode":"compare","reference":"CTXc_0123456789abcdef0123456789abcdef"}`, `{"mode":"compare","reference":null}`, `{"mode":"issue","extra":true}`} {
		inputs = append(inputs, `{"query":"q","budget":1,"continuity":`+c+`}`)
	}
	for _, input := range inputs {
		a := &contextContractAssembler{}
		w := contextContractHandler(t, a)(`{` + contextMetadata + `,"input":` + input + `}`)
		if w.Code != 400 || a.calls != 0 || !strings.Contains(w.Body.String(), "invalid_request") {
			t.Fatalf("invalid request accepted: status=%d calls=%d", w.Code, a.calls)
		}
	}
}
