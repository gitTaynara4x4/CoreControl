#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
if [ ! -f .env ]; then
  echo "Arquivo .env nao encontrado. Copie .env.example para .env e preencha as senhas."
  exit 1
fi
set -a
. ./.env
set +a
python3 -m venv .venv
. .venv/bin/activate
python -m pip install --disable-pip-version-check -r requirements.txt
cd central
python run.py
