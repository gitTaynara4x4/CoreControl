# CoreTuner Central 0.4.3

Projeto completo e limpo com site público, cadastro/login de empresas, painel multiempresa, aplicativo Windows, instalador e agente de telemetria.

## Correção principal desta versão

A coleta técnica do `CoreTunerAgent.exe` e do `CoreTuner.exe` foi reescrita para usar APIs nativas do Windows. Eles não iniciam `powershell.exe` para coletar CPU, memória, disco, identificação da máquina, processos ou áudio. Assim, nenhuma janela de PowerShell deve abrir durante o monitoramento.

O Setup também encerra uma instalação antiga do Agent antes de substituí-la, evitando múltiplas versões em execução.

## Rodar localmente na porta 8002

```powershell
py -3 -m venv .venv
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\.venv\Scripts\Activate.ps1
python -m pip install -r requirements.txt
python run.py
```

Ou execute `Iniciar_CoreTuner_Local.bat`.

- Site: `http://127.0.0.1:8002`
- Central: `http://127.0.0.1:8002/central`
- API: `http://127.0.0.1:8002/api/docs`
- Health: `http://127.0.0.1:8002/health`

O `.env` local já está configurado para a porta 8002 e senha de download `0610`.

## EasyPanel

- Porta interna de produção: `8280`
- Healthcheck: `/health`
- Use `.env.production.example` como referência.
- A URL real do PostgreSQL deve ficar apenas nas variáveis protegidas do EasyPanel.

## Estrutura sem duplicações

- `app/`: backend, site, painel e executáveis servidos.
- `app/downloads/CoreTunerSetup.exe`: instalador baixado pelo site.
- `app/downloads/CoreTuner.exe`: aplicativo completo baixado pelo Setup.
- `app/downloads/CoreTunerAgent.exe`: agente baixado pelo Setup.
- `desktop/setup/src/`: código-fonte do Setup.
- `desktop/app/src/`: código-fonte do aplicativo completo.
- `agent/src/`: código-fonte do Agent.

Não existem mais a pasta duplicada `central`, uma `.venv` empacotada, bancos locais usados em testes ou cópias diferentes do Agent.

## Segurança

O Agent é somente leitura e não acessa documentos, conversas ou senhas. Não aplica otimizações, não executa scripts remotos, não desativa o Defender e não apaga arquivos.

Os executáveis ainda não possuem assinatura digital. Por isso, o SmartScreen pode mostrar “Fornecedor desconhecido”; isso é diferente de abrir janelas de PowerShell.
