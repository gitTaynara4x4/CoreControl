# Deploy no EasyPanel

## Serviço principal `coretuner`

Use o Dockerfile da raiz e configure:

- porta interna: `8280`;
- healthcheck: `/health`;
- domínio público: `https://apps-coretuner.9ywrah.easypanel.host`;
- volume persistente em `/data`.

Copie os nomes de `.env.production.example` para as variáveis protegidas do serviço. Não envie um `.env` preenchido ao GitHub.

## Banco

A aplicação cria tabelas ausentes automaticamente. Para revisão ou criação manual, use:

```text
database/postgresql_schema.sql
```

## Acesso remoto

O serviço `coretuner-remote` é separado. No serviço principal, mantenha todas as variáveis `CORETUNER_REMOTE_*` da documentação `docs/ACESSO_REMOTO.md`.

A chave correta pode ser validada dentro do contêiner principal com o MeshCtrl. Depois de validada, não deve ser alterada sem também gerar e atualizar uma nova chave no MeshCentral.

## Executáveis

O Setup e o aplicativo deste pacote já foram compilados com:

```text
https://apps-coretuner.9ywrah.easypanel.host
```

Após o deploy, apague o instalador antigo do Windows e baixe novamente pelo site.
