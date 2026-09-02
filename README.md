# CoreControl v10.5 — correção do acesso remoto automático

Diagnóstico real encontrado:

- o backend gera `remote-session` corretamente;
- o MeshCentral aceita o login token;
- o nó correto é enviado pelo backend;
- durante o login, o MeshCentral remove `ctnode`/`gotonode`;
- `coretuner` sobrevive ao login;
- o `custom.js` do serviço `coretuner-remote` estava com **0 bytes**;
- existem nós antigos duplicados, portanto nunca se deve escolher o PC somente pelo nome.

Correção:

1. `app/meshcentral.py`: transporta o node ID exato, em Base64URL, dentro do parâmetro `coretuner`, que sobrevive ao login.
2. `app/remote_assets/meshcentral-custom.js`: decodifica o node ID, seleciona apenas o nó exato, espera o Mesh Agent online e chama `connectDesktop(null, 1)`.
3. `tools/install_meshcentral_custom.sh`: instala o script no container MeshCentral e rejeita arquivo vazio/inválido.

## Depois de aplicar no CoreControl

Faça Force Rebuild do serviço CoreControl.

No terminal do serviço `coretuner-remote`, rode:

```sh
curl -fsSL https://apps-corecontrol.9ywrah.easypanel.host/remote-assets/meshcentral-custom.js \
  -o /opt/meshcentral/meshcentral/public/scripts/custom.js

node --check /opt/meshcentral/meshcentral/public/scripts/custom.js
wc -c /opt/meshcentral/meshcentral/public/scripts/custom.js
grep -n "CoreControl Remote v10.5" /opt/meshcentral/meshcentral/public/scripts/custom.js | head
```

O tamanho deve ser maior que zero e o grep deve mostrar `CoreControl Remote v10.5`.

Não é necessário reinstalar o PC da Luiza.
