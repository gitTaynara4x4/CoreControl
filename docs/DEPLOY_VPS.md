# Publicação em VPS com Docker Compose

## Pré-requisitos

- VPS Linux com Docker e Docker Compose.
- Domínio apontando para a VPS.
- Portas 80 e 443 liberadas.

## Passos

1. Envie a pasta do projeto para a VPS.
2. Copie `.env.example` para `.env`.
3. Preencha a chave, o acesso administrativo e a senha privada do download.
4. Execute `docker compose up -d --build`.
5. Publique a porta local 8280 por um proxy HTTPS.

Exemplo de Caddyfile:

```caddy
coretuner.seudominio.com.br {
    reverse_proxy 127.0.0.1:8280
}
```

Para não expor a porta diretamente, altere a publicação do Compose para:

```yaml
ports:
  - "127.0.0.1:8280:8280"
```

## Antes de instalar em clientes

- Confirme certificado HTTPS válido.
- Não publique o arquivo `.env` no GitHub.
- Troque as credenciais administrativas iniciais.
- Confirme o volume persistente em `/data`.
- Faça backup periódico do volume.
- Teste primeiro em computadores próprios.
- Não use `AllowInsecureHttp` fora de laboratório local.
