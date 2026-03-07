param(
    [Parameter(Mandatory = $false)]
    [string]$Version = "0.1.0-alpha",
    [Parameter(Mandatory = $false)]
    [string]$OutputDir = "dist\release"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-External {
    param(
        [Parameter(Mandatory = $true)]
        [scriptblock]$Command
    )
    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed with exit code $LASTEXITCODE"
    }
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$outputRoot = Join-Path $repoRoot $OutputDir
$packageRoot = Join-Path $outputRoot "package"
$stageRoot = Join-Path $packageRoot "Bookwyrm"
$stageBin = Join-Path $stageRoot "bin"
$stageConfig = Join-Path $stageRoot "config"
$stageDocs = Join-Path $stageRoot "docs"
$stageScripts = Join-Path $stageRoot "scripts"
$stageLogs = Join-Path $stageRoot "logs"
$stageData = Join-Path $stageRoot "data"

Write-Host "Preparing release directories..."
Remove-Item -Recurse -Force $packageRoot -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $stageBin | Out-Null
New-Item -ItemType Directory -Force -Path $stageConfig | Out-Null
New-Item -ItemType Directory -Force -Path $stageDocs | Out-Null
New-Item -ItemType Directory -Force -Path $stageScripts | Out-Null
New-Item -ItemType Directory -Force -Path $stageLogs | Out-Null
New-Item -ItemType Directory -Force -Path $stageData | Out-Null

Write-Host "Building web assets..."
Push-Location (Join-Path $repoRoot "app\backend\web")
Invoke-External { npm ci }
Invoke-External { npm run build }
Pop-Location

Write-Host "Building Windows binaries..."
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

Push-Location (Join-Path $repoRoot "launcher")
Invoke-External { go build -trimpath -ldflags "-X main.version=$Version" -o (Join-Path $stageBin "bookwyrm-launcher.exe") ./cmd/bookwyrm-launcher }
Pop-Location

Push-Location (Join-Path $repoRoot "metadata-service")
Invoke-External { go build -trimpath -ldflags "-X main.version=$Version" -o (Join-Path $stageBin "metadata-service.exe") ./cmd/server }
Pop-Location

Push-Location (Join-Path $repoRoot "indexer-service")
Invoke-External { go build -trimpath -ldflags "-X main.version=$Version" -o (Join-Path $stageBin "indexer-service.exe") ./cmd/server }
Pop-Location

Push-Location (Join-Path $repoRoot "app\backend")
Invoke-External { go build -trimpath -ldflags "-X main.version=$Version" -o (Join-Path $stageBin "backend.exe") ./cmd/server }
Pop-Location

Write-Host "Copying config templates and docs..."
Copy-Item (Join-Path $repoRoot "launcher\config\bookwyrm.env.example") (Join-Path $stageConfig "bookwyrm.env.example")
Copy-Item (Join-Path $repoRoot "launcher\config\bookwyrm.env.example") (Join-Path $stageConfig "bookwyrm.env")
Copy-Item (Join-Path $repoRoot "launcher\config\metadata-service.yaml.example") (Join-Path $stageConfig "metadata-service.yaml.example")
Copy-Item (Join-Path $repoRoot "launcher\config\metadata-service.yaml.example") (Join-Path $stageConfig "metadata-service.yaml")
Copy-Item (Join-Path $repoRoot "README.md") (Join-Path $stageRoot "README.md")
Copy-Item (Join-Path $repoRoot "docs\RUN-BOOKWYRM-WINDOWS.md") (Join-Path $stageDocs "RUN-BOOKWYRM-WINDOWS.md")
Copy-Item (Join-Path $repoRoot "docs\windows-native.md") (Join-Path $stageDocs "windows-native.md")
Copy-Item (Join-Path $repoRoot "docs\troubleshooting.md") (Join-Path $stageDocs "troubleshooting.md")
Copy-Item (Join-Path $repoRoot "launcher\packaging\windows\scripts\start-bookwyrm.ps1") (Join-Path $stageScripts "start-bookwyrm.ps1")
Copy-Item (Join-Path $repoRoot "launcher\packaging\windows\scripts\install-service.ps1") (Join-Path $stageScripts "install-service.ps1")
Copy-Item (Join-Path $repoRoot "launcher\packaging\windows\scripts\uninstall-service.ps1") (Join-Path $stageScripts "uninstall-service.ps1")

New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
$zipPath = Join-Path $outputRoot ("bookwyrm-{0}-windows.zip" -f $Version)
if (Test-Path $zipPath) {
    Remove-Item -Force $zipPath
}
Compress-Archive -Path $stageRoot -DestinationPath $zipPath
Write-Host "Created zip artifact: $zipPath"
Write-Warning "Installer (.exe) packaging is intentionally disabled for open alpha distribution."
