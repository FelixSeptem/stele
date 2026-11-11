package benchmark

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestTaskSeriesV2InconclusiveIndependentCoverage(t *testing.T) {
	m := validTaskSeriesManifest()
	m.SchemaVersion, m.MetricVersion = TaskSeriesSchemaVersionV2, TaskSeriesMetricVersionV2
	no, zero := false, int64(0)
	for ci := range m.Conditions {
		for si := range m.Conditions[ci].Stages {
			s := &m.Conditions[ci].Stages[si]
			s.RepeatedError, s.MaintenanceCost = &no, &zero
		}
		m.Conditions[ci].Stages[0].Verdict = TaskSeriesVerdictInconclusive
	}
	r, err := EvaluateTaskSeries(m, taskSeriesObservationsForManifest(m))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range r.Conditions {
		if c.RepeatedErrorCoverage != 1 || c.MaintenanceCoverage != 1 {
			t.Fatalf("independent coverage must use both current stages: %+v", c)
		}
	}
	if _, err := MarshalTaskSeriesReport(r); err != nil {
		t.Fatal(err)
	}
}

func v3Bundle(t *testing.T) TaskSeriesSourceBundle {
	t.Helper()
	scope := memory.Scope{Tenant: "benchmark", Project: "ev5", Namespace: "locked"}
	b := TaskSeriesSourceBundle{SyntheticFixture: true, Corpora: map[string]NormalizedCorpus{}}
	b.Manifest = TaskSeriesManifest{SchemaVersion: TaskSeriesSchemaVersionV3, MetricVersion: TaskSeriesMetricVersionV3, SourceChecksum: testChecksum, QRELChecksum: testChecksum, Scope: scope,
		Experiment: &TaskSeriesExperiment{ModelDigest: testChecksum, RuntimeDigest: testChecksum, CriteriaDigest: testChecksum, Seeds: []int64{7}, Limits: TaskSeriesLimits{MaxItems: 8, MaxContentBytes: 4096, MaxTokens: 1024, MaxCitations: 16, MaxLatencyMS: 60000}}}
	for i, role := range []string{"no_memory", "source_records", "organized", "verified_corrected", "resident"} {
		c := TaskSeriesCondition{ID: role, Role: role, Baseline: i == 0}
		corpus := NormalizedCorpus{SchemaVersion: SchemaVersion}
		if i > 0 {
			for n, text := range []string{"memory rule value 999", "memory rule value 47"} {
				id := []string{"prior", "corrected"}[n]
				corpus.Events = append(corpus.Events, MemoryEventRecord{ID: id, Scope: scope, SessionID: "series", Class: memory.MemoryClassProfile, ExpectedState: memory.MemoryStateActive, Text: text})
				window := TaskSeriesSourceWindow{Reference: id, SourceVersion: int64(n + 1), ContentDigest: checksumBytes([]byte(text)), AvailableStage: n + 1, Metadata: memory.SourceMetadata{Version: memory.SourceMetadataVersion, Trust: memory.SourceTrustAttestedData, Classification: memory.SourceClassificationLow}}
				if n == 0 {
					window.WithdrawnStage = 2
				}
				c.Sources = append(c.Sources, window)
			}
		}
		for n, id := range []string{"first", "later"} {
			s := TaskSeriesStage{ID: id, QueryID: "query-" + id, Scope: scope, Verdict: TaskSeriesVerdictInconclusive, HeldOut: n > 0}
			q := BenchmarkQuery{ID: s.QueryID, Scope: scope, SessionID: "series", Text: "memory rule"}
			if i > 0 {
				ref := []string{"prior", "corrected"}[n]
				s.Evidence = []TaskSeriesEvidence{{Reference: ref}}
				q.EvidenceGroups = []EvidenceGroup{{ID: "expected", EvidenceIDs: []string{ref}, Required: true}}
				corpus.QRELs = append(corpus.QRELs, QREL{QueryID: q.ID, EvidenceID: ref, Grade: 1})
			}
			c.Stages = append(c.Stages, s)
			corpus.Queries = append(corpus.Queries, q)
		}
		b.Manifest.Conditions = append(b.Manifest.Conditions, c)
		b.Corpora[c.ID] = corpus
	}
	lockV3(t, &b)
	return b
}

func lockV3(t *testing.T, b *TaskSeriesSourceBundle) {
	t.Helper()
	q, s, err := TaskSeriesSourceLocks(*b)
	if err != nil {
		t.Fatal(err)
	}
	b.Manifest.QRELChecksum = q
	b.Manifest.SourceChecksum = s
}

