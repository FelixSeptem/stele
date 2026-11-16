package docs_test

import (
	"os"
	"strings"
	"testing"

	"github.com/FelixSeptem/stele/openapi"
)

func TestSelfHostingSmokeLoopDocumentsReplayContextAndMetrics(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()

	for _, want := range []string{
		"冒烟夹具作用域",
		"tenant-a",
		"project-a",
		"namespace-a",
		"smoke.provider_failure",
		"smoke.operator_recovery",
		"/v1/admin/derived-insight-replays:dry-run",
		"/v1/admin/derived-insight-replays",
		"/v1/admin/derived-insight-replays/<replay-run-id>/report",
		`"include_diagnostics":true`,
		"stele_derived_insight_replay_total",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting smoke docs missing %q", want)
		}
	}

	for _, route := range []string{
		"/v1/context/assemble",
		"/v1/admin/derived-insight-replays:dry-run",
		"/v1/admin/derived-insight-replays",
		"/v1/admin/derived-insight-replays/{replay_run_id}/report",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented route %q", route)
		}
	}

	for _, requiredConfig := range []string{
		"STELE_AUTH_BOOTSTRAP_ADMIN_KEY",
		"STELE_AUTH_DEFAULT_TENANT",
		"STELE_AUTH_DEFAULT_PROJECT",
		"STELE_AUTH_DEFAULT_NAMESPACE",
	} {
		if !strings.Contains(content, requiredConfig) {
			t.Fatalf("self-hosting smoke docs missing required config %q", requiredConfig)
		}
	}
	for _, obsolete := range []string{"STELE_AUTH_API_KEYS", "STELE_AUTH_ADMIN_API_KEYS"} {
		if strings.Contains(content, obsolete+"=") {
			t.Fatalf("self-hosting docs must not present obsolete auth setting as live configuration: %q", obsolete)
		}
	}
}

func TestSelfHostingDocsDescribeReasoningProviderBoundary(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{
		"推理 Provider 边界",
		"默认禁用",
		"离线回放",
		"影子执行",
		"候选或受治理的意图",
		"规范记忆",
		"思维链",
		"原始 Provider 载荷",
		"hypothesis",
		"causal_link",
		"回滚",
		"STELE_REASONING_ENABLED",
		"STELE_REASONING_MODE",
		"GET /v1/reasoning/capabilities",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting reasoning docs missing %q", want)
		}
	}
}

func TestSelfHostingDocsDescribeOpenAICompatibleReasoningAdapter(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{
		"STELE_REASONING_ENDPOINT",
		"STELE_REASONING_MODEL",
		"STELE_REASONING_API_KEY",
		"shadow",
		"一次有界的",
		"自动重试",
		"离线",
		"回放绝不会联系",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting adapter docs missing %q", want)
		}
	}
}

func TestRoadmapTracksCurrentP8Proposal(t *testing.T) {
	roadmapBytes, err := os.ReadFile("roadmaps/2026-05-28-stele-v1-roadmap.md")
	if err != nil {
		t.Fatalf("read v1 roadmap: %v", err)
	}
	roadmap := string(roadmapBytes)
	for _, want := range []string{
		"P8.2b: agent self-model conventions (archived change 047)",
		"P8.3: reasoning-provider boundary (archived change 048)",
		"P8.5: governed reserved-insight activation (archived change 050)",
		"P8.8",
		"`retrieval-release-evidence-refresh-and-controlled-activation`",
		"`governed-reserved-insight-activation`",
		"changes 060–065",
		"`governed-release-evidence-freshness-reconciliation`",
		"self-hosting-first-ten-minutes-smoke-path",
		"governed-autonomous-reasoning-insights",
		"governed-contradiction-insights",
		"governed-memory-intents",
		"current bounded post-v1 candidate",
		"exact-scope activation",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("roadmap missing reasoning proposal status %q", want)
		}
	}
}

