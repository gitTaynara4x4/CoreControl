(function () {
  'use strict';

  const CT = window.CoreTuner;
  const REPORT_TABS = new Set(['center', 'audit', 'exports']);

  const icons = {
    reports: '<svg viewBox="0 0 24 24"><rect x="4" y="3.5" width="16" height="17" rx="2"/><path d="M8 8h8M8 12h8M8 16h5"/></svg>',
    computers: '<svg viewBox="0 0 24 24"><rect x="3.5" y="4.5" width="17" height="12" rx="2"/><path d="M8 20h8M12 16.5V20"/></svg>',
    monitor: '<svg viewBox="0 0 24 24"><path d="M4 17V7a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v10"/><path d="M8 21h8M12 17v4"/></svg>',
    admin: '<svg viewBox="0 0 24 24"><circle cx="9" cy="8" r="3"/><path d="M3.5 19c.8-3.1 2.7-4.8 5.5-4.8s4.7 1.7 5.5 4.8"/><path d="M17 9v6M14 12h6"/></svg>',
    operations: '<svg viewBox="0 0 24 24"><path d="M4 7h10M4 12h16M4 17h7"/><circle cx="17" cy="7" r="2"/><circle cx="14" cy="17" r="2"/></svg>',
    search: '<svg viewBox="0 0 24 24"><circle cx="10.5" cy="10.5" r="6"/><path d="m15 15 5 5"/></svg>',
    export: '<svg viewBox="0 0 24 24"><path d="M12 3v12M7.5 10.5 12 15l4.5-4.5"/><path d="M5 20h14"/></svg>',
    audit: '<svg viewBox="0 0 24 24"><path d="M12 3 5 6v5c0 4.6 2.7 8 7 10 4.3-2 7-5.4 7-10V6l-7-3Z"/><path d="m9 12 2 2 4-4"/></svg>',
    clock: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/><path d="M12 7.5V12l3 2"/></svg>',
    file: '<svg viewBox="0 0 24 24"><path d="M6 3h8l4 4v14H6z"/><path d="M14 3v5h5M9 13h6M9 17h4"/></svg>',
    lock: '<svg viewBox="0 0 24 24"><rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/></svg>',
  };

  const groups = [
    {
      key: 'computers',
      title: 'Computadores',
      description: 'Inventário, hardware, software e saúde dos equipamentos.',
      icon: 'computers',
      reports: ['Inventário', 'Hardware', 'Software', 'Saúde'],
    },
    {
      key: 'monitoring',
      title: 'Monitoramento',
      description: 'Disponibilidade, alertas e eventos importantes do ambiente.',
      icon: 'monitor',
      reports: ['Disponibilidade', 'Alertas', 'Eventos'],
    },
    {
      key: 'administration',
      title: 'Administração',
      description: 'Empresas, usuários, permissões e acessos administrativos.',
      icon: 'admin',
      reports: ['Empresas', 'Usuários', 'Acessos'],
    },
    {
      key: 'operations',
      title: 'Operações',
      description: 'Acesso remoto, scripts, atualizações e otimizações executadas.',
      icon: 'operations',
      reports: ['Acesso remoto', 'Scripts', 'Atualizações', 'Otimizações'],
    },
  ];

  const allReports = groups.flatMap((group) => group.reports);

  function readTab() {
    const value = new URL(window.location.href).searchParams.get('tab');
    return REPORT_TABS.has(value) ? value : 'center';
  }

  function writeTab(tab) {
    if (!REPORT_TABS.has(tab) || CT.state.page !== 'reports') return;
    const url = new URL(window.location.href);
    url.searchParams.set('page', 'reports');
    url.searchParams.set('tab', tab);
    window.history.replaceState({ corecontrol: true, page: 'reports', tab }, '', url);
  }

  function statusSummary() {
    return `
      <section class="reports-status-card">
        <div class="reports-status-main">
          <span class="reports-status-icon" aria-hidden="true">${icons.reports}</span>
          <div>
            <span class="reports-eyebrow">CENTRAL DE RELATÓRIOS</span>
            <h2>Encontre o relatório certo sem procurar em vários módulos</h2>
            <p>Os relatórios estão organizados por área. Escolha um tipo e prepare a exportação no formato desejado.</p>
          </div>
        </div>
        <div class="reports-status-meta">
          <span><b>${allReports.length}</b> tipos</span>
          <span><b>${groups.length}</b> áreas</span>
        </div>
      </section>`;
  }

  function center() {
    const cards = groups.map((group) => `
      <article class="reports-category-card" data-report-group="${group.key}">
        <div class="reports-category-head">
          <span class="reports-category-icon" aria-hidden="true">${icons[group.icon]}</span>
          <div>
            <h3>${CT.esc(group.title)}</h3>
            <p>${CT.esc(group.description)}</p>
          </div>
        </div>
        <div class="reports-category-actions">
          ${group.reports.map((item) => `
            <button type="button" data-report-select="${CT.esc(item)}" data-report-search="${CT.esc(`${group.title} ${item}`.toLowerCase())}">
              <span>${CT.esc(item)}</span><b aria-hidden="true">→</b>
            </button>`).join('')}
        </div>
      </article>`).join('');

    return `
      ${statusSummary()}
      <section class="card reports-library-card">
        <div class="reports-card-header reports-card-header-wrap">
          <div>
            <span class="reports-eyebrow">BIBLIOTECA</span>
            <h2>Relatórios disponíveis</h2>
            <p>Busque por assunto ou navegue pelas categorias.</p>
          </div>
          <span class="reports-count-badge">${allReports.length} relatórios</span>
        </div>
        <div class="reports-filterbar">
          <label class="reports-search-field">
            <span aria-hidden="true">${icons.search}</span>
            <input id="reportsSearch" type="search" placeholder="Buscar relatório, ex.: hardware, alertas, scripts" autocomplete="off">
          </label>
          <button class="btn small" type="button" data-reports-clear>Limpar</button>
        </div>
        <div id="reportsCategoryGrid" class="reports-category-grid">${cards}</div>
        <div id="reportsSearchEmpty" class="reports-inline-empty hidden">
          <span>${icons.search}</span>
          <strong>Nenhum relatório encontrado</strong>
          <small>Tente outro termo de busca.</small>
        </div>
      </section>`;
  }

  function audit() {
    const allowed = CT.state.user?.role === 'global_admin';
    if (!allowed) {
      return `
        <section class="reports-audit-intro">
          <span class="reports-audit-intro-icon" aria-hidden="true">${icons.lock}</span>
          <div><span class="reports-eyebrow">AUDITORIA</span><h2>Acesso restrito</h2><p>A auditoria global é exclusiva do Administrador Global.</p></div>
        </section>
        <section class="card reports-empty-card">
          <div class="reports-inline-empty tall"><span>${icons.lock}</span><strong>Sem permissão para visualizar auditoria</strong><small>Entre com uma conta de Administrador Global para acessar os registros administrativos.</small></div>
        </section>`;
    }

    return `
      <section class="reports-audit-intro">
        <span class="reports-audit-intro-icon" aria-hidden="true">${icons.audit}</span>
        <div><span class="reports-eyebrow">AUDITORIA</span><h2>Histórico administrativo do CoreControl</h2><p>Alterações críticas, acessos e operações ficam registradas para rastreabilidade.</p></div>
      </section>
      <section class="card reports-audit-card">
        <div class="reports-card-header">
          <div><h2>Registros de auditoria</h2><p>A visualização consolidada ainda precisa ser conectada ao endpoint de auditoria do backend.</p></div>
          <button class="btn" type="button" data-report-action="export-audit" disabled title="A exportação será habilitada quando o backend de auditoria estiver conectado.">${icons.export}<span>Exportar</span></button>
        </div>
        <div class="reports-audit-filters">
          <label class="reports-search-field disabled"><span aria-hidden="true">${icons.search}</span><input type="search" placeholder="Buscar usuário, ação ou empresa" disabled></label>
          <select class="reports-filter-select" disabled><option>Todas as ações</option></select>
          <select class="reports-filter-select" disabled><option>Últimos 30 dias</option></select>
        </div>
        <div class="reports-inline-empty tall">
          <span>${icons.audit}</span>
          <strong>Auditoria aguardando integração</strong>
          <small>O CoreControl já registra eventos internamente, mas esta lista consolidada ainda não possui endpoint de leitura no backend.</small>
        </div>
      </section>`;
  }

  function exportsView(selectedReport = '') {
    const options = allReports.map((item) => `<option value="${CT.esc(item)}" ${item === selectedReport ? 'selected' : ''}>${CT.esc(item)}</option>`).join('');
    return `
      <section class="reports-export-intro">
        <span class="reports-export-intro-icon" aria-hidden="true">${icons.export}</span>
        <div><span class="reports-eyebrow">EXPORTAÇÕES</span><h2>Prepare um relatório para compartilhar</h2><p>Escolha o conteúdo, período e formato. A geração só ficará ativa quando o endpoint de exportação estiver conectado.</p></div>
      </section>
      <div class="reports-export-layout">
        <section class="card reports-export-form-card">
          <div class="reports-card-header compact"><div><h2>Novo relatório</h2><p>Configure o arquivo que deseja gerar.</p></div></div>
          <form id="reportsExportForm" class="reports-export-form">
            <label class="reports-field"><span>Relatório</span><select id="reportsExportType"><option value="">Selecione...</option>${options}</select></label>
            <label class="reports-field"><span>Período</span><select id="reportsExportPeriod"><option value="7">Últimos 7 dias</option><option value="30" selected>Últimos 30 dias</option><option value="90">Últimos 90 dias</option><option value="all">Todo o período</option></select></label>
            <div class="reports-format-group">
              <span>Formato</span>
              <div class="reports-format-options">
                <label class="reports-format-option selected"><input type="radio" name="reportFormat" value="PDF" checked><span>${icons.file}<b>PDF</b><small>Documento pronto para compartilhar</small></span></label>
                <label class="reports-format-option"><input type="radio" name="reportFormat" value="Excel"><span>${icons.file}<b>Excel</b><small>Dados para análise e filtros</small></span></label>
                <label class="reports-format-option"><input type="radio" name="reportFormat" value="CSV"><span>${icons.file}<b>CSV</b><small>Formato simples para integração</small></span></label>
              </div>
            </div>
            <div class="reports-export-actions">
              <button class="btn primary" type="submit">${icons.export}<span>Gerar relatório</span></button>
              <small>A geração real ainda depende do backend de exportações.</small>
            </div>
          </form>
        </section>
        <section class="card reports-recent-card">
          <div class="reports-card-header compact"><div><h2>Exportações recentes</h2><p>Arquivos gerados pela sua conta aparecerão aqui.</p></div><span class="reports-count-badge">0</span></div>
          <div class="reports-inline-empty tall"><span>${icons.clock}</span><strong>Nenhuma exportação ainda</strong><small>Quando um arquivo for gerado, você poderá acompanhar o status e baixá-lo por aqui.</small></div>
        </section>
      </div>`;
  }

  CT.registerPage('reports', async function renderReports() {
    await CT.mountPage('reports');
    const page = CT.$('.page-reports');
    const view = CT.$('#reportsView');
    let selectedReport = '';

    const render = (tab) => {
      const safeTab = REPORT_TABS.has(tab) ? tab : 'center';
      CT.$$('[data-reports-tab]', page).forEach((button) => button.classList.toggle('active', button.dataset.reportsTab === safeTab));
      view.innerHTML = safeTab === 'audit' ? audit() : safeTab === 'exports' ? exportsView(selectedReport) : center();
      writeTab(safeTab);
    };

    function filterReports() {
      const query = (CT.$('#reportsSearch', page)?.value || '').trim().toLowerCase();
      let visibleButtons = 0;
      CT.$$('[data-report-group]', page).forEach((card) => {
        let visibleInCard = 0;
        CT.$$('[data-report-select]', card).forEach((button) => {
          const haystack = `${button.dataset.reportSearch || ''} ${button.textContent || ''}`.toLowerCase();
          const visible = !query || haystack.includes(query);
          button.classList.toggle('hidden', !visible);
          if (visible) visibleInCard += 1;
        });
        card.classList.toggle('hidden', visibleInCard === 0);
        visibleButtons += visibleInCard;
      });
      CT.$('#reportsSearchEmpty', page)?.classList.toggle('hidden', visibleButtons > 0);
    }

    page.addEventListener('input', (event) => {
      if (event.target.id === 'reportsSearch') filterReports();
    });

    page.addEventListener('change', (event) => {
      if (event.target.matches('input[name="reportFormat"]')) {
        CT.$$('.reports-format-option', page).forEach((label) => label.classList.toggle('selected', Boolean(label.querySelector('input:checked'))));
      }
    });

    page.addEventListener('submit', (event) => {
      if (event.target.id !== 'reportsExportForm') return;
      event.preventDefault();
      const reportType = CT.$('#reportsExportType', page)?.value;
      if (!reportType) {
        CT.toast('Escolha qual relatório deseja gerar.', true);
        return;
      }
      CT.toast('A interface está pronta. Falta conectar a geração de arquivos ao backend.', true);
    });

    page.addEventListener('click', (event) => {
      const tabButton = event.target.closest('[data-reports-tab]');
      if (tabButton) return render(tabButton.dataset.reportsTab);

      const clear = event.target.closest('[data-reports-clear]');
      if (clear) {
        const field = CT.$('#reportsSearch', page);
        if (field) field.value = '';
        filterReports();
        field?.focus();
        return;
      }

      const reportButton = event.target.closest('[data-report-select]');
      if (reportButton) {
        selectedReport = reportButton.dataset.reportSelect || '';
        render('exports');
        return;
      }
    });

    render(readTab());
  });
})();