func TestTaskSeriesV3LockedFixtureAndLegacyLocks(t *testing.T) {
	v3Path := filepath.Join("testdata", "locked-longitudinal-task-series-v3.json")
	v3Source, err := os.ReadFile(v3Path)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := LoadTaskSeriesSource(v3Source)
	if err != nil {
		t.Fatal(err)
	}
	qrel, source, err := TaskSeriesSourceLocks(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if qrel != bundle.Manifest.QRELChecksum || source != bundle.Manifest.SourceChecksum {
		t.Fatal("locked v3 fixture checksums drifted")
	}
	generated := v3Bundle(t)
	gq, gs, err := TaskSeriesSourceLocks(generated)
	if err != nil {
		t.Fatal(err)
	}
	if gq != qrel || gs != source {
		t.Fatal("locked v3 fixture no longer matches the five-condition contract")
	}
	tampered := bytes.Replace(v3Source, []byte(source), []byte(strings.Repeat("0", 64)), 1)
	if _, err := LoadTaskSeriesSource(tampered); StatusOf(err) != StatusChecksumMismatch {
		t.Fatalf("locked v3 fixture tampering accepted: %v", err)
	}
	legacySource, err := os.ReadFile(filepath.Join("testdata", "verified-memory-correction-task-series-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := LoadTaskSeriesSource(legacySource)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Manifest.SchemaVersion != TaskSeriesSchemaVersionV2 || legacy.Manifest.SourceChecksum != "312eb46803466ffd547e8ed0c4cfd30ac7291b9750d3932ef12ac34e4729fa39" || legacy.Manifest.QRELChecksum != "432ebc8d8131fd92c0e100d5a2d776651a7cedca013db9d7103120b83a8269f7" {
		t.Fatal("legacy v2 fixture identity changed")
	}
}

func TestTaskSeriesV3ContractAndLegacyLocks(t *testing.T) {
	b := v3Bundle(t)
	if err := ValidateTaskSeriesCorpora(b.Manifest, b.Corpora); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(b)
	if _, err := LoadTaskSeriesSource(data); err != nil {
		t.Fatal(err)
	}
	m := validTaskSeriesManifest()
	m.Experiment = b.Manifest.Experiment
	if err := m.Validate(); err == nil {
		t.Fatal("v1 accepted v3 experiment")
	}
	for _, field := range []string{"model_digest", "runtime_digest", "criteria_digest", "seeds", "max_latency_ms", "available_stage", "content_digest", "held_out", "source_version"} {
		if !bytes.Contains(data, []byte(field)) {
			t.Fatalf("missing locked field %s", field)
		}
	}
	tampered := bytes.Replace(data, []byte(`"available_stage":2`), []byte(`"available_stage":1`), 1)
	if _, err := LoadTaskSeriesSource(tampered); StatusOf(err) != StatusChecksumMismatch {
		t.Fatalf("tamper: %v", err)
	}
}

func TestTaskSeriesV3RejectsIncompatibleInputs(t *testing.T) {
	tests := map[string]func(*TaskSeriesSourceBundle){
		"role":            func(b *TaskSeriesSourceBundle) { b.Manifest.Conditions[1].Role = "resident" },
		"condition count": func(b *TaskSeriesSourceBundle) { b.Manifest.Conditions = b.Manifest.Conditions[:4] },
		"criteria":        func(b *TaskSeriesSourceBundle) { b.Manifest.Experiment.CriteriaDigest = "" },
		"duplicate seeds": func(b *TaskSeriesSourceBundle) { b.Manifest.Experiment.Seeds = []int64{1, 1} },
		"limit":           func(b *TaskSeriesSourceBundle) { b.Manifest.Experiment.Limits.MaxItems = 65 },
		"task mismatch": func(b *TaskSeriesSourceBundle) {
			c := b.Corpora["resident"]
			c.Queries[0].Text += " changed"
			b.Corpora["resident"] = c
		},
		"heldout":         func(b *TaskSeriesSourceBundle) { b.Manifest.Conditions[1].Stages[1].HeldOut = false },
		"future evidence": func(b *TaskSeriesSourceBundle) { b.Manifest.Conditions[1].Sources[0].AvailableStage = 2 },
		"withdrawn":       func(b *TaskSeriesSourceBundle) { b.Manifest.Conditions[1].Sources[1].WithdrawnStage = 2 },
		"content":         func(b *TaskSeriesSourceBundle) { b.Manifest.Conditions[1].Sources[0].ContentDigest = testChecksum },
		"version":         func(b *TaskSeriesSourceBundle) { b.Manifest.Conditions[1].Sources[0].SourceVersion = 0 },
		"restricted": func(b *TaskSeriesSourceBundle) {
			b.Manifest.Conditions[1].Sources[0].Metadata.Classification = memory.SourceClassificationRestricted
		},
		"unmapped source": func(b *TaskSeriesSourceBundle) {
			b.Manifest.Conditions[1].Sources = b.Manifest.Conditions[1].Sources[:1]
		},
		"no memory":   func(b *TaskSeriesSourceBundle) { b.Manifest.Conditions[0].Sources = b.Manifest.Conditions[1].Sources },
		"cost bounds": func(b *TaskSeriesSourceBundle) { n := int64(-1); b.Manifest.Conditions[0].Stages[0].ToolCalls = &n },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			b := v3Bundle(t)
			mutate(&b)
			lockV3(t, &b)
			if err := ValidateTaskSeriesCorpora(b.Manifest, b.Corpora); err == nil {
				t.Fatal("unsafe experiment accepted")
			}
		})
	}
}