func TestSelfHostingDocsDescribeRedactedRetrievalIntegrityEvaluation(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{
		"脱敏检索轨迹与记忆完整性评估",
		"GET /v1/admin/retrieval/integrity-reports",
		"原始查询文本",
		"information-integrity",
		"硬性未通过结果",
		"仅追加的派生 PostgreSQL 记录",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting docs missing retrieval integrity guidance %q", want)
		}
	}
}

func TestSelfHostingDocsDescribeScopedAgentSelfModelConvention(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{
		"Agent 自我模型记忆约定",
		"agents/{agent-id}/self/{category}",
		"`capabilities`、`preferences`、`limitations` 和 `lessons`",
		"profile",
		"procedural",
		"agent-id 仅是",
		"仅是路径数据",
		"服务器发布的能力、授权、",
		"运行时强制限制仍为权威",
		"共享的记忆路径校验",
		"生命周期可见性",
		"段边界",
		"预算",
		`"path":"agents/agent-a/self/preferences"`,
		`"path_prefix":"agents/agent-a/self"`,
		"X-Stele-Tenant: tenant-a",
		"X-Stele-Project: project-a",
		"X-Stele-Namespace: namespace-a",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting self-model docs missing contract %q", want)
		}
	}
}

func TestSelfHostingDocsDescribeReleaseEvidenceReconciliation(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{"发布证据新鲜度核对", "release_evidence_reconciliation", "/v1/admin/release-evidence/reconciliation", "新的兼容证据移交", "失败即关闭"} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting docs missing reconciliation guidance %q", want)
		}
	}
}

func TestSelfModelSpecPreservesExistingClassesAndAuthority(t *testing.T) {
	specBytes, err := os.ReadFile("../openspec/specs/scoped-agent-self-model-conventions/spec.md")
	if err != nil {
		t.Fatalf("read self-model specification: %v", err)
	}
	spec := string(specBytes)
	for _, want := range []string{
		"agents/{agent-id}/self/{category}",
		"`capabilities`, `preferences`, `limitations`, and `lessons",
		"ordinary governed `profile` memory",
		"existing `procedural` memory class",
		"NOT act as a scope, grant, or authorization selector",
		"MUST NOT grant access, change runtime configuration, or override server-published",
		"Self-model memories MUST NOT be",
		"automatically injected into ordinary context",
	} {
		if !strings.Contains(spec, want) {
			t.Fatalf("self-model specification missing normative contract %q", want)
		}
	}
}

func TestRoadmapTracksScopedMemoryAndSelfModelProposalStatus(t *testing.T) {
	roadmapBytes, err := os.ReadFile("roadmaps/2026-05-28-stele-v1-roadmap.md")
	if err != nil {
		t.Fatalf("read v1 roadmap: %v", err)
	}
	roadmap := string(roadmapBytes)
	for _, want := range []string{
		"Changes 060–065",
		"Agent self-model conventions are complete and archived as change 047.",
		"P8.2: scoped memory path conventions (archived change 046)",
		"P8.2b: agent self-model conventions (archived change 047)",
		"P8.3: reasoning-provider boundary (archived change 048)",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("v1 roadmap missing proposal status contract %q", want)
		}
	}
}

func TestBootstrapSmokeScriptIsDocumentedAndConstrained(t *testing.T) {
	script, err := os.ReadFile("../scripts/stele-bootstrap-smoke.ps1")
	if err != nil {
		t.Fatalf("read bootstrap smoke script: %v", err)
	}
	content := string(script)
	for _, want := range []string{"CredentialOutputDirectory", "/v1/admin/principals", "/grants", "bootstrap credential was still accepted", "/openapi.yaml", "/version", "/v1/events", "Idempotency-Key", "/v1/memories/search", "/v1/context/assemble", "/v1/admin/scope-proofs", "same-scope product smoke proof", "$admin.principal.id", "$runtime.principal.id", "ungranted scope read", "runtime principal admin access", "idempotency payload conflict"} {
		if !strings.Contains(content, want) {
			t.Fatalf("bootstrap smoke script missing %q", want)
		}
	}
}

