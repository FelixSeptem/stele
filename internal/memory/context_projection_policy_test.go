package memory

import "testing"

func TestResolveContextProjectionPolicyIsClassAwareAndLifecycleSafe(t *testing.T) {
	policy := DefaultContextProjectionPolicy("policy-v1")
	cases := []struct {
		name   string
		kind   ContextProjectionKind
		class  MemoryClass
		state  MemoryState
		conf   float64
		length int
		allow  bool
		reason ContextProjectionOmissionReason
	}{
		{"eligible profile", ContextProjectionKindAlwaysVisible, MemoryClassProfile, MemoryStateActive, 0.9, 20, true, ""},
		{"low confidence profile", ContextProjectionKindAlwaysVisible, MemoryClassProfile, MemoryStateActive, 0.1, 20, false, ContextProjectionOmissionPolicy},
		{"summary session", ContextProjectionKindSession, MemoryClassSummary, MemoryStateActive, 0, 20, true, ""},
		{"episodic on demand", ContextProjectionKindAlwaysVisible, MemoryClassEpisodic, MemoryStateActive, 1, 20, false, ContextProjectionOmissionClass},
		{"raw archival", ContextProjectionKindArchivalHistory, MemoryClassEpisodic, MemoryStateActive, 0, 20, true, ""},
		{"suppressed", ContextProjectionKindSession, MemoryClassSummary, MemoryStateSuppressed, 1, 20, false, ContextProjectionOmissionLifecycle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := ResolveContextProjectionPolicy(policy, tc.kind, tc.class, tc.state, tc.conf, tc.length, false)
			if decision.Include != tc.allow || decision.Reason != tc.reason {
				t.Fatalf("decision = %+v, want include=%v reason=%q", decision, tc.allow, tc.reason)
			}
		})
	}
}
