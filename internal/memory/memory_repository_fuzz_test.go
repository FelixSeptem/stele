package memory

import "testing"

func FuzzParseMemoryRepositorySnapshot(f *testing.F) {
	f.Add("- ordinary fact\n- [[topic]]\n", "- linked fact\n")
	f.Add("```sh\n- ignored\n```\n", "- ordinary fact [added: 2026-10-16]\n")
	f.Fuzz(func(t *testing.T, manifest, topic string) {
		if len(manifest) > MemoryRepositoryMaxFileBytes || len(topic) > MemoryRepositoryMaxFileBytes {
			t.Skip()
		}
		result, err := ParseMemoryRepositorySnapshot(MemoryRepositorySnapshot{
			FormatVersion: MemoryRepositoryFormatVersion,
			Revision:      "fuzz",
			Files:         []MemoryRepositoryFile{{Path: "MEMORY.md", Content: manifest}, {Path: "topic.md", Content: topic}},
		})
		if err == nil && len(result.Entries) > MemoryRepositoryMaxEntries {
			t.Fatalf("entries=%d exceeds maximum", len(result.Entries))
		}
	})
}
