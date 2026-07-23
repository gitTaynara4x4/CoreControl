# Deploy no EasyPanel

## 1. Banco

No console SQL do PostgreSQL, execute:

```text
database/postgresql_schema.sql
```

## 2. Serviço do CoreTuner

Crie um serviço a partir do repositório GitHub e use o Dockerfile da raiz.

Porta interna:

```text
8280
```

Variáveis obrigatórias:

```env
CORETUNER_ENV=production
CORETUNER_PORT=8280
CORETUNER_SECRET_KEY=UMA_CHAVE_LONGA_E_ALEATORIA
CORETUNER_ADMIN_EMAIL=admin@seudominio.com.br
CORETUNER_ADMIN_PASSWORD=UMA_SENHA_FORTE
CORETUNER_DOWNLOAD_PASSWORD=0610
CORETUNER_DATABASE_URL=postgresql+psycopg://USUARIO:SENHA@HOST:PORTA/BANCO?sslmode=disable
CORETUNER_PUBLIC_URL=https://coretuner.seudominio.com.br
```

## 3. Domínio e HTTPS

Configure o domínio no EasyPanel e confirme que `/health` retorna `status: ok`.

O Agent exige HTTPS em produção. HTTP só é permitido para localhost ou quando o usuário marca explicitamente um ambiente de teste.

## 4. Executável

O arquivo servido pelo site fica em:

```text
app/downloads/CoreTunerSetup.exe
```

A versão incluída no projeto inicia com `http://127.0.0.1:8002` como servidor padrão, mas o endereço pode ser alterado na tela do aplicativo. Para gerar uma versão já apontada ao domínio, execute `desktop/Build_Windows.ps1` com `CORETUNER_PUBLIC_URL` configurada.
