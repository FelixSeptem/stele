package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type MemoryRepositorySelection struct {
	Path       string
	EntryKey   string
	Class      MemoryClass
	MemoryPath string
}

// MemoryRepositoryRecord is scoped source registration state. SourceURL is an
// opaque, untrusted display label; it is never a fetch instruction or grant.
type MemoryRepositoryRecord struct {
	ID         string
	Scope      Scope
	Name       string
	SourceURL  string
	Generation int64
	CreatedAt  time.Time
}

func (r MemoryRepositoryRecord) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.Name) == "" || len(r.Name) > 256 || len(r.SourceURL) > 2048 || r.Generation < 0 {
		return fmt.Errorf("memory repository registration is invalid")
	}
	return nil
}

type MemoryRepositoryPreviewRequest struct {
	RepositoryID       string
	Scope              Scope
	Actor              string
	Reason             string
	ExpectedGeneration int64
	Snapshot           MemoryRepositorySnapshot
	Selections         []MemoryRepositorySelection
	SourceMetadata     SourceMetadata
}

type MemoryRepositoryPreviewEntry struct {
	EntryKey     string
	Path         string
	Occurrences  []int
	Text         string
	Metadata     map[string]string
	Class        MemoryClass
	MemoryPath   string
	SourceDigest string
}

type MemoryRepositoryPreview struct {
	ID              string
	RepositoryID    string
	Scope           Scope
	Generation      int64
	Revision        string
	Actor           string
	Reason          string
	SnapshotDigest  string
	PlanDigest      string
	WarningDigest   string
	ParserVersion   string
	ExpiresAt       time.Time
	CreatedAt       time.Time
	EffectiveSource SourceMetadata
	Entries         []MemoryRepositoryPreviewEntry
	Navigation      []MemoryRepositoryNavigation
	Diagnostics     []MemoryRepositoryDiagnostic
}

type MemoryRepositoryImportStatus string

const (
	MemoryRepositoryImportQueued    MemoryRepositoryImportStatus = "queued"
	MemoryRepositoryImportRunning   MemoryRepositoryImportStatus = "running"
	MemoryRepositoryImportCompleted MemoryRepositoryImportStatus = "completed"
)

func (s MemoryRepositoryImportStatus) Valid() bool {
	return s == MemoryRepositoryImportQueued || s == MemoryRepositoryImportRunning || s == MemoryRepositoryImportCompleted
}

type MemoryRepositoryEntryDispatchStatus string

const (
	MemoryRepositoryEntryQueued    MemoryRepositoryEntryDispatchStatus = "queued"
	MemoryRepositoryEntrySubmitted MemoryRepositoryEntryDispatchStatus = "submitted"
	MemoryRepositoryEntryDuplicate MemoryRepositoryEntryDispatchStatus = "duplicate"
	MemoryRepositoryEntryRejected  MemoryRepositoryEntryDispatchStatus = "rejected"
	MemoryRepositoryEntryFailed    MemoryRepositoryEntryDispatchStatus = "failed"
)

func (s MemoryRepositoryEntryDispatchStatus) Valid() bool {
	return s == MemoryRepositoryEntryQueued || s == MemoryRepositoryEntrySubmitted || s == MemoryRepositoryEntryDuplicate || s == MemoryRepositoryEntryRejected || s == MemoryRepositoryEntryFailed
}

// MemoryRepositoryChildIdempotencyKey ties an entry dispatch to an accepted
// import. A crash between intent submission and local acknowledgement can
// replay this exact child key without creating new evidence.
func MemoryRepositoryChildIdempotencyKey(importID, entryKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(importID) + "\x00" + strings.TrimSpace(entryKey)))
	return "memory-repository:" + hex.EncodeToString(sum[:])
}

