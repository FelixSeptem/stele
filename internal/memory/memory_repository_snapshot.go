package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"golang.org/x/text/unicode/norm"
)

const (
	MemoryRepositoryFormatVersion = "memory-repo-v1"
	MemoryRepositoryParserVersion = "memory-repo-parser-v1"

	MemoryRepositoryMaxFiles         = 128
	MemoryRepositoryMaxFileBytes     = 64 * 1024
	MemoryRepositoryMaxContentBytes  = 1024 * 1024
	MemoryRepositoryMaxRequestBytes  = 2 * 1024 * 1024
	MemoryRepositoryMaxEntries       = 1000
	MemoryRepositoryMaxEntryBytes    = 8 * 1024
	MemoryRepositoryMaxPathBytes     = 256
	MemoryRepositoryMaxRevisionBytes = 256
	MemoryRepositoryMaxMetadataPairs = 16
	MemoryRepositoryMaxMetadataBytes = 2048
)

// MemoryRepositoryFile is one caller-supplied UTF-8 Markdown file. It is
// deliberately content-only: importers do not receive server filesystem paths
// or remote repository locations.
type MemoryRepositoryFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type MemoryRepositoryEntry struct {
	Path        string
	Line        int
	Occurrences []int
	Key         string
	Text        string
	Metadata    map[string]string
}
type MemoryRepositoryNavigation struct {
	Path   string
	Line   int
	Target string
}
type MemoryRepositoryDiagnostic struct {
	Path string
	Line int
	Code string
}
type MemoryRepositoryParseResult struct {
	Entries     []MemoryRepositoryEntry
	Navigation  []MemoryRepositoryNavigation
	Diagnostics []MemoryRepositoryDiagnostic
}

// ParseMemoryRepositorySnapshot parses the deliberately narrow import subset.
// It never reads paths, follows links, fetches URLs, or executes entry text.
func ParseMemoryRepositorySnapshot(snapshot MemoryRepositorySnapshot) (MemoryRepositoryParseResult, error) {
	if err := snapshot.Validate(); err != nil {
		return MemoryRepositoryParseResult{}, err
	}
	result := MemoryRepositoryParseResult{}
	knownFiles := make(map[string]struct{}, len(snapshot.Files))
	for _, file := range snapshot.Files {
		knownFiles[file.Path] = struct{}{}
	}
	entryIndexes := map[string]int{}
	navigationByPath := map[string][]MemoryRepositoryNavigation{}
	for _, file := range snapshot.Files {
		source := []byte(file.Content)
		document := goldmark.DefaultParser().Parse(text.NewReader(source))
		fencedLines := memoryRepositoryFencedLines(document, source)
		for index, line := range strings.Split(file.Content, "\n") {
			lineNumber := index + 1
			trimmed := strings.TrimSpace(line)
			if fencedLines[lineNumber] || trimmed == "" {
				continue
			}
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				if strings.HasPrefix(trimmed, "- ") {
					result.Diagnostics = append(result.Diagnostics, MemoryRepositoryDiagnostic{Path: file.Path, Line: lineNumber, Code: "nested_list_unsupported"})
				}
				continue
			}
			if !strings.HasPrefix(line, "- ") {
				continue
			}
			body := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			if target, isLink, valid := memoryRepositoryWikiLink(body); isLink {
				if !valid {
					result.Diagnostics = append(result.Diagnostics, MemoryRepositoryDiagnostic{Path: file.Path, Line: lineNumber, Code: "invalid_wiki_link"})
					continue
				}
				if _, found := knownFiles[target]; !found {
					result.Diagnostics = append(result.Diagnostics, MemoryRepositoryDiagnostic{Path: file.Path, Line: lineNumber, Code: "unresolved_wiki_link"})
					continue
				}
				navigation := MemoryRepositoryNavigation{Path: file.Path, Line: lineNumber, Target: target}
				result.Navigation = append(result.Navigation, navigation)
				navigationByPath[file.Path] = append(navigationByPath[file.Path], navigation)
				continue
			}
			entry, err := parseMemoryRepositoryEntry(file.Path, lineNumber, body)
			if err != nil {
				return MemoryRepositoryParseResult{}, err
			}
			if existing, found := entryIndexes[entry.Key]; found {
				result.Entries[existing].Occurrences = append(result.Entries[existing].Occurrences, lineNumber)
				continue
			}
			entry.Occurrences = []int{lineNumber}
			entryIndexes[entry.Key] = len(result.Entries)
			result.Entries = append(result.Entries, entry)
			if len(result.Entries) > MemoryRepositoryMaxEntries {
				return MemoryRepositoryParseResult{}, fmt.Errorf("memory repository exceeds %d entries", MemoryRepositoryMaxEntries)
			}
		}
	}
	for _, path := range memoryRepositoryCyclicPaths(navigationByPath) {
		for _, navigation := range navigationByPath[path] {
			if memoryRepositoryPathReaches(navigation.Target, path, navigationByPath, map[string]bool{}) {
				result.Diagnostics = append(result.Diagnostics, MemoryRepositoryDiagnostic{Path: navigation.Path, Line: navigation.Line, Code: "wiki_link_cycle"})
			}
		}
	}
	return result, nil
}

