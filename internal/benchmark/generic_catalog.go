package benchmark

// GenericSubsetLock records the intentionally small, local-first subset
// selection used for strategy regression. It is metadata only; users fetch
// the upstream artifacts separately and lock their actual checksums in a
// DatasetManifest before execution.
type GenericSubsetLock struct {
	Dataset         string `json:"dataset"`
	Subset          string `json:"subset"`
	License         string `json:"license"`
	Language        string `json:"language"`
	CorpusSize      int    `json:"corpus_size"`
	StorageBudgetMB int    `json:"storage_budget_mb"`
}

func LockedGenericSubsetCatalog() []GenericSubsetLock {
	return []GenericSubsetLock{
		{Dataset: "c-mteb", Subset: "T2Retrieval", License: "upstream-task-license-review-required", Language: "zh", CorpusSize: 1000, StorageBudgetMB: 512},
		{Dataset: "mteb", Subset: "T2Retrieval", License: "upstream-task-license-review-required", Language: "en", CorpusSize: 1000, StorageBudgetMB: 512},
		{Dataset: "beir", Subset: "nfcorpus", License: "Apache-2.0-or-upstream-review", Language: "en", CorpusSize: 3633, StorageBudgetMB: 1024},
	}
}
