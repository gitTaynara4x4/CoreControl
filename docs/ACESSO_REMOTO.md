# CoreTuner Remote — instalação automática por empresa

A versão 0.4.9 não usa mais um MeshAgent fixo gravado no pacote.

## Fluxo automático

1. O CoreTuner Setup pergunta se o cliente autoriza o acesso remoto.
2. O backend cria ou localiza um grupo exclusivo para a empresa no MeshCentral.
3. O backend cria o usuário técnico `coretuner-integracao`, quando ele ainda não existe, e concede ao grupo somente as permissões necessárias para controle remoto.
4. O backend baixa do próprio MeshCentral um agente vinculado ao identificador real daquele grupo.
5. O Setup verifica se já existe MeshAgent no Windows. Quando o agente existente pertence a outro servidor ou grupo, ele é removido e substituído somente após a autorização do usuário.
6. A instalação só é considerada concluída quando o serviço está rodando e o computador aparece online no MeshCentral.
7. O painel salva o ID real do nó remoto e abre diretamente o computador selecionado.

## Variáveis exigidas no serviço `coretuner`

```env
CORETUNER_REMOTE_ENABLED=true
CORETUNER_REMOTE_URL=https://apps-coretuner-remote.9ywrah.easypanel.host
CORETUNER_REMOTE_LOGIN_TOKEN_KEY=CHAVE_PRIVADA_ATUAL_DO_MESHCENTRAL
CORETUNER_REMOTE_LOGIN_USER=coretuner-integracao
CORETUNER_REMOTE_LOGIN_DOMAIN=
CORETUNER_REMOTE_LOGIN_TOKEN_MINUTES=2
CORETUNER_REMOTE_ADMIN_USER=user-mesh-adm
CORETUNER_REMOTE_NODE_PATH=node
CORETUNER_REMOTE_MESHCTRL_PATH=/opt/meshcentral-client/node_modules/meshcentral/meshctrl.js
CORETUNER_REMOTE_AGENT_TYPE=4
CORETUNER_REMOTE_AGENT_INSTALL_FLAGS=2
CORETUNER_REMOTE_GROUP_FEATURES=2
CORETUNER_REMOTE_GROUP_CONSENT=73
```

A chave nunca deve ser enviada ao navegador, gravada no GitHub ou colocada dentro dos executáveis.

## Observações

- `CORETUNER_REMOTE_LOGIN_DOMAIN=` permanece vazio quando o MeshCentral usa o domínio padrão.
- O arquivo antigo `app/downloads/CoreTunerRemoteAgent.exe` não é mais publicado pelo manifesto e pode ser removido do repositório.
- O Dockerfile instala Node.js e MeshCtrl no serviço principal `coretuner`.
- O painel só mostra o remoto como disponível após confirmação de conexão real no MeshCentral.