func TestTaskSeriesV3SnapshotPhysicallyOmitsFutureAndWithdrawn(t *testing.T) {
	b := v3Bundle(t)
	c := b.Manifest.Conditions[3]
	for n, ref := range []string{"prior", "corrected"} {
		snapshot, err := taskSeriesStageSnapshot(b.Manifest, c, b.Corpora[c.ID], n)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(snapshot)
		forbidden := []string{"corrected", "prior"}[n]
		if len(snapshot.Events) != 1 || snapshot.Events[0].ID != ref || bytes.Contains(encoded, []byte(forbidden)) {
			t.Fatalf("stage %d leaked future/withdrawn input: %s", n, encoded)
		}
	}
	data, _ := json.Marshal(b)
	for name, changed := range map[string][]byte{
		"alias":     bytes.Replace(data, []byte(`"held_out":true`), []byte(`"Held_Out":true`), 1),
		"duplicate": bytes.Replace(data, []byte(`"held_out":true`), []byte(`"held_out":true,"held_out":true`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadTaskSeriesSource(changed); err == nil || !strings.Contains(err.Error(), "decode") {
				t.Fatalf("ambiguous JSON accepted: %v", err)
			}
		})
	}
}

func v3Observations(m TaskSeriesManifest) TaskSeriesObservations {
	obs := taskSeriesObservationsForManifest(m)
	for _, c := range m.Conditions {
		for _, s := range c.Stages {
			v := obs[c.ID][s.ID]
			yes := true
			n := int64(100)
			v.DisclosureSafe = &yes
			v.StageVisible = &yes
			v.RollbackMatched = &yes
			v.LatencyMicros = &n
			obs[c.ID][s.ID] = v
		}
	}
	return obs
}

func TestTaskSeriesV3CostsGatesMissingAndRedaction(t *testing.T) {
	b := v3Bundle(t)
	m := b.Manifest
	zero := int64(0)
	no := false
	for ci := range m.Conditions {
		for si := range m.Conditions[ci].Stages {
			s := &m.Conditions[ci].Stages[si]
			s.ToolCalls = &zero
			s.RepeatedError = &no
			s.MaintenanceCost = &zero
		}
	}
	r, err := EvaluateTaskSeries(m, v3Observations(m))
	if err != nil {
		t.Fatal(err)
	}
	if r.Status == StatusSuccess || r.V3 == nil || r.V3.GateStatus != "passed" || r.V3.ComparisonComplete {
		t.Fatalf("missing outcomes/costs gained authority: %+v", r)
	}
	for _, c := range r.Conditions {
		if c.V3 == nil || c.V3.ToolCalls.Coverage != 1 || c.V3.ToolCalls.Average == nil || *c.V3.ToolCalls.Average != 0 || c.V3.ModelTokens.Average != nil || c.V3.ModelTokens.Coverage != 0 {
			t.Fatalf("missing vs zero costs: %+v", c.V3)
		}
	}
	data, err := MarshalTaskSeriesReport(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"source_records", "organized", "resident", "series", "memory rule", "benchmark", "first", "query-"} {
		// Versioned task-series schema names are intentionally public.
		if secret == "series" {
			continue
		}
		if bytes.Contains(data, []byte(secret)) {
			t.Fatalf("report leaked %q: %s", secret, data)
		}
	}
	all := m
	for ci := range all.Conditions {
		for si := range all.Conditions[ci].Stages {
			s := &all.Conditions[ci].Stages[si]
			s.ModelTokens = &zero
			s.Verdict = TaskSeriesVerdictSucceeded
		}
	}
	r, err = EvaluateTaskSeries(all, v3Observations(all))
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusSuccess || !r.V3.ComparisonComplete {
		t.Fatalf("fully observed conformance failed: %+v", r)
	}
	for _, gate := range []string{"latency", "disclosure", "visibility", "rollback", "missing"} {
		t.Run(gate, func(t *testing.T) {
			obs := v3Observations(all)
			v := obs[all.Conditions[1].ID]["later"]
			bad := false
			slow := int64(60000001)
			switch gate {
			case "latency":
				v.LatencyMicros = &slow
			case "disclosure":
				v.DisclosureSafe = &bad
			case "visibility":
				v.StageVisible = &bad
			case "rollback":
				v.RollbackMatched = &bad
			case "missing":
				v.RollbackMatched = nil
			}
			obs[all.Conditions[1].ID]["later"] = v
			r, err := EvaluateTaskSeries(all, obs)
			if err != nil {
				t.Fatal(err)
			}
			if r.Status == StatusSuccess || r.V3.GateStatus == "passed" {
				t.Fatal("utility overrode hard gate")
			}
		})
	}
}

