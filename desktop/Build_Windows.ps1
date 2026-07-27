$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$agentSource = Join-Path $root 'agent\src'
$appSource = Join-Path $PSScriptRoot 'app\src'
$setupSource = Join-Path $PSScriptRoot 'setup\src'
$iconPatchSource = Join-Path $PSScriptRoot 'tools\iconpatch'
$downloadDir = Join-Path $root 'app\downloads'
$publicUrl = if ($env:CORETUNER_PUBLIC_URL) { $env:CORETUNER_PUBLIC_URL } else { 'http://127.0.0.1:8002' }
$iconSource = Join-Path $PSScriptRoot 'assets\coretuner.ico'
if (-not (Test-Path $iconSource)) {
    throw "Ícone oficial não encontrado em $iconSource"
}
Copy-Item -Force $iconSource (Join-Path $appSource 'coretuner.ico')
Copy-Item -Force $iconSource (Join-Path $setupSource 'coretuner.ico')

New-Item -ItemType Directory -Force -Path $downloadDir | Out-Null

$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'

Push-Location $agentSource
go test ./...
go vet ./...
go build -trimpath -ldflags '-H windowsgui -s -w' -o (Join-Path $downloadDir 'CoreTunerAgent.exe') .
Pop-Location

Push-Location $appSource
go test ./...
go vet ./...
go build -trimpath -ldflags "-H windowsgui -s -w -X main.defaultServerURL=$publicUrl" -o (Join-Path $downloadDir 'CoreTuner.exe') .
Pop-Location

Push-Location $setupSource
go test ./...
go vet ./...
go build -trimpath -ldflags "-H windowsgui -s -w -X main.defaultServerURL=$publicUrl" -o (Join-Path $downloadDir 'CoreTunerSetup.exe') .
Pop-Location

Push-Location $iconPatchSource
go test ./...
go run . -exe (Join-Path $downloadDir 'CoreTunerAgent.exe') -ico $iconSource
go run . -exe (Join-Path $downloadDir 'CoreTuner.exe') -ico $iconSource
go run . -exe (Join-Path $downloadDir 'CoreTunerSetup.exe') -ico $iconSource
Pop-Location

Write-Host 'CoreTunerSetup.exe, CoreTuner.exe e CoreTunerAgent.exe gerados em app\downloads.' -ForegroundColor Green
Write-Host 'Esta compilação não usa PowerShell para coletar diagnóstico.' -ForegroundColor Green
