# Aplicativos Windows do CoreControl

O projeto possui três executáveis separados:

- `CoreControlSetup.exe`: login/cadastro, vínculo da máquina e instalação.
- `CoreControl.exe`: aplicativo local com diagnóstico, testes, perfis, programas, relatórios deste computador e suporte.
- `CoreControlAgent.exe`: agente silencioso que envia telemetria ao Central.

O aplicativo local não exibe a lista de outros computadores da empresa. Visão geral, quantidade de máquinas, online/offline, alertas e administração permanecem no painel web, conforme as permissões da conta.

O Setup mostra somente a empresa vinculada, o usuário conectado e os dados necessários para identificar a máquina. Ele não apresenta quantidade de computadores da empresa.

O Setup instala os executáveis em `%LOCALAPPDATA%\Programs\CoreControl`.

Por compatibilidade com instalações anteriores, os dados locais ainda usam a pasta técnica legada `%LOCALAPPDATA%\CoreTuner`; isso evita perder sessão, backup e configuração durante a troca de marca.

## Interação

Os menus e botões do aplicativo usam cursor de mão e realce ao passar o mouse. Os botões do Setup também usam cursor de mão; campos de texto mantêm o cursor de edição normal do Windows.

## Compilar no Windows

No PowerShell, na raiz do projeto:

```powershell
$env:CORETUNER_PUBLIC_URL='https://coretuner.seudominio.com.br'
.\desktop\Build_Windows.ps1
```

O script recompila o Agent, o aplicativo completo e o Setup, e copia os executáveis para `app\downloads`.
