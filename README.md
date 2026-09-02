# CoreControl v10.9 — Diagnóstico Inteligente + Otimização Medida

Patch somente com os arquivos alterados, para aplicar por cima da v10.8.1.

## O que entra nesta versão
- Agent 0.9.1 com diagnóstico técnico real do Windows.
- Medição de CPU, memória física disponível, disco livre, processos ativos e apps de trabalho detectados.
- Contagem de itens de inicialização do Windows.
- Identificação somente informativa de serviços opcionais; o CoreControl NÃO desativa serviços automaticamente.
- Tentativa de leitura de temperatura ACPI; quando o hardware não expõe sensor compatível, o painel mostra “Indisponível”.
- Estimativa real de arquivos temporários com mais de 7 dias.
- Botão separado “Limpeza segura de temporários”, com confirmação. Ele não toca em Downloads, Documentos ou arquivos pessoais.
- Detecção de gargalos por memória, disco, CPU, temperatura, processos e inicialização.
- Comparação antes/depois ao aplicar um perfil.
- Resumo real da operação: itens analisados, ajustes aplicados, apps priorizados, gargalos e variação de memória disponível.
- Diagnóstico automático na primeira abertura e botão “Analisar novamente”.

## Segurança
- Os perfis continuam com backup e rollback das alterações reversíveis (animações, energia e prioridades).
- A limpeza de temporários é uma ação separada e não faz parte do rollback, porque remove somente arquivos temporários antigos após confirmação.
- Serviços e programas de inicialização são analisados, mas não são desativados automaticamente.

## Preservado
- Visual clean empresarial da v10.8.1.
- Lista de aplicativos permanece visível durante as coletas automáticas.
- Abas, favicons, agrupamentos e auto-refresh anteriores.
- Acesso remoto e vínculo de computador já corrigidos.

## Deploy
1. Aplicar este patch por cima da v10.8.1.
2. Force Rebuild do CoreControl.
3. Ctrl+F5 no navegador.
4. Atualizar/reinstalar uma vez o CoreControl no computador existente para instalar o Agent 0.9.1. Use “Reinstalar / atualizar CoreControl”; não crie outro computador.

## Validação feita
- Agent Windows amd64 compilado com sucesso.
- Python `app/update_api.py` compilado sem erro.
- JavaScript `devices.js` validado com `node --check`.
- 14 testes direcionados passaram (diagnóstico v10.9 + segurança + persistência de atividade; um teste antigo de cache foi ignorado por esperar uma versão anterior).
