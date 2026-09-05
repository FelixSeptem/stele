package benchmark

import (
	"errors"
	"fmt"
	"strings"
)

// GenericDatasetLock is the repository-owned metadata lock for a small
// external generic-retrieval input. The data itself remains in the user's
// local benchmark cache; only this audit metadata is checked in.
type GenericDatasetLock struct {
	Dataset                  string               `json:"dataset"`
	Family                   DatasetFamily        `json:"family"`
	UpstreamURL              string               `json:"upstream_url"`
	UpstreamRevision         string               `json:"upstream_revision"`
	License                  string               `json:"license"`
	LicenseStatus            LicenseStatus        `json:"license_status"`
	Language                 string               `json:"language"`
	Subset                   string               `json:"subset"`
	CorpusRecords            int                  `json:"corpus_records"`
	QueryRecords             int                  `json:"query_records"`
	QRELRecords              int                  `json:"qrel_records"`
	RawBytes                 int64                `json:"raw_bytes"`
	EstimatedNormalizedBytes int64                `json:"estimated_normalized_bytes"`
	LocalStorageBudgetBytes  int64                `json:"local_storage_budget_bytes"`
	EmbeddingProfile         string               `json:"embedding_profile"`
	Redistribution           RedistributionStatus `json:"redistribution"`
	Support                  SupportState         `json:"support"`
	Files                    []DatasetLockFile    `json:"files"`
}

type DatasetLockFile struct {
	Path      string `json:"path"`
	Role      string `json:"role"`
	Revision  string `json:"revision"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

func (l GenericDatasetLock) Validate() error {
	for name, value := range map[string]string{
		"dataset": l.Dataset, "upstream_url": l.UpstreamURL,
		"upstream_revision": l.UpstreamRevision, "license": l.License,
		"language": l.Language, "subset": l.Subset,
		"embedding_profile": l.EmbeddingProfile,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if l.Family != FamilyGenericRetrieval {
		return fmt.Errorf("dataset lock family must be %q", FamilyGenericRetrieval)
	}
	if l.LicenseStatus == "" || l.Redistribution == "" || l.Support == "" {
		return errors.New("dataset lock license, redistribution, and support are required")
	}
	if l.CorpusRecords < 0 || l.QueryRecords < 0 || l.QRELRecords < 0 {
		return errors.New("dataset lock record counts cannot be negative")
	}
	if l.RawBytes <= 0 || l.EstimatedNormalizedBytes <= 0 || l.LocalStorageBudgetBytes < l.RawBytes {
		return errors.New("dataset lock storage budgets are invalid")
	}
	if len(l.Files) == 0 {
		return errors.New("dataset lock must list source files")
	}
	for _, file := range l.Files {
		if strings.TrimSpace(file.Path) == "" || strings.TrimSpace(file.Role) == "" || strings.TrimSpace(file.Revision) == "" {
			return errors.New("dataset lock file path, role, and revision are required")
		}
		if file.SizeBytes <= 0 || !sha256Pattern.MatchString(file.SHA256) {
			return fmt.Errorf("dataset lock file %q has invalid size or sha256", file.Path)
		}
	}
	return nil
}

func (l GenericDatasetLock) File(path string) (DatasetLockFile, bool) {
	for _, file := range l.Files {
		if file.Path == path {
			return file, true
		}
	}
	return DatasetLockFile{}, false
}
