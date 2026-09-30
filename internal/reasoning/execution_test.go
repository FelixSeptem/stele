package reasoning

import (
	"context"
	"testing"
)

type staticProvider struct {
	called int
	value  Candidate
	err    error
}

func (p *staticProvider) Derive(_ context.Context, _ InvocationRequest) (Candidate, error) {
	p.called++
	return p.value, p.err
}

func TestOfflineReplayIsDeterministicAndDoesNotCallProvider(t *testing.T) {
	fixture := Fixture{ID: "fixture-1", Request: validRequest(), ExpectedCandidate: validCandidate(), ExpectedFingerprint: ""}
	p := &staticProvider{value: validCandidate()}
	runner := OfflineRunner{Provider: p, Limits: DefaultLimits()}
	first, err := runner.Replay(context.Background(), fixture)
	if err != nil {
		t.Fatalf("first replay: %v", err)
	}
	fixture.ExpectedFingerprint = first.Fingerprint
	second, err := runner.Replay(context.Background(), fixture)
	if err != nil {
		t.Fatalf("second replay: %v", err)
	}
	if first.Fingerprint != second.Fingerprint || first.Outcome != second.Outcome {
		t.Fatalf("replay changed: first=%+v second=%+v", first, second)
	}
	if p.called != 0 {
		t.Fatalf("offline replay called provider %d times", p.called)
	}
}

func TestShadowExecutionNeverReturnsProviderCandidateAsAuthoritative(t *testing.T) {
	p := &staticProvider{value: validCandidate()}
	shadow := ShadowExecutor{Provider: p, Limits: DefaultLimits()}
	result, err := shadow.Execute(context.Background(), validRequest(), validCandidate())
	if err != nil {
		t.Fatalf("shadow execute: %v", err)
	}
	if result.Authoritative {
		t.Fatal("shadow result became authoritative")
	}
	if !result.Equivalent {
		t.Fatal("matching candidate was not equivalent")
	}
	if p.called != 1 {
		t.Fatalf("provider calls = %d, want 1", p.called)
	}
}

func TestExecutionFailureUsesStableCategoryAndFallback(t *testing.T) {
	p := &staticProvider{err: &ProviderError{Category: ErrorCategoryTimeout, Code: "deadline", Message: "provider deadline", Retryable: true}}
	exec := Executor{Provider: p, Limits: DefaultLimits()}
	result, err := exec.Execute(context.Background(), validRequest(), validCandidate())
	if err != nil {
		t.Fatalf("fallback should not fail: %v", err)
	}
	if result.Outcome != OutcomeFallback || result.Error == nil || result.Error.Category != ErrorCategoryTimeout {
		t.Fatalf("unexpected fallback result: %+v", result)
	}
	if result.Candidate.ID != "cand-1" {
		t.Fatalf("fallback candidate lost: %+v", result.Candidate)
	}
}

func TestMalformedProviderOutputIsRejectedWithoutFallbackMutation(t *testing.T) {
	p := &staticProvider{value: Candidate{ID: "bad", Scope: validCandidate().Scope, Kind: "derived_insight", InsightType: "hypothesis", Content: "unsafe", Evidence: validCandidate().Evidence, PolicyVersion: "p", ProviderVersion: "v", ReplayID: "r"}}
	exec := Executor{Provider: p, Limits: DefaultLimits()}
	result, err := exec.Execute(context.Background(), validRequest(), validCandidate())
	if err != nil {
		t.Fatalf("malformed output should be categorized: %v", err)
	}
	if result.Outcome != OutcomeFallback || result.Error == nil || result.Error.Category != ErrorCategoryMalformedOutput {
		t.Fatalf("unexpected malformed result: %+v", result)
	}
}
