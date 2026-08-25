@echo off
setlocal
cd /d "%~dp0"

echo ==========================================
echo       CoreControl - Local
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

echo.
echo Iniciando em http://127.0.0.1:8002
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
