param(
    [string]$BaseDir = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$launcher = Join-Path $BaseDir "bin\bookwyrm-launcher.exe"
if (-not (Test-Path $launcher)) { throw "Missing launcher: $launcher" }

Write-Host "Installing Bookwyrm Windows service (base-dir: $BaseDir)"
& $launcher install-service --base-dir $BaseDir
if ($LASTEXITCODE -ne 0) { throw "install-service failed with exit code $LASTEXITCODE" }

Write-Host "Starting Bookwyrm Windows service"
& $launcher start-service --base-dir $BaseDir
if ($LASTEXITCODE -ne 0) { throw "start-service failed with exit code $LASTEXITCODE" }
