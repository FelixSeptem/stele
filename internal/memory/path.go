package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// MemoryPathRoot is the persisted and public representation of an omitted path.
const MemoryPathRoot = "/"

const (
	MemoryPathMaxLength        = 512
	MemoryPathMaxSegmentLength = 128
)

type MemoryPathSelectorKind string

const (
	MemoryPathSelectorNone   MemoryPathSelectorKind = ""
	MemoryPathSelectorExact  MemoryPathSelectorKind = "path"
	MemoryPathSelectorPrefix MemoryPathSelectorKind = "path_prefix"
)

type MemoryPathSelector struct {
	Kind  MemoryPathSelectorKind
	Value string
}

func NormalizeMemoryPath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value == MemoryPathRoot {
		return MemoryPathRoot, nil
	}
	if len(value) > MemoryPathMaxLength {
		return "", fmt.Errorf("memory path must be at most %d bytes", MemoryPathMaxLength)
	}
	if strings.Contains(value, "\\") {
		return "", fmt.Errorf("memory path must use slash separators")
	}
	value = strings.Trim(value, "/")
	if value == "" {
		return MemoryPathRoot, nil
	}
	segments := strings.Split(value, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("memory path contains an invalid segment")
		}
		if len(segment) > MemoryPathMaxSegmentLength {
			return "", fmt.Errorf("memory path segment must be at most %d bytes", MemoryPathMaxSegmentLength)
		}
		for _, r := range segment {
			if unicode.IsControl(r) || r == '*' || r == '?' || r == '[' || r == ']' {
				return "", fmt.Errorf("memory path contains an invalid character")
			}
		}
		lower := strings.ToLower(segment)
		if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") {
			return "", fmt.Errorf("memory path contains an encoded separator")
		}
	}
	if len(value) > MemoryPathMaxLength {
		return "", fmt.Errorf("memory path must be at most %d bytes", MemoryPathMaxLength)
	}
	return value, nil
}

func NewMemoryPathSelector(path, prefix string) (MemoryPathSelector, error) {
	if strings.TrimSpace(path) != "" && strings.TrimSpace(prefix) != "" {
		return MemoryPathSelector{}, fmt.Errorf("path and path_prefix are mutually exclusive")
	}
	if strings.TrimSpace(path) != "" {
		value, err := NormalizeMemoryPath(path)
		if err != nil {
			return MemoryPathSelector{}, err
		}
		return MemoryPathSelector{Kind: MemoryPathSelectorExact, Value: value}, nil
	}
	if strings.TrimSpace(prefix) != "" {
		value, err := NormalizeMemoryPath(prefix)
		if err != nil {
			return MemoryPathSelector{}, err
		}
		return MemoryPathSelector{Kind: MemoryPathSelectorPrefix, Value: value}, nil
	}
	return MemoryPathSelector{Kind: MemoryPathSelectorNone, Value: ""}, nil
}

func (s MemoryPathSelector) Matches(path string) bool {
	normalized, err := NormalizeMemoryPath(path)
	if err != nil {
		return false
	}
	switch s.Kind {
	case MemoryPathSelectorNone:
		return true
	case MemoryPathSelectorExact:
		return normalized == s.Value
	case MemoryPathSelectorPrefix:
		return normalized == s.Value || (s.Value != MemoryPathRoot && strings.HasPrefix(normalized, s.Value+"/")) || (s.Value == MemoryPathRoot)
	default:
		return false
	}
}

func (s MemoryPathSelector) Fingerprint() string {
	return fingerprintPathPayload(struct {
		Kind  MemoryPathSelectorKind `json:"kind"`
		Value string                 `json:"value"`
	}{s.Kind, s.Value})
}

type MemoryCursor struct {
	Position string
	Selector MemoryPathSelector
}

func NewMemoryCursor(position string, selector MemoryPathSelector) MemoryCursor {
	return MemoryCursor{Position: position, Selector: selector}
}

func (c MemoryCursor) Fingerprint() string {
	return fingerprintPathPayload(struct {
		Position string             `json:"position"`
		Selector MemoryPathSelector `json:"selector"`
	}{c.Position, c.Selector})
}

func fingerprintPathPayload(value any) string {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
