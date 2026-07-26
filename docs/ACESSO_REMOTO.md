# CoreTuner Remote — instalação automática por empresa

## Fluxo

1. O Setup pergunta se o cliente autoriza o acesso remoto.
2. O backend cria ou localiza um grupo exclusivo para a empresa no MeshCentral.
3. O backend garante o usuário técnico `coretuner-integracao` e concede somente as permissões necessárias ao grupo.
4. O backend baixa do próprio MeshCentral o agente vinculado ao grupo.
5. O Setup verifica um Mesh Agent existente. Se ele pertencer a outro servidor ou grupo, solicita autorização do Windows para substituí-lo.
6. A instalação só é concluída quando o serviço está rodando e o computador aparece online no MeshCentral.
7. O painel salva o ID real do nó e abre diretamente o computador selecionado.

## Compatibilidade do ID do grupo

O CoreTuner aceita:

- 32 bytes / 64 caracteres hexadecimais, para instalações antigas;
- 48 bytes / 96 caracteres hexadecimais, para o MeshCentral atual;
- identificadores Base64 modificados do MeshCentral;
- valores hexadecimais com ou sem prefixo `0x`.

## Variáveis exigidas no serviço `coretuner`

```env
CORETUNER_REMOTE_ENABLED=true
CORETUNER_REMOTE_URL=https://apps-coretuner-remote.9ywrah.easypanel.host
CORETUNER_REMOTE_LOGIN_TOKEN_KEY=CHAVE_HEXADECIMAL_ATUAL_DE_160_CARACTERES
CORETUNER_REMOTE_LOGIN_USER=coretuner-integracao
CORETUNER_REMOTE_LOGIN_DOMAIN=
CORETUNER_REMOTE_LOGIN_TOKEN_MINUTES=2
CORETUNER_REMOTE_ADMIN_USER=user-mesh-adm
CORETUNER_REMOTE_NODE_PATH=node
CORETUNER_REMOTE_MESHCTRL_PATH=/opt/meshcentral-client/node_modules/meshcentral/meshctrl.js
CORETUNER_REMOTE_AGENT_FILENAME=CoreTunerRemoteAgent.exe
CORETUNER_REMOTE_AGENT_TYPE=4
CORETUNER_REMOTE_AGENT_INSTALL_FLAGS=2
CORETUNER_REMOTE_GROUP_FEATURES=2
CORETUNER_REMOTE_GROUP_CONSENT=73
CORETUNER_REMOTE_COMMAND_TIMEOUT_SECONDS=45
CORETUNER_REMOTE_AGENT_DOWNLOAD_TIMEOUT_SECONDS=90
CORETUNER_REMOTE_AGENT_CACHE_SECONDS=86400
CORETUNER_REMOTE_STATUS_CACHE_SECONDS=15
CORETUNER_REMOTE_STATUS_STALE_SECONDS=120
```

A chave nunca deve ser gravada no GitHub, enviada ao navegador ou incorporada aos executáveis.

## Observações

- O domínio fica vazio quando o MeshCentral usa o domínio padrão.
- O Dockerfile instala Node.js e MeshCtrl no serviço principal `coretuner`.
- O agente remoto específico da empresa é armazenado somente no cache persistente `/data/remote-agents`.
- O painel só mostra o remoto como disponível após confirmação real no MeshCentral.
