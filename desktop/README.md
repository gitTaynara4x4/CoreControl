# Aplicativos Windows do CoreTuner

O projeto possui três executáveis separados:

- `CoreTunerSetup.exe`: login/cadastro, vínculo da máquina e instalação.
- `CoreTuner.exe`: aplicativo local com diagnóstico, testes, perfis, programas, relatórios deste computador e suporte.
- `CoreTunerAgent.exe`: agente silencioso que envia telemetria ao Central.

O aplicativo local não exibe a lista de outros computadores da empresa. Visão geral, quantidade de máquinas, online/offline, alertas e administração permanecem no painel web, conforme as permissões da conta.

O Setup mostra somente a empresa vinculada, o usuário conectado e os dados necessários para identificar a máquina. Ele não apresenta quantidade de computadores da empresa.

O Setup instala os arquivos em `C:\Program Files\CoreTuner` e grava:

- Dados do aplicativo e sessão: `%LOCALAPPDATA%\CoreTuner`
- Credencial protegida do Agent: `C:\ProgramData\CoreTuner\Agent`

## Interação

Os menus e botões do aplicativo usam cursor de mão e realce ao passar o mouse. Os botões do Setup também usam cursor de mão; campos de texto mantêm o cursor de edição normal do Windows.

## Compilar no Windows

No PowerShell, na raiz do projeto:

```powershell
$env:CORETUNER_PUBLIC_URL='https://coretuner.seudominio.com.br'
.\desktop\Build_Windows.ps1
```

O script recompila o Agent, o aplicativo completo e o Setup, e copia os executáveis para `app\downloads`.
