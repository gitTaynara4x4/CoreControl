# CoreControl v10.9.6 — Telemetria de GPU e temperatura real

Patch somente com arquivos alterados, para aplicar por cima da sequência atual v10.9.4 + v10.9.5.

## Alterações
- Agent atualizado para 0.9.3.
- Coleta NVIDIA via `nvidia-smi` sem abrir janela no computador monitorado.
- Temperatura da GPU passa a preencher o card principal quando o Windows não expõe temperatura ACPI.
- Coleta uso da GPU, VRAM usada/total, modelo e versão do driver.
- Painel principal ganha card `GPU` e identifica `Temperatura GPU` quando esta for a fonte real.
- Diagnóstico inteligente mostra modelo da GPU, uso e VRAM junto da temperatura.
- Diagnóstico continua sem inventar temperatura de CPU: se ACPI não estiver disponível, a fonte é explicitamente GPU NVIDIA.

## Instalação
1. Aplicar os arquivos por cima da versão atual.
2. Force Rebuild do CoreControl.
3. Ctrl+F5 no painel.
4. Reinstalar/atualizar o CoreControl no mesmo cadastro do computador uma vez para receber o Agent 0.9.3.

Não usar `Adicionar computador` e não excluir o dispositivo existente.

## Validação
- Agent Windows amd64 compilado com sucesso.
- `go test ./...` OK.
- `node --check app/static/js/pages/devices.js` OK.
- `python -m py_compile app/api.py` OK.
- 16 testes Python direcionados passaram.
- `go vet` ainda aponta um aviso preexistente em `activity_icons_windows.go` sobre `unsafe.Pointer`; não foi introduzido por este patch.