func TestFirstTenMinutesSmokeScriptIsBoundedAndRedacted(t *testing.T) {
	contentBytes, err := os.ReadFile("../scripts/stele-first-ten-minutes.ps1")
	if err != nil {
		t.Fatalf("read first-ten-minutes smoke script: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{
		"TimeoutSeconds",
		"600",
		"900",
		"STELE_FIRST_TEN_MINUTES_CI",
		"COMPOSE_PROJECT_NAME",
		"^[a-z0-9][a-z0-9_-]{2,50}$",
		"preflight",
		"start",
		"discovery",
		"readiness",
		"bootstrap",
		"lifecycle",
		"telemetry",
		"cleanup",
		"stele-bootstrap-smoke.ps1",
		"/health",
		"/readyz",
		"/version",
		"/openapi.yaml",
		"/metrics",
		"down --volumes --remove-orphans",
		"KeepResources",
		"missing_prerequisite",
		"timeout",
		"DSN",
		"API key",
		"raw process errors",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("first-ten-minutes smoke script missing bounded contract %q", want)
		}
	}
	if strings.Contains(content, "Write-Output $env:STELE_POSTGRES_PASSWORD") || strings.Contains(content, "Write-Output $env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY") {
		t.Fatal("first-ten-minutes smoke script must not print credentials")
	}
}

func TestDeploymentContractRejectsObsoleteAuthAndRequiresBootstrapVariables(t *testing.T) {
	compose, err := os.ReadFile("../docker-compose.yml")
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	envExample, err := os.ReadFile("../.env.example")
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	for _, content := range []string{string(compose), string(envExample)} {
		for _, obsolete := range []string{"STELE_AUTH_API_KEYS", "STELE_AUTH_ADMIN_API_KEYS"} {
			if strings.Contains(content, obsolete) {
				t.Fatalf("deployment asset contains obsolete auth setting %q", obsolete)
			}
		}
	}
	for _, required := range []string{"STELE_AUTH_BOOTSTRAP_ADMIN_KEY", "STELE_AUTH_DEFAULT_TENANT", "STELE_AUTH_DEFAULT_PROJECT", "STELE_AUTH_DEFAULT_NAMESPACE", "STELE_DATABASE_MIGRATION_POLICY"} {
		if !strings.Contains(string(compose), required) || !strings.Contains(string(envExample), required) {
			t.Fatalf("deployment contract missing required variable %q", required)
		}
	}
}

func TestDeploymentContractSupportsSharedLocalProviderEnvironment(t *testing.T) {
	compose, err := os.ReadFile("../docker-compose.yml")
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	template, err := os.ReadFile("../.env.local.example")
	if err != nil {
		t.Fatalf("read .env.local.example: %v", err)
	}
	composeContent := string(compose)
	for _, service := range []string{"api:", "worker:", "scheduler:"} {
		if !strings.Contains(composeContent, service) {
			t.Fatalf("compose missing service %q", service)
		}
	}
	if strings.Count(composeContent, "path: .env.local") != 3 {
		t.Fatalf("compose must inject .env.local into api, worker, and scheduler")
	}
	for _, required := range []string{
		"STELE_EMBEDDING_DEFAULT_PROVIDER",
		"STELE_EMBEDDING_OPENAI_API_KEY",
		"STELE_RERANK_ENABLED",
		"STELE_RERANK_ENDPOINT",
		"STELE_RERANK_API_KEY",
		"STELE_GRAPH_MAX_HOPS",
		"STELE_GRAPH_MAX_ELAPSED",
	} {
		if !strings.Contains(string(template), required) {
			t.Fatalf("local environment template missing provider variable %q", required)
		}
	}
}

