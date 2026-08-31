# CoreControl — grupos expansíveis estilo Gerenciador de Tarefas (v8)

Esta atualização é **somente do painel web**. O Agent continua na versão 0.8.6 e **não precisa ser reinstalado novamente no computador da Luiza** se ele já estiver nessa versão.

## O que mudou

- Google Chrome, Edge, Opera e Brave passam a aparecer como uma linha principal agrupada.
- Quando o Agent devolve abas, a linha mostra a quantidade: `Google Chrome (14)`.
- Uma seta à esquerda permite expandir/recolher as abas, no estilo do Gerenciador de Tarefas do Windows.
- Cada aba aparece como linha filha com título, domínio/site e status `Em uso` ou `Aba aberta`.
- Aplicativos com várias janelas também são agrupados.
- O Explorador de Arquivos pode ser expandido para mostrar as pastas/abas detectadas.
- O estado expandido permanece enquanto a tela é atualizada, evitando fechar a árvore após uma nova renderização.
- A tabela mantém rolagem interna para listas grandes.

## Importante

Para o Chrome realmente mostrar todas as abas, o computador monitorado precisa estar com o Agent **0.8.6** (correção v7). Esta v8 não altera o Agent; muda apenas a forma como os dados já coletados são exibidos.

Depois de substituir os arquivos, reinicie/republique a Central e atualize a página com `Ctrl+F5`.
