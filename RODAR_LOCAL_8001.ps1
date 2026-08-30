$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$env:CORETUNER_ENV = "development"
$env:CORETUNER_DEV_WEB = "1"
$env:CORETUNER_PUBLIC_URL = "http://127.0.0.1:8001"
$env:CORETUNER_SERVER_URL = "http://127.0.0.1:8001"
$env:CORETUNER_PORT = "8001"
$env:PORT = "8001"

if (-not (Test-Path ".venv\Scripts\python.exe")) {
    throw "Ambiente .venv nao encontrado. Rode Iniciar_CoreControl_Local.ps1 uma vez."
}

& ".\.venv\Scripts\python.exe" -m uvicorn app.main:app --reload --host 127.0.0.1 --port 8001