// BuildMemoryRepositoryPreview is pure review planning. It neither executes
// supplied text nor creates intents, canonical memory, filesystem, or network
// effects.
func BuildMemoryRepositoryPreview(request MemoryRepositoryPreviewRequest, now time.Time) (MemoryRepositoryPreview, error) {
	if err := request.Scope.Validate(); err != nil {
		return MemoryRepositoryPreview{}, err
	}
	if strings.TrimSpace(request.RepositoryID) == "" || strings.TrimSpace(request.Actor) == "" || strings.TrimSpace(request.Reason) == "" || request.ExpectedGeneration < 0 {
		return MemoryRepositoryPreview{}, fmt.Errorf("memory repository preview identity, actor, reason, and generation are required")
	}
	if len(request.Selections) == 0 || len(request.Selections) > MemoryRepositoryMaxEntries {
		return MemoryRepositoryPreview{}, fmt.Errorf("memory repository preview requires bounded explicit selections")
	}
	parsed, err := ParseMemoryRepositorySnapshot(request.Snapshot)
	if err != nil {
		return MemoryRepositoryPreview{}, err
	}
	snapshotDigest, err := request.Snapshot.CanonicalDigest()
	if err != nil {
		return MemoryRepositoryPreview{}, err
	}
	entries := make(map[string]MemoryRepositoryEntry, len(parsed.Entries))
	for _, entry := range parsed.Entries {
		entries[entry.Key] = entry
	}
	effective := request.SourceMetadata.Effective()
	planned := make([]MemoryRepositoryPreviewEntry, 0, len(request.Selections))
	seen := map[string]struct{}{}
	for _, selection := range request.Selections {
		entry, found := entries[selection.EntryKey]
		if !found || entry.Path != selection.Path {
			return MemoryRepositoryPreview{}, fmt.Errorf("memory repository selection does not reference a parsed entry")
		}
		if !selection.Class.Valid() {
			return MemoryRepositoryPreview{}, fmt.Errorf("memory repository selection has invalid memory class")
		}
		memoryPath, err := NormalizeMemoryPath(selection.MemoryPath)
		if err != nil {
			return MemoryRepositoryPreview{}, fmt.Errorf("memory repository selection memory path: %w", err)
		}
		identity := selection.EntryKey + "\x00" + string(selection.Class) + "\x00" + memoryPath
		if _, duplicate := seen[identity]; duplicate {
			return MemoryRepositoryPreview{}, fmt.Errorf("memory repository selection is duplicated")
		}
		seen[identity] = struct{}{}
		entryIdentity, err := memoryRepositoryEntryIdentity(request.RepositoryID, entry, selection.Class, memoryPath, effective)
		if err != nil {
			return MemoryRepositoryPreview{}, err
		}
		planned = append(planned, MemoryRepositoryPreviewEntry{EntryKey: entryIdentity, Path: entry.Path, Occurrences: append([]int(nil), entry.Occurrences...), Text: entry.Text, Metadata: cloneMemoryRepositoryMetadata(entry.Metadata), Class: selection.Class, MemoryPath: memoryPath, SourceDigest: snapshotDigest})
	}
	sortMemoryRepositoryPreviewEntries(planned)
	planDigest, err := memoryRepositoryHash([]any{request.RepositoryID, request.Scope.Normalized(), request.Actor, request.Reason, request.ExpectedGeneration, snapshotDigest, effective, planned})
	if err != nil {
		return MemoryRepositoryPreview{}, err
	}
	warningDigest, err := memoryRepositoryHash(parsed.Diagnostics)
	if err != nil {
		return MemoryRepositoryPreview{}, err
	}
	return MemoryRepositoryPreview{RepositoryID: request.RepositoryID, Scope: request.Scope.Normalized(), Generation: request.ExpectedGeneration, Revision: request.Snapshot.Revision, Actor: strings.TrimSpace(request.Actor), Reason: strings.TrimSpace(request.Reason), SnapshotDigest: snapshotDigest, PlanDigest: planDigest, WarningDigest: warningDigest, ParserVersion: MemoryRepositoryParserVersion, ExpiresAt: now.UTC().Add(24 * time.Hour), CreatedAt: now.UTC(), EffectiveSource: effective, Entries: planned, Navigation: append([]MemoryRepositoryNavigation(nil), parsed.Navigation...), Diagnostics: append([]MemoryRepositoryDiagnostic(nil), parsed.Diagnostics...)}, nil
}

