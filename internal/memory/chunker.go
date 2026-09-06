package memory

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type ChunkingInput struct {
	Source  ChunkSourceReference
	Scope   Scope
	Class   MemoryClass
	Content string
	Policy  ChunkPolicy
}

func (i ChunkingInput) Validate() error {
	if err := i.Scope.Validate(); err != nil {
		return fmt.Errorf("chunk scope: %w", err)
	}
	if i.Scope.Normalized() != i.Source.Scope.Normalized() {
		return fmt.Errorf("chunk source scope does not match requested scope")
	}
	if err := i.Source.Validate(); err != nil {
		return err
	}
	if !validMemoryClass(i.Class) {
		return fmt.Errorf("invalid chunk memory class %q", i.Class)
	}
	if strings.TrimSpace(i.Content) == "" {
		return fmt.Errorf("chunk content is required")
	}
	if err := i.Policy.Validate(); err != nil {
		return err
	}
	return nil
}

type chunkUnit struct {
	start, end int
	text       string
}

// ChunkText deterministically segments source text at semantic boundaries and
// applies hard character/token bounds only when those boundaries cannot fit.
func ChunkText(input ChunkingInput) ([]MemoryChunk, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	cp, _ := input.Policy.ClassPolicy(input.Class)
	units := boundaryUnits(input.Content)
	var chunks []MemoryChunk
	for _, unit := range units {
		if runeLen(unit.text) > cp.MaxCharacters || countChunkTokens(unit.text) > cp.MaxTokens {
			for _, fragment := range hardSplitUnit(unit, cp) {
				chunks = append(chunks, makeChunk(input, fragment, len(chunks)))
			}
			continue
		}
		chunks = append(chunks, makeChunk(input, unit, len(chunks)))
	}
	return chunks, nil
}

func makeChunk(input ChunkingInput, unit chunkUnit, ordinal int) MemoryChunk {
	c := MemoryChunk{Scope: input.Scope.Normalized(), Source: input.Source, Class: input.Class, Ordinal: ordinal, Content: unit.text, SourceRange: ChunkRange{Start: unit.start, End: unit.end}, CharacterCount: runeLen(unit.text), TokenCount: countChunkTokens(unit.text), LifecycleState: MemoryStateActive, PolicyVersion: input.Policy.Version, RendererVersion: input.Policy.RendererVersion}
	c.ID = chunkIdentity(c.Source, c.PolicyVersion, c.RendererVersion, c.Ordinal, c.SourceRange, c.Content)
	return c
}

func boundaryUnits(content string) []chunkUnit {
	var units []chunkUnit
	start := 0
	for start < len(content) {
		if strings.HasPrefix(content[start:], "```") {
			end := len(content)
			if closing := strings.Index(content[start+3:], "\n```"); closing >= 0 {
				end = start + 3 + closing + len("\n```")
			}
			units = appendTrimmedUnit(units, content, start, end, false)
			start = end
			if strings.HasPrefix(content[start:], "\n\n") {
				start += 2
			}
			continue
		}
		end := len(content)
		if idx := strings.Index(content[start:], "\n\n"); idx >= 0 {
			end = start + idx
		}
		units = appendTrimmedUnit(units, content, start, end, true)
		if end == len(content) {
			break
		}
		start = end + 2
	}
	return units
}

func appendTrimmedUnit(units []chunkUnit, source string, start, end int, sentenceBoundary bool) []chunkUnit {
	part := strings.TrimSpace(source[start:end])
	if part == "" {
		return units
	}
	off := strings.Index(source[start:end], part)
	unitStart := start + off
	if isListBlock(part) || !sentenceBoundary {
		return append(units, chunkUnit{start: unitStart, end: unitStart + len(part), text: part})
	}
	for lineStart := 0; lineStart < len(part); {
		lineEnd := len(part)
		if next := strings.IndexByte(part[lineStart:], '\n'); next >= 0 {
			lineEnd = lineStart + next
		}
		line := strings.TrimSpace(part[lineStart:lineEnd])
		if line != "" {
			lineOff := strings.Index(part[lineStart:lineEnd], line)
			units = append(units, sentenceUnits(line, unitStart+lineStart+lineOff)...)
		}
		if lineEnd == len(part) {
			break
		}
		lineStart = lineEnd + 1
	}
	return units
}

func isListBlock(s string) bool {
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return false
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !(strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || (len(line) >= 3 && line[0] >= '0' && line[0] <= '9' && strings.Contains(line[:3], "."))) {
			return false
		}
	}
	return true
}

func sentenceUnits(text string, base int) []chunkUnit {
	var out []chunkUnit
	start := 0
	for i, r := range text {
		if strings.ContainsRune(".!?。！？", r) {
			end := i + utf8.RuneLen(r)
			for end < len(text) && (text[end] == ' ' || text[end] == '\t' || text[end] == '\n') {
				end++
			}
			part := strings.TrimSpace(text[start:end])
			if part != "" {
				off := strings.Index(text[start:end], part)
				out = append(out, chunkUnit{start: base + start + off, end: base + start + off + len(part), text: part})
			}
			start = end
		}
	}
	if part := strings.TrimSpace(text[start:]); part != "" {
		off := strings.Index(text[start:], part)
		out = append(out, chunkUnit{start: base + start + off, end: base + start + off + len(part), text: part})
	}
	return out
}

func hardSplitUnit(unit chunkUnit, policy ChunkClassPolicy) []chunkUnit {
	max := policy.MaxCharacters
	if max <= 0 {
		max = 1
	}
	runes := []rune(unit.text)
	var out []chunkUnit
	for pos := 0; pos < len(runes); {
		end := pos + max
		if end > len(runes) {
			end = len(runes)
		}
		for end > pos+1 && countChunkTokens(string(runes[pos:end])) > policy.MaxTokens {
			end--
		}
		if end <= pos {
			end = pos + 1
		}
		text := string(runes[pos:end])
		prefix := string(runes[:pos])
		start := unit.start + len([]byte(prefix))
		out = append(out, chunkUnit{start: start, end: start + len([]byte(text)), text: text})
		pos = end
	}
	return out
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }
