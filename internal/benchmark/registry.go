package benchmark

import (
	"fmt"
	"sort"

	"github.com/FelixSeptem/stele/internal/memory"
)

type DatasetRegistration struct {
	Layer       int             `json:"layer"`
	Description string          `json:"description"`
	Family      string          `json:"family"`
	Manifest    DatasetManifest `json:"manifest"`
}

type Registry struct {
	entries map[string]DatasetRegistration
}

type DatasetAdapter interface {
	NormalizeLocal(scope memory.Scope, source []byte) (NormalizedCorpus, error)
}

// AdapterOptions gates experimental adapters without changing the default
// registry support state or silently substituting another dataset.
type AdapterOptions struct {
	EnableLongMemEvalSpike bool
	LongMemEvalSubset      string
}

func DefaultRegistry() Registry {
	return Registry{entries: map[string]DatasetRegistration{
		"stele-fixture":      registration(0, "Repository-owned lifecycle and isolation regression fixture", "stele-fixture", SupportRunnable, RedistributionPermitted),
		"locomo":             registration(1, "Long-term conversational memory benchmark", "locomo", SupportRunnable, RedistributionRestricted),
		"longmemeval":        registration(2, "Long-term memory update and conflict benchmark", "longmemeval", SupportMetadataOnly, RedistributionRestricted),
		"bfcl-memory":        registration(2, "Offline agent memory provider contract", "bfcl-memory", SupportRunnable, RedistributionRestricted),
		"multi-session-chat": registration(3, "Cross-session conversation and profile benchmark", "multi-session-chat", SupportMetadataOnly, RedistributionRestricted),
		"personachat":        registration(3, "Persona and preference benchmark", "personachat", SupportMetadataOnly, RedistributionRestricted),
		"hotpotqa":           registration(4, "Multi-hop retrieval pressure benchmark", "hotpotqa", SupportMetadataOnly, RedistributionRestricted),
		"timeqa":             registration(4, "Temporal retrieval pressure benchmark", "timeqa", SupportMetadataOnly, RedistributionRestricted),
		"beir":               registration(4, "General information retrieval pressure benchmark", "beir", SupportMetadataOnly, RedistributionRestricted),
		"c-mteb":             registration(4, "Chinese general retrieval subset", "c-mteb", SupportMetadataOnly, RedistributionRestricted),
		"mteb":               registration(4, "General retrieval subset", "mteb", SupportMetadataOnly, RedistributionRestricted),
		"needle":             registration(5, "Controlled long-context needle stress", "needle", SupportPlanned, RedistributionUnknown),
		"mrcr":               registration(5, "OpenAI multi-round context stress", "mrcr", SupportPlanned, RedistributionUnknown),
		"longbench-v2":       registration(5, "Long-context task stress subset", "longbench-v2", SupportPlanned, RedistributionUnknown),
		"vtcbench":           registration(5, "Visual/text context stress subset", "vtcbench", SupportPlanned, RedistributionUnknown),
	}}
}

func registration(layer int, description, name string, support SupportState, redistribution RedistributionStatus) DatasetRegistration {
	family := "memory"
	switch name {
	case "longmemeval", "locomo":
		family = "memory"
	case "bfcl-memory":
		family = "provider_contract"
	case "multi-session-chat", "personachat", "hotpotqa", "timeqa":
		family = "specialized_retrieval"
	case "beir", "c-mteb", "mteb":
		family = "generic_retrieval"
	case "needle", "mrcr", "longbench-v2", "vtcbench":
		family = "stress"
	}
	return DatasetRegistration{
		Layer:       layer,
		Description: description,
		Family:      family,
		Manifest: DatasetManifest{
			SchemaVersion:     SchemaVersion,
			Name:              name,
			Family:            family,
			Version:           "unlocked",
			License:           "verify-upstream-license-before-fetch",
			UpstreamURL:       "https://example.invalid/" + name,
			UpstreamRevision:  "unlocked",
			SHA256:            "0000000000000000000000000000000000000000000000000000000000000000",
			SourcePath:        "user-provided",
			ConversionVersion: "v1",
			Redistribution:    redistribution,
			Support:           support,
			Splits:            map[string]SplitSpec{"smoke": {Source: "smoke"}, "full": {Source: "full"}},
			Embedding:         EmbeddingProfile{Name: "unconfigured", Normalization: "none"},
		},
	}
}

func (r Registry) Get(name string) (DatasetRegistration, bool) {
	entry, ok := r.entries[name]
	return entry, ok
}

func (r Registry) List() []DatasetRegistration {
	items := make([]DatasetRegistration, 0, len(r.entries))
	for _, entry := range r.entries {
		items = append(items, entry)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Layer == items[j].Layer {
			return items[i].Manifest.Name < items[j].Manifest.Name
		}
		return items[i].Layer < items[j].Layer
	})
	return items
}

func (r Registry) Adapter(name string) (DatasetAdapter, error) {
	return r.AdapterWithOptions(name, AdapterOptions{})
}

func (r Registry) AdapterWithOptions(name string, options AdapterOptions) (DatasetAdapter, error) {
	entry, ok := r.Get(name)
	if !ok {
		return nil, &StatusError{Status: StatusPrerequisiteMissing, Message: "benchmark dataset is not registered"}
	}
	spikeEnabled := name == "longmemeval" && options.EnableLongMemEvalSpike
	if entry.Manifest.Support != SupportRunnable && !spikeEnabled {
		return nil, &StatusError{Status: StatusPrerequisiteMissing, Message: fmt.Sprintf("benchmark dataset %s is %s", name, entry.Manifest.Support)}
	}
	switch name {
	case "locomo":
		return loCoMoRegistryAdapter{}, nil
	case "longmemeval":
		if !options.EnableLongMemEvalSpike {
			return nil, &StatusError{Status: StatusPrerequisiteMissing, Message: "longmemeval adapter feature flag is disabled"}
		}
		return LongMemEvalAdapter{Enabled: true, Subset: options.LongMemEvalSubset}, nil
	default:
		return nil, &StatusError{Status: StatusPrerequisiteMissing, Message: "benchmark adapter is not implemented"}
	}
}

type loCoMoRegistryAdapter struct{}

func (loCoMoRegistryAdapter) NormalizeLocal(scope memory.Scope, source []byte) (NormalizedCorpus, error) {
	dataset, err := LoadLoCoMoDatasetFromBytes(source)
	if err != nil {
		return NormalizedCorpus{}, err
	}
	return NewLoCoMoAdapter().Normalize(dataset, scope)
}
