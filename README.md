# CoreControl — atualização automática de aplicativos (v10.2)

Esta correção NÃO altera o Agent. A Luiza pode continuar no Agent 0.8.9.

O que foi corrigido:
- ao entrar no computador, a última lista salva aparece imediatamente;
- uma nova coleta é solicitada silenciosamente em segundo plano;
- enquanto a tela daquele computador estiver aberta, o painel solicita novas coletas automaticamente;
- intervalo de atualização: aproximadamente 5 segundos + tempo de resposta do Agent;
- o Agent 0.8.9 já consulta comandos rapidamente, então não é necessário reinstalar;
- durante uma nova coleta, a última lista válida continua visível (não some nem volta para "aguardando");
- ao sair da página do computador, a atualização automática para sozinha;
- mantém agrupamento/seta, abas reais, favicons e demais evoluções do devices.js atual.

Resultado esperado:
se a Luiza abrir/fechar um aplicativo ou trocar/abrir uma aba, o painel deve refletir a mudança
automaticamente em poucos segundos, sem precisar clicar em "Atualizar aplicativos".
