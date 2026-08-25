@echo off
setlocal
cd /d "%~dp0"
title CoreControl WEB DEV - 127.0.0.1:8001

echo ==========================================
echo          CoreControl - WEB DEV
echo ==========================================
echo.
echo Interface: http://127.0.0.1:8001
echo API docs : http://127.0.0.1:8001/api/docs
echo.
echo Nao e necessario instalar ou abrir CoreTuner.exe.
echo Deixe esta janela aberta enquanto estiver desenvolvendo.
echo Para encerrar, pressione CTRL+C.
echo.

if not exist ".env" (
  copy /Y ".env.example" ".env" >nul
  echo Arquivo .env criado a partir do exemplo.
)

if not exist ".venv\Scripts\python.exe" (
  echo Criando ambiente virtual...
  py -3 -m venv .venv
  if errorlevel 1 goto :error
)

call ".venv\Scripts\activate.bat"
if errorlevel 1 goto :error

if not exist ".venv\.coretuner_deps_ok" (
  echo Instalando dependencias pela primeira vez...
  python -m pip install --disable-pip-version-check -r requirements.txt
  if errorlevel 1 goto :error
  type nul > ".venv\.coretuner_deps_ok"
)

set "CORETUNER_ENV=development"
set "CORETUNER_DEV_WEB=1"
set "CORETUNER_PUBLIC_URL=http://127.0.0.1:8001"
set "CORETUNER_PORT=8001"

rem Abre o navegador uma unica vez. O servidor permanece ligado nesta janela.
start "" powershell -NoProfile -WindowStyle Hidden -Command "Start-Sleep -Milliseconds 1200; Start-Process 'http://127.0.0.1:8001'"

python dev_web.py
if errorlevel 1 goto :error
goto :eof

:error
echo.
echo Nao foi possivel iniciar o CoreControl WEB DEV.
echo Confira o erro acima.
pause
exit /b 1
