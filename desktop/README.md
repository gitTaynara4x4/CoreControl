# Aplicativos Windows do CoreTuner

O projeto possui três executáveis separados:

- `CoreTunerSetup.exe`: login/cadastro, vínculo da máquina e instalação.
- `CoreTuner.exe`: aplicativo completo com diagnóstico, testes, perfis, relatórios e administração da empresa.
- `CoreTunerAgent.exe`: agente silencioso que envia telemetria ao Central.

O Setup instala os arquivos em `C:\Program Files\CoreTuner` e grava:

- Dados do aplicativo e sessão: `%LOCALAPPDATA%\CoreTuner`
- Credencial protegida do Agent: `C:\ProgramData\CoreTuner\Agent`

## Compilar no Windows

No PowerShell, na raiz do projeto:

```powershell
$env:CORETUNER_PUBLIC_URL='https://coretuner.seudominio.com.br'
.\desktop\Build_Windows.ps1
```

O script recompila o Agent, o aplicativo completo e o Setup, e copia o Setup para `app\downloads`.
