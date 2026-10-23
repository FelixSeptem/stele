# External Developer Documentation Portal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give external developers a task-oriented path to understand, run,
integrate, and extend Stele while preserving existing detailed contract and
operations documentation as the authority.

**Architecture:** Keep `README.md` as the compact repository landing page and
add `docs/README.md` as the documentation portal. Add a short first-run guide,
make the architecture explain runtime, authorization, and memory data flows,
and place integration choice guidance at the existing Provider and MCP entry
points. Extend the existing offline documentation checker so the new public
entry pages and their local links remain verified.

**Tech Stack:** Markdown, Mermaid, Go `internal/docscheck`, Go tests, existing
PowerShell documentation checker, OpenSpec validation.

---

## File Structure

- Create: `docs/README.md` — task-oriented documentation portal and authority
  map for external developers, operators, and maintainers.
- Create: `docs/getting-started.md` — first-run tutorial that points to, but
  does not duplicate, full self-hosting instructions.
- Modify: `README.md` — concise landing page, primary paths, integration
  decision table, and a link to the documentation portal.
- Modify: `docs/architecture.md` — visual runtime, authorization, lifecycle,
  and integration-boundary explanations.
- Modify: `docs/integrations/mcp-agent-skill.md` — role statement, portal
  link, and Provider/MCP/native API choice guidance.
- Modify: `docs/agent-runtime-memory-provider.md` — role statement, portal
  link, and clear cross-reference to administrator MCP instead of fallback.
- Modify: `docs/self-hosting.md` — role statement and short link to the
  first-run guide.
- Modify: `internal/docscheck/check.go` — require and locally validate the
  portal and first-run guide; verify their navigation headings and boundary
  terms.
- Modify: `internal/docscheck/check_test.go` — make temporary fixtures include
  the new required public docs and test missing navigation diagnostics.

### Task 1: Add the documentation portal and first-run path

**Files:**
- Create: `docs/README.md`
- Create: `docs/getting-started.md`
- Test: `internal/docscheck/check_test.go`

- [ ] **Step 1: Write failing documentation-checker tests for the new public documents.**

  Add both files to the fixture list in `TestCheckReportsFileAndRuleForUnsafeFixture`.
  Introduce this shared fixture builder in `internal/docscheck/check_test.go` so
  later rule tests start from a complete, safe documentation tree:

  ```go
  func completeDocumentationFixture(t *testing.T) string {
      t.Helper()
      root := t.TempDir()
      contents := map[string]string{
          "README.md": "# Stele\n## Documentation\n[portal](docs/README.md)\n## Choose an integration boundary\n",
          "docs/README.md": "# Docs\n## Understand Stele\n## Run Stele\n## Integrate Stele\n## Extend and maintain Stele\nOpenAPI Provider MCP Self-hosting OpenSpec authoritative\n",
          "docs/getting-started.md": "**Role:** tutorial\nPostgreSQL is the only system of record\n## Choose your next integration path\n[self-hosting](self-hosting.md)\n",
          "docs/architecture.md": "## Authorization and disclosure flow\n## Event-to-memory lifecycle\n## Choosing an integration boundary\nPostgreSQL is the only system of record\n",
          "docs/components.md": "components\n",
          "docs/best-practices.md": "practices\n",
          "docs/self-hosting.md": "**Role:** operations reference\n[Getting started](getting-started.md)\n",
          "docs/agent-runtime-memory-provider.md": "**Role:** reference\n[portal](README.md)\n",
          "docs/integrations/mcp-agent-skill.md": "**Role:** reference\n[portal](../README.md)\ntenant project namespace idempotency who_am_i memory_remember memory_forget_preview memory_forget_apply\nwho_am_i memory_search memory_context memory_browse memory_remember memory_forget memory_forget_preview memory_forget_apply\n",
          "skills/stele-memory/SKILL.md": "tenant project namespace idempotency who_am_i memory_remember memory_forget_preview memory_forget_apply\nwho_am_i memory_search memory_context memory_browse memory_remember memory_forget memory_forget_preview memory_forget_apply\n",
      }
      for path, content := range contents {
          full := filepath.Join(root, filepath.FromSlash(path))
          if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil { t.Fatal(err) }
          if err := os.WriteFile(full, []byte(content), 0o644); err != nil { t.Fatal(err) }
      }
      return root
  }
  ```

  Then add a test that writes syntactically safe files missing the required
  portal heading and expects a `portal-navigation` failure:

  ```go
  func TestCheckRequiresPortalNavigation(t *testing.T) {
      root := completeDocumentationFixture(t)
      if err := os.WriteFile(filepath.Join(root, "docs", "README.md"), []byte("# Docs\n"), 0o644); err != nil {
          t.Fatal(err)
      }
      if err := Check(root); err == nil || !strings.Contains(err.Error(), "portal-navigation") {
          t.Fatalf("Check() error = %v, want portal-navigation", err)
      }
  }
  ```

