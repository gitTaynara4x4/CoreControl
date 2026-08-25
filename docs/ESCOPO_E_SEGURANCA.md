# CoreControl — escopo e segurança

## O que funciona nesta entrega

- Cadastro de empresas com isolamento lógico por `company_id`.
- Cadastro de usuários e perfis administrativos/técnicos.
- Token de instalação de agente com validade e uso único.
- Agente Windows com telemetria de identificação, CPU, RAM, disco, rede, uptime, Defender e Firewall.
- Histórico de telemetria, nota de saúde, alertas e trilha de auditoria.
- Acesso remoto integrado quando o MeshCentral estiver configurado.
- Fila autenticada de comandos restritos do agente para **Atualizações**.
- Verificação de Windows Update, drivers e aplicativos via winget quando disponível.
- Instalação das atualizações selecionadas e políticas de janela de manutenção.

## Limites atuais do canal de comandos

O agente **não é um terminal remoto**. O servidor só pode enfileirar tipos de comando conhecidos pelo binário. Nesta versão são aceitos `updates.scan` e `updates.install`.

A operação de Windows Update usa código interno fixo e não recebe scripts do painel. Aplicativos são atualizados pelo identificador que o próprio agente encontrou no winget. Toda solicitação nasce de um usuário autenticado/permissão válida ou de uma política salva e fica registrada na auditoria.

O CoreControl **não reinicia o Windows automaticamente** depois de instalar uma atualização; apenas sinaliza quando o reinício é necessário.

## O que deliberadamente NÃO existe ainda

- Terminal remoto genérico.
- Transferência de arquivos pelo agente CoreControl.
- Execução remota de scripts arbitrários.
- Descoberta ativa de rede.

Esses recursos exigem controles adicionais próprios antes de serem habilitados.

## Regras obrigatórias para produção

1. Publicar exclusivamente por HTTPS com certificado válido.
2. Alterar chaves e credenciais administrativas iniciais.
3. Restringir o painel por firewall/VPN quando fizer sentido para a operação.
4. Fazer backup diário do banco e testar a restauração.
5. Não usar `allow_insecure_http=true` em computadores de clientes.
6. Manter autorização contratual/LGPD adequada para telemetria, acesso remoto e manutenção.
7. Migrar SQLite para PostgreSQL antes de uma operação com muitos agentes simultâneos.
