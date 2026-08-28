[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Artifact,
    [Parameter(Mandatory = $true)][string]$Manifest,
    [Parameter(Mandatory = $true)][string]$TargetDsn,
    [string]$SourceDsn,
    [switch]$ConfirmDestructive
)

$ErrorActionPreference = "Stop"
if (-not $ConfirmDestructive) { throw "restore is destructive; pass -ConfirmDestructive explicitly" }
if ([string]::IsNullOrWhiteSpace($TargetDsn)) { throw "TargetDsn must be explicit and non-empty" }
if ($TargetDsn -match '(?i)(^|[=/])(?:postgres|template0|template1)(?:$|[?])') { throw "refusing broad or template restore target" }
if (-not [string]::IsNullOrWhiteSpace($SourceDsn) -and $TargetDsn.Trim() -eq $SourceDsn.Trim()) { throw "refusing source-equal restore target" }
if (-not (Test-Path -LiteralPath $Artifact -PathType Leaf)) { throw "backup artifact does not exist" }
if (-not (Test-Path -LiteralPath $Manifest -PathType Leaf)) { throw "backup manifest does not exist" }
if (-not (Get-Command pg_restore -ErrorAction SilentlyContinue)) { throw "pg_restore is required; install PostgreSQL client tools" }

$metadata = Get-Content -Raw -LiteralPath $Manifest | ConvertFrom-Json
$actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $Artifact).Hash.ToLowerInvariant()
if ($actualHash -ne ([string]$metadata.sha256).ToLowerInvariant()) { throw "backup checksum mismatch; refusing restore" }

& pg_restore --exit-on-error --clean --if-exists --no-owner --no-privileges --dbname=$TargetDsn $Artifact
if ($LASTEXITCODE -ne 0) { throw "pg_restore failed with exit code $LASTEXITCODE" }
Write-Host "Restore completed into the explicit target database. Credentials and connection details were not printed."