func TestBackupRecoveryScriptsExposeSafetyGuards(t *testing.T) {
	checks := map[string][]string{
		"../scripts/stele-backup.ps1":         {"SourceDsn", "Destination", "pg_dump", "SHA256", "manifest"},
		"../scripts/stele-restore.ps1":        {"Artifact", "Manifest", "TargetDsn", "ConfirmDestructive", "source-equal", "checksum mismatch", "pg_restore"},
		"../scripts/stele-restore-verify.ps1": {"TargetDsn", "Manifest", "schema_migrations", "X-Stele-Tenant", "authorized scoped service proof", "RecordAssurance", "backup_restore_proof"},
	}
	for path, required := range checks {
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(contentBytes)
		for _, want := range required {
			if !strings.Contains(content, want) {
				t.Fatalf("%s missing safety/verification guard %q", path, want)
			}
		}
	}
}

func TestSelfHostingDocsDescribeChecksummedMigrationIntegrity(t *testing.T) {
	content, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	for _, want := range []string{
		"integrity_status",
		"stele_schema_migration_ledger",
		"SHA-256",
		"前向修复迁移",
		"禁止自动向下迁移",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("self-hosting guide missing migration integrity guidance %q", want)
		}
	}
}

func TestProductVerificationEntryPointDocumentsPrerequisiteAndOwnershipGuards(t *testing.T) {
	contentBytes, err := os.ReadFile("../scripts/stele-product-verify.ps1")
	if err != nil {
		t.Fatalf("read product verification script: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{"STELE_PRODUCT_VERIFY_CI", "SKIP:", "COMPOSE_PROJECT_NAME", "--build -d", "--volumes --remove-orphans", "KeepResources", "stele-bootstrap-smoke.ps1", "CREATE DATABASE", "STELE_TEST_POSTGRES_DSN", "TestMigrationRunnerSerializesConcurrentApply", "STELE_TEST_POSTGRES_UPGRADE_DSN", "TestMigrationRunnerUpgradesPopulatedPriorRelease", "$($migrationDatabase)?sslmode=disable", "docker compose", "stop -t", "readyz", "restart", "Idempotency-Key", "pending_raw_events", "pg_dump", "pg_restore", "targetDatabase", "sha256", "restored scoped behavior"} {
		if !strings.Contains(content, want) {
			t.Fatalf("product verification entrypoint missing %q", want)
		}
	}
}

func TestProceduralVerificationOwnedStackScriptCreatesDatabaseAfterSQLReadiness(t *testing.T) {
	contentBytes, err := os.ReadFile("../scripts/stele-procedural-verification-verify.ps1")
	if err != nil {
		t.Fatalf("read procedural verification owned-stack script: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{
		"SELECT pg_postmaster_start_time()",
		"$previousPostmasterStart",
		"$postgresReady",
		"createdb -U stele --maintenance-db=postgres stele",
		"psql -U stele -d stele -At -c \"CREATE EXTENSION IF NOT EXISTS vector",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("procedural verification owned-stack script missing %q", want)
		}
	}
	if strings.Contains(content, "pg_isready -U stele -d stele") {
		t.Fatal("procedural verification owned-stack script must use a SQL readiness check, not pg_isready for a possibly absent database")
	}
}

func TestProceduralVerificationOperationsAreDocumentedAsAdminOnly(t *testing.T) {
	checks := map[string][]string{
		"../README.md": {
			"程序性验证维护",
			"仅限管理员",
			"受治理更新",
		},
		"architecture.md": {
			"程序性验证与受治理更正",
			"仅追加的验证台账",
			"不会覆盖规范记忆",
		},
		"components.md": {
			"程序性验证维护",
			"procedural_revalidation",
			"PostgreSQL 仍是记录系统",
		},
		"self-hosting.md": {
			"程序性验证维护",
			"STELE_PROCEDURAL_VERIFICATION_ENABLED",
			"STELE_PROCEDURAL_VERIFICATION_MAX_ATTEMPTS",
			"stele-procedural-verification-verify.ps1",
			"回滚",
		},
		"integrations/mcp-agent-skill.md": {
			"程序性验证维护",
			"不是 MCP 工具",
			"不向公共 Provider 凭据开放",
		},
		"agent-runtime-memory-provider.md": {
			"程序性验证仅限管理员",
			"不新增 Provider 路由",
		},
		"../skills/stele-memory/SKILL.md": {
			"Procedural verification is an administrator-only maintenance workflow",
			"must not use Provider or MCP",
		},
	}
	for path, required := range checks {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, want := range required {
			if !strings.Contains(string(content), want) {
				t.Errorf("%s missing procedural-verification operations guidance %q", path, want)
			}
		}
	}
}

func TestSelfHostingDocsIncludeFirstTenMinutesOwnedPath(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{
		"最初十分钟的自有冒烟路径",
		"scripts/stele-first-ten-minutes.ps1",
		"STELE_FIRST_TEN_MINUTES_CI=1",
		"600 秒",
		"900 秒",
		"preflight",
		"discovery",
		"readiness",
		"telemetry",
		"SKIP",
		"KeepResources",
		"完整产品验证",
		"不新增 API 路由",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting guide missing first-ten-minutes contract %q", want)
		}
	}
}

