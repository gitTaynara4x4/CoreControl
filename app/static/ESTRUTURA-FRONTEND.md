# Estrutura do frontend do CoreControl

O painel continua sendo uma aplicação de página única, mas cada tela agora possui seu próprio HTML e seu próprio JavaScript. As APIs, rotas, autenticação, acesso remoto V7 e banco de dados não foram alterados.

## Onde editar cada tela

| Tela | HTML | JavaScript |
|---|---|---|
| Login e criação de empresa | `pages/login.html` | `js/auth.js` |
| Visão geral | `pages/overview.html` | `js/pages/overview.js` |
| Empresas | `pages/companies.html` | `js/pages/companies.js` |
| Detalhes da empresa | `pages/company.html` | `js/pages/companies.js` |
| Computadores | `pages/devices.html` | `js/pages/devices.js` |
| Detalhes do computador | `pages/device.html` | `js/pages/devices.js` |
| Alertas | `pages/alerts.html` | `js/pages/alerts.js` |
| Acesso remoto | `pages/remote.html` | `js/pages/remote.js` |
| Usuários | `pages/users.html` | `js/pages/users.js` |

## Arquivos compartilhados

- `index.html`: sidebar, topo e áreas principais do painel.
- `js/core.js`: API, estado, formatação e carregamento dos HTMLs.
- `js/router.js`: navegação, atualização automática e títulos.
- `js/ui.js`: tabelas, cards, gráfico e visualizador remoto.
- `js/modals.js`: cadastro de empresa, usuário e token de instalação.
- `components/remote-viewer.html`: janela integrada do acesso remoto.
- `components/modals/`: conteúdo visual dos modais.
- `styles.css`: estilos atuais, preservados para não mudar o design.

## Regra importante

Os IDs usados pelo JavaScript devem continuar iguais. Exemplo: a busca de computadores usa `deviceSearch` e a área da tabela usa `deviceTableArea`. É possível alterar textos, posições e classes sem problema, mas ao renomear um ID também é necessário atualizar o JavaScript da mesma tela.
