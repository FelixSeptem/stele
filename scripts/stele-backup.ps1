[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$SourceDsn,
    [Parameter(Mandatory = $true)][string]$Destination,
    [string]$ServiceVersion = "unknown",
    [string]$OpenApiDigest = "unknown"
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($SourceDsn)) { throw "SourceDsn must be explicit and non-empty" }
if ([string]::IsNullOrWhiteSpace($Destination)) { throw "Destination must be explicit and non-empty" }
if (-not (Get-Command pg_dump -ErrorAction SilentlyContinue)) { throw "pg_dump is required; install PostgreSQL client tools" }

$destinationPath = [System.IO.Path]::GetFullPath($Destination)
$parent = Split-Path -Parent $destinationPath
if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
$temporaryPath = "$destinationPath.partial"
if (Test-Path -LiteralPath $temporaryPath) { Remove-Item -LiteralPath $temporaryPath -Force }

try {
    & pg_dump --format=custom --no-owner --no-privileges --file=$temporaryPath --dbname=$SourceDsn
    if ($LASTEXITCODE -ne 0) { throw "pg_dump failed with exit code $LASTEXITCODE" }
    Move-Item -LiteralPath $temporaryPath -Destination $destinationPath -Force
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $destinationPath).Hash.ToLowerInvariant()
    $schemaVersion = "unknown"
    if (Get-Command psql -ErrorAction SilentlyContinue) {
        $value = & psql --no-psqlrc --tuples-only --csv --dbname=$SourceDsn --command "SELECT COALESCE(MAX(version), 0) FROM schema_migrations" 2>$null
        if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace(($value | Out-String))) { $schemaVersion = ($value | Out-String).Trim() }
    }
    $manifest = [ordered]@{
        artifact = [System.IO.Path]::GetFileName($destinationPath)
        sha256 = $hash
        created_at_utc = [DateTime]::UtcNow.ToString("o")
        service_version = $ServiceVersion
        openapi_digest = $OpenApiDigest
        schema_version = $schemaVersion
        format = "pg_dump custom"
    }
    $manifestPath = "$destinationPath.manifest.json"
    $manifest | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $manifestPath -Encoding UTF8 -NoNewline
    Write-Host "Backup created: $([System.IO.Path]::GetFileName($destinationPath))"
    Write-Host "Manifest created: $([System.IO.Path]::GetFileName($manifestPath))"
}
finally {
    if (Test-Path -LiteralPath $temporaryPath) { Remove-Item -LiteralPath $temporaryPath -Force }
}
