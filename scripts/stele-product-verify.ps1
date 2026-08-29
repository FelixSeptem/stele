[CmdletBinding()]
param(
    [string]$ProjectName = "stele-verify-$([guid]::NewGuid().ToString('N').Substring(0, 12))",
    [string]$ComposeFile = "docker-compose.yml",
    [switch]$KeepResources
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
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
    $env:STELE_POSTGRES_PASSWORD = "verify-$([guid]::NewGuid().ToString('N'))"
    $env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY = "verify-bootstrap-$([guid]::NewGuid().ToString('N'))"
    $env:STELE_AUTH_DEFAULT_TENANT = "tenant-verify-$ProjectName"
    $env:STELE_AUTH_DEFAULT_PROJECT = "project-verify-$ProjectName"
    $env:STELE_AUTH_DEFAULT_NAMESPACE = "namespace-verify-$ProjectName"
    $env:STELE_POSTGRES_HOST_PORT = (Get-Random -Minimum 15432 -Maximum 25432).ToString()
    $env:STELE_HTTP_HOST_PORT = (Get-Random -Minimum 18080 -Maximum 28080).ToString()
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

    function Get-HttpStatus([string]$Path, [hashtable]$Headers) {
        try {
            $response = Invoke-WebRequest -Method Get -Uri ($baseUrl.TrimEnd('/') + $Path) -Headers $Headers -ErrorAction Stop
            return [int]$response.StatusCode
        } catch {
            try { return [int]$_.Exception.Response.StatusCode } catch { return 0 }
        }
    }

    Write-Output "Starting isolated product verification project '$ProjectName'"
    & docker compose -f $ComposeFile up --build -d
    if ($LASTEXITCODE -ne 0) { throw "Compose stack failed to start" }

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

        Write-Output "Verifying worker and scheduler bounded termination and durable background continuation..."
        Assert-BoundedStop "worker"
        $pendingKey = "product-verify-worker-$([guid]::NewGuid().ToString('N'))"
        $pendingHeaders = $runtimeHeaders.Clone(); $pendingHeaders["Idempotency-Key"] = $pendingKey
        $pendingBody = @{ event_type = "product.verify.worker-continuation"; content = "durable continuation fixture"; metadata = @{ fixture = "product-verify" } }
        Invoke-RestMethod -Method Post -Uri ($baseUrl.TrimEnd('/') + "/v1/events") -Headers $pendingHeaders -Body ($pendingBody | ConvertTo-Json -Depth 10) -ErrorAction Stop | Out-Null
        $pendingBefore = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/jobs/governance/status") -Headers $adminHeaders -ErrorAction Stop
        if ([int64]$pendingBefore.pending_raw_events -lt 1) { throw "worker-stop fixture was not durably pending" }
        Assert-RestartReady "worker"
        Invoke-Condition {
            try {
                $status = Invoke-RestMethod -Method Get -Uri ($baseUrl.TrimEnd('/') + "/v1/admin/jobs/governance/status") -Headers $adminHeaders -ErrorAction Stop
                return ([int64]$status.pending_raw_events -eq 0 -and [int64]$status.processed_raw_events -ge [int64]$pendingBefore.processed_raw_events + 1)
            } catch { return $false }
        } 45 "worker did not continue durable eligible work after restart"
        Assert-BoundedStop "scheduler"
        Assert-RestartReady "scheduler"

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

        Remove-Item -LiteralPath $credentialDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-Output "PASS: isolated product verification completed"
    }
	finally {
		Remove-Item Env:STELE_TEST_POSTGRES_DSN -ErrorAction SilentlyContinue
		Remove-Item Env:STELE_TEST_POSTGRES_UPGRADE_DSN -ErrorAction SilentlyContinue
		if (-not $KeepResources) {
            & docker compose -f $ComposeFile down --volumes --remove-orphans
            if ($LASTEXITCODE -ne 0) { Write-Warning "owned Compose cleanup returned exit code $LASTEXITCODE" }
		} else {
			Write-Output "Resources retained for diagnostics under Compose project '$ProjectName'"
		}
		if ($credentialDir -and (Test-Path -LiteralPath $credentialDir)) {
			Remove-Item -LiteralPath $credentialDir -Recurse -Force -ErrorAction SilentlyContinue
		}
    }
}
finally {
    Pop-Location
}
