# Banco PostgreSQL

1. Abra o console SQL do banco no EasyPanel.
2. Execute `postgresql_schema.sql` uma vez.
3. Configure `CORETUNER_DATABASE_URL` nas variáveis protegidas do serviço.

Não coloque a URL real do banco em arquivos enviados ao GitHub. O backend é o único componente autorizado a acessar o PostgreSQL. O site, o CoreTunerSetup e o Agent conversam apenas com a API HTTPS.
