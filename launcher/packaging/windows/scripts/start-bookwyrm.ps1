param(
    [string]$BaseDir = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path,
    [switch]$OpenBrowser
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$binDir = Join-Path $BaseDir "bin"
$configDir = Join-Path $BaseDir "config"
$launcher = Join-Path $binDir "bookwyrm-launcher.exe"
$envFile = Join-Path $configDir "bookwyrm.env"
$envExample = Join-Path $configDir "bookwyrm.env.example"
$metaFile = Join-Path $configDir "metadata-service.yaml"
$metaExample = Join-Path $configDir "metadata-service.yaml.example"

if (-not (Test-Path $launcher)) { throw "Missing launcher: $launcher" }
if (-not (Test-Path $binDir)) { throw "Missing bin directory: $binDir" }
if (-not (Test-Path $configDir)) { throw "Missing config directory: $configDir" }

if (-not (Test-Path $envFile) -and (Test-Path $envExample)) {
    Copy-Item $envExample $envFile
    Write-Host "Created $envFile from template."
}
if (-not (Test-Path $metaFile) -and (Test-Path $metaExample)) {
    Copy-Item $metaExample $metaFile
    Write-Host "Created $metaFile from template."
}

if (-not (Test-Path $envFile) -or -not (Test-Path $metaFile)) {
    throw "Missing config files. Ensure both $envFile and $metaFile exist."
}

if ($OpenBrowser) {
    Start-Process "http://localhost:8090" | Out-Null
}

Write-Host "Starting Bookwyrm from $BaseDir ..."
& $launcher run --base-dir $BaseDir
