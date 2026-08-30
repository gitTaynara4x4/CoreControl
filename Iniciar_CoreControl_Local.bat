@echo off
setlocal
cd /d "%~dp0"

echo ==========================================
echo       CoreControl - Local :8001
echo ==========================================
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

echo Instalando ou conferindo dependencias...
python -m pip install --disable-pip-version-check -r requirements.txt
if errorlevel 1 goto :error

rem IMPORTANTE: estas variaveis valem SOMENTE para esta janela/local.
rem Nao alteram o .env usado na VPS.
set "CORETUNER_ENV=development"
set "CORETUNER_DEV_WEB=1"
set "CORETUNER_PUBLIC_URL=http://127.0.0.1:8001"
set "CORETUNER_SERVER_URL=http://127.0.0.1:8001"
set "CORETUNER_PORT=8001"
set "PORT=8001"

echo.
echo Iniciando em http://127.0.0.1:8001
echo API docs: http://127.0.0.1:8001/api/docs
echo Para encerrar, pressione CTRL+C.
echo.
python run.py
if errorlevel 1 goto :error

goto :eof

:error
echo.
echo Nao foi possivel iniciar o CoreControl.
echo Confira se o Python esta instalado e envie o erro exibido acima.
pause
exit /b 1
