(function () {
  'use strict';
  const CT = window.CoreTuner;

  const categories = [
    ['Diagnóstico', 'Coletas técnicas e verificações do Windows.'],
    ['Manutenção', 'Rotinas seguras e manutenção assistida.'],
    ['Rede', 'Testes de conectividade e configuração.'],
    ['Windows', 'Ações administrativas no sistema operacional.'],
    ['Aplicativos', 'Inventário e automações de software.'],
    ['Personalizado', 'Scripts criados pela sua equipe.'],
  ];

  function library() {
    return `
      <div class="module-kpis">
        ${CT.stat('Scripts disponíveis', 0, 'Biblioteca da empresa')}
        ${CT.stat('Execuções hoje', 0, 'Nenhuma execução registrada', 'var(--green)')}
        ${CT.stat('Em execução', 0, 'Fila atual', 'var(--amber)')}
        ${CT.stat('Falhas', 0, 'Últimas 24 horas', 'var(--red)')}
      </div>
      <div class="module-feature-grid module-feature-grid-3">${categories.map(([name, description]) => `<article class="module-feature"><div><strong>${name}</strong><p>${description}</p></div><span>0 scripts</span></article>`).join('')}</div>
      <div class="card module-section"><div class="card-header"><div><h2>Biblioteca de scripts</h2><p>PowerShell e CMD com parâmetros, timeout e auditoria.</p></div><button class="btn primary" data-placeholder-action="Novo script">Novo script</button></div><div class="module-empty compact"><strong>Sua biblioteca está vazia</strong><span>O editor será conectado ao executor seguro do agente na etapa funcional.</span></div></div>`;
  }

  function executions() {
    return `<div class="card module-section"><div class="card-header"><div><h2>Histórico de execuções</h2><p>Quem executou, onde, duração, saída e código de retorno.</p></div></div><div class="table-wrap"><table><thead><tr><th>Data</th><th>Script</th><th>Computador</th><th>Empresa</th><th>Usuário</th><th>Duração</th><th>Resultado</th></tr></thead><tbody><tr><td colspan="7" class="table-empty-cell">Nenhuma execução registrada.</td></tr></tbody></table></div></div>`;
  }

  function schedules() {
    return `<div class="card module-section"><div class="card-header"><div><h2>Agendamentos</h2><p>Execuções únicas ou recorrentes em computadores e empresas.</p></div><button class="btn primary" data-placeholder-action="Novo agendamento">Novo agendamento</button></div><div class="module-empty compact"><strong>Nenhum agendamento</strong><span>Quando o executor estiver ativo, os próximos disparos aparecerão aqui.</span></div></div>`;
  }

  CT.registerPage('scripts', async function renderScripts() {
    await CT.mountPage('scripts');
    const page = CT.$('.page-scripts');
    const view = CT.$('#scriptsView');
    const render = (tab) => {
      CT.$$('[data-module-tab]', page).forEach((button) => button.classList.toggle('active', button.dataset.moduleTab === tab));
      view.innerHTML = tab === 'executions' ? executions() : tab === 'schedules' ? schedules() : library();
    };
    page.addEventListener('click', (event) => {
      const tab = event.target.closest('[data-module-tab]');
      if (tab) return render(tab.dataset.moduleTab);
      if (event.target.closest('#scriptRunBtn')) return CT.toast('A seleção de computadores será ligada ao executor de scripts na próxima etapa.');
      const action = event.target.closest('[data-placeholder-action], #scriptNewBtn');
      if (action) CT.toast('Editor de scripts: estrutura pronta para a próxima etapa funcional.');
    });
    render('library');
  });
})();
