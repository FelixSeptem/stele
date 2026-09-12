[CmdletBinding()]
param(
    [string]$TestDSN = $env:STELE_TEST_RETRIEVAL_EVALUATION_DSN,
    [string]$ReportDirectory = $env:STELE_RETRIEVAL_EVALUATION_REPORT_DIR
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
if ([string]::IsNullOrWhiteSpace($ReportDirectory)) {
    $ReportDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ("stele-retrieval-evaluation-" + [guid]::NewGuid().ToString('N'))
}
$ReportDirectory = [System.IO.Path]::GetFullPath($ReportDirectory)
$env:STELE_RETRIEVAL_EVALUATION_REPORT_DIR = $ReportDirectory
go test ./internal/storage/postgres -run '^TestEvaluationFixtureRunsOwnedPostgresEvaluation$' -count=1 -v
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
if ((Test-Path -LiteralPath (Join-Path $ReportDirectory 'baseline.json')) -and
    (Test-Path -LiteralPath (Join-Path $ReportDirectory 'candidate.json')) -and
    (Test-Path -LiteralPath (Join-Path $ReportDirectory 'gate.json'))) {
    Write-Output "RETRIEVAL_EVALUATION_REPORT_DIR=$ReportDirectory"
} else {
    Write-Error 'retrieval evaluation completed without retaining required redacted reports'
    exit 1
}