- [ ] **Step 2: Run the focused test to verify it fails.**

  Run:

  ```powershell
  go test ./internal/docscheck -run TestCheckRequiresPortalNavigation -count=1
  ```

  Expected: FAIL because the checker has no portal navigation rule.

- [ ] **Step 3: Write `docs/README.md` as the task-oriented portal.**

  Use the exact top-level flow below and link only to real local files:

  ```markdown
  # Stele documentation

  This is the documentation entry point for developers evaluating, operating,
  integrating, or extending Stele. The [repository README](../README.md) is the
  short project overview; this page tells you where to go next.

  ## Start here

  1. Read [Getting started](getting-started.md) to run the smallest local stack.
  2. Read [Architecture](architecture.md) to understand runtime and memory flow.
  3. Choose an integration boundary in [Integrate Stele](#integrate-stele).

  ## Understand Stele
  ## Run Stele
  ## Integrate Stele
  ## Extend and maintain Stele
  ## Authoritative references
  ```

  In the four path sections, link to architecture, components, best practices,
  self-hosting, Provider, MCP, disclosure/source trust, OpenAPI source,
  migrations, tests, and OpenSpec. State that OpenAPI and the versioned Provider
  contract outrank overview prose when behavior differs.

- [ ] **Step 4: Write `docs/getting-started.md` as a bounded first-run tutorial.**

  Give it this structure and keep secrets as placeholders:

  ```markdown
  # Getting started with Stele

  **Role:** tutorial. This guide starts a local development stack; use
  [Self-hosting](self-hosting.md) for production deployment, migrations,
  backup, restore, and recovery.

  ## Before you start
  ## Start the local stack
  ## Verify process health and readiness
  ## Understand the first safe workflow
  ## Choose your next integration path
  ## Clean up and continue
  ```

  Include the existing `.env.local.example`/`docker compose --env-file
  .env.local up --build -d` command, describe `/health`, `/ready`, `/livez`,
  and `/readyz` without claiming they establish semantic readiness, and explain
  that a bootstrap admin and exact-scope runtime principal are required before
  protected calls. Link rather than copy request bodies from Provider/MCP docs.
  Explicitly state that PostgreSQL is the only system of record and no example
  should write directly to the database.

- [ ] **Step 5: Implement the portal checks.**

  In `internal/docscheck/check.go`, add both new pages to `files`. After the
  existing safety checks, require these strings in `docs/README.md`:

  ```go
  requiredPortalHeadings := []string{
      "## Understand Stele",
      "## Run Stele",
      "## Integrate Stele",
      "## Extend and maintain Stele",
  }
  ```

  Report missing headings as `portal-navigation`. Require `docs/getting-started.md`
  to contain `**Role:** tutorial`, `PostgreSQL is the only system of record`,
  `## Choose your next integration path`, and `self-hosting.md`; report missing
  strings as `getting-started-contract`.

  Add this local helper in `internal/docscheck/check.go` and use it for each
  new document-rule group:

  ```go
  func requiredTextIssues(path, rule, text string, required []string) []Issue {
      issues := make([]Issue, 0)
      for _, want := range required {
          if !strings.Contains(text, want) {
              issues = append(issues, Issue{Path: path, Rule: rule, Message: fmt.Sprintf("required text %q is missing", want)})
          }
      }
      return issues
  }
  ```

- [ ] **Step 6: Run focused documentation checks.**

  Run:

  ```powershell
  go test ./internal/docscheck ./docs -count=1
  pwsh -NoProfile -File scripts/check-documentation.ps1
  ```

  Expected: PASS. The checker validates all local links and public entry-page
  navigation without network access.

- [ ] **Step 7: Commit the portal and first-run tutorial.**

  ```powershell
  git add docs/README.md docs/getting-started.md internal/docscheck/check.go internal/docscheck/check_test.go
  git commit -m "docs: add external developer portal"
  ```

### Task 2: Make the architecture explain the governing flows

**Files:**
- Modify: `docs/architecture.md`
- Test: `internal/docscheck/check.go`
- Test: `internal/docscheck/check_test.go`

- [ ] **Step 1: Add a failing architecture vocabulary assertion.**

  Extend the checker with a `requiredArchitectureTerms` list:

  ```go
  requiredArchitectureTerms := []string{
      "## Authorization and disclosure flow",
      "## Event-to-memory lifecycle",
      "## Choosing an integration boundary",
      "PostgreSQL is the only system of record",
  }
  ```

  Missing terms must report `architecture-navigation`. Add a temporary-fixture
  test that replaces `docs/architecture.md` with a safe minimal file and
  expects that rule.