func memoryRepositoryWikiLink(value string) (target string, isLink, valid bool) {
	if !strings.HasPrefix(value, "[[") || !strings.HasSuffix(value, "]]") {
		return "", false, false
	}
	if strings.Count(value, "[[") != 1 || strings.Count(value, "]]") != 1 {
		return "", true, false
	}
	target = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "[["), "]]"))
	if target == "" || strings.ContainsAny(target, "#\\") || strings.Contains(target, "..") {
		return "", true, false
	}
	filename := target[strings.LastIndex(target, "/")+1:]
	if strings.Contains(filename, ".") && !strings.HasSuffix(target, ".md") {
		return "", true, false
	}
	if !strings.HasSuffix(target, ".md") {
		target += ".md"
	}
	if err := validateMemoryRepositoryPath(target); err != nil {
		return "", true, false
	}
	return target, true, true
}

func memoryRepositoryFencedLines(document ast.Node, source []byte) map[int]bool {
	lines := map[int]bool{}
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if fenced, ok := node.(*ast.FencedCodeBlock); ok {
			for i := 0; i < fenced.Lines().Len(); i++ {
				segment := fenced.Lines().At(i)
				lines[1+strings.Count(string(source[:segment.Start]), "\n")] = true
			}
		}
		return ast.WalkContinue, nil
	})
	return lines
}

func memoryRepositoryCyclicPaths(navigation map[string][]MemoryRepositoryNavigation) []string {
	paths := make([]string, 0, len(navigation))
	for path := range navigation {
		if memoryRepositoryPathReaches(path, path, navigation, map[string]bool{}) {
			paths = append(paths, path)
		}
	}
	return paths
}

func memoryRepositoryPathReaches(current, target string, navigation map[string][]MemoryRepositoryNavigation, seen map[string]bool) bool {
	if seen[current] {
		return false
	}
	seen[current] = true
	for _, edge := range navigation[current] {
		if edge.Target == target || memoryRepositoryPathReaches(edge.Target, target, navigation, seen) {
			return true
		}
	}
	return false
}

func parseMemoryRepositoryEntry(path string, line int, body string) (MemoryRepositoryEntry, error) {
	metadata := map[string]string{}
	textValue := strings.TrimSpace(body)
	if strings.HasSuffix(textValue, "]") {
		if start := strings.LastIndex(textValue, " ["); start >= 0 {
			raw := textValue[start+2 : len(textValue)-1]
			textValue = strings.TrimSpace(textValue[:start])
			for _, pair := range strings.Split(raw, ";") {
				key, value, ok := strings.Cut(strings.TrimSpace(pair), ":")
				key, value = strings.TrimSpace(key), strings.TrimSpace(value)
				if !ok || key == "" || value == "" || len(value) > MemoryRepositoryMaxMetadataBytes {
					return MemoryRepositoryEntry{}, fmt.Errorf("memory repository metadata is invalid at %s:%d", path, line)
				}
				if _, duplicate := metadata[key]; duplicate {
					return MemoryRepositoryEntry{}, fmt.Errorf("memory repository metadata has duplicate key at %s:%d", path, line)
				}
				if key == "added" {
					if _, err := time.Parse("2006-01-02", value); err != nil {
						return MemoryRepositoryEntry{}, fmt.Errorf("memory repository added metadata is invalid at %s:%d", path, line)
					}
				}
				metadata[key] = value
				if len(metadata) > MemoryRepositoryMaxMetadataPairs {
					return MemoryRepositoryEntry{}, fmt.Errorf("memory repository metadata has too many pairs at %s:%d", path, line)
				}
			}
		}
	}
	if textValue == "" || len(textValue) > MemoryRepositoryMaxEntryBytes {
		return MemoryRepositoryEntry{}, fmt.Errorf("memory repository entry is invalid at %s:%d", path, line)
	}
	canonicalMetadata, err := json.Marshal(metadata)
	if err != nil {
		return MemoryRepositoryEntry{}, fmt.Errorf("encode memory repository metadata: %w", err)
	}
	digest := sha256.Sum256([]byte(path + "\x00" + textValue + "\x00" + string(canonicalMetadata)))
	return MemoryRepositoryEntry{Path: path, Line: line, Key: hex.EncodeToString(digest[:]), Text: textValue, Metadata: metadata}, nil
}

