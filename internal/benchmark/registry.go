package benchmark

import (
	"fmt"
	"sort"

	"github.com/FelixSeptem/stele/internal/memory"
)

type DatasetRegistration struct {
	Layer              int                 `json:"layer"`
	Family             DatasetFamily       `json:"family"`
	Description        string              `json:"description"`
	LicenseStatus      LicenseStatus       `json:"license_status"`
	LocalPrerequisites []LocalPrerequisite `json:"local_prerequisites"`
	Manifest           DatasetManifest     `json:"manifest"`
}

type LicenseStatus string

const (
	LicenseReviewed    LicenseStatus = "reviewed"
	LicenseRestricted  LicenseStatus = "restricted"
	LicenseNeedsReview LicenseStatus = "needs_review"
)

type LocalPrerequisite struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type Registry struct {
	entries map[string]DatasetRegistration
}

type DatasetAdapter interface {
	NormalizeLocal(scope memory.Scope, source []byte) (NormalizedCorpus, error)
}

func DefaultRegistry() Registry {
	return Registry{entries: map[string]DatasetRegistration{
		"stele-fixture":      registration(0, FamilyAgentMemory, "Repository-owned lifecycle and isolation regression fixture", "stele-fixture", SupportRunnable, RedistributionPermitted, LicenseReviewed),
		"locomo":             registration(1, FamilyAgentMemory, "Long-term conversational memory benchmark", "locomo", SupportRunnable, RedistributionRestricted, LicenseRestricted),
		"longmemeval":        registration(2, FamilyAgentMemory, "Long-term memory update and conflict benchmark", "longmemeval", SupportMetadataOnly, RedistributionRestricted, LicenseRestricted),
		"bfcl":               registration(2, FamilyContract, "Offline memory-provider contract benchmark", "bfcl", SupportPlanned, RedistributionRestricted, LicenseNeedsReview),
		"multi-session-chat": registration(3, FamilySpecialized, "Cross-session conversation and profile benchmark", "multi-session-chat", SupportMetadataOnly, RedistributionRestricted, LicenseRestricted),
		"personachat":        registration(3, FamilySpecialized, "Persona and preference benchmark", "personachat", SupportMetadataOnly, RedistributionRestricted, LicenseRestricted),
		"hotpotqa":           registration(4, FamilySpecialized, "Multi-hop retrieval pressure benchmark", "hotpotqa", SupportMetadataOnly, RedistributionRestricted, LicenseRestricted),
		"timeqa":             registration(4, FamilySpecialized, "Temporal retrieval pressure benchmark", "timeqa", SupportMetadataOnly, RedistributionRestricted, LicenseRestricted),
		"beir":               registration(4, FamilyGenericRetrieval, "General information retrieval pressure benchmark", "beir", SupportMetadataOnly, RedistributionRestricted, LicenseRestricted),
		"mteb":               registration(4, FamilyGenericRetrieval, "C-MTEB/MTEB retrieval subset benchmark", "mteb", SupportPlanned, RedistributionRestricted, LicenseNeedsReview),
		"c-mteb":             registration(4, FamilyGenericRetrieval, "Chinese MTEB retrieval subset benchmark", "c-mteb", SupportPlanned, RedistributionRestricted, LicenseNeedsReview),
		"needle-haystack":    registration(5, FamilyStress, "Controlled Needle-in-a-Haystack stress benchmark", "needle-haystack", SupportPlanned, RedistributionPermitted, LicenseReviewed),
		"openai-mrcr":        registration(5, FamilyStress, "OpenAI MRCR long-context stress subset", "openai-mrcr", SupportMetadataOnly, RedistributionRestricted, LicenseRestricted),
		"longbench-v2":       registration(5, FamilyStress, "Long-context capacity benchmark", "longbench-v2", SupportMetadataOnly, RedistributionRestricted, LicenseRestricted),
		"vtcbench":           registration(5, FamilyStress, "Text and visual capability stress benchmark", "vtcbench", SupportMetadataOnly, RedistributionRestricted, LicenseRestricted),
	}}
}

func registration(layer int, family DatasetFamily, description, name string, support SupportState, redistribution RedistributionStatus, licenseStatus LicenseStatus) DatasetRegistration {
	return DatasetRegistration{
		Layer:              layer,
		Family:             family,
		Description:        description,
		LicenseStatus:      licenseStatus,
		LocalPrerequisites: []LocalPrerequisite{{ID: "manifest", Description: "checksum-locked local dataset manifest", Required: true}, {ID: "raw-data", Description: "user-provided local raw dataset", Required: true}},
		Manifest: DatasetManifest{
			SchemaVersion:     SchemaVersion,
			Family:            family,
			Name:              name,
			Version:           "unlocked",
			License:           "verify-upstream-license-before-fetch",
			UpstreamURL:       "https://example.invalid/" + name,
			UpstreamRevision:  "unlocked",
			SHA256:            "0000000000000000000000000000000000000000000000000000000000000000",
			QRELChecksum:      "0000000000000000000000000000000000000000000000000000000000000000",
			SourcePath:        "user-provided",
			ConversionVersion: "v1",
			Redistribution:    redistribution,
			Support:           support,
			Splits:            map[string]SplitSpec{"smoke": {Identity: name + "/smoke", Source: "smoke"}, "full": {Identity: name + "/full", Source: "full"}},
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

// LongMemEvalAdapter exposes the subset-aware adapter without weakening the
// generic DatasetAdapter contract. Callers must still explicitly select s, m,
// or oracle when loading their locked local artifact.
func (r Registry) LongMemEvalAdapter() (LongMemEvalAdapter, error) {
	entry, ok := r.Get("longmemeval")
	if !ok || entry.Family != FamilyAgentMemory {
		return LongMemEvalAdapter{}, &StatusError{Status: StatusPrerequisiteMissing, Message: "LongMemEval benchmark dataset is not registered"}
	}
	return NewLongMemEvalAdapter(), nil
}

// LongMemEvalDatasetAdapter enables an explicitly selected, checksum-locked
// LongMemEval subset without changing the generic metadata-only guard used by
// Registry.Adapter. The caller remains responsible for declaring the same
// split in its local manifest before normalized output can be retained.
func (r Registry) LongMemEvalDatasetAdapter(subset LongMemEvalSubset) (DatasetAdapter, error) {
	if _, err := r.LongMemEvalAdapter(); err != nil {
		return nil, err
	}
	if err := subset.Validate(); err != nil {
		return nil, &StatusError{Status: StatusInvalidManifest, Message: err.Error(), Cause: err}
	}
	return longMemEvalRegistryAdapter{subset: subset}, nil
}

type loCoMoRegistryAdapter struct{}

func (loCoMoRegistryAdapter) NormalizeLocal(scope memory.Scope, source []byte) (NormalizedCorpus, error) {
	dataset, err := LoadLoCoMoDatasetFromBytes(source)
	if err != nil {
		return NormalizedCorpus{}, err
	}
	return NewLoCoMoAdapter().Normalize(dataset, scope)
}

type longMemEvalRegistryAdapter struct {
	subset LongMemEvalSubset
}

func (a longMemEvalRegistryAdapter) NormalizeLocal(scope memory.Scope, source []byte) (NormalizedCorpus, error) {
	dataset, err := LoadLongMemEvalDatasetFromBytes(source, a.subset)
	if err != nil {
		return NormalizedCorpus{}, err
	}
	return NewLongMemEvalAdapter().Normalize(dataset, scope)
}
