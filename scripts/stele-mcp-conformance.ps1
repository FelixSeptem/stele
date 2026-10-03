[CmdletBinding()]
param(
    [string]$EvidencePath,
    [string]$ApiBaseUrl,
    [int]$TimeoutSeconds = 150,
    [switch]$ExpectEnabled,
    [string]$McpPath = "/mcp"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    if ([string]::IsNullOrWhiteSpace($McpPath) -or $McpPath -notmatch '^/[a-zA-Z0-9/_-]{1,64}$') {
        throw "McpPath must be a bounded absolute endpoint path"
    }
    if ([string]::IsNullOrWhiteSpace($env:STELE_TEST_POSTGRES_MCP_DSN)) {
        Write-Output "SKIP: STELE_TEST_POSTGRES_MCP_DSN is not configured; MCP real-stack conformance did not run"
        exit 2
    }
    if ($TimeoutSeconds -lt 30 -or $TimeoutSeconds -gt 600) {
        throw "TimeoutSeconds must be between 30 and 600"
    }
    if (-not [string]::IsNullOrWhiteSpace($EvidencePath)) {
        $resolvedEvidencePath = [IO.Path]::GetFullPath($EvidencePath)
        $evidenceDirectory = Split-Path -Parent $resolvedEvidencePath
        if (-not [string]::IsNullOrWhiteSpace($evidenceDirectory)) {
            New-Item -ItemType Directory -Force -Path $evidenceDirectory | Out-Null
        }
        $env:STELE_MCP_CONFORMANCE_EVIDENCE = $resolvedEvidencePath
    } else {
        Remove-Item Env:STELE_MCP_CONFORMANCE_EVIDENCE -ErrorAction SilentlyContinue
    }

    if (-not [string]::IsNullOrWhiteSpace($ApiBaseUrl)) {
        $baseUrl = $ApiBaseUrl.TrimEnd('/')
        foreach ($path in @('/health', '/openapi.yaml')) {
            try {
                $response = Invoke-WebRequest -Method Get -Uri ($baseUrl + $path) -ErrorAction Stop
                if ([int]$response.StatusCode -ne 200) {
                    throw "API probe $path returned HTTP $($response.StatusCode)"
                }
            } catch {
                throw "API probe $path failed without exposing response details"
            }
        }
        $mcpUri = $baseUrl + "/" + $McpPath.TrimStart('/')
        if ($ExpectEnabled) {
            try {
                $mcpResponse = Invoke-WebRequest -Method Get -Uri $mcpUri -ErrorAction Stop
                $mcpStatus = [int]$mcpResponse.StatusCode
            } catch {
                $mcpStatus = $null
                try { $mcpStatus = [int]$_.Exception.Response.StatusCode } catch { }
            }
            if ($mcpStatus -notin @(200, 400, 401, 405)) {
                throw "MCP enabled probe did not expose the configured endpoint"
            }
        } else {
            try {
                $mcpResponse = Invoke-WebRequest -Method Get -Uri $mcpUri -ErrorAction Stop
                throw "MCP disabled probe returned HTTP $($mcpResponse.StatusCode), expected not-found"
            } catch {
                $statusCode = $null
                try { $statusCode = [int]$_.Exception.Response.StatusCode } catch { }
                if ($statusCode -ne 404) {
                    throw "MCP disabled probe did not fail closed"
                }
            }
        }
    }

    Write-Output "Running opt-in MCP PostgreSQL + pgvector conformance matrix..."
    & go test ./internal/storage/postgres -run '^TestMCPPostgresConformanceMatrix$' -count=1 -timeout "$($TimeoutSeconds)s"
    if ($LASTEXITCODE -ne 0) {
        throw "MCP real-stack conformance failed"
    }
    Write-Output "PASS: MCP real-stack conformance completed with bounded redacted evidence"
}
finally {
    Remove-Item Env:STELE_MCP_CONFORMANCE_EVIDENCE -ErrorAction SilentlyContinue
    Pop-Location
}