- [ ] **Step 2: Run the focused test to verify it fails.**

  Run:

  ```powershell
  go test ./internal/docscheck -run TestCheckRequiresArchitectureNavigation -count=1
  ```

  Expected: FAIL because the checker does not yet require architecture sections.

- [ ] **Step 3: Expand `docs/architecture.md` with four explanatory flows.**

  Preserve existing runtime modes and persistence boundaries. Add a role line:

  ```markdown
  **Role:** overview. This page explains system boundaries and data flow;
  [components](components.md), [self-hosting](self-hosting.md), the
  [Provider contract](agent-runtime-memory-provider.md), and the OpenAPI source
  remain the detailed authorities for their subjects.
  ```

  Add sections with the exact headings required in Step 1:

  - **Authorization and disclosure flow**: Mermaid sequence/flow diagram from
    principal to exact scope, runtime binding, disclosure profile, service,
    lifecycle visibility, and bounded response. State that caller fields cannot
    widen scope, role, profile, or memory path.
  - **Event-to-memory lifecycle**: diagram and prose from event/intent through
    durable records, candidate processing, canonical version/provenance,
    active/suppressed/forgotten/deleted lifecycle, retrieval and citations.
    State that worker processing is asynchronous and canonical memory is not
    overwritten in place.
  - **Choosing an integration boundary**: decision diagram/table explaining
    Provider for public agent runtimes, MCP for administrators, native OpenAPI
    for endpoint-specific custom integrations, and Go packages for maintainers.
    State that Provider denials do not permit MCP/native fallback.
  - **What Stele does not own**: model inference, prompts, product task state,
    business roles, generation commits, and client-side token budgeting.

- [ ] **Step 4: Implement the checker and pass its focused test.**

  Add the architecture rule in `Check` using the `requiredTextIssues` helper
  introduced in Task 1. Run:

  ```powershell
  go test ./internal/docscheck ./docs -count=1
  ```

  Expected: PASS, confirming the rendered source contains the required
  explanatory entry points and all local links exist.

- [ ] **Step 5: Commit the architecture narrative.**

  ```powershell
  git add docs/architecture.md internal/docscheck/check.go internal/docscheck/check_test.go
  git commit -m "docs: explain Stele architecture flows"
  ```

### Task 3: Clarify integration selection and normalize entry points

**Files:**
- Modify: `README.md`
- Modify: `docs/integrations/mcp-agent-skill.md`
- Modify: `docs/agent-runtime-memory-provider.md`
- Modify: `docs/self-hosting.md`
- Test: `internal/docscheck/check.go`
- Test: `internal/docscheck/check_test.go`

- [ ] **Step 1: Add failing entry-point assertions.**

  Require the README to contain a `## Documentation` link to `docs/README.md`
  and an integration choice table heading. Require the Provider and MCP guides
  to contain `**Role:** reference` and a link to `../README.md` or
  `README.md` appropriate to their directory. Report missing material as
  `entry-point-navigation`.

  Add a fixture test that removes the README portal link and expects the rule:

  ```go
  func TestCheckRequiresPublicEntryPointNavigation(t *testing.T) {
      root := completeDocumentationFixture(t)
      path := filepath.Join(root, "README.md")
      if err := os.WriteFile(path, []byte("# Stele\n## Documentation\n"), 0o644); err != nil {
          t.Fatal(err)
      }
      if err := Check(root); err == nil || !strings.Contains(err.Error(), "entry-point-navigation") {
          t.Fatalf("Check() error = %v, want entry-point-navigation", err)
      }
  }
  ```

- [ ] **Step 2: Run the focused test to verify it fails.**

  Run:

  ```powershell
  go test ./internal/docscheck -run TestCheckRequiresPublicEntryPointNavigation -count=1
  ```

  Expected: FAIL because README and integration-guide navigation is not checked.

- [ ] **Step 3: Reshape `README.md` into the landing page.**

  Retain the existing product definition, quick-start command, MCP tool matrix,
  and safety principles. Before the long Provider/MCP details, add:

  ```markdown
  ## Documentation

  Start with the [documentation portal](docs/README.md). It provides the
  recommended path for understanding, running, integrating, and maintaining
  Stele.

  ## Choose an integration boundary

  | You need to... | Start with | Do not use it for |
  | --- | --- | --- |
  | Give a public agent runtime governed context or writes | [Provider runtime](docs/agent-runtime-memory-provider.md) | administrator MCP or native-read fallback |
  | Let an administrator inspect or review memory | [MCP integration](docs/integrations/mcp-agent-skill.md) | public agent credentials |
  | Build a custom service client | [OpenAPI](openapi/spec.go) | bypassing principal and scope rules |
  | Operate a deployment | [Self-hosting](docs/self-hosting.md) | application business-state management |
  ```

  Replace the existing flat documentation map with links grouped under the four
  portal paths. Keep detailed PC3/PC4/PC5/PC6 behavior in the Provider guide;
  README may retain one concise compatibility statement and a deep link.

