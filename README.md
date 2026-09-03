# CoreControl v10.10 — Visão Geral / Central de Operação

Patch cumulativo de interface + backend para aplicar por cima da v10.9.6.

## O que mudou

A Visão Geral para administradores de empresa deixou de ser um resumo genérico de quantidade de empresas e virou uma Central de Operação.

- cabeçalho com situação geral da empresa;
- computadores online, saúde média, computadores que precisam de atenção, perfis otimizados e atualizações pendentes;
- painel "Computadores agora" com aplicativo em foco, CPU, RAM, disco, GPU, temperatura, saúde, perfil e versão do Agent;
- atalhos para Ver atividade, Acessar e Otimizar;
- painel "Precisa da sua atenção" com problemas acionáveis;
- "Em foco agora" para acompanhar a atividade atual da equipe sem abrir computador por computador;
- resumo das últimas 24 horas com otimizações, diagnósticos, acessos remotos e limpezas seguras;
- visão simplificada de proteção, armazenamento, temperatura e uptime;
- histórico dos últimos acontecimentos reais usando os logs administrativos do CoreControl;
- visão antiga da plataforma preservada para Administrador Global.

## Arquivos alterados

- `app/api.py`
- `app/static/index.html`
- `app/static/styles.css`
- `app/static/js/pages/overview.js`

## Implantação

1. Aplicar este patch por cima da v10.9.6.
2. Fazer Force Rebuild do CoreControl.
3. Atualizar o navegador com Ctrl+F5.

Não é necessário reinstalar o Agent nos computadores. Esta versão não altera o Agent Windows.
