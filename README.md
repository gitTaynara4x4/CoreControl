# CoreControl v10.11 — Ligar / Desligar computador

Patch incremental para aplicar por cima da v10.10.1 (e das evoluções v10.9.x já aplicadas).

## O que muda

- PC ligado: botão **Desligar computador** na Visão Geral e na página do computador.
- PC desligado/offline no MeshCentral: botão **Ligar computador**.
- O desligamento pede confirmação antes de enviar o comando.
- O painel acompanha o estado remoto por até ~90 segundos e atualiza quando o PC liga/desliga.
- As ações ficam registradas no histórico (`power.off.sent` / `power.wake.sent`).
- O estado de energia usa o MeshCentral quando disponível, evitando confundir "Agent CoreControl sem telemetria" com "PC fisicamente desligado".
- Não altera o CoreControl Agent Windows: **não precisa reinstalar a Luiza**.

## Como funciona

O CoreControl usa `MeshCtrl DevicePower --off` para desligar e `MeshCtrl DevicePower --wake` para Wake-on-LAN.

### Importante sobre ligar um PC totalmente desligado

Wake-on-LAN depende de suporte/configuração do PC e da rede. Em uma instalação MeshCentral por Agent, normalmente é necessário existir outro Mesh Agent online na mesma rede/sub-rede (e no mesmo grupo) para retransmitir o pacote de wake, ou então o equipamento precisa oferecer um caminho fora de banda como Intel AMT.

Se não houver relay disponível, o CoreControl ainda envia o pedido e acompanha por ~90 segundos; se o computador não voltar, o painel informa que o wake não foi confirmado.

## Aplicação

Substitua somente os arquivos deste ZIP no projeto atual e faça Force Rebuild do CoreControl. Depois use Ctrl+F5 no navegador.

## Validação feita

- `python -m py_compile app/api.py app/meshcentral.py`
- `node --check` em `ui.js`, `overview.js` e `devices.js`
- `pytest -q tests/test_power_control.py` → 3 testes aprovados
- rota `/api/devices/{device_id}/power` carregada como POST em FastAPI com banco SQLite de teste
