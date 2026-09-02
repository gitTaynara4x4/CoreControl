# CoreControl v10.7 — Otimização remota pelo painel

Patch incremental para aplicar **por cima da v10.6**. Contém somente arquivos alterados.

## O que muda

- Adiciona um card **Otimização** na tela do computador.
- Administrador Global, Administrador da plataforma e Administrador da empresa podem aplicar remotamente:
  1. Conservador
  2. Equilibrado
  3. Modo Atendimento
  4. Alto Desempenho
  5. Desativar otimização / restaurar original
- Usa a fila autenticada que o Agent já consulta a cada ~5 segundos.
- Mostra no painel: fila, aplicação em andamento, sucesso, falha, alterações concluídas e avisos.
- Registra solicitação e resultado em `audit_logs` / `agent_commands`.
- Mantém backup automático em `%LOCALAPPDATA%\CoreTuner\optimization-state.json` antes da primeira alteração.
- A restauração reutiliza o mesmo backup e só o arquiva depois de uma restauração completa.
- Não apaga arquivos, não esvazia Lixeira/Downloads e não desativa Defender ou Firewall.
- Prioridade de aplicativos, quando usada, fica somente em **Acima do normal**; não usa prioridade Alta/Tempo real.
- Agent atualizado para **0.9.0**.

## Depois de publicar

1. Substitua os arquivos deste ZIP no projeto atual e faça **Force Rebuild** do CoreControl.
2. Abra o computador já existente da Luiza.
3. Clique em **Reinstalar / atualizar CoreControl** e execute o instalador no mesmo PC.
   - Não use “Adicionar computador”.
   - O mesmo dispositivo/histórico é preservado.
   - Essa atualização é necessária uma única vez para instalar o Agent 0.9.0.
4. Quando o painel mostrar `Agente 0.9.0`, abra o card **Otimização** e escolha o perfil.

## Validações executadas

- build Windows amd64 do Agent 0.9.0;
- `go test -c`/compilação Windows do Agent;
- `python -m py_compile` em API/schemas;
- `node --check` no JavaScript da página de computadores;
- testes direcionados da v10.7 e preservação do auto-refresh v10.2: **7 passed**.
