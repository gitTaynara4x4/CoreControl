# CoreControl — atividade com todas as janelas/pastas

Correção para estações com muitas janelas e pastas abertas.

## O que mudou
- Agent 0.8.4.
- A coleta normal via EnumWindows continua ativa.
- No Windows 11, o Agent também consulta as abas do Explorador de Arquivos via UI Automation, porque várias pastas podem existir dentro de uma única janela do Explorer.
- O limite técnico do snapshot passou de 24 para 200 janelas.
- O painel deixou de cortar a lista em 14 linhas.
- A lista de aplicativos agora tem rolagem interna e cabeçalho fixo.
- O painel mostra a quantidade de janelas retornadas.
- O contador de ícones agora considera cada linha/janela, reutilizando corretamente o ícone do mesmo processo.

## Aplicação
Substitua somente os arquivos do ZIP no projeto/servidor. Depois reinicie o CoreControl e use o fluxo "Reinstalar / atualizar CoreControl" no computador da Luiza para atualizar o Agent para 0.8.4. Em seguida clique em "Atualizar aplicativos".
