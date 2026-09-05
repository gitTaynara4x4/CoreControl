#!/bin/sh
set -eu
DESTINO="/opt/meshcentral/meshcentral/public/scripts/custom.js"
TEMP="${DESTINO}.tmp"
URL="${CORECONTROL_REMOTE_ASSET_URL:-https://apps-corecontrol.9ywrah.easypanel.host/remote-assets/meshcentral-custom.js}"

echo "Baixando custom.js do CoreControl: $URL"
curl -fsSL --connect-timeout 10 --max-time 30 "$URL" -o "$TEMP"
node --check "$TEMP"
grep -q "CoreControl Remote v10.8-node-bind" "$TEMP"
test -s "$TEMP"
cp "$DESTINO" "${DESTINO}.bak" 2>/dev/null || true
mv "$TEMP" "$DESTINO"
chmod 0644 "$DESTINO"
echo "Instalado com sucesso."
wc -c "$DESTINO"
grep -n "CoreControl Remote v10.8-node-bind" "$DESTINO" | head -1
