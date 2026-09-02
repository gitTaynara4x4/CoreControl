# CoreControl v10.7.2 — atividade sem sumir durante atualização

Correção cumulativa sobre v10.7 + v10.7.1, sem alterar o Agent.

- mantém a última coleta de aplicativos/janelas/abas visível enquanto a próxima coleta fica `queued`/`running`;
- o backend agora devolve explicitamente `cached_command`, usando a última coleta `succeeded`;
- o frontend também mantém uma cópia local da última coleta válida como proteção adicional;
- falha momentânea de polling não apaga a tabela já exibida;
- preserva atualização automática, agrupamento, setas, abas, favicons, CSS v10.7.1 e otimização remota v10.7;
- não exige reinstalar o Agent.
