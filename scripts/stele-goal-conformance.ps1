[CmdletBinding()]
param(
    [string]$TestDSN = $env:STELE_TEST_POSTGRES_GOAL_DSN,
    [int]$TimeoutSeconds = 180
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    if ([string]::IsNullOrWhiteSpace($TestDSN)) {
        Write-Output 'SKIP: STELE_TEST_POSTGRES_GOAL_DSN is not configured; goal conformance did not run and readiness is not claimed'
        exit 2
    }
    if ($TimeoutSeconds -lt 30 -or $TimeoutSeconds -gt 900) {
        throw 'TimeoutSeconds must be between 30 and 900'
    }
    try {
        $uri = [System.Uri]$TestDSN
        if ($uri.Scheme -notin @('postgres', 'postgresql') -or [string]::IsNullOrWhiteSpace($uri.Host)) {
            throw 'invalid PostgreSQL DSN'
        }
    } catch {
        throw 'TestDSN must be an explicit PostgreSQL DSN with a host'
    }
    $env:STELE_TEST_POSTGRES_GOAL_DSN = $TestDSN
    Write-Output 'Running governed goal PostgreSQL + pgvector conformance matrix...'
    & go test ./internal/storage/postgres -run '^TestGovernedGoalInsightPostgresPgvectorConformanceMatrix$' -count=1 -timeout ("{0}s" -f $TimeoutSeconds) -v
    if ($LASTEXITCODE -ne 0) {
        throw 'governed goal conformance failed'
    }
    Write-Output 'PASS: governed goal conformance completed with bounded review-only evidence'
}
finally {
    Remove-Item Env:STELE_TEST_POSTGRES_GOAL_DSN -ErrorAction SilentlyContinue
    Pop-Location
}
