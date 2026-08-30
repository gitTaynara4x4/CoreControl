$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$env:CORETUNER_DEV = "1"
$env:CORETUNER_SERVER_URL = "http://127.0.0.1:8001"
$env:CORETUNER_PUBLIC_URL = "http://127.0.0.1:8001"
$env:CORETUNER_DATA_DIR = Join-Path $PSScriptRoot ".dev\CoreTuner"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go nao foi encontrado no PATH. Instale o Go uma unica vez para desenvolver o CoreControl."
}

Write-Host "CoreControl DEV - sem instalacao"
Write-Host "Dados DEV: $env:CORETUNER_DATA_DIR"
Write-Host "Central:   $env:CORETUNER_SERVER_URL"

Set-Location (Join-Path $PSScriptRoot "desktop\app\src")
go run .
