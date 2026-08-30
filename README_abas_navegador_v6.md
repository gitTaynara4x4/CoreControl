# CoreControl — abas reais do navegador (v6)

Agent: **0.8.5**

## O que mudou

- Chrome, Edge e Opera passam a ter as abas abertas detectadas diretamente pelo Windows UI Automation.
- A extensão/Browser Bridge continua sendo usada quando estiver instalada, pois ela também consegue fornecer domínio/URL.
- Quando a extensão não existe, o CoreControl usa o fallback nativo do Windows e mostra pelo menos o título real de cada aba aberta.
- Abas duplicadas com o mesmo título não são removidas.
- A aba realmente em uso é marcada como `Em uso` quando o Windows permite identificar a seleção na janela em primeiro plano.
- O painel ganhou a seção `Abas abertas do navegador`, contador total e rolagem própria para listas grandes.
- Ao atualizar para o Agent 0.8.5, o painel força uma nova coleta se detectar Chrome/Edge/Opera aberto e ainda estiver vendo um snapshot antigo sem abas.

## Arquivos alterados

- `agent/src/browser_tabs_windows.go`
- `agent/src/main.go`
- `app/downloads/CoreControlAgent.exe`
- `app/downloads/CoreTunerAgent.exe`
- `app/static/js/pages/devices.js`
- `app/static/styles.css`
- `app/static/index.html`
- `tests/test_activity_icons.py`
- `tests/test_browser_tabs_uia.py`

## Aplicação

Substitua os arquivos mantendo as mesmas pastas. Reinicie a Central.

Como a coleta de abas é feita no computador monitorado, use `Reinstalar / atualizar CoreControl` no computador da Luiza para atualizar o Agent para **0.8.5**. Depois clique em `Atualizar aplicativos`.

Não é necessário instalar manualmente a extensão do Chrome para o fallback novo funcionar.

## Validação executada

- build Windows amd64 do Agent: OK
- `go vet`: OK
- sintaxe de `devices.js` com Node: OK
- testes específicos: 10 aprovados
