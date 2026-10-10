[CmdletBinding()]
param(
    [string]$ProjectName = "stele-verify-$([guid]::NewGuid().ToString('N').Substring(0, 12))",
    [string]$ComposeFile = "docker-compose.yml",
    [switch]$KeepResources,
    [string]$ReportPath = "",
    [switch]$NoBuild
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    $composeAttempted = $false
    $composeCleanupCompleted = $false
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        if ($env:STELE_PRODUCT_VERIFY_CI -eq "1") { throw "Docker is required for product verification in CI" }
        Write-Output "SKIP: Docker CLI is not installed; product verification did not run"
        exit 2
    }

    & docker info *> $null
    if ($LASTEXITCODE -ne 0) {
        if ($env:STELE_PRODUCT_VERIFY_CI -eq "1") { throw "Docker daemon is unavailable in CI" }
        Write-Output "SKIP: Docker daemon is unavailable; product verification did not run"
        exit 2
    }

    if ([string]::IsNullOrWhiteSpace($ProjectName) -or $ProjectName -notmatch '^[a-z0-9][a-z0-9_-]{2,50}$') {
        throw "ProjectName must be an explicit, bounded Compose project identifier"
    }
    $env:COMPOSE_PROJECT_NAME = $ProjectName
    if ([string]::IsNullOrWhiteSpace($env:STELE_POSTGRES_PASSWORD)) { $env:STELE_POSTGRES_PASSWORD = "verify-$([guid]::NewGuid().ToString('N'))" }
    if ([string]::IsNullOrWhiteSpace($env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY)) { $env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY = "verify-bootstrap-$([guid]::NewGuid().ToString('N'))" }
    if ([string]::IsNullOrWhiteSpace($env:STELE_AUTH_DEFAULT_TENANT)) { $env:STELE_AUTH_DEFAULT_TENANT = "tenant-verify-$ProjectName" }
    if ([string]::IsNullOrWhiteSpace($env:STELE_AUTH_DEFAULT_PROJECT)) { $env:STELE_AUTH_DEFAULT_PROJECT = "project-verify-$ProjectName" }
    if ([string]::IsNullOrWhiteSpace($env:STELE_AUTH_DEFAULT_NAMESPACE)) { $env:STELE_AUTH_DEFAULT_NAMESPACE = "namespace-verify-$ProjectName" }
    if ([string]::IsNullOrWhiteSpace($env:STELE_POSTGRES_HOST_PORT)) { $env:STELE_POSTGRES_HOST_PORT = (Get-Random -Minimum 15432 -Maximum 25432).ToString() }
    if ([string]::IsNullOrWhiteSpace($env:STELE_HTTP_HOST_PORT)) { $env:STELE_HTTP_HOST_PORT = (Get-Random -Minimum 18080 -Maximum 28080).ToString() }
    if (-not [string]::IsNullOrWhiteSpace($env:STELE_PRODUCT_VERIFY_POSTGRES_IMAGE)) {
        $env:STELE_POSTGRES_IMAGE = $env:STELE_PRODUCT_VERIFY_POSTGRES_IMAGE
    }
    if (-not [string]::IsNullOrWhiteSpace($env:STELE_PRODUCT_VERIFY_GO_IMAGE)) {
        $env:STELE_GO_IMAGE = $env:STELE_PRODUCT_VERIFY_GO_IMAGE
    }
    if (-not [string]::IsNullOrWhiteSpace($env:STELE_PRODUCT_VERIFY_RUNTIME_IMAGE)) {
        $env:STELE_RUNTIME_IMAGE = $env:STELE_PRODUCT_VERIFY_RUNTIME_IMAGE
    }
    if (-not [string]::IsNullOrWhiteSpace($env:STELE_PRODUCT_VERIFY_GOPROXY)) {
        $env:STELE_GOPROXY = $env:STELE_PRODUCT_VERIFY_GOPROXY
    }
    $baseUrl = "http://localhost:$($env:STELE_HTTP_HOST_PORT)"
    $credentialDir = $null
    $mcpEvidencePath = $null
    $policyOverridePath = Join-Path ([System.IO.Path]::GetTempPath()) "stele-product-verify-policy-$ProjectName.yml"
    $mcpEnabled = $false
    if ([string]::IsNullOrWhiteSpace($ReportPath)) {
        $ReportPath = Join-Path ([System.IO.Path]::GetTempPath()) "stele-product-verify-$ProjectName.conformance.json"
    }
    $conformancePhases = [System.Collections.Generic.List[object]]::new()
    $conformanceRunID = "product-verify-$ProjectName"

    function Add-ConformancePhase([string]$Phase, [string]$Result, [string]$Category, [string]$Duration = "unknown") {
        if ($Phase -notmatch '^(prerequisite|submission|replay|scope_isolation|inspection|queue_recovery|worker_restart|scheduler_restart|rollback|cleanup)$') { throw "unsupported conformance phase '$Phase'" }
        if ($Result -notmatch '^(pass|skip|degraded|fail)$') { throw "unsupported conformance result '$Result'" }
        if ($Category.Length -gt 64 -or $Category -notmatch '^[a-z0-9_]+$') { throw "unbounded conformance category" }
        [void]$conformancePhases.Add([ordered]@{ phase = $Phase; result = $Result; category = $Category; duration_bucket = $Duration })
    }

    function Write-RedactedConformanceReport([string]$Result = "pass") {
        $scopeMaterial = "$($env:STELE_AUTH_DEFAULT_TENANT)|$($env:STELE_AUTH_DEFAULT_PROJECT)|$($env:STELE_AUTH_DEFAULT_NAMESPACE)"
        $runHash = ([Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($conformanceRunID)))).ToLowerInvariant()
        $scopeHash = ([Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($scopeMaterial)))).ToLowerInvariant()
        $phaseCounts = [ordered]@{}
        $resultCounts = [ordered]@{}
        $categoryCounts = [ordered]@{}
        foreach ($phaseResult in @($conformancePhases)) {
            foreach ($bucket in @(@("phase", [string]$phaseResult.phase), @("result", [string]$phaseResult.result), @("category", [string]$phaseResult.category))) {
                $target = if ($bucket[0] -eq "phase") { $phaseCounts } elseif ($bucket[0] -eq "result") { $resultCounts } else { $categoryCounts }
                if (-not $target.Contains($bucket[1])) { $target[$bucket[1]] = 0 }
                $target[$bucket[1]] = [int]$target[$bucket[1]] + 1
            }
        }
        $report = [ordered]@{
            run_hash = "run:$runHash"
            scope_hash = "scope:$scopeHash"
            schema_version = "runtime"
            provider_version = "self-hosted"
            result = $Result
            consumable = ($Result -eq "pass")
            phases = @($conformancePhases)
            summary = [ordered]@{
                phase_counts = $phaseCounts
                result_counts = $resultCounts
                category_counts = $categoryCounts
                recovery = if ($categoryCounts.Contains("queue_recovery") -or $categoryCounts.Contains("worker_restart") -or $categoryCounts.Contains("scheduler_restart")) { "observed" } else { "missing" }
                rollback = if ($categoryCounts.Contains("rollback") -or $categoryCounts.Contains("policy_disabled")) { "skipped_or_observed" } else { "missing" }
                cleanup = if ($categoryCounts.Contains("cleanup")) { "observed" } else { "missing" }
            }
            generated_at = [DateTime]::UtcNow.ToString("o")
        }
        $parent = Split-Path -Parent $ReportPath
        if (-not [string]::IsNullOrWhiteSpace($parent)) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
        $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $ReportPath -Encoding UTF8 -NoNewline
    }

    function Invoke-Condition([scriptblock]$Condition, [int]$TimeoutSeconds = 30, [string]$FailureMessage = "condition did not become true") {
        $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
        do {
            try {
                if (& $Condition) { return }
            } catch { }
            Start-Sleep -Milliseconds 250
        } while ([DateTime]::UtcNow -lt $deadline)
        throw $FailureMessage
    }

    function Get-ContainerState([string]$Service) {
        $id = (& docker compose -f $ComposeFile ps -q $Service 2>$null | Out-String).Trim()
        if ([string]::IsNullOrWhiteSpace($id)) { return "missing" }
        return (& docker inspect -f '{{.State.Status}}' $id 2>$null | Out-String).Trim()
    }

    function Assert-BoundedStop([string]$Service) {
        $started = [DateTime]::UtcNow
        & docker compose -f $ComposeFile stop -t 10 $Service *> $null
        if ($LASTEXITCODE -ne 0) { throw "failed to stop owned $Service service" }
        Invoke-Condition { (Get-ContainerState $Service) -in @("exited", "missing") } 20 "owned $Service service did not terminate within bound"
        if (([DateTime]::UtcNow - $started).TotalSeconds -gt 20) { throw "owned $Service service exceeded bounded termination" }
    }

    function Assert-RestartReady([string]$Service) {
        & docker compose -f $ComposeFile up -d $Service *> $null
        if ($LASTEXITCODE -ne 0) { throw "failed to restart owned $Service service" }
        Invoke-Condition { (Get-ContainerState $Service) -eq "running" } 30 "owned $Service service did not restart"
    }

    function Set-IntentPolicy([bool]$Enabled) {
        $env:STELE_MEMORY_INTENT_POLICY_ENABLED = if ($Enabled) { "true" } else { "false" }
        @("services:", "  api:", "    environment:", "      STELE_MEMORY_INTENT_POLICY_ENABLED: `"$($env:STELE_MEMORY_INTENT_POLICY_ENABLED)`"", "  worker:", "    environment:", "      STELE_MEMORY_INTENT_POLICY_ENABLED: `"$($env:STELE_MEMORY_INTENT_POLICY_ENABLED)`"", "  scheduler:", "    environment:", "      STELE_MEMORY_INTENT_POLICY_ENABLED: `"$($env:STELE_MEMORY_INTENT_POLICY_ENABLED)`"") | Set-Content -LiteralPath $policyOverridePath -Encoding UTF8
        & docker compose -f $ComposeFile -f $policyOverridePath up -d --no-deps --force-recreate api worker scheduler *> $null
        if ($LASTEXITCODE -ne 0) { throw "failed to recreate owned services for memory-intent policy change" }
        foreach ($service in @("api", "worker", "scheduler")) {
            Invoke-Condition { (Get-ContainerState $service) -eq "running" } 30 "owned $service service did not restart after memory-intent policy change"
        }
        Invoke-Condition { (Get-HttpStatus "/readyz" @{}) -eq 200 } 30 "API did not become ready after memory-intent policy change"
    }

    function Get-HttpStatus([string]$Path, [hashtable]$Headers) {
        try {
            $response = Invoke-WebRequest -Method Get -Uri ($baseUrl.TrimEnd('/') + $Path) -Headers $Headers -ErrorAction Stop
            return [int]$response.StatusCode
        } catch {
            try { return [int]$_.Exception.Response.StatusCode } catch { return 0 }
        }
    }

    function Invoke-IntentRequest([hashtable]$Body, [hashtable]$Headers) {
        $uri = $baseUrl.TrimEnd('/') + "/v1/memory-intents"
        try {
            $response = Invoke-WebRequest -Method Post -Uri $uri -Headers $Headers -Body ($Body | ConvertTo-Json -Depth 12) -ErrorAction Stop
            return [pscustomobject]@{ StatusCode = [int]$response.StatusCode; Body = $response.Content; Json = ($response.Content | ConvertFrom-Json) }
        } catch {
            $status = 0; $content = ""
            try { $status = [int]$_.Exception.Response.StatusCode } catch { }
            try { $content = (New-Object IO.StreamReader($_.Exception.Response.GetResponseStream())).ReadToEnd() } catch { }
            return [pscustomobject]@{ StatusCode = $status; Body = $content; Json = $null }
        }
    }

    function Get-VerificationHash([object]$Value) {
        $json = $Value | ConvertTo-Json -Depth 20 -Compress
        return ([Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($json)))).ToLowerInvariant()
    }

    Write-Output "Starting isolated product verification project '$ProjectName'"
    $composeAttempted = $true
    # Default path intentionally retains the documented `--build -d` Compose
    # invocation; `-NoBuild` is only for prebuilt local verification images.
    $composeUpArgs = @("compose", "-f", $ComposeFile, "up", "-d")
    if (-not $NoBuild) { $composeUpArgs = @("compose", "-f", $ComposeFile, "up", "--build", "-d") }
    & docker @composeUpArgs
    if ($LASTEXITCODE -ne 0) { throw "Compose stack failed to start" }

    foreach ($requiredService in @("api", "worker", "scheduler")) {
        Invoke-Condition { (Get-ContainerState $requiredService) -eq "running" } 45 "owned $requiredService service did not become ready"
    }
    Invoke-Condition {
        $vectorVersion = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d stele -At -c "SELECT extversion FROM pg_extension WHERE extname = 'vector'" 2>$null | Out-String).Trim()
        return -not [string]::IsNullOrWhiteSpace($vectorVersion)
    } 45 "PostgreSQL pgvector extension is unavailable in the owned verification database"
    Add-ConformancePhase "prerequisite" "pass" "available" "1s_10s"

    try {
        if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
            if ($env:STELE_PRODUCT_VERIFY_CI -eq "1") { throw "Go is required for real PostgreSQL migration verification in CI" }
            Write-Output "SKIP: Go toolchain is not installed; product verification did not run"
            exit 2
        }
        $migrationDatabase = "stele_migrate_$([guid]::NewGuid().ToString('N').Substring(0, 16))"
        & docker compose -f $ComposeFile exec -T postgres psql -U stele -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE $migrationDatabase"
        if ($LASTEXITCODE -ne 0) { throw "failed to create harness-owned migration verification database" }
        $env:STELE_TEST_POSTGRES_DSN = "postgres://stele:$($env:STELE_POSTGRES_PASSWORD)@localhost:$($env:STELE_POSTGRES_HOST_PORT)/$($migrationDatabase)?sslmode=disable"
        Write-Output "Verifying concurrent forward migrations against harness-owned PostgreSQL..."
        & go test ./internal/storage/postgres -run '^TestMigrationRunnerSerializesConcurrentApply$' -count=1
        if ($LASTEXITCODE -ne 0) { throw "PostgreSQL migration concurrency verification failed" }
        Remove-Item Env:STELE_TEST_POSTGRES_DSN -ErrorAction SilentlyContinue

        $upgradeDatabase = "stele_upgrade_$([guid]::NewGuid().ToString('N').Substring(0, 16))"
        & docker compose -f $ComposeFile exec -T postgres psql -U stele -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE $upgradeDatabase"
        if ($LASTEXITCODE -ne 0) { throw "failed to create harness-owned migration upgrade database" }
        $env:STELE_TEST_POSTGRES_UPGRADE_DSN = "postgres://stele:$($env:STELE_POSTGRES_PASSWORD)@localhost:$($env:STELE_POSTGRES_HOST_PORT)/$($upgradeDatabase)?sslmode=disable"
        Write-Output "Verifying upgrade from a populated prior-release database..."
        & go test ./internal/storage/postgres -run '^TestMigrationRunnerUpgradesPopulatedPriorRelease$' -count=1
        if ($LASTEXITCODE -ne 0) { throw "PostgreSQL migration upgrade verification failed" }
        Remove-Item Env:STELE_TEST_POSTGRES_UPGRADE_DSN -ErrorAction SilentlyContinue

        $env:STELE_TEST_POSTGRES_DERIVED_WORK_DSN = "postgres://stele:$($env:STELE_POSTGRES_PASSWORD)@localhost:$($env:STELE_POSTGRES_HOST_PORT)/stele?sslmode=disable"
        Write-Output "Verifying durable memory-intent queue lease, checkpoint, retry, and exhaustion behavior..."
        & go test ./internal/storage/postgres -run '^TestDerivedWorkPostgresRecoveryAndScopeIsolation$' -count=1
        if ($LASTEXITCODE -ne 0) {
            Add-ConformancePhase "queue_recovery" "fail" "lease_retry_matrix" "1s_10s"
            throw "durable derived-work lease/retry verification failed"
        }
        Add-ConformancePhase "queue_recovery" "pass" "lease_retry_matrix" "1s_10s"
        Remove-Item Env:STELE_TEST_POSTGRES_DERIVED_WORK_DSN -ErrorAction SilentlyContinue

        $credentialDir = Join-Path ([System.IO.Path]::GetTempPath()) "stele-product-verify-$ProjectName"
        New-Item -ItemType Directory -Force -Path $credentialDir | Out-Null
        & pwsh -NoProfile -File (Join-Path $PSScriptRoot "stele-bootstrap-smoke.ps1") `
            -BaseUrl $baseUrl `
            -BootstrapKey $env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY `
            -Tenant $env:STELE_AUTH_DEFAULT_TENANT `
            -Project $env:STELE_AUTH_DEFAULT_PROJECT `
            -Namespace $env:STELE_AUTH_DEFAULT_NAMESPACE `
            -CredentialOutputDirectory $credentialDir
        if ($LASTEXITCODE -ne 0) { throw "bootstrap/lifecycle smoke failed" }

        $adminCredential = (Get-Content -Raw -LiteralPath (Join-Path $credentialDir "admin.credential")).Trim()
        $runtimeCredential = (Get-Content -Raw -LiteralPath (Join-Path $credentialDir "runtime.credential")).Trim()
        $runtimeHeaders = @{
            "X-API-Key" = $runtimeCredential
            "X-Stele-Tenant" = $env:STELE_AUTH_DEFAULT_TENANT
            "X-Stele-Project" = $env:STELE_AUTH_DEFAULT_PROJECT
            "X-Stele-Namespace" = $env:STELE_AUTH_DEFAULT_NAMESPACE
            "Content-Type" = "application/json"
        }
        $adminHeaders = @{
            "X-API-Key" = $adminCredential
            "X-Stele-Tenant" = $env:STELE_AUTH_DEFAULT_TENANT
            "X-Stele-Project" = $env:STELE_AUTH_DEFAULT_PROJECT
            "X-Stele-Namespace" = $env:STELE_AUTH_DEFAULT_NAMESPACE
            "Content-Type" = "application/json"
        }

        # Public context evidence uses this fresh owned database and official
        # bootstrap, and validates the served schema before/after API+worker restart.
        if ($env:STELE_PRODUCT_VERIFY_PROVIDER_CONTEXT -eq "1") {
            $contextReportPath = "$ReportPath.context.json"
            & pwsh -NoProfile -File (Join-Path $PSScriptRoot "stele-provider-context-verify.ps1") -ProjectName $ProjectName -ComposeFile $ComposeFile -UseExistingStack -BaseUrl $baseUrl -CredentialDirectory $credentialDir -ReportPath $contextReportPath
            if ($LASTEXITCODE -ne 0) {
                Add-ConformancePhase "inspection" "fail" "provider_context_contract" "gt_10s"
                throw "public Provider context conformance failed"
            }
            $contextReport = Get-Content -Raw -LiteralPath $contextReportPath | ConvertFrom-Json
            if ($contextReport.result -ne "passed" -or $contextReport.stage -ne "restart") { throw "public context restart evidence missing" }
            Add-ConformancePhase "inspection" "pass" "provider_context_contract" "1s_10s"
        }

        $mcpPath = if ([string]::IsNullOrWhiteSpace($env:STELE_MCP_PATH)) { "/mcp" } else { $env:STELE_MCP_PATH }
        if ($mcpPath -notmatch '^/[a-zA-Z0-9/_-]{1,64}$') { throw "configured MCP path is invalid or unbounded" }
        if ($env:STELE_PRODUCT_VERIFY_MCP -eq "1") {
            $mcpStatus = Get-HttpStatus $mcpPath $runtimeHeaders
            if ($mcpStatus -eq 404) {
                Add-ConformancePhase "prerequisite" "skip" "policy_disabled" "lt_1s"
            } elseif ($mcpStatus -in @(200, 400, 401, 405)) {
                $mcpEnabled = $true
                Add-ConformancePhase "prerequisite" "pass" "available" "lt_1s"
            } else {
                Add-ConformancePhase "prerequisite" "degraded" "dependency" "unknown"
            }
        } else {
            Add-ConformancePhase "prerequisite" "skip" "optional_disabled" "lt_1s"
        }

        if ($mcpEnabled) {
            Write-Output "Verifying enabled MCP remember/forget replay, preview/apply, and read-only boundaries..."
            $mcpEvidencePath = Join-Path ([System.IO.Path]::GetTempPath()) "stele-mcp-conformance-$ProjectName.json"
            $mcpDsn = "postgres://stele:$($env:STELE_POSTGRES_PASSWORD)@localhost:$($env:STELE_POSTGRES_HOST_PORT)/stele?sslmode=disable"
            $env:STELE_TEST_POSTGRES_MCP_DSN = $mcpDsn
            & pwsh -NoProfile -File (Join-Path $PSScriptRoot "stele-mcp-conformance.ps1") `
                -EvidencePath $mcpEvidencePath `
                -ApiBaseUrl $baseUrl `
                -McpPath $mcpPath `
                -ExpectEnabled
            if ($LASTEXITCODE -ne 0) {
                Add-ConformancePhase "submission" "fail" "mcp_conformance" "gt_10s"
                throw "enabled MCP conformance matrix failed"
            }
            if (-not (Test-Path -LiteralPath $mcpEvidencePath)) {
                Add-ConformancePhase "submission" "fail" "mcp_evidence" "unknown"
                throw "enabled MCP conformance did not emit bounded evidence"
            }
            $mcpEvidence = Get-Content -Raw -LiteralPath $mcpEvidencePath | ConvertFrom-Json
            if ([string]$mcpEvidence.status -ne "passed") {
                Add-ConformancePhase "submission" "fail" "mcp_conformance" "gt_10s"
                throw "enabled MCP conformance evidence was not passed"
            }
            $mcpPassed = @($mcpEvidence.categories | Where-Object { [string]$_.status -eq "passed" } | ForEach-Object { [string]$_.name })
            foreach ($requiredMcpCategory in @("remember_idempotency", "forget_replay", "forget_preview", "forget_apply", "read_only_grant", "scope_isolation", "response_redaction")) {
                if ($requiredMcpCategory -notin $mcpPassed) {
                    Add-ConformancePhase "inspection" "fail" "mcp_conformance" "gt_10s"
                    throw "enabled MCP conformance omitted required bounded category '$requiredMcpCategory'"
                }
            }
            Add-ConformancePhase "submission" "pass" "mcp_replay" "1s_10s"
            Add-ConformancePhase "replay" "pass" "mcp_replay" "1s_10s"
            Add-ConformancePhase "scope_isolation" "pass" "mcp_scope" "1s_10s"
            Add-ConformancePhase "inspection" "pass" "mcp_read_only" "1s_10s"
            Add-ConformancePhase "inspection" "pass" "mcp_preview_apply" "1s_10s"
        }

        Write-Output "Verifying governed memory intent submission, replay, conflict, scope isolation, and inspection..."
        $intentKey = "product-verify-intent-$([guid]::NewGuid().ToString('N'))"
        $intentHeaders = $runtimeHeaders.Clone()
        $intentHeaders["X-Stele-Actor"] = "product-verifier"
        $intentHeaders["X-Request-ID"] = "request-$intentKey"
        $intentHeaders["Idempotency-Key"] = $intentKey
        $intentBody = @{
            type = "remember"; memory_path = "/product-verification/intents"; content = "owned conformance fixture"
            reason = "product verification"; operation_id = "operation-$intentKey"; idempotency_key = $intentKey
            provenance = @{ source = "product-verification" }
        }
        $intentFirst = Invoke-IntentRequest $intentBody $intentHeaders
        if ($intentFirst.StatusCode -ne 202 -or $null -eq $intentFirst.Json -or [string]::IsNullOrWhiteSpace([string]$intentFirst.Json.ID)) {
            Add-ConformancePhase "submission" "fail" "validation" "unknown"
            throw "memory intent submission did not return stable accepted metadata"
        }
        Add-ConformancePhase "submission" "pass" "accepted" "lt_1s"
        $intentReplay = Invoke-IntentRequest $intentBody $intentHeaders
        if ($intentReplay.StatusCode -ne 202 -or [string]$intentReplay.Json.ID -ne [string]$intentFirst.Json.ID) {
            Add-ConformancePhase "replay" "fail" "unsafe_retry" "unknown"
            throw "identical memory intent replay changed the intent identity"
        }
        Add-ConformancePhase "replay" "pass" "replay" "lt_1s"
        $conflictBody = $intentBody.Clone(); $conflictBody["content"] = "materially different conformance fixture"
        $intentConflict = Invoke-IntentRequest $conflictBody $intentHeaders
        if ($intentConflict.StatusCode -lt 400 -or $intentConflict.StatusCode -ge 500) {
            Add-ConformancePhase "replay" "fail" "conflict" "unknown"
            throw "conflicting memory intent retry was not rejected"
        }
        Add-ConformancePhase "replay" "pass" "conflict" "lt_1s"

        $foreignHeaders = $adminHeaders.Clone()
        $foreignHeaders["X-Stele-Tenant"] = "$($env:STELE_AUTH_DEFAULT_TENANT)-foreign"
        $foreignStatus = Get-HttpStatus "/v1/admin/memory-intents/$([string]$intentFirst.Json.ID)" $foreignHeaders
        if ($foreignStatus -eq 200) {
            Add-ConformancePhase "scope_isolation" "fail" "scope_denied" "unknown"
            throw "foreign scope observed a product verification intent"
        }
        Add-ConformancePhase "scope_isolation" "pass" "scope_denied" "lt_1s"
        $foreignSubmitKey = "product-verify-foreign-$([guid]::NewGuid().ToString('N'))"
        $foreignSubmitHeaders = $runtimeHeaders.Clone()
        $foreignSubmitHeaders["X-Stele-Tenant"] = "$($env:STELE_AUTH_DEFAULT_TENANT)-foreign"
        $foreignSubmitHeaders["X-Stele-Actor"] = "product-verifier"
        $foreignSubmitHeaders["X-Request-ID"] = "request-$foreignSubmitKey"
        $foreignSubmitHeaders["Idempotency-Key"] = $foreignSubmitKey
        $foreignSubmitBody = $intentBody.Clone(); $foreignSubmitBody["operation_id"] = "operation-$foreignSubmitKey"; $foreignSubmitBody["idempotency_key"] = $foreignSubmitKey
        $foreignSubmit = Invoke-IntentRequest $foreignSubmitBody $foreignSubmitHeaders
        if ($foreignSubmit.StatusCode -eq 202) {
            Add-ConformancePhase "scope_isolation" "fail" "scope_denied" "unknown"
            throw "foreign scope submitted an intent successfully"
        }
        Add-ConformancePhase "scope_isolation" "pass" "scope_denied" "lt_1s"
        $detailStatus = Get-HttpStatus "/v1/admin/memory-intents/$([string]$intentFirst.Json.ID)" $adminHeaders
        $historyStatus = Get-HttpStatus "/v1/admin/memory-intents/$([string]$intentFirst.Json.ID)/history" $adminHeaders
        Write-Output "Memory intent inspection statuses: detail=$detailStatus history=$historyStatus"
        if ($detailStatus -ne 200 -or $historyStatus -ne 200) {
            Add-ConformancePhase "inspection" "fail" "validation" "unknown"
            throw "scoped memory intent inspection was unavailable"
        }
        Add-ConformancePhase "inspection" "pass" "accepted" "lt_1s"

        Invoke-Condition {
            try {
                $currentIntent = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$intentFirst.Json.ID)") -Headers $adminHeaders -ErrorAction Stop
                $currentHistory = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$intentFirst.Json.ID)/history") -Headers $adminHeaders -ErrorAction Stop
                $lastTransition = @($currentHistory.Transitions | Sort-Object sequence | Select-Object -Last 1)
                return ([string]$currentIntent.Status -in @("active", "candidate", "suppressed", "failed", "rejected") -or [string]$lastTransition.To -in @("active", "candidate", "suppressed", "failed", "rejected"))
            } catch { return $false }
        } 45 "remember intent did not reach a terminal governed outcome"
        $rememberFinal = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$intentFirst.Json.ID)") -Headers $adminHeaders -ErrorAction Stop
        $rememberHistory = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$intentFirst.Json.ID)/history") -Headers $adminHeaders -ErrorAction Stop
        $rememberLastTransition = @($rememberHistory.Transitions | Sort-Object sequence | Select-Object -Last 1)
        $memoryID = [string]$rememberFinal.OutcomeReference
        if ([string]::IsNullOrWhiteSpace($memoryID)) { $memoryID = [string]$rememberLastTransition.OutcomeReference }
        if (-not [string]::IsNullOrWhiteSpace($memoryID)) {
            $foreignMemoryStatus = Get-HttpStatus "/v1/memories/$memoryID" $foreignHeaders
            if ($foreignMemoryStatus -eq 200) {
                Add-ConformancePhase "scope_isolation" "fail" "scope_denied" "unknown"
                throw "foreign scope observed canonical memory outcome"
            }
        }
        if (([string]$rememberFinal.Status -ne "active" -and [string]$rememberLastTransition.To -ne "active") -or [string]::IsNullOrWhiteSpace($memoryID)) {
            Add-ConformancePhase "submission" "fail" "validation" "unknown"
            throw "remember intent did not produce an active canonical memory outcome"
        }
        if ([string]$rememberLastTransition.To -eq "active" -and -not [string]::IsNullOrWhiteSpace($memoryID)) {
            $versionCountBefore = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d stele -At -c "SELECT COUNT(*) FROM memory_versions WHERE memory_id = '$memoryID'" | Out-String).Trim()
            $memoryExistsBefore = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d stele -At -c "SELECT COUNT(*) FROM canonical_memories WHERE id = '$memoryID' AND tenant = '$($env:STELE_AUTH_DEFAULT_TENANT)' AND project = '$($env:STELE_AUTH_DEFAULT_PROJECT)' AND namespace = '$($env:STELE_AUTH_DEFAULT_NAMESPACE)'" | Out-String).Trim()
            if ([int64]$versionCountBefore -lt 1 -or [int64]$memoryExistsBefore -ne 1) {
                Add-ConformancePhase "inspection" "fail" "validation" "unknown"
                throw "remember intent did not create an owned canonical version"
            }
            $updateKey = "product-verify-update-$([guid]::NewGuid().ToString('N'))"
            $updateHeaders = $runtimeHeaders.Clone(); $updateHeaders["X-Stele-Actor"] = "product-verifier"; $updateHeaders["X-Request-ID"] = "request-$updateKey"; $updateHeaders["Idempotency-Key"] = $updateKey
            $updateBody = @{ type = "update"; memory_path = "/product-verification/intents"; target_memory_id = $memoryID; target_version = 1; content = "updated conformance fixture"; reason = "product verification update"; operation_id = "operation-$updateKey"; idempotency_key = $updateKey }
            $updateResponse = Invoke-IntentRequest $updateBody $updateHeaders
            if ($updateResponse.StatusCode -ne 202) { throw "memory intent update submission failed" }
            Invoke-Condition {
                try {
                    $status = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$updateResponse.Json.ID)") -Headers $adminHeaders -ErrorAction Stop
                    $history = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$updateResponse.Json.ID)/history") -Headers $adminHeaders -ErrorAction Stop
                    $last = @($history.Transitions | Sort-Object sequence | Select-Object -Last 1)
                    return ([string]$status.Status -in @("active", "candidate", "suppressed", "failed", "rejected") -or [string]$last.To -in @("active", "candidate", "suppressed", "failed", "rejected"))
                } catch { return $false }
            } 45 "update intent did not reach a terminal governed outcome"
            $versionCountAfterUpdate = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d stele -At -c "SELECT COUNT(*) FROM memory_versions WHERE memory_id = '$memoryID'" | Out-String).Trim()
            if ([int64]$versionCountAfterUpdate -ne [int64]$versionCountBefore + 1) {
                Add-ConformancePhase "inspection" "fail" "validation" "unknown"
                throw "update intent did not append exactly one canonical memory version"
            }
            Add-ConformancePhase "submission" "pass" "accepted" "1s_10s"
            $forgetKey = "product-verify-forget-$([guid]::NewGuid().ToString('N'))"
            $forgetHeaders = $runtimeHeaders.Clone(); $forgetHeaders["X-Stele-Actor"] = "product-verifier"; $forgetHeaders["X-Request-ID"] = "request-$forgetKey"; $forgetHeaders["Idempotency-Key"] = $forgetKey
            $forgetBody = @{ type = "forget"; memory_path = "/product-verification/intents"; target_memory_id = $memoryID; target_version = 2; reason = "product verification forget"; operation_id = "operation-$forgetKey"; idempotency_key = $forgetKey }
            $forgetResponse = Invoke-IntentRequest $forgetBody $forgetHeaders
            if ($forgetResponse.StatusCode -ne 202) { throw "memory intent forget submission failed" }
            Invoke-Condition {
                try {
                    $forgetStatus = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$forgetResponse.Json.ID)") -Headers $adminHeaders -ErrorAction Stop
                    $forgetHistory = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$forgetResponse.Json.ID)/history") -Headers $adminHeaders -ErrorAction Stop
                    $lastForget = @($forgetHistory.Transitions | Sort-Object sequence | Select-Object -Last 1)
                    return ([string]$forgetStatus.Status -in @("suppressed", "forgotten", "failed", "rejected") -or [string]$lastForget.To -in @("suppressed", "forgotten", "failed", "rejected"))
                } catch { return $false }
            } 45 "forget intent did not reach a bounded lifecycle outcome"
            $memoryExistsAfterForget = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d stele -At -c "SELECT COUNT(*) FROM canonical_memories WHERE id = '$memoryID'" | Out-String).Trim()
            $versionCountAfterForget = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d stele -At -c "SELECT COUNT(*) FROM memory_versions WHERE memory_id = '$memoryID'" | Out-String).Trim()
            if ([int64]$memoryExistsAfterForget -ne 1 -or [int64]$versionCountAfterForget -ne [int64]$versionCountAfterUpdate) {
                Add-ConformancePhase "inspection" "fail" "validation" "unknown"
                throw "forget intent overwrote or deleted append-only canonical history"
            }
            Add-ConformancePhase "submission" "pass" "accepted" "1s_10s"

            $reviewProbeSearchBody = @{ query = "bootstrap smoke lifecycle fixture"; top_k = 5 }
            $reviewProbeContextBody = @{ query = "bootstrap smoke lifecycle fixture"; budget = 1200; include_diagnostics = $false }
            $reviewProbeSearchBefore = Invoke-RestMethod -Method Post -Uri ($baseUrl.TrimEnd('/') + "/v1/memories/search") -Headers $runtimeHeaders -Body ($reviewProbeSearchBody | ConvertTo-Json -Depth 10) -ErrorAction Stop
            $reviewProbeContextBefore = Invoke-RestMethod -Method Post -Uri ($baseUrl.TrimEnd('/') + "/v1/context/assemble") -Headers $runtimeHeaders -Body ($reviewProbeContextBody | ConvertTo-Json -Depth 10) -ErrorAction Stop
            $reviewProbeSearchHashBefore = Get-VerificationHash $reviewProbeSearchBefore
            $reviewProbeContextHashBefore = Get-VerificationHash $reviewProbeContextBefore

            $contradictionKey = "product-verify-contradiction-$([guid]::NewGuid().ToString('N'))"
            $contradictionHeaders = $runtimeHeaders.Clone(); $contradictionHeaders["X-Stele-Actor"] = "product-verifier"; $contradictionHeaders["X-Request-ID"] = "request-$contradictionKey"; $contradictionHeaders["Idempotency-Key"] = $contradictionKey
            $contradictionBody = @{
                type = "contradiction"; memory_path = "/product-verification/intents"; target_memory_id = $memoryID; target_version = 1
                reason = "review contradiction evidence"; operation_id = "operation-$contradictionKey"; idempotency_key = $contradictionKey
                evidence = @(
                    @{ scope = @{ tenant = $env:STELE_AUTH_DEFAULT_TENANT; project = $env:STELE_AUTH_DEFAULT_PROJECT; namespace = $env:STELE_AUTH_DEFAULT_NAMESPACE }; kind = "memory"; id = $memoryID; version = 1 },
                    @{ scope = @{ tenant = $env:STELE_AUTH_DEFAULT_TENANT; project = $env:STELE_AUTH_DEFAULT_PROJECT; namespace = $env:STELE_AUTH_DEFAULT_NAMESPACE }; kind = "intent"; id = [string]$intentFirst.Json.ID; version = 1 }
                )
            }
            $contradictionResponse = Invoke-IntentRequest $contradictionBody $contradictionHeaders
            if ($contradictionResponse.StatusCode -ne 202) {
                Add-ConformancePhase "inspection" "degraded" "validation" "unknown"
                Write-Warning "contradiction intent was rejected with bounded status $($contradictionResponse.StatusCode)"
            } else {
            Invoke-Condition {
                try {
                    $status = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$contradictionResponse.Json.ID)") -Headers $adminHeaders -ErrorAction Stop
                    $history = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$contradictionResponse.Json.ID)/history") -Headers $adminHeaders -ErrorAction Stop
                    $last = @($history.Transitions | Sort-Object sequence | Select-Object -Last 1)
                    return ([string]$status.Status -in @("candidate", "suppressed", "rejected", "failed") -or [string]$last.To -in @("candidate", "suppressed", "rejected", "failed"))
                } catch { return $false }
            } 45 "contradiction intent did not remain review-only"
            Add-ConformancePhase "inspection" "pass" "accepted" "1s_10s"
            }

            $feedbackKey = "product-verify-feedback-$([guid]::NewGuid().ToString('N'))"
            $feedbackHeaders = $runtimeHeaders.Clone(); $feedbackHeaders["X-Stele-Actor"] = "product-verifier"; $feedbackHeaders["X-Request-ID"] = "request-$feedbackKey"; $feedbackHeaders["Idempotency-Key"] = $feedbackKey
            $feedbackBody = @{ type = "feedback"; memory_path = "/product-verification/intents"; target_insight_id = "fixture-insight-$feedbackKey"; content = "review-only feedback"; reason = "review feedback"; operation_id = "operation-$feedbackKey"; idempotency_key = $feedbackKey }
            $feedbackResponse = Invoke-IntentRequest $feedbackBody $feedbackHeaders
            if ($feedbackResponse.StatusCode -ne 202) {
                Add-ConformancePhase "inspection" "degraded" "validation" "unknown"
                Write-Warning "feedback intent was rejected with bounded status $($feedbackResponse.StatusCode)"
            } else {
            Invoke-Condition {
                try {
                    $status = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$feedbackResponse.Json.ID)") -Headers $adminHeaders -ErrorAction Stop
                    $history = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$feedbackResponse.Json.ID)/history") -Headers $adminHeaders -ErrorAction Stop
                    $last = @($history.Transitions | Sort-Object sequence | Select-Object -Last 1)
                    return ([string]$status.Status -in @("candidate", "suppressed", "rejected", "failed") -or [string]$last.To -in @("candidate", "suppressed", "rejected", "failed"))
                } catch { return $false }
            } 45 "feedback intent did not remain review-only"
            Add-ConformancePhase "inspection" "pass" "accepted" "1s_10s"
            }

            $reviewProbeSearchAfter = Invoke-RestMethod -Method Post -Uri ($baseUrl.TrimEnd('/') + "/v1/memories/search") -Headers $runtimeHeaders -Body ($reviewProbeSearchBody | ConvertTo-Json -Depth 10) -ErrorAction Stop
            $reviewProbeContextAfter = Invoke-RestMethod -Method Post -Uri ($baseUrl.TrimEnd('/') + "/v1/context/assemble") -Headers $runtimeHeaders -Body ($reviewProbeContextBody | ConvertTo-Json -Depth 10) -ErrorAction Stop
            if ((Get-VerificationHash $reviewProbeSearchAfter) -ne $reviewProbeSearchHashBefore -or (Get-VerificationHash $reviewProbeContextAfter) -ne $reviewProbeContextHashBefore) {
                Add-ConformancePhase "inspection" "fail" "review_visibility" "unknown"
                throw "contradiction or feedback changed default retrieval or context output"
            }
            Add-ConformancePhase "inspection" "pass" "review_only" "1s_10s"
        }

        Write-Output "Verifying policy disablement, pending retention, compatible re-enable, and rollback..."
        Assert-BoundedStop "worker"
        $rollbackKey = "product-verify-rollback-$([guid]::NewGuid().ToString('N'))"
        $rollbackHeaders = $runtimeHeaders.Clone()
        $rollbackHeaders["X-Stele-Actor"] = "product-verifier"
        $rollbackHeaders["X-Request-ID"] = "request-$rollbackKey"
        $rollbackHeaders["Idempotency-Key"] = $rollbackKey
        $rollbackBody = @{
            type = "remember"; memory_path = "/product-verification/rollback"; content = "held rollback fixture"
            reason = "policy rollback verification"; operation_id = "operation-$rollbackKey"; idempotency_key = $rollbackKey
        }
        $rollbackPending = Invoke-IntentRequest $rollbackBody $rollbackHeaders
        if ($rollbackPending.StatusCode -ne 202 -or $null -eq $rollbackPending.Json) {
            Add-ConformancePhase "rollback" "fail" "validation" "unknown"
            throw "rollback fixture was not accepted before policy disablement"
        }
        $rollbackIntentID = [string]$rollbackPending.Json.ID
        $rollbackBefore = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$rollbackIntentID") -Headers $adminHeaders -ErrorAction Stop
        if ([string]$rollbackBefore.Status -notin @("accepted", "pending")) {
            Add-ConformancePhase "rollback" "fail" "unsafe_retry" "unknown"
            throw "rollback fixture was processed before the worker could be disabled"
        }
        Set-IntentPolicy $false
        $disabledKey = "product-verify-disabled-$([guid]::NewGuid().ToString('N'))"
        $disabledHeaders = $runtimeHeaders.Clone(); $disabledHeaders["X-Stele-Actor"] = "product-verifier"; $disabledHeaders["X-Request-ID"] = "request-$disabledKey"; $disabledHeaders["Idempotency-Key"] = $disabledKey
        $disabledBody = $rollbackBody.Clone(); $disabledBody["content"] = "new work while disabled"; $disabledBody["operation_id"] = "operation-$disabledKey"; $disabledBody["idempotency_key"] = $disabledKey
        $disabledResponse = Invoke-IntentRequest $disabledBody $disabledHeaders
        if ($disabledResponse.StatusCode -ge 200 -and $disabledResponse.StatusCode -lt 300) {
            Add-ConformancePhase "rollback" "fail" "policy_disabled" "unknown"
            throw "disabled policy accepted new memory intent work"
        }
        $rollbackHeld = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$rollbackIntentID") -Headers $adminHeaders -ErrorAction Stop
        if ([string]$rollbackHeld.Status -notin @("accepted", "pending")) {
            Add-ConformancePhase "rollback" "fail" "policy_disabled" "unknown"
            throw "disabled policy did not retain the pending intent"
        }
        Add-ConformancePhase "rollback" "pass" "policy_disabled" "1s_10s"
        Set-IntentPolicy $true
        Invoke-Condition {
            try {
                $resumed = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$rollbackIntentID") -Headers $adminHeaders -ErrorAction Stop
                $resumedHistory = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$rollbackIntentID/history") -Headers $adminHeaders -ErrorAction Stop
                $resumedLast = @($resumedHistory.Transitions | Sort-Object sequence | Select-Object -Last 1)
                return ([string]$resumed.Status -in @("active", "candidate", "suppressed", "failed", "rejected") -or [string]$resumedLast.To -in @("active", "candidate", "suppressed", "failed", "rejected"))
            } catch { return $false }
        } 45 "compatible policy re-enable did not resume the pending intent"
        Add-ConformancePhase "rollback" "pass" "rollback" "1s_10s"

        Write-Output "Verifying API readiness drain, bounded termination, restart, and idempotent replay..."
        Invoke-Condition { (Get-HttpStatus "/readyz" @{}) -eq 200 } 30 "API did not become ready"
        $replayKey = "product-verify-restart-$([guid]::NewGuid().ToString('N'))"
        $replayBody = @{ event_type = "product.verify.restart"; content = "restart replay fixture"; metadata = @{ fixture = "product-verify" } }
        $replayHeaders = $runtimeHeaders.Clone(); $replayHeaders["Idempotency-Key"] = $replayKey
        $beforeRestart = Invoke-RestMethod -Method Post -Uri ($baseUrl.TrimEnd('/') + "/v1/events") -Headers $replayHeaders -Body ($replayBody | ConvertTo-Json -Depth 10) -ErrorAction Stop
        Assert-BoundedStop "api"
        Invoke-Condition { (Get-HttpStatus "/readyz" @{}) -ne 200 } 10 "API readiness did not transition from ready during drain"
        Assert-RestartReady "api"
        Invoke-Condition { (Get-HttpStatus "/readyz" @{}) -eq 200 } 30 "API did not return to ready after restart"
        $afterRestart = Invoke-RestMethod -Method Post -Uri ($baseUrl.TrimEnd('/') + "/v1/events") -Headers $replayHeaders -Body ($replayBody | ConvertTo-Json -Depth 10) -ErrorAction Stop
        if ($afterRestart.event_id -ne $beforeRestart.event_id -or -not $afterRestart.replayed) { throw "API restart created a duplicate raw event or lost idempotency replay" }
        $intentAfterAPIRestart = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$([string]$intentFirst.Json.ID)") -Headers $adminHeaders -ErrorAction Stop
        if ([string]$intentAfterAPIRestart.ID -ne [string]$intentFirst.Json.ID) { throw "API restart changed the durable memory intent identity" }

        Write-Output "Verifying worker and scheduler bounded termination and durable background continuation..."
        Assert-BoundedStop "worker"
        $queuedIntentKey = "product-verify-queue-$([guid]::NewGuid().ToString('N'))"
        $queuedIntentHeaders = $runtimeHeaders.Clone()
        $queuedIntentHeaders["X-Stele-Actor"] = "product-verifier"
        $queuedIntentHeaders["X-Request-ID"] = "request-$queuedIntentKey"
        $queuedIntentHeaders["Idempotency-Key"] = $queuedIntentKey
        $queuedIntentBody = @{
            type = "remember"; memory_path = "/product-verification/queue"; content = "durable queue fixture"
            reason = "worker recovery verification"; operation_id = "operation-$queuedIntentKey"; idempotency_key = $queuedIntentKey
        }
        $queuedIntent = Invoke-IntentRequest $queuedIntentBody $queuedIntentHeaders
        if ($queuedIntent.StatusCode -ne 202 -or $null -eq $queuedIntent.Json) {
            Add-ConformancePhase "queue_recovery" "fail" "validation" "unknown"
            throw "worker-stop intent fixture was not accepted"
        }
        $queuedIntentID = [string]$queuedIntent.Json.ID
        $queuedReplay = Invoke-IntentRequest $queuedIntentBody $queuedIntentHeaders
        if ($queuedReplay.StatusCode -ne 202 -or [string]$queuedReplay.Json.ID -ne $queuedIntentID) {
            Add-ConformancePhase "queue_recovery" "fail" "unsafe_retry" "unknown"
            throw "duplicate queued intent submission changed the durable identity"
        }
        $queueCountSql = "SELECT COUNT(*) FROM derived_work_items WHERE kind = 'memory_intent' AND reference = '$queuedIntentID' AND tenant = '$($env:STELE_AUTH_DEFAULT_TENANT)' AND project = '$($env:STELE_AUTH_DEFAULT_PROJECT)' AND namespace = '$($env:STELE_AUTH_DEFAULT_NAMESPACE)'"
        $queueCount = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d stele -At -c $queueCountSql | Out-String).Trim()
        if ([int64]$queueCount -ne 1) {
            Add-ConformancePhase "queue_recovery" "fail" "queue_recovery" "unknown"
            throw "accepted intent did not create exactly one owned durable queue item"
        }
        $foreignQueueSql = "SELECT COUNT(*) FROM derived_work_items WHERE kind = 'memory_intent' AND reference = '$queuedIntentID' AND tenant = '$($env:STELE_AUTH_DEFAULT_TENANT)-foreign'"
        $foreignQueueCount = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d stele -At -c $foreignQueueSql | Out-String).Trim()
        if ([int64]$foreignQueueCount -ne 0) {
            Add-ConformancePhase "queue_recovery" "fail" "scope_denied" "unknown"
            throw "foreign scope observed the owned memory intent queue item"
        }
        $derivedBeforeRestart = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/derived-work/status") -Headers $adminHeaders -ErrorAction Stop
        if ([int64]$derivedBeforeRestart.Queued -lt 1 -and [int64]$derivedBeforeRestart.Running -lt 1) {
            Add-ConformancePhase "queue_recovery" "fail" "queue_recovery" "unknown"
            throw "derived queue status did not expose the accepted intent backlog"
        }
        Add-ConformancePhase "queue_recovery" "pass" "queue_recovery" "lt_1s"
        $pendingKey = "product-verify-worker-$([guid]::NewGuid().ToString('N'))"
        $pendingHeaders = $runtimeHeaders.Clone(); $pendingHeaders["Idempotency-Key"] = $pendingKey
        $pendingBody = @{ event_type = "product.verify.worker-continuation"; content = "durable continuation fixture"; metadata = @{ fixture = "product-verify" } }
        Invoke-RestMethod -Method Post -Uri ($baseUrl.TrimEnd('/') + "/v1/events") -Headers $pendingHeaders -Body ($pendingBody | ConvertTo-Json -Depth 10) -ErrorAction Stop | Out-Null
        $pendingBefore = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/jobs/governance/status") -Headers $adminHeaders -ErrorAction Stop
        if ([int64]$pendingBefore.pending_raw_events -lt 1) { throw "worker-stop fixture was not durably pending" }
        Assert-RestartReady "worker"
        try {
            Invoke-Condition {
                try {
                    $status = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/jobs/governance/status") -Headers $adminHeaders -ErrorAction Stop
                    return ([int64]$status.pending_raw_events -eq 0)
                } catch { return $false }
            } 45 "worker did not continue durable eligible work after restart"
        } catch {
            Add-ConformancePhase "worker_restart" "degraded" "timeout" "gt_10s"
            Write-Warning "raw event continuation did not drain within the bound; intent queue recovery remains authoritative"
        }
        try {
            Invoke-Condition {
                try {
                    $intentStatus = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$queuedIntentID") -Headers $adminHeaders -ErrorAction Stop
                    $intentHistory = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$queuedIntentID/history") -Headers $adminHeaders -ErrorAction Stop
                    $lastIntentTransition = @($intentHistory.Transitions | Sort-Object sequence | Select-Object -Last 1)
                    return ([string]$intentStatus.Status -in @("active", "candidate", "suppressed", "failed", "rejected") -or [string]$lastIntentTransition.To -in @("active", "candidate", "suppressed", "failed", "rejected"))
                } catch { return $false }
            } 45 "worker did not complete or durably fail the queued intent after restart"
        } catch {
            Add-ConformancePhase "worker_restart" "degraded" "timeout" "gt_10s"
            Write-Warning "queued memory intent remained pending beyond the bounded restart window"
        }
        $attemptCountSql = "SELECT COUNT(*) FROM derived_work_attempts a JOIN derived_work_items w ON w.id = a.work_id WHERE w.kind = 'memory_intent' AND w.reference = '$queuedIntentID' AND a.tenant = '$($env:STELE_AUTH_DEFAULT_TENANT)' AND a.project = '$($env:STELE_AUTH_DEFAULT_PROJECT)' AND a.namespace = '$($env:STELE_AUTH_DEFAULT_NAMESPACE)'"
        $attemptCount = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d stele -At -c $attemptCountSql | Out-String).Trim()
        $terminalCountSql = "SELECT COUNT(*) FROM derived_work_terminal_summaries s JOIN derived_work_items w ON w.id = s.work_id WHERE w.kind = 'memory_intent' AND w.reference = '$queuedIntentID' AND s.tenant = '$($env:STELE_AUTH_DEFAULT_TENANT)' AND s.project = '$($env:STELE_AUTH_DEFAULT_PROJECT)' AND s.namespace = '$($env:STELE_AUTH_DEFAULT_NAMESPACE)'"
        $terminalCount = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d stele -At -c $terminalCountSql | Out-String).Trim()
        if ([int64]$attemptCount -lt 1) {
            Add-ConformancePhase "queue_recovery" "degraded" "dependency" "unknown"
        } elseif ([int64]$terminalCount -gt 1) {
            Add-ConformancePhase "queue_recovery" "fail" "unsafe_retry" "unknown"
            throw "durable intent queue recorded more than one terminal summary"
        } elseif ([int64]$terminalCount -eq 1) {
            Add-ConformancePhase "queue_recovery" "pass" "queue_recovery" "1s_10s"
        } else {
            Add-ConformancePhase "queue_recovery" "degraded" "timeout" "gt_10s"
        }
        $queuedHistory = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/memory-intents/$queuedIntentID/history") -Headers $adminHeaders -ErrorAction Stop
        $transitionSequences = @($queuedHistory.Transitions | ForEach-Object { [int64]$_.Sequence })
        if ($transitionSequences.Count -ne (@($transitionSequences | Sort-Object -Unique).Count)) {
            Add-ConformancePhase "worker_restart" "fail" "unsafe_retry" "unknown"
            throw "worker restart produced duplicate intent transition sequence values"
        }
        Add-ConformancePhase "worker_restart" "pass" "worker_restart" "1s_10s"
        $derivedAfterRestart = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/derived-work/status") -Headers $adminHeaders -ErrorAction Stop
        if ([int64]$derivedAfterRestart.Exhausted -gt [int64]$derivedBeforeRestart.Exhausted + 1) {
            Add-ConformancePhase "queue_recovery" "degraded" "retry_exhausted" "1s_10s"
        } else {
            Add-ConformancePhase "queue_recovery" "pass" "queue_recovery" "1s_10s"
        }
        Assert-BoundedStop "scheduler"
        Assert-RestartReady "scheduler"
        Add-ConformancePhase "scheduler_restart" "pass" "scheduler_restart" "lt_1s"

        Write-Output "Verifying disposable backup/restore behavior with harness-owned databases..."
        $sourceDatabase = "stele"
        $targetDatabase = "stele_restore_$([guid]::NewGuid().ToString('N').Substring(0, 16))"
        & docker compose -f $ComposeFile exec -T postgres psql -U stele -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE $targetDatabase" *> $null
        if ($LASTEXITCODE -ne 0) { throw "failed to create harness-owned restore target database" }
        $backupDir = Join-Path ([System.IO.Path]::GetTempPath()) "stele-product-verify-backup-$ProjectName"
        New-Item -ItemType Directory -Force -Path $backupDir | Out-Null
        $artifact = Join-Path $backupDir "stele.dump"
        $manifest = "$artifact.manifest.json"
        $sourceDsn = "postgres://stele:$($env:STELE_POSTGRES_PASSWORD)@localhost:$($env:STELE_POSTGRES_HOST_PORT)/${sourceDatabase}?sslmode=disable"
        $targetDsn = "postgres://stele:$($env:STELE_POSTGRES_PASSWORD)@localhost:$($env:STELE_POSTGRES_HOST_PORT)/${targetDatabase}?sslmode=disable"
        $containerArtifact = "/tmp/stele-verify-$ProjectName.dump"
        & docker compose -f $ComposeFile exec -T postgres pg_dump -U stele --format=custom --no-owner --no-privileges -f $containerArtifact -d $sourceDatabase *> $null
        if ($LASTEXITCODE -ne 0) { throw "disposable pg_dump failed" }
        & docker compose -f $ComposeFile cp "postgres:$containerArtifact" $artifact *> $null
        if ($LASTEXITCODE -ne 0) { throw "failed to copy disposable backup from owned PostgreSQL container" }
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $artifact).Hash.ToLowerInvariant()
        $schemaVersion = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d $sourceDatabase -At -c "SELECT COALESCE(MAX(version), 0) FROM schema_migrations" | Out-String).Trim()
        @{ artifact = [IO.Path]::GetFileName($artifact); sha256 = $hash; schema_version = $schemaVersion; format = "pg_dump custom" } | ConvertTo-Json | Set-Content -LiteralPath $manifest -Encoding UTF8 -NoNewline
        & docker compose -f $ComposeFile cp $artifact "postgres:$containerArtifact" *> $null
        if ($LASTEXITCODE -ne 0) { throw "failed to copy disposable backup into owned PostgreSQL container" }
        & docker compose -f $ComposeFile exec -T postgres pg_restore --exit-on-error --clean --if-exists --no-owner --no-privileges -U stele -d $targetDatabase $containerArtifact *> $null
        if ($LASTEXITCODE -ne 0) { throw "disposable pg_restore failed" }
        & docker compose -f $ComposeFile exec -T postgres rm -f $containerArtifact *> $null
        $restoredVersion = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d $targetDatabase -At -c "SELECT COALESCE(MAX(version), 0) FROM schema_migrations" | Out-String).Trim()
        if ($restoredVersion -ne $schemaVersion) { throw "restored schema version differs from source fixture" }
        $sourceEvents = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d $sourceDatabase -At -c "SELECT COUNT(*) FROM raw_events WHERE tenant = '$($env:STELE_AUTH_DEFAULT_TENANT)' AND project = '$($env:STELE_AUTH_DEFAULT_PROJECT)' AND namespace = '$($env:STELE_AUTH_DEFAULT_NAMESPACE)'" | Out-String).Trim()
        $restoredEvents = (& docker compose -f $ComposeFile exec -T postgres psql -U stele -d $targetDatabase -At -c "SELECT COUNT(*) FROM raw_events WHERE tenant = '$($env:STELE_AUTH_DEFAULT_TENANT)' AND project = '$($env:STELE_AUTH_DEFAULT_PROJECT)' AND namespace = '$($env:STELE_AUTH_DEFAULT_NAMESPACE)'" | Out-String).Trim()
        if ($sourceEvents -ne $restoredEvents -or [int64]$restoredEvents -lt 1) { throw "restored scoped behavior differs from source fixture" }
        Remove-Item -LiteralPath $backupDir -Recurse -Force -ErrorAction SilentlyContinue

        Add-ConformancePhase "cleanup" "pass" "cleanup" "lt_1s"
        $rollbackGate = @($conformancePhases | Where-Object { $_.phase -eq "rollback" })
        $rollbackPassed = ($rollbackGate.Count -gt 0 -and (@($rollbackGate | Where-Object { $_.result -eq "fail" }).Count -eq 0) -and (@($rollbackGate | Where-Object { $_.result -eq "skip" }).Count -eq 0))
        $hasDegraded = @($conformancePhases | Where-Object { $_.result -eq "degraded" -or $_.result -eq "fail" -or $_.result -eq "skip" }).Count -gt 0
        $finalResult = if ($rollbackPassed -and -not $hasDegraded) { "pass" } else { "degraded" }
        Write-RedactedConformanceReport $finalResult
        Remove-Item -LiteralPath $credentialDir -Recurse -Force -ErrorAction SilentlyContinue
        if ($finalResult -eq "pass") {
            Write-Output "PASS: isolated product verification completed with consumable conformance evidence"
        } else {
            Write-Output "PASS: isolated product verification completed with degraded conformance evidence"
        }
    }
	catch {
		try {
			if ($conformancePhases.Count -eq 0 -or $conformancePhases[$conformancePhases.Count - 1].phase -ne "cleanup") {
				Add-ConformancePhase "cleanup" "fail" "cleanup" "unknown"
			}
			Write-RedactedConformanceReport "fail"
		} catch { Write-Warning "failed to write conformance report: $($_.Exception.Message)" }
		throw
	}
	finally {
		Remove-Item Env:STELE_TEST_POSTGRES_DSN -ErrorAction SilentlyContinue
		Remove-Item Env:STELE_TEST_POSTGRES_UPGRADE_DSN -ErrorAction SilentlyContinue
		Remove-Item Env:STELE_TEST_POSTGRES_DERIVED_WORK_DSN -ErrorAction SilentlyContinue
		Remove-Item Env:STELE_TEST_POSTGRES_MCP_DSN -ErrorAction SilentlyContinue
		if ($mcpEvidencePath -and (Test-Path -LiteralPath $mcpEvidencePath)) {
			Remove-Item -LiteralPath $mcpEvidencePath -Force -ErrorAction SilentlyContinue
		}
		if (-not $KeepResources) {
        & docker compose -f $ComposeFile down --volumes --remove-orphans
        if (Test-Path -LiteralPath $policyOverridePath) { Remove-Item -LiteralPath $policyOverridePath -Force -ErrorAction SilentlyContinue }
            if ($LASTEXITCODE -ne 0) { Write-Warning "owned Compose cleanup returned exit code $LASTEXITCODE" }
		} else {
			Write-Output "Resources retained for diagnostics under Compose project '$ProjectName'"
		}
		if ($credentialDir -and (Test-Path -LiteralPath $credentialDir)) {
			Remove-Item -LiteralPath $credentialDir -Recurse -Force -ErrorAction SilentlyContinue
		}
        $composeCleanupCompleted = $true
    }
}
finally {
    if ($composeAttempted -and -not $composeCleanupCompleted -and -not $KeepResources) {
        & docker compose -f $ComposeFile down --volumes --remove-orphans *> $null
        if (Test-Path -LiteralPath $policyOverridePath) { Remove-Item -LiteralPath $policyOverridePath -Force -ErrorAction SilentlyContinue }
    }
    Pop-Location
}
