param(
    [Parameter(Mandatory = $false)]
    [string]$BaseUrl = "http://localhost:8090"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-Check {
    param(
        [string]$Name,
        [string]$Path
    )

    $url = "$BaseUrl$Path"
    Write-Host "Checking $Name ($url)"
    $resp = Invoke-RestMethod -Method Get -Uri $url
    return $resp
}

$health = Invoke-Check -Name "healthz" -Path "/api/v1/healthz"
$ready = Invoke-Check -Name "readyz" -Path "/api/v1/readyz"
$deps = Invoke-Check -Name "dependencies" -Path "/api/v1/system/dependencies"
$migrations = Invoke-Check -Name "migration-status" -Path "/api/v1/system/migration-status"

Write-Host "healthz status: $($health.status)"
Write-Host "readyz status: $($ready.status)"
Write-Host "can_function_now: $($deps.can_function_now)"
Write-Host "migration status: $($migrations.status)"

if (-not $deps.can_function_now) {
    throw "Functional readiness invariant failed: can_function_now=false"
}
if ($migrations.status -ne "ok") {
    throw "Migration status is not ok: $($migrations.status)"
}

Write-Host "Alpha install validation checks passed."
