# CoreControl v10.9.5 — Notificação de otimização no Windows

Patch somente com os arquivos alterados, para aplicar por cima da v10.9.4.

## O que mudou
- CoreControl Agent atualizado para **0.9.2**.
- Ao aplicar um perfil com sucesso, o usuário do computador recebe uma notificação discreta do Windows.
- A notificação não abre janela modal e some sozinha.
- Mensagens específicas para Conservador, Equilibrado, Modo Atendimento e Alto Desempenho.
- Ao usar **Restaurar original**, o Windows informa que a otimização foi desativada e as configurações anteriores foram restauradas.
- A notificação só é disparada depois que o perfil realmente foi aplicado com sucesso.
- O resultado técnico completo continua sendo mostrado no painel administrativo do CoreControl.

## Exemplos
- `CoreControl — Modo Atendimento ativado` — O computador foi preparado para priorizar seus aplicativos de trabalho.
- `CoreControl — Alto Desempenho ativado` — O computador foi otimizado para maior desempenho.
- `CoreControl — Otimização desativada` — As configurações anteriores do computador foram restauradas.

## Deploy
1. Aplicar este patch por cima da v10.9.4.
2. Fazer Force Rebuild do CoreControl.
3. No computador existente, usar **Reinstalar / atualizar CoreControl** uma vez para instalar o Agent 0.9.2.
4. Não usar “Adicionar computador” e não excluir o dispositivo existente.

## Arquivos alterados
- `agent/src/main.go`
- `agent/src/optimization_windows.go`
- `app/downloads/CoreControlAgent.exe`
- `app/downloads/CoreTunerAgent.exe`

## Validação
- Agent Windows amd64 compilado com sucesso.
- `GOOS=windows GOARCH=amd64` compilou o pacote completo.
- Ícone oficial CoreControl incorporado ao executável.
- SHA-256 do `CoreControlAgent.exe`: `4d3caa627d129c995a7db99930a9587f573cb868f984e312230a464aa12d0a36`.
- `go vet` ainda acusa um aviso preexistente em `activity_icons_windows.go` sobre `unsafe.Pointer`; essa área não foi alterada por este patch.
