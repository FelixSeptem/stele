[CmdletBinding()]
param(
    [string]$ProjectName = "stele-first-ten-$([guid]::NewGuid().ToString('N').Substring(0, 12))",
    [string]$ComposeFile = "docker-compose.yml",
    [ValidateRange(30, 900)][int]$TimeoutSeconds = 600,
    [switch]$KeepResources
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot

$composePath = $null
$baseUrl = $null
$deadline = $null
$stackOwned = $false
$credentialDir = $null
$currentPhase = "preflight"
$failedPhase = $null
$exitCode = 0
$environmentNames = @(
    "COMPOSE_PROJECT_NAME",
    "STELE_POSTGRES_PASSWORD",
    "STELE_AUTH_BOOTSTRAP_ADMIN_KEY",
    "STELE_AUTH_DEFAULT_TENANT",
    "STELE_AUTH_DEFAULT_PROJECT",
    "STELE_AUTH_DEFAULT_NAMESPACE",
    "STELE_POSTGRES_HOST_PORT",
    "STELE_HTTP_HOST_PORT"
)
$environmentSnapshot = @{}

# Redaction contract: never print a DSN, API key, scope value, record ID,
# request body, provider payload, or raw process errors.

function Write-Phase([string]$Name, [string]$Result, [string]$Category) {
    Write-Output ("phase={0} result={1} category={2}" -f $Name, $Result, $Category)
}

function Get-Category([System.Exception]$Exception, [string]$Phase) {
    if ($Exception -is [System.TimeoutException] -or $Exception.Message -eq "timeout") { return "timeout" }
    switch ($Phase) {
        "preflight" { return "missing_prerequisite" }
        "start" { return "start_failed" }
        "discovery" { return "discovery_failed" }
        "readiness" { return "not_ready" }
        "cleanup" { return "cleanup_failed" }
        default { return "assertion_failed" }
    }
}

function Assert-ProjectName([string]$Value) {
    if ([string]::IsNullOrWhiteSpace($Value) -or $Value -notmatch '^[a-z0-9][a-z0-9_-]{2,50}$') {
        throw "invalid project name"
    }
}

function Assert-Budget {
    if ($null -eq $deadline) { return }
    if ([DateTime]::UtcNow -ge $deadline) { throw [System.TimeoutException]::new("timeout") }
}

function Get-RemainingSeconds {
    Assert-Budget
    $remaining = [math]::Floor(($deadline - [DateTime]::UtcNow).TotalSeconds)
    if ($remaining -lt 1) { throw [System.TimeoutException]::new("timeout") }
    return [int]$remaining
}

function Invoke-Phase([string]$Name, [scriptblock]$Action) {
    $script:currentPhase = $Name
    try {
        if ($Name -ne "preflight") { Assert-Budget }
        & $Action
        if ($Name -eq "preflight" -and $script:exitCode -eq 2) { return }
        Write-Phase $Name "pass" "ok"
    } catch {
        $script:failedPhase = $Name
        throw
    }
}

function Invoke-Compose([string[]]$Arguments) {
    # Cleanup is always scoped to the validated project: docker compose down --volumes --remove-orphans.
    & docker compose -f $composePath -p $ProjectName @Arguments *> $null
    if ($LASTEXITCODE -ne 0) { throw "compose command failed" }
}

function Invoke-HttpStatus([string]$Path) {
    try {
        $response = Invoke-WebRequest -Method Get -Uri ($baseUrl.TrimEnd('/') + $Path) -ErrorAction Stop
        return [int]$response.StatusCode
    } catch {
        try { return [int]$_.Exception.Response.StatusCode } catch { return 0 }
    }
}

function Wait-HttpStatus([string]$Path, [int]$ExpectedStatus) {
    do {
        Assert-Budget
        if ((Invoke-HttpStatus $Path) -eq $ExpectedStatus) { return }
        Start-Sleep -Milliseconds 250
    } while ([DateTime]::UtcNow -lt $deadline)
    throw [System.TimeoutException]::new("timeout")
}

function Invoke-JsonRequest([string]$Method, [string]$Path, [hashtable]$Headers, [object]$Body) {
    Assert-Budget
    $params = @{
        Method = $Method
        Uri = ($baseUrl.TrimEnd('/') + $Path)
        Headers = $Headers
        ErrorAction = "Stop"
    }
    if ($null -ne $Body) { $params.Body = ($Body | ConvertTo-Json -Depth 10) }
    return Invoke-RestMethod @params
}

function Save-Environment {
    foreach ($name in $environmentNames) {
        $item = Get-Item "Env:$name" -ErrorAction SilentlyContinue
        if ($null -eq $item) { $environmentSnapshot[$name] = $null } else { $environmentSnapshot[$name] = $item.Value }
    }
}

function Restore-Environment {
    foreach ($name in $environmentNames) {
        $value = $environmentSnapshot[$name]
        if ($null -eq $value) {
            Remove-Item "Env:$name" -ErrorAction SilentlyContinue
        } else {
            Set-Item "Env:$name" $value
        }
    }
}

function Set-HarnessEnvironment {
    $suffix = [guid]::NewGuid().ToString("N").Substring(0, 12)
    $env:COMPOSE_PROJECT_NAME = $ProjectName
    $env:STELE_POSTGRES_PASSWORD = "verify-$suffix"
    $env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY = "first-ten-bootstrap-$suffix"
    $env:STELE_AUTH_DEFAULT_TENANT = "tenant-first-$suffix"
    $env:STELE_AUTH_DEFAULT_PROJECT = "project-first-$suffix"
    $env:STELE_AUTH_DEFAULT_NAMESPACE = "namespace-first-$suffix"
    $env:STELE_POSTGRES_HOST_PORT = (Get-Random -Minimum 15432 -Maximum 25432).ToString()
    $env:STELE_HTTP_HOST_PORT = (Get-Random -Minimum 18080 -Maximum 28080).ToString()
    $script:baseUrl = "http://localhost:$($env:STELE_HTTP_HOST_PORT)"
}

try {
    if ($TimeoutSeconds -lt 30 -or $TimeoutSeconds -gt 900) {
        Write-Phase "preflight" "fail" "invalid_timeout"
        $exitCode = 1
    } else {
        try { Assert-ProjectName $ProjectName } catch {
            Write-Phase "preflight" "fail" "invalid_project"
            $exitCode = 1
        }
    }

    if ($exitCode -eq 0) {
        Invoke-Phase "preflight" {
            $script:composePath = [IO.Path]::GetFullPath((Join-Path $repoRoot $ComposeFile))
            if (-not (Test-Path -LiteralPath $composePath -PathType Leaf)) { throw "compose file missing" }
            if ($null -eq (Get-Command docker -ErrorAction SilentlyContinue)) {
                if ($env:STELE_FIRST_TEN_MINUTES_CI -eq "1") { throw "docker is required in CI" }
                Write-Phase "preflight" "skip" "missing_prerequisite"
                $script:exitCode = 2
                return
            }
            if ($null -eq (Get-Command pwsh -ErrorAction SilentlyContinue)) {
                if ($env:STELE_FIRST_TEN_MINUTES_CI -eq "1") { throw "pwsh is required in CI" }
                Write-Phase "preflight" "skip" "missing_prerequisite"
                $script:exitCode = 2
                return
            }
            & docker info *> $null
            if ($LASTEXITCODE -ne 0) {
                if ($env:STELE_FIRST_TEN_MINUTES_CI -eq "1") { throw "docker daemon unavailable in CI" }
                Write-Phase "preflight" "skip" "missing_prerequisite"
                $script:exitCode = 2
            }
        }
    }

    if ($exitCode -eq 0) {
        Save-Environment
        Set-HarnessEnvironment
        $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)

        Invoke-Phase "start" {
            $script:stackOwned = $true
            Invoke-Compose @("up", "--build", "-d")
        }

        Invoke-Phase "discovery" {
            Wait-HttpStatus "/health" 200
            Wait-HttpStatus "/version" 200
            Wait-HttpStatus "/openapi.yaml" 200
        }

        Invoke-Phase "readiness" {
            Wait-HttpStatus "/readyz" 200
        }

        Invoke-Phase "bootstrap" {
            $script:credentialDir = Join-Path ([IO.Path]::GetTempPath()) "stele-first-ten-$ProjectName"
            New-Item -ItemType Directory -Force -Path $credentialDir | Out-Null
            $bootstrapScript = Join-Path $PSScriptRoot "stele-bootstrap-smoke.ps1"
            & pwsh -NoProfile -File $bootstrapScript `
                -BaseUrl $baseUrl `
                -BootstrapKey $env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY `
                -Tenant $env:STELE_AUTH_DEFAULT_TENANT `
                -Project $env:STELE_AUTH_DEFAULT_PROJECT `
                -Namespace $env:STELE_AUTH_DEFAULT_NAMESPACE `
                -CredentialOutputDirectory $credentialDir *> $null
            if ($LASTEXITCODE -ne 0) { throw "bootstrap smoke failed" }
        }

        Invoke-Phase "lifecycle" {
            $adminCredential = (Get-Content -Raw -LiteralPath (Join-Path $credentialDir "admin.credential")).Trim()
            $runtimeCredential = (Get-Content -Raw -LiteralPath (Join-Path $credentialDir "runtime.credential")).Trim()
            if ([string]::IsNullOrWhiteSpace($adminCredential) -or [string]::IsNullOrWhiteSpace($runtimeCredential)) { throw "bootstrap credentials missing" }
            $adminHeaders = @{
                "X-API-Key" = $adminCredential
                "X-Stele-Tenant" = $env:STELE_AUTH_DEFAULT_TENANT
                "X-Stele-Project" = $env:STELE_AUTH_DEFAULT_PROJECT
                "X-Stele-Namespace" = $env:STELE_AUTH_DEFAULT_NAMESPACE
                "Content-Type" = "application/json"
            }
            $runtimeHeaders = $adminHeaders.Clone()
            $runtimeHeaders["X-API-Key"] = $runtimeCredential
            $before = Invoke-JsonRequest GET "/v1/admin/jobs/governance/status" $adminHeaders $null
            $idempotencyKey = "first-ten-$([guid]::NewGuid().ToString('N'))"
            $eventHeaders = $runtimeHeaders.Clone()
            $eventHeaders["Idempotency-Key"] = $idempotencyKey
            $event = Invoke-JsonRequest POST "/v1/events" $eventHeaders @{ event_type = "self-hosting.first-ten-minutes"; content = "first-ten-minutes lifecycle fixture"; metadata = @{ fixture = "first-ten-minutes" } }
            if ([string]::IsNullOrWhiteSpace([string]$event.event_id)) { throw "event was not accepted" }
            do {
                Assert-Budget
                $status = Invoke-JsonRequest GET "/v1/admin/jobs/governance/status" $adminHeaders $null
                if ([int64]$status.pending_raw_events -eq 0 -and [int64]$status.processed_raw_events -ge ([int64]$before.processed_raw_events + 1)) { break }
                Start-Sleep -Milliseconds 500
            } while ([DateTime]::UtcNow -lt $deadline)
            if ([int64]$status.pending_raw_events -ne 0) { throw [System.TimeoutException]::new("timeout") }
            Invoke-JsonRequest POST "/v1/memories/search" $runtimeHeaders @{ query = "first-ten-minutes lifecycle fixture"; top_k = 5 } | Out-Null
            Invoke-JsonRequest POST "/v1/context/assemble" $runtimeHeaders @{ query = "first-ten-minutes lifecycle fixture"; budget = 1200; include_diagnostics = $true } | Out-Null
        }

        Invoke-Phase "telemetry" {
            $metricsResponse = Invoke-WebRequest -Method Get -Uri ($baseUrl.TrimEnd('/') + "/metrics") -ErrorAction Stop
            if ([int]$metricsResponse.StatusCode -ne 200 -or [string]::IsNullOrWhiteSpace($metricsResponse.Content)) { throw "metrics unavailable" }
        }
    }
} catch {
    if ($exitCode -eq 0) { $exitCode = 1 }
    if ($null -eq $failedPhase) { $failedPhase = $currentPhase }
    Write-Phase $failedPhase "fail" (Get-Category $_.Exception $failedPhase)
} finally {
    if ($stackOwned -and -not $KeepResources) {
        try {
            $currentPhase = "cleanup"
            Invoke-Compose @("down", "--volumes", "--remove-orphans")
            Write-Phase "cleanup" "pass" "ok"
        } catch {
            Write-Phase "cleanup" "fail" "cleanup_failed"
            $exitCode = 1
        }
    } elseif ($stackOwned -and $KeepResources) {
        Write-Output ("resources=retained project={0}" -f $ProjectName)
    }
    if ($credentialDir -and (Test-Path -LiteralPath $credentialDir)) {
        Remove-Item -LiteralPath $credentialDir -Recurse -Force -ErrorAction SilentlyContinue
    }
    if ($environmentSnapshot.Count -gt 0) { Restore-Environment }
    Pop-Location
}

if ($exitCode -eq 0) {
    Write-Output "PASS: first-ten-minutes smoke completed"
} elseif ($exitCode -eq 2) {
    Write-Output "SKIP: first-ten-minutes smoke did not run"
} else {
    Write-Output "FAIL: first-ten-minutes smoke did not complete"
}
exit $exitCode