- [ ] **Step 4: Add integration-guide roles and non-fallback guidance.**

  At the start of `docs/integrations/mcp-agent-skill.md`, add:

  ```markdown
  **Role:** reference for administrator-operated MCP workflows. Start at the
  [documentation portal](../README.md) when choosing an integration boundary.
  Public agent runtimes use the [Provider contract](../agent-runtime-memory-provider.md),
  not MCP.
  ```

  At the start of `docs/agent-runtime-memory-provider.md`, add:

  ```markdown
  **Role:** reference for public agent runtime integration. Start at the
  [documentation portal](README.md) for the full integration decision. A
  Provider denial does not authorize MCP, native-read, alternate-credential,
  local-memory, ranking, or context-reconstruction fallback.
  ```

  At the start of `docs/self-hosting.md`, add:

  ```markdown
  **Role:** operations reference. For a small local development path, start
  with [Getting started](getting-started.md); return here for production
  configuration, migration, backup, restore, and recovery.
  ```

- [ ] **Step 5: Implement the entry-point checker and pass the documentation suite.**

  Add the new required phrases to `Check`, keeping expected relative links exact
  for each document. Run:

  ```powershell
  go test ./internal/docscheck ./docs -count=1
  pwsh -NoProfile -File scripts/check-documentation.ps1
  ```

  Expected: PASS with local-link validation covering every changed public entry
  page.

- [ ] **Step 6: Commit integration navigation.**

  ```powershell
  git add README.md docs/integrations/mcp-agent-skill.md docs/agent-runtime-memory-provider.md docs/self-hosting.md internal/docscheck/check.go internal/docscheck/check_test.go
  git commit -m "docs: clarify integration entry points"
  ```

### Task 4: Audit links, authority boundaries, and repository-wide documentation gates

**Files:**
- Modify: `docs/README.md`
- Modify: `docs/getting-started.md`
- Modify: `docs/architecture.md`
- Modify: `README.md`
- Modify: `internal/docscheck/check.go`
- Test: `docs/documentation_test.go`

- [ ] **Step 1: Write a failing authority-boundary test.**

  Add `TestPublicDocumentationPortalHasAuthorityMap` in
  `docs/documentation_test.go`. It reads `docs/README.md` and asserts links or
  labels for OpenAPI, Provider, MCP, self-hosting, components, and OpenSpec;
  it must also assert text saying OpenAPI/versioned contracts are authoritative.

  ```go
  func TestPublicDocumentationPortalHasAuthorityMap(t *testing.T) {
      data, err := os.ReadFile("README.md")
      if err != nil { t.Fatal(err) }
      for _, want := range []string{"OpenAPI", "Provider", "MCP", "Self-hosting", "OpenSpec", "authoritative"} {
          if !strings.Contains(string(data), want) { t.Fatalf("portal missing %q", want) }
      }
  }
  ```

- [ ] **Step 2: Run the focused test to verify it fails.**

  Run:

  ```powershell
  go test ./docs -run TestPublicDocumentationPortalHasAuthorityMap -count=1
  ```

  Expected: FAIL before the portal is present in the docs test package.

- [ ] **Step 3: Audit all new prose against authoritative contracts.**

  Check every statement about API routes, scope, roles, readiness, Provider,
  MCP, continuity, storage, and migration against `openapi/spec.go`, the
  Provider/MCP guides, and `docs/self-hosting.md`. Replace duplicate protocol
  detail with links. Confirm the portal labels each destination as overview,
  tutorial, reference, or operations and directs readers to OpenAPI or a
  versioned contract when exact behavior matters.

- [ ] **Step 4: Add the authority test and run all documentation gates.**

  Run:

  ```powershell
  go test ./docs ./internal/docscheck -count=1
  pwsh -NoProfile -File scripts/check-documentation.ps1
  git diff --check
  openspec validate --all --strict
  ```

  Expected: all commands exit 0. Treat OpenSpec informational long-requirement
  notices as information unless validation reports a failure.

- [ ] **Step 5: Commit final documentation validation.**

  ```powershell
  git add docs/README.md docs/getting-started.md docs/architecture.md README.md internal/docscheck/check.go internal/docscheck/check_test.go docs/documentation_test.go
  git commit -m "test(docs): verify external developer navigation"
  ```

## Final Verification

- [ ] Run `git status --short` and confirm only intended commits are present.
- [ ] Run `git log --oneline main..HEAD` and confirm the design, portal,
  architecture, integration-navigation, and final-validation commits are all
  visible.
- [ ] Run the full repository test suite with `go test ./... -count=1`.
- [ ] Re-read `docs/README.md` as an external developer: it must answer what
  Stele is, what it does not own, how to run it, which boundary to use, and
  where normative contracts live without requiring source inspection.
