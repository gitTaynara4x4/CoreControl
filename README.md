# CoreControl v10.6 - aviso remoto discreto

Este patch e cumulativo sobre a v10.5 e **nao remove o aviso de acesso remoto**.

Objetivo:
- manter o acesso remoto exatamente como esta funcionando;
- remover a barra azul grande e persistente do MeshCentral;
- manter uma notificacao pequena no Windows quando a sessao remota comeca;
- manter o vinculo exato do computador e o auto-connect da v10.5.

## Mudanca de consentimento

No MeshCentral, os bits relevantes sao:
- `1` = notificar o usuario ao iniciar Desktop remoto;
- `64` = mostrar a barra de privacidade persistente.

O projeto estava usando `65` (`1 + 64`). Para o modo discreto use:

```env
CORETUNER_REMOTE_GROUP_CONSENT=1
```

A v10.6 tambem corrige uma limitacao anterior: agora grupos de empresas ja existentes sao atualizados com o consentimento configurado, em vez de aplicar o valor somente quando o grupo e criado.

## Depois do deploy

1. No EasyPanel / CoreControl, altere `CORETUNER_REMOTE_GROUP_CONSENT=1`.
2. Force Rebuild do CoreControl.
3. No terminal do CoreControl execute:

```bash
python -m tools.aplicar_aviso_remoto_discreto
```

4. Opcionalmente personalize a notificacao do MeshCentral no `config.json` para:

`CoreControl: acesso remoto ativo.`

Nao e necessario reinstalar o Agent no computador da Luiza.
