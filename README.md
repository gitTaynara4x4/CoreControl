# CoreControl — correção da seta expansível (v8.1)

A v8 anterior atualizou o HTML/CSS, mas o arquivo `app/static/js/pages/devices.js` não foi incluído no ZIP. Por isso a mensagem "Clique na seta..." aparecia, porém nenhuma seta era renderizada.

Esta correção inclui o JavaScript real do agrupamento:
- seta `>` à esquerda dos aplicativos que possuem abas/janelas filhas;
- Chrome/Edge/Opera/Brave mostram a quantidade de abas quando `browser_tabs` chega do Agent;
- aplicativos com várias janelas também podem ser expandidos;
- estado expandido permanece entre atualizações da tabela;
- nenhuma alteração no Agent: se a Luiza já estiver no Agent 0.8.6, não precisa reinstalar.

Depois de substituir os arquivos na Central/VPS, reinicie a Central e faça Ctrl+F5.
