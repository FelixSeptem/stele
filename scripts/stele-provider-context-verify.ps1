[CmdletBinding()]
param(
    [string]$ProjectName = "stele-context-$([guid]::NewGuid().ToString('N').Substring(0, 12))",
    [string]$ComposeFile = "docker-compose.yml",
    [string]$Image = "stele-pc6-verify:local",
    [string]$ReportPath = "",
    [string]$ReferenceReportPath = "",
    [string]$ProjectionReportPath = "",
    [switch]$UseExistingStack,
    [string]$BaseUrl = "",
    [string]$CredentialDirectory = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
$ownedStarted = $false
$overridePath = $null
$ownedOverride = $false
$overrideID = $null
$ownedCredentials = $null
$phase = "preflight"
$verificationVariables = @("STELE_CONTEXT_VERIFY_URL", "STELE_CONTEXT_VERIFY_STAGE", "STELE_CONTEXT_VERIFY_REPORT", "STELE_REFERENCE_VERIFY_REPORT", "STELE_PROJECTION_VERIFY_REPORT", "STELE_PROJECTION_VERIFY_PHASE", "STELE_CONTEXT_VERIFY_CREDENTIAL_DIR", "STELE_CONTEXT_VERIFY_IMAGE_DIGEST", "STELE_CONTEXT_VERIFY_SOURCE_REVISION", "STELE_CONTEXT_VERIFY_SOURCE_DIGEST", "STELE_CONTEXT_VERIFY_POSTGRES_DSN")
$changedVariables = $verificationVariables + @("COMPOSE_PROJECT_NAME", "STELE_POSTGRES_IMAGE", "STELE_POSTGRES_PASSWORD", "STELE_AUTH_BOOTSTRAP_ADMIN_KEY", "STELE_AUTH_DEFAULT_TENANT", "STELE_AUTH_DEFAULT_PROJECT", "STELE_AUTH_DEFAULT_NAMESPACE", "STELE_POSTGRES_HOST_PORT", "STELE_HTTP_HOST_PORT")
$previousVariables = @{}
foreach ($variable in $changedVariables) { $previousVariables[$variable] = [Environment]::GetEnvironmentVariable($variable, "Process") }
try {
    if ($ProjectName -notmatch '^[a-z0-9][a-z0-9_-]{2,50}$') { throw "invalid owned context project identifier" }
    if ([string]::IsNullOrWhiteSpace($ReportPath)) { $ReportPath = Join-Path ([IO.Path]::GetTempPath()) "$ProjectName.context-conformance.json" }
    if ([string]::IsNullOrWhiteSpace($ReferenceReportPath)) { $ReferenceReportPath = "$ReportPath.reference.json" }
    if ([string]::IsNullOrWhiteSpace($ProjectionReportPath)) { $ProjectionReportPath = "$ReportPath.projection.json" }
    $ReportPath = [IO.Path]::GetFullPath($ReportPath)
    $ReferenceReportPath = [IO.Path]::GetFullPath($ReferenceReportPath)
    $ProjectionReportPath = [IO.Path]::GetFullPath($ProjectionReportPath)
    $reportParent = Split-Path -Parent $ReportPath
    if ($reportParent) { New-Item -ItemType Directory -Force -Path $reportParent | Out-Null }
    $referenceReportParent = Split-Path -Parent $ReferenceReportPath
    if ($referenceReportParent) { New-Item -ItemType Directory -Force -Path $referenceReportParent | Out-Null }
    $projectionReportParent = Split-Path -Parent $ProjectionReportPath
    if ($projectionReportParent) { New-Item -ItemType Directory -Force -Path $projectionReportParent | Out-Null }
    $phase = "docker_dependency"
    & docker info *> $null
    if ($LASTEXITCODE -ne 0) { throw "Docker dependency unavailable" }
    $env:COMPOSE_PROJECT_NAME = $ProjectName
    $composeArgs = @("compose", "-f", $ComposeFile)
    if ($UseExistingStack) {
        if (-not $BaseUrl -or -not $CredentialDirectory) { throw "existing owned stack requires URL and official credentials" }
        if (-not $env:STELE_CONTEXT_VERIFY_POSTGRES_DSN) { throw "existing stack requires explicit observation DSN" }
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
    $overrideID = [guid]::NewGuid().ToString('N')
    $overridePath = [IO.Path]::GetFullPath((Join-Path ([IO.Path]::GetTempPath()) "stele-context-$ProjectName-$overrideID.yml"))
    @("services:", "  api:", "    image: $imageDigest", "    environment:", "      STELE_PROVIDER_ENABLED: `"true`"", "      STELE_PROVIDER_SCHEMA_VERSIONS: `"provider-v1`"", "  worker:", "    image: $imageDigest", "    environment:", "      STELE_PROVIDER_ENABLED: `"true`"", "  scheduler:", "    image: $imageDigest", "    environment:", "      STELE_PROVIDER_ENABLED: `"true`"") | Set-Content -LiteralPath $overridePath -Encoding UTF8
    $ownedOverride = $true
    $composeArgs += @("-f", $overridePath)
    $ownedStarted = -not $UseExistingStack
    $phase = "stack_start"
    # Compose startup output contains only orchestrator status; retaining it
    # makes a missing image/config diagnosable without exposing HTTP payloads,
    # credentials, continuity references, or database contents.
    & docker @composeArgs up -d --no-build
    if ($LASTEXITCODE -ne 0) { throw "owned context stack failed to start" }
    function Wait-ContextReady {
        $deadline = [DateTime]::UtcNow.AddSeconds(45)
        do {
            try { if ((Invoke-WebRequest -Uri "$BaseUrl/readyz" -ErrorAction Stop).StatusCode -eq 200) { return } } catch { }
            Start-Sleep -Milliseconds 250
        } while ([DateTime]::UtcNow -lt $deadline)
        throw "owned context API readiness unavailable"
    }
    $phase = "api_readiness"
    Wait-ContextReady
    $phase = "pgvector_dependency"
    $vector = (& docker @composeArgs exec -T postgres psql -U stele -d stele -At -c "SELECT extversion FROM pg_extension WHERE extname='vector'" | Out-String).Trim()
    if (-not $vector -or $LASTEXITCODE -ne 0) { throw "owned pgvector dependency missing" }
    if (-not $UseExistingStack) {
        $phase = "official_bootstrap"
        & pwsh -NoProfile -File (Join-Path $PSScriptRoot "stele-bootstrap-smoke.ps1") -BaseUrl $BaseUrl -BootstrapKey $env:STELE_AUTH_BOOTSTRAP_ADMIN_KEY -Tenant $env:STELE_AUTH_DEFAULT_TENANT -Project $env:STELE_AUTH_DEFAULT_PROJECT -Namespace $env:STELE_AUTH_DEFAULT_NAMESPACE -CredentialOutputDirectory $CredentialDirectory -DisclosureProfile memory-content-v1 -SkipLifecycle
        if ($LASTEXITCODE -ne 0) { throw "official context bootstrap failed" }
    }
    $env:STELE_CONTEXT_VERIFY_URL = $BaseUrl
    if (-not $UseExistingStack) { $env:STELE_CONTEXT_VERIFY_POSTGRES_DSN = "postgres://stele:$($env:STELE_POSTGRES_PASSWORD)@127.0.0.1:$($env:STELE_POSTGRES_HOST_PORT)/stele?sslmode=disable" }
    $env:STELE_CONTEXT_VERIFY_REPORT = $ReportPath
    $env:STELE_REFERENCE_VERIFY_REPORT = [IO.Path]::GetFullPath($ReferenceReportPath)
    $env:STELE_PROJECTION_VERIFY_REPORT = $ProjectionReportPath
    $env:STELE_CONTEXT_VERIFY_CREDENTIAL_DIR = $CredentialDirectory
    $env:STELE_CONTEXT_VERIFY_IMAGE_DIGEST = $imageDigest
    $env:STELE_CONTEXT_VERIFY_SOURCE_REVISION = (& git rev-parse HEAD | Out-String).Trim()
    $sourceParts = [Collections.Generic.List[string]]::new()
    $sourceFiles = @(& rg --files -g '*.go' -g '!**/*_test.go' -g 'go.mod' -g 'go.sum' -g 'internal/storage/postgres/migrations/*.sql' -g 'Dockerfile' -g 'docker-compose.yml' | Sort-Object)
    foreach ($sourceFile in $sourceFiles) { $sourceParts.Add("$sourceFile|$((Get-FileHash -LiteralPath $sourceFile -Algorithm SHA256).Hash)") }
    $env:STELE_CONTEXT_VERIFY_SOURCE_DIGEST = ([Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes(($sourceParts -join "`n"))))).ToLowerInvariant()
    foreach ($stage in @("initial", "restart")) {
        if ($stage -eq "restart") {
            & docker @composeArgs restart api worker *> $null
            if ($LASTEXITCODE -ne 0) { throw "owned context API/worker restart failed" }
            Wait-ContextReady
        }
        $env:STELE_CONTEXT_VERIFY_STAGE = $stage
        $phase = "public_$stage"
        & go test ./internal/assurance -run '^TestProviderContextFreshPostgresLive$' -count=1 -timeout 3m
        if ($LASTEXITCODE -ne 0) { throw "public context $stage verification failed" }
        if (-not (Test-Path -LiteralPath $ReportPath)) { throw "context evidence report missing" }
        $report = Get-Content -Raw -LiteralPath $ReportPath | ConvertFrom-Json
        if ($report.result -ne "passed" -or $report.stage -ne $stage -or $report.context_digest_version -ne "context-digest-v1" -or $report.context_continuity_version -ne "context-continuity-v1" -or -not $report.continuity_checked) { throw "context evidence is not PC4-ready" }
        Copy-Item -LiteralPath $ReportPath -Destination "$ReportPath.$stage.json"
        & go test ./internal/assurance -run '^TestProviderReferenceFreshPostgresLive$' -count=1 -timeout 3m
        if ($LASTEXITCODE -ne 0) { throw "public durable reference $stage verification failed" }
        if (-not (Test-Path -LiteralPath $ReferenceReportPath)) { throw "durable reference evidence report missing" }
        $referenceReport = Get-Content -Raw -LiteralPath $ReferenceReportPath | ConvertFrom-Json
        if ($referenceReport.schema_version -ne "provider-v1" -or $referenceReport.fixture_version -ne "provider-durable-reference-live-v1" -or $referenceReport.migration_version -lt 1 -or -not $referenceReport.migration_integrity_verified -or $referenceReport.migration_manifest_digest -notmatch '^[a-f0-9]{64}$') { throw "durable reference schema, migration or fixture provenance missing" }
        if ($referenceReport.result -ne "passed" -or $referenceReport.contract_version -ne "durable-reference-v1" -or $referenceReport.stage -ne $stage -or $referenceReport.governance_completed -ne 4 -or -not $referenceReport.durable_lineage_checked -or ($stage -eq "restart" -and (-not $referenceReport.restart_checked -or -not $referenceReport.lifecycle_checked -or -not $referenceReport.selector_replacement_checked -or -not $referenceReport.history_retention_checked))) { throw "durable reference evidence is not PC5-ready" }
        Copy-Item -LiteralPath $ReferenceReportPath -Destination "$ReferenceReportPath.$stage.json"
        if ($stage -eq "initial") {
            # Freeze actual async consumers only in this explicitly owned stack.
            # Admission is public HTTP; no governance/feedback state is fabricated.
            & docker @composeArgs stop worker scheduler *> $null
            if ($LASTEXITCODE -ne 0) { throw "owned projection consumers failed to stop" }
            $phase = "projection_pending_admission"
            $env:STELE_PROJECTION_VERIFY_PHASE = "pending"
            & go test ./internal/assurance -run '^TestProviderProjectionFreshPostgresLive$' -count=1 -timeout 3m
            if ($LASTEXITCODE -ne 0) { throw "public pending projection admission failed" }
            $pendingReport = Get-Content -Raw -LiteralPath $ProjectionReportPath | ConvertFrom-Json
            if ($pendingReport.result -ne "pending_checked" -or -not $pendingReport.pending_checked -or -not $pendingReport.feedback_nonactivation_checked) { throw "pending projection evidence incomplete" }
            Copy-Item -LiteralPath $ProjectionReportPath -Destination "$ProjectionReportPath.pending.json"
            # Restart during pending evidence, then prove ordinary jobs recover.
            & docker @composeArgs restart api *> $null
            if ($LASTEXITCODE -ne 0) { throw "pending projection API restart failed" }
            & docker @composeArgs start worker scheduler *> $null
            if ($LASTEXITCODE -ne 0) { throw "owned projection consumers failed to resume" }
            Wait-ContextReady
        }
        $phase = "projection_$stage"
        $env:STELE_PROJECTION_VERIFY_PHASE = "settled"
        & go test ./internal/assurance -run '^TestProviderProjectionFreshPostgresLive$' -count=1 -timeout 4m
        if ($LASTEXITCODE -ne 0) { throw "public projection $stage verification failed" }
        $projectionReport = Get-Content -Raw -LiteralPath $ProjectionReportPath | ConvertFrom-Json
        if ($projectionReport.result -ne "passed" -or $projectionReport.stage -ne $stage -or $projectionReport.schema_version -ne "provider-v1" -or $projectionReport.contract_version -ne "turn-outcome-feedback-projection-v1" -or $projectionReport.fixture_version -ne "provider-turn-outcome-feedback-live-v1" -or -not $projectionReport.migration_integrity_verified -or $projectionReport.migration_version -lt 1 -or $projectionReport.migration_manifest_digest -notmatch '^[a-f0-9]{64}$' -or $projectionReport.feedback_identity_hash -notmatch '^[a-f0-9]{64}$' -or -not $projectionReport.pending_to_completed_checked -or -not $projectionReport.correction_history_checked -or -not $projectionReport.exact_scope_checked -or -not $projectionReport.context_receipt_checked -or -not $projectionReport.feedback_nonactivation_checked -or -not $projectionReport.async_feedback_checked -or ($stage -eq "restart" -and (-not $projectionReport.restart_checked -or -not $projectionReport.lifecycle_checked -or -not $projectionReport.privileged_history_checked))) { throw "projection evidence is not PC6-ready" }
        Copy-Item -LiteralPath $ProjectionReportPath -Destination "$ProjectionReportPath.$stage.json"
    }
    Write-Output "PASS: public context, durable reference and runtime evidence projection schemas, governed paths, isolation, lifecycle, replay, and API/worker restart; bounded reports: $ReportPath, $ReferenceReportPath and $ProjectionReportPath"
} catch {
    # Do not retain exception text, credentials, raw HTTP bodies, or scope values.
    if (-not $ReportPath) { $ReportPath = Join-Path ([IO.Path]::GetTempPath()) "$ProjectName.context-conformance.json" }
    @{ result = "failed"; category = "context_verification_incomplete"; phase = $phase; consumable = $false } | ConvertTo-Json | Set-Content -LiteralPath $ReportPath -Encoding UTF8
    if ($ReferenceReportPath) { @{ result = "failed"; category = "reference_verification_incomplete"; phase = $phase; consumable = $false } | ConvertTo-Json | Set-Content -LiteralPath $ReferenceReportPath -Encoding UTF8 }
    if ($ProjectionReportPath) { @{ result = "failed"; category = "projection_verification_incomplete"; phase = $phase; consumable = $false } | ConvertTo-Json | Set-Content -LiteralPath $ProjectionReportPath -Encoding UTF8 }
    throw "Provider context verification failed; bounded evidence: $ReportPath"
} finally {
    if ($ownedStarted) { & docker @composeArgs down --volumes --remove-orphans *> $null }
    if ($ownedOverride -and $overridePath) {
        $resolvedOverride = [IO.Path]::GetFullPath($overridePath)
        $expectedOverride = [IO.Path]::GetFullPath((Join-Path ([IO.Path]::GetTempPath()) "stele-context-$ProjectName-$overrideID.yml"))
        if ($resolvedOverride -ne $expectedOverride) { throw "owned override cleanup path mismatch" }
        if (Test-Path -LiteralPath $resolvedOverride) { Remove-Item -LiteralPath $resolvedOverride -Force }
    }
    if ($ownedCredentials -and (Test-Path -LiteralPath $ownedCredentials)) {
        $resolvedCredentials = [IO.Path]::GetFullPath($ownedCredentials)
        $expectedCredentials = [IO.Path]::GetFullPath((Join-Path ([IO.Path]::GetTempPath()) "stele-context-credentials-$ProjectName"))
        if ($resolvedCredentials -ne $expectedCredentials) { throw "owned credential cleanup path mismatch" }
        Remove-Item -LiteralPath $resolvedCredentials -Recurse -Force
    }
    foreach ($variable in $changedVariables) { [Environment]::SetEnvironmentVariable($variable, $previousVariables[$variable], "Process") }
    Pop-Location
}
