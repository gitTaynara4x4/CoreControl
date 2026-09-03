# CoreControl v10.11 — Energia remota

Patch somente com arquivos alterados, para aplicar por cima da v10.10.1.

## Novo
- PC ligado: botão **Desligar computador**.
- PC offline: botão **Ligar computador**.
- Disponível na Visão Geral e na página do computador.
- Desligamento exige confirmação no painel.
- Liga/desliga usa o MeshCentral já instalado; não exige novo Agent CoreControl.
- Wake usa o `DevicePower --wake` do MeshCentral e aguarda até 90 segundos pelo retorno do computador.
- Ações ficam registradas no histórico administrativo.

## Importante sobre Ligar computador
Wake-on-LAN depende do hardware/rede. O PC precisa aceitar WOL na BIOS/NIC. Em redes remotas, o MeshCentral normalmente usa outro agente online no mesmo grupo/sub-rede para retransmitir o pacote. Se não existir outro computador online na rede e não houver Intel AMT/roteador com WOL, nenhum software hospedado na nuvem consegue ligar fisicamente um PC totalmente desligado.

## Arquivos alterados
- app/api.py
- app/static/index.html
- app/static/js/pages/overview.js
- app/static/js/pages/devices.js

Não é necessário reinstalar o CoreControl Agent da Luiza.
