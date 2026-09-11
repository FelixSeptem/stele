[CmdletBinding()]
param(
    [string]$TestDSN = $env:STELE_TEST_RETRIEVAL_EVALUATION_DSN
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($TestDSN)) {
    Write-Output 'SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED'
    exit 2
}

# Real-stack evaluation must be explicitly opted into with the harness-owned
# variable. Never inherit an operator/runtime DSN, even when it is present.
if (-not [string]::IsNullOrWhiteSpace($env:STELE_POSTGRES_DSN) -and $TestDSN -eq $env:STELE_POSTGRES_DSN) {
    Write-Error 'STELE_TEST_RETRIEVAL_EVALUATION_DSN must not reuse STELE_POSTGRES_DSN'
    exit 1
}
try {
    $uri = [System.Uri]$TestDSN
    if ($uri.Scheme -notin @('postgres','postgresql') -or [string]::IsNullOrWhiteSpace($uri.Host)) {
        throw 'DSN must be a PostgreSQL URL with an explicit host'
    }
} catch {
    Write-Error 'STELE_TEST_RETRIEVAL_EVALUATION_DSN must be an explicit PostgreSQL DSN'
    exit 1
}

$env:STELE_TEST_RETRIEVAL_EVALUATION_DSN = $TestDSN
go test ./internal/storage/postgres -run '^TestEvaluationFixtureSeederSeedsOwnedPostgresFixture$' -count=1
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