func TestTaskSeriesV3RefusesObservedFutureEvidence(t *testing.T) {
	b := v3Bundle(t)
	obs := v3Observations(b.Manifest)
	c := b.Manifest.Conditions[1]
	s := c.Stages[0]
	// Declaring an alias in the stage cannot override its source window.
	s.Evidence = append(s.Evidence, TaskSeriesEvidence{Reference: "corrected"})
	b.Manifest.Conditions[1].Stages[0] = s
	v := obs[c.ID][s.ID]
	v.Evidence = append(v.Evidence, TaskSeriesObservedEvidence{Reference: "corrected", ScopeEligible: true, LifecycleVisible: true, ProvenanceVerified: true})
	obs[c.ID][s.ID] = v
	if _, err := EvaluateTaskSeries(b.Manifest, obs); err == nil {
		t.Fatal("observed future evidence accepted")
	}
}

func TestTaskSeriesV3MissingVerdictRemainsInconclusive(t *testing.T) {
	b := v3Bundle(t)
	b.Manifest.Conditions[1].Stages[0].Verdict = ""
	r, err := EvaluateTaskSeries(b.Manifest, v3Observations(b.Manifest))
	if err != nil {
		t.Fatal(err)
	}
	if r.Conditions[1].InconclusiveStages != 2 || r.Status == StatusSuccess || r.V3.ComparisonComplete {
		t.Fatalf("missing verdict inferred: %+v", r)
	}
}

func TestTaskSeriesLegacyRejectsV3FieldPresence(t *testing.T) {
	for _, v2 := range []bool{false, true} {
		m := validTaskSeriesManifest()
		if v2 {
			m.SchemaVersion = TaskSeriesSchemaVersionV2
			m.MetricVersion = TaskSeriesMetricVersionV2
		}
		b := TaskSeriesSourceBundle{Manifest: m, Corpora: validTaskSeriesCorpora(m), SyntheticFixture: true}
		lockV3(t, &b)
		data, _ := json.Marshal(b)
		for _, edit := range []struct{ from, to string }{
			{`"conditions":`, `"experiment":null,"conditions":`},
			{`"stages":`, `"role":"","stages":`},
			{`"stages":`, `"sources":[],"stages":`},
			{`"verdict":`, `"held_out":false,"verdict":`},
			{`"verdict":`, `"model_tokens":null,"verdict":`},
			{`"verdict":`, `"tool_calls":null,"verdict":`},
		} {
			changed := bytes.Replace(data, []byte(edit.from), []byte(edit.to), 1)
			if _, err := LoadTaskSeriesSource(changed); err == nil {
				t.Fatalf("legacy %t accepted %s", v2, edit.to)
			}
		}
	}
}

func TestTaskSeriesV3MapKeysRemainCaseSensitive(t *testing.T) {
	b := v3Bundle(t)
	c := b.Corpora["resident"]
	c.Events[0].Provenance = map[string]string{"A": "upper", "a": "lower"}
	b.Corpora["resident"] = c
	lockV3(t, &b)
	data, _ := json.Marshal(b)
	if _, err := LoadTaskSeriesSource(data); err != nil {
		t.Fatal(err)
	}
}
