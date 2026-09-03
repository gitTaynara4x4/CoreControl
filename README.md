# CoreControl v10.12 — Safe Wake Relay

Esta versão corrige o ponto mais perigoso do controle de energia: o CoreControl não considera mais que “o comando Wake foi enviado” significa que o computador conseguirá ligar.

## O que muda

- O CoreControl Agent passa para a versão **0.9.4**.
- O Agent informa o **MAC** e a **sub-rede (CIDR)** da conexão local.
- Todo Agent 0.9.4 online pode atuar automaticamente como **Wake Relay** para outro computador da mesma rede.
- Ao ligar um PC, o servidor tenta o Wake Relay local e mantém o MeshCentral como fallback.
- O Wake Relay envia o magic packet localmente nas portas UDP 9 e 7, em mais de uma tentativa.
- Antes de desligar um computador, o CoreControl verifica novamente se existe uma rota de wake verificada.
- Por padrão, se não existir outro Relay na mesma LAN, o **desligamento total é bloqueado**. Isso evita desligar uma máquina e depois descobrir que não existe caminho para ligá-la remotamente.
- A Visão Geral e a página do computador passam a usar o estado `power.safe_to_power_off` / `power.wake_verified`.

## Caso da Luiza

Se a Luiza continuar sendo o **único computador/Agent online da rede da Rosiane**, não existe software no servidor que consiga criar sozinho um pacote Wake-on-LAN dentro daquela LAN depois que a própria Luiza estiver desligada. Nesse cenário o CoreControl v10.12 **não deixa desligar a Luiza por engano**.

Para o desligar → ligar ficar realmente utilizável, é necessário pelo menos um destes caminhos:

1. outro CoreControl Agent 0.9.4 sempre ligado na mesma rede (Wake Relay automático);
2. roteador/gateway com Wake-on-LAN remoto configurável;
3. Intel AMT/out-of-band configurado no equipamento.

Além disso, Wake-on-LAN precisa estar habilitado na BIOS/UEFI e na placa de rede do PC alvo.

## Atualização do Agent

Como esta versão adiciona o Wake Relay ao **CoreControlAgent.exe**, os computadores que participarão do wake precisam receber o Agent 0.9.4. Não é necessário desinstalar manualmente: execute novamente o **CoreControl Setup** enquanto a máquina estiver ligada para substituir o Agent instalado.

O `app/downloads/CoreControlAgent.exe` deste pacote já foi recompilado para Windows x64 e contém a versão 0.9.4.

## Configuração de segurança

Nova variável:

```env
CORETUNER_POWER_REQUIRE_VERIFIED_WAKE=true
```

Mantenha `true` em produção. Se for alterada para `false`, o CoreControl volta a permitir desligamento mesmo sem um Wake Relay verificado.

## Validação desta alteração

- `python -m py_compile app/api.py app/config.py`
- `node --check` em `ui.js`, `overview.js` e `devices.js`
- `pytest -q tests/test_power_control.py` → **5 aprovados**
- `go test ./...` em `agent/src` → **aprovado**
- build Windows x64 do `CoreControlAgent.exe` → **aprovado**
- seleção de relay testada com banco SQLite: somente Agent online na mesma CIDR foi selecionado.

A suíte completa do ZIP já possuía 26 testes antigos falhando antes desta alteração; a comparação com o ZIP original mostrou os mesmos 26 failures. Esta correção não acrescentou regressões nessa suíte.