func TestComposeSupportsIsolatedScopesPortsAndMirrorImageOverride(t *testing.T) {
	contentBytes, err := os.ReadFile("../docker-compose.yml")
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{"STELE_POSTGRES_IMAGE", "STELE_POSTGRES_HOST_PORT", "STELE_HTTP_HOST_PORT", "STELE_AUTH_DEFAULT_TENANT", "STELE_AUTH_DEFAULT_PROJECT", "STELE_AUTH_DEFAULT_NAMESPACE", "STELE_GO_IMAGE", "STELE_RUNTIME_IMAGE", "STELE_GOPROXY"} {
		if !strings.Contains(content, want) {
			t.Fatalf("compose missing isolated verification override %q", want)
		}
	}
	if !strings.Contains(content, "stele-postgres-data:/var/lib/postgresql") {
		t.Fatal("compose must mount PostgreSQL data at the major-version-compatible parent directory")
	}
	docsBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	for _, want := range []string{"docker.1ms.run/pgvector/pgvector:pg17", "docker.1ms.run/library/golang:1.25-bookworm", "docker.1ms.run/library/debian:bookworm-slim", "https://goproxy.cn,direct"} {
		if !strings.Contains(string(docsBytes), want) {
			t.Fatalf("self-hosting docs must include the explicit 1ms.run mirror example %q", want)
		}
	}
}

func TestSelfHostingDocsDescribeBoundedRuntimeTelemetry(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	for _, want := range []string{"运行时启动与排空遥测", "迁移校验", "就绪/排空", "不会输出 DSN", "产品验证只输出", "阶段/结果类别"} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting docs missing bounded telemetry contract %q", want)
		}
	}
}

func TestSelfHostingDocsIncludeMemoryQualityRepairLoop(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()

	for _, want := range []string{
		"记忆质量与修复循环",
		"/v1/admin/memory-quality/evaluations",
		"/v1/admin/memory-quality/evaluations/<evaluation-run-id>/findings",
		"/v1/admin/memory-quality/repair-plans",
		"/v1/admin/memory-quality/repair-plans/<repair-plan-id>:approve",
		"/v1/admin/memory-quality/repair-plans/<repair-plan-id>:verify",
		"admission.decision",
		"stele_quality_",
		"manual_review",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting quality repair docs missing %q", want)
		}
	}

	for _, route := range []string{
		"/v1/admin/memory-quality/evaluations",
		"/v1/admin/memory-quality/evaluations/{evaluation_run_id}/findings",
		"/v1/admin/memory-quality/repair-plans",
		"/v1/admin/memory-quality/repair-plans/{repair_plan_id}:approve",
		"/v1/admin/memory-quality/repair-plans/{repair_plan_id}:verify",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented quality route %q", route)
		}
	}
}

