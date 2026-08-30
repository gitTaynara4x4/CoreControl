# CoreControl - correção real da leitura de abas do navegador (Agent 0.8.6)

## Causa encontrada
A versão 0.8.5 executava um script PowerShell para enumerar as abas via Windows UI Automation.
O script usava a variável `$pid` para guardar o ID do processo da janela.

No PowerShell, `$PID` é uma variável automática somente leitura e os nomes de variáveis não diferenciam maiúsculas/minúsculas.
Por isso, a atribuição `$pid=...` falhava silenciosamente (o script estava com `SilentlyContinue`) e nenhuma aba era devolvida ao Agent.

## Correção
- `$pid` foi substituído por `$processId`.
- A saída do PowerShell foi fixada em UTF-8 para títulos com acentos.
- Agent atualizado de 0.8.5 para 0.8.6.
- `CoreControlAgent.exe` recompilado para Windows x64.
- `CoreTunerAgent.exe` atualizado como alias legado.

## Testes
- 3 testes específicos: OK.
- Build Windows x64: OK.

Depois de publicar estes arquivos, atualize/reinstale o CoreControl no computador monitorado para que ele passe a usar o Agent 0.8.6.
