[CmdletBinding()]
param(
    [string]$ProjectName = "stele-context-$([guid]::NewGuid().ToString('N').Substring(0, 12))",
    [string]$ComposeFile = "docker-compose.yml",
    [string]$Image = "stele-pc1-verify:local",
    [string]$ReportPath = "",
    [switch]$UseExistingStack,
    [string]$BaseUrl = "",
    [string]$CredentialDirectory = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
$ownedStarted = $false
$overridePath = Join-Path ([IO.Path]::GetTempPath()) "stele-context-$ProjectName.yml"
$ownedCredentials = $null
$verificationVariables = @("STELE_CONTEXT_VERIFY_URL", "STELE_CONTEXT_VERIFY_STAGE", "STELE_CONTEXT_VERIFY_REPORT", "STELE_CONTEXT_VERIFY_CREDENTIAL_DIR", "STELE_CONTEXT_VERIFY_IMAGE_DIGEST", "STELE_CONTEXT_VERIFY_SOURCE_REVISION", "STELE_CONTEXT_VERIFY_SOURCE_DIGEST")
try {
    if ($ProjectName -notmatch '^[a-z0-9][a-z0-9_-]{2,50}$') { throw "invalid owned context project identifier" }
    if ([string]::IsNullOrWhiteSpace($ReportPath)) { $ReportPath = Join-Path ([IO.Path]::GetTempPath()) "$ProjectName.context-conformance.json" }
    $ReportPath = [IO.Path]::GetFullPath($ReportPath)
    $reportParent = Split-Path -Parent $ReportPath
    if ($reportParent) { New-Item -ItemType Directory -Force -Path $reportParent | Out-Null }
    & docker info *> $null
    if ($LASTEXITCODE -ne 0) { throw "Docker dependency unavailable" }
    $env:COMPOSE_PROJECT_NAME = $ProjectName
    $composeArgs = @("compose", "-f", $ComposeFile)
    if ($UseExistingStack) {
        if (-not $BaseUrl -or -not $CredentialDirectory) { throw "existing owned stack requires URL and official credentials" }
        $container = (& docker @composeArgs ps -q api | Out-String).Trim()
        if (-not $container) { throw "owned API container missing" }
        $imageDigest = (& docker inspect --format '{{.Image}}' $container | Out-String).Trim()
    } else {
        # A new random project owns a new PostgreSQL/pgvector volume. Never reuse
        # another project's database or bootstrap credentials.
        $existing = (& docker @composeArgs ps -aq | Out-String).Trim()
        $existingVolumes = (& docker volume ls --filter "label=com.docker.compose.project=$ProjectName" -q | Out-String).Trim()
        if ($existing -or $existingVolumes) { throw "fresh context verification project already owns resources" }
        $imageDigest = (& docker image inspect --format '{{.Id}}' $Image | Out-String).Trim()
        if ($LASTEXITCODE -ne 0) { throw "repaired verification image unavailable" }
        $env:STELE_POSTGRES_IMAGE = if ($env:STELE_PRODUCT_VERIFY_POSTGRES_IMAGE) { $env:STELE_PRODUCT_VERIFY_POSTGRES_IMAGE } else { "docker.1ms.run/pgvector/pgvector:pg18" }
        $env:STELE_POSTGRES_PASSWORD = "context-$([guid]::NewGuid().ToString('N'))"
        $env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY = "context-bootstrap-$([guid]::NewGuid().ToString('N'))"
        $env:STELE_AUTH_DEFAULT_TENANT = "tenant-$ProjectName"
        $env:STELE_AUTH_DEFAULT_PROJECT = "project-$ProjectName"
        $env:STELE_AUTH_DEFAULT_NAMESPACE = "namespace-$ProjectName"
        $env:STELE_POSTGRES_HOST_PORT = (Get-Random -Minimum 15432 -Maximum 25432).ToString()
        $env:STELE_HTTP_HOST_PORT = (Get-Random -Minimum 18080 -Maximum 28080).ToString()
        $BaseUrl = "http://127.0.0.1:$($env:STELE_HTTP_HOST_PORT)"
        $ownedCredentials = Join-Path ([IO.Path]::GetTempPath()) "stele-context-credentials-$ProjectName"
        $CredentialDirectory = $ownedCredentials
    }
    if ($imageDigest -notmatch '^sha256:[a-f0-9]{64}$') { throw "pinned context image digest missing" }
    @("services:", "  api:", "    image: $imageDigest", "    environment:", "      STELE_PROVIDER_ENABLED: `"true`"", "      STELE_PROVIDER_SCHEMA_VERSIONS: `"provider-v1`"", "  worker:", "    image: $imageDigest", "  scheduler:", "    image: $imageDigest") | Set-Content -LiteralPath $overridePath -Encoding UTF8
    $composeArgs += @("-f", $overridePath)
    $ownedStarted = -not $UseExistingStack
    & docker @composeArgs up -d --no-build *> $null
    if ($LASTEXITCODE -ne 0) { throw "owned context stack failed to start" }
    function Wait-ContextReady {
        $deadline = [DateTime]::UtcNow.AddSeconds(45)
        do {
            try { if ((Invoke-WebRequest -Uri "$BaseUrl/readyz" -ErrorAction Stop).StatusCode -eq 200) { return } } catch { }
            Start-Sleep -Milliseconds 250
        } while ([DateTime]::UtcNow -lt $deadline)
        throw "owned context API readiness unavailable"
    }
    Wait-ContextReady
    $vector = (& docker @composeArgs exec -T postgres psql -U stele -d stele -At -c "SELECT extversion FROM pg_extension WHERE extname='vector'" | Out-String).Trim()
    if (-not $vector -or $LASTEXITCODE -ne 0) { throw "owned pgvector dependency missing" }
    if (-not $UseExistingStack) {
        & pwsh -NoProfile -File (Join-Path $PSScriptRoot "stele-bootstrap-smoke.ps1") -BaseUrl $BaseUrl -BootstrapKey $env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY -Tenant $env:STELE_AUTH_DEFAULT_TENANT -Project $env:STELE_AUTH_DEFAULT_PROJECT -Namespace $env:STELE_AUTH_DEFAULT_NAMESPACE -CredentialOutputDirectory $CredentialDirectory -SkipLifecycle
        if ($LASTEXITCODE -ne 0) { throw "official context bootstrap failed" }
    }
    $env:STELE_CONTEXT_VERIFY_URL = $BaseUrl
    $env:STELE_CONTEXT_VERIFY_REPORT = $ReportPath
    $env:STELE_CONTEXT_VERIFY_CREDENTIAL_DIR = $CredentialDirectory
    $env:STELE_CONTEXT_VERIFY_IMAGE_DIGEST = $imageDigest
    $env:STELE_CONTEXT_VERIFY_SOURCE_REVISION = (& git rev-parse HEAD | Out-String).Trim()
    $sourceParts = [Collections.Generic.List[string]]::new()
    $sourceFiles = @(& rg --files -g '*.go' -g '!**/*_test.go' -g 'go.mod' -g 'go.sum' | Sort-Object)
    foreach ($sourceFile in $sourceFiles) { $sourceParts.Add("$sourceFile|$((Get-FileHash -LiteralPath $sourceFile -Algorithm SHA256).Hash)") }
    $env:STELE_CONTEXT_VERIFY_SOURCE_DIGEST = ([Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes(($sourceParts -join "`n"))))).ToLowerInvariant()
    foreach ($stage in @("initial", "restart")) {
        if ($stage -eq "restart") {
            & docker @composeArgs restart api worker *> $null
            if ($LASTEXITCODE -ne 0) { throw "owned context API/worker restart failed" }
            Wait-ContextReady
        }
        $env:STELE_CONTEXT_VERIFY_STAGE = $stage
        & go test ./internal/assurance -run '^TestProviderContextFreshPostgresLive$' -count=1 -timeout 3m
        if ($LASTEXITCODE -ne 0) { throw "public context $stage verification failed" }
        if (-not (Test-Path -LiteralPath $ReportPath)) { throw "context evidence report missing" }
        $report = Get-Content -Raw -LiteralPath $ReportPath | ConvertFrom-Json
        if ($report.result -ne "passed" -or $report.stage -ne $stage) { throw "context evidence is not passing" }
    }
    Write-Output "PASS: public context schema, governed paths, isolation, lifecycle, replay, and API/worker restart; bounded report: $ReportPath"
} catch {
    # Do not retain exception text, credentials, raw HTTP bodies, or scope values.
    if (-not $ReportPath) { $ReportPath = Join-Path ([IO.Path]::GetTempPath()) "$ProjectName.context-conformance.json" }
    @{ result = "failed"; category = "context_verification_incomplete"; consumable = $false } | ConvertTo-Json | Set-Content -LiteralPath $ReportPath -Encoding UTF8
    throw "Provider context verification failed; bounded evidence: $ReportPath"
} finally {
    if ($ownedStarted) { & docker @composeArgs down --volumes --remove-orphans *> $null }
    if (Test-Path -LiteralPath $overridePath) { Remove-Item -LiteralPath $overridePath -Force }
    if ($ownedCredentials -and (Test-Path -LiteralPath $ownedCredentials)) {
        $resolvedCredentials = [IO.Path]::GetFullPath($ownedCredentials)
        $expectedCredentials = [IO.Path]::GetFullPath((Join-Path ([IO.Path]::GetTempPath()) "stele-context-credentials-$ProjectName"))
        if ($resolvedCredentials -ne $expectedCredentials) { throw "owned credential cleanup path mismatch" }
        Remove-Item -LiteralPath $resolvedCredentials -Recurse -Force
    }
    foreach ($variable in $verificationVariables) { Remove-Item -LiteralPath "Env:$variable" -ErrorAction SilentlyContinue }
    Pop-Location
}