func TestSelfHostingDocsIncludeScopeProofAndMemorySessionLoop(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()

	for _, want := range []string{
		"持久作用域证明与记忆会话循环",
		"/v1/admin/scope-proofs",
		"/v1/admin/scope-proofs/<proof-run-id>/report",
		"/v1/admin/scope-proofs/<proof-run-id>:rerun",
		"/v1/memory-sessions",
		"/v1/memory-sessions/<session-id>/turns",
		"/v1/memory-sessions/<session-id>/turns/<turn-id>:outcome",
		"/v1/memory-sessions/<session-id>:verify",
		"/v1/memory-sessions/<session-id>/report",
		"inspect_context_diagnostics",
		"open_quality_evaluation",
		"open_repair_plan",
		"stele_scope_proof_steps_total",
		"stele_memory_session_verifications_total",
		"SDK/UI",
		"外部 agent 运行时集成",
		"容量/负载证明",
		"备份/恢复证明",
		"长期记忆有用性评分",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting proof/session docs missing %q", want)
		}
	}

	for _, route := range []string{
		"/v1/admin/scope-proofs",
		"/v1/admin/scope-proofs/{proof_run_id}/report",
		"/v1/admin/scope-proofs/{proof_run_id}:rerun",
		"/v1/memory-sessions",
		"/v1/memory-sessions/{session_id}/turns",
		"/v1/memory-sessions/{session_id}/turns/{turn_id}:outcome",
		"/v1/memory-sessions/{session_id}:verify",
		"/v1/memory-sessions/{session_id}/report",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented proof/session route %q", route)
		}
	}
}

func TestSelfHostingDocsIncludeTaskSuccessLoop(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()

	for _, want := range []string{
		"外部 agent 任务成功循环",
		"/v1/task-evaluations",
		"/v1/task-evaluations/<task-evaluation-id>/report",
		"/v1/admin/task-evaluations",
		"/v1/admin/task-evaluations/<task-evaluation-id>",
		"/v1/admin/task-evaluations/<task-evaluation-id>/supersede",
		"/v1/admin/task-evaluations/summary",
		"不透明的调用方证据令牌",
		"有界的关联 ID",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting task success docs missing %q", want)
		}
	}

	for _, route := range []string{
		"/v1/task-evaluations",
		"/v1/task-evaluations/{evaluation_id}/report",
		"/v1/admin/task-evaluations",
		"/v1/admin/task-evaluations/{evaluation_id}",
		"/v1/admin/task-evaluations/summary",
		"/v1/admin/task-evaluations/{evaluation_id}/supersede",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented task success route %q", route)
		}
	}
}

func TestSelfHostingDocsStateRemainingFeedbackLoopProductGaps(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)

	for _, want := range []string{
		"反馈循环剩余的产品缺口",
		"受治理的排序发布（rollout）策略",
		"SDK/UI 采集界面",
		"外部 agent 运行时集成",
		"托管故障管理",
		"厂商专用告警集成",
		"高级评分校准",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting remaining product gaps docs missing %q", want)
		}
	}

	if strings.Contains(content, "Default feedback-aware ranking rollout. Current behavior is diagnostics-first") {
		t.Fatalf("self-hosting docs still describe governed ranking rollout as a remaining gap")
	}
}

func TestSelfHostingDocsIncludeAssuranceConformanceReadinessLoop(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()

	for _, want := range []string{
		"生产就绪保障与一致性循环",
		"/v1/admin/assurance/health-evaluations",
		"/v1/admin/assurance/conformance-profiles",
		"/v1/admin/assurance/conformance-runs",
		"/v1/admin/assurance/readiness-reports",
		"/v1/admin/assurance/incidents",
		"/v1/admin/assurance/alert-candidates",
		"/v1/admin/assurance/recovery-verifications",
		"容量/负载证明",
		"备份/恢复证明",
		"`disabled`、`stdout` 和通用 `webhook`",
		"默认使用 HTTPS",
		"拒绝不安全目标",
		"stele_assurance_health_evaluations_total",
		"stele_conformance_runs_total",
		"component=assurance event=lifecycle",
		"component=conformance event=lifecycle",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting assurance/conformance docs missing %q", want)
		}
	}

	for _, route := range []string{
		"/v1/admin/assurance/health-evaluations",
		"/v1/admin/assurance/incidents",
		"/v1/admin/assurance/alert-candidates",
		"/v1/admin/assurance/conformance-profiles",
		"/v1/admin/assurance/conformance-runs",
		"/v1/admin/assurance/readiness-reports",
		"/v1/admin/assurance/recovery-verifications",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented assurance route %q", route)
		}
	}
}

