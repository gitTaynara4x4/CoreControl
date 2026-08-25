# CoreControl 0.4.11

Projeto completo com site público, cadastro e login de empresas, painel multiempresa, aplicativo Windows, instalador, agente de telemetria e acesso remoto automático por empresa via MeshCentral.

## Correção do acesso remoto

O backend e o instalador aceitam os dois formatos de identificador de grupo do MeshCentral:

- 32 bytes, equivalentes a 64 caracteres hexadecimais;
- 48 bytes, equivalentes a 96 caracteres hexadecimais, usados pelas versões atuais.

O agente remoto é baixado dinamicamente do MeshCentral para o grupo exato da empresa. Não existe um MeshAgent genérico fixo no pacote.

## Executar localmente

No Windows, execute `Iniciar_CoreControl_Local.bat`. O script:

1. cria `.env` a partir de `.env.example`, quando necessário;
2. cria `.venv`;
3. instala `requirements.txt`;
4. inicia em `http://127.0.0.1:8002`.

Execução manual:

```powershell
py -3 -m venv .venv
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\.venv\Scripts\Activate.ps1
python -m pip install -r requirements.txt
python run.py
```

- Site: `http://127.0.0.1:8002`
- Central: `http://127.0.0.1:8002/central`
- API local: `http://127.0.0.1:8002/api/docs`
- Health: `http://127.0.0.1:8002/health`

## Produção no EasyPanel

- Porta interna: `8280`
- Healthcheck: `/health`
- Dockerfile: raiz do projeto
- Variáveis: use `.env.production.example` apenas como referência
- Banco: PostgreSQL pela variável `CORETUNER_DATABASE_URL`

Senhas, chave do MeshCentral e credenciais SMTP devem ficar somente nas variáveis protegidas do EasyPanel.

## Executáveis

- `app/downloads/CoreControlSetup.exe`: login, cadastro e instalação.
- `app/downloads/CoreControl.exe`: aplicativo local focado somente no computador em uso.
- `app/downloads/CoreControlAgent.exe`: telemetria silenciosa.

Os três executáveis foram recompilados para Windows x64 com o domínio de produção `https://apps-coretuner.9ywrah.easypanel.host` embutido no Setup e no aplicativo.

Nesta versão, o Setup não exibe quantidade de computadores, e o aplicativo local não mostra outros equipamentos da empresa. A administração completa continua no painel web. Menus e botões clicáveis agora exibem cursor de mão e realce ao passar o mouse.

## Segurança

O Agent coleta dados técnicos usando APIs nativas do Windows. Ele não acessa documentos, conversas ou senhas. Os executáveis ainda não possuem assinatura digital; o SmartScreen pode exibir “Fornecedor desconhecido”.
