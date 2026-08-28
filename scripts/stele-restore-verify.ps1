[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$TargetDsn,
    [Parameter(Mandatory = $true)][string]$Manifest,
    [Parameter(Mandatory = $true)][string]$BaseUrl,
    [Parameter(Mandatory = $true)][string]$ApiKey,
    [string]$AssuranceApiKey,
    [switch]$RecordAssurance,
    [Parameter(Mandatory = $true)][string]$Tenant,
    [Parameter(Mandatory = $true)][string]$Project,
    [Parameter(Mandatory = $true)][string]$Namespace
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path -LiteralPath $Manifest -PathType Leaf)) { throw "backup manifest does not exist" }
if (-not (Get-Command psql -ErrorAction SilentlyContinue)) { throw "psql is required; install PostgreSQL client tools" }
$metadata = Get-Content -Raw -LiteralPath $Manifest | ConvertFrom-Json
$schemaVersion = & psql --no-psqlrc --tuples-only --csv --dbname=$TargetDsn --command "SELECT COALESCE(MAX(version), 0) FROM schema_migrations"
if ($LASTEXITCODE -ne 0) { throw "restore verification failed: migration ledger unavailable" }
if ([string]$schemaVersion.Trim() -ne [string]$metadata.schema_version.Trim() -and [string]$metadata.schema_version -ne "unknown") { throw "restore verification failed: schema version differs from manifest" }

$headers = @{
    "X-API-Key" = $ApiKey
    "X-Stele-Tenant" = $Tenant
    "X-Stele-Project" = $Project
    "X-Stele-Namespace" = $Namespace
}
try {
    $response = Invoke-RestMethod -Method Get -Uri ($BaseUrl.TrimEnd('/') + "/v1/memories?limit=1") -Headers $headers -ErrorAction Stop
} catch {
    throw "restore verification failed: authorized scoped service proof was not available"
}
if ($null -eq $response) { throw "restore verification failed: scoped proof returned no response" }
Write-Host "Restore verification succeeded: current migration ledger and authorized scoped read proof are valid."
if ($RecordAssurance) {
    if ([string]::IsNullOrWhiteSpace($AssuranceApiKey)) { throw "-AssuranceApiKey is required when -RecordAssurance is set" }
    $assuranceHeaders = @{
        "X-API-Key" = $AssuranceApiKey
        "X-Stele-Tenant" = $Tenant
        "X-Stele-Project" = $Project
        "X-Stele-Namespace" = $Namespace
        "Content-Type" = "application/json"
    }
    $proof = @{ target = "backup_restore_proof"; target_id = [string]$metadata.sha256; status = "healthy"; checked_surfaces = @("migration_status", "scoped_read"); result_category = "backup_restore_fresh"; linked_evidence = @{ manifest_sha256 = [string]$metadata.sha256; schema_version = [string]$schemaVersion.Trim() }; actor = "stele-restore-verify"; reason = "record verified disposable restore"; verified_at = [DateTime]::UtcNow.ToString("o") }
    Invoke-RestMethod -Method Post -Uri ($BaseUrl.TrimEnd('/') + "/v1/admin/assurance/recovery-verifications") -Headers $assuranceHeaders -Body ($proof | ConvertTo-Json -Depth 10) -ErrorAction Stop | Out-Null
    Write-Host "Successful restore verification was recorded as backup/restore proof."
} else {
    Write-Host "Record the manifest checksum and verification time through the assurance recovery-verification workflow."
}