func TestSelfHostingDocsStateRemainingAssuranceProductGaps(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)

	for _, want := range []string{
		"保障功能的剩余产品缺口",
		"SDK/UI 采集界面",
		"外部 agent 运行时实现",
		"厂商专用告警集成",
		"托管故障管理",
		"自适应评分校准",
		"模型调用",
		"提示编排",
		"最终答案生成",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting assurance gaps docs missing %q", want)
		}
	}
}

func TestSelfHostingDocsIncludeIntegrationEvidenceWorkflowGoldenPath(t *testing.T) {
	contentBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	content := string(contentBytes)
	spec := openapi.SpecYAML()
	for _, want := range []string{
		"集成证据工作流黄金路径",
		"/v1/admin/workflows/templates",
		"/v1/workflows/runs",
		"/v1/workflows/runs/<workflow-run-id>/steps",
		"/v1/workflows/runs/<workflow-run-id>/next-actions",
		"/v1/admin/workflows/runs/<workflow-run-id>/diagnostics",
		"/v1/admin/workflows/evidence-links/<evidence-link-id>/supersede",
		"STELE_WORKFLOW_MAINTENANCE_ENABLED",
		"stele_workflow_lifecycle_total",
		"外部 agent 执行",
		"模型调用",
		"提示编排",
		"最终答案生成",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("self-hosting workflow docs missing %q", want)
		}
	}
	for _, route := range []string{
		"/v1/admin/workflows/templates",
		"/v1/workflows/runs",
		"/v1/workflows/runs/{workflow_run_id}/steps",
		"/v1/workflows/runs/{workflow_run_id}/next-actions",
		"/v1/admin/workflows/runs/{workflow_run_id}/diagnostics",
		"/v1/admin/workflows/evidence-links/{evidence_link_id}/supersede",
	} {
		if !strings.Contains(spec, route) {
			t.Fatalf("OpenAPI spec missing documented workflow route %q", route)
		}
	}
}

func TestFusionContractAndRolloutRunbookAreDocumented(t *testing.T) {
	qualityBytes, err := os.ReadFile("retrieval-quality-baseline.md")
	if err != nil {
		t.Fatalf("read retrieval-quality-baseline.md: %v", err)
	}
	quality := string(qualityBytes)
	for _, want := range []string{
		"rrf:rrf-v1",
		"weight(channel) / (rank_constant + rank)",
		"Per-channel candidate bound: `50`",
		"Total candidate bound: `200`",
		"stele_retrieval_fusion_total",
		"rollback_restored",
	} {
		if !strings.Contains(quality, want) {
			t.Fatalf("retrieval-quality baseline missing fusion contract %q", want)
		}
	}

	hostingBytes, err := os.ReadFile("self-hosting.md")
	if err != nil {
		t.Fatalf("read self-hosting.md: %v", err)
	}
	hosting := string(hostingBytes)
	for _, want := range []string{
		"稳定融合发布（rollout）与评估器运行手册",
		"/v1/admin/ranking-rollouts",
		"normalized_weighted:normalized-weighted-v1",
		"active_for_scope",
		"STELE_TEST_RETRIEVAL_EVALUATION_DSN",
		"STELE_TEST_RETRIEVAL_EVALUATION_OWNED",
		"-TimeoutSeconds 300",
		"release-evidence.json",
		"脱敏的 `evidence` 对象",
		"SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED",
	} {
		if !strings.Contains(hosting, want) {
			t.Fatalf("self-hosting guide missing fusion runbook contract %q", want)
		}
	}
}
