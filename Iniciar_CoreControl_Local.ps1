$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if (-not (Test-Path ".env")) {
    Copy-Item ".env.example" ".env"
}

if (-not (Test-Path ".venv\Scripts\python.exe")) {
    py -3 -m venv .venv
}

& ".\.venv\Scripts\python.exe" -m pip install --disable-pip-version-check -r requirements.txt

# Somente para a execucao LOCAL desta janela.
# O .env de producao/VPS permanece intacto.
$env:CORETUNER_ENV = "development"
$env:CORETUNER_DEV_WEB = "1"
$env:CORETUNER_PUBLIC_URL = "http://127.0.0.1:8001"
$env:CORETUNER_SERVER_URL = "http://127.0.0.1:8001"
$env:CORETUNER_PORT = "8001"
$env:PORT = "8001"

Write-Host "CoreControl LOCAL: http://127.0.0.1:8001"
Write-Host "API docs:          http://127.0.0.1:8001/api/docs"
Write-Host ""

& ".\.venv\Scripts\python.exe" run.py
