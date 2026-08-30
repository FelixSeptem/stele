package benchmark

import (
	"fmt"
	"sort"

	"github.com/FelixSeptem/stele/internal/memory"
)

type DatasetRegistration struct {
	Layer       int             `json:"layer"`
	Description string          `json:"description"`
	Manifest    DatasetManifest `json:"manifest"`
}

type Registry struct {
	entries map[string]DatasetRegistration
}

type DatasetAdapter interface {
	NormalizeLocal(scope memory.Scope, source []byte) (NormalizedCorpus, error)
}

func DefaultRegistry() Registry {
	return Registry{entries: map[string]DatasetRegistration{
		"stele-fixture":      registration(0, "Repository-owned lifecycle and isolation regression fixture", "stele-fixture", SupportRunnable, RedistributionPermitted),
		"locomo":             registration(1, "Long-term conversational memory benchmark", "locomo", SupportRunnable, RedistributionRestricted),
		"longmemeval":        registration(2, "Long-term memory update and conflict benchmark", "longmemeval", SupportMetadataOnly, RedistributionRestricted),
		"multi-session-chat": registration(3, "Cross-session conversation and profile benchmark", "multi-session-chat", SupportMetadataOnly, RedistributionRestricted),
		"personachat":        registration(3, "Persona and preference benchmark", "personachat", SupportMetadataOnly, RedistributionRestricted),
		"hotpotqa":           registration(4, "Multi-hop retrieval pressure benchmark", "hotpotqa", SupportMetadataOnly, RedistributionRestricted),
		"timeqa":             registration(4, "Temporal retrieval pressure benchmark", "timeqa", SupportMetadataOnly, RedistributionRestricted),
		"beir":               registration(4, "General information retrieval pressure benchmark", "beir", SupportMetadataOnly, RedistributionRestricted),
	}}
}

func registration(layer int, description, name string, support SupportState, redistribution RedistributionStatus) DatasetRegistration {
	return DatasetRegistration{
		Layer:       layer,
		Description: description,
		Manifest: DatasetManifest{
			SchemaVersion:     SchemaVersion,
			Name:              name,
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
	entry, ok := r.Get(name)
	if !ok {
		return nil, &StatusError{Status: StatusPrerequisiteMissing, Message: "benchmark dataset is not registered"}
	}
	if entry.Manifest.Support != SupportRunnable {
		return nil, &StatusError{Status: StatusPrerequisiteMissing, Message: fmt.Sprintf("benchmark dataset %s is %s", name, entry.Manifest.Support)}
	}
	switch name {
	case "locomo":
		return loCoMoRegistryAdapter{}, nil
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
