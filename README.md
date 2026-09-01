# CoreControl — correção definitiva do acesso remoto na instalação por código (v10.3)

Este patch corrige o fluxo em que o computador era cadastrado na empresa correta pelo código de instalação, mas o Mesh Agent antigo continuava vinculado a outro grupo/empresa no MeshCentral.

## O que mudou

- `/api/agent/enroll` agora prepara automaticamente o grupo remoto da empresa que emitiu o código.
- O Setup recebe o agente MeshCentral específico daquela empresa.
- O download do agente remoto usa a credencial exclusiva do próprio CoreControlAgent recém-cadastrado; nenhum login administrativo é exposto ao funcionário.
- Se já existir Mesh Agent de outra empresa/servidor, o Setup detecta, remove e instala o agente correto.
- O Setup confirma o vínculo remoto pelo endpoint autenticado `/api/agent/remote-status` antes de informar sucesso do acesso remoto.
- O dispositivo existente do CoreControl é reaproveitado pelo `device_uid`; não é necessário excluir manualmente o computador do painel.

## Arquivos alterados

- `app/api.py`
- `desktop/setup/src/main.go`
- `app/downloads/CoreControlSetup.exe`
- `app/downloads/CoreTunerSetup.exe` (alias legado)

## Aplicação

1. Sobrescreva estes arquivos no projeto atual.
2. Faça rebuild/redeploy do serviço **CoreControl** no EasyPanel.
3. Não altere o serviço `coretuner-remote`.
4. Depois do deploy, gere um **novo código de instalação da Rosiane**.
5. Execute o novo `CoreControlSetup` no PC da Luiza usando esse código.
6. Não apague manualmente o CoreControl nem o Mesh Agent antes: o instalador atualizado faz a substituição do vínculo remoto antigo.

## Resultado esperado no caso da Luiza

Antes:
- CoreControl: Rosiane Restaurante Industrial
- MeshCentral: TaynaraSolution

Depois:
- CoreControl: Rosiane Restaurante Industrial
- MeshCentral: CoreTuner - Rosiane Restaurante Industrial
- `mesh_node_id`: preenchido
- `remote_online`: `true`

## Validação realizada

- Python `py_compile`: OK
- Fluxo/API: 11 testes aprovados
- Fluxo + testes do Setup/privacidade: 17 testes aprovados
- Go `gofmt`: OK
- Go `vet` para Windows: OK
- Build Windows amd64 do `CoreControlSetup.exe`: OK
- Ícone oficial reaplicado ao executável: OK

Este patch não altera o CoreControlAgent 0.8.9 nem o frontend v10.2, evitando regressão das correções anteriores.
