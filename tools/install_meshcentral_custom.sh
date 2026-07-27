#!/bin/sh
set -eu

DESTINO="/opt/meshcentral/meshcentral/public/scripts/custom.js"
TEMPORARIO="${DESTINO}.tmp"
URL_INTERNA="${CORETUNER_INTERNAL_ASSET_URL:-http://apps-coretuner:8280/remote-assets/meshcentral-custom.js}"
URL_PUBLICA="${CORETUNER_PUBLIC_ASSET_URL:-https://apps-coretuner.9ywrah.easypanel.host/remote-assets/meshcentral-custom.js}"

baixar() {
    url="$1"
    echo "Baixando: $url"
    curl -fsSL --connect-timeout 10 --max-time 30 "$url" -o "$TEMPORARIO"
}

rm -f "$TEMPORARIO"
if ! baixar "$URL_INTERNA"; then
    echo "A URL interna falhou; tentando a URL pública."
    baixar "$URL_PUBLICA"
fi

node --check "$TEMPORARIO"
grep -q "CoreTuner Remote v4" "$TEMPORARIO"

cp "$DESTINO" "${DESTINO}.bak" 2>/dev/null || true
mv "$TEMPORARIO" "$DESTINO"
chmod 0644 "$DESTINO"

printf 'Tamanho: '
wc -c < "$DESTINO"
printf 'Hash SHA-256: '
sha256sum "$DESTINO" | awk '{print $1}'

echo "Script V4 instalado com sucesso."
