# Memory repository Markdown parser decision

## Decision

The `memory-repo-v1` snapshot importer uses
[`github.com/yuin/goldmark`](https://pkg.go.dev/github.com/yuin/goldmark) v1.8.6
for bounded CommonMark AST traversal. Stele will not render Markdown, enable
extensions, execute content, fetch links, or treat parsed text as instructions.
The application accepts only the versioned subset defined in the OpenSpec
change: top-level single-line bullets and root-relative wiki links outside code
fences.

## Package review

The package pages were reviewed on pkg.go.dev on 2026-10-16.

| Candidate | Assessment | Decision |
| --- | --- | --- |
| `github.com/yuin/goldmark` v1.8.6 | Maintained Go CommonMark parser with a public AST that distinguishes list nesting and fenced code blocks. Its parser can be used without an HTML renderer. | Selected. |
| `github.com/russross/blackfriday/v2` v2.1.0 | A mature parser, but its latest stable line is materially older and the first implementation needs explicit AST traversal boundaries. | Not selected. |
| Hand-written line parser | Could recognize a tiny grammar, but would need to reproduce code-fence and list-context behavior that an established parser already supplies. | Not selected. |

## Constraints and verification

Goldmark is a syntax classifier, not a policy engine. Stele still applies its
own UTF-8, size, NFC path, metadata, link-target, scope, provenance, and
lifecycle rules. A parser-version field is included in every preview/import
plan so behavior remains attributable. Fixtures must verify that code fences,
nested lists, unsupported prose, malformed metadata, and wiki links cannot
silently become importable entries. Parser fuzz tests must demonstrate bounded
operation and no filesystem, network, or command side effects.