// memoryRepositoryEntryIdentity is the stable deduplication identity used by
// durable import state. It intentionally excludes source revision and line
// location while binding the repository, source fact fingerprint, selected
// class, destination path, and effective source labels.
func memoryRepositoryEntryIdentity(repositoryID string, entry MemoryRepositoryEntry, class MemoryClass, memoryPath string, source SourceMetadata) (string, error) {
	return memoryRepositoryHash([]any{strings.TrimSpace(repositoryID), entry.Path, entry.Key, class, memoryPath, source})
}

func (c MemoryClass) Valid() bool {
	switch c {
	case MemoryClassProfile, MemoryClassEpisodic, MemoryClassProcedural, MemoryClassSummary, MemoryClassRelation:
		return true
	default:
		return false
	}
}

func memoryRepositoryHash(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func cloneMemoryRepositoryMetadata(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func sortMemoryRepositoryPreviewEntries(entries []MemoryRepositoryPreviewEntry) {
	for index := 1; index < len(entries); index++ {
		for previous := index; previous > 0 && entries[previous].EntryKey < entries[previous-1].EntryKey; previous-- {
			entries[previous], entries[previous-1] = entries[previous-1], entries[previous]
		}
	}
}

type MemoryRepositoryImportRequest struct {
	RepositoryID       string
	PreviewID          string
	Scope              Scope
	PlanDigest         string
	WarningDigest      string
	ExpectedGeneration int64
	IdempotencyKey     string
}

var (
	ErrMemoryRepositoryStaleGeneration     = fmt.Errorf("memory repository generation is stale")
	ErrMemoryRepositoryPreviewExpired      = fmt.Errorf("memory repository preview is expired")
	ErrMemoryRepositoryRevisionConflict    = fmt.Errorf("memory repository revision is already bound to different content")
	ErrMemoryRepositoryIdempotencyConflict = fmt.Errorf("memory repository idempotency key conflicts with an existing import")
)

type MemoryRepositoryImportEntry struct {
	EntryKey       string
	DispatchStatus MemoryRepositoryEntryDispatchStatus
	IntentID       string
	Payload        MemoryRepositoryPreviewEntry
}

type MemoryRepositoryImportRecord struct {
	ID                 string
	RepositoryID       string
	Scope              Scope
	PreviewID          string
	Generation         int64
	IdempotencyKey     string
	RequestFingerprint string
	Status             MemoryRepositoryImportStatus
	CreatedAt          time.Time
	CompletedAt        time.Time
	Entries            []MemoryRepositoryImportEntry
}

// MemoryRepositoryManifest is navigation only. It is intentionally distinct
// from the retained preview/import payload so it cannot become an alternate
// content-retrieval channel.
type MemoryRepositoryManifest struct {
	RepositoryID     string
	Generation       int64
	ProcessingStatus MemoryRepositoryImportStatus
	Items            []MemoryRepositoryManifestItem
}

type MemoryRepositoryManifestItem struct {
	MemoryID   string
	Version    int64
	SourcePath string
}

func (r MemoryRepositoryImportRecord) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.RepositoryID) == "" || strings.TrimSpace(r.PreviewID) == "" || r.Generation < 1 || !r.Status.Valid() || !memoryRepositoryDigest(r.RequestFingerprint) {
		return fmt.Errorf("memory repository import record is invalid")
	}
	return nil
}

func (r MemoryRepositoryImportRequest) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.RepositoryID) == "" || strings.TrimSpace(r.PreviewID) == "" {
		return fmt.Errorf("repository and preview identity are required")
	}
	if r.ExpectedGeneration < 0 {
		return fmt.Errorf("expected generation cannot be negative")
	}
	if !memoryRepositoryDigest(r.PlanDigest) || !memoryRepositoryDigest(r.WarningDigest) {
		return fmt.Errorf("plan and warning digests must be SHA-256 hex")
	}
	if strings.TrimSpace(r.IdempotencyKey) == "" || len(r.IdempotencyKey) > 256 {
		return fmt.Errorf("idempotency key is required and bounded")
	}
	return nil
}

func memoryRepositoryDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
