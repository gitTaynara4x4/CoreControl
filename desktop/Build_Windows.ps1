$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$agentSource = Join-Path $root 'agent\src'
$appSource = Join-Path $PSScriptRoot 'app\src'
$setupSource = Join-Path $PSScriptRoot 'setup\src'
$downloadDir = Join-Path $root 'app\downloads'
$publicUrl = if ($env:CORETUNER_PUBLIC_URL) { $env:CORETUNER_PUBLIC_URL } else { 'http://127.0.0.1:8002' }

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

Write-Host 'CoreTunerSetup.exe, CoreTuner.exe e CoreTunerAgent.exe gerados em app\downloads.' -ForegroundColor Green
Write-Host 'Esta compilação não usa PowerShell para coletar diagnóstico.' -ForegroundColor Green
