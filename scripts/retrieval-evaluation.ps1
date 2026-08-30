[CmdletBinding()]
param(
    [string]$TestDSN = $env:STELE_TEST_RETRIEVAL_EVALUATION_DSN
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($TestDSN)) {
    Write-Output 'SKIP_RETRIEVAL_EVALUATION_DSN_REQUIRED'
    exit 2
}

$env:STELE_TEST_RETRIEVAL_EVALUATION_DSN = $TestDSN
go test ./internal/storage/postgres -run '^TestEvaluationFixtureSeederSeedsOwnedPostgresFixture$' -count=1
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
