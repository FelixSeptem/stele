package benchmark

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/FelixSeptem/stele/internal/memory"
)

// GenericRetrievalSource is a locked local corpus and qrels input. It is kept
// separate from conversational adapters so generic IR metrics cannot silently
// be interpreted as agent-memory quality.
type GenericRetrievalSource struct {
	CorpusIdentity string                     `json:"corpus_identity"`
	QRELIdentity   string                     `json:"qrels_identity"`
	Documents      []GenericRetrievalDocument `json:"documents"`
	Queries        []GenericRetrievalQuery    `json:"queries"`
	QRELs          []GenericRetrievalQREL     `json:"qrels"`
}

type GenericRetrievalDocument struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type GenericRetrievalQuery struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type GenericRetrievalQREL struct {
	QueryID    string `json:"query_id"`
	DocumentID string `json:"document_id"`
	Grade      int    `json:"grade"`
}

type GenericRetrievalCorpus struct {
	Family         DatasetFamily    `json:"family"`
	CorpusIdentity string           `json:"corpus_identity"`
	QRELIdentity   string           `json:"qrels_identity"`
	Corpus         NormalizedCorpus `json:"corpus"`
}

func NormalizeGenericRetrieval(source GenericRetrievalSource, scope memory.Scope) (GenericRetrievalCorpus, error) {
	if strings.TrimSpace(source.CorpusIdentity) == "" || strings.TrimSpace(source.QRELIdentity) == "" {
		return GenericRetrievalCorpus{}, errors.New("generic retrieval corpus and qrels identities are required")
	}
	if err := scope.Validate(); err != nil {
		return GenericRetrievalCorpus{}, fmt.Errorf("generic retrieval scope: %w", err)
	}
	corpus := NormalizedCorpus{SchemaVersion: SchemaVersion}
	for _, document := range source.Documents {
		corpus.Events = append(corpus.Events, MemoryEventRecord{ID: document.ID, Scope: scope, Class: memory.MemoryClassSummary, Text: document.Text, ExpectedState: memory.MemoryStateActive, Provenance: map[string]string{"family": string(FamilyGenericRetrieval), "corpus_identity": source.CorpusIdentity}})
	}
	for _, query := range source.Queries {
		corpus.Queries = append(corpus.Queries, BenchmarkQuery{ID: query.ID, Scope: scope, Text: query.Text, QueryType: "generic_retrieval"})
	}
	for _, qrel := range source.QRELs {
		corpus.QRELs = append(corpus.QRELs, QREL{QueryID: qrel.QueryID, EvidenceID: qrel.DocumentID, Grade: qrel.Grade, Role: "generic_retrieval"})
	}
	corpus = corpus.Canonical()
	if err := corpus.Validate(); err != nil {
		return GenericRetrievalCorpus{}, fmt.Errorf("validate generic retrieval corpus: %w", err)
	}
	return GenericRetrievalCorpus{Family: FamilyGenericRetrieval, CorpusIdentity: source.CorpusIdentity, QRELIdentity: source.QRELIdentity, Corpus: corpus}, nil
}

type StrategyProfile struct {
	ID                string            `json:"id"`
	Kind              RetrievalStrategy `json:"kind"`
	ChunkSize         int               `json:"chunk_size,omitempty"`
	CandidatePoolSize int               `json:"candidate_pool_size"`
	UseReranker       bool              `json:"use_reranker,omitempty"`
	RerankerIdentity  string            `json:"reranker_identity,omitempty"`
}

func DefaultGenericStrategyProfiles() []StrategyProfile {
	return []StrategyProfile{
		{ID: "lexical-v1", Kind: StrategyLexical, CandidatePoolSize: 100},
		{ID: "semantic-v1", Kind: StrategySemantic, CandidatePoolSize: 100},
		{ID: "hybrid-v1", Kind: StrategyHybrid, CandidatePoolSize: 100},
		{ID: "chunk-512-v1", Kind: StrategyChunk, ChunkSize: 512, CandidatePoolSize: 100},
		{ID: "hybrid-rank-v1", Kind: StrategyHybridRank, CandidatePoolSize: 100},
		{ID: "reranker-optional-v1", Kind: StrategyReranker, CandidatePoolSize: 100, UseReranker: true, RerankerIdentity: "operator-configured"},
	}
}

var (
	ErrIncompatibleGenericRun = errors.New("generic retrieval runs are not comparable")
	ErrGenericProductionScope = errors.New("generic retrieval runs must use benchmark scope")
)

type GenericRunIdentity struct {
	CorpusChecksum    string       `json:"corpus_checksum"`
	QRELChecksum      string       `json:"qrels_checksum"`
	EmbeddingIdentity string       `json:"embedding_identity"`
	Scope             memory.Scope `json:"scope"`
}

type GenericStrategyResult struct {
	Profile           StrategyProfile    `json:"profile"`
	Identity          GenericRunIdentity `json:"identity"`
	CandidatePoolSize int                `json:"candidate_pool_size"`
	Report            EvaluationReport   `json:"report"`
}

func (r GenericStrategyResult) Validate() error {
	if strings.TrimSpace(r.Profile.ID) == "" || r.Profile.Kind == "" {
		return fmt.Errorf("generic strategy profile is required: %w", ErrIncompatibleGenericRun)
	}
	if strings.TrimSpace(r.Identity.CorpusChecksum) == "" || strings.TrimSpace(r.Identity.QRELChecksum) == "" || strings.TrimSpace(r.Identity.EmbeddingIdentity) == "" {
		return fmt.Errorf("generic run corpus, qrels, and embedding identities are required: %w", ErrIncompatibleGenericRun)
	}
	if err := r.Identity.Scope.Validate(); err != nil {
		return fmt.Errorf("generic run scope: %w", err)
	}
	if !strings.HasPrefix(r.Identity.Scope.Project, "benchmark") || (!strings.HasPrefix(r.Identity.Scope.Namespace, "run-") && !strings.HasPrefix(r.Identity.Scope.Namespace, "generic-")) {
		return ErrGenericProductionScope
	}
	return nil
}

type GenericStrategyComparison struct {
	CorpusChecksum string                `json:"corpus_checksum"`
	QRELChecksum   string                `json:"qrels_checksum"`
	Left           GenericStrategyResult `json:"left"`
	Right          GenericStrategyResult `json:"right"`
}

func CompareGenericStrategies(left, right GenericStrategyResult) (GenericStrategyComparison, error) {
	if err := left.Validate(); err != nil {
		return GenericStrategyComparison{}, err
	}
	if err := right.Validate(); err != nil {
		return GenericStrategyComparison{}, err
	}
	if left.Identity.CorpusChecksum != right.Identity.CorpusChecksum || left.Identity.QRELChecksum != right.Identity.QRELChecksum || left.Identity.EmbeddingIdentity != right.Identity.EmbeddingIdentity || left.Identity.Scope != right.Identity.Scope {
		return GenericStrategyComparison{}, ErrIncompatibleGenericRun
	}
	return GenericStrategyComparison{CorpusChecksum: left.Identity.CorpusChecksum, QRELChecksum: left.Identity.QRELChecksum, Left: left, Right: right}, nil
}

// SortGenericResults makes persisted strategy reports reproducible when the
// caller compares more than two strategy profiles.
func SortGenericResults(results []GenericStrategyResult) {
	sort.SliceStable(results, func(i, j int) bool { return results[i].Profile.ID < results[j].Profile.ID })
}
