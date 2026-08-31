# CoreControl v10.1 — correção da regressão sem perder evoluções

Este pacote corrige o erro da v10 anterior.

A v10 anterior foi montada sobre uma base antiga do Agent 0.8.6/0.8.7 e, ao substituir
`main.go` e `browser_tabs_windows.go`, acabou removendo melhorias mais novas que já estavam
no Agent 0.8.8.

Esta v10.1 parte da versão mais nova (0.8.8) e aplica SOMENTE o filtro de abas internas
de sites, gerando o Agent 0.8.9.

Preservado:
- polling rápido de comandos a cada 5 segundos;
- atualização de aplicativos sem esperar o ciclo normal de telemetria;
- compatibilidade UI Automation + MSAA;
- fallback de detecção de abas;
- Browser Bridge;
- ícones reais dos aplicativos;
- abas reais do navegador;
- todas as melhorias do Agent 0.8.8.

Corrigido:
- widgets internos de páginas como Disney+ "SUGESTÕES", "DETALHES", "EXTRAS" e "VERSÕES"
  não devem mais ser tratados como abas do Chrome.

Este ZIP não contém HTML, CSS ou JavaScript e portanto não sobrescreve as melhorias visuais
(seta expansível, favicons e CSS clean) já aplicadas na Central.

Depois de substituir os arquivos, a Luiza precisa atualizar/reinstalar o Agent uma vez para 0.8.9.
