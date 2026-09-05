package benchmark

import (
	"errors"
	"testing"

	"github.com/FelixSeptem/stele/internal/memory"
)

func TestDefaultRegistryDescribesEveryFamilyAndLocalPrerequisites(t *testing.T) {
	registry := DefaultRegistry()
	for _, name := range []string{"locomo", "longmemeval", "bfcl", "personachat", "timeqa", "hotpotqa", "beir", "mteb", "c-mteb", "needle-haystack", "openai-mrcr", "longbench-v2", "vtcbench"} {
		entry, ok := registry.Get(name)
		if !ok {
			t.Fatalf("registry does not contain %q", name)
		}
		if entry.Family == "" || entry.LicenseStatus == "" || len(entry.LocalPrerequisites) == 0 {
			t.Fatalf("registry entry %q lacks family, license status, or local prerequisites: %#v", name, entry)
		}
	}
	if got, _ := registry.Get("beir"); got.Family != FamilyGenericRetrieval {
		t.Fatalf("beir family = %q, want %q", got.Family, FamilyGenericRetrieval)
	}
}

func TestManifestRequiresQRELChecksumAndSplitIdentity(t *testing.T) {
	manifest := validManifest()
	manifest.QRELChecksum = ""
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected missing qrels checksum to be rejected")
	}
	manifest = validManifest()
	manifest.Splits["smoke"] = SplitSpec{Source: "fixture", Identity: ""}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected missing split identity to be rejected")
	}
}

func TestFamilyCacheLayoutSeparatesGenericArtifactsAndDetectsManifestQRELDrift(t *testing.T) {
	cache := NewCache(t.TempDir())
	manifest := validManifest()
	manifest.Family = FamilyGenericRetrieval
	manifest.Name = "beir"
	paths, err := cache.EnsureManifestLayout(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if paths.Family != string(FamilyGenericRetrieval) {
		t.Fatalf("cache family = %q, want %q", paths.Family, FamilyGenericRetrieval)
	}
	corpus := NormalizedCorpus{Events: []MemoryEventRecord{{ID: "doc-1", Scope: memoryScope(), Text: "document"}}}
	if _, err := cache.WriteNormalized(manifest, "smoke", corpus); err != nil {
		t.Fatal(err)
	}
	manifest.QRELChecksum = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	if _, _, err := cache.LoadNormalized(manifest, "smoke"); StatusOf(err) != StatusChecksumMismatch {
		t.Fatalf("expected qrels manifest drift status, got %v", err)
	}
}

func TestGenericNormalizationIsStableAndCarriesGenericFamily(t *testing.T) {
	source := GenericRetrievalSource{
		CorpusIdentity: "beir/scifact/corpus@v1", QRELIdentity: "beir/scifact/qrels@test",
		Documents: []GenericRetrievalDocument{{ID: "doc-b", Text: "second"}, {ID: "doc-a", Text: "first"}},
		Queries:   []GenericRetrievalQuery{{ID: "query-1", Text: "which document"}},
		QRELs:     []GenericRetrievalQREL{{QueryID: "query-1", DocumentID: "doc-a", Grade: 2}},
	}
	corpus, err := NormalizeGenericRetrieval(source, memoryScope())
	if err != nil {
		t.Fatal(err)
	}
	if corpus.Family != FamilyGenericRetrieval || corpus.CorpusIdentity != source.CorpusIdentity || corpus.QRELIdentity != source.QRELIdentity {
		t.Fatalf("normalization lost generic retrieval identity: %#v", corpus)
	}
	if len(corpus.Corpus.Events) != 2 || corpus.Corpus.Events[0].ID != "doc-a" {
		t.Fatalf("documents were not deterministically normalized: %#v", corpus.Corpus.Events)
	}
}

func TestGenericComparisonRejectsIncompatibleInputsAndProductionScopes(t *testing.T) {
	identity := GenericRunIdentity{CorpusChecksum: "corpus-a", QRELChecksum: "qrels-a", EmbeddingIdentity: "embed-a", Scope: memory.Scope{Tenant: "benchmark", Project: "benchmark", Namespace: "generic-run"}}
	left := GenericStrategyResult{Profile: StrategyProfile{ID: "lexical-v1", Kind: StrategyLexical}, Identity: identity, Report: EvaluationReport{Metrics: EvaluationMetrics{QueryCount: 1}}}
	right := left
	right.Profile.Kind = StrategyHybridRank
	right.Identity.QRELChecksum = "qrels-b"
	if _, err := CompareGenericStrategies(left, right); !errors.Is(err, ErrIncompatibleGenericRun) {
		t.Fatalf("expected incompatible generic run error, got %v", err)
	}
	left.Identity.Scope.Project = "production"
	if err := left.Validate(); !errors.Is(err, ErrGenericProductionScope) {
		t.Fatalf("expected production scope rejection, got %v", err)
	}
}

func TestStrategyProfilesAreDeterministicAndIncludeAllRequiredPaths(t *testing.T) {
	profiles := DefaultGenericStrategyProfiles()
	if len(profiles) != 6 {
		t.Fatalf("profile count = %d, want 6", len(profiles))
	}
	want := []RetrievalStrategy{StrategyLexical, StrategySemantic, StrategyHybrid, StrategyChunk, StrategyHybridRank, StrategyReranker}
	for index, strategy := range want {
		if profiles[index].Kind != strategy || profiles[index].ID == "" {
			t.Fatalf("profile %d = %#v, want %q", index, profiles[index], strategy)
		}
	}
}
