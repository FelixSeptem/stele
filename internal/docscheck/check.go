// Package docscheck validates the repository's public documentation contracts
// without requiring network access or a running Stele instance.
package docscheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/FelixSeptem/stele/internal/mcp"
)

// Issue identifies one documentation consistency failure.
type Issue struct {
	Path    string
	Line    int
	Rule    string
	Message string
}

func (i Issue) Error() string {
	location := i.Path
	if i.Line > 0 {
		location = fmt.Sprintf("%s:%d", location, i.Line)
	}
	return fmt.Sprintf("%s [%s]: %s", location, i.Rule, i.Message)
}

// Check validates the documentation owned by this change. root must be the
// repository root, which makes the check usable from tests and scripts.
func Check(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	files := []string{
		"README.md",
		"docs/README.md",
		"docs/getting-started.md",
		"docs/architecture.md",
		"docs/components.md",
		"docs/best-practices.md",
		"docs/self-hosting.md",
		"docs/agent-runtime-memory-provider.md",
		"docs/integrations/mcp-agent-skill.md",
		"skills/stele-memory/SKILL.md",
	}

	contents := make(map[string]string, len(files))
	issues := make([]Issue, 0)
	for _, relative := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			issues = append(issues, Issue{Path: relative, Rule: "required-file", Message: readErr.Error()})
			continue
		}
		contents[relative] = string(data)
		issues = append(issues, localLinkIssues(root, relative, string(data))...)
		issues = append(issues, unsafeExampleIssues(relative, string(data))...)
	}
	issues = append(issues, requiredTextIssues("docs/README.md", "portal-navigation", contents["docs/README.md"], []string{
		"## 理解 Stele",
		"## 运行 Stele",
		"## 集成 Stele",
		"## 扩展与维护 Stele",
	})...)
	issues = append(issues, requiredTextIssues("docs/getting-started.md", "getting-started-contract", contents["docs/getting-started.md"], []string{
		"**角色：** 教程",
		"PostgreSQL 是唯一的记录系统",
		"## 选择下一步集成路径",
		"self-hosting.md",
	})...)
	issues = append(issues, requiredTextIssues("docs/architecture.md", "architecture-navigation", contents["docs/architecture.md"], []string{
		"## 授权与披露流程",
		"## 事件到记忆的生命周期",
		"## 选择集成边界",
		"PostgreSQL 是唯一的记录系统",
	})...)
	issues = append(issues, requiredTextIssues("README.md", "entry-point-navigation", contents["README.md"], []string{
		"## 文档",
		"docs/README.md",
		"## 选择集成边界",
	})...)
	issues = append(issues, requiredTextIssues("docs/agent-runtime-memory-provider.md", "entry-point-navigation", contents["docs/agent-runtime-memory-provider.md"], []string{
		"**角色：**",
		"README.md",
		"不授权",
	})...)
	issues = append(issues, requiredTextIssues("docs/integrations/mcp-agent-skill.md", "entry-point-navigation", contents["docs/integrations/mcp-agent-skill.md"], []string{
		"**角色：**",
		"../README.md",
		"面向管理员操作",
	})...)
	issues = append(issues, requiredTextIssues("docs/self-hosting.md", "entry-point-navigation", contents["docs/self-hosting.md"], []string{
		"**角色：** 运维参考",
		"getting-started.md",
	})...)

	for _, descriptor := range mcp.ToolDescriptors() {
		for _, relative := range []string{"docs/integrations/mcp-agent-skill.md", "skills/stele-memory/SKILL.md"} {
			if text, ok := contents[relative]; ok && !strings.Contains(text, "`"+descriptor.Name+"`") && !strings.Contains(text, descriptor.Name) {
				issues = append(issues, Issue{Path: relative, Rule: "mcp-tool-matrix", Message: fmt.Sprintf("current MCP tool %q is not documented", descriptor.Name)})
			}
		}
	}

	required := []string{"tenant", "project", "namespace", "idempotency", "who_am_i", "memory_remember", "memory_forget_preview", "memory_forget_apply"}
	for _, relative := range []string{"docs/integrations/mcp-agent-skill.md", "skills/stele-memory/SKILL.md"} {
		text, ok := contents[relative]
		if !ok {
			continue
		}
		for _, term := range required {
			if !strings.Contains(strings.ToLower(text), strings.ToLower(term)) {
				issues = append(issues, Issue{Path: relative, Rule: "required-safety-rule", Message: fmt.Sprintf("required safety term %q is missing", term)})
			}
		}
	}

	if len(issues) == 0 {
		return nil
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Path != issues[j].Path {
			return issues[i].Path < issues[j].Path
		}
		if issues[i].Line != issues[j].Line {
			return issues[i].Line < issues[j].Line
		}
		return issues[i].Rule < issues[j].Rule
	})
	messages := make([]string, 0, len(issues))
	for _, issue := range issues {
		messages = append(messages, issue.Error())
	}
	return fmt.Errorf("documentation consistency check failed:\n- %s", strings.Join(messages, "\n- "))
}

func requiredTextIssues(path, rule, text string, required []string) []Issue {
	issues := make([]Issue, 0)
	for _, want := range required {
		if !strings.Contains(text, want) {
			issues = append(issues, Issue{Path: path, Rule: rule, Message: fmt.Sprintf("required text %q is missing", want)})
		}
	}
	return issues
}

var markdownLinkPattern = regexp.MustCompile(`\[[^\]]*\]\(\s*(?:<([^>]+)>|([^\s)]+))`)
var credentialPattern = regexp.MustCompile(`(?i)(?:\bsk-[a-z0-9]{16,}\b|\bAKIA[0-9A-Z]{16}\b|\b(?:postgres|postgresql)://[^\s<]+)`)
var sqlWritePattern = regexp.MustCompile(`(?i)\b(?:insert|update|delete|alter|create|drop|truncate)\s+(?:into\s+)?[a-z_]`)

func localLinkIssues(root, relative, text string) []Issue {
	issues := make([]Issue, 0)
	base := filepath.Dir(filepath.Join(root, filepath.FromSlash(relative)))
	for lineNumber, line := range strings.Split(text, "\n") {
		matches := markdownLinkPattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			target := match[1]
			if target == "" {
				target = match[2]
			}
			target = strings.TrimSpace(target)
			lower := strings.ToLower(target)
			if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") {
				continue
			}
			if fragment := strings.IndexByte(target, '#'); fragment >= 0 {
				target = target[:fragment]
			}
			if target == "" {
				continue
			}
			resolved := filepath.Clean(filepath.Join(base, filepath.FromSlash(target)))
			if _, err := os.Stat(resolved); err != nil {
				issues = append(issues, Issue{Path: relative, Line: lineNumber + 1, Rule: "local-link", Message: fmt.Sprintf("link target %q does not exist", target)})
			}
		}
	}
	return issues
}

func unsafeExampleIssues(relative, text string) []Issue {
	issues := make([]Issue, 0)
	inFence := false
	for lineNumber, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if credentialPattern.MatchString(line) {
			issues = append(issues, Issue{Path: relative, Line: lineNumber + 1, Rule: "credential-literal", Message: "documentation contains a credential-looking literal"})
		}
		if inFence && sqlWritePattern.MatchString(line) {
			issues = append(issues, Issue{Path: relative, Line: lineNumber + 1, Rule: "direct-sql-example", Message: "code examples must use the API or MCP contract instead of direct SQL writes"})
		}
	}
	return issues
}
