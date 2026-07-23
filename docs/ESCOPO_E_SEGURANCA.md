# CoreTuner Central 0.1.0 — escopo e segurança

## O que funciona nesta entrega

- Cadastro de empresas com isolamento lógico por `company_id`.
- Cadastro de usuários e perfis: administrador da plataforma, administrador da empresa, técnico e visualização.
- Token de instalação de agente com validade de 24 horas e uso único.
- Agente Windows x64 iniciando com o computador por uma tarefa agendada do sistema.
- Estado online/offline, identificação, CPU, RAM, disco, rede, uptime, Defender, Firewall e temperatura quando disponível.
- Histórico de telemetria, nota de saúde e alertas automáticos.
- Reconhecimento de alertas e trilha de auditoria de ações administrativas.

## O que deliberadamente NÃO existe ainda

- Controle remoto de tela.
- Terminal remoto.
- Transferência de arquivos.
- Execução remota de scripts.
- Instalação de atualizações.
- Descoberta ativa de rede.
- Alteração do Windows pelo agente.

Essas funções só devem ser adicionadas depois de HTTPS, autenticação em dois fatores,
permissões detalhadas, assinatura de comandos e auditoria estarem em produção.

## Regras obrigatórias para produção

1. Publicar exclusivamente por HTTPS com certificado válido.
2. Alterar a chave secreta e a senha administrativa inicial.
3. Restringir o painel por firewall/VPN quando possível.
4. Fazer backup diário do banco e testar a restauração.
5. Não usar `allow_insecure_http=true` em computadores de clientes.
6. Revisar LGPD, contrato e autorização expressa para coleta e acesso remoto futuro.
7. Migrar SQLite para PostgreSQL antes de uma operação com muitos agentes simultâneos.
