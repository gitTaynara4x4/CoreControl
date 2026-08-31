# CoreControl — abas reais do navegador sem elementos internos da página (Agent 0.8.7)

Corrige o caso em que páginas como Disney+ expõem componentes internos com ARIA/UI Automation
do tipo `TabItem` e o Agent os confundia com abas reais do Chrome.

O Agent agora:
- rejeita `TabItem` que esteja dentro do `Document`/área renderizada da página;
- rejeita elementos descendentes de `Chrome_RenderWidgetHostHWND`;
- usa uma segunda proteção por posição, limitada à faixa superior do navegador;
- continua preservando abas reais repetidas;
- não usa blacklist por texto ("SUGESTÕES", "DETALHES" etc.), então a correção vale para
  Disney+, CRMs, dashboards e outros sites que possuam abas internas.

Versão do Agent: 0.8.7

Como mudou o Agent, o computador monitorado precisa receber a atualização uma vez pelo fluxo
"Reinstalar / atualizar CoreControl". O cadastro, histórico e vínculo do computador são mantidos.
