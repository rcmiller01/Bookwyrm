param(
    [string]$BaseDir = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$launcher = Join-Path $BaseDir "bin\bookwyrm-launcher.exe"
if (-not (Test-Path $launcher)) { throw "Missing launcher: $launcher" }

Write-Host "Stopping Bookwyrm Windows service"
& $launcher stop-service --base-dir $BaseDir
if ($LASTEXITCODE -ne 0) { Write-Warning "stop-service returned exit code $LASTEXITCODE" }

Write-Host "Uninstalling Bookwyrm Windows service"
& $launcher uninstall-service --base-dir $BaseDir
if ($LASTEXITCODE -ne 0) { throw "uninstall-service failed with exit code $LASTEXITCODE" }
