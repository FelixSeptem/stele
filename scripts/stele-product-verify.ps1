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

    Write-Output "Starting isolated product verification project '$ProjectName'"
    & docker compose -f $ComposeFile up --build -d
    if ($LASTEXITCODE -ne 0) { throw "Compose stack failed to start" }

    try {
        $credentialDir = Join-Path ([System.IO.Path]::GetTempPath()) "stele-product-verify-$ProjectName"
        New-Item -ItemType Directory -Force -Path $credentialDir | Out-Null
        & pwsh -NoProfile -File (Join-Path $PSScriptRoot "stele-bootstrap-smoke.ps1") `
            -BaseUrl "http://localhost:8080" `
            -BootstrapKey $env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY `
            -Tenant $env:STELE_AUTH_DEFAULT_TENANT `
            -Project $env:STELE_AUTH_DEFAULT_PROJECT `
            -Namespace $env:STELE_AUTH_DEFAULT_NAMESPACE `
            -CredentialOutputDirectory $credentialDir
        if ($LASTEXITCODE -ne 0) { throw "bootstrap/lifecycle smoke failed" }
        Write-Output "PASS: isolated product verification completed"
    }
    finally {
        if (-not $KeepResources) {
            & docker compose -f $ComposeFile down --volumes --remove-orphans
            if ($LASTEXITCODE -ne 0) { Write-Warning "owned Compose cleanup returned exit code $LASTEXITCODE" }
        } else {
            Write-Output "Resources retained for diagnostics under Compose project '$ProjectName'"
        }
    }
}
finally {
    Pop-Location
}