// MemoryRepositorySnapshot is a bounded, caller-supplied source observation.
// Revision is an opaque claim and has no ordering or trust semantics.
type MemoryRepositorySnapshot struct {
	FormatVersion string                 `json:"format_version"`
	Revision      string                 `json:"source_revision"`
	Files         []MemoryRepositoryFile `json:"files"`
}

func (s MemoryRepositorySnapshot) Validate() error {
	if s.FormatVersion != MemoryRepositoryFormatVersion {
		return fmt.Errorf("memory repository format version must be %q", MemoryRepositoryFormatVersion)
	}
	if !utf8.ValidString(s.Revision) || strings.TrimSpace(s.Revision) == "" || len(s.Revision) > MemoryRepositoryMaxRevisionBytes {
		return fmt.Errorf("memory repository revision is required and must be at most %d bytes", MemoryRepositoryMaxRevisionBytes)
	}
	if len(s.Files) == 0 || len(s.Files) > MemoryRepositoryMaxFiles {
		return fmt.Errorf("memory repository must contain between 1 and %d files", MemoryRepositoryMaxFiles)
	}

	seen := make(map[string]struct{}, len(s.Files))
	totalBytes := 0
	hasManifest := false
	for _, file := range s.Files {
		if err := validateMemoryRepositoryPath(file.Path); err != nil {
			return err
		}
		if !utf8.ValidString(file.Content) || len(file.Content) > MemoryRepositoryMaxFileBytes {
			return fmt.Errorf("memory repository file %q content must be valid UTF-8 and at most %d bytes", file.Path, MemoryRepositoryMaxFileBytes)
		}
		totalBytes += len(file.Content)
		if totalBytes > MemoryRepositoryMaxContentBytes {
			return fmt.Errorf("memory repository content exceeds %d bytes", MemoryRepositoryMaxContentBytes)
		}
		if _, duplicate := seen[file.Path]; duplicate {
			return fmt.Errorf("memory repository contains duplicate file path %q", file.Path)
		}
		seen[file.Path] = struct{}{}
		if file.Path == "MEMORY.md" {
			hasManifest = true
		}
	}
	if !hasManifest {
		return fmt.Errorf("memory repository requires root MEMORY.md")
	}
	return nil
}

// CanonicalDigest binds the format version, opaque claimed revision, and exact
// file bytes. File order is not semantic; content and line endings are.
func (s MemoryRepositorySnapshot) CanonicalDigest() (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	files := append([]MemoryRepositoryFile(nil), s.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	payload := struct {
		FormatVersion string                 `json:"format_version"`
		Revision      string                 `json:"source_revision"`
		Files         []MemoryRepositoryFile `json:"files"`
	}{
		FormatVersion: s.FormatVersion,
		Revision:      s.Revision,
		Files:         files,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode memory repository snapshot: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validateMemoryRepositoryPath(path string) error {
	if !utf8.ValidString(path) || path == "" || len(path) > MemoryRepositoryMaxPathBytes || !norm.NFC.IsNormalString(path) {
		return fmt.Errorf("memory repository path is invalid")
	}
	if strings.HasPrefix(path, "/") || strings.Contains(path, "\\") || strings.Contains(path, "%") || (len(path) >= 2 && path[1] == ':') || !strings.HasSuffix(path, ".md") {
		return fmt.Errorf("memory repository path is invalid")
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("memory repository path is invalid")
		}
		for _, r := range segment {
			if unicode.IsControl(r) {
				return fmt.Errorf("memory repository path is invalid")
			}
		}
	}
	return nil
}
