Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    $env:GOTELEMETRY = "off"
    $env:GOCACHE = Join-Path $repoRoot ".gocache"
    go test ./docs -run 'SelfHostingSmokeLoop' -count=1
    if ($LASTEXITCODE -ne 0) {
        throw "self-hosting smoke docs check failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}
