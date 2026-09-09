(function () {
  'use strict';

  const CT = window.CoreTuner;

  const icons = {
    terminal: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m5 7 4 4-4 4M11 17h7"/></svg>',
    activity: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 12h4l2.2-6 4.2 12 2.3-6H21"/></svg>',
    play: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 7 8 5-8 5Z"/></svg>',
    alert: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 9v4m0 4h.01M10.3 4.2 2.9 17a2 2 0 0 0 1.7 3h14.8a2 2 0 0 0 1.7-3L13.7 4.2a2 2 0 0 0-3.4 0Z"/></svg>',
    diagnose: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="6"/><path d="m16 16 4 4M8 11h6M11 8v6"/></svg>',
    maintenance: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M14.5 6.5a4 4 0 0 0-5-5L7 4l3 3 2.5-2.5a4 4 0 0 0 2 2ZM9.5 8.5 3 15a2.1 2.1 0 0 0 3 3l6.5-6.5"/></svg>',
    network: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="9" y="3" width="6" height="5" rx="1"/><rect x="3" y="16" width="6" height="5" rx="1"/><rect x="15" y="16" width="6" height="5" rx="1"/><path d="M12 8v4M6 16v-4h12v4"/></svg>',
    windows: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 5.5 10.5 4v7H3Zm9-1.8L21 2v9h-9ZM3 12.5h7.5v7L3 18Zm9 0h9v9l-9-1.7Z"/></svg>',
    apps: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>',
    custom: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m8 9-4 3 4 3M16 9l4 3-4 3M14 5l-4 14"/></svg>',
    search: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="6"/><path d="m16 16 4 4"/></svg>',
    history: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5M12 7v5l3 2"/></svg>',
    calendar: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="5" width="18" height="16" rx="2"/><path d="M7 3v4M17 3v4M3 10h18"/></svg>',
    shield: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 5 6v5c0 4.5 2.6 7.7 7 10 4.4-2.3 7-5.5 7-10V6Z"/><path d="m9 12 2 2 4-4"/></svg>',
  };

  const categories = [
    { key: 'diagnostic', name: 'Diagnóstico', description: 'Coletas técnicas e verificações do Windows.', icon: 'diagnose' },
    { key: 'maintenance', name: 'Manutenção', description: 'Rotinas seguras e manutenção assistida.', icon: 'maintenance' },
    { key: 'network', name: 'Rede', description: 'Testes de conectividade e configuração.', icon: 'network' },
    { key: 'windows', name: 'Windows', description: 'Ações administrativas no sistema operacional.', icon: 'windows' },
    { key: 'apps', name: 'Aplicativos', description: 'Inventário e automações de software.', icon: 'apps' },
    { key: 'custom', name: 'Personalizado', description: 'Scripts criados pela sua equipe.', icon: 'custom' },
  ];

  function kpi(label, value, detail, icon, tone = '') {
    return `<article class="scripts-kpi ${tone}"><span class="scripts-kpi-icon">${icons[icon]}</span><div class="scripts-kpi-copy"><span>${label}</span><strong>${value}</strong><small>${detail}</small></div></article>`;
  }

  function library() {
    return `
      <section class="scripts-status-card">
        <div class="scripts-status-main">
          <span class="scripts-status-icon">${icons.shield}</span>
          <div>
            <span class="scripts-eyebrow">AUTOMAÇÃO CONTROLADA</span>
            <h2>Biblioteca de scripts da empresa</h2>
            <p>Centralize rotinas administrativas e mantenha execução, destino e resultado rastreáveis em um único lugar.</p>
          </div>
        </div>
        <div class="scripts-status-meta"><span><b>0</b> scripts</span><span><b>0</b> em execução</span></div>
      </section>

      <div class="scripts-kpi-grid">
        ${kpi('Scripts disponíveis', '0', 'Biblioteca da empresa', 'terminal')}
        ${kpi('Execuções hoje', '0', 'Nenhuma execução registrada', 'activity', 'good')}
        ${kpi('Em execução', '0', 'Fila atual', 'play', 'attention')}
        ${kpi('Falhas', '0', 'Últimas 24 horas', 'alert', 'critical')}
      </div>

      <section class="scripts-section-block">
        <div class="scripts-section-heading">
          <div><h2>Categorias</h2><p>Organize as automações por finalidade para encontrar a rotina certa rapidamente.</p></div>
        </div>
        <div class="scripts-category-grid">
          ${categories.map((category) => `
            <button class="scripts-category-card" type="button" data-script-category="${category.key}">
              <span class="scripts-category-icon">${icons[category.icon]}</span>
              <span class="scripts-category-copy"><strong>${category.name}</strong><small>${category.description}</small></span>
              <span class="scripts-category-count">0</span>
            </button>`).join('')}
        </div>
      </section>

      <section class="card scripts-table-card">
        <div class="scripts-card-header">
          <div><span class="scripts-eyebrow">BIBLIOTECA</span><h2>Todos os scripts</h2><p>PowerShell e CMD com parâmetros, timeout e histórico de execução.</p></div>
          <span class="scripts-count-badge">0 scripts</span>
        </div>
        <div class="scripts-filterbar">
          <label class="scripts-search-field">
            <span>${icons.search}</span>
            <input id="scriptSearch" type="search" placeholder="Pesquisar script..." autocomplete="off">
          </label>
          <select id="scriptCategoryFilter" class="scripts-filter-select" aria-label="Filtrar por categoria">
            <option value="">Todas as categorias</option>
            ${categories.map((category) => `<option value="${category.key}">${category.name}</option>`).join('')}
          </select>
        </div>
        <div class="scripts-empty-state">
          <span class="scripts-empty-icon">${icons.terminal}</span>
          <strong>Nenhum script cadastrado</strong>
          <p>Crie a primeira rotina para começar a montar a biblioteca da empresa.</p>
          <button class="btn primary" type="button" data-placeholder-action="Novo script">Novo script</button>
        </div>
      </section>`;
  }

  function executions() {
    return `
      <div class="scripts-kpi-grid scripts-kpi-grid-3">
        ${kpi('Execuções hoje', '0', 'Total iniciado hoje', 'history')}
        ${kpi('Em execução', '0', 'Comandos em andamento', 'play', 'attention')}
        ${kpi('Falhas', '0', 'Últimas 24 horas', 'alert', 'critical')}
      </div>
      <section class="card scripts-table-card">
        <div class="scripts-card-header">
          <div><span class="scripts-eyebrow">RASTREABILIDADE</span><h2>Histórico de execuções</h2><p>Veja quem executou, em qual computador, duração e resultado.</p></div>
          <span class="scripts-count-badge">0 registros</span>
        </div>
        <div class="table-wrap scripts-table-wrap">
          <table class="scripts-table">
            <thead><tr><th>Data</th><th>Script</th><th>Computador</th><th>Empresa</th><th>Usuário</th><th>Duração</th><th>Resultado</th></tr></thead>
            <tbody><tr><td colspan="7"><div class="scripts-inline-empty"><span>${icons.history}</span><strong>Nenhuma execução registrada</strong><small>As execuções realizadas pelo CoreControl aparecerão aqui.</small></div></td></tr></tbody>
          </table>
        </div>
      </section>`;
  }

  function schedules() {
    return `
      <section class="scripts-schedule-intro">
        <span class="scripts-schedule-icon">${icons.calendar}</span>
        <div><span class="scripts-eyebrow">AGENDAMENTOS</span><h2>Automatize rotinas recorrentes</h2><p>Defina quando uma rotina deve executar e mantenha o histórico de cada disparo.</p></div>
        <button class="btn primary" type="button" data-placeholder-action="Novo agendamento">Novo agendamento</button>
      </section>
      <div class="scripts-kpi-grid scripts-kpi-grid-3">
        ${kpi('Agendamentos ativos', '0', 'Rotinas habilitadas', 'calendar', 'good')}
        ${kpi('Próximas execuções', '0', 'Nas próximas 24 horas', 'play')}
        ${kpi('Falhas recentes', '0', 'Últimas 24 horas', 'alert', 'critical')}
      </div>
      <section class="card scripts-table-card">
        <div class="scripts-card-header"><div><h2>Rotinas agendadas</h2><p>Execuções únicas ou recorrentes em computadores e empresas.</p></div><span class="scripts-count-badge">0 agendamentos</span></div>
        <div class="scripts-empty-state compact"><span class="scripts-empty-icon">${icons.calendar}</span><strong>Nenhum agendamento criado</strong><p>Os próximos disparos ficarão organizados nesta tela.</p></div>
      </section>`;
  }

  function readTab() {
    const tab = new URL(window.location.href).searchParams.get('tab');
    return ['library', 'executions', 'schedules'].includes(tab) ? tab : 'library';
  }

  function writeTab(tab) {
    const url = new URL(window.location.href);
    url.searchParams.set('page', 'scripts');
    url.searchParams.set('tab', tab);
    window.history.replaceState({ corecontrol: true, page: 'scripts', tab }, '', url);
  }

  CT.registerPage('scripts', async function renderScripts() {
    await CT.mountPage('scripts');
    const page = CT.$('.page-scripts');
    const view = CT.$('#scriptsView');
    let activeTab = readTab();

    const render = (tab, persist = true) => {
      activeTab = ['library', 'executions', 'schedules'].includes(tab) ? tab : 'library';
      CT.$$('[data-script-tab]', page).forEach((button) => {
        const selected = button.dataset.scriptTab === activeTab;
        button.classList.toggle('active', selected);
        button.setAttribute('aria-selected', selected ? 'true' : 'false');
      });
      view.innerHTML = activeTab === 'executions' ? executions() : activeTab === 'schedules' ? schedules() : library();
      if (persist) writeTab(activeTab);
    };

    page.addEventListener('click', (event) => {
      const tab = event.target.closest('[data-script-tab]');
      if (tab) return render(tab.dataset.scriptTab);

      const category = event.target.closest('[data-script-category]');
      if (category) {
        const select = CT.$('#scriptCategoryFilter');
        if (select) select.value = category.dataset.scriptCategory;
        return;
      }

      if (event.target.closest('#scriptRunBtn')) {
        return CT.toast('Crie um script na biblioteca antes de iniciar uma execução.');
      }

      const action = event.target.closest('[data-placeholder-action], #scriptNewBtn');
      if (action) return CT.toast('Editor de scripts ainda não está habilitado nesta versão.');
    });

    render(activeTab, false);
  });
})();
