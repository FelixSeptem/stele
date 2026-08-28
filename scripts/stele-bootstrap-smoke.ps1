[CmdletBinding()]
param(
    [string]$BaseUrl = "http://localhost:8080",
    [Parameter(Mandatory = $true)][string]$BootstrapKey,
    [Parameter(Mandatory = $true)][string]$Tenant,
    [Parameter(Mandatory = $true)][string]$Project,
    [Parameter(Mandatory = $true)][string]$Namespace,
    [Parameter(Mandatory = $true)][string]$CredentialOutputDirectory
)

$ErrorActionPreference = "Stop"
$scopeHeaders = @{
    "X-API-Key" = $BootstrapKey
    "X-Stele-Tenant" = $Tenant
    "X-Stele-Project" = $Project
    "X-Stele-Namespace" = $Namespace
    "Content-Type" = "application/json"
}

function Invoke-Json([string]$Method, [string]$Path, [hashtable]$Headers, [object]$Body) {
    $params = @{ Method = $Method; Uri = ($BaseUrl.TrimEnd('/') + $Path); Headers = $Headers; ErrorAction = "Stop" }
    if ($null -ne $Body) { $params.Body = ($Body | ConvertTo-Json -Depth 10) }
    Invoke-RestMethod @params
}

New-Item -ItemType Directory -Force -Path $CredentialOutputDirectory | Out-Null
Write-Host "Checking runtime discovery endpoints..."
Invoke-Json GET "/health" @{} $null | Out-Null
Invoke-Json GET "/openapi.yaml" @{} $null | Out-Null
Invoke-Json GET "/version" @{} $null | Out-Null

Write-Host "Creating durable administrator in the configured default scope..."
$admin = Invoke-Json POST "/v1/admin/principals" $scopeHeaders @{ role = "admin"; label = "bootstrap-smoke-admin"; actor = "bootstrap-smoke"; reason = "first self-hosted bootstrap" }
if ([string]::IsNullOrWhiteSpace($admin.credential_secret) -or [string]::IsNullOrWhiteSpace($admin.id)) { throw "bootstrap response did not contain one-time administrator credential" }
$adminCredentialPath = Join-Path $CredentialOutputDirectory "admin.credential"
Set-Content -LiteralPath $adminCredentialPath -Value $admin.credential_secret -NoNewline

$adminHeaders = @{
    "X-API-Key" = $admin.credential_secret
    "X-Stele-Tenant" = $Tenant
    "X-Stele-Project" = $Project
    "X-Stele-Namespace" = $Namespace
    "Content-Type" = "application/json"
}
Write-Host "Creating least-privilege runtime principal and exact grant..."
$runtime = Invoke-Json POST "/v1/admin/principals" $adminHeaders @{ role = "public"; label = "bootstrap-smoke-runtime"; actor = "bootstrap-smoke-admin"; reason = "least-privilege runtime principal" }
if ([string]::IsNullOrWhiteSpace($runtime.credential_secret) -or [string]::IsNullOrWhiteSpace($runtime.id)) { throw "runtime principal response did not contain one-time credential" }
Invoke-Json POST ("/v1/admin/principals/{0}/grants" -f $runtime.id) $adminHeaders @{ tenant = $Tenant; project = $Project; namespace = $Namespace; actor = "bootstrap-smoke-admin"; reason = "exact runtime scope grant" } | Out-Null
$runtimeCredentialPath = Join-Path $CredentialOutputDirectory "runtime.credential"
Set-Content -LiteralPath $runtimeCredentialPath -Value $runtime.credential_secret -NoNewline

Write-Host "Verifying bootstrap credential is no longer accepted after durable admin creation..."
try {
    Invoke-Json POST "/v1/admin/principals" $scopeHeaders @{ role = "admin"; label = "bootstrap-disabled-check"; actor = "bootstrap-smoke"; reason = "bootstrap deactivation check" } | Out-Null
    throw "bootstrap credential was still accepted after durable administrator creation"
} catch {
    if ($_.Exception.Message -like "bootstrap credential was still accepted*") { throw }
    $statusCode = $null
    try { $statusCode = [int]$_.Exception.Response.StatusCode } catch { }
    if ($statusCode -notin @(401, 403, 409)) { throw "bootstrap deactivation check failed with an unexpected non-secret status" }
}

Write-Host "Bootstrap smoke completed. One-time credentials were written to: $CredentialOutputDirectory"
Write-Host "Do not commit this directory; move credentials into an operator secret manager and remove the files."
