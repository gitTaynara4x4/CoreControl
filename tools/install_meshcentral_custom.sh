#!/bin/sh
set -eu

CORECONTROL_URL="${CORECONTROL_URL:-https://apps-corecontrol.9ywrah.easypanel.host}"
DEST="${MESH_CUSTOM_JS:-/opt/meshcentral/meshcentral/public/scripts/custom.js}"

curl -fsSL "$CORECONTROL_URL/remote-assets/meshcentral-custom.js" -o "$DEST"
node --check "$DEST"
grep "CoreControl Remote v10.9-front-direct-bind" "$DEST" | head

echo "CoreControl Remote v10.9 instalado em: $DEST"
